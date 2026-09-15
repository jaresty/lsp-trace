package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	"lsp-trace/internal/mcpcontract"
	"lsp-trace/internal/operation"
)

type unavailableSymbolChurnExecutor struct{ calls []operation.Request }

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
	if !ok || len(full.Tools()) != 39 || len(full.Advertised()) != 39 || len(compact.Tools()) != 39 || len(compact.Advertised()) != 10 {
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
