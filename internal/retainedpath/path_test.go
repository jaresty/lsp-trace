package retainedpath

import (
	"context"
	"reflect"
	"testing"
)

func diamond() ([]string, []Edge) {
	return []string{"a", "b", "c", "d", "isolate"}, []Edge{
		{GroupID: "z", Caller: "a", Callee: "b", Weight: 1, OccurrenceIDs: []string{"z-site"}},
		{GroupID: "ac", Caller: "a", Callee: "c", Weight: 1, OccurrenceIDs: []string{"ac-site"}},
		{GroupID: "ab", Caller: "a", Callee: "b", Weight: 1, OccurrenceIDs: []string{}},
		{GroupID: "cd", Caller: "c", Callee: "d", Weight: 1, OccurrenceIDs: []string{}},
		{GroupID: "bd", Caller: "b", Callee: "d", Weight: 1, OccurrenceIDs: []string{"bd-1", "bd-2"}},
	}
}

// Err becomes cancelled after an exact number of checks, avoiding timing races.
type cancellingContext struct {
	context.Context
	checks int
}

func (c *cancellingContext) Err() error {
	c.checks--
	if c.checks < 0 {
		return context.Canceled
	}
	return nil
}
func TestCancellationDuringSearch(t *testing.T) {
	nodes, edges := diamond()
	// Initial check, node a, first edge succeed; next edge is cancelled.
	ctx := &cancellingContext{Context: context.Background(), checks: 3}
	b := &Budget{Context: ctx, Left: 20}
	got, err := Search(nodes, edges, "a", "d", b)
	if err != nil || got.Status != "INCOMPLETE" || got.Reason != "CANCELLED" || b.Left != 18 || !reflect.DeepEqual(got.Path, emptyPath()) {
		t.Fatal("mid-search cancellation accounting", got, b, err)
	}
}
func TestExactWorkAndWitness(t *testing.T) {
	nodes, edges := diamond()
	for _, tc := range []struct {
		end, status string
		cost        int
		path        Path
	}{
		{"a", "FOUND", 1, Path{[]string{"a"}, []string{}, [][]string{}}},
		{"b", "FOUND", 5, Path{[]string{"a", "b"}, []string{"ab"}, [][]string{{}}}},
		{"d", "FOUND", 9, Path{[]string{"a", "b", "d"}, []string{"ab", "bd"}, [][]string{{}, {"bd-1", "bd-2"}}}},
		{"isolate", "NOT_FOUND_IN_RETAINED_GRAPH", 9, emptyPath()},
	} {
		for work := 0; work <= tc.cost+1; work++ {
			b := &Budget{Context: context.Background(), Left: work}
			got, err := Search(nodes, edges, "a", tc.end, b)
			if err != nil {
				t.Fatal(err)
			}
			if work < tc.cost {
				if got.Status != "INCOMPLETE" || got.Reason != "LIMIT" || b.Left != 0 || !reflect.DeepEqual(got.Path, emptyPath()) {
					t.Fatalf("work=%d: %+v %+v", work, got, b)
				}
			} else {
				if got.Status != tc.status || got.Reason != "" || b.Left != work-tc.cost || !reflect.DeepEqual(got.Path, tc.path) {
					t.Fatalf("end=%s work=%d: %+v %+v", tc.end, work, got, b)
				}
				if err := Prove(edges, "a", tc.end, got.Status, got.Path); err != nil {
					t.Fatal(err)
				}
			}
		}
	}
}
func TestMissingEqualAndSharedBudget(t *testing.T) {
	nodes, edges := diamond()
	b := &Budget{Context: context.Background(), Left: 2}
	for _, pair := range [][2]string{{"missing", "a"}, {"a", "missing"}, {"missing", "missing"}} {
		if _, err := Search(nodes, edges, pair[0], pair[1], b); err == nil {
			t.Fatal("missing endpoint accepted", pair)
		}
		if b.Left != 2 {
			t.Fatal("admission consumed work")
		}
	}
	for i := 0; i < 3; i++ {
		got, err := Search(nodes, edges, "a", "a", b)
		if err != nil {
			t.Fatal(err)
		}
		want := "FOUND"
		if i == 2 {
			want = "INCOMPLETE"
		}
		if got.Status != want {
			t.Fatal(got)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	b = &Budget{Context: ctx, Left: 0}
	got, err := Search(nodes, edges, "a", "a", b)
	if err != nil || got.Reason != "CANCELLED" || b.Reason != "CANCELLED" || b.Left != 0 {
		t.Fatal("cancellation must precede work exhaustion", got, b, err)
	}
}
func TestIndexesAndIndependentProofRejects(t *testing.T) {
	nodes, edges := diamond()
	before := append([]Edge{}, edges...)
	out, in := Indexes(edges)
	if !reflect.DeepEqual(before, edges) || out["a"][0].GroupID != "ab" || out["a"][1].GroupID != "z" || out["a"][2].GroupID != "ac" || in["d"][0].GroupID != "bd" {
		t.Fatal("index ordering or mutation", out, in)
	}
	good, err := Search(nodes, edges, "a", "d", &Budget{Context: context.Background(), Left: 100})
	if err != nil {
		t.Fatal(err)
	}
	for name, bad := range map[string]Path{
		"wrong-tie":        {[]string{"a", "c", "d"}, []string{"ac", "cd"}, [][]string{{"ac-site"}, {}}},
		"parallel-tie":     {[]string{"a", "b", "d"}, []string{"z", "bd"}, [][]string{{"z-site"}, {"bd-1", "bd-2"}}},
		"lost-middle":      {[]string{"a", "d"}, []string{"ab", "bd"}, [][]string{{}, {"bd-1", "bd-2"}}},
		"substituted-node": {[]string{"a", "c", "d"}, good.Path.GroupIDs, good.Path.OccurrenceIDs},
		"lost-witness":     {good.Path.Nodes, good.Path.GroupIDs, [][]string{{}, {"bd-1"}}},
		"nil-not-empty":    {good.Path.Nodes, good.Path.GroupIDs, [][]string{nil, {"bd-1", "bd-2"}}},
		"reversed":         {[]string{"d", "b", "a"}, []string{"bd", "ab"}, [][]string{{"bd-1", "bd-2"}, {}}},
	} {
		if err := Prove(edges, "a", "d", "FOUND", bad); err == nil {
			t.Fatal("proof accepted", name)
		}
	}
	if err := Prove(edges, "a", "d", "NOT_FOUND_IN_RETAINED_GRAPH", emptyPath()); err == nil {
		t.Fatal("false not-found")
	}
	if err := Prove(edges, "d", "a", "FOUND", good.Path); err == nil {
		t.Fatal("wrong direction")
	}
}
