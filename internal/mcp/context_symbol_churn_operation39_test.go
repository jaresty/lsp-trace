package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"lsp-trace/internal/mcpcontract"
	"lsp-trace/internal/operation"
	"lsp-trace/internal/vcssymbolsidecar"
)

type unavailableSymbolChurnExecutor struct{ calls []operation.Request }

type unavailableSymbolChurnCaptureExecutor struct{ calls []operation.Request }

type successfulSymbolChurnV3Executor struct{ calls []operation.Request }

const symbolChurnV3ArtifactFixture = `{"schema_version":"lsp-trace.vcs-symbol-churn-sidecar.v3","graph_artifact_digest":"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","from_revision":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb","to_revision":"cccccccccccccccccccccccccccccccccccccccc","old_acquisition":[],"new_acquisition":[],"authority":0,"source_graph_complete":"UNKNOWN","attribution":"HISTORICAL_SYMBOL_RANGE","cross_revision_identity":"NOT_EVALUATED","line_count":0,"attributed_line_count":0,"ambiguous_line_count":0,"unmatched_line_count":0,"lines":[],"historical_symbol_metrics":[],"current_symbol_metrics":[]}`

func (e *successfulSymbolChurnV3Executor) Execute(_ context.Context, req operation.Request) (operation.Result, *operation.Failure) {
	e.calls = append(e.calls, req)
	return operation.Result{Artifact: []byte(symbolChurnV3ArtifactFixture)}, nil
}

func (e *unavailableSymbolChurnCaptureExecutor) Execute(_ context.Context, req operation.Request) (operation.Result, *operation.Failure) {
	e.calls = append(e.calls, req)
	if req.Name != operation.Name("context_symbol_churn_capture") {
		return operation.Result{}, &operation.Failure{Code: operation.FailureNotImplemented, Err: operation.ErrNotImplemented}
	}
	return operation.Result{}, &operation.Failure{Code: "PROFILE_UNAVAILABLE", Err: errors.New("historical LSP profile unavailable")}
}

func (e *unavailableSymbolChurnExecutor) Execute(_ context.Context, req operation.Request) (operation.Result, *operation.Failure) {
	e.calls = append(e.calls, req)
	if req.Name != operation.Name("context_symbol_churn") {
		return operation.Result{}, &operation.Failure{Code: operation.FailureNotImplemented, Err: operation.ErrNotImplemented}
	}
	return operation.Result{}, &operation.Failure{Code: "PROFILE_UNAVAILABLE", Err: errors.New("historical LSP profile unavailable")}
}

func TestContextSymbolChurnOperation39AppendOnlyContract(t *testing.T) {
	const tool = "lsp_trace_v1_context_symbol_churn"
	full := NewRegistryWithProfile(false, ToolProfileFull)
	compact := NewRegistryWithProfile(false, ToolProfileCompact)
	_, ok := full.ResolveCanonical(tool)
	if !ok || len(full.Tools()) != 43 || len(full.Advertised()) != 43 || len(compact.Tools()) != 43 || len(compact.Advertised()) != 13 {
		t.Fatalf("ASSERT_CONTEXT_SYMBOL_CHURN_OPERATION39_APPEND_ONLY: ok=%t full=%d/%d compact=%d/%d", ok, len(full.Tools()), len(full.Advertised()), len(compact.Tools()), len(compact.Advertised()))
	}
	if _, ok := compact.ResolveCanonical(tool); !ok {
		t.Fatal("ASSERT_CONTEXT_SYMBOL_CHURN_COMPACT_DISPATCHABLE")
	}
	for _, candidate := range compact.Advertised() {
		if candidate.Name == tool {
			t.Fatal("ASSERT_CONTEXT_SYMBOL_CHURN_COMPACT_HIDDEN")
		}
	}
}

func TestContextSymbolChurnCaptureOperation40AppendOnlyContract(t *testing.T) {
	const tool = "lsp_trace_v1_context_symbol_churn_capture"
	full := NewRegistryWithProfile(false, ToolProfileFull)
	compact := NewRegistryWithProfile(false, ToolProfileCompact)
	_, fullOK := full.ResolveCanonical(tool)
	_, compactOK := compact.ResolveCanonical(tool)
	if !fullOK || !compactOK || len(full.Tools()) != 43 || len(full.Advertised()) != 43 || len(compact.Tools()) != 43 || len(compact.Advertised()) != 13 {
		t.Fatalf("ASSERT_SYMBOL_CHURN_OPERATION40_APPEND_ONLY: full_ok=%t compact_ok=%t full=%d/%d compact=%d/%d", fullOK, compactOK, len(full.Tools()), len(full.Advertised()), len(compact.Tools()), len(compact.Advertised()))
	}
	for _, candidate := range compact.Advertised() {
		if candidate.Name == tool {
			t.Fatal("ASSERT_SYMBOL_CHURN_OPERATION40_APPEND_ONLY: compact profile advertised operation 40")
		}
	}
}

func TestContextSymbolChurnV3ContractsAreAppendOnlyOnOperations39And40(t *testing.T) {
	const (
		artifactV3 = "https://jaresty.github.io/lsp-trace/schemas/lsp-trace.vcs-symbol-churn-sidecar.v3.schema.json"
		op39V3     = "https://jaresty.github.io/lsp-trace/mcp/schemas/envelope-context-symbol-churn-result.v3.schema.json"
		op40V2     = "https://jaresty.github.io/lsp-trace/mcp/schemas/envelope-context-symbol-churn-capture-result.v2.schema.json"
	)
	registry := NewRegistryWithProfile(false, ToolProfileFull)
	for _, tc := range []struct {
		name     string
		input    string
		envelope string
	}{
		{mcpcontract.ContextSymbolChurnTool, mcpcontract.ContextSymbolChurnInputID, op39V3},
		{mcpcontract.ContextSymbolChurnCaptureTool, mcpcontract.ContextSymbolChurnCaptureInputID, op40V2},
	} {
		tool, ok := registry.ResolveCanonical(tc.name)
		joinedArtifacts := strings.Join(tool.ArtifactSchemaIDs, "\n")
		joinedEnvelopes := strings.Join(tool.EnvelopeSchemaIDs, "\n")
		if !ok || tool.InputSchemaID != tc.input || !strings.Contains(joinedArtifacts, mcpcontract.ContextSymbolChurnResultID) || !strings.Contains(joinedArtifacts, artifactV3) || !strings.Contains(joinedEnvelopes, tc.envelope) {
			t.Fatalf("ASSERT_SYMBOL_CHURN_V3_APPEND_ONLY_OPERATION_CONTRACT: tool=%s ok=%t contract=%+v", tc.name, ok, tool)
		}
	}
	compact := NewRegistryWithProfile(false, ToolProfileCompact)
	if len(registry.Tools()) != 43 || len(registry.Advertised()) != 43 || len(compact.Tools()) != 43 || len(compact.Advertised()) != 13 {
		t.Fatalf("ASSERT_SYMBOL_CHURN_V3_TOOL_COUNTS_UNCHANGED: full=%d/%d compact=%d/%d", len(registry.Tools()), len(registry.Advertised()), len(compact.Tools()), len(compact.Advertised()))
	}
}

func symbolChurnEnvelopeBytes(t *testing.T, got response, gateway bool) []byte {
	t.Helper()
	if got.Error != nil {
		t.Fatalf("unexpected JSON-RPC error: %+v", got.Error)
	}
	wrapped, err := json.Marshal(got.Result)
	if err != nil {
		t.Fatal(err)
	}
	var decoded struct {
		StructuredContent json.RawMessage `json:"structuredContent"`
	}
	if err := json.Unmarshal(wrapped, &decoded); err != nil {
		t.Fatal(err)
	}
	if !gateway {
		return decoded.StructuredContent
	}
	var delegated struct {
		Envelope string `json:"delegated_envelope"`
	}
	if err := json.Unmarshal(decoded.StructuredContent, &delegated); err != nil {
		t.Fatal(err)
	}
	return []byte(delegated.Envelope)
}

func TestContextSymbolChurnV3DirectGatewayAndCaptureEnvelopeParity(t *testing.T) {
	const (
		op39V3 = "https://jaresty.github.io/lsp-trace/mcp/schemas/envelope-context-symbol-churn-result.v3.schema.json"
		op40V2 = "https://jaresty.github.io/lsp-trace/mcp/schemas/envelope-context-symbol-churn-capture-result.v2.schema.json"
	)
	for _, tc := range []struct {
		name     string
		family   ExecutorFamily
		expected string
		args     map[string]any
	}{
		{mcpcontract.ContextSymbolChurnTool, ContextSymbolChurnExecutorFamily, op39V3, map[string]any{"input": "{}", "workspace": "/tmp/repo", "from_revision": "HEAD~1", "to_revision": "HEAD", "profile": "go", "language_id": "go"}},
		{mcpcontract.ContextSymbolChurnCaptureTool, ContextSymbolChurnCaptureExecutorFamily, op40V2, map[string]any{"session_id": "project", "generation": 1, "uri": "file:///tmp/repo/a.go", "symbol": "A", "down_depth": 1, "up_depth": 0, "max_nodes": 10, "timeout_ms": 1000, "request_timeout_ms": 500, "analysis": map[string]any{"kind": "NEIGHBORHOOD"}, "from_revision": "HEAD~1", "to_revision": "HEAD", "profile": "go", "language_id": "go"}},
	} {
		executor := &successfulSymbolChurnV3Executor{}
		s := &Server{Registry: NewRegistryWithProfile(false, ToolProfileFull), Executors: map[ExecutorFamily]Executor{tc.family: executor}}
		direct := s.callContext(context.Background(), response{JSONRPC: "2.0", ID: float64(1)}, mustCallParams(t, tc.name, tc.args))
		gateway := s.callContext(context.Background(), response{JSONRPC: "2.0", ID: float64(2)}, mustCallParams(t, "lsp_trace_v1_execute", map[string]any{"request": map[string]any{"operation": tc.name, "arguments": tc.args}}))
		directRaw := symbolChurnEnvelopeBytes(t, direct, false)
		gatewayRaw := symbolChurnEnvelopeBytes(t, gateway, true)
		for i, raw := range [][]byte{directRaw, gatewayRaw} {
			if !strings.Contains(string(raw), `"envelope_schema_id":"`+tc.expected+`"`) || !strings.Contains(string(raw), `"schema_version":"lsp-trace.vcs-symbol-churn-sidecar.v3"`) {
				t.Fatalf("ASSERT_SYMBOL_CHURN_V3_DIRECT_GATEWAY_CAPTURE_SUCCESS[%s/%d]: %s", tc.name, i, raw)
			}
		}
		if len(executor.calls) != 2 {
			t.Fatalf("ASSERT_SYMBOL_CHURN_V3_SHARED_DISPATCH[%s]: calls=%d", tc.name, len(executor.calls))
		}
	}
}

func TestContextSymbolChurnSummaryBalances(t *testing.T) {
	summary := contextSymbolChurnSummary(vcssymbolsidecar.Result{LineCount: 578, AttributedLineCount: 511, AmbiguousLineCount: 1, UnmatchedLineCount: 66, OldAcquisition: []vcssymbolsidecar.FileOutcome{{Status: "COMPLETE"}, {Status: "FAILED"}}, NewAcquisition: []vcssymbolsidecar.FileOutcome{{Status: "EMPTY"}, {Status: "COMPLETE"}}})
	if summary["attribution_basis_points"] != 8840 || summary["line_count"] != 578 || summary["file_outcomes"].(map[string]int)["complete"] != 2 || summary["file_outcomes"].(map[string]int)["empty"] != 1 || summary["file_outcomes"].(map[string]int)["failed"] != 1 {
		t.Fatalf("ASSERT_SYMBOL_CHURN_COMPACT_SUMMARY_BALANCES: %+v", summary)
	}
}

func TestContextSymbolChurnCaptureDirectGatewayParity(t *testing.T) {
	executor := &unavailableSymbolChurnCaptureExecutor{}
	s := &Server{Registry: NewRegistryWithProfile(false, ToolProfileFull), Executors: map[ExecutorFamily]Executor{ContextSymbolChurnCaptureExecutorFamily: executor}}
	args := map[string]any{"session_id": "project", "generation": 1, "uri": "file:///tmp/repo/a.go", "symbol": "A", "down_depth": 1, "up_depth": 0, "max_nodes": 10, "timeout_ms": 1000, "request_timeout_ms": 500, "analysis": map[string]any{"kind": "NEIGHBORHOOD"}, "from_revision": "HEAD~1", "to_revision": "HEAD", "profile": "missing", "language_id": "go"}
	direct := s.callContext(context.Background(), response{JSONRPC: "2.0", ID: float64(1)}, mustCallParams(t, mcpcontract.ContextSymbolChurnCaptureTool, args))
	gateway := s.callContext(context.Background(), response{JSONRPC: "2.0", ID: float64(1)}, mustCallParams(t, "lsp_trace_v1_execute", map[string]any{"request": map[string]any{"operation": mcpcontract.ContextSymbolChurnCaptureTool, "arguments": args}}))
	if direct.Error != nil || gateway.Error != nil || len(executor.calls) != 2 {
		t.Fatalf("ASSERT_SYMBOL_CHURN_CAPTURE_DIRECT_GATEWAY_PARITY: direct=%+v gateway=%+v calls=%d", direct.Error, gateway.Error, len(executor.calls))
	}
	var a, b map[string]any
	_ = json.Unmarshal(executor.calls[0].Input, &a)
	_ = json.Unmarshal(executor.calls[1].Input, &b)
	if !reflect.DeepEqual(a, b) || executor.calls[0].Name != operation.Name("context_symbol_churn_capture") || executor.calls[1].Name != operation.Name("context_symbol_churn_capture") {
		t.Fatal("ASSERT_SYMBOL_CHURN_CAPTURE_DIRECT_GATEWAY_PARITY: dispatch inputs differ")
	}
}

func TestContextSymbolChurnCaptureDeliveryCheckTypedParity(t *testing.T) {
	const assertion = "ASSERT_SYMBOL_CHURN_CAPTURE_DELIVERY_CHECK_TYPED_PARITY"
	executor := &structuralContextRecordingExecutor{artifact: []byte(`{"not":"a churn artifact"}`)}
	s := &Server{Registry: NewRegistryWithProfile(false, ToolProfileFull), Executors: map[ExecutorFamily]Executor{ContextSymbolChurnCaptureExecutorFamily: executor}}
	args := map[string]any{"session_id": "project", "generation": 1, "uri": "file:///tmp/repo/a.go", "symbol": "A", "down_depth": 1, "up_depth": 0, "max_nodes": 10, "timeout_ms": 1000, "request_timeout_ms": 500, "analysis": map[string]any{"kind": "NEIGHBORHOOD"}, "from_revision": "HEAD~1", "to_revision": "HEAD", "profile": "go", "language_id": "go"}
	responses := []response{
		s.callContext(context.Background(), response{JSONRPC: "2.0", ID: float64(1)}, mustCallParams(t, mcpcontract.ContextSymbolChurnCaptureTool, args)),
		s.callContext(context.Background(), response{JSONRPC: "2.0", ID: float64(2)}, mustCallParams(t, "lsp_trace_v1_execute", map[string]any{"request": map[string]any{"operation": mcpcontract.ContextSymbolChurnCaptureTool, "arguments": args}})),
	}
	for i, got := range responses {
		if got.Error != nil {
			t.Fatalf("%s[%d]: unexpected JSON-RPC error: %+v", assertion, i, got.Error)
		}
		wrapped, _ := json.Marshal(got.Result)
		var decoded struct {
			StructuredContent json.RawMessage `json:"structuredContent"`
		}
		_ = json.Unmarshal(wrapped, &decoded)
		raw := decoded.StructuredContent
		if i == 1 {
			var gateway struct {
				DelegatedEnvelope string `json:"delegated_envelope"`
			}
			_ = json.Unmarshal(raw, &gateway)
			raw = []byte(gateway.DelegatedEnvelope)
		}
		if err := mcpcontract.ValidateContextSymbolChurnCaptureEnvelopeExclusive(raw); err != nil || !strings.Contains(string(raw), `"phase":"DELIVERY_CHECK"`) {
			t.Fatalf("%s[%d]: envelope=%s validation=%v", assertion, i, raw, err)
		}
	}
}

func TestContextSymbolChurnDirectGatewayDomainParity(t *testing.T) {
	executor := &unavailableSymbolChurnExecutor{}
	s := &Server{Registry: NewRegistryWithProfile(false, ToolProfileFull), Executors: map[ExecutorFamily]Executor{ContextSymbolChurnExecutorFamily: executor}}
	args := map[string]any{"input": "{}", "workspace": "/tmp/repo", "from_revision": "HEAD~1", "to_revision": "HEAD", "profile": "missing", "language_id": "go"}
	direct := s.callContext(context.Background(), response{JSONRPC: "2.0", ID: float64(1)}, mustCallParams(t, mcpcontract.ContextSymbolChurnTool, args))
	gateway := s.callContext(context.Background(), response{JSONRPC: "2.0", ID: float64(1)}, mustCallParams(t, "lsp_trace_v1_execute", map[string]any{"request": map[string]any{"operation": mcpcontract.ContextSymbolChurnTool, "arguments": args}}))
	if direct.Error != nil || gateway.Error != nil || len(executor.calls) != 2 {
		t.Fatalf("ASSERT_CONTEXT_SYMBOL_CHURN_DIRECT_GATEWAY_DOMAIN_PARITY: direct=%+v gateway=%+v calls=%d", direct.Error, gateway.Error, len(executor.calls))
	}
	var a, b map[string]any
	_ = json.Unmarshal(executor.calls[0].Input, &a)
	_ = json.Unmarshal(executor.calls[1].Input, &b)
	if !reflect.DeepEqual(a, b) || executor.calls[0].Name != operation.Name("context_symbol_churn") || executor.calls[1].Name != operation.Name("context_symbol_churn") {
		t.Fatal("ASSERT_CONTEXT_SYMBOL_CHURN_SHARED_DISPATCH")
	}
}
