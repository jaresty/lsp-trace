package programc

import (
	"fmt"
	"testing"
)

func TestPrivateMatchingGuardBoundaries(t *testing.T) {
	if !privateMatchingWorkFits(1000000, 1, 1000000) || privateMatchingWorkFits(1000001, 1, 1000000) || privateMatchingWorkFits(4, uint64(1)<<62, 1000000) {
		t.Fatal("division boundary")
	}
	for _, tc := range []struct {
		n, m  int
		limit uint64
		guard string
	}{{1, 0, 1000000, "MATCHING_EMPTY"}, {1, 1, 1000000, ""}, {4, 62, 1000000, "MATCHING_WORK_LIMIT"}, {1, 63, 1000000, "MATCHING_STATE_WIDTH"}, {15625, 6, 1000000, ""}, {15625, 6, 999999, "MATCHING_WORK_LIMIT"}, {1000000, 0, 1000000, "MATCHING_EMPTY"}, {1000001, 0, 1000000, "MATCHING_EMPTY"}} {
		g, _ := privateMatchingGuard(tc.n, tc.m, tc.limit)
		if g != tc.guard {
			t.Fatalf("n=%d m=%d limit=%d guard=%s want=%s", tc.n, tc.m, tc.limit, g, tc.guard)
		}
	}
}
func TestPrivateMatchingAsymmetric62(t *testing.T) {
	nodes := make([]string, 62)
	singles := make([]Community, 62)
	four := make([]Community, 4)
	for i := range nodes {
		nodes[i] = fmt.Sprintf("n%02d", i)
		singles[i].Members = []string{nodes[i]}
		four[i%4].Members = append(four[i%4].Members, nodes[i])
	}
	for _, p := range [][2][]Community{{four, singles}, {singles, four}} {
		m, err := ComparePartitionsMathPrivate(nodes, p[0], p[1])
		if err != nil || m.Outcome != "BLOCKED_MATCHING_RESOURCE" || m.Guard != "MATCHING_WORK_LIMIT" || m.Smaller != 4 || m.Larger != 62 || m.MatchedDenominator != 0 || m.WorkRepresentable || m.Work != 0 || m.States != uint64(1)<<62 {
			t.Fatalf("%+v %v", m, err)
		}
	}
}
