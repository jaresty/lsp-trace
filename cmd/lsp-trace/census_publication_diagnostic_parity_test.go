package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"testing"

	"lsp-trace/internal/captureset"
	"lsp-trace/internal/censusresult"
	"lsp-trace/internal/publication"
)

type failingCensusCaptureSetPublisher struct {
	result captureset.PublicationResult
}

func (p failingCensusCaptureSetPublisher) PublishCaptureSet(captureset.Manifest, [][]byte, captureset.ExactBytesAuthority) captureset.PublicationResult {
	return p.result
}
func (failingCensusCaptureSetPublisher) Verify(string, captureset.ExactBytesAuthority) (captureset.Manifest, error) {
	return captureset.Manifest{}, errors.New("unexpected verify")
}
func (failingCensusCaptureSetPublisher) ResolveConstituent(string, string, captureset.ExactBytesAuthority) ([]byte, error) {
	return nil, errors.New("unexpected resolve")
}

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

func TestCensusCaptureSetPublicationPreservesTypedPreCommitFailure(t *testing.T) {
	old := newCensusCaptureSetPublisher
	t.Cleanup(func() { newCensusCaptureSetPublisher = old })
	observed, limit := uint64(70<<20), uint64(64<<20)
	failure := &captureset.PublicationFailure{Stage: "ENCODE", Reason: "BUNDLE_ENCODE", CandidateByteLength: observed, CodecLimit: limit, Err: errors.New("private provider prose must not escape")}
	newCensusCaptureSetPublisher = func(*publication.Root) censusCaptureSetPublisher {
		return failingCensusCaptureSetPublisher{result: captureset.PublicationResult{Failure: failure, Err: failure}}
	}
	root := t.TempDir()
	if err := os.Chmod(root, 0o700); err != nil {
		t.Fatal(err)
	}
	outcome := publishCensusCaptureSet(context.Background(), censusPublicationCapabilityFor(t, censusPublicationProjection(t, 1)), root)
	if outcome.Result != nil || outcome.Diagnostic == nil || outcome.Diagnostic.PublicationFailure != censusresult.PublicationFailureEncode || outcome.Diagnostic.PublicationAccounting == nil || outcome.Diagnostic.PublicationAccounting.Observed == nil || *outcome.Diagnostic.PublicationAccounting.Observed != observed || outcome.Diagnostic.PublicationAccounting.Limit == nil || *outcome.Diagnostic.PublicationAccounting.Limit != limit {
		t.Fatalf("ASSERT_TYPED_PUBLICATION_FAILURE_PRESERVED: %+v", outcome)
	}
	raw, err := marshalCensusCLIDiagnostic(*outcome.Diagnostic)
	if err != nil || bytes.Contains(raw, []byte("private provider prose")) || !bytes.Contains(raw, []byte(`"publication_failure":"ENCODE"`)) || !bytes.Contains(raw, []byte(`"observed":73400320,"limit":67108864`)) {
		t.Fatalf("ASSERT_TYPED_PUBLICATION_FAILURE_PRIVATE_CANONICAL: raw=%s err=%v", raw, err)
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 0 {
		t.Fatalf("ASSERT_TYPED_PUBLICATION_FAILURE_ZERO_COMMIT: entries=%v err=%v", entries, err)
	}
}
