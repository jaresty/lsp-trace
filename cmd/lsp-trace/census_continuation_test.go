package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"lsp-trace/internal/captureset"
	"lsp-trace/internal/censuscontinuation"
	"lsp-trace/internal/censusresult"
	"lsp-trace/internal/programccompose"
)

func TestCLICommittedContinuationCustody(t *testing.T) {
	root := t.TempDir()
	if err := os.Chmod(root, 0o700); err != nil {
		t.Fatal(err)
	}
	projection := censusPublicationProjection(t, 65)
	metadata := programccompose.ExactMetadata{
		WorkspaceIdentity:    projection.Workspace,
		RevisionCustody:      "CALLER_ASSERTED",
		PositionEncoding:     "utf-16",
		AcquisitionSemantics: "managed-lsp-v1",
		PrivacyPolicy:        "private-census-v1",
	}
	outcome := publishCensusCaptureSetWithContinuation(context.Background(), censusPublicationCapabilityFor(t, projection), root, metadata)
	if outcome.Result == nil || outcome.Diagnostic != nil {
		t.Fatalf("ASSERT_CLI_COMMITTED_HANDOFF_BUILDS completion=%+v", outcome)
	}
	before, err := json.Marshal(outcome.Result)
	if err != nil {
		t.Fatal(err)
	}
	handoff, err := outcome.BuildCommittedHandoff()
	if err != nil {
		t.Fatalf("ASSERT_CLI_COMMITTED_HANDOFF_BUILDS: %v", err)
	}
	if err := handoff.Validate(); err != nil {
		t.Fatalf("ASSERT_CLI_COMMITTED_HANDOFF_VALIDATES: %v", err)
	}
	after, err := json.Marshal(outcome.Result)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatalf("ASSERT_CLI_HISTORICAL_BYTES_IDENTICAL before=%q after=%q err=%v", before, after, err)
	}
	if bytes.Contains(after, []byte("workspace_identity")) || bytes.Contains(after, []byte("constituents")) || bytes.Contains(after, []byte(projection.Workspace)) {
		t.Fatalf("ASSERT_CLI_HISTORICAL_EXCLUDES_CUSTODY: %s", after)
	}

	projection.Constituents[0].Raw[0] ^= 1
	p, err := handoff.Projection()
	if err != nil || bytes.Equal(p.Constituents[0].Raw, projection.Constituents[0].Raw) {
		t.Fatalf("ASSERT_CLI_HANDOFF_DEFENSIVE_CLONE err=%v", err)
	}

	for name, mutate := range map[string]func(*censusPublicationOutcome){
		"missing": func(o *censusPublicationOutcome) { o.continuation = nil },
		"receipt": func(o *censusPublicationOutcome) {
			o.continuation.publication.Receipt = captureset.PublicationReceipt{}
		},
		"manifest": func(o *censusPublicationOutcome) { o.continuation.publication.Manifest = captureset.Manifest{} },
		"resolved": func(o *censusPublicationOutcome) { o.continuation.publication.Resolved = nil },
		"metadata": func(o *censusPublicationOutcome) { o.continuation.metadata = programccompose.ExactMetadata{} },
	} {
		t.Run(name, func(t *testing.T) {
			copy := outcome
			custody, cloneErr := cloneCensusContinuationCustody(*outcome.continuation)
			if cloneErr != nil {
				t.Fatal(cloneErr)
			}
			copy.continuation = &custody
			mutate(&copy)
			if _, err := copy.BuildCommittedHandoff(); err == nil {
				t.Fatalf("ASSERT_CLI_INCOMPLETE_CUSTODY_REJECTED_%s", strings.ToUpper(name))
			}
		})
	}

	degraded := outcome
	diagnostic, _ := buildCensusCLIDiagnostic(censusStageCommitted, nil)
	degraded.Diagnostic = &diagnostic
	if _, err := degraded.BuildCommittedHandoff(); err == nil {
		t.Fatal("ASSERT_CLI_DEGRADED_HANDOFF_REJECTED")
	}
	if _, err := censusresult.Marshal(censusresult.Result{}); err == nil {
		// Keep this package linked to the historical marshaler used by production.
	}
	_ = censuscontinuation.CompletenessUnknown
}
