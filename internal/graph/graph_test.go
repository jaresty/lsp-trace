package graph

import (
	"bytes"
	"testing"
)

func testItem() Item {
	return Item{
		Name: "leaf", Kind: 12, Detail: "func()", URI: "file:///workspace/a.go",
		Range:          Range{Start: Position{Line: 1}, End: Position{Line: 3}},
		SelectionRange: Range{Start: Position{Line: 1, Character: 5}, End: Position{Line: 1, Character: 9}},
		Data:           jsonBytes(`{"token":7,"nested":{"ok":true}}`),
	}
}

func jsonBytes(s string) []byte { return []byte(s) }

func TestNodeIdentityDeterministicAndDataIndependent(t *testing.T) {
	a := testItem()
	b := testItem()
	b.Data = jsonBytes(`{"unstable":"different"}`)

	first := NewNode(a)
	second := NewNode(b)
	if first.ID == "" || first.ID != second.ID {
		t.Fatalf("ASSERT_NODE_ID_DETERMINISTIC: ids = %q and %q", first.ID, second.ID)
	}
	if !bytes.Equal(first.Data, a.Data) {
		t.Fatalf("ASSERT_OPAQUE_DATA_PRESERVED: got %s, want %s", first.Data, a.Data)
	}
}

func TestMergeEdgeUnionsAndSortsCallSites(t *testing.T) {
	r1 := Range{Start: Position{Line: 8}, End: Position{Line: 8, Character: 4}}
	r2 := Range{Start: Position{Line: 2}, End: Position{Line: 2, Character: 4}}
	edges := MergeEdge(nil, Edge{CallerNodeID: "caller", CalleeNodeID: "callee", CallSites: []Range{r1}})
	edges = MergeEdge(edges, Edge{CallerNodeID: "caller", CalleeNodeID: "callee", CallSites: []Range{r2, r1}})

	if len(edges) != 1 || len(edges[0].CallSites) != 2 || edges[0].CallSites[0] != r2 || edges[0].CallSites[1] != r1 {
		t.Fatalf("ASSERT_EDGE_RANGE_UNION_SORTED: %#v", edges)
	}
}
