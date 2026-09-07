package boundedanalysis

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// These complete canonical analytical artifacts were captured before extraction.
// Updating them is deliberately not supported by the regression test.
func TestPathKernelHistoricalBytes(t *testing.T) {
	raw, ids := retainedFixture(t)
	cases := []struct {
		name       string
		start, end int
		work       int
		cancel     bool
	}{
		{"found", 0, 2, MaxWork, false},
		{"not-found", 0, 4, MaxWork, false},
		{"equal", 0, 0, 1, false},
		{"unreported", 1, 2, MaxWork, false},
		{"work-boundary", 0, 2, 1, false},
		{"cancel", 0, 0, MaxWork, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			if tc.cancel {
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			}
			got, err := Analyze(ctx, raw, Parameters{Operation: "PATH", Start: ids[tc.start], End: ids[tc.end], MaxWork: tc.work})
			if err != nil {
				t.Fatal(err)
			}
			if _, err = ValidateFor(got, Family, "v1"); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join("testdata", "path-kernel-"+tc.name+".json")
			want, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(want, got) {
				t.Fatal("ASSERT_HISTORICAL_CANONICAL_PATH_BYTES", tc.name)
			}
		})
	}
	// A diamond and parallel groups exercise the exact next-node/group tie policy.
	e := topology([]string{"a", "b", "c", "d"}, []Edge{
		{GroupID: "z", Caller: "a", Callee: "b", Weight: 1, OccurrenceIDs: []string{"z-site"}},
		{GroupID: "ab", Caller: "a", Callee: "b", Weight: 1, OccurrenceIDs: []string{}},
		{GroupID: "ac", Caller: "a", Callee: "c", Weight: 1, OccurrenceIDs: []string{"ac-site"}},
		{GroupID: "bd", Caller: "b", Callee: "d", Weight: 1, OccurrenceIDs: []string{"bd-1", "bd-2"}},
		{GroupID: "cd", Caller: "c", Callee: "d", Weight: 1, OccurrenceIDs: []string{}},
	}, Parameters{Operation: "PATH", Start: "a", End: "d", MaxWork: MaxWork})
	run(context.Background(), &e)
	got, _ := json.Marshal(e)
	path := filepath.Join("testdata", "path-kernel-diamond.json")
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(want, got) {
		t.Fatal("ASSERT_HISTORICAL_DIAMOND_BYTES")
	}
}
