package acquisitionops

import (
	"bytes"
	"testing"

	"lsp-trace/internal/acquisition"
)

func TestExplicitTraceAdmissionCanonicalExactAndUnforgeable(t *testing.T) {
	line, character := uint32(0), uint32(2)
	input := Input{SessionID: "s", Generation: 7, OutputVersion: "lsp-trace.graph-provenance.v5", SeedManifest: Manifest{SchemaVersion: ManifestVersion, CoordinateConvention: "zero-based-session", Root: Target{ID: "root", Locator: acquisition.Locator{URI: "file:///w/a.go", Line: &line, Character: &character}}, RequiredTargets: []Target{}}}
	canonical := []byte(`{"schema_version":"lsp-trace.seeds.v2","coordinate_convention":"one-based","defaults":{},"seeds":[{"type":"position","label":"root","path":"a.go","line":1,"column":3}]}`)
	admission, err := NewExplicitTraceAdmission(canonical, "/w", "request-a", input)
	if err != nil {
		t.Fatal(err)
	}
	got, err := admission.admit("request-a", "/w", input)
	if err != nil || !bytes.Equal(got, canonical) {
		t.Fatalf("ASSERT_EXPLICIT_TRACE_CANONICAL_ADMISSION: %v", err)
	}
	if _, err := (ExplicitTraceAdmission{}).admit("request-a", "/w", input); err == nil {
		t.Fatal("ASSERT_EXPLICIT_TRACE_STRUCT_LITERAL_FORGERY_REJECTED")
	}
	for name, bad := range map[string][]byte{
		"noncanonical": append([]byte(" "), canonical...),
		"position":     bytes.Replace(canonical, []byte(`"column":3`), []byte(`"column":4`), 1),
		"label":        bytes.Replace(canonical, []byte(`"root"`), []byte(`"other"`), 1),
		"schema":       bytes.Replace(canonical, []byte(`lsp-trace.seeds.v2`), []byte(`lsp-trace.seeds.v1`), 1),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := NewExplicitTraceAdmission(bad, "/w", "request-a", input); err == nil {
				t.Fatal("ASSERT_EXPLICIT_TRACE_MUTATION_REJECTED")
			}
		})
	}
	crossed := input
	crossed.Generation++
	if _, err := admission.admit("request-a", "/w", crossed); err == nil {
		t.Fatal("ASSERT_EXPLICIT_TRACE_CROSS_OPERATION_REJECTED")
	}
}
