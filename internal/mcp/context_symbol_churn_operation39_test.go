package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	"lsp-trace/internal/mcpcontract"
	"lsp-trace/internal/operation"
	"lsp-trace/internal/vcssymbolsidecar"
)

type unavailableSymbolChurnExecutor struct{ calls []operation.Request }

type unavailableSymbolChurnCaptureExecutor struct{ calls []operation.Request }

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
	if !ok || len(full.Tools()) != 41 || len(full.Advertised()) != 41 || len(compact.Tools()) != 41 || len(compact.Advertised()) != 11 {
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
	if !fullOK || !compactOK || len(full.Tools()) != 41 || len(full.Advertised()) != 41 || len(compact.Tools()) != 41 || len(compact.Advertised()) != 11 {
		t.Fatalf("ASSERT_SYMBOL_CHURN_OPERATION40_APPEND_ONLY: full_ok=%t compact_ok=%t full=%d/%d compact=%d/%d", fullOK, compactOK, len(full.Tools()), len(full.Advertised()), len(compact.Tools()), len(compact.Advertised()))
	}
	for _, candidate := range compact.Advertised() {
		if candidate.Name == tool {
			t.Fatal("ASSERT_SYMBOL_CHURN_OPERATION40_APPEND_ONLY: compact profile advertised operation 40")
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
