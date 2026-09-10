package mcp

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"lsp-trace/internal/operation"
)

type selectorCountingExecutor struct {
	calls []operation.Request
}

func (e *selectorCountingExecutor) Execute(_ context.Context, request operation.Request) (operation.Result, *operation.Failure) {
	e.calls = append(e.calls, request)
	return operation.Result{Artifact: []byte(`{"$id":"https://jaresty.github.io/lsp-trace/schemas/lsp-trace.graph.v3.schema.json"}`)}, nil
}

func sliceSelectorResponse(t *testing.T, server *Server, tool string, arguments map[string]any) response {
	t.Helper()
	if tool == "lsp_trace_v1_execute" {
		arguments = map[string]any{"request": map[string]any{"tool": "lsp_trace_v1_slice", "arguments": arguments}}
	}
	return server.callContext(context.Background(), response{JSONRPC: "2.0", ID: float64(1)}, mustCallParams(t, tool, arguments))
}

func TestSliceDirectAndGatewaySelectorsReachIdenticalAcquisition(t *testing.T) {
	base := map[string]any{"session_id": "session", "generation": float64(1), "start_mode": "at", "uri": "file:///workspace/main.go"}
	for _, tc := range []struct {
		name     string
		selector map[string]any
	}{
		{"symbol", map[string]any{"symbol": "Target"}},
		{"position", map[string]any{"line": float64(4), "character": float64(7)}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var inputs []json.RawMessage
			for _, entry := range []string{"lsp_trace_v1_slice", "lsp_trace_v1_execute"} {
				executor := &selectorCountingExecutor{}
				server := &Server{Registry: NewRegistry(false), Executors: map[ExecutorFamily]Executor{SliceExecutorFamily: executor}}
				arguments := cloneMap(base)
				for key, value := range tc.selector {
					arguments[key] = value
				}
				got := sliceSelectorResponse(t, server, entry, arguments)
				if got.Error != nil || len(executor.calls) != 1 {
					t.Fatalf("ASSERT_SLICE_SELECTOR_REACHES_ACQUISITION_%s_%s: error=%v calls=%d", tc.name, entry, got.Error, len(executor.calls))
				}
				inputs = append(inputs, executor.calls[0].Input)
			}
			if !reflect.DeepEqual(inputs[0], inputs[1]) {
				t.Fatalf("ASSERT_SLICE_SELECTOR_DIRECT_GATEWAY_INPUT_PARITY_%s: direct=%s gateway=%s", tc.name, inputs[0], inputs[1])
			}
		})
	}
}

func TestSliceDirectAndGatewayRejectInvalidSelectorsBeforeAcquisition(t *testing.T) {
	base := map[string]any{"session_id": "session", "generation": float64(1), "start_mode": "at", "uri": "file:///workspace/main.go"}
	for _, tc := range []struct {
		name, diagnostic string
		selector         map[string]any
	}{
		{"missing-character", "character is required when line is provided", map[string]any{"line": float64(4)}},
		{"missing-line", "line is required when character is provided", map[string]any{"character": float64(7)}},
		{"mixed-symbol-position", "symbol is mutually exclusive with line and character", map[string]any{"symbol": "Target", "line": float64(4), "character": float64(7)}},
		{"missing-selector", "one target selector is required: symbol or line and character", map[string]any{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var diagnostics []string
			for _, entry := range []string{"lsp_trace_v1_slice", "lsp_trace_v1_execute"} {
				executor := &selectorCountingExecutor{}
				server := &Server{Registry: NewRegistry(false), Executors: map[ExecutorFamily]Executor{SliceExecutorFamily: executor}}
				arguments := cloneMap(base)
				for key, value := range tc.selector {
					arguments[key] = value
				}
				got := sliceSelectorResponse(t, server, entry, arguments)
				if got.Error == nil || got.Error.Code != -32602 || !strings.Contains(got.Error.Message, tc.diagnostic) || len(executor.calls) != 0 {
					t.Fatalf("ASSERT_SLICE_SELECTOR_REJECTED_BEFORE_ACQUISITION_%s_%s: error=%v calls=%d", tc.name, entry, got.Error, len(executor.calls))
				}
				diagnostics = append(diagnostics, got.Error.Message)
			}
			if diagnostics[0] != diagnostics[1] {
				t.Fatalf("ASSERT_SLICE_SELECTOR_DIRECT_GATEWAY_DIAGNOSTIC_PARITY_%s: %q != %q", tc.name, diagnostics[0], diagnostics[1])
			}
		})
	}
}
