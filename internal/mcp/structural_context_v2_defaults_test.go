package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"reflect"
	"strings"
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
		{"regex", map[string]any{"session_id": "s", "generation": float64(1), "regex_locator": map[string]any{"uri": "file:///workspace/a.go", "pattern": "A", "match_index": float64(0), "limits": map[string]any{}}}},
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
				if tc.name == "regex" {
					limits := got["regex_locator"].(map[string]any)["limits"].(map[string]any)
					// 60 KiB documents plus the full 4 KiB pattern allowance and up to
					// 100 counted matches fit within the smallest ordinary binary work
					// tier above the observed 59,531-unit single-file cases: 64 KiB.
					wantLimits := map[string]any{"max_document_bytes": float64(60 * 1024), "max_matches": float64(100), "max_pattern_bytes": float64(4 * 1024), "max_work": float64(64 * 1024)}
					if !reflect.DeepEqual(limits, wantLimits) {
						t.Fatalf("ASSERT_V2_REGEX_COHERENT_ORDINARY_DEFAULTS_call%d: got=%#v want=%#v", i, limits, wantLimits)
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
	locator := tool.InputSchema["properties"].(map[string]any)["regex_locator"].(map[string]any)
	limits := locator["properties"].(map[string]any)["limits"].(map[string]any)
	properties := limits["properties"].(map[string]any)
	for field, want := range map[string]struct{ defaultValue, maximum float64 }{
		"max_document_bytes": {60 * 1024, 16777216},
		"max_matches":        {100, 1000},
		"max_pattern_bytes":  {4 * 1024, 4096},
		"max_work":           {64 * 1024, 536870912},
	} {
		property := properties[field].(map[string]any)
		if property["default"] != want.defaultValue || property["maximum"] != want.maximum {
			t.Fatalf("ASSERT_V2_REGEX_DEFAULT_AND_HARD_MAX_%s: %#v", field, property)
		}
	}
	if nestedRequired, present := limits["required"]; present && len(nestedRequired.([]any)) != 0 {
		t.Fatalf("ASSERT_V2_REGEX_LIMITS_DEFAULTABLE: %#v", nestedRequired)
	}
}

func TestStructuralContextV2SourceOnlyProjectionOmissionDefaultsDirectAndGateway(t *testing.T) {
	executor := &structuralContextRecordingExecutor{artifact: unifiedStructuralContextV2Artifact()}
	server := &Server{Registry: NewRegistryWithProfile(false, ToolProfileCompact), Executors: map[ExecutorFamily]Executor{StructuralContextV2ExecutorFamily: executor}}
	minimal := map[string]any{
		"session_id": "s", "generation": float64(1), "uri": "file:///workspace/a.go", "line": float64(0), "character": float64(0),
		"up_depth": float64(0), "down_depth": float64(0),
		"projection": map[string]any{"mode": "TARGET", "body": "INCLUDE", "include_relation_occurrences": false, "privacy_policy_id": "public"},
	}
	direct := server.callContext(context.Background(), response{JSONRPC: "2.0", ID: float64(1)}, mustCallParams(t, mcpcontract.StructuralContextV2Tool, cloneStructuralContextArgs(t, minimal)))
	gateway := server.callContext(context.Background(), response{JSONRPC: "2.0", ID: float64(2)}, mustCallParams(t, "lsp_trace_v1_execute", map[string]any{"request": map[string]any{"operation": mcpcontract.StructuralContextV2Tool, "arguments": cloneStructuralContextArgs(t, minimal)}}))
	if direct.Error != nil || gateway.Error != nil || len(executor.calls) != 2 {
		t.Fatalf("ASSERT_SOURCE_ONLY_DEFAULTS_TRANSPORT: direct=%v gateway=%v calls=%d", direct.Error, gateway.Error, len(executor.calls))
	}
	wantLimits := map[string]any{
		"max_objects": float64(80), "max_ranges": float64(80), "max_source_bytes": float64(2097152), "max_work": float64(10000), "max_response_bytes": float64(8388608),
		"max_additional_documents": float64(20), "max_document_requests": float64(21), "max_document_bytes": float64(1048576), "max_total_document_bytes": float64(8388608),
		"max_document_messages": float64(32), "max_document_acquisition_work": float64(21), "max_display_resolution_work": float64(21),
	}
	for i, call := range executor.calls {
		var got map[string]any
		if err := json.Unmarshal(call.Input, &got); err != nil {
			t.Fatal(err)
		}
		projection := got["projection"].(map[string]any)
		if projection["body"] != "INCLUDE" || projection["privacy_policy_id"] != "public" || projection["include_ancillary"] != false || projection["display_range_policy"] != "FULL_DEFINITION" || !reflect.DeepEqual(projection["limits"], wantLimits) {
			t.Fatalf("ASSERT_SOURCE_ONLY_DEFAULTS_call%d: %#v", i, projection)
		}
	}
}

func TestStructuralContextV2SourceOnlyProjectionExplicitValuesPreserved(t *testing.T) {
	executor := &structuralContextRecordingExecutor{artifact: unifiedStructuralContextV2Artifact()}
	server := &Server{Registry: NewRegistryWithProfile(false, ToolProfileFull), Executors: map[ExecutorFamily]Executor{StructuralContextV2ExecutorFamily: executor}}
	limits := map[string]any{"max_objects": float64(1), "max_ranges": float64(2), "max_source_bytes": float64(3), "max_work": float64(4), "max_response_bytes": float64(4096), "max_additional_documents": float64(0), "max_document_requests": float64(1), "max_document_bytes": float64(6), "max_total_document_bytes": float64(7), "max_document_messages": float64(8), "max_document_acquisition_work": float64(9), "max_display_resolution_work": float64(10)}
	args := map[string]any{"session_id": "s", "generation": float64(1), "symbol": "A", "uri": "file:///workspace/a.go", "up_depth": float64(0), "down_depth": float64(0), "projection": map[string]any{"mode": "TARGET", "body": "OMIT", "include_relation_occurrences": false, "include_ancillary": true, "display_range_policy": "FULL_DEFINITION", "limits": limits, "privacy_policy_id": "private"}}
	response := server.callContext(context.Background(), response{JSONRPC: "2.0", ID: float64(1)}, mustCallParams(t, mcpcontract.StructuralContextV2Tool, args))
	if response.Error != nil || len(executor.calls) != 1 {
		t.Fatalf("ASSERT_SOURCE_ONLY_EXPLICIT_TRANSPORT: response=%v calls=%d", response.Error, len(executor.calls))
	}
	var got map[string]any
	_ = json.Unmarshal(executor.calls[0].Input, &got)
	projection := got["projection"].(map[string]any)
	if projection["include_ancillary"] != true || projection["privacy_policy_id"] != "private" || !reflect.DeepEqual(projection["limits"], limits) {
		t.Fatalf("ASSERT_SOURCE_ONLY_EXPLICIT_PRESERVED: %#v", projection)
	}
}

func TestStructuralContextV2SourceOnlyProjectionStrictAndScoped(t *testing.T) {
	server := &Server{Registry: NewRegistryWithProfile(false, ToolProfileFull)}
	for name, raw := range map[string]string{
		"unknown":   `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"lsp_trace_v2_structural_context","arguments":{"session_id":"s","generation":1,"uri":"file:///workspace/a.go","line":0,"character":0,"up_depth":0,"down_depth":0,"projection":{"mode":"TARGET","body":"OMIT","include_relation_occurrences":false,"privacy_policy_id":"public","unknown":true}}}}`,
		"duplicate": `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"lsp_trace_v2_structural_context","arguments":{"session_id":"s","generation":1,"uri":"file:///workspace/a.go","line":0,"character":0,"up_depth":0,"down_depth":0,"projection":{"mode":"TARGET","body":"OMIT","body":"INCLUDE","include_relation_occurrences":false,"privacy_policy_id":"public"}}}}`,
	} {
		t.Run(name, func(t *testing.T) {
			var out bytes.Buffer
			if err := server.Serve(strings.NewReader(raw+"\n"), &out); err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(out.String(), `"code":-32`) {
				t.Fatalf("ASSERT_SOURCE_ONLY_STRICT_%s: %s", name, out.String())
			}
		})
	}
	for name, projection := range map[string]map[string]any{
		"projected":   {"mode": "PROJECTED", "body": "OMIT", "include_relation_occurrences": false, "privacy_policy_id": "public"},
		"occurrences": {"mode": "TARGET", "body": "OMIT", "include_relation_occurrences": true, "privacy_policy_id": "public"},
	} {
		t.Run(name, func(t *testing.T) {
			args := map[string]any{"session_id": "s", "generation": float64(1), "uri": "file:///workspace/a.go", "line": float64(0), "character": float64(0), "up_depth": float64(0), "down_depth": float64(0), "projection": projection}
			response := server.callContext(context.Background(), response{JSONRPC: "2.0", ID: float64(1)}, mustCallParams(t, mcpcontract.StructuralContextV2Tool, args))
			if response.Error == nil || !strings.Contains(response.Error.Message, "limits") {
				t.Fatalf("ASSERT_SOURCE_ONLY_DEFAULTS_SCOPED_%s: %#v", name, response)
			}
		})
	}
}

func TestStructuralContextV2CompactAdvertisementBound(t *testing.T) {
	tool, ok := NewRegistryWithProfile(false, ToolProfileCompact).ResolveCanonical(mcpcontract.StructuralContextV2Tool)
	if !ok {
		t.Fatal("ASSERT_SOURCE_ONLY_COMPACT_TOOL")
	}
	raw, err := json.Marshal(tool.PresentationInputSchema)
	if err != nil || len(raw) > 64*1024 {
		t.Fatalf("ASSERT_SOURCE_ONLY_COMPACT_ADVERTISEMENT_BOUND: bytes=%d err=%v", len(raw), err)
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
