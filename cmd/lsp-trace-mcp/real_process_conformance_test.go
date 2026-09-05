package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"lsp-trace/internal/graph"
	"lsp-trace/internal/observationadapter"
	"lsp-trace/internal/provider"
)

const productionProviderContractGap = "production MCP bootstrap has no host-provisioned relation-provider registry/collector boundary; composeHostSelectorRuntime injects only the LSP session runtime"

type realProviderComposition struct {
	Complete  bool              `json:"complete"`
	Calls     json.RawMessage   `json:"calls"`
	Providers []json.RawMessage `json:"providers"`
}

func requireRealProviderGraphV4(t *testing.T, assertion string, call processCall) graph.NormalizedRelations {
	t.Helper()
	if call.env["operation_status"] != "SUCCEEDED" {
		t.Fatalf("%s: provider operation failed: envelope=%v", assertion, call.env)
	}
	var result graph.NormalizedRelations
	if err := json.Unmarshal(inlineArtifactBytes(t, call.env), &result); err != nil || result.SchemaVersion != graph.NormalizedRelationsSchemaVersion || len(result.Relations) != 1 || result.Relations[0].Kind != graph.RelationPassesCallback {
		t.Fatalf("%s: graph-v4 evidence absent or malformed: result=%+v err=%v", assertion, result, err)
	}
	return result
}

func requireRealProviderComposition(t *testing.T, assertion string, call processCall) realProviderComposition {
	t.Helper()
	if call.env["operation_status"] != "SUCCEEDED" {
		t.Fatalf("%s: %s; envelope=%v", assertion, productionProviderContractGap, call.env)
	}
	var composition realProviderComposition
	if err := json.Unmarshal(inlineArtifactBytes(t, call.env), &composition); err != nil || len(composition.Providers) != 1 || !bytes.Contains(composition.Providers[0], []byte(`"provider_id":"fake@1.0.0"`)) || !bytes.Contains(composition.Providers[0], []byte(`"kind":"PASSES_CALLBACK"`)) {
		t.Fatalf("%s: provider evidence absent or malformed: composition=%+v err=%v", assertion, composition, err)
	}
	return composition
}

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
		var envelope observationadapter.Envelope
		if err := json.Unmarshal(a.Response, &envelope); err != nil {
			t.Fatalf("ASSERT_REAL_PROVIDER_SUCCESS_SCHEMA_AND_REPLAY: decode strict envelope: %v", err)
		}
		adapted, err := observationadapter.Adapt(envelope)
		if err != nil || len(adapted.Observations) != 1 {
			t.Fatalf("ASSERT_REAL_PROVIDER_SUCCESS_SCHEMA_AND_REPLAY: adapt=%+v err=%v", adapted, err)
		}
		observation := adapted.Observations[0]
		if observation.OriginalAnchor.DocumentID != "fixture-original" || observation.OriginalAnchor.URI != "file:///fixture/main.go" || observation.OriginalAnchor.Range.Start.Line != 0 || observation.OriginalAnchor.Range.Start.Character != 0 || observation.OriginalAnchor.Range.End.Line != 0 || observation.OriginalAnchor.Range.End.Character != 7 || len(adapted.Custody.Documents) != 1 || adapted.Custody.Documents[0].OriginalURI != observation.OriginalAnchor.URI {
			t.Fatalf("ASSERT_REAL_PROVIDER_SUCCESS_SCHEMA_AND_REPLAY: original custody or exact anchor changed: observation=%+v custody=%+v", observation, adapted.Custody)
		}
		t.Log("PASS ASSERT_REAL_PROVIDER_SUCCESS_SCHEMA_AND_REPLAY")
	})
	t.Run("ASSERT_REAL_PROVIDER_NO_ITEM_STRICT_ENVELOPE", func(t *testing.T) {
		receipt := run(t, "fake@1", "no-item", context.Background(), limits)
		var envelope observationadapter.Envelope
		if receipt.Failure != nil || !receipt.Reaped || json.Unmarshal(receipt.Response, &envelope) != nil {
			t.Fatalf("ASSERT_REAL_PROVIDER_NO_ITEM_STRICT_ENVELOPE: receipt=%+v", receipt)
		}
		adapted, err := observationadapter.Adapt(envelope)
		if err != nil || len(adapted.Observations) != 0 || len(adapted.Custody.Documents) != 1 {
			t.Fatalf("ASSERT_REAL_PROVIDER_NO_ITEM_STRICT_ENVELOPE: adapted=%+v err=%v", adapted, err)
		}
		t.Log("PASS ASSERT_REAL_PROVIDER_NO_ITEM_STRICT_ENVELOPE")
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

func TestManagedSliceReturnsGraphV4ForReadyExternalProvider(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("managed fake LSP requires LocalDarwinSupervisor")
	}
	mcpBinary := buildMCPBinary(t)
	fakeLSP := buildBinary(t, "fake-lsp", "./cmd/fake-lsp")
	fakeProvider := buildBinary(t, "fake-relation-provider", "./cmd/lsp-trace-mcp/testdata/fake-relation-provider")
	workspace := t.TempDir()
	config := map[string]any{
		"version": 1,
		"processes": []any{map[string]any{
			"alias":     "fixture",
			"profile":   map[string]any{"trust_domain": "managed-graph-v4-red", "workspace": workspace, "profile": "fake-lsp", "environment_reference": "hermetic"},
			"execution": map[string]any{"path": fakeLSP, "directory": workspace},
		}},
		"providers": []any{map[string]any{
			"schema_version": "lsp-trace.bootstrap-provider.v1",
			"identity":       "fake@1.0.0", "version": "1.0.0",
			"protocol":     map[string]any{"name": "lsp-trace.provider-observations", "version": "1"},
			"execution":    map[string]any{"path": fakeProvider, "directory": workspace, "environment": []string{"LSP_TRACE_PROVIDER_MODE=success"}},
			"capabilities": map[string]any{"relations": []string{"PASSES_CALLBACK"}, "languages": []string{"go"}, "frameworks": []string{"fixture"}},
			"limits":       map[string]any{"request_bytes": 4096, "response_bytes": 4096, "protocol_messages": 1, "stderr_bytes": 128, "wall_time_ms": 1000, "termination_grace_ms": 50},
		}},
	}
	base := map[string]any{
		"session_id": "fixture", "generation": 1, "start_mode": "at", "uri": "file:///fixture/main.go", "line": 0, "character": 0,
		"up_depth": 2, "down_depth": 2, "max_nodes": 20, "timeout_ms": 1000, "request_timeout_ms": 500,
	}
	nonCalls := cloneMap(base)
	nonCalls["relations"] = []string{"PASSES_CALLBACK"}
	nonCalls["providers"] = []string{"fake@1.0.0"}
	omitted := cloneMap(base)
	calls := cloneMap(base)
	calls["relations"] = []string{"CALLS"}
	responses, err := runMCPProcessForAcceptance(mcpBinary, []string{"--bootstrap-config", writeBootstrapJSON(t, config)}, []map[string]any{
		callRequest(1, "lsp_session_v1_list", map[string]any{}),
		callRequest(2, "lsp_trace_v1_slice", omitted),
		callRequest(3, "lsp_trace_v1_slice", calls),
		callRequest(4, "lsp_trace_v1_slice", nonCalls),
	})
	if err != nil {
		t.Fatalf("setup: production MCP process failed: %v", err)
	}
	readyRaw, _ := json.Marshal(responses[0])
	if !bytes.Contains(readyRaw, []byte(`"State":"READY"`)) || !bytes.Contains(readyRaw, []byte(`"Generation":1`)) {
		t.Fatalf("setup: managed session did not reach READY: response=%s", readyRaw)
	}
	omittedCall := decodeProcessCall(t, responses[1])
	var omittedGraph graph.Result
	omittedArtifact := inlineArtifactBytes(t, omittedCall.env)
	if omittedCall.env["operation_status"] != "SUCCEEDED" || json.Unmarshal(omittedArtifact, &omittedGraph) != nil || omittedGraph.SchemaVersion != graph.SchemaVersionV3 {
		t.Fatalf("omitted control: envelope=%v artifact=%q", omittedCall.env, omittedArtifact)
	}
	callsCall := decodeProcessCall(t, responses[2])
	callsArtifact := inlineArtifactBytes(t, callsCall.env)
	if callsCall.env["operation_status"] != "SUCCEEDED" || len(callsArtifact) == 0 || !bytes.Contains(callsArtifact, []byte(`"schema_version":"lsp-trace.slice-composition.v1"`)) || !bytes.Contains(callsArtifact, []byte(`"calls":`)) {
		t.Fatalf("CALLS control: envelope=%v artifact=%q", callsCall.env, callsArtifact)
	}
	nonCallsCall := decodeProcessCall(t, responses[3])
	nonCallsArtifact := inlineArtifactBytes(t, nonCallsCall.env)
	var identity struct {
		SchemaVersion string `json:"schema_version"`
	}
	decodeErr := json.Unmarshal(nonCallsArtifact, &identity)
	if nonCallsCall.env["operation_status"] != "SUCCEEDED" || decodeErr != nil || identity.SchemaVersion != "lsp-trace.graph.v4" {
		rawResponse, _ := json.Marshal(responses[3])
		t.Fatalf("ASSERT_MANAGED_SLICE_RETURNS_GRAPH_V4_FOR_READY_EXTERNAL_PROVIDER: internal_artifact=%q internal_decode_error=%v internal_schema_version=%q mcp_response=%s", nonCallsArtifact, decodeErr, identity.SchemaVersion, rawResponse)
	}
}

func TestManagedIncomingReturnsGraphV4ForReadyExternalProvider(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("managed fake LSP requires LocalDarwinSupervisor")
	}
	mcpBinary := buildMCPBinary(t)
	fakeLSP := buildBinary(t, "fake-lsp", "./cmd/fake-lsp")
	fakeProvider := buildBinary(t, "fake-relation-provider", "./cmd/lsp-trace-mcp/testdata/fake-relation-provider")
	workspace := t.TempDir()
	config := map[string]any{
		"version": 1,
		"processes": []any{map[string]any{
			"alias": "fixture", "profile": map[string]any{"trust_domain": "managed-incoming-graph-v4", "workspace": workspace, "profile": "fake-lsp", "environment_reference": "hermetic"},
			"execution": map[string]any{"path": fakeLSP, "directory": workspace},
		}},
		"providers": []any{map[string]any{
			"schema_version": "lsp-trace.bootstrap-provider.v1", "identity": "fake@1.0.0", "version": "1.0.0",
			"protocol":     map[string]any{"name": "lsp-trace.provider-observations", "version": "1"},
			"execution":    map[string]any{"path": fakeProvider, "directory": workspace, "environment": []string{"LSP_TRACE_PROVIDER_MODE=success"}},
			"capabilities": map[string]any{"relations": []string{"PASSES_CALLBACK"}, "languages": []string{"go"}, "frameworks": []string{"fixture"}},
			"limits":       map[string]any{"request_bytes": 4096, "response_bytes": 4096, "protocol_messages": 1, "stderr_bytes": 128, "wall_time_ms": 1000, "termination_grace_ms": 50},
		}},
	}
	request := map[string]any{
		"session_id": "fixture", "generation": 1, "uri": "file:///fixture/main.go", "line": 0, "character": 0,
		"max_depth": 2, "max_nodes": 20, "timeout_ms": 1000, "request_timeout_ms": 500,
		"relations": []string{"PASSES_CALLBACK"}, "providers": []string{"fake@1.0.0"},
	}
	responses, err := runMCPProcessForAcceptance(mcpBinary, []string{"--bootstrap-config", writeBootstrapJSON(t, config)}, []map[string]any{
		callRequest(1, "lsp_session_v1_list", map[string]any{}),
		callRequest(2, "lsp_trace_v1_incoming", request),
	})
	if err != nil {
		t.Fatalf("setup: production MCP process failed: %v", err)
	}
	readyRaw, _ := json.Marshal(responses[0])
	if !bytes.Contains(readyRaw, []byte(`"State":"READY"`)) {
		t.Fatalf("setup: managed session did not reach READY: response=%s", readyRaw)
	}
	call := decodeProcessCall(t, responses[1])
	artifact := inlineArtifactBytes(t, call.env)
	var identity struct {
		SchemaVersion string `json:"schema_version"`
	}
	decodeErr := json.Unmarshal(artifact, &identity)
	if call.env["operation_status"] != "SUCCEEDED" || decodeErr != nil || identity.SchemaVersion != "lsp-trace.graph.v4" {
		rawResponse, _ := json.Marshal(responses[1])
		t.Fatalf("ASSERT_MANAGED_INCOMING_RETURNS_GRAPH_V4_FOR_READY_EXTERNAL_PROVIDER: internal_artifact=%q internal_decode_error=%v internal_schema_version=%q mcp_response=%s", artifact, decodeErr, identity.SchemaVersion, rawResponse)
	}
}

func TestManagedNonCallsNeverReturnsEmptyToolResult(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("managed fake LSP requires LocalDarwinSupervisor")
	}
	mcpBinary := buildMCPBinary(t)
	fakeLSP := buildBinary(t, "fake-lsp", "./cmd/fake-lsp")
	fakeProvider := buildBinary(t, "fake-relation-provider", "./cmd/lsp-trace-mcp/testdata/fake-relation-provider")
	for _, tc := range []struct {
		mode        string
		wantStatus  string
		wantCode    string
		unavailable bool
	}{
		{"success", "SUCCEEDED", "", false},
		{"provider-unavailable", "FAILED", "RESOURCE_EXHAUSTED", true},
		{"malformed-frame", "FAILED", "RESOURCE_EXHAUSTED", false},
		{"malformed-payload", "FAILED", "RESOURCE_EXHAUSTED", false},
		{"oversized", "FAILED", "RESOURCE_EXHAUSTED", false},
		{"hang", "FAILED", "RESOURCE_EXHAUSTED", false},
		{"no-item", "FAILED", "RESOURCE_EXHAUSTED", false},
		{"adapter-mismatch", "FAILED", "RESOURCE_EXHAUSTED", false},
		{"revision-mismatch", "FAILED", "RESOURCE_EXHAUSTED", false},
		{"kind-mismatch", "FAILED", "RESOURCE_EXHAUSTED", false},
		{"invalid-coverage", "FAILED", "RESOURCE_EXHAUSTED", false},
	} {
		t.Run(tc.mode, func(t *testing.T) {
			workspace := t.TempDir()
			wallTimeMS := 1000
			if tc.mode == "hang" {
				wallTimeMS = 100
			}
			providerPath := fakeProvider
			if tc.unavailable {
				providerPath = filepath.Join(t.TempDir(), "missing-provider")
			}
			config := map[string]any{
				"version":   1,
				"processes": []any{map[string]any{"alias": "fixture", "profile": map[string]any{"trust_domain": "noncalls-failure-" + tc.mode, "workspace": workspace, "profile": "fake-lsp", "environment_reference": "hermetic"}, "execution": map[string]any{"path": fakeLSP, "directory": workspace}}},
				"providers": []any{map[string]any{
					"schema_version": "lsp-trace.bootstrap-provider.v1", "identity": "fake@1.0.0", "version": "1.0.0",
					"protocol":     map[string]any{"name": "lsp-trace.provider-observations", "version": "1"},
					"execution":    map[string]any{"path": providerPath, "directory": workspace, "environment": []string{"LSP_TRACE_PROVIDER_MODE=" + tc.mode}},
					"capabilities": map[string]any{"relations": []string{"PASSES_CALLBACK"}, "languages": []string{"go"}, "frameworks": []string{"fixture"}},
					"limits":       map[string]any{"request_bytes": 4096, "response_bytes": 4096, "protocol_messages": 1, "stderr_bytes": 128, "wall_time_ms": wallTimeMS, "termination_grace_ms": 20},
				}},
			}
			request := map[string]any{"session_id": "fixture", "generation": 1, "uri": "file:///fixture/main.go", "line": 0, "character": 0, "max_depth": 2, "max_nodes": 20, "timeout_ms": 1000, "request_timeout_ms": 500, "relations": []string{"PASSES_CALLBACK"}, "providers": []string{"fake@1.0.0"}, "workspace_revision": map[string]any{"kind": "git", "value": strings.Repeat("b", 40), "custody": "CALLER_ASSERTED"}}
			responses, err := runMCPProcessForAcceptance(mcpBinary, []string{"--bootstrap-config", writeBootstrapJSON(t, config)}, []map[string]any{callRequest(1, "lsp_trace_v1_incoming", request)})
			if err != nil {
				t.Fatalf("ASSERT_MANAGED_NONCALLS_NEVER_RETURNS_EMPTY_TOOL_RESULT_%s: process=%v", tc.mode, err)
			}
			call := decodeProcessCall(t, responses[0])
			if call.env["operation_status"] != tc.wantStatus || (tc.wantCode != "" && call.env["code"] != tc.wantCode) {
				t.Fatalf("ASSERT_MANAGED_NONCALLS_NEVER_RETURNS_EMPTY_TOOL_RESULT_%s: envelope=%v", tc.mode, call.env)
			}
			if tc.wantStatus == "SUCCEEDED" && len(inlineArtifactBytes(t, call.env)) == 0 {
				t.Fatalf("ASSERT_MANAGED_NONCALLS_NEVER_RETURNS_EMPTY_TOOL_RESULT_%s: empty artifact", tc.mode)
			}
		})
	}
}

func TestProductionMCPRealProviderConformance(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Fatalf("ASSERT_PRODUCTION_PROVIDER_PLATFORM: skip-free production process conformance requires supported LocalDarwinSupervisor")
	}
	mcpBinary := buildMCPBinary(t)
	fakeLSP := buildBinary(t, "fake-lsp", "./cmd/fake-lsp")
	fakeProvider := buildBinary(t, "fake-relation-provider", "./cmd/lsp-trace-mcp/testdata/fake-relation-provider")
	workspace := t.TempDir()
	providerStartMarker := filepath.Join(t.TempDir(), "provider-started")
	baseConfig := map[string]any{"version": 1, "processes": []any{map[string]any{"alias": "fixture", "profile": map[string]any{"trust_domain": "real-provider-conformance", "workspace": workspace, "profile": "fake-lsp", "environment_reference": "hermetic"}, "execution": map[string]any{"path": fakeLSP, "directory": workspace}}}}
	config := cloneMap(baseConfig)
	config["providers"] = []any{map[string]any{
		"schema_version": "lsp-trace.bootstrap-provider.v1",
		"identity":       "fake@1.0.0", "version": "1.0.0",
		"protocol": map[string]any{"name": "lsp-trace.provider-observations", "version": "1"},
		"execution": map[string]any{"path": fakeProvider, "directory": workspace,
			"environment": []string{"LSP_TRACE_PROVIDER_MODE=success", "LSP_TRACE_PROVIDER_START_MARKER=" + providerStartMarker}},
		"capabilities": map[string]any{"relations": []string{"PASSES_CALLBACK"}, "languages": []string{"go"}, "frameworks": []string{"fixture"}},
		"limits":       map[string]any{"request_bytes": 4096, "response_bytes": 4096, "protocol_messages": 1, "stderr_bytes": 128, "wall_time_ms": 1000, "termination_grace_ms": 50},
	}}
	configPath := writeBootstrapJSON(t, config)
	legacyConfigPath := writeBootstrapJSON(t, baseConfig)
	base := map[string]any{"session_id": "fixture", "generation": 1, "uri": "file:///fixture/main.go", "line": 0, "character": 0, "max_depth": 2, "max_nodes": 20, "timeout_ms": 1000, "request_timeout_ms": 500}

	t.Run("ASSERT_PRODUCTION_MCP_INCOMING_NONCALLS_REAL_PROVIDER", func(t *testing.T) {
		request := cloneMap(base)
		request["relations"] = []string{"PASSES_CALLBACK"}
		request["providers"] = []string{"fake@1.0.0"}
		responses, err := runMCPProcessForAcceptance(mcpBinary, []string{"--bootstrap-config", configPath}, []map[string]any{callRequest(1, "lsp_trace_v1_incoming", request)})
		if err != nil {
			t.Fatalf("ASSERT_PRODUCTION_MCP_INCOMING_NONCALLS_REAL_PROVIDER: host provider bootstrap unavailable: %v", err)
		}
		call := decodeProcessCall(t, responses[0])
		requireRealProviderGraphV4(t, "ASSERT_PRODUCTION_MCP_INCOMING_NONCALLS_REAL_PROVIDER", call)
		t.Log("PASS ASSERT_PRODUCTION_MCP_INCOMING_NONCALLS_REAL_PROVIDER")
	})
	t.Run("ASSERT_PRODUCTION_MCP_SLICE_NONCALLS_REAL_PROVIDER", func(t *testing.T) {
		request := cloneMap(base)
		delete(request, "max_depth")
		request["start_mode"] = "at"
		request["up_depth"] = 2
		request["down_depth"] = 2
		request["relations"] = []string{"PASSES_CALLBACK"}
		request["providers"] = []string{"fake@1.0.0"}
		responses, err := runMCPProcessForAcceptance(mcpBinary, []string{"--bootstrap-config", configPath}, []map[string]any{callRequest(2, "lsp_trace_v1_slice", request)})
		if err != nil {
			t.Fatalf("ASSERT_PRODUCTION_MCP_SLICE_NONCALLS_REAL_PROVIDER: host provider bootstrap unavailable: %v", err)
		}
		call := decodeProcessCall(t, responses[0])
		requireRealProviderGraphV4(t, "ASSERT_PRODUCTION_MCP_SLICE_NONCALLS_REAL_PROVIDER", call)
		t.Log("PASS ASSERT_PRODUCTION_MCP_SLICE_NONCALLS_REAL_PROVIDER")
	})
	for _, operation := range []string{"incoming", "slice"} {
		t.Run("ASSERT_PRODUCTION_PROVIDER_SUCCESS_NEVER_MASKS_"+strings.ToUpper(operation)+"_CALLS_GAP", func(t *testing.T) {
			request := cloneMap(base)
			request["max_nodes"] = 1
			request["relations"] = []string{"CALLS", "PASSES_CALLBACK"}
			request["providers"] = []string{"fake@1.0.0"}
			tool := "lsp_trace_v1_incoming"
			if operation == "slice" {
				tool = "lsp_trace_v1_slice"
				delete(request, "max_depth")
				request["start_mode"] = "at"
				request["up_depth"] = 2
				request["down_depth"] = 2
			}
			assertion := "ASSERT_PRODUCTION_PROVIDER_SUCCESS_NEVER_MASKS_" + strings.ToUpper(operation) + "_CALLS_GAP"
			responses, err := runMCPProcessForAcceptance(mcpBinary, []string{"--bootstrap-config", configPath}, []map[string]any{callRequest(4, tool, request)})
			if err != nil {
				t.Fatalf("%s: process=%v", assertion, err)
			}
			composition := requireRealProviderComposition(t, assertion, decodeProcessCall(t, responses[0]))
			var calls graph.Result
			if len(composition.Calls) == 0 || json.Unmarshal(composition.Calls, &calls) != nil || calls.Summary.Complete || composition.Complete {
				t.Fatalf("%s: provider success masked CALLS gap: composition_complete=%v calls=%+v", assertion, composition.Complete, calls.Summary)
			}
			t.Log("PASS " + assertion)
		})
	}
	t.Run("ASSERT_PRODUCTION_OMISSION_ZERO_PROVIDER_START_EXACT_GRAPH_V3", func(t *testing.T) {
		if err := os.Remove(providerStartMarker); err != nil && !os.IsNotExist(err) {
			t.Fatal(err)
		}
		request := cloneMap(base)
		legacyResponses, err := runMCPProcessForAcceptance(mcpBinary, []string{"--bootstrap-config", legacyConfigPath}, []map[string]any{callRequest(3, "lsp_trace_v1_incoming", request)})
		if err != nil {
			t.Fatalf("ASSERT_PRODUCTION_OMISSION_ZERO_PROVIDER_START_EXACT_GRAPH_V3: historical graph-v3 run failed: %v", err)
		}
		providerResponses, err := runMCPProcessForAcceptance(mcpBinary, []string{"--bootstrap-config", configPath}, []map[string]any{callRequest(3, "lsp_trace_v1_incoming", request)})
		if err != nil {
			t.Fatalf("ASSERT_PRODUCTION_OMISSION_ZERO_PROVIDER_START_EXACT_GRAPH_V3: host provider bootstrap unavailable: %v", err)
		}
		if _, err := os.Stat(providerStartMarker); !os.IsNotExist(err) {
			t.Fatalf("ASSERT_PRODUCTION_OMISSION_ZERO_PROVIDER_START_EXACT_GRAPH_V3: omitted relations started provider: marker_err=%v", err)
		}
		legacy := decodeProcessCall(t, legacyResponses[0])
		withProvider := decodeProcessCall(t, providerResponses[0])
		legacyBytes, providerBytes := inlineArtifactBytes(t, legacy.env), inlineArtifactBytes(t, withProvider.env)
		var providerGraph graph.Result
		if !bytes.Equal(providerBytes, legacyBytes) || json.Unmarshal(providerBytes, &providerGraph) != nil || providerGraph.SchemaVersion != graph.SchemaVersionV3 {
			t.Fatalf("ASSERT_PRODUCTION_OMISSION_ZERO_PROVIDER_START_EXACT_GRAPH_V3: graph-v3 bytes changed or schema drifted: schema=%q\nprovider: %q\nhistorical: %q", providerGraph.SchemaVersion, providerBytes, legacyBytes)
		}
		t.Log("PASS ASSERT_PRODUCTION_OMISSION_ZERO_PROVIDER_START_EXACT_GRAPH_V3")
	})
}

func writeBootstrapJSON(t *testing.T, config map[string]any) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "bootstrap.json")
	raw, err := json.Marshal(config)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func runMCPProcessForAcceptance(binary string, args []string, requests []map[string]any) ([]map[string]any, error) {
	var stdin bytes.Buffer
	encoder := json.NewEncoder(&stdin)
	encoder.SetEscapeHTML(false)
	for _, request := range requests {
		if err := encoder.Encode(request); err != nil {
			return nil, err
		}
	}
	command := exec.Command(binary, args...)
	command.Stdin = &stdin
	var stdout, stderr bytes.Buffer
	command.Stdout, command.Stderr = &stdout, &stderr
	if err := command.Run(); err != nil {
		return nil, fmt.Errorf("run: %w stderr=%s", err, stderr.String())
	}
	lines := strings.Split(strings.TrimSpace(stdout.String()), "\n")
	if len(lines) != len(requests) {
		return nil, fmt.Errorf("responses=%d requests=%d stdout=%q", len(lines), len(requests), stdout.String())
	}
	responses := make([]map[string]any, len(lines))
	for i, line := range lines {
		if err := json.Unmarshal([]byte(line), &responses[i]); err != nil {
			return nil, fmt.Errorf("response[%d]=%q: %w", i, line, err)
		}
	}
	return responses, nil
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
