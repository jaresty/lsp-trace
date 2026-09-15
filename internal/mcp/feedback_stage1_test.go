package mcp

import (
	"reflect"
	"strings"
	"testing"
)

type compactConformanceCase struct {
	name string
	args map[string]any
}

func compactConformanceCases() []compactConformanceCase {
	return []compactConformanceCase{
		{"lsp_session_v1_derive_workspace", map[string]any{"session_id": "missing", "generation": 1, "workspace_uri": "file:///missing"}},
		{"lsp_session_v1_list", map[string]any{}},
		{"lsp_session_v1_restart", map[string]any{"session_id": "missing", "caller_id": "test"}},
		{"lsp_session_v1_status", map[string]any{"session_id": "missing"}},
		{"lsp_session_v1_stop", map[string]any{"session_id": "missing", "caller_id": "test"}},
		{"lsp_trace_v1_capabilities", map[string]any{}},
		{"lsp_trace_v1_execute", map[string]any{"request": map[string]any{"operation": "lsp_trace_v1_capabilities", "arguments": map[string]any{}}}},
		{"lsp_trace_v1_incoming", map[string]any{"session_id": "missing", "uri": "file:///x.go", "symbol": "X"}},
		{"lsp_trace_v1_inspect_hydrated", map[string]any{"input": `{}`}},
		{"lsp_trace_v1_schema_get", map[string]any{"schema": map[string]any{"family": "graph", "version": "v1"}}},
		{"lsp_trace_v1_slice", map[string]any{"session_id": "missing", "start_mode": "at", "uri": "file:///x.go", "symbol": "X"}},
		{"lsp_trace_v1_structural_context_symbol", map[string]any{"session_id": "missing", "generation": 1, "symbol": "X", "down_depth": 1, "up_depth": 1, "max_nodes": 10, "timeout_ms": 1000, "request_timeout_ms": 500, "analysis": map[string]any{"kind": "NEIGHBORHOOD"}}},
	}
}

func TestCompactDirectToolGeneratedMinimalConformance(t *testing.T) {
	cases := compactConformanceCases()
	if len(cases) != 12 {
		t.Fatalf("ASSERT_COMPACT_CASE_DECLARATIONS_EXACT: %d", len(cases))
	}
	r := NewRegistryWithProfile(false, ToolProfileCompact)
	if len(r.Advertised()) != 12 || len(r.tools) != 42 {
		t.Fatalf("ASSERT_REGISTRY_COUNTS: advertised=%d operations=%d", len(r.Advertised()), len(r.tools))
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tool, ok := r.ResolveCanonical(tc.name)
			if !ok {
				t.Fatalf("ASSERT_DECLARED_TOOL_RESOLVES")
			}
			if err := validateArguments(tool, tc.args); err != nil {
				t.Fatalf("ASSERT_MINIMAL_PASSES_ADVERTISED_SCHEMA: %v", err)
			}
			if tc.name == "lsp_trace_v1_execute" {
				return
			}
			directExec := matrixExecutor(r)
			direct := directMatrixCall(matrixServer(r, directExec), tc.name, tc.args)
			wrappedExec := matrixExecutor(r)
			wrapped := runServerMessages(t, matrixServer(r, wrappedExec), callMessage("lsp_trace_v1_execute", map[string]any{"request": map[string]any{"operation": tc.name, "arguments": tc.args}}))[0]
			if wrapped["error"] != nil {
				t.Fatalf("ASSERT_EXECUTE_REACHES_DOMAIN: %v", wrapped["error"])
			}
			if direct.Error != nil {
				t.Fatalf("ASSERT_DIRECT_REACHES_DOMAIN: %v", direct.Error)
			}
			if len(directExec.calls) != len(wrappedExec.calls) || (len(directExec.calls) == 1 && !reflect.DeepEqual(directExec.calls[0].Input, wrappedExec.calls[0].Input)) {
				t.Fatalf("ASSERT_DIRECT_EXECUTE_PARITY: direct=%v execute=%v", directExec.calls, wrappedExec.calls)
			}
		})
	}
}

func TestHydratedInlineInputBoundaryDirectExecuteParity(t *testing.T) {
	r := NewRegistry(false)
	direct, ok := r.ResolveCanonical("lsp_trace_v1_inspect_hydrated")
	if !ok {
		t.Fatal("ASSERT_HYDRATED_INLINE_LIMIT_TOOL")
	}
	for _, tc := range []struct {
		name      string
		size      int
		wantError bool
	}{
		{name: "equality", size: 1 << 20, wantError: false},
		{name: "plus-one", size: (1 << 20) + 1, wantError: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			arguments := map[string]any{"input": strings.Repeat("x", tc.size)}
			directExec := matrixExecutor(r)
			directResponse := directMatrixCall(matrixServer(r, directExec), direct.Name, arguments)
			executeExec := matrixExecutor(r)
			executeResponse := runServerMessages(t, matrixServer(r, executeExec), callMessage("lsp_trace_v1_execute", map[string]any{"request": map[string]any{"operation": direct.Name, "arguments": arguments}}))[0]
			directFailed := directResponse.Error != nil
			executeFailed := executeResponse["error"] != nil
			if directFailed != tc.wantError || executeFailed != tc.wantError || len(directExec.calls) != len(executeExec.calls) {
				t.Fatalf("ASSERT_HYDRATED_INLINE_LIMIT_DIRECT_EXECUTE_PARITY_%s: direct=%v execute=%v direct_calls=%d execute_calls=%d", tc.name, directResponse.Error, executeResponse["error"], len(directExec.calls), len(executeExec.calls))
			}
		})
	}
}

func TestExecuteCanonicalShapeAndLegacyDenial(t *testing.T) {
	r := NewRegistry(false)
	s := matrixServer(r, matrixExecutor(r))
	legacy := runServerMessages(t, s, callMessage("lsp_trace_v1_execute", map[string]any{"request": map[string]any{"tool": "lsp_trace_v1_capabilities", "arguments": map[string]any{}}}))[0]
	if legacy["error"] == nil || !strings.Contains(legacy["error"].(map[string]any)["message"].(string), "Invalid tool arguments") {
		t.Fatalf("ASSERT_LEGACY_TOOL_DENIED: %v", legacy)
	}
	canonical := runServerMessages(t, s, callMessage("lsp_trace_v1_execute", map[string]any{"request": map[string]any{"operation": "lsp_trace_v1_capabilities", "arguments": map[string]any{}}}))[0]
	if canonical["error"] != nil {
		t.Fatalf("ASSERT_CANONICAL_EXECUTE_ACCEPTED: %v", canonical)
	}
}
