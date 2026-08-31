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
}

type Options struct {
	MaxDepth      int
	MaxNodes      int
	NodeFactory   func(graph.Item) graph.Node
	SchemaVersion string
}
type queued struct {
	item  lsp.CallHierarchyItem
	node  graph.Node
	depth int
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
	if len(items) == 0 {
		result.Terminals = append(result.Terminals, graph.Boundary{Reason: graph.PrepareReturnedNoItem})
		result.Canonicalize()
		return result
	}
	seen := map[string]graph.Node{}
	expanded := map[string]bool{}
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
			for _, fromRange := range call.FromRanges {
				if err := graph.ValidateRange(rng(fromRange), caller.Range); err != nil {
					invalidRange = err
					break
				}
			}
			if invalidRange != nil {
				result.Terminals = append(result.Terminals, graph.Boundary{NodeID: caller.ID, Reason: graph.InvalidServerResponse, Message: invalidRange.Error()})
				result.Diagnostics = append(result.Diagnostics, graph.Diagnostic{Phase: "traverse", Method: "callHierarchy/incomingCalls", NodeID: q.node.ID, Message: invalidRange.Error()})
				result.Summary.Complete = false
				continue
			}
			if existing, ok := seen[caller.ID]; ok && !graph.SameNodeIdentity(existing, caller) {
				result.Terminals = append(result.Terminals, graph.Boundary{NodeID: caller.ID, Reason: graph.NodeIDCollision})
				result.Summary.Complete = false
				continue
			}
			_, known := seen[caller.ID]
			if !known && opts.MaxNodes > 0 && len(seen) >= opts.MaxNodes {
				result.Frontier = append(result.Frontier, graph.Boundary{NodeID: caller.ID, Reason: graph.MaxNodes})
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
			before := len(result.Edges)
			result.Edges = graph.MergeEdge(result.Edges, graph.Edge{CallerNodeID: caller.ID, CalleeNodeID: q.node.ID, CallSites: ranges})
			if len(result.Edges) > before && caller.URI != q.node.URI {
				result.CapabilityQuality.CrossFileEdges++
			}
		}
	}
	for _, n := range seen {
		result.Nodes = append(result.Nodes, n)
	}
	result.Canonicalize()
	return result
}

func node(i lsp.CallHierarchyItem, factory func(graph.Item) graph.Node) graph.Node {
	return factory(graph.Item{Name: i.Name, Kind: i.Kind, Detail: i.Detail, URI: i.URI, Range: rng(i.Range), SelectionRange: rng(i.SelectionRange), Data: i.Data})
}
func rng(r lsp.Range) graph.Range {
	return graph.Range{Start: graph.Position{Line: r.Start.Line, Character: r.Start.Character}, End: graph.Position{Line: r.End.Line, Character: r.End.Character}}
}
