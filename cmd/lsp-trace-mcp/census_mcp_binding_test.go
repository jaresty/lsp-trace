package main

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

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

func ptrCensusResult(value censusresult.Result) *censusresult.Result { return &value }
