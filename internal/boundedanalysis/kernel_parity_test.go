package boundedanalysis

import (
	"context"
	"reflect"
	"testing"

	"lsp-trace/internal/retainedpath"
)

func TestSharedPathKernelParity(t *testing.T) {
	edges := []Edge{
		{GroupID: "z", Caller: "a", Callee: "b", Weight: 1, OccurrenceIDs: []string{"z"}},
		{GroupID: "ab", Caller: "a", Callee: "b", Weight: 1, OccurrenceIDs: []string{}},
		{GroupID: "ac", Caller: "a", Callee: "c", Weight: 1, OccurrenceIDs: []string{"ac"}},
		{GroupID: "bd", Caller: "b", Callee: "d", Weight: 1, OccurrenceIDs: []string{"one", "two"}},
		{GroupID: "cd", Caller: "c", Callee: "d", Weight: 1, OccurrenceIDs: []string{}},
	}
	neutral := make([]retainedpath.Edge, len(edges))
	for i, e := range edges {
		neutral[i] = retainedpath.Edge(e)
	}
	for _, end := range []string{"a", "b", "d", "isolate"} {
		for work := 1; work <= 20; work++ {
			for _, cancelled := range []bool{false, true} {
				ctx, cancel := context.WithCancel(context.Background())
				if cancelled {
					cancel()
				}
				p := Parameters{Operation: "PATH", Start: "a", End: end, MaxWork: work}
				e := topology([]string{"a", "b", "c", "d", "isolate"}, edges, p)
				run(ctx, &e)
				b := &retainedpath.Budget{Context: ctx, Left: work}
				got, err := retainedpath.Search(e.Nodes, neutral, "a", end, b)
				cancel()
				if err != nil {
					t.Fatal(err)
				}
				if got.Status != e.Status || got.Reason != e.Reason || !reflect.DeepEqual(Path(got.Path), e.Path) {
					t.Fatalf("ASSERT_SHARED_PATH_PARITY end=%s work=%d cancelled=%v got=%+v want=%+v", end, work, cancelled, got, e)
				}
			}
		}
	}
}
