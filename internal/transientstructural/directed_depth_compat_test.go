package transientstructural

import (
	"reflect"
	"testing"

	"lsp-trace/internal/graph"
)

func TestCallsDirectedDepthsRemainStableAcrossSharedKernel(t *testing.T) {
	edges := []graph.Edge{
		{CallerNodeID: "A", CalleeNodeID: "B"},
		{CallerNodeID: "B", CalleeNodeID: "C"},
		{CallerNodeID: "C", CalleeNodeID: "A"},
		{CallerNodeID: "B", CalleeNodeID: "C"},
	}
	original := append([]graph.Edge(nil), edges...)
	for _, tc := range []struct {
		name    string
		limit   int
		reverse bool
		want    map[string]int
	}{
		{"forward zero", 0, false, map[string]int{"A": 0}},
		{"forward two", 2, false, map[string]int{"A": 0, "B": 1, "C": 2}},
		{"reverse zero", 0, true, map[string]int{"A": 0}},
		{"reverse two", 2, true, map[string]int{"A": 0, "C": 1, "B": 2}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := directedDepths("A", edges, tc.limit, tc.reverse)
			if !reflect.DeepEqual(got, tc.want) || !reflect.DeepEqual(edges, original) {
				t.Fatalf("ASSERT_CALLS_DIRECTED_DEPTH_PARITY: got=%v want=%v edges=%v", got, tc.want, edges)
			}
			t.Log("ASSERT_CALLS_DIRECTED_DEPTH_PARITY: PASS")
		})
	}
}
