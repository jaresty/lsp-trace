package main

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"lsp-trace/internal/captureset"
	"lsp-trace/internal/censusresult"
	"lsp-trace/internal/mcpcontract"
	"lsp-trace/internal/operation"
)

type privateCensusRunnerFixture struct {
	completion censusCompletion
	calls      []operation.Request
}

func (f *privateCensusRunnerFixture) run(_ context.Context, request operation.Request) censusCompletion {
	f.calls = append(f.calls, request)
	return f.completion
}

func privateCensusSuccessFixture() censusresult.Result {
	return censusresult.Result{
		SchemaVersion: censusresult.SchemaVersion, Status: "SUCCEEDED", CensusID: "census", CaptureSetID: "sha256:" + strings.Repeat("a", 64),
		SessionID: "session", Generation: 1, TargetCount: 1, BatchCount: 1,
		FileAccounting: censusresult.FileAccounting{Denominator: 1, Processed: 1}, SymbolAccounting: censusresult.SymbolAccounting{Denominator: 1, Prepared: 1},
		Authority: 0, SourceGraphComplete: "UNKNOWN", CrossCaptureCalls: []string{},
		Publication: censusresult.PublicationReceipt{Selector: "capture-sets/v1/census.json", Digest: "sha256:" + strings.Repeat("b", 64), ByteLength: 1, VerificationStatus: "VERIFIED", DirectorySyncStatus: censusresult.DirectorySyncComplete, CloseStatus: censusresult.CloseComplete},
	}
}

func TestPrivateCensusMCPBindingDirectCanonicalParity(t *testing.T) {
	batch := 1
	degraded, err := censusresult.NewDiagnostic(censusresult.StageCommitted, nil)
	if err != nil {
		t.Fatal(err)
	}
	domain, err := censusresult.NewDiagnostic(censusresult.StageAcquisition, &batch)
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name, outcome, status, schemaID string
		isError                         bool
		completion                      censusCompletion
	}{
		{name: "success", outcome: "COMPLETE", status: "SUCCEEDED", schemaID: mcpcontract.FutureCensusSuccessID, completion: censusCompletion{Result: ptrCensusResult(privateCensusSuccessFixture())}},
		{name: "committed-degraded", outcome: "COMMITTED_DEGRADED", status: "SUCCEEDED", schemaID: mcpcontract.FutureCensusSuccessID, completion: censusCompletion{Diagnostic: &degraded}},
		{name: "domain-error", outcome: "DOMAIN_ERROR", status: "FAILED", schemaID: mcpcontract.FutureCensusDomainErrorID, isError: true, completion: censusCompletion{Diagnostic: &domain}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fixture := &privateCensusRunnerFixture{completion: tc.completion}
			binding := newPrivateCensusMCPBinding(fixture)
			request := operation.Request{Name: "census", RequestID: "request-34", Input: []byte(`{"session_id":"s","generation":1,"sources":["."]}`)}
			direct, err := binding.callDirect(context.Background(), request)
			if err != nil {
				t.Fatal(err)
			}
			canonical, err := binding.callCanonical(context.Background(), request)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(direct.Text, direct.Structured) || !bytes.Equal(canonical.Text, canonical.Structured) || !bytes.Equal(direct.Structured, canonical.Structured) {
				t.Fatalf("ASSERT_PRIVATE_CENSUS_TEXT_STRUCTURED_DIRECT_CANONICAL_BYTES: direct=%q/%q canonical=%q/%q", direct.Text, direct.Structured, canonical.Text, canonical.Structured)
			}
			if err := mcpcontract.ValidateFutureCensusEnvelopeExclusive(direct.Structured); err != nil {
				t.Fatalf("ASSERT_PRIVATE_CENSUS_SCHEMA_EXCLUSIVE_VALIDATION: %v", err)
			}
			if len(fixture.calls) != 2 || fixture.calls[0].RequestID != "request-34" || fixture.calls[1].RequestID != "request-34" {
				t.Fatalf("ASSERT_PRIVATE_CENSUS_RUNTIME_RUN_AND_REQUEST_ID: calls=%+v", fixture.calls)
			}
			var env map[string]any
			if err := json.Unmarshal(direct.Structured, &env); err != nil {
				t.Fatal(err)
			}
			if env["request_id"] != "request-34" || env["tool"] != mcpcontract.FutureCensusTool || env["outcome"] != tc.outcome || env["operation_status"] != tc.status || env["envelope_schema_id"] != tc.schemaID || env["isError"] != tc.isError {
				t.Fatalf("ASSERT_PRIVATE_CENSUS_ENVELOPE_DIMENSIONS: %v", env)
			}
			_, hasResult := env["result"]
			_, hasError := env["error"]
			if hasResult == hasError || hasResult != !tc.isError {
				t.Fatalf("ASSERT_PRIVATE_CENSUS_SCHEMA_EXCLUSIVITY: %v", env)
			}
			raw := string(direct.Structured)
			if strings.Contains(raw, "output_selector") || strings.Contains(raw, "/private/") || strings.Contains(raw, "secret") {
				t.Fatalf("ASSERT_PRIVATE_CENSUS_NO_SELECTOR_ERROR_PATH_LEAK: %s", raw)
			}
			if tc.outcome == "COMPLETE" && env["result"].(map[string]any)["authority"] != float64(0) {
				t.Fatalf("ASSERT_PRIVATE_CENSUS_ZERO_AUTHORITY: %v", env)
			}
		})
	}
}

func TestPrivateCensusMCPBindingRejectsCallerOutputSelector(t *testing.T) {
	fixture := &privateCensusRunnerFixture{completion: censusCompletion{Result: ptrCensusResult(privateCensusSuccessFixture())}}
	binding := newPrivateCensusMCPBinding(fixture)
	request := operation.Request{RequestID: "request-34", Input: []byte(`{"session_id":"s","generation":1,"sources":["."],"output_selector":"private.json"}`)}
	if _, err := binding.callDirect(context.Background(), request); err == nil || len(fixture.calls) != 0 {
		t.Fatalf("ASSERT_PRIVATE_CENSUS_CALLER_OUTPUT_SELECTOR_REJECTED: err=%v calls=%+v", err, fixture.calls)
	}
}

func TestPrivateCensusMCPBindingRejectsInvalidRequestID(t *testing.T) {
	completion := censusCompletion{Result: ptrCensusResult(privateCensusSuccessFixture())}
	for _, requestID := range []string{"", strings.Repeat("r", 257)} {
		fixture := &privateCensusRunnerFixture{completion: completion}
		binding := newPrivateCensusMCPBinding(fixture)
		request := operation.Request{RequestID: requestID, Input: []byte(`{"session_id":"s","generation":1,"sources":["."]}`)}
		if _, err := binding.callDirect(context.Background(), request); err == nil || len(fixture.calls) != 0 {
			t.Fatalf("ASSERT_PRIVATE_CENSUS_REQUEST_ID_PREFLIGHT_BOUND: length=%d err=%v calls=%+v", len(requestID), err, fixture.calls)
		}
		if _, err := projectPrivateCensusMCP(requestID, completion); err == nil {
			t.Fatalf("ASSERT_PRIVATE_CENSUS_REQUEST_ID_ENVELOPE_BOUND: length=%d", len(requestID))
		}
	}
}

func TestPrivateCensusMCPInvocationScopedRequestIDs(t *testing.T) {
	fixture := &privateCensusRunnerFixture{completion: censusCompletion{Result: ptrCensusResult(privateCensusSuccessFixture())}}
	binding := newPrivateCensusMCPBinding(fixture)
	calls := []struct {
		requestID string
		invoke    func(context.Context, operation.Request) (privateCensusMCPResult, error)
	}{
		{requestID: "outer-direct-34", invoke: binding.callDirect},
		{requestID: "outer-canonical-34", invoke: binding.callCanonical},
	}
	for _, call := range calls {
		request := operation.Request{Name: "census", RequestID: call.requestID, Input: []byte(`{"session_id":"s","generation":1,"sources":["."]}`)}
		got, err := call.invoke(context.Background(), request)
		if err != nil {
			t.Fatal(err)
		}
		var envelope censusMCPEnvelope
		if err := json.Unmarshal(got.Structured, &envelope); err != nil {
			t.Fatal(err)
		}
		if envelope.RequestID != call.requestID {
			t.Fatalf("ASSERT_PRIVATE_CENSUS_INVOCATION_REQUEST_ID: envelope=%q request=%q", envelope.RequestID, call.requestID)
		}
		if !bytes.Equal(got.Text, got.Structured) || got.IsError {
			t.Fatalf("ASSERT_PRIVATE_CENSUS_INVOCATION_BYTES: %+v", got)
		}
	}
	if len(fixture.calls) != len(calls) || fixture.calls[0].RequestID != calls[0].requestID || fixture.calls[1].RequestID != calls[1].requestID || fixture.calls[0].RequestID == fixture.calls[1].RequestID {
		t.Fatalf("ASSERT_PRIVATE_CENSUS_REQUEST_IDS_ARE_PER_INVOCATION: calls=%+v", fixture.calls)
	}
}

func TestPrivateCensusMCPTerminalPublicationParity(t *testing.T) {
	cases := []struct {
		name          string
		mutateReceipt func(*captureset.PublicationReceipt)
		wantOutcome   string
	}{
		{name: "complete", wantOutcome: "COMPLETE"},
		{name: "committed-degraded", mutateReceipt: func(receipt *captureset.PublicationReceipt) { receipt.CloseStatus = "COMMITTED_CLOSE_FAILED" }, wantOutcome: "COMMITTED_DEGRADED"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			projection, receipt := validMCPCompletionProjection(), validMCPCompletionReceipt()
			if tc.mutateReceipt != nil {
				tc.mutateReceipt(&receipt)
			}
			publications := 0
			publisher := completionPublisher(receipt, func() { publications++ })
			verifier := &censusVerifierStub{manifest: captureset.Manifest{Constituents: []captureset.Constituent{{ImmutableSelector: "graphs/v5/a.json"}}}}
			completion := completeCensusProjectionWith(context.Background(), projection, publisher, verifier)
			got, err := projectPrivateCensusMCP("outer-34", completion)
			if err != nil {
				t.Fatal(err)
			}
			if publications != 1 {
				t.Fatalf("ASSERT_PRIVATE_CENSUS_ZERO_DUPLICATE_PUBLICATION: publications=%d", publications)
			}
			if err := mcpcontract.ValidateFutureCensusEnvelopeExclusive(got.Structured); err != nil {
				t.Fatalf("ASSERT_PRIVATE_CENSUS_TERMINAL_SCHEMA_EXCLUSIVE: %v", err)
			}
			var envelope map[string]any
			if err := json.Unmarshal(got.Structured, &envelope); err != nil {
				t.Fatal(err)
			}
			_, hasResult := envelope["result"]
			_, hasError := envelope["error"]
			if envelope["outcome"] != tc.wantOutcome || envelope["envelope_schema_id"] != mcpcontract.FutureCensusSuccessID || !hasResult || hasError || !bytes.Equal(got.Text, got.Structured) {
				t.Fatalf("ASSERT_PRIVATE_CENSUS_TERMINAL_PARITY: envelope=%v text=%q structured=%q", envelope, got.Text, got.Structured)
			}
			if tc.wantOutcome == "COMPLETE" {
				if authority := envelope["result"].(map[string]any)["authority"].(float64); authority > 0 {
					t.Fatalf("ASSERT_PRIVATE_CENSUS_AUTHORITY_CEILING: authority=%v ceiling=0", authority)
				}
			} else if diagnostic := completion.Diagnostic; diagnostic == nil || diagnostic.Stage != censusresult.StageCommitted {
				t.Fatalf("ASSERT_PRIVATE_CENSUS_COMMITTED_DIAGNOSTIC: %+v", completion)
			}
		})
	}
}

func ptrCensusResult(value censusresult.Result) *censusresult.Result { return &value }
