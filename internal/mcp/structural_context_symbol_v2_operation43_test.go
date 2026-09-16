package mcp

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"lsp-trace/internal/mcpcontract"
)

const structuralContextSymbolV2Tool = "lsp_trace_v2_structural_context_symbol"

func TestStructuralContextSymbolV2Operation43CountsCompatibility(t *testing.T) {
	const assertion = "ASSERT_STRUCTURAL_CONTEXT_SYMBOL_V2_OPERATION43_COUNTS_COMPATIBILITY"
	full := NewRegistryWithProfile(false, ToolProfileFull)
	compact := NewRegistryWithProfile(false, ToolProfileCompact)
	if len(full.Tools()) != 43 || len(full.Advertised()) != 43 || len(compact.Tools()) != 43 || len(compact.Advertised()) != 13 {
		t.Fatalf("%s: full=%d/%d compact=%d/%d", assertion, len(full.Tools()), len(full.Advertised()), len(compact.Tools()), len(compact.Advertised()))
	}
	v1, v1OK := full.ResolveCanonical(structuralContextSymbolTool)
	v2, v2OK := full.ResolveCanonical(structuralContextSymbolV2Tool)
	if !v1OK || !v2OK || v1.Name != structuralContextSymbolTool || v2.Name != structuralContextSymbolV2Tool {
		t.Fatalf("%s: v1=%+v/%v v2=%+v/%v", assertion, v1, v1OK, v2, v2OK)
	}
	if _, ok := compact.ResolveCanonical(structuralContextSymbolV2Tool); !ok {
		t.Fatalf("%s: V2 operation not compact-advertised", assertion)
	}
}

func TestStructuralContextSymbolV2DispatchVersionBoundary(t *testing.T) {
	const assertion = "ASSERT_STRUCTURAL_CONTEXT_SYMBOL_V2_DISPATCH_VERSION_BOUNDARY"
	full := NewRegistryWithProfile(false, ToolProfileFull)
	v1, v1OK := full.ResolveCanonical(structuralContextSymbolTool)
	v2, v2OK := full.ResolveCanonical(structuralContextSymbolV2Tool)
	if !v1OK || !v2OK || operationName(v1.Name) != "structural_context_symbol" || operationName(v2.Name) != "structural_context_symbol_v2" {
		t.Fatalf("%s: v1=%+v/%v v2=%+v/%v names=%s/%s", assertion, v1, v1OK, v2, v2OK, operationName(v1.Name), operationName(v2.Name))
	}
}

func TestStructuralContextSymbolV2SourceBearingDirectGatewayParity(t *testing.T) {
	const sourceAssertion = "ASSERT_STRUCTURAL_CONTEXT_SYMBOL_V2_SOURCE_BEARING_ENVELOPE"
	const parityAssertion = "ASSERT_STRUCTURAL_CONTEXT_SYMBOL_V2_DIRECT_GATEWAY_PARITY"
	artifact := []byte(`{"schema_version":"lsp-trace.transient-structural-result.v2","authority":0,"source_graph_complete":"UNKNOWN","position_encoding":"utf-16","target_node_id":"tn_0123456789abcdef0123456789abcdef","nodes":[{"node_id":"tn_0123456789abcdef0123456789abcdef","name":"A","kind":12,"path":"src/a.go","declaration_range":{"start":{"line":0,"character":0},"end":{"line":0,"character":1}}}],"calls":[],"analytics_scope":"BOUNDED_LOCAL","coupling":[{"node_id":"tn_0123456789abcdef0123456789abcdef","ca":0,"ce":0,"instability":0}],"strong_components":[{"node_ids":["tn_0123456789abcdef0123456789abcdef"],"cyclic":false}],"weak_projection":"DIRECTED_ARCS_COLLAPSED_TO_SIMPLE_UNDIRECTED_PAIRS; SELF_LOOPS_IGNORED; PARALLEL_AND_ANTIPARALLEL_ARCS_COLLAPSED","weak_bridges":[],"articulation_points":[],"pagerank_damping":0.85,"analytics_tolerance":1e-12,"pagerank":[{"node_id":"tn_0123456789abcdef0123456789abcdef","score":1}],"hits":[{"node_id":"tn_0123456789abcdef0123456789abcdef","hub":0,"authority":0}],"external_nodes_omitted":0,"external_calls_omitted":0}`)
	if err := mcpcontract.ValidateJSON(mcpcontract.StructuralContextV2ResultID, artifact); err != nil {
		t.Fatalf("%s: invalid fixture: %v", sourceAssertion, err)
	}
	executor := &structuralContextRecordingExecutor{artifact: artifact}
	registry := NewRegistryWithProfile(false, ToolProfileFull)
	tool, ok := registry.ResolveCanonical(structuralContextSymbolV2Tool)
	if !ok {
		t.Fatalf("%s: operation missing", sourceAssertion)
	}
	server := &Server{Registry: registry, Executors: map[ExecutorFamily]Executor{tool.ExecutorFamily: executor}}
	args := map[string]any{"session_id": "s", "generation": float64(1), "symbol": "A", "analysis": map[string]any{"kind": "NEIGHBORHOOD"}}
	direct := server.callContext(context.Background(), response{JSONRPC: "2.0", ID: float64(1)}, mustCallParams(t, structuralContextSymbolV2Tool, args))
	gateway := server.callContext(context.Background(), response{JSONRPC: "2.0", ID: float64(2)}, mustCallParams(t, "lsp_trace_v1_execute", map[string]any{"request": map[string]any{"operation": structuralContextSymbolV2Tool, "arguments": args}}))
	if direct.Error != nil || gateway.Error != nil || len(executor.calls) != 2 {
		t.Fatalf("%s: direct=%v gateway=%v calls=%d", parityAssertion, direct.Error, gateway.Error, len(executor.calls))
	}
	directRaw, _ := json.Marshal(direct.Result)
	gatewayRaw, _ := json.Marshal(gateway.Result)
	for _, required := range []string{`"tool":"lsp_trace_v2_structural_context_symbol"`, `"schema_version":"lsp-trace.transient-structural-result.v2"`, `"authority":0`, `"source_graph_complete":"UNKNOWN"`, `"path":"src/a.go"`, `"declaration_range"`} {
		if !strings.Contains(string(directRaw), required) {
			t.Fatalf("%s: missing %s in %s", sourceAssertion, required, directRaw)
		}
	}
	var wrapped map[string]any
	if json.Unmarshal(gatewayRaw, &wrapped) != nil || !strings.Contains(string(gatewayRaw), `lsp_trace_v2_structural_context_symbol`) {
		t.Fatalf("%s: direct=%s gateway=%s", parityAssertion, directRaw, gatewayRaw)
	}
	if tool.InputSchemaID != mcpcontract.StructuralContextSymbolInputID {
		t.Fatalf("%s: input schema=%s", sourceAssertion, tool.InputSchemaID)
	}
}
