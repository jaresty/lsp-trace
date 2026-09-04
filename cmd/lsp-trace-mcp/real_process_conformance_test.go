package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"lsp-trace/internal/provider"
)

const productionProviderContractGap = "production MCP bootstrap has no host-provisioned relation-provider registry/collector boundary; composeHostSelectorRuntime injects only the LSP session runtime"

func TestRealProviderProcessTransportConformance(t *testing.T) {
	binary := buildBinary(t, "fake-relation-provider", "./cmd/lsp-trace-mcp/testdata/fake-relation-provider")
	request := json.RawMessage(`{"relations":["PASSES_CALLBACK"]}`)
	limits := provider.Limits{RequestBytes: 4096, ResponseBytes: 4096, ProtocolMessages: 1, StderrBytes: 128, WallTime: time.Second, TerminationGrace: 50 * time.Millisecond}
	run := func(t *testing.T, id, mode string, ctx context.Context, configured provider.Limits) provider.Receipt {
		t.Helper()
		registry := provider.NewRegistry()
		start, exit := filepath.Join(t.TempDir(), "start"), filepath.Join(t.TempDir(), "exit")
		env := []string{"LSP_TRACE_PROVIDER_MODE=" + mode, "LSP_TRACE_PROVIDER_START_MARKER=" + start, "LSP_TRACE_PROVIDER_EXIT_MARKER=" + exit}
		if err := registry.Register(provider.Registration{ID: id, Path: binary, Env: env}); err != nil {
			t.Fatal(err)
		}
		return provider.NewRuntime(registry).Execute(ctx, id, request, configured)
	}

	t.Run("ASSERT_REAL_PROVIDER_SUCCESS_SCHEMA_AND_REPLAY", func(t *testing.T) {
		a := run(t, "fake@1", "success", context.Background(), limits)
		b := run(t, "fake@1", "success", context.Background(), limits)
		if a.Failure != nil || b.Failure != nil || !a.Reaped || !b.Reaped || !json.Valid(a.Response) || !bytes.Equal(a.Response, b.Response) {
			t.Fatalf("ASSERT_REAL_PROVIDER_SUCCESS_SCHEMA_AND_REPLAY: a=%+v b=%+v", a, b)
		}
		t.Log("PASS ASSERT_REAL_PROVIDER_SUCCESS_SCHEMA_AND_REPLAY")
	})
	for _, tc := range []struct {
		name, mode string
		kind       provider.FailureKind
	}{
		{"ASSERT_REAL_PROVIDER_MALFORMED_FRAME_REAP", "malformed-frame", provider.ProtocolFailed},
		{"ASSERT_REAL_PROVIDER_MALFORMED_PAYLOAD_REAP", "malformed-payload", provider.ProtocolFailed},
		{"ASSERT_REAL_PROVIDER_OUTPUT_BOUND_REAP", "oversized", provider.LimitExceeded},
		{"ASSERT_REAL_PROVIDER_TIMEOUT_TERMINATION_REAP", "hang", provider.TimedOut},
	} {
		t.Run(tc.name, func(t *testing.T) {
			configured := limits
			if tc.mode == "hang" {
				configured.WallTime = 25 * time.Millisecond
			}
			r := run(t, "fake@1", tc.mode, context.Background(), configured)
			ended := r.Reaped && (r.Terminated || r.ExitCode >= 0)
			if r.Failure == nil || r.Failure.Kind != tc.kind || !ended {
				t.Fatalf("%s: receipt=%+v", tc.name, r)
			}
			t.Log("PASS " + tc.name)
		})
	}
	t.Run("ASSERT_REAL_PROVIDER_CANCELLATION_TERMINATION_REAP", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		go func() { time.Sleep(20 * time.Millisecond); cancel() }()
		r := run(t, "fake@1", "hang", ctx, limits)
		if r.Failure == nil || r.Failure.Kind != provider.Canceled || !r.Terminated || !r.Reaped {
			t.Fatalf("ASSERT_REAL_PROVIDER_CANCELLATION_TERMINATION_REAP: receipt=%+v", r)
		}
		t.Log("PASS ASSERT_REAL_PROVIDER_CANCELLATION_TERMINATION_REAP")
	})
	t.Run("ASSERT_REAL_PROVIDER_UNKNOWN_SELECTOR_ZERO_START", func(t *testing.T) {
		registry := provider.NewRegistry()
		r := provider.NewRuntime(registry).Execute(context.Background(), "unknown", request, limits)
		if r.Failure == nil || r.Failure.Kind != provider.ProviderUnknown || r.Terminated || r.Reaped {
			t.Fatalf("ASSERT_REAL_PROVIDER_UNKNOWN_SELECTOR_ZERO_START: receipt=%+v", r)
		}
		t.Log("PASS ASSERT_REAL_PROVIDER_UNKNOWN_SELECTOR_ZERO_START")
	})
}

func TestProductionMCPPublishedConformance(t *testing.T) {
	binary := buildMCPBinary(t)
	artifact := json.RawMessage(graphFixture(t))
	requests := []map[string]any{
		{"jsonrpc": "2.0", "id": 1, "method": "tools/list", "params": map[string]any{}},
		callRequest(2, "lsp_trace_v1_schema_get", map[string]any{"schema": map[string]any{"family": "graph", "version": "v3"}}),
		callRequest(3, "lsp_trace_v1_validate", map[string]any{"input": artifact, "schema": map[string]any{"family": "graph", "version": "v3"}}),
		callRequest(4, "lsp_trace_v1_schema_get", map[string]any{"schema": map[string]any{"family": "graph", "version": "v3"}, "output_selector": "../unsafe.json"}),
	}
	responses := runMCPProcess(t, binary, nil, requests)
	tools := responses[0]["result"].(map[string]any)["tools"].([]any)
	if len(tools) != 13 {
		t.Fatalf("ASSERT_PRODUCTION_MCP_TOOL_COUNT_UNCHANGED: got=%d", len(tools))
	}
	for i, assertion := range []string{"ASSERT_PRODUCTION_MCP_SCHEMA_RETRIEVAL", "ASSERT_PRODUCTION_MCP_SCHEMA_VALIDATION"} {
		call := decodeProcessCall(t, responses[i+1])
		if call.env["operation_status"] != "SUCCEEDED" {
			t.Fatalf("%s: envelope=%v", assertion, call.env)
		}
		t.Log("PASS " + assertion)
	}
	unsafe := decodeProcessCall(t, responses[3])
	if unsafe.env["code"] != "OUTPUT_SELECTOR_UNSAFE" {
		t.Fatalf("ASSERT_PRODUCTION_MCP_UNKNOWN_SELECTOR: envelope=%v", unsafe.env)
	}
	report, err := os.ReadFile(filepath.Join(conformanceRepositoryRoot(t), "qualification", "retained", "provider-qualification", "report.json"))
	if err != nil || !bytes.Contains(report, []byte(`"id": "glint"`)) || !bytes.Contains(report, []byte(`"outcome": "BLOCKED"`)) {
		t.Fatalf("ASSERT_GLINT_REMAINS_BLOCKED: err=%v report=%s", err, report)
	}
	t.Log("PASS ASSERT_PRODUCTION_MCP_TOOL_COUNT_UNCHANGED")
	t.Log("PASS ASSERT_PRODUCTION_MCP_UNKNOWN_SELECTOR")
	t.Log("PASS ASSERT_GLINT_REMAINS_BLOCKED")
}

func TestProductionMCPRealProviderConformance(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Fatalf("ASSERT_PRODUCTION_PROVIDER_PLATFORM: skip-free production process conformance requires supported LocalDarwinSupervisor")
	}
	mcpBinary := buildMCPBinary(t)
	fakeLSP := buildBinary(t, "fake-lsp", "./cmd/fake-lsp")
	workspace := t.TempDir()
	configPath := filepath.Join(t.TempDir(), "bootstrap.json")
	config := map[string]any{"version": 1, "processes": []any{map[string]any{"alias": "fixture", "profile": map[string]any{"trust_domain": "real-provider-conformance", "workspace": workspace, "profile": "fake-lsp", "environment_reference": "hermetic"}, "execution": map[string]any{"path": fakeLSP, "directory": workspace}}}}
	raw, _ := json.Marshal(config)
	if err := os.WriteFile(configPath, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	base := map[string]any{"session_id": "fixture", "generation": 1, "uri": "file:///fixture/main.go", "line": 0, "character": 0, "max_depth": 2, "max_nodes": 20, "timeout_ms": 1000, "request_timeout_ms": 500}

	t.Run("ASSERT_PRODUCTION_MCP_INCOMING_NONCALLS_REAL_PROVIDER", func(t *testing.T) {
		request := cloneMap(base)
		request["relations"] = []string{"PASSES_CALLBACK"}
		request["providers"] = []string{"fake@1"}
		response := runMCPProcess(t, mcpBinary, []string{"--bootstrap-config", configPath}, []map[string]any{callRequest(1, "lsp_trace_v1_incoming", request)})[0]
		call := decodeProcessCall(t, response)
		if call.env["operation_status"] != "SUCCEEDED" {
			t.Fatalf("ASSERT_PRODUCTION_MCP_INCOMING_NONCALLS_REAL_PROVIDER: %s; envelope=%v", productionProviderContractGap, call.env)
		}
		t.Log("PASS ASSERT_PRODUCTION_MCP_INCOMING_NONCALLS_REAL_PROVIDER")
	})
	t.Run("ASSERT_PRODUCTION_MCP_SLICE_NONCALLS_REAL_PROVIDER", func(t *testing.T) {
		request := cloneMap(base)
		request["start_mode"] = "at"
		request["up_depth"] = 2
		request["down_depth"] = 2
		request["relations"] = []string{"PASSES_CALLBACK"}
		request["providers"] = []string{"fake@1"}
		response := runMCPProcess(t, mcpBinary, []string{"--bootstrap-config", configPath}, []map[string]any{callRequest(2, "lsp_trace_v1_slice", request)})[0]
		call := decodeProcessCall(t, response)
		if call.env["operation_status"] != "SUCCEEDED" {
			t.Fatalf("ASSERT_PRODUCTION_MCP_SLICE_NONCALLS_REAL_PROVIDER: %s; envelope=%v", productionProviderContractGap, call.env)
		}
		t.Log("PASS ASSERT_PRODUCTION_MCP_SLICE_NONCALLS_REAL_PROVIDER")
	})
	t.Run("ASSERT_PRODUCTION_OMISSION_ZERO_PROVIDER_START_EXACT_GRAPH_V3", func(t *testing.T) {
		t.Fatalf("ASSERT_PRODUCTION_OMISSION_ZERO_PROVIDER_START_EXACT_GRAPH_V3: %s; no separately provisioned provider exists to mark non-invocation, so exact omission parity cannot be observed without fabricating authority", productionProviderContractGap)
	})
}

func conformanceRepositoryRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("repository root unavailable")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}

func cloneMap(source map[string]any) map[string]any {
	out := make(map[string]any, len(source))
	for k, v := range source {
		out[k] = v
	}
	return out
}
