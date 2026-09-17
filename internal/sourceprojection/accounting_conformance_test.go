package sourceprojection

import "testing"

func TestCommonProjectionAccountingExactBoundaries(t *testing.T) {
	const assertion = "ASSERT_C03_ALL_LIMITS_EXACTLY_RECONCILE"
	tests := []struct {
		name       string
		policy     Policy
		wantStatus string
		selected   int
		omitted    int
		cause      string
		wantError  bool
	}{
		{name: "all-exact", policy: Policy{PolicyID: "public", BodyRequested: true, MaxBytes: 42, MaxRanges: 2, MaxObjects: 2, MaxWork: 2, EnforceLimits: true}, wantStatus: "COMPLETE", selected: 2},
		{name: "bytes-minus-one", policy: Policy{PolicyID: "public", BodyRequested: true, MaxBytes: 41, MaxRanges: 2, MaxObjects: 2, MaxWork: 2, EnforceLimits: true}, wantStatus: "TRUNCATED", omitted: 2, cause: "BYTE_LIMIT"},
		{name: "ranges-minus-one", policy: Policy{PolicyID: "public", BodyRequested: true, MaxBytes: 42, MaxRanges: 1, MaxObjects: 2, MaxWork: 2, EnforceLimits: true}, wantStatus: "TRUNCATED", selected: 1, omitted: 1, cause: "RANGE_LIMIT"},
		{name: "objects-minus-one", policy: Policy{PolicyID: "public", BodyRequested: true, MaxBytes: 42, MaxRanges: 2, MaxObjects: 1, MaxWork: 2, EnforceLimits: true}, wantStatus: "TRUNCATED", selected: 1, omitted: 1, cause: "OBJECT_LIMIT"},
		{name: "work-minus-one", policy: Policy{PolicyID: "public", BodyRequested: true, MaxBytes: 42, MaxRanges: 2, MaxObjects: 2, MaxWork: 1, EnforceLimits: true}, wantError: true},
		{name: "zero-objects", policy: Policy{PolicyID: "public", BodyRequested: true, MaxBytes: 42, MaxRanges: 2, MaxObjects: 0, MaxWork: 2, EnforceLimits: true}, wantStatus: "TRUNCATED", omitted: 2, cause: "OBJECT_LIMIT"},
		{name: "zero-ranges", policy: Policy{PolicyID: "public", BodyRequested: true, MaxBytes: 42, MaxRanges: 0, MaxObjects: 2, MaxWork: 2, EnforceLimits: true}, wantStatus: "TRUNCATED", omitted: 2, cause: "RANGE_LIMIT"},
		{name: "zero-bytes", policy: Policy{PolicyID: "public", BodyRequested: true, MaxBytes: 0, MaxRanges: 2, MaxObjects: 2, MaxWork: 2, EnforceLimits: true}, wantStatus: "TRUNCATED", omitted: 2, cause: "BYTE_LIMIT"},
		{name: "zero-work", policy: Policy{PolicyID: "public", BodyRequested: true, MaxBytes: 42, MaxRanges: 2, MaxObjects: 2, MaxWork: 0, EnforceLimits: true}, wantError: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Project(fixtureCandidates(), fixtureSources(true), tc.policy)
			if tc.wantError {
				if err == nil || got.Status != "" || len(got.Units) != 0 || len(got.Omissions) != 0 {
					t.Fatalf("%s_FAIL_CLOSED: result=%+v err=%v", assertion, got, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("%s: %v", assertion, err)
			}
			if got.Status != tc.wantStatus || got.Accounting.Candidates != got.Accounting.Selected+got.Accounting.Omitted || got.Accounting.Selected != tc.selected || got.Accounting.Omitted != tc.omitted || got.Accounting.Evaluated != got.Accounting.Terminal+got.Accounting.Unevaluated {
				t.Fatalf("%s: result=%+v", assertion, got)
			}
			seen := map[string]string{}
			for _, omission := range got.Omissions {
				if previous, duplicate := seen[omission.UnitID]; duplicate {
					t.Fatalf("%s_MUTUALLY_EXCLUSIVE: unit=%s causes=%s,%s", assertion, omission.UnitID, previous, omission.Cause)
				}
				seen[omission.UnitID] = omission.Cause
				if tc.cause != "" && omission.Cause != tc.cause {
					t.Fatalf("%s_CAUSE: got=%s want=%s", assertion, omission.Cause, tc.cause)
				}
			}
			if tc.name == "bytes-minus-one" && (got.Accounting.LogicalSelectedBytes != 0 || got.Accounting.UniqueEmittedBytes != 0 || len(got.EmittedSpans) != 0) {
				t.Fatalf("%s_WHOLE_RANGES: %+v", assertion, got)
			}
		})
	}
}
