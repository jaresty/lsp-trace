package mcp

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

	"lsp-trace/internal/mcpcontract"
	"lsp-trace/internal/operation"
)

func TestContextChurnOperation38AppendOnlyContract(t *testing.T) {
	full := NewRegistryWithProfile(false, ToolProfileFull)
	compact := NewRegistryWithProfile(false, ToolProfileCompact)
	_, ok := full.ResolveCanonical(mcpcontract.ContextChurnTool)
	if !ok || len(full.Tools()) != 38 || len(full.Advertised()) != 38 || len(compact.Tools()) != 38 || len(compact.Advertised()) != 10 {
		t.Fatalf("ASSERT_CONTEXT_CHURN_OPERATION38_APPEND_ONLY: ok=%t full=%d/%d compact=%d/%d", ok, len(full.Tools()), len(full.Advertised()), len(compact.Tools()), len(compact.Advertised()))
	}
	if _, ok := compact.ResolveCanonical(mcpcontract.ContextChurnTool); !ok {
		t.Fatal("ASSERT_CONTEXT_CHURN_COMPACT_DISPATCHABLE")
	}
	for _, tool := range compact.Advertised() {
		if tool.Name == mcpcontract.ContextChurnTool {
			t.Fatal("ASSERT_CONTEXT_CHURN_COMPACT_HIDDEN")
		}
	}
}

func TestContextChurnDirectGatewayParity(t *testing.T) {
	artifact, _ := json.Marshal(validContextChurnResult())
	e := &structuralContextRecordingExecutor{artifact: artifact}
	s := &Server{Registry: NewRegistryWithProfile(false, ToolProfileFull), Executors: map[ExecutorFamily]Executor{ContextChurnExecutorFamily: e}}
	args := validContextChurnArgs()
	direct := s.callContext(context.Background(), response{JSONRPC: "2.0", ID: float64(1)}, mustCallParams(t, mcpcontract.ContextChurnTool, args))
	gateway := s.callContext(context.Background(), response{JSONRPC: "2.0", ID: float64(2)}, mustCallParams(t, "lsp_trace_v1_execute", map[string]any{"request": map[string]any{"operation": mcpcontract.ContextChurnTool, "arguments": args}}))
	if direct.Error != nil || gateway.Error != nil || len(e.calls) != 2 {
		t.Fatalf("ASSERT_CONTEXT_CHURN_TRANSPORT_PARITY: direct=%v gateway=%v calls=%d", direct.Error, gateway.Error, len(e.calls))
	}
	var a, b map[string]any
	_ = json.Unmarshal(e.calls[0].Input, &a)
	_ = json.Unmarshal(e.calls[1].Input, &b)
	if !reflect.DeepEqual(a, b) || e.calls[0].Name != operation.Name("context_churn") || e.calls[1].Name != operation.Name("context_churn") {
		t.Fatal("ASSERT_CONTEXT_CHURN_SHARED_DISPATCH")
	}
}

func TestContextChurnRejectsUnknownInputField(t *testing.T) {
	s := &Server{Registry: NewRegistryWithProfile(false, ToolProfileFull), Executors: map[ExecutorFamily]Executor{ContextChurnExecutorFamily: &structuralContextRecordingExecutor{}}}
	args := validContextChurnArgs()
	args["environment"] = map[string]any{"PATH": "/tmp"}
	r := s.callContext(context.Background(), response{JSONRPC: "2.0", ID: float64(1)}, mustCallParams(t, mcpcontract.ContextChurnTool, args))
	if r.Error == nil || r.Error.Code != -32602 {
		t.Fatalf("ASSERT_CONTEXT_CHURN_CLOSED_INPUT: %+v", r)
	}
}

func validContextChurnArgs() map[string]any {
	raw, _ := json.Marshal(validDeltaArgs()["before"])
	return map[string]any{"input": string(raw), "workspace": "/tmp/repo", "from_revision": "HEAD~1", "to_revision": "HEAD", "timeout_ms": 30000}
}

func validContextChurnResult() map[string]any {
	return map[string]any{
		"schema_version": "lsp-trace.vcs-churn-sidecar.v1", "authority": 0, "source_graph_complete": "UNKNOWN", "attribution": "FILE_PATH_ONLY",
		"graph_artifact_digest": "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "vcs": "git",
		"from_revision": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "to_revision": "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", "history_scope_complete": "COMPLETE",
		"node_count": 1, "changed_node_count": 0, "zero_churn_node_count": 1,
		"nodes": []any{map[string]any{"node_id": "tn_0123456789abcdef0123456789abcdef", "path": "src/a.go", "kind": 12, "name": "A", "commit_count": 0, "lines_added": 0, "lines_deleted": 0, "binary_changes": 0}},
	}
}
