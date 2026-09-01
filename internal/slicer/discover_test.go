package slicer

import (
	"context"
	"encoding/json"
	"testing"

	"lsp-trace/internal/graph"
	"lsp-trace/internal/lsp"
)

type fakeClient struct {
	symbols  []lsp.DocumentSymbol
	prepared map[uint32][]lsp.CallHierarchyItem
	outgoing map[string][]lsp.CallHierarchyOutgoingCall
}

func (f *fakeClient) SupportsDocumentSymbols() bool { return true }
func (f *fakeClient) DocumentSymbols(context.Context, lsp.DocumentSymbolParams) ([]lsp.DocumentSymbol, error) {
	return f.symbols, nil
}
func (f *fakeClient) PrepareCallHierarchy(_ context.Context, p lsp.PrepareCallHierarchyParams) ([]lsp.CallHierarchyItem, error) {
	return f.prepared[p.Position.Line], nil
}
func (f *fakeClient) OutgoingCalls(_ context.Context, item lsp.CallHierarchyItem) ([]lsp.CallHierarchyOutgoingCall, bool, error) {
	return f.outgoing[item.Name], false, nil
}

func callItem(name string, line uint32) lsp.CallHierarchyItem {
	r := lsp.Range{Start: lsp.Position{Line: line}, End: lsp.Position{Line: line, Character: 1}}
	return lsp.CallHierarchyItem{Name: name, Kind: 12, URI: "file:///w/code.go", Range: r, SelectionRange: r, Data: json.RawMessage(`{"name":"` + name + `"}`)}
}

func symbol(name string, line uint32, children ...lsp.DocumentSymbol) lsp.DocumentSymbol {
	r := lsp.Range{Start: lsp.Position{Line: line}, End: lsp.Position{Line: line, Character: 1}}
	return lsp.DocumentSymbol{Name: name, Kind: 12, Range: r, SelectionRange: r, Children: children}
}

func TestDiscoverExactDepthFrontierIsDeterministicAndDeduplicated(t *testing.T) {
	a, b, c, d := callItem("a", 1), callItem("b", 2), callItem("c", 3), callItem("d", 4)
	f := &fakeClient{
		symbols:  []lsp.DocumentSymbol{symbol("outer", 20, symbol("a", 1)), symbol("b", 2)},
		prepared: map[uint32][]lsp.CallHierarchyItem{1: {a}, 2: {b}},
		outgoing: map[string][]lsp.CallHierarchyOutgoingCall{
			"a": {{To: c}}, "b": {{To: c}}, "c": {{To: d}}, "d": {},
		},
	}
	got := Discover(context.Background(), f, "file:///w/code.go", Options{DownDepth: 2})
	if !got.Complete || len(got.StartNodeIDs) != 2 || len(got.Layers) != 3 || len(got.Layers[1].NodeIDs) != 1 || len(got.FrontierItems) != 1 || got.FrontierItems[0].Name != "d" || len(got.Edges) != 3 {
		t.Fatalf("ASSERT_SLICE_EXACT_DEPTH_DEDUP: %#v", got)
	}
	if got.Layers[0].Depth != 0 || got.Layers[2].Depth != 2 || got.Layers[2].NodeIDs[0] != graph.NewNode(graph.Item{Name: d.Name, Kind: d.Kind, URI: d.URI, Range: graph.Range{Start: graph.Position{Line: 4}, End: graph.Position{Line: 4, Character: 1}}, SelectionRange: graph.Range{Start: graph.Position{Line: 4}, End: graph.Position{Line: 4, Character: 1}}, Data: d.Data}).ID {
		t.Fatalf("ASSERT_SLICE_LAYER_IDS_NATIVE: %#v", got.Layers)
	}
}
