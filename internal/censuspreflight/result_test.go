package censuspreflight

import "testing"

func TestMarshalDiagnosticRejectsInvalidObservedLimitInvariants(t *testing.T) {
	for _, tc := range []struct {
		name string
		v    Diagnostic
	}{
		{"equal", Diagnostic{SchemaVersion: DiagnosticSchemaVersion, Category: "MAX_WORK", Observed: 200000, Limit: 200000}},
		{"swapped", Diagnostic{SchemaVersion: DiagnosticSchemaVersion, Category: "MAX_WORK", Observed: 200000, Limit: 200001}},
		{"skipped", Diagnostic{SchemaVersion: DiagnosticSchemaVersion, Category: "MAX_WORK", Observed: 200002, Limit: 200000}},
		{"bad_category", Diagnostic{SchemaVersion: DiagnosticSchemaVersion, Category: "PATH", Observed: 2, Limit: 1}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := MarshalDiagnostic(tc.v); err == nil {
				t.Fatal("ASSERT_DIAGNOSTIC_OBSERVED_LIMIT_REJECTED")
			}
		})
	}
}

func TestMarshalResultRequiresUnknownEpistemicCompleteness(t *testing.T) {
	v := Result{SchemaVersion: SchemaVersion, Terminal: "COMPLETE", Authority: 0, Completeness: "UNKNOWN"}
	if _, err := MarshalResult(v); err != nil {
		t.Fatalf("ASSERT_RESULT_UNKNOWN_ACCEPTED: %v", err)
	}
	v.Completeness = "COMPLETE"
	if _, err := MarshalResult(v); err == nil {
		t.Fatal("ASSERT_RESULT_COMPLETE_REJECTED")
	}
}
