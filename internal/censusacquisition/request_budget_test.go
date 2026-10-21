package censusacquisition

import (
	"math"
	"testing"
)

func TestAcquisitionMaxRequests(t *testing.T) {
	tests := []struct {
		name                 string
		private              bool
		maxNodes, maxTargets int
		want                 int
		wantErr              bool
	}{
		{name: "ordinary-preserves-legacy-with-zero-plan", maxNodes: 0, maxTargets: 0, want: 1000},
		{name: "private-one", private: true, maxNodes: 1, maxTargets: 1, want: 69},
		{name: "private-sixty-three", private: true, maxNodes: 63, maxTargets: 63, want: 317},
		{name: "private-sixty-four-nodes", private: true, maxNodes: 64, maxTargets: 63, want: 319},
		{name: "private-ten-thousand", private: true, maxNodes: 10000, maxTargets: 63, want: 20191},
		{name: "zero-nodes", private: true, maxNodes: 0, maxTargets: 1, wantErr: true},
		{name: "zero-targets", private: true, maxNodes: 1, maxTargets: 0, wantErr: true},
		{name: "batch-over-ceiling", private: true, maxNodes: 1, maxTargets: 64, wantErr: true},
		{name: "node-over-ceiling", private: true, maxNodes: 10001, maxTargets: 1, wantErr: true},
		{name: "global-symbol-limit-is-not-a-core-node-limit", private: true, maxNodes: 100000, maxTargets: 63, wantErr: true},
		{name: "overflowing-node-input", private: true, maxNodes: math.MaxInt, maxTargets: 1, wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := AcquisitionMaxRequests(tc.private, tc.maxNodes, tc.maxTargets)
			if (err != nil) != tc.wantErr || got != tc.want {
				t.Fatalf("ASSERT_CENSUS_ACQUISITION_REQUEST_BUDGET_BOUNDARY: got=%d err=%v want=%d want_err=%t", got, err, tc.want, tc.wantErr)
			}
		})
	}
}
