package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"

	"lsp-trace/internal/captureset"
	"lsp-trace/internal/censuschunking"
	"lsp-trace/internal/censusresult"
	"lsp-trace/internal/mcp"
	"lsp-trace/internal/mcpcontract"
	"lsp-trace/internal/operation"
	"lsp-trace/internal/publication"
	"lsp-trace/internal/session"
)

type privateCensusRunnerFixture struct {
	completion censusCompletion
	calls      []operation.Request
}

func (f *privateCensusRunnerFixture) run(_ context.Context, request operation.Request) censusCompletion {
	f.calls = append(f.calls, request)
	return f.completion
}

func TestCensusMCPBindingRejectsMismatchedOperationBeforeDecodeRuntime(t *testing.T) {
	fixture := &privateCensusRunnerFixture{}
	binding := newPrivateCensusMCPBinding(fixture)
	_, failure := binding.Execute(context.Background(), operation.Request{Name: operation.Verify, RequestID: "r", Input: []byte(`{}`)})
	if failure == nil || failure.Code != operation.FailureInvalidInput || len(fixture.calls) != 0 {
		t.Fatalf("ASSERT_CENSUS_NAME_MISMATCH_REJECTED_BEFORE_DECODE_RUNTIME: failure=%v calls=%d", failure, len(fixture.calls))
	}
}

func TestPrivateCensusMCPBootstrapActivationExactDefaultOffAndRebinding(t *testing.T) {
	for _, tc := range []struct {
		value string
		want  bool
	}{
		{value: "", want: false},
		{value: "true", want: false},
		{value: "01", want: false},
		{value: "1", want: true},
	} {
		t.Run("value-"+tc.value, func(t *testing.T) {
			t.Setenv("LSP_TRACE_PRIVATE_CENSUS_CHUNKING", tc.value)
			server, _, err := newServerRuntime(false)
			if err != nil {
				t.Fatal(err)
			}
			binding, ok := server.Executors[mcp.CensusExecutorFamily].(*privateCensusMCPBinding)
			if !ok || binding.privateCensusChunking != tc.want {
				t.Fatalf("ASSERT_PRIVATE_CENSUS_BOOTSTRAP_EXACT_VALUE: value=%q enabled=%v want=%v", tc.value, binding.privateCensusChunking, tc.want)
			}
		})
	}

	t.Setenv("LSP_TRACE_PRIVATE_CENSUS_CHUNKING", "")
	server, _, err := newServerRuntime(false)
	if err != nil {
		t.Fatal(err)
	}
	binding, ok := server.Executors[mcp.CensusExecutorFamily].(*privateCensusMCPBinding)
	if !ok || binding.privateCensusChunking {
		t.Fatalf("ASSERT_PRIVATE_CENSUS_BOOTSTRAP_DEFAULT_OFF: binding=%+v", binding)
	}

	rebound := &privateCensusMCPBinding{runtime: &privateCensusRunnerFixture{}, privateCensusChunking: true}
	rebound = replacePrivateCensusMCPContinuation(rebound, &continuationMCPHostFixture{})
	if !rebound.privateCensusChunking {
		t.Fatal("ASSERT_PRIVATE_CENSUS_BOOTSTRAP_REBIND_PRESERVES")
	}
}

func TestPrivateCensusMCPGatewayComposesBootstrapActivationThroughDocumentSymbolPolicy(t *testing.T) {
	for _, tc := range []struct {
		name              string
		env               string
		requestActivation bool
		wantSymbols       int
		wantDocuments     int
		wantState         session.Failure
		wantGatewayReject bool
	}{
		{name: "exact-env-enabled", env: "1", wantSymbols: 2, wantDocuments: 2},
		{name: "default-off", wantSymbols: 1, wantDocuments: 1, wantState: session.LifecycleConflict},
		{name: "request-input-cannot-enable", requestActivation: true, wantGatewayReject: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("LSP_TRACE_PRIVATE_CENSUS_CHUNKING", tc.env)
			workspace := t.TempDir()
			writeRuntimeSource(t, workspace, "a.go")
			writeRuntimeSource(t, workspace, "b.go")
			runtime, starter, started := censusRuntimeFixture(t, censusRuntimeOptions{
				workspace: workspace, ready: true, documentSymbolSupport: true, callHierarchySupport: true,
				notifications: 65, results: map[string]json.RawMessage{"textDocument/documentSymbol": json.RawMessage(`[]`)},
			})
			root, err := publication.OpenRoot(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = root.Close() })
			server, bootstrapManager, err := newServerRuntime(false, root)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = bootstrapManager.Shutdown(context.Background()) })
			binding, ok := server.Executors[mcp.CensusExecutorFamily].(*privateCensusMCPBinding)
			if !ok || binding.privateCensusChunking != (tc.env == "1") {
				t.Fatalf("ASSERT_COMPOSED_CENSUS_BOOTSTRAP_BINDING: binding=%T enabled=%v env=%q", server.Executors[mcp.CensusExecutorFamily], binding.privateCensusChunking, tc.env)
			}
			binding.runtime = newCensusRuntime(runtime)

			arguments := map[string]any{"session_id": "project", "generation": started.Generation, "sources": []string{"."}, "max_nodes": 2, "timeout_ms": 1000, "request_timeout_ms": 500}
			if tc.requestActivation {
				arguments["private_census_chunking"] = true
			}
			request := map[string]any{
				"jsonrpc": "2.0", "id": 1, "method": "tools/call",
				"params": map[string]any{"name": "lsp_trace_v1_execute", "arguments": map[string]any{"request": map[string]any{"operation": mcpcontract.CensusTool, "arguments": arguments}}},
			}
			raw, err := json.Marshal(request)
			if err != nil {
				t.Fatal(err)
			}
			var output bytes.Buffer
			if err := server.Serve(bytes.NewReader(append(raw, '\n')), &output); err != nil {
				t.Fatal(err)
			}
			if tc.wantGatewayReject != strings.Contains(output.String(), `"code":-32602`) {
				t.Fatalf("ASSERT_COMPOSED_CENSUS_REQUEST_ACTIVATION_STRICTLY_REJECTED: reject=%t output=%s", tc.wantGatewayReject, output.String())
			}

			_, methods, _, _ := starter.snapshot()
			joined := strings.Join(methods, ",")
			if got := strings.Count(joined, "textDocument/documentSymbol"); got != tc.wantSymbols {
				t.Fatalf("ASSERT_COMPOSED_CENSUS_DOCUMENT_SYMBOL_POLICY: symbols=%d want=%d methods=%v output=%s", got, tc.wantSymbols, methods, output.String())
			}
			if got := strings.Count(joined, "textDocument/didOpen"); got != tc.wantDocuments {
				t.Fatalf("ASSERT_COMPOSED_CENSUS_LATER_FILE_POLICY: documents=%d want=%d methods=%v", got, tc.wantDocuments, methods)
			}
			_, stateFailure := runtime.Manager.Metadata(started.SessionID, started.Generation)
			if stateFailure != tc.wantState {
				t.Fatalf("ASSERT_COMPOSED_CENSUS_SESSION_STATE: got=%s want=%s methods=%v", stateFailure, tc.wantState, methods)
			}
			if tc.requestActivation && binding.privateCensusChunking {
				t.Fatal("ASSERT_COMPOSED_CENSUS_REQUEST_CANNOT_AUTHORIZE_PROVIDER_OR_RUNTIME")
			}
		})
	}
}

func TestPrivateCensusMCPBindingActivationCustody(t *testing.T) {
	input := []byte(`{"session_id":"s","generation":1,"sources":["."]}`)
	completion := censusCompletion{Result: ptrCensusResult(privateCensusSuccessFixture())}
	ordinaryRunner := &privateCensusRunnerFixture{completion: completion}
	enabledRunner := &privateCensusRunnerFixture{completion: completion}
	ordinary := &privateCensusMCPBinding{runtime: ordinaryRunner}
	enabled := &privateCensusMCPBinding{runtime: enabledRunner, privateCensusChunking: true}
	request := operation.Request{Name: operation.Census, RequestID: "activation-custody", Input: input}

	ordinaryResult, ordinaryErr := ordinary.callDirect(context.Background(), request)
	enabledResult, enabledErr := enabled.callDirect(context.Background(), request)
	if ordinaryErr != nil || enabledErr != nil {
		t.Fatalf("ASSERT_PRIVATE_CENSUS_ACTIVATION_REQUEST_ACCEPTED: ordinary=%v enabled=%v", ordinaryErr, enabledErr)
	}
	if !bytes.Equal(ordinaryResult.Structured, enabledResult.Structured) || len(ordinaryRunner.calls) != 1 || len(enabledRunner.calls) != 1 {
		t.Fatalf("ASSERT_PRIVATE_CENSUS_ACTIVATION_WIRE_INVARIANT: ordinary=%q enabled=%q calls=%d/%d", ordinaryResult.Structured, enabledResult.Structured, len(ordinaryRunner.calls), len(enabledRunner.calls))
	}
	ordinaryRequest, enabledRequest := ordinaryRunner.calls[0], enabledRunner.calls[0]
	if !bytes.Equal(ordinaryRequest.Input, enabledRequest.Input) || ordinaryRequest.PrivateCensusChunking || !enabledRequest.PrivateCensusChunking {
		t.Fatalf("ASSERT_PRIVATE_CENSUS_ACTIVATION_INTERNAL_ONLY: ordinary=%+v enabled=%+v", ordinaryRequest, enabledRequest)
	}
	var jsonRequest operation.Request
	if err := json.Unmarshal([]byte(`{"PrivateCensusChunking":true}`), &jsonRequest); err != nil || jsonRequest.PrivateCensusChunking {
		t.Fatalf("ASSERT_PRIVATE_CENSUS_ACTIVATION_JSON_CANNOT_ENABLE: request=%+v err=%v", jsonRequest, err)
	}
}

func TestPrivateCensusChunkSuccessProjectsSuccessorCustodyOnly(t *testing.T) {
	terminal := realCoordinatorTerminalFixture(t)
	parsed, err := censuschunking.ParseTerminalIndex(terminal.terminalIndex)
	if err != nil || parsed.ImmutableSelector != terminal.terminalSelector || parsed.Authority != 0 || parsed.Accepted || parsed.SourceGraphComplete != "UNKNOWN" || parsed.CrossChunkCalls != "NOT_COMPUTED" || parsed.AggregateCapture != "NOT_MATERIALIZED" {
		t.Fatalf("ASSERT_PRIVATE_CENSUS_REAL_CANONICAL_TERMINAL: parsed=%+v err=%v", parsed, err)
	}
	completion := censusCompletion{privateChunk: terminal}
	direct, err := projectPrivateCensusMCP("chunk-success", completion)
	if err != nil {
		t.Fatalf("ASSERT_PRIVATE_CENSUS_SUCCESSOR_PROJECTED: %v", err)
	}
	canonical, err := projectPrivateCensusMCP("chunk-success", completion)
	if err != nil || !bytes.Equal(direct.Structured, canonical.Structured) {
		t.Fatalf("ASSERT_PRIVATE_CENSUS_SUCCESSOR_DIRECT_CANONICAL_PARITY: err=%v direct=%q canonical=%q", err, direct.Structured, canonical.Structured)
	}
	var envelope map[string]any
	if err := json.Unmarshal(direct.Structured, &envelope); err != nil {
		t.Fatal(err)
	}
	_, hasResult := envelope["result"]
	custody, hasCustody := envelope["custody_receipt"].(map[string]any)
	encoded, _ := custody["terminal_index_base64"].(string)
	projectedBytes, decodeErr := base64.StdEncoding.DecodeString(encoded)
	if envelope["envelope_schema_id"] != mcpcontract.CensusChunkTerminalSuccessID || !hasCustody || hasResult || custody["terminal_selector"] != terminal.terminalSelector || decodeErr != nil || !bytes.Equal(projectedBytes, terminal.terminalIndex) {
		t.Fatalf("ASSERT_PRIVATE_CENSUS_SUCCESSOR_CUSTODY_ONLY: envelope=%v decode=%v", envelope, decodeErr)
	}
	terminal.terminalIndex[0] ^= 0xff
	if bytes.Equal(projectedBytes, terminal.terminalIndex) || !bytes.Equal(direct.Text, direct.Structured) {
		t.Fatal("ASSERT_PRIVATE_CENSUS_SUCCESSOR_DEEP_COPY_ALIAS_RESISTANCE")
	}
}

func TestPrivateCensusChunkProjectionRejectsInvalidTerminal(t *testing.T) {
	valid := realCoordinatorTerminalFixture(t)
	var ceiling map[string]any
	if err := json.Unmarshal(valid.terminalIndex, &ceiling); err != nil {
		t.Fatal(err)
	}
	ceiling["authority"] = float64(1)
	forbidden, err := json.Marshal(ceiling)
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name       string
		completion *privateCensusChunkCompletion
	}{
		{name: "empty-object", completion: &privateCensusChunkCompletion{terminalSelector: valid.terminalSelector, terminalIndex: []byte(`{}`)}},
		{name: "wrong-selector", completion: &privateCensusChunkCompletion{terminalSelector: "sha256:" + strings.Repeat("f", 64), terminalIndex: append([]byte(nil), valid.terminalIndex...)}},
		{name: "forbidden-ceiling", completion: &privateCensusChunkCompletion{terminalSelector: valid.terminalSelector, terminalIndex: forbidden}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := projectPrivateCensusMCP("invalid-terminal", censusCompletion{privateChunk: tc.completion}); err == nil {
				t.Fatalf("ASSERT_PRIVATE_CENSUS_INVALID_TERMINAL_REJECTED: %s", tc.name)
			}
		})
	}
}

func TestPrivateCensusV1EnvelopeRejectsCustodyReceipt(t *testing.T) {
	result := privateCensusSuccessFixture()
	env := censusMCPEnvelope{
		EnvelopeVersion: "1", EnvelopeSchemaID: mcpcontract.FutureCensusSuccessID,
		Tool: mcpcontract.FutureCensusTool, RequestID: "custody-rejection", Outcome: "COMPLETE",
		OperationStatus: "SUCCEEDED", Result: result,
	}
	raw, err := json.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}
	var object map[string]any
	if err := json.Unmarshal(raw, &object); err != nil {
		t.Fatal(err)
	}
	object["custody_receipt"] = map[string]any{"schema_version": "lsp-trace.census-chunk-terminal-custody-receipt.private.v1", "terminal_selector": "sha256:terminal", "terminal_index_base64": "e30="}
	raw, err = json.Marshal(object)
	if err != nil {
		t.Fatal(err)
	}
	if err := mcpcontract.ValidateFutureCensusEnvelopeExclusive(raw); err == nil {
		t.Fatal("ASSERT_PRIVATE_CENSUS_V1_REJECTS_CUSTODY")
	}
}

func TestPrivateCensusSuccessorSchemaAvailable(t *testing.T) {
	const successorID = "https://jaresty.github.io/lsp-trace/mcp/schemas/envelope-census-chunk-terminal-result.v1.schema.json"
	raw, err := mcpcontract.FutureCensusSchemaJSON(successorID)
	if err != nil || len(raw) == 0 {
		t.Fatalf("ASSERT_PRIVATE_CENSUS_SUCCESSOR_SCHEMA_AVAILABLE: bytes=%d err=%v", len(raw), err)
	}
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
