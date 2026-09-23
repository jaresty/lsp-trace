package directedwalk_test

import (
	"reflect"
	"testing"

	"lsp-trace/internal/directedwalk"
)

type typedOccurrence struct {
	ID, Kind, From, To string
}

func TestMixedKindsShareDirectedDepthsWithoutRetyping(t *testing.T) {
	occurrences := []typedOccurrence{
		{"call-1", "CALLS", "A", "B"},
		{"definition-1", "RESOLVES_TO_DEFINITION", "B", "C"},
		{"reference-1", "REFERENCES_SYMBOL", "C", "D"},
		{"reference-2", "REFERENCES_SYMBOL", "C", "D"},
	}
	original := append([]typedOccurrence(nil), occurrences...)
	edges := make([]directedwalk.Edge, len(occurrences))
	for i, occurrence := range occurrences {
		edges[i] = directedwalk.Edge{From: occurrence.From, To: occurrence.To}
	}
	for _, tc := range []struct {
		name, root string
		depth      int
		reverse    bool
		want       map[string]int
	}{
		{"outgoing zero", "A", 0, false, map[string]int{"A": 0}},
		{"outgoing two", "A", 2, false, map[string]int{"A": 0, "B": 1, "C": 2}},
		{"outgoing three", "A", 3, false, map[string]int{"A": 0, "B": 1, "C": 2, "D": 3}},
		{"incoming zero", "D", 0, true, map[string]int{"D": 0}},
		{"incoming two", "D", 2, true, map[string]int{"D": 0, "C": 1, "B": 2}},
		{"incoming three", "D", 3, true, map[string]int{"D": 0, "C": 1, "B": 2, "A": 3}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := directedwalk.Depths(tc.root, edges, tc.depth, tc.reverse)
			if !reflect.DeepEqual(got, tc.want) || !reflect.DeepEqual(occurrences, original) || len(edges) != len(occurrences) {
				t.Fatalf("ASSERT_TYPED_SHARED_DEPTH: got=%v want=%v occurrences=%v", got, tc.want, occurrences)
			}
			t.Log("ASSERT_TYPED_SHARED_DEPTH: PASS")
		})
	}
}

func TestDirectedDepthsRetainShortestCyclePaths(t *testing.T) {
	edges := []directedwalk.Edge{{From: "A", To: "B"}, {From: "B", To: "C"}, {From: "C", To: "A"}, {From: "A", To: "B"}}
	for _, tc := range []struct {
		reverse bool
		want    map[string]int
	}{
		{false, map[string]int{"A": 0, "B": 1, "C": 2}},
		{true, map[string]int{"A": 0, "C": 1, "B": 2}},
	} {
		got := directedwalk.Depths("A", edges, 4, tc.reverse)
		if !reflect.DeepEqual(got, tc.want) {
			t.Fatalf("ASSERT_TYPED_SHARED_CYCLE: reverse=%t got=%v want=%v", tc.reverse, got, tc.want)
		}
		t.Log("ASSERT_TYPED_SHARED_CYCLE: PASS")
	}
}
