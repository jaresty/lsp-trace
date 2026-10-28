package graphprovenance

import (
	"lsp-trace/internal/manageddiagnostic"
	"strings"
	"testing"
)

// A genuine existing native V5 fixture does not make a private source descriptor
// ordinary provenance V2. Inspection qualification must not alter this gate.
func TestRetainedPartialOrdinaryV5SourceBoundaryPrivate(t *testing.T) {
	_, native := v5Fixture(t)
	query := manageddiagnostic.QueryResult{Status: manageddiagnostic.QueryUnavailable, Records: []manageddiagnostic.Record{}}
	source := &EvidenceV2{SchemaVersion: VersionSourceWithholdingPrivateV1, Policy: PolicySourceWithholdingPrivateV1}
	seed := []byte(`{"schema_version":"lsp-trace.seeds.v2","coordinate_convention":"one-based","seeds":[{"type":"position","label":"seed","path":"a.go","line":1,"column":1}]}`)
	if _, e := CaptureV5WithSeedSpec(native, "session", 1, query, seed, source); e == nil || !strings.Contains(e.Error(), "source snapshot must be admitted provenance v2") {
		t.Fatal("ASSERT_PRIVATE_SOURCE_CANNOT_WRAP_NATIVE_V5", e)
	}
	t.Log("ASSERT_PRIVATE_SOURCE_CANNOT_WRAP_NATIVE_V5: PASS")
}
