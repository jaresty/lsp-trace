package scopejob

import (
	"context"
	"testing"
)

func TestEnumerationSharedReplayDepth(t *testing.T) {
	for _, tc := range []struct {
		name  string
		depth int
		roots []string
		parts []segment
		valid bool
	}{
		{"original-counterexample", 1, []string{"."}, []segment{{Records: []Record{directory("."), directory("a")}, Done: true}}, false},
		{"exact-boundary", 2, []string{"."}, []segment{{Records: []Record{directory("."), directory("a")}, Done: true}}, true},
		{"cross-segment-overflow", 1, []string{"."}, []segment{{Records: []Record{directory(".")}, Frontier: []string{"."}}, {Records: []Record{directory("a")}, Done: true}}, false},
		{"siblings", 1, []string{"a", "b", "c"}, []segment{{Records: []Record{directory("a"), directory("b"), directory("c")}, Done: true}}, true},
		{"sibling-segments", 1, []string{"a", "b"}, []segment{{Records: []Record{directory("a")}, Frontier: []string{"b"}}, {Records: []Record{directory("b")}, Done: true}}, true},
		{"invalid-frontier", 2, []string{"."}, []segment{{Records: []Record{directory(".")}, Frontier: []string{".", "invented"}}}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, _, _ := fixture(t)
			c.Budget.Depth = tc.depth
			c.Roots = tc.roots
			old := progress{}
			fresh := EnumerationReplay{}
			oldOK, newOK := true, true
			for _, s := range tc.parts {
				oldOK = oldOK && old.segment(c, s, 128)
				newOK = newOK && fresh.Apply(context.Background(), c, EnumerationBatch{Records: s.Records, Frontier: s.Frontier, Done: s.Done, Reason: s.Reason}, 128) == nil
			}
			if oldOK != tc.valid || newOK != oldOK {
				t.Fatalf("legacy=%v inventory=%v want=%v", oldOK, newOK, tc.valid)
			}
		})
	}
}
