package traverse

import (
	"context"
	"errors"
	"sort"

	"lsp-trace/internal/graph"
	"lsp-trace/internal/lsp"
)

type Client interface {
	PrepareCallHierarchy(context.Context, lsp.PrepareCallHierarchyParams) ([]lsp.CallHierarchyItem, error)
	IncomingCalls(context.Context, lsp.CallHierarchyItem) ([]lsp.CallHierarchyIncomingCall, bool, error)
	SupportsDocumentSymbols() bool
	DocumentSymbols(context.Context, lsp.DocumentSymbolParams) ([]lsp.DocumentSymbol, error)
}

type Options struct {
	MaxDepth               int
	MaxNodes               int
	NodeFactory            func(graph.Item) graph.Node
	SchemaVersion          string
	IncludeTopmostSiblings bool
}
type queued struct {
	item  lsp.CallHierarchyItem
	node  graph.Node
	depth int
}

type preparedClient struct {
	Client
	items []lsp.CallHierarchyItem
}

func (c preparedClient) PrepareCallHierarchy(context.Context, lsp.PrepareCallHierarchyParams) ([]lsp.CallHierarchyItem, error) {
	return c.items, nil
}

// IncomingPrepared reuses the incoming traversal with already-prepared items.
func IncomingPrepared(ctx context.Context, client Client, items []lsp.CallHierarchyItem, opts Options) graph.Result {
	return Incoming(ctx, preparedClient{Client: client, items: items}, lsp.PrepareCallHierarchyParams{}, opts)
}

func Incoming(ctx context.Context, client Client, params lsp.PrepareCallHierarchyParams, opts Options) graph.Result {
	version := opts.SchemaVersion
	if version == "" {
		version = graph.SchemaVersion
	}
	result := graph.Result{SchemaVersion: version, Summary: graph.Summary{Complete: true}, CapabilityQuality: graph.CapabilityQuality{CrossModuleEdges: graph.Unknown}}
	newNode := opts.NodeFactory
	if newNode == nil {
		newNode = graph.NewNode
	}
	items, err := client.PrepareCallHierarchy(ctx, params)
	if err != nil {
		result.Diagnostics = append(result.Diagnostics, graph.Diagnostic{Phase: "prepare", Method: "textDocument/prepareCallHierarchy", Message: err.Error()})
		result.Summary.Complete = false
		switch {
		case errors.Is(err, context.DeadlineExceeded):
			result.Terminals = append(result.Terminals, graph.Boundary{Reason: graph.RequestTimeout, Message: err.Error()})
		case errors.Is(err, context.Canceled):
			result.Terminals = append(result.Terminals, graph.Boundary{Reason: graph.Cancelled, Message: err.Error()})
		}
		result.Canonicalize()
		return result
	}
	result.CapabilityQuality.PrepareSucceeded = true
	if opts.IncludeTopmostSiblings && client.SupportsDocumentSymbols() {
		symbols, symbolErr := client.DocumentSymbols(ctx, lsp.DocumentSymbolParams{TextDocument: params.TextDocument})
		if symbolErr != nil {
			result.Diagnostics = append(result.Diagnostics, graph.Diagnostic{Phase: "siblings", Method: "textDocument/documentSymbol", Message: symbolErr.Error()})
		} else {
			for _, symbol := range topmostSymbols(symbols) {
				candidate := newNode(graph.Item{Name: symbol.Name, Kind: symbol.Kind, Detail: symbol.Detail, URI: params.TextDocument.URI, Range: rng(symbol.Range), SelectionRange: rng(symbol.SelectionRange)})
				if graph.ValidateItem(candidate.Item) == nil {
					result.SiblingCandidates = append(result.SiblingCandidates, graph.SiblingCandidate{SeedURI: params.TextDocument.URI, Candidate: candidate})
				}
			}
		}
	}
	if len(items) == 0 {
		result.Terminals = append(result.Terminals, graph.Boundary{Reason: graph.PrepareReturnedNoItem})
		result.Canonicalize()
		return result
	}
	seen := map[string]graph.Node{}
	expanded := map[string]bool{}
	edgeIndex := map[[2]string]int{}
	queue := make([]queued, 0, len(items))
	for _, item := range items {
		n := node(item, newNode)
		if err := graph.ValidateItem(n.Item); err != nil {
			result.Terminals = append(result.Terminals, graph.Boundary{NodeID: n.ID, Reason: graph.InvalidServerResponse, Message: err.Error()})
			result.Diagnostics = append(result.Diagnostics, graph.Diagnostic{Phase: "prepare", Method: "textDocument/prepareCallHierarchy", NodeID: n.ID, Message: err.Error()})
			result.Summary.Complete = false
			continue
		}
		if _, ok := seen[n.ID]; !ok {
			seen[n.ID] = n
			queue = append(queue, queued{item: item, node: n})
		}
		result.Targets = append(result.Targets, n.ID)
	}
	for len(queue) > 0 {
		sort.Slice(queue, func(i, j int) bool {
			if queue[i].depth != queue[j].depth {
				return queue[i].depth < queue[j].depth
			}
			return queue[i].node.ID < queue[j].node.ID
		})
		q := queue[0]
		queue = queue[1:]
		if err := ctx.Err(); err != nil {
			reason := graph.Cancelled
			if errors.Is(err, context.DeadlineExceeded) {
				reason = graph.GlobalTimeout
			}
			result.Summary.Complete = false
			result.Frontier = append(result.Frontier, graph.Boundary{NodeID: q.node.ID, Reason: reason, Message: err.Error()})
			for _, pending := range queue {
				if !expanded[pending.node.ID] {
					result.Frontier = append(result.Frontier, graph.Boundary{NodeID: pending.node.ID, Reason: reason, Message: err.Error()})
				}
			}
			break
		}
		if expanded[q.node.ID] {
			continue
		}
		if opts.MaxDepth > 0 && q.depth >= opts.MaxDepth {
			result.Frontier = append(result.Frontier, graph.Boundary{NodeID: q.node.ID, Reason: graph.MaxDepth})
			result.Summary.Complete = false
			result.Summary.Truncated = true
			continue
		}
		calls, wasNull, err := client.IncomingCalls(ctx, q.item)
		expanded[q.node.ID] = true
		if err != nil {
			reason := graph.ServerError
			if errors.Is(err, context.DeadlineExceeded) {
				reason = graph.RequestTimeout
			}
			if errors.Is(err, context.Canceled) {
				reason = graph.Cancelled
			}
			result.Terminals = append(result.Terminals, graph.Boundary{NodeID: q.node.ID, Reason: reason, Message: err.Error()})
			result.Diagnostics = append(result.Diagnostics, graph.Diagnostic{Phase: "traverse", Method: "callHierarchy/incomingCalls", NodeID: q.node.ID, Message: err.Error()})
			result.Summary.Complete = false
			continue
		}
		result.CapabilityQuality.IncomingRequestSuccesses++
		if wasNull {
			result.Terminals = append(result.Terminals, graph.Boundary{NodeID: q.node.ID, Reason: graph.IncomingReturnedNull})
			continue
		}
		if len(calls) == 0 {
			result.Terminals = append(result.Terminals, graph.Boundary{NodeID: q.node.ID, Reason: graph.ServerReportedNoIncoming})
			continue
		}
		for _, call := range calls {
			caller := node(call.From, newNode)
			if err := graph.ValidateItem(caller.Item); err != nil {
				result.Terminals = append(result.Terminals, graph.Boundary{NodeID: caller.ID, Reason: graph.InvalidServerResponse, Message: err.Error()})
				result.Diagnostics = append(result.Diagnostics, graph.Diagnostic{Phase: "traverse", Method: "callHierarchy/incomingCalls", NodeID: q.node.ID, Message: err.Error()})
				result.Summary.Complete = false
				continue
			}
			invalidRange := error(nil)
			outsideCallerRange := false
			for _, fromRange := range call.FromRanges {
				normalized := rng(fromRange)
				if err := graph.ValidateRange(normalized); err != nil {
					invalidRange = err
					break
				}
				if !graph.RangeContains(caller.Range, normalized) {
					outsideCallerRange = true
				}
			}
			if invalidRange != nil {
				result.Terminals = append(result.Terminals, graph.Boundary{NodeID: caller.ID, Reason: graph.InvalidServerResponse, Message: invalidRange.Error()})
				result.Diagnostics = append(result.Diagnostics, graph.Diagnostic{Phase: "traverse", Method: "callHierarchy/incomingCalls", NodeID: q.node.ID, Message: invalidRange.Error()})
				result.Summary.Complete = false
				continue
			}
			if outsideCallerRange {
				result.Diagnostics = append(result.Diagnostics, graph.Diagnostic{Phase: "traverse", Method: "callHierarchy/incomingCalls", NodeID: caller.ID, Message: "SERVER_CALL_SITE_OUTSIDE_CALLER_RANGE"})
			}
			if existing, ok := seen[caller.ID]; ok && !graph.SameNodeIdentity(existing, caller) {
				result.Terminals = append(result.Terminals, graph.Boundary{NodeID: caller.ID, Reason: graph.NodeIDCollision})
				result.Summary.Complete = false
				continue
			}
			_, known := seen[caller.ID]
			if !known && opts.MaxNodes > 0 && len(seen) >= opts.MaxNodes {
				result.Frontier = append(result.Frontier, graph.Boundary{NodeID: q.node.ID, Reason: graph.MaxNodes, Message: "caller omitted by node bound: " + caller.ID})
				result.Summary.Complete = false
				result.Summary.Truncated = true
				continue
			}
			if !known {
				seen[caller.ID] = caller
				queue = append(queue, queued{item: call.From, node: caller, depth: q.depth + 1})
			}
			ranges := make([]graph.Range, len(call.FromRanges))
			for i, r := range call.FromRanges {
				ranges[i] = rng(r)
			}
			key := [2]string{caller.ID, q.node.ID}
			if index, ok := edgeIndex[key]; ok {
				result.Edges[index].CallSites = append(result.Edges[index].CallSites, ranges...)
			} else {
				edgeIndex[key] = len(result.Edges)
				result.Edges = append(result.Edges, graph.Edge{CallerNodeID: caller.ID, CalleeNodeID: q.node.ID, CallSites: ranges})
				if caller.URI != q.node.URI {
					result.CapabilityQuality.CrossFileEdges++
				}
			}
		}
	}
	for _, n := range seen {
		result.Nodes = append(result.Nodes, n)
	}
	result.Canonicalize()
	return result
}

func topmostSymbols(symbols []lsp.DocumentSymbol) []lsp.DocumentSymbol {
	// Hierarchical document symbols are a forest. Flattening the forest while
	// retaining entries without an ancestor selects exactly its roots.
	out := make([]lsp.DocumentSymbol, len(symbols))
	copy(out, symbols)
	return out
}

func node(i lsp.CallHierarchyItem, factory func(graph.Item) graph.Node) graph.Node {
	return factory(graph.Item{Name: i.Name, Kind: i.Kind, Detail: i.Detail, URI: i.URI, Range: rng(i.Range), SelectionRange: rng(i.SelectionRange), Data: i.Data})
}
func rng(r lsp.Range) graph.Range {
	return graph.Range{Start: graph.Position{Line: r.Start.Line, Character: r.Start.Character}, End: graph.Position{Line: r.End.Line, Character: r.End.Character}}
}
