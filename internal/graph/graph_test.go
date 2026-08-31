package graph

import (
	"bytes"
	"encoding/json"
	"reflect"
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

func TestCanonicalizeSortsAllCollectionsAndCountsCycles(t *testing.T) {
	r1 := Range{Start: Position{Line: 8}, End: Position{Line: 8, Character: 4}}
	r2 := Range{Start: Position{Line: 2}, End: Position{Line: 2, Character: 4}}
	r := Result{
		Targets: []string{"b", "a"},
		Nodes:   []Node{{ID: "b", Item: Item{URI: "file:///b"}}, {ID: "a", Item: Item{URI: "file:///a"}}},
		Edges: []Edge{
			{CallerNodeID: "b", CalleeNodeID: "a", CallSites: []Range{r1, r2, r1}},
			{CallerNodeID: "a", CalleeNodeID: "b"},
		},
		Terminals:   []Boundary{{NodeID: "a", Reason: ServerError, Message: "z"}, {NodeID: "a", Reason: ServerError, Message: "a"}},
		Frontier:    []Boundary{{NodeID: "b", Reason: MaxNodes, Message: "z"}, {NodeID: "b", Reason: MaxNodes, Message: "a"}},
		Diagnostics: []Diagnostic{{Phase: "traverse", Method: "z", NodeID: "b", Message: "z"}, {Phase: "prepare", Method: "a", NodeID: "a", Message: "a"}},
	}
	r.Canonicalize()

	if r.Summary.CycleCount != 1 {
		t.Fatalf("ASSERT_SCC_CYCLE_COUNT: got %d, want 1", r.Summary.CycleCount)
	}
	if got := r.Edges[1].CallSites; !reflect.DeepEqual(got, []Range{r2, r1}) {
		t.Fatalf("ASSERT_CANONICAL_CALL_SITES: %#v", got)
	}
	if r.Terminals[0].Message != "a" || r.Frontier[0].Message != "a" || r.Diagnostics[0].Phase != "prepare" {
		t.Fatalf("ASSERT_CANONICAL_AUXILIARY_ORDER: terminals=%#v frontier=%#v diagnostics=%#v", r.Terminals, r.Frontier, r.Diagnostics)
	}
}

func TestValidateItemRejectsMissingFieldsAndInvalidRanges(t *testing.T) {
	valid := testItem()
	for _, tc := range []struct {
		name string
		edit func(*Item)
	}{
		{name: "missing name", edit: func(i *Item) { i.Name = "" }},
		{name: "invalid URI", edit: func(i *Item) { i.URI = "relative.go" }},
		{name: "reversed range", edit: func(i *Item) { i.Range = Range{Start: Position{Line: 3}, End: Position{Line: 2}} }},
		{name: "selection outside range", edit: func(i *Item) { i.SelectionRange = Range{Start: Position{Line: 9}, End: Position{Line: 9}} }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			item := valid
			tc.edit(&item)
			if err := ValidateItem(item); err == nil {
				t.Fatalf("ASSERT_INVALID_ITEM_REJECTED: %s", tc.name)
			}
		})
	}
	if err := ValidateItem(valid); err != nil {
		t.Fatalf("ASSERT_VALID_ITEM_ACCEPTED: %v", err)
	}
}

func TestSameNodeIdentityDetectsFabricatedIDCollision(t *testing.T) {
	a := NewNode(testItem())
	b := a
	b.URI = "file:///workspace/other.go"
	if SameNodeIdentity(a, b) {
		t.Fatal("ASSERT_NODE_ID_COLLISION_DETECTED: differing identity fields treated as equal")
	}
	b = a
	b.Data = jsonBytes(`{"transport":"different"}`)
	if !SameNodeIdentity(a, b) {
		t.Fatal("ASSERT_OPAQUE_DATA_NOT_COLLISION: opaque data changed identity")
	}
}

func TestResultSchemaSeedProjection(t *testing.T) {
	r := Result{SchemaVersion: SchemaVersionV2, Seeds: []SeedResult{{Label: "interface", Requested: Target{URI: "file:///a.go", Line: 1, Column: 1}}}}
	encoded, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	var v2 map[string]any
	if err := json.Unmarshal(encoded, &v2); err != nil {
		t.Fatal(err)
	}
	if _, ok := v2["seeds"]; !ok {
		t.Fatalf("ASSERT_V2_SEED_PROVENANCE: %s", encoded)
	}

	r.SchemaVersion = SchemaVersionV1
	r.Seeds = nil
	encoded, err = json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	var v1 map[string]any
	if err := json.Unmarshal(encoded, &v1); err != nil {
		t.Fatal(err)
	}
	if _, ok := v1["seeds"]; ok {
		t.Fatalf("ASSERT_V1_HAS_NO_SEEDS: %s", encoded)
	}
}

func TestMergeResultsCollapsesDuplicateNodesAndPreservesSeedOutcomes(t *testing.T) {
	node := NewNode(testItem())
	r1 := Result{
		SchemaVersion:     SchemaVersionV2,
		Nodes:             []Node{node},
		Targets:           []string{node.ID},
		Terminals:         []Boundary{{NodeID: node.ID, Reason: ServerReportedNoIncoming}},
		Diagnostics:       []Diagnostic{{Phase: "z", Message: "same"}},
		Summary:           Summary{Complete: true},
		CapabilityQuality: CapabilityQuality{PrepareSucceeded: true, IncomingRequestSuccesses: 1, CrossModuleEdges: Unknown},
		Seeds:             []SeedResult{{Label: "interface", Requested: Target{URI: node.URI, Line: 1, Column: 1}, PreparedTargetIDs: []string{node.ID}, ReachedNodeIDs: []string{node.ID}}},
	}
	r2 := r1
	r2.Seeds = []SeedResult{{Label: "implementation", Requested: Target{URI: node.URI, Line: 2, Column: 1}, PreparedTargetIDs: []string{node.ID}, ReachedNodeIDs: []string{node.ID}, Failure: &SeedFailure{Phase: "prepare", Message: "failed"}}}
	r2.Summary.Complete = false

	merged := MergeResults(r2, r1)
	if len(merged.Nodes) != 1 || len(merged.Targets) != 1 || len(merged.Terminals) != 1 || len(merged.Diagnostics) != 1 {
		t.Fatalf("ASSERT_DUPLICATE_SEEDS_COLLAPSE: %#v", merged)
	}
	if merged.Summary.Complete || len(merged.Seeds) != 2 || merged.Seeds[0].Label != "implementation" || merged.Seeds[1].Label != "interface" {
		t.Fatalf("ASSERT_FAILED_SEED_INCOMPLETE_WITH_GRAPH: %#v", merged)
	}
}

func TestCanonicalizeEquivalentInsertionOrdersProduceIdenticalJSON(t *testing.T) {
	makeResult := func(reverse bool) Result {
		nodes := []Node{{ID: "a", Item: Item{URI: "file:///a"}}, {ID: "b", Item: Item{URI: "file:///b"}}}
		edges := []Edge{{CallerNodeID: "a", CalleeNodeID: "b"}, {CallerNodeID: "b", CalleeNodeID: "a"}}
		if reverse {
			nodes[0], nodes[1] = nodes[1], nodes[0]
			edges[0], edges[1] = edges[1], edges[0]
		}
		return Result{SchemaVersion: SchemaVersion, Nodes: nodes, Edges: edges}
	}
	a, b := makeResult(false), makeResult(true)
	a.Canonicalize()
	b.Canonicalize()
	aj, _ := json.Marshal(a)
	bj, _ := json.Marshal(b)
	if !bytes.Equal(aj, bj) {
		t.Fatalf("ASSERT_CANONICAL_JSON_STABLE: %s != %s", aj, bj)
	}
}
