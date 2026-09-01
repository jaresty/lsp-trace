package slicer

import (
	"context"
	"errors"
	"sort"

	"lsp-trace/internal/graph"
	"lsp-trace/internal/lsp"
)

type Client interface {
	SupportsDocumentSymbols() bool
	DocumentSymbols(context.Context, lsp.DocumentSymbolParams) ([]lsp.DocumentSymbol, error)
	PrepareCallHierarchy(context.Context, lsp.PrepareCallHierarchyParams) ([]lsp.CallHierarchyItem, error)
	OutgoingCalls(context.Context, lsp.CallHierarchyItem) ([]lsp.CallHierarchyOutgoingCall, bool, error)
}

type Layer struct {
	Depth   int      `json:"depth"`
	NodeIDs []string `json:"node_ids"`
}

type Discovery struct {
	SourceURI             string
	StartNodeIDs          []string
	Layers                []Layer
	FrontierItems         []lsp.CallHierarchyItem
	OutgoingTerminalItems []lsp.CallHierarchyItem
	UpwardStartItems      []lsp.CallHierarchyItem
	Nodes                 []graph.Node
	Edges                 []graph.Edge
	Diagnostics           []graph.Diagnostic
	Complete              bool
	Truncated             bool
}

type Options struct {
	DownDepth int
	MaxNodes  int
}

type queued struct {
	item  lsp.CallHierarchyItem
	node  graph.Node
	depth int
}

type preparedStartClient struct {
	Client
	items map[uint32]lsp.CallHierarchyItem
}

func (c preparedStartClient) SupportsDocumentSymbols() bool { return true }
func (c preparedStartClient) DocumentSymbols(context.Context, lsp.DocumentSymbolParams) ([]lsp.DocumentSymbol, error) {
	lines := make([]int, 0, len(c.items))
	for line := range c.items {
		lines = append(lines, int(line))
	}
	sort.Ints(lines)
	out := make([]lsp.DocumentSymbol, 0, len(lines))
	for _, raw := range lines {
		line := uint32(raw)
		position := lsp.Position{Line: line}
		out = append(out, lsp.DocumentSymbol{Name: c.items[line].Name, Kind: c.items[line].Kind, Range: lsp.Range{Start: position, End: position}, SelectionRange: lsp.Range{Start: position, End: position}})
	}
	return out, nil
}
func (c preparedStartClient) PrepareCallHierarchy(_ context.Context, params lsp.PrepareCallHierarchyParams) ([]lsp.CallHierarchyItem, error) {
	item, ok := c.items[params.Position.Line]
	if !ok {
		return nil, nil
	}
	return []lsp.CallHierarchyItem{item}, nil
}

// DiscoverPrepared runs the same outgoing BFS from call-hierarchy items prepared by the caller.
func DiscoverPrepared(ctx context.Context, client Client, items []lsp.CallHierarchyItem, opts Options) Discovery {
	byLine := make(map[uint32]lsp.CallHierarchyItem, len(items))
	for i, item := range items {
		byLine[uint32(i)] = item
	}
	return Discover(ctx, preparedStartClient{Client: client, items: byLine}, "lsp-trace://prepared-starts", opts)
}

func Discover(ctx context.Context, client Client, sourceURI string, opts Options) Discovery {
	result := Discovery{SourceURI: sourceURI, Complete: true}
	if !client.SupportsDocumentSymbols() {
		result.Complete = false
		result.Diagnostics = append(result.Diagnostics, graph.Diagnostic{Phase: "slice-symbols", Method: "textDocument/documentSymbol", Message: "document symbols unsupported"})
		return result
	}
	symbols, err := client.DocumentSymbols(ctx, lsp.DocumentSymbolParams{TextDocument: lsp.TextDocumentIdentifier{URI: sourceURI}})
	if err != nil {
		result.Complete = false
		result.Diagnostics = append(result.Diagnostics, graph.Diagnostic{Phase: "slice-symbols", Method: "textDocument/documentSymbol", Message: err.Error()})
		return result
	}
	flat := flatten(symbols)
	sort.Slice(flat, func(i, j int) bool {
		if flat[i].SelectionRange.Start != flat[j].SelectionRange.Start {
			return lessPosition(flat[i].SelectionRange.Start, flat[j].SelectionRange.Start)
		}
		if flat[i].Kind != flat[j].Kind {
			return flat[i].Kind < flat[j].Kind
		}
		return flat[i].Name < flat[j].Name
	})

	itemsByID := map[string]lsp.CallHierarchyItem{}
	nodesByID := map[string]graph.Node{}
	depthByID := map[string]int{}
	queue := []queued{}
	for _, symbol := range flat {
		items, prepareErr := client.PrepareCallHierarchy(ctx, lsp.PrepareCallHierarchyParams{TextDocument: lsp.TextDocumentIdentifier{URI: sourceURI}, Position: symbol.SelectionRange.Start})
		if prepareErr != nil {
			result.Complete = false
			result.Diagnostics = append(result.Diagnostics, graph.Diagnostic{Phase: "slice-prepare", Method: "textDocument/prepareCallHierarchy", Message: prepareErr.Error()})
			continue
		}
		for _, item := range items {
			n := node(item)
			if err := graph.ValidateItem(n.Item); err != nil {
				result.Complete = false
				result.Diagnostics = append(result.Diagnostics, graph.Diagnostic{Phase: "slice-prepare", Method: "textDocument/prepareCallHierarchy", NodeID: n.ID, Message: err.Error()})
				continue
			}
			if existing, ok := nodesByID[n.ID]; ok && !graph.SameNodeIdentity(existing, n) {
				result.Complete = false
				continue
			}
			if _, ok := itemsByID[n.ID]; !ok {
				if opts.MaxNodes > 0 && len(itemsByID) >= opts.MaxNodes {
					result.Complete, result.Truncated = false, true
					continue
				}
				itemsByID[n.ID], nodesByID[n.ID], depthByID[n.ID] = item, n, 0
				queue = append(queue, queued{item: item, node: n})
				result.StartNodeIDs = append(result.StartNodeIDs, n.ID)
			}
		}
	}

	expanded := map[string]bool{}
	for len(queue) > 0 {
		sort.Slice(queue, func(i, j int) bool {
			if queue[i].depth != queue[j].depth {
				return queue[i].depth < queue[j].depth
			}
			return queue[i].node.ID < queue[j].node.ID
		})
		q := queue[0]
		queue = queue[1:]
		if q.depth >= opts.DownDepth || expanded[q.node.ID] {
			continue
		}
		if err := ctx.Err(); err != nil {
			result.Complete, result.Truncated = false, true
			result.Diagnostics = append(result.Diagnostics, graph.Diagnostic{Phase: "slice-outgoing", Method: "callHierarchy/outgoingCalls", NodeID: q.node.ID, Message: err.Error()})
			break
		}
		calls, wasNull, callErr := client.OutgoingCalls(ctx, q.item)
		expanded[q.node.ID] = true
		if callErr != nil || wasNull {
			result.Complete = false
			message := "server returned null"
			if callErr != nil {
				message = callErr.Error()
			}
			result.Diagnostics = append(result.Diagnostics, graph.Diagnostic{Phase: "slice-outgoing", Method: "callHierarchy/outgoingCalls", NodeID: q.node.ID, Message: message})
			continue
		}
		if len(calls) == 0 {
			result.OutgoingTerminalItems = append(result.OutgoingTerminalItems, q.item)
			continue
		}
		sort.Slice(calls, func(i, j int) bool { return node(calls[i].To).ID < node(calls[j].To).ID })
		for _, call := range calls {
			callee := node(call.To)
			if err := graph.ValidateItem(callee.Item); err != nil {
				result.Complete = false
				result.Diagnostics = append(result.Diagnostics, graph.Diagnostic{Phase: "slice-outgoing", Method: "callHierarchy/outgoingCalls", NodeID: q.node.ID, Message: err.Error()})
				continue
			}
			ranges := make([]graph.Range, len(call.FromRanges))
			valid := true
			for i, r := range call.FromRanges {
				ranges[i] = rng(r)
				if err := graph.ValidateRange(ranges[i]); err != nil {
					valid = false
					result.Complete = false
					result.Diagnostics = append(result.Diagnostics, graph.Diagnostic{Phase: "slice-outgoing", Method: "callHierarchy/outgoingCalls", NodeID: q.node.ID, Message: err.Error()})
					break
				}
			}
			if !valid {
				continue
			}
			result.Edges = graph.MergeEdge(result.Edges, graph.Edge{CallerNodeID: q.node.ID, CalleeNodeID: callee.ID, CallSites: ranges})
			if _, known := itemsByID[callee.ID]; !known {
				if opts.MaxNodes > 0 && len(itemsByID) >= opts.MaxNodes {
					result.Complete, result.Truncated = false, true
					continue
				}
				itemsByID[callee.ID], nodesByID[callee.ID], depthByID[callee.ID] = call.To, callee, q.depth+1
				queue = append(queue, queued{item: call.To, node: callee, depth: q.depth + 1})
			}
		}
	}

	for id, n := range nodesByID {
		result.Nodes = append(result.Nodes, n)
		depth := depthByID[id]
		for len(result.Layers) <= depth {
			result.Layers = append(result.Layers, Layer{Depth: len(result.Layers)})
		}
		result.Layers[depth].NodeIDs = append(result.Layers[depth].NodeIDs, id)
		if depth == opts.DownDepth {
			result.FrontierItems = append(result.FrontierItems, itemsByID[id])
		}
	}
	for i := range result.Layers {
		sort.Strings(result.Layers[i].NodeIDs)
	}
	sort.Strings(result.StartNodeIDs)
	sort.Slice(result.Nodes, func(i, j int) bool { return result.Nodes[i].ID < result.Nodes[j].ID })
	sort.Slice(result.FrontierItems, func(i, j int) bool { return node(result.FrontierItems[i]).ID < node(result.FrontierItems[j]).ID })
	sort.Slice(result.OutgoingTerminalItems, func(i, j int) bool {
		return node(result.OutgoingTerminalItems[i]).ID < node(result.OutgoingTerminalItems[j]).ID
	})
	upwardByID := make(map[string]lsp.CallHierarchyItem, len(result.FrontierItems)+len(result.OutgoingTerminalItems))
	for _, item := range append(append([]lsp.CallHierarchyItem{}, result.FrontierItems...), result.OutgoingTerminalItems...) {
		upwardByID[node(item).ID] = item
	}
	upwardIDs := make([]string, 0, len(upwardByID))
	for id := range upwardByID {
		upwardIDs = append(upwardIDs, id)
	}
	sort.Strings(upwardIDs)
	for _, id := range upwardIDs {
		result.UpwardStartItems = append(result.UpwardStartItems, upwardByID[id])
	}
	sort.Slice(result.Edges, func(i, j int) bool {
		if result.Edges[i].CallerNodeID != result.Edges[j].CallerNodeID {
			return result.Edges[i].CallerNodeID < result.Edges[j].CallerNodeID
		}
		return result.Edges[i].CalleeNodeID < result.Edges[j].CalleeNodeID
	})
	if errors.Is(ctx.Err(), context.Canceled) {
		result.Complete = false
	}
	return result
}

func flatten(symbols []lsp.DocumentSymbol) []lsp.DocumentSymbol {
	var out []lsp.DocumentSymbol
	for _, symbol := range symbols {
		out = append(out, symbol)
		out = append(out, flatten(symbol.Children)...)
	}
	return out
}

func lessPosition(a, b lsp.Position) bool {
	return a.Line < b.Line || (a.Line == b.Line && a.Character < b.Character)
}
func rng(r lsp.Range) graph.Range {
	return graph.Range{Start: graph.Position{Line: r.Start.Line, Character: r.Start.Character}, End: graph.Position{Line: r.End.Line, Character: r.End.Character}}
}
func node(item lsp.CallHierarchyItem) graph.Node {
	return graph.NewNode(graph.Item{Name: item.Name, Kind: item.Kind, Detail: item.Detail, URI: item.URI, Range: rng(item.Range), SelectionRange: rng(item.SelectionRange), Data: item.Data})
}
