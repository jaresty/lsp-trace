package main

import (
	"bytes"
	"testing"

	"lsp-trace/internal/censusresult"
)

func TestCLIPublicationDiagnosticMatchesCanonicalAdditiveProjection(t *testing.T) {
	observed, limit := uint64(17), uint64(64<<20)
	d, _ := buildCensusCLIDiagnostic(censusresult.StagePublication, nil)
	d.PublicationFailure = censusresult.PublicationFailureTempWrite
	d.PublicationAccounting = &censusresult.PublicationAccounting{Category: censusresult.PublicationAccountingCandidateBytes, Observed: &observed, Limit: &limit}
	got, err := marshalCensusCLIDiagnostic(d)
	want, wantErr := censusresult.MarshalDiagnostic(d)
	if err != nil || wantErr != nil || !bytes.Equal(got, want) || !bytes.Contains(got, []byte(`"publication_accounting":{"category":"CANDIDATE_BYTES","observed":17,"limit":67108864}`)) {
		t.Fatalf("ASSERT_PUBLICATION_DIAGNOSTIC_CLI_PARITY: got=%s want=%s err=%v wantErr=%v", got, want, err, wantErr)
	}
}
