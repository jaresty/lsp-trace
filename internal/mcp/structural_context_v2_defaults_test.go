package mcp

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

	"lsp-trace/internal/mcpcontract"
)

func cloneStructuralContextArgs(t *testing.T, args map[string]any) map[string]any {
	t.Helper()
	raw, err := json.Marshal(args)
	if err != nil {
		t.Fatal(err)
	}
	var clone map[string]any
	if err := json.Unmarshal(raw, &clone); err != nil {
		t.Fatal(err)
	}
	return clone
}

func TestStructuralContextV2OmissionDefaultsDirectAndGateway(t *testing.T) {
	minimal := []struct {
		name string
		args map[string]any
	}{
		{"symbol", map[string]any{"session_id": "s", "generation": float64(1), "symbol": "A"}},
		{"position", map[string]any{"session_id": "s", "generation": float64(1), "uri": "file:///workspace/a.go", "line": float64(0), "character": float64(0)}},
		{"regex", map[string]any{"session_id": "s", "generation": float64(1), "regex_locator": map[string]any{"uri": "file:///workspace/a.go", "pattern": "A", "match_index": float64(0), "limits": map[string]any{"max_document_bytes": float64(1), "max_matches": float64(1), "max_pattern_bytes": float64(1), "max_work": float64(1)}}}},
	}
	want := map[string]any{"analysis": map[string]any{"kind": "NEIGHBORHOOD"}, "up_depth": float64(1), "down_depth": float64(1), "max_nodes": float64(100), "timeout_ms": float64(30000), "request_timeout_ms": float64(15000), "max_messages": float64(64), "max_bytes": float64(4194304)}
	for _, tc := range minimal {
		t.Run(tc.name, func(t *testing.T) {
			executor := &structuralContextRecordingExecutor{artifact: unifiedStructuralContextV2Artifact()}
			server := &Server{Registry: NewRegistryWithProfile(false, ToolProfileFull), Executors: map[ExecutorFamily]Executor{StructuralContextV2ExecutorFamily: executor}}
			original := cloneStructuralContextArgs(t, tc.args)
			directArgs := cloneStructuralContextArgs(t, tc.args)
			gatewayArgs := cloneStructuralContextArgs(t, tc.args)
			for _, args := range []map[string]any{directArgs, gatewayArgs} {
				for field := range want {
					if _, present := args[field]; present {
						t.Fatalf("ASSERT_V2_OMISSION_INPUT_%s: %#v", field, args)
					}
				}
			}
			direct := server.callContext(context.Background(), response{JSONRPC: "2.0", ID: float64(1)}, mustCallParams(t, mcpcontract.StructuralContextV2Tool, directArgs))
			gateway := server.callContext(context.Background(), response{JSONRPC: "2.0", ID: float64(2)}, mustCallParams(t, "lsp_trace_v1_execute", map[string]any{"request": map[string]any{"operation": mcpcontract.StructuralContextV2Tool, "arguments": gatewayArgs}}))
			if !reflect.DeepEqual(tc.args, original) {
				t.Fatalf("ASSERT_V2_CALLER_MAP_UNCHANGED: got=%#v want=%#v", tc.args, original)
			}
			if direct.Error != nil || gateway.Error != nil || len(executor.calls) != 2 {
				t.Fatalf("ASSERT_V2_DEFAULTS_TRANSPORT: direct=%v gateway=%v calls=%d", direct.Error, gateway.Error, len(executor.calls))
			}
			for i, call := range executor.calls {
				var got map[string]any
				if err := json.Unmarshal(call.Input, &got); err != nil {
					t.Fatal(err)
				}
				for field, value := range want {
					if !reflect.DeepEqual(got[field], value) {
						t.Fatalf("ASSERT_V2_DEFAULT_%s_call%d: got=%#v want=%#v", field, i, got[field], value)
					}
				}
			}
		})
	}
}

func TestStructuralContextV2AdvertisesOmissionDefaults(t *testing.T) {
	tool, ok := NewRegistryWithProfile(false, ToolProfileFull).ResolveCanonical(mcpcontract.StructuralContextV2Tool)
	if !ok {
		t.Fatal("ASSERT_V2_DEFAULTS_TOOL")
	}
	required := tool.InputSchema["required"].([]any)
	for _, field := range []string{"analysis", "up_depth", "down_depth", "max_nodes", "timeout_ms", "request_timeout_ms", "max_messages", "max_bytes"} {
		for _, name := range required {
			if name == field {
				t.Fatalf("ASSERT_V2_DEFAULTS_NOT_REQUIRED_%s", field)
			}
		}
		property := tool.InputSchema["properties"].(map[string]any)[field].(map[string]any)
		if property["default"] == nil {
			t.Fatalf("ASSERT_V2_DEFAULTS_ANNOTATED_%s", field)
		}
	}
}

func TestStructuralContextV2InvalidExplicitValuesRemainRejected(t *testing.T) {
	server := &Server{Registry: NewRegistryWithProfile(false, ToolProfileFull)}
	for name, args := range map[string]map[string]any{
		"zero-max-nodes":   {"session_id": "s", "generation": float64(1), "symbol": "A", "max_nodes": float64(0)},
		"timeout-relation": {"session_id": "s", "generation": float64(1), "symbol": "A", "timeout_ms": float64(1000), "request_timeout_ms": float64(1001)},
	} {
		t.Run(name, func(t *testing.T) {
			response := server.callContext(context.Background(), response{JSONRPC: "2.0", ID: float64(1)}, mustCallParams(t, mcpcontract.StructuralContextV2Tool, args))
			if response.Error == nil || response.Error.Code != -32602 {
				t.Fatalf("ASSERT_V2_INVALID_EXPLICIT_%s: %#v", name, response)
			}
		})
	}
}

func TestStructuralContextV2ExplicitValuesRemainEffective(t *testing.T) {
	executor := &structuralContextRecordingExecutor{artifact: unifiedStructuralContextV2Artifact()}
	server := &Server{Registry: NewRegistryWithProfile(false, ToolProfileFull), Executors: map[ExecutorFamily]Executor{StructuralContextV2ExecutorFamily: executor}}
	args := map[string]any{"session_id": "s", "generation": float64(1), "symbol": "A", "up_depth": float64(0), "down_depth": float64(0), "max_nodes": float64(7), "timeout_ms": float64(2000), "request_timeout_ms": float64(1000), "max_messages": float64(8), "max_bytes": float64(1024), "analysis": map[string]any{"kind": "NEIGHBORHOOD"}}
	response := server.callContext(context.Background(), response{JSONRPC: "2.0", ID: float64(1)}, mustCallParams(t, mcpcontract.StructuralContextV2Tool, args))
	if response.Error != nil || len(executor.calls) != 1 {
		t.Fatalf("ASSERT_V2_EXPLICIT_EFFECTIVE: response=%v calls=%d", response.Error, len(executor.calls))
	}
	var got map[string]any
	_ = json.Unmarshal(executor.calls[0].Input, &got)
	for field, want := range map[string]any{"up_depth": float64(0), "down_depth": float64(0), "max_nodes": float64(7), "timeout_ms": float64(2000), "request_timeout_ms": float64(1000), "max_messages": float64(8), "max_bytes": float64(1024)} {
		if got[field] != want {
			t.Fatalf("ASSERT_V2_EXPLICIT_%s: got=%v want=%v", field, got[field], want)
		}
	}
}
