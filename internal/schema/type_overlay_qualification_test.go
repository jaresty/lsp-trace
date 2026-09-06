package schema

import "testing"

func TestAnalysisOnlyTypeOverlayQualificationSchemaIsRegistered(t *testing.T) {
	document := []byte(`{
		"schema_version":"lsp-trace.analysis-only-type-overlay-qualification.v1",
		"authority":"SOURCE_CONSTRAINED_ANALYSIS_INTERVENTION",
		"purpose":"QUALIFICATION_ONLY",
		"automatic_continuation":false,
		"admission_mutated":false,
		"inventory_mutated":false,
		"pinned_source":{"commit":"326718ae733cb26097bd30246276cecd371a4e79","immutable":true},
		"stages":[{"name":"BASELINE","commit":"326718ae733cb26097bd30246276cecd371a4e79","observation":{}}],
		"comparisons":[]
	}`)
	version, err := ValidateFor(document, FamilyTypeOverlayQualification, "v1")
	if err != nil {
		t.Fatal(err)
	}
	if version != TypeOverlayQualificationVersionV1 {
		t.Fatalf("version = %q", version)
	}
}
