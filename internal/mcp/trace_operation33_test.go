package mcp

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

	"lsp-trace/internal/mcpcontract"
	"lsp-trace/internal/operation"
)

type traceRecordingExecutor struct{ calls []operation.Request }

func (e *traceRecordingExecutor) Execute(_ context.Context, r operation.Request) (operation.Result, *operation.Failure) {
	e.calls = append(e.calls, r)
	return operation.Result{Artifact: []byte(`{"schema_version":"lsp-trace.graph-provenance.v5"}`)}, nil
}

func TestTraceOperation33ProfilesSchemaAndExecuteParity(t *testing.T) {
	full := NewRegistryWithProfile(false, ToolProfileFull)
	compact := NewRegistryWithProfile(false, ToolProfileCompact)
	tool, ok := full.ResolveCanonical(mcpcontract.TraceTool)
	if !ok || len(full.Tools()) != 33 || len(full.Advertised()) != 33 || len(compact.Tools()) != 33 || len(compact.Advertised()) != 10 {
		t.Fatalf("ASSERT_TRACE_OPERATION_33_APPEND_ONLY: ok=%t full=%d/%d compact=%d/%d", ok, len(full.Tools()), len(full.Advertised()), len(compact.Tools()), len(compact.Advertised()))
	}
	if tool.InputSchemaID != mcpcontract.TraceInputID || tool.ExecutorFamily != TraceExecutorFamily || len(tool.ArtifactSchemaIDs) != 1 || tool.ArtifactSchemaIDs[0] != mcpcontract.GraphProvenanceV5ArtifactID {
		t.Fatalf("ASSERT_TRACE_V1_SCHEMA_GRAPH_V5_INDEPENDENCE: %+v", tool)
	}
	for _, profile := range []ToolProfile{ToolProfileDefault, ToolProfileAdvanced, ToolProfileFull} {
		names := toolNames(NewRegistryWithProfile(false, profile).Advertised())
		found := false
		for _, n := range names {
			found = found || n == mcpcontract.TraceTool
		}
		if !found {
			t.Fatalf("ASSERT_TRACE_PROFILE_ADVERTISED_%s: %v", profile, names)
		}
	}
	for _, n := range toolNames(compact.Advertised()) {
		if n == mcpcontract.TraceTool {
			t.Fatal("ASSERT_TRACE_COMPACT_EXACT10")
		}
	}
	valid := map[string]any{"session_id": "s", "uri": "file:///w/a.go", "positions": []any{map[string]any{"line": float64(0), "character": float64(1)}}}
	if err := validateArguments(tool, valid); err != nil {
		t.Fatalf("ASSERT_TRACE_DIRECT_SCHEMA_VALID: %v", err)
	}
	zeroDepth := map[string]any{"session_id": "s", "uri": "file:///w/a.go", "positions": []any{map[string]any{"line": float64(0), "character": float64(1)}}, "down_depth": float64(0), "up_depth": float64(0)}
	if err := validateArguments(tool, zeroDepth); err != nil {
		t.Fatalf("ASSERT_TRACE_SCHEMA_ZERO_DEPTH_VALID: %v", err)
	}
	for _, bad := range []map[string]any{{"session_id": "s", "uri": "file:///w/a.go", "symbol": "A", "positions": []any{map[string]any{"line": 0, "character": 0}}}, {"session_id": "s", "uri": "file:///w/a.go", "positions": []any{}}, {"session_id": "s", "uri": "file:///w/a.go", "symbol": "A", "unknown": true}, {"session_id": "s", "uri": "file:///w/a.go", "symbol": "A", "down_depth": float64(65)}} {
		if validateArguments(tool, bad) == nil {
			t.Fatalf("ASSERT_TRACE_STRICT_TARGET_SCHEMA_REJECTS: %#v", bad)
		}
	}
	executor := &traceRecordingExecutor{}
	server := &Server{Registry: full, Executors: map[ExecutorFamily]Executor{TraceExecutorFamily: executor}}
	direct := server.callContext(context.Background(), response{JSONRPC: "2.0", ID: float64(1)}, mustCallParams(t, mcpcontract.TraceTool, valid))
	gateway := server.callContext(context.Background(), response{JSONRPC: "2.0", ID: float64(2)}, mustCallParams(t, "lsp_trace_v1_execute", map[string]any{"request": map[string]any{"operation": mcpcontract.TraceTool, "arguments": valid}}))
	if direct.Error != nil || gateway.Error != nil || len(executor.calls) != 2 || executor.calls[0].Name != "trace" || executor.calls[1].Name != "trace" || len(executor.calls[0].RetainedSeedSpec) != 0 || len(executor.calls[1].RetainedSeedSpec) != 0 {
		t.Fatalf("ASSERT_TRACE_DIRECT_EXECUTE_PARITY_ZERO_SEED_CUSTODY: direct=%v gateway=%v calls=%+v", direct.Error, gateway.Error, executor.calls)
	}
	var a, b map[string]any
	_ = json.Unmarshal(executor.calls[0].Input, &a)
	_ = json.Unmarshal(executor.calls[1].Input, &b)
	if !reflect.DeepEqual(a, b) {
		t.Fatalf("ASSERT_TRACE_DIRECT_EXECUTE_ARGUMENT_PARITY: %#v %#v", a, b)
	}
}
