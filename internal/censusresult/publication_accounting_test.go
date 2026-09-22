package censusresult

import (
	"bytes"
	"testing"
)

func TestPublicationDiagnosticSafeAccountingAndHistoricalBytes(t *testing.T) {
	historical := Diagnostic{SchemaVersion: DiagnosticSchemaVersion, Status: "FAILED", Stage: StagePublication, Code: CodePublicationFailed, Retry: true, PublicationFailure: PublicationFailureInternal}
	raw, err := MarshalDiagnostic(historical)
	want := []byte("{\"schema_version\":\"lsp-trace.census-diagnostic.v1\",\"status\":\"FAILED\",\"stage\":\"publication\",\"code\":\"PUBLICATION_FAILED\",\"retry\":true,\"publication_failure\":\"INTERNAL\"}\n")
	if err != nil || !bytes.Equal(raw, want) {
		t.Fatalf("ASSERT_CENSUS_DIAGNOSTIC_V1_BYTES_STABLE: %q err=%v", raw, err)
	}
	observed, limit := uint64(17), uint64(64<<20)
	d := historical
	d.PublicationAccounting = &PublicationAccounting{Category: PublicationAccountingCandidateBytes, Observed: &observed, Limit: &limit}
	raw, err = MarshalDiagnostic(d)
	if err != nil || !bytes.Contains(raw, []byte(`"publication_accounting":{"category":"CANDIDATE_BYTES","observed":17,"limit":67108864}`)) {
		t.Fatalf("ASSERT_PUBLICATION_SAFE_KNOWN_ACCOUNTING: %q err=%v", raw, err)
	}
	unknown := historical
	unknown.PublicationAccounting = &PublicationAccounting{Category: PublicationAccountingUnknown}
	raw, err = MarshalDiagnostic(unknown)
	if err != nil || bytes.Contains(raw, []byte("observed")) || bytes.Contains(raw, []byte("limit")) {
		t.Fatalf("ASSERT_PUBLICATION_UNKNOWN_OMITS_NUMERICS: %q err=%v", raw, err)
	}
	tooLarge := uint64(MaxPublicationDiagnosticCount + 1)
	d.PublicationAccounting.Observed = &tooLarge
	if _, err := MarshalDiagnostic(d); err == nil {
		t.Fatal("ASSERT_PUBLICATION_ACCOUNTING_CAP")
	}
}

func TestPublicationDiagnosticStrictJSONRejection(t *testing.T) {
	for _, raw := range [][]byte{
		[]byte(`{"schema_version":"lsp-trace.census-diagnostic.v1","schema_version":"lsp-trace.census-diagnostic.v1"}`),
		[]byte(`{"schema_version":"lsp-trace.census-diagnostic.v1","unknown":1}`),
		[]byte(`{"schema_version":"lsp-trace.census-diagnostic.v1"} trailing`),
	} {
		if _, err := DecodeDiagnostic(raw); err == nil {
			t.Fatalf("ASSERT_PUBLICATION_DIAGNOSTIC_STRICT_REJECTION: %q", raw)
		}
	}
}
