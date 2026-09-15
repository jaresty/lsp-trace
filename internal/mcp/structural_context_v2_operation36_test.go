package mcp

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

	"lsp-trace/internal/mcpcontract"
	"lsp-trace/internal/operation"
)

func TestStructuralContextV2Operation36Contract(t *testing.T) {
	full := NewRegistryWithProfile(false, ToolProfileFull)
	compact := NewRegistryWithProfile(false, ToolProfileCompact)
	tool, ok := full.ResolveCanonical(mcpcontract.StructuralContextV2Tool)
	if !ok || len(full.Tools()) != 36 || len(full.Advertised()) != 36 || len(compact.Tools()) != 36 || len(compact.Advertised()) != 10 {
		t.Fatalf("ASSERT_STRUCTURAL_CONTEXT_V2_OPERATION36_APPEND_ONLY: ok=%t full=%d/%d compact=%d/%d", ok, len(full.Tools()), len(full.Advertised()), len(compact.Tools()), len(compact.Advertised()))
	}
	if _, ok := compact.ResolveCanonical(mcpcontract.StructuralContextV2Tool); !ok {
		t.Fatal("ASSERT_STRUCTURAL_CONTEXT_V2_COMPACT_DISPATCHABLE")
	}
	for _, advertised := range compact.Advertised() {
		if advertised.Name == mcpcontract.StructuralContextV2Tool {
			t.Fatal("ASSERT_STRUCTURAL_CONTEXT_V2_COMPACT_HIDDEN")
		}
	}
	for _, field := range []string{"source_body", "absolute_path", "file_uri", "environment", "commands", "provider_internals", "publication", "retained_input", "hydration", "custody", "source_supply"} {
		bad := structuralContextArgs()
		bad[field] = "forbidden"
		if validateArguments(tool, bad) == nil {
			t.Fatalf("ASSERT_STRUCTURAL_CONTEXT_V2_REJECTS_%s", field)
		}
	}
}

func TestStructuralContextV2DirectGatewayParity(t *testing.T) {
	artifact := []byte(`{"schema_version":"lsp-trace.transient-structural-result.v2","authority":0,"source_graph_complete":"UNKNOWN","position_encoding":"utf-16","target_node_id":"tn_0123456789abcdef0123456789abcdef","nodes":[{"node_id":"tn_0123456789abcdef0123456789abcdef","name":"A","kind":12,"path":"src/a.go","declaration_range":{"start":{"line":0,"character":0},"end":{"line":0,"character":1}}}],"calls":[]}`)
	executor := &structuralContextRecordingExecutor{artifact: artifact}
	server := &Server{Registry: NewRegistryWithProfile(false, ToolProfileFull), Executors: map[ExecutorFamily]Executor{StructuralContextV2ExecutorFamily: executor}}
	args := structuralContextArgs()
	direct := server.callContext(context.Background(), response{JSONRPC: "2.0", ID: float64(1)}, mustCallParams(t, mcpcontract.StructuralContextV2Tool, args))
	gateway := server.callContext(context.Background(), response{JSONRPC: "2.0", ID: float64(2)}, mustCallParams(t, "lsp_trace_v1_execute", map[string]any{"request": map[string]any{"operation": mcpcontract.StructuralContextV2Tool, "arguments": args}}))
	if direct.Error != nil || gateway.Error != nil || len(executor.calls) != 2 {
		t.Fatalf("ASSERT_STRUCTURAL_CONTEXT_V2_DIRECT_GATEWAY: direct=%v gateway=%v calls=%d", direct.Error, gateway.Error, len(executor.calls))
	}
	var a, b map[string]any
	_ = json.Unmarshal(executor.calls[0].Input, &a)
	_ = json.Unmarshal(executor.calls[1].Input, &b)
	if !reflect.DeepEqual(a, b) || executor.calls[0].Name != operation.Name("structural_context_v2") || executor.calls[1].Name != operation.Name("structural_context_v2") {
		t.Fatal("ASSERT_STRUCTURAL_CONTEXT_V2_SHARED_DISPATCH")
	}
}
