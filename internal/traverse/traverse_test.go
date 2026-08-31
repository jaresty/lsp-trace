package traverse

import (
	"context"
	"encoding/json"
	"testing"

	"lsp-trace/internal/graph"
	"lsp-trace/internal/lsp"
)

type fakeClient struct {
	targets  []lsp.CallHierarchyItem
	calls    map[string][]lsp.CallHierarchyIncomingCall
	seenData map[string]json.RawMessage
}

func (f *fakeClient) PrepareCallHierarchy(context.Context, lsp.PrepareCallHierarchyParams) ([]lsp.CallHierarchyItem, error) {
	return f.targets, nil
}
func (f *fakeClient) IncomingCalls(_ context.Context, item lsp.CallHierarchyItem) ([]lsp.CallHierarchyIncomingCall, bool, error) {
	if f.seenData != nil {
		f.seenData[item.Name] = append([]byte(nil), item.Data...)
	}
	return f.calls[item.Name], false, nil
}
func item(name string, line uint32) lsp.CallHierarchyItem {
	return lsp.CallHierarchyItem{Name: name, Kind: 12, URI: "file:///w/a.go", Range: lsp.Range{Start: lsp.Position{Line: line}, End: lsp.Position{Line: line, Character: 4}}, SelectionRange: lsp.Range{Start: lsp.Position{Line: line}, End: lsp.Position{Line: line, Character: 4}}, Data: json.RawMessage(`{"name":"` + name + `"}`)}
}

func TestIncomingBranchingDiamondAndOpaqueData(t *testing.T) {
	leaf, a, b, root := item("leaf", 8), item("a", 5), item("b", 6), item("root", 1)
	f := &fakeClient{targets: []lsp.CallHierarchyItem{leaf}, seenData: map[string]json.RawMessage{}, calls: map[string][]lsp.CallHierarchyIncomingCall{
		"leaf": {{From: b, FromRanges: []lsp.Range{b.Range}}, {From: a, FromRanges: []lsp.Range{a.Range}}},
		"a":    {{From: root, FromRanges: []lsp.Range{root.Range}}}, "b": {{From: root, FromRanges: []lsp.Range{root.Range}}}, "root": {}}}
	r := Incoming(context.Background(), f, lsp.PrepareCallHierarchyParams{}, Options{})
	if !r.Summary.Complete || r.Summary.NodeCount != 4 || r.Summary.EdgeCount != 4 || r.Summary.TerminalCount != 1 {
		t.Fatalf("ASSERT_TRAVERSAL_COMPLETE_ACCOUNTED: %#v", r.Summary)
	}
	if string(f.seenData["leaf"]) != string(leaf.Data) {
		t.Fatalf("ASSERT_OPAQUE_DATA_REPLAYED: got %s want %s", f.seenData["leaf"], leaf.Data)
	}
	if r.Nodes[0].Name != "root" {
		t.Fatalf("ASSERT_CANONICAL_NODE_ORDER: first=%s", r.Nodes[0].Name)
	}
}

func TestIncomingDepthLimitCreatesFrontier(t *testing.T) {
	leaf, caller := item("leaf", 8), item("caller", 4)
	f := &fakeClient{targets: []lsp.CallHierarchyItem{leaf}, calls: map[string][]lsp.CallHierarchyIncomingCall{"leaf": {{From: caller}}}}
	r := Incoming(context.Background(), f, lsp.PrepareCallHierarchyParams{}, Options{MaxDepth: 1})
	if r.Summary.Complete || !r.Summary.Truncated || len(r.Frontier) != 1 || r.Frontier[0].Reason != graph.MaxDepth {
		t.Fatalf("ASSERT_DEPTH_FRONTIER_VISIBLE: %#v %#v", r.Summary, r.Frontier)
	}
}
