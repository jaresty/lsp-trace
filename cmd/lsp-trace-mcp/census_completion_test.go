package main

import (
	"context"
	"errors"
	"strings"
	"testing"

	"lsp-trace/internal/captureset"
	"lsp-trace/internal/censusacquisition"
	"lsp-trace/internal/censusresult"
	"lsp-trace/internal/publication"
)

type censusVerifierStub struct {
	manifest   captureset.Manifest
	verifyErr  error
	resolveErr error
	verifies   int
	resolves   int
}

func (v *censusVerifierStub) Verify(string, captureset.ExactBytesAuthority) (captureset.Manifest, error) {
	v.verifies++
	return v.manifest, v.verifyErr
}
func (v *censusVerifierStub) ResolveConstituent(string, string, captureset.ExactBytesAuthority) ([]byte, error) {
	v.resolves++
	return []byte("exact"), v.resolveErr
}

func validMCPCompletionProjection() censusacquisition.Projection {
	return censusacquisition.Projection{
		Session:  censusacquisition.SessionIdentity{SessionID: "session", Generation: 1},
		CensusID: "census",
		Manifest: captureset.Manifest{
			LogicalDigest: "capture-set",
			FileLedger:    captureset.Ledger{Denominator: 1, Entries: []captureset.LedgerEntry{{Ordinal: 0, Identity: "private/file.go", Disposition: censusacquisition.FileProcessed}}},
			SymbolLedger:  captureset.Ledger{Denominator: 1, Entries: []captureset.LedgerEntry{{Ordinal: 0, Identity: "private.symbol", Disposition: censusacquisition.SymbolPrepared}}},
			Targets:       []captureset.Target{{CensusOrdinal: 0}}, Batches: []captureset.Batch{{Ordinal: 0, TargetCount: 1}},
		},
		Constituents: []censusacquisition.Constituent{{Raw: []byte("exact")}},
	}
}

func completionPublisher(receipt captureset.PublicationReceipt, after func()) censusProjectionPublisher {
	return censusProjectionPublisherFunc(func(captureset.Manifest, [][]byte, captureset.ExactBytesAuthority) captureset.PublicationResult {
		if after != nil {
			after()
		}
		return captureset.PublicationResult{Receipt: &receipt}
	})
}

func validMCPCompletionReceipt() captureset.PublicationReceipt {
	return captureset.PublicationReceipt{Selector: "capture-sets/v1/sha256/a.bundle", ArtifactSHA256: "sha256:" + strings.Repeat("a", 64), ByteLength: 12, VerificationStatus: "VERIFIED", DirectorySyncStatus: publication.DirectorySyncComplete, CloseStatus: publication.CloseComplete}
}

func TestCompleteCensusProjectionVerifiedSuccess(t *testing.T) {
	projection := validMCPCompletionProjection()
	verifier := &censusVerifierStub{manifest: captureset.Manifest{Constituents: []captureset.Constituent{{ImmutableSelector: "graphs/v5/a.json"}}}}
	got := completeCensusProjectionWith(context.Background(), projection, completionPublisher(validMCPCompletionReceipt(), nil), verifier)
	if got.Result == nil || got.Diagnostic != nil || verifier.verifies != 1 || verifier.resolves != 1 {
		t.Fatalf("ASSERT_MCP_CENSUS_COMPLETION_SUCCESS: completion=%+v verifies=%d resolves=%d", got, verifier.verifies, verifier.resolves)
	}
	if got.Result.Publication.DirectorySyncStatus != censusresult.DirectorySyncComplete || got.Result.Publication.CloseStatus != censusresult.CloseComplete {
		t.Fatalf("ASSERT_MCP_CENSUS_COMPLETION_NORMALIZED: %+v", got.Result.Publication)
	}
}

func TestCompleteCensusProjectionPostCommitDegradation(t *testing.T) {
	privateErr := errors.New("/private/workspace/secret.go")
	cases := []struct {
		name             string
		mutateProjection func(*censusacquisition.Projection)
		mutateReceipt    func(*captureset.PublicationReceipt)
		verifier         *censusVerifierStub
	}{
		{name: "verify", verifier: &censusVerifierStub{verifyErr: privateErr}},
		{name: "resolve", verifier: &censusVerifierStub{manifest: captureset.Manifest{Constituents: []captureset.Constituent{{ImmutableSelector: "graphs/v5/a.json"}}}, resolveErr: privateErr}},
		{name: "directory-sync", mutateReceipt: func(r *captureset.PublicationReceipt) { r.DirectorySyncStatus = publication.DirectorySyncFailed }, verifier: &censusVerifierStub{}},
		{name: "close", mutateReceipt: func(r *captureset.PublicationReceipt) { r.CloseStatus = publication.CloseFailed }, verifier: &censusVerifierStub{}},
		{name: "projection", mutateProjection: func(p *censusacquisition.Projection) { p.Session.Generation = 0 }, verifier: &censusVerifierStub{}},
		{name: "nil-verifier"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			projection, receipt := validMCPCompletionProjection(), validMCPCompletionReceipt()
			if tc.mutateProjection != nil {
				tc.mutateProjection(&projection)
			}
			if tc.mutateReceipt != nil {
				tc.mutateReceipt(&receipt)
			}
			var verifier censusProjectionVerifier
			if tc.verifier != nil {
				verifier = tc.verifier
			}
			got := completeCensusProjectionWith(context.Background(), projection, completionPublisher(receipt, nil), verifier)
			if got.Result != nil || got.Diagnostic == nil || got.Diagnostic.Stage != censusresult.StageCommitted || got.Diagnostic.Code != censusresult.CodeCommittedDegraded || got.Diagnostic.Retry {
				t.Fatalf("ASSERT_MCP_CENSUS_POSTCOMMIT_DEGRADATION: %+v", got)
			}
			raw, err := censusresult.MarshalDiagnostic(*got.Diagnostic)
			if err != nil || strings.Contains(string(raw), "private") || strings.Contains(string(raw), "secret") {
				t.Fatalf("ASSERT_MCP_CENSUS_DEGRADATION_PRIVACY: raw=%q err=%v", raw, err)
			}
		})
	}
}

func TestCompleteCensusProjectionDeadlineAfterCommitSkipsVerification(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	verifier := &censusVerifierStub{}
	got := completeCensusProjectionWith(ctx, validMCPCompletionProjection(), completionPublisher(validMCPCompletionReceipt(), cancel), verifier)
	if got.Result != nil || got.Diagnostic == nil || verifier.verifies != 0 || verifier.resolves != 0 {
		t.Fatalf("ASSERT_MCP_CENSUS_DEADLINE_GOVERNS_POSTCOMMIT_PHASES: completion=%+v verifies=%d resolves=%d", got, verifier.verifies, verifier.resolves)
	}
}
