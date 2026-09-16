package transientstructural

import (
	"encoding/json"
	"reflect"
	"testing"

	"lsp-trace/internal/graph"
)

func TestProjectionPreservesItemAndSelectionRangesInNodeFacts(t *testing.T) {
	itemRange := graph.Range{
		Start: graph.Position{Line: 10, Character: 2},
		End:   graph.Position{Line: 14, Character: 1},
	}
	selectionRange := graph.Range{
		Start: graph.Position{Line: 10, Character: 7},
		End:   graph.Position{Line: 10, Character: 13},
	}
	node := graph.NewNode(graph.Item{
		Name:           "Target",
		Kind:           12,
		URI:            "file:///workspace/target.go",
		Range:          itemRange,
		SelectionRange: selectionRange,
	})
	projection, err := project(
		"session",
		1,
		traversalProjection{root: node.ID, nodes: []graph.Node{node}},
		traversalProjection{root: node.ID, nodes: []graph.Node{node}},
		testBounds(),
		Accounting{},
	)
	if err != nil || len(projection.nodes) != 1 {
		t.Fatalf("ASSERT_TRANSIENT_NODE_FACT_PRESERVES_ITEM_AND_SELECTION_RANGES: projection=%+v err=%v", projection, err)
	}

	fact := reflect.ValueOf(projection.nodes[0])
	gotItem := fact.FieldByName("ItemRange")
	gotSelection := fact.FieldByName("SelectionRange")
	if !gotItem.IsValid() || !gotSelection.IsValid() ||
		gotItem.Interface().(graph.Range) != itemRange ||
		gotSelection.Interface().(graph.Range) != selectionRange {
		t.Fatalf("ASSERT_TRANSIENT_NODE_FACT_PRESERVES_ITEM_AND_SELECTION_RANGES: fact=%+v item_field=%t selection_field=%t", projection.nodes[0], gotItem.IsValid(), gotSelection.IsValid())
	}

	raw, err := json.Marshal(projection.nodes[0])
	if err != nil {
		t.Fatal(err)
	}
	var public map[string]any
	if err := json.Unmarshal(raw, &public); err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"Range", "ItemRange", "SelectionRange", "range", "item_range", "selection_range"} {
		if _, ok := public[forbidden]; ok {
			t.Fatalf("ASSERT_TRANSIENT_NODE_RANGE_CARRIERS_PRIVATE: field=%s json=%s", forbidden, raw)
		}
	}
}
