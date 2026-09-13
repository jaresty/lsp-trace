package mcp

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"lsp-trace/internal/graph"
	"lsp-trace/internal/graphprovenance"
	"lsp-trace/internal/manageddiagnostic"
	"lsp-trace/internal/mcpcontract"
	"lsp-trace/internal/operation"
)

type traceRecordingExecutor struct {
	calls    []operation.Request
	artifact []byte
	failure  *operation.Failure
}

func (e *traceRecordingExecutor) Execute(_ context.Context, r operation.Request) (operation.Result, *operation.Failure) {
	e.calls = append(e.calls, r)
	if e.failure != nil {
		return operation.Result{}, e.failure
	}
	artifact := e.artifact
	if artifact == nil {
		artifact = []byte(`{"schema_version":"lsp-trace.graph-provenance.v5"}`)
	}
	return operation.Result{Artifact: artifact}, nil
}

func validTraceV5(t *testing.T, complete, truncated bool) []byte {
	t.Helper()
	uri := "file:///w/a.go"
	p := graph.Position{}
	node := graph.NewNode(graph.Item{Name: "A", Kind: 12, URI: uri, Range: graph.Range{Start: p, End: graph.Position{Character: 1}}, SelectionRange: graph.Range{Start: p, End: graph.Position{Character: 1}}})
	result := graph.Result{SchemaVersion: graph.SchemaVersionV5, Nodes: []graph.Node{node}, Targets: []string{node.ID}, Invocation: graph.Invocation{Target: graph.Target{URI: uri, Line: 0, Column: 0}, Server: graph.ServerInvocation{Command: "fixture"}, Provenance: graph.InvocationProvenance{InvocationID: "fixture", SourceRevision: graph.Unknown, ServerVersion: "fixture@1"}}, Summary: graph.Summary{Truncated: truncated}}
	if !complete {
		result.Frontier = []graph.Boundary{{NodeID: node.ID, Reason: graph.RequestTimeout}}
	}
	native, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := graphprovenance.CaptureV5(native, "s", 1, manageddiagnostic.QueryResult{Status: manageddiagnostic.QueryUnavailable, Records: []manageddiagnostic.Record{}}, &graphprovenance.EvidenceV2{SchemaVersion: graphprovenance.VersionV2, Policy: graphprovenance.PolicyV2, WorkspaceURI: "file:///w", AnalyzedVersion: graphprovenance.Unverified, DependencyCompleteness: "UNKNOWN_INCOMPLETE", Supplies: []graphprovenance.SupplyReceiptV2{}, Captures: []graphprovenance.Receipt{}, Bindings: []graphprovenance.BindingV2{}})
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestTraceIncompleteAndUnsupportedEnvelopeDirectCanonicalParity(t *testing.T) {
	full := NewRegistryWithProfile(false, ToolProfileFull)
	valid := map[string]any{"session_id": "s", "uri": "file:///w/a.go", "line": float64(0), "character": float64(1)}
	for _, tc := range []struct {
		name     string
		artifact []byte
	}{
		{"partial", validTraceV5(t, false, false)},
		{"truncated", validTraceV5(t, true, true)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			executor := &traceRecordingExecutor{artifact: tc.artifact}
			server := &Server{Registry: full, Executors: map[ExecutorFamily]Executor{TraceExecutorFamily: executor}}
			for i, params := range []json.RawMessage{mustCallParams(t, mcpcontract.TraceTool, valid), mustCallParams(t, "lsp_trace_v1_execute", map[string]any{"request": map[string]any{"operation": mcpcontract.TraceTool, "arguments": valid}})} {
				response := server.callContext(context.Background(), response{JSONRPC: "2.0", ID: float64(i + 1)}, params)
				raw, _ := json.Marshal(response.Result)
				want := `"outcome":"PARTIAL"`
				if i == 1 {
					want = `"delegated_outcome":"PARTIAL"`
				}
				if !strings.Contains(string(raw), want) {
					t.Fatalf("ASSERT_TRACE_INCOMPLETE_DIRECT_CANONICAL_PARITY_%s: %s", tc.name, raw)
				}
			}
		})
	}
	executor := &traceRecordingExecutor{failure: &operation.Failure{Code: "UNSUPPORTED_DOCUMENT_SYMBOL"}}
	server := &Server{Registry: full, Executors: map[ExecutorFamily]Executor{TraceExecutorFamily: executor}}
	for i, params := range []json.RawMessage{mustCallParams(t, mcpcontract.TraceTool, map[string]any{"session_id": "s", "uri": "file:///w/a.go", "symbol": "A"}), mustCallParams(t, "lsp_trace_v1_execute", map[string]any{"request": map[string]any{"operation": mcpcontract.TraceTool, "arguments": map[string]any{"session_id": "s", "uri": "file:///w/a.go", "symbol": "A"}}})} {
		response := server.callContext(context.Background(), response{JSONRPC: "2.0", ID: float64(i + 1)}, params)
		raw, _ := json.Marshal(response.Result)
		if !strings.Contains(string(raw), "UNSUPPORTED_CALL_HIERARCHY") {
			t.Fatalf("ASSERT_TRACE_UNSUPPORTED_DOCUMENT_SYMBOL_PUBLIC_CODE_PARITY: %s", raw)
		}
	}
}

func TestTraceMalformedV5DirectCanonicalParity(t *testing.T) {
	full := NewRegistryWithProfile(false, ToolProfileFull)
	valid := map[string]any{"session_id": "s", "uri": "file:///w/a.go", "line": float64(0), "character": float64(1)}
	production := validTraceV5(t, true, false)
	var envelope map[string]any
	if err := json.Unmarshal(production, &envelope); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name     string
		artifact []byte
	}{
		{"malformed-envelope", []byte(`{"schema_version":"lsp-trace.graph-provenance.v5"}`)},
		{"mutated-native", func() []byte {
			mutated := mapsClone(envelope)
			mutated["graph_v5"] = "%%%"
			raw, _ := json.Marshal(mutated)
			return raw
		}()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			executor := &traceRecordingExecutor{artifact: tc.artifact}
			server := &Server{Registry: full, Executors: map[ExecutorFamily]Executor{TraceExecutorFamily: executor}}
			for _, params := range []json.RawMessage{mustCallParams(t, mcpcontract.TraceTool, valid), mustCallParams(t, "lsp_trace_v1_execute", map[string]any{"request": map[string]any{"operation": mcpcontract.TraceTool, "arguments": valid}})} {
				response := server.callContext(context.Background(), response{JSONRPC: "2.0", ID: float64(1)}, params)
				raw, _ := json.Marshal(response.Result)
				if !strings.Contains(string(raw), "OUTPUT_VALIDATION_FAILED") {
					t.Fatalf("ASSERT_TRACE_MALFORMED_V5_DIRECT_CANONICAL_PARITY_%s: %s", tc.name, raw)
				}
			}
		})
	}
}

func mapsClone(in map[string]any) map[string]any {
	out := make(map[string]any, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
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
	valid := map[string]any{"session_id": "s", "uri": "file:///w/a.go", "line": float64(0), "character": float64(1)}
	if err := validateArguments(tool, valid); err != nil {
		t.Fatalf("ASSERT_TRACE_DIRECT_SCHEMA_VALID: %v", err)
	}
	zeroDepth := map[string]any{"session_id": "s", "uri": "file:///w/a.go", "line": float64(0), "character": float64(1), "down_depth": float64(0), "up_depth": float64(0)}
	if err := validateArguments(tool, zeroDepth); err != nil {
		t.Fatalf("ASSERT_TRACE_SCHEMA_ZERO_DEPTH_VALID: %v", err)
	}
	for _, bad := range []map[string]any{{"session_id": "s", "uri": "file:///w/a.go", "symbol": "A", "line": 0, "character": 0}, {"session_id": "s", "uri": "file:///w/a.go", "line": 0}, {"session_id": "s", "uri": "file:///w/a.go", "symbol": "A", "unknown": true}, {"session_id": "s", "uri": "file:///w/a.go", "symbol": "A", "down_depth": float64(65)}} {
		if validateArguments(tool, bad) == nil {
			t.Fatalf("ASSERT_TRACE_STRICT_TARGET_SCHEMA_REJECTS: %#v", bad)
		}
	}
	executor := &traceRecordingExecutor{}
	server := &Server{Registry: full, Executors: map[ExecutorFamily]Executor{TraceExecutorFamily: executor}}
	direct := server.callContext(context.Background(), response{JSONRPC: "2.0", ID: float64(1)}, mustCallParams(t, mcpcontract.TraceTool, valid))
	gateway := server.callContext(context.Background(), response{JSONRPC: "2.0", ID: float64(2)}, mustCallParams(t, "lsp_trace_v1_execute", map[string]any{"request": map[string]any{"operation": mcpcontract.TraceTool, "arguments": valid}}))
	if direct.Error != nil || gateway.Error != nil || len(executor.calls) != 2 || executor.calls[0].Name != "trace" || executor.calls[1].Name != "trace" {
		t.Fatalf("ASSERT_TRACE_DIRECT_EXECUTE_PARITY_ZERO_SEED_CUSTODY: direct=%v gateway=%v calls=%+v", direct.Error, gateway.Error, executor.calls)
	}
	var a, b map[string]any
	_ = json.Unmarshal(executor.calls[0].Input, &a)
	_ = json.Unmarshal(executor.calls[1].Input, &b)
	if !reflect.DeepEqual(a, b) {
		t.Fatalf("ASSERT_TRACE_DIRECT_EXECUTE_ARGUMENT_PARITY: %#v %#v", a, b)
	}
}
