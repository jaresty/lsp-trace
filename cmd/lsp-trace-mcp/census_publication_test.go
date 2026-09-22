package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"lsp-trace/internal/captureset"
	"lsp-trace/internal/censusacquisition"
	"lsp-trace/internal/censusresult"
	"lsp-trace/internal/operation"
	"lsp-trace/internal/publication"
)

type censusProjectionPublisherFunc func(captureset.Manifest, [][]byte, captureset.ExactBytesAuthority) captureset.PublicationResult

type fixedPublicationDiagnosticRunner struct{ completion censusCompletion }

func (r fixedPublicationDiagnosticRunner) run(context.Context, operation.Request) censusCompletion {
	return r.completion
}

func (f censusProjectionPublisherFunc) PublishCaptureSet(m captureset.Manifest, raw [][]byte, authority captureset.ExactBytesAuthority) captureset.PublicationResult {
	return f(m, raw, authority)
}

func TestPublishCensusProjectionCopiesBytesAndPreservesCommittedReceipt(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	projection := censusacquisition.Projection{Constituents: []censusacquisition.Constituent{{Raw: []byte("exact")}}}
	want := &captureset.PublicationReceipt{Selector: "capture-sets/v1/sha256/x.bundle", VerificationStatus: "COMMITTED_VERIFICATION_FAILED"}
	publisher := censusProjectionPublisherFunc(func(_ captureset.Manifest, raw [][]byte, authority captureset.ExactBytesAuthority) captureset.PublicationResult {
		if len(raw) != 1 || string(raw[0]) != "exact" || authority.AdmitGraphProvenanceV5 == nil {
			t.Fatalf("ASSERT_MCP_CENSUS_PUBLICATION_EXACT_INPUT: raw=%q", raw)
		}
		raw[0][0] = 'X'
		cancel()
		return captureset.PublicationResult{Receipt: want, Err: errors.New("ignored after receipt")}
	})
	got, failure, _ := publishCensusProjectionWith(ctx, projection, publisher)
	if failure != nil || got != want || string(projection.Constituents[0].Raw) != "exact" || ctx.Err() == nil {
		t.Fatalf("ASSERT_MCP_CENSUS_COMMITTED_RECEIPT_SURVIVES_DEGRADATION: receipt=%+v failure=%+v raw=%q err=%v", got, failure, projection.Constituents[0].Raw, ctx.Err())
	}
}

func TestBasePublicationFailureLedgerSinkValidatedAtStartup(t *testing.T) {
	parent := t.TempDir()
	if err := os.Chmod(parent, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(parent, "publication-failures.ndjson")
	t.Setenv(basePublicationFailureLedgerPathEnv, path)
	if err := validateBasePublicationFailureLedgerSink(); err != nil {
		t.Fatal(err)
	}
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm() != 0o600 {
		t.Fatalf("ASSERT_BASE_LEDGER_SINK_READY: info=%v err=%v", info, err)
	}
}

func TestBasePublicationFailureLedgerSinkUnavailableFailsBeforeServing(t *testing.T) {
	t.Setenv(basePublicationFailureLedgerPathEnv, filepath.Join(t.TempDir(), "missing", "publication-failures.ndjson"))
	if err := validateBasePublicationFailureLedgerSink(); err == nil {
		t.Fatal("ASSERT_BASE_LEDGER_SINK_UNAVAILABLE")
	}
}

func TestBasePublicationProductionConfigWithoutLSP(t *testing.T) {
	rootPath := os.Getenv("LSP_TRACE_BASE_PUBLICATION_TEST_ROOT")
	if rootPath == "" {
		t.Skip("set LSP_TRACE_BASE_PUBLICATION_TEST_ROOT for the isolated production-root diagnostic")
	}
	root, err := publication.OpenRoot(rootPath)
	if err != nil {
		t.Fatalf("ASSERT_BASE_PUBLICATION_ROOT_OPEN: %v", err)
	}
	defer root.Close()
	if !filepath.IsAbs(root.Path()) || filepath.Clean(root.Path()) != rootPath {
		t.Fatal("ASSERT_BASE_PUBLICATION_ROOT_IDENTITY_EQUAL=false")
	}

	var events []publication.BoundFileTraceEvent
	publisher := captureset.NewPublisherWithTrace(root, func(event publication.BoundFileTraceEvent) { events = append(events, event) })
	receipt, failure, _ := publishCensusProjectionWith(context.Background(), mcpContinuationProjection(t), publisher)
	if failure != nil || receipt == nil {
		t.Fatalf("ASSERT_BASE_PUBLICATION_FIXTURE: receipt=%v failure=%v events=%v", receipt != nil, failure != nil, events)
	}
	published := filepath.Join(rootPath, filepath.FromSlash(receipt.Selector))
	t.Cleanup(func() {
		_ = os.Remove(published)
		_ = os.Remove(filepath.Dir(published))
		_ = os.Remove(filepath.Dir(filepath.Dir(published)))
		_ = os.Remove(filepath.Dir(filepath.Dir(filepath.Dir(published))))
	})
	want := map[string]bool{"OPEN_VALIDATE": false, "CANDIDATE": false, "TARGET": false, "TEMP": false, "WRITE_FSYNC": false, "HARDLINK": false, "TARGET_EQUAL": false, "RECEIPT": false, "CLEANUP": false}
	for _, event := range events {
		if _, ok := want[event.Stage]; ok && event.OK {
			want[event.Stage] = true
		}
	}
	for stage, ok := range want {
		if !ok {
			t.Fatalf("ASSERT_BASE_PUBLICATION_TRACE_STAGE[%s]", stage)
		}
	}
}

func TestPublishCensusProjectionPreservesTypedPublisherFailure(t *testing.T) {
	sentinel := errors.New("private sentinel must not be public")
	publisher := censusProjectionPublisherFunc(func(captureset.Manifest, [][]byte, captureset.ExactBytesAuthority) captureset.PublicationResult {
		return captureset.PublicationResult{Err: sentinel}
	})
	_, failure, reason := publishCensusProjectionWith(context.Background(), censusacquisition.Projection{}, publisher)
	if failure == nil || !errors.Is(failure.err, sentinel) {
		t.Fatalf("ASSERT_MCP_CENSUS_PUBLICATION_INTERNAL_CAUSE: failure=%+v", failure)
	}
	if reason != reasonPublicationInternal {
		t.Fatalf("ASSERT_MCP_CENSUS_PUBLICATION_PUBLIC_REASON: got=%q want=%q", reason, reasonPublicationInternal)
	}
	completion := censusFailureCompletion(failure, reason)
	if completion.Diagnostic == nil || completion.Diagnostic.PublicationFailure != censusresult.PublicationFailureInternal {
		t.Fatalf("ASSERT_MCP_CENSUS_PUBLICATION_PUBLIC_ENUM: diagnostic=%+v", completion.Diagnostic)
	}
	encoded, err := censusresult.MarshalDiagnostic(*completion.Diagnostic)
	if err != nil || bytes.Contains(encoded, []byte(sentinel.Error())) {
		t.Fatalf("ASSERT_MCP_CENSUS_PUBLICATION_PRIVATE_ERROR_NOT_EXPOSED: encoded=%q err=%v", encoded, err)
	}
}

func TestPublicationDiagnosticDirectAndDelegatedGatewayParity(t *testing.T) {
	observed, limit := uint64(17), uint64(64<<20)
	d, _ := censusresult.NewDiagnostic(censusresult.StagePublication, nil)
	d.PublicationFailure = censusresult.PublicationFailureTempWrite
	d.PublicationAccounting = &censusresult.PublicationAccounting{Category: censusresult.PublicationAccountingCandidateBytes, Observed: &observed, Limit: &limit}
	binding := newPrivateCensusMCPBinding(fixedPublicationDiagnosticRunner{completion: censusCompletion{Diagnostic: &d}})
	req := operation.Request{Name: operation.Census, RequestID: "publication-parity", Input: []byte(`{"session_id":"s","generation":1,"sources":["."]}`)}
	direct, err := binding.callDirect(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	gateway, err := binding.callCanonical(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(direct.Structured, gateway.Structured) || !bytes.Contains(direct.Structured, []byte(`"publication_accounting":{"category":"CANDIDATE_BYTES","observed":17,"limit":67108864}`)) {
		t.Fatalf("ASSERT_PUBLICATION_DIAGNOSTIC_DIRECT_GATEWAY_PARITY: direct=%s gateway=%s", direct.Structured, gateway.Structured)
	}
}

func TestPublishCensusProjectionProjectsOnlySafeKnownAccounting(t *testing.T) {
	private := &captureset.PublicationFailure{Stage: "TEMP_WRITE", Reason: "FAILED", CandidateSHA256: "sha256:" + strings.Repeat("a", 64), CandidateByteLength: 17, CodecCategory: "CAPTURE_SET_V1", CodecLimit: 64 << 20, RootSource: "HOST_PUBLICATION_ROOT", Err: errors.New("/private/secret raw error")}
	publisher := censusProjectionPublisherFunc(func(captureset.Manifest, [][]byte, captureset.ExactBytesAuthority) captureset.PublicationResult {
		return captureset.PublicationResult{Failure: private, Err: private}
	})
	_, failure, reason := publishCensusProjectionWith(context.Background(), censusacquisition.Projection{}, publisher)
	completion := censusFailureCompletion(failure, reason)
	raw, err := censusresult.MarshalDiagnostic(*completion.Diagnostic)
	if err != nil || !bytes.Contains(raw, []byte(`"publication_accounting":{"category":"CANDIDATE_BYTES","observed":17,"limit":67108864}`)) || bytes.Contains(raw, []byte("private")) || bytes.Contains(raw, []byte("secret")) {
		t.Fatalf("ASSERT_PUBLICATION_SAFE_ACCOUNTING_ONLY: raw=%s err=%v", raw, err)
	}
}

func TestPublishCensusProjectionFailsBeforeReceipt(t *testing.T) {
	publisher := censusProjectionPublisherFunc(func(captureset.Manifest, [][]byte, captureset.ExactBytesAuthority) captureset.PublicationResult {
		return captureset.PublicationResult{Err: errors.New("precommit")}
	})
	if receipt, failure, _ := publishCensusProjectionWith(context.Background(), censusacquisition.Projection{}, publisher); receipt != nil || failure == nil || failure.stage != censusStagePublication || failure.code != censusCodePublicationFailed {
		t.Fatalf("ASSERT_MCP_CENSUS_PRECOMMIT_PUBLICATION_FAILURE: receipt=%+v failure=%+v", receipt, failure)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if receipt, failure, _ := publishCensusProjectionWith(ctx, censusacquisition.Projection{}, publisher); receipt != nil || failure == nil {
		t.Fatalf("ASSERT_MCP_CENSUS_CANCEL_BEFORE_PUBLICATION: receipt=%+v failure=%+v", receipt, failure)
	}
	if receipt, failure, _ := publishCensusProjection(context.Background(), operation.Request{}, censusacquisition.Projection{}); receipt != nil || failure == nil {
		t.Fatalf("ASSERT_MCP_CENSUS_HOST_ROOT_REQUIRED: receipt=%+v failure=%+v", receipt, failure)
	}
}
