package mcp

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

	"lsp-trace/internal/graphkernel"
	"lsp-trace/internal/mcpcontract"
	"lsp-trace/internal/operation"
	delta "lsp-trace/internal/transientstructuraldelta"
)

func TestStructuralDeltaOperation37AppendOnlyContract(t *testing.T) {
	full := NewRegistryWithProfile(false, ToolProfileFull)
	compact := NewRegistryWithProfile(false, ToolProfileCompact)
	_, ok := full.ResolveCanonical(mcpcontract.StructuralDeltaTool)
	if !ok || len(full.Tools()) != 39 || len(full.Advertised()) != 39 || len(compact.Tools()) != 39 || len(compact.Advertised()) != 10 {
		t.Fatalf("ASSERT_STRUCTURAL_DELTA_OPERATION37_APPEND_ONLY: ok=%t full=%d/%d compact=%d/%d", ok, len(full.Tools()), len(full.Advertised()), len(compact.Tools()), len(compact.Advertised()))
	}
	if _, ok := compact.ResolveCanonical(mcpcontract.StructuralDeltaTool); !ok {
		t.Fatal("ASSERT_STRUCTURAL_DELTA_COMPACT_DISPATCHABLE")
	}
	for _, tool := range compact.Advertised() {
		if tool.Name == mcpcontract.StructuralDeltaTool {
			t.Fatal("ASSERT_STRUCTURAL_DELTA_COMPACT_HIDDEN")
		}
	}
}
func TestStructuralDeltaDirectGatewayParity(t *testing.T) {
	artifact, _ := json.Marshal(delta.Result{SchemaVersion: delta.SchemaVersion, Qualification: delta.Qualification{Authority: 0, SourceGraphComplete: "UNKNOWN", ComparisonScope: "TWO_BOUNDED_LOCAL_RESULTS", AcquisitionScopeComparable: "UNKNOWN", NodeUniverseEqual: true, AnalyticsPoliciesEqual: true, ScopeSensitive: []string{"HITS", "PAGERANK"}}, AddedSymbols: []delta.SymbolKey{}, RemovedSymbols: []delta.SymbolKey{}, MatchedSymbols: []delta.SymbolKey{}, Calls: delta.CallDelta{Added: []delta.CallCount{}, Removed: []delta.CallCount{}, CountChanged: []delta.CountChange{}}, Analytics: []delta.AnalyticsDelta{}, AddedWeakBridges: []delta.BridgeKey{}, RemovedWeakBridges: []delta.BridgeKey{}, AffectedCallers: []delta.SymbolKey{}})
	e := &structuralContextRecordingExecutor{artifact: artifact}
	s := &Server{Registry: NewRegistryWithProfile(false, ToolProfileFull), Executors: map[ExecutorFamily]Executor{StructuralDeltaExecutorFamily: e}}
	args := validDeltaArgs()
	direct := s.callContext(context.Background(), response{JSONRPC: "2.0", ID: float64(1)}, mustCallParams(t, mcpcontract.StructuralDeltaTool, args))
	gateway := s.callContext(context.Background(), response{JSONRPC: "2.0", ID: float64(2)}, mustCallParams(t, "lsp_trace_v1_execute", map[string]any{"request": map[string]any{"operation": mcpcontract.StructuralDeltaTool, "arguments": args}}))
	if direct.Error != nil || gateway.Error != nil || len(e.calls) != 2 {
		t.Fatalf("ASSERT_STRUCTURAL_DELTA_TRANSPORT_PARITY: direct=%v gateway=%v calls=%d", direct.Error, gateway.Error, len(e.calls))
	}
	var a, b map[string]any
	_ = json.Unmarshal(e.calls[0].Input, &a)
	_ = json.Unmarshal(e.calls[1].Input, &b)
	if !reflect.DeepEqual(a, b) || e.calls[0].Name != operation.Name("structural_delta") || e.calls[1].Name != operation.Name("structural_delta") {
		t.Fatal("ASSERT_STRUCTURAL_DELTA_SHARED_DISPATCH")
	}
}
func TestStructuralDeltaDomainParity(t *testing.T) {
	e := &structuralContextRecordingExecutor{failure: &operation.Failure{Code: string(delta.CodeAmbiguousSymbolKey), Err: &delta.DomainError{Code: delta.CodeAmbiguousSymbolKey, Message: "duplicate"}}}
	s := &Server{Registry: NewRegistryWithProfile(false, ToolProfileFull), Executors: map[ExecutorFamily]Executor{StructuralDeltaExecutorFamily: e}}
	r := s.callContext(context.Background(), response{JSONRPC: "2.0", ID: float64(1)}, mustCallParams(t, mcpcontract.StructuralDeltaTool, validDeltaArgs()))
	if r.Error != nil {
		t.Fatal(r.Error)
	}
	env := r.Result.(callResult).StructuredContent
	if env.EnvelopeSchemaID != mcpcontract.StructuralDeltaDomainErrorID || env.State != string(delta.CodeAmbiguousSymbolKey) {
		t.Fatalf("ASSERT_STRUCTURAL_DELTA_DOMAIN_PARITY: %+v", env)
	}
}
func validDeltaArgs() map[string]any {
	id := "tn_0123456789abcdef0123456789abcdef"
	v := map[string]any{"schema_version": "lsp-trace.transient-structural-result.v2", "authority": 0, "source_graph_complete": "UNKNOWN", "position_encoding": "utf-16", "target_node_id": id, "nodes": []any{map[string]any{"node_id": id, "name": "A", "kind": 12, "path": "src/a.go", "declaration_range": map[string]any{"start": map[string]any{"line": 0, "character": 0}, "end": map[string]any{"line": 0, "character": 1}}}}, "calls": []any{}, "analytics_scope": "BOUNDED_LOCAL", "coupling": []any{map[string]any{"node_id": id, "ca": 0, "ce": 0, "instability": 0}}, "strong_components": []any{map[string]any{"node_ids": []any{id}, "cyclic": false}}, "weak_projection": graphkernel.WeakProjectionPolicy, "weak_bridges": []any{}, "articulation_points": []any{}, "pagerank_damping": .85, "analytics_tolerance": 1e-12, "pagerank": []any{map[string]any{"node_id": id, "score": 1}}, "hits": []any{map[string]any{"node_id": id, "hub": 0, "authority": 0}}, "external_nodes_omitted": 0, "external_calls_omitted": 0}
	return map[string]any{"before": v, "after": v}
}
