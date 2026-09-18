package mcp

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"lsp-trace/internal/mcpcontract"
	"lsp-trace/internal/operation"
	"lsp-trace/internal/transientstructural"
)

func structuralContextV2Args() map[string]any {
	args := structuralContextArgs()
	delete(args, "uri")
	return args
}

func unifiedStructuralContextV2Artifact() []byte {
	structural := `{"schema_version":"lsp-trace.transient-structural-result.v2","authority":0,"source_graph_complete":"UNKNOWN","position_encoding":"utf-16","target_node_id":"tn_0123456789abcdef0123456789abcdef","nodes":[{"node_id":"tn_0123456789abcdef0123456789abcdef","name":"A","kind":12,"path":"src/a.go","declaration_range":{"start":{"line":0,"character":0},"end":{"line":0,"character":1}}}],"calls":[],"analytics_scope":"BOUNDED_LOCAL","coupling":[{"node_id":"tn_0123456789abcdef0123456789abcdef","ca":0,"ce":0,"instability":0}],"external_nodes_omitted":0,"external_calls_omitted":0}`
	return []byte(`{"schema_version":"lsp-trace.unified-structural-context-result.v2","structural":` + structural + `}`)
}

func TestStructuralContextV2Operation36Contract(t *testing.T) {
	full := NewRegistryWithProfile(false, ToolProfileFull)
	compact := NewRegistryWithProfile(false, ToolProfileCompact)
	tool, ok := full.ResolveCanonical(mcpcontract.StructuralContextV2Tool)
	if !ok || len(full.Tools()) != 41 || len(full.Advertised()) != 41 || len(compact.Tools()) != 41 || len(compact.Advertised()) != 12 {
		t.Fatalf("ASSERT_STRUCTURAL_CONTEXT_V2_OPERATION36_APPEND_ONLY: ok=%t full=%d/%d compact=%d/%d", ok, len(full.Tools()), len(full.Advertised()), len(compact.Tools()), len(compact.Advertised()))
	}
	if _, ok := compact.ResolveCanonical(mcpcontract.StructuralContextV2Tool); !ok {
		t.Fatal("ASSERT_STRUCTURAL_CONTEXT_V2_COMPACT_DISPATCHABLE")
	}
	if tool, ok := compact.ResolveCanonical(mcpcontract.StructuralContextV2Tool); !ok || tool.Name != mcpcontract.StructuralContextV2Tool {
		t.Fatal("ASSERT_STRUCTURAL_CONTEXT_V2_COMPACT_ADVERTISED")
	}
	for _, field := range []string{"source_body", "absolute_path", "file_uri", "environment", "commands", "provider_internals", "publication", "retained_input", "hydration", "custody", "source_supply"} {
		bad := structuralContextArgs()
		bad[field] = "forbidden"
		if validateArguments(tool, bad) == nil {
			t.Fatalf("ASSERT_STRUCTURAL_CONTEXT_V2_REJECTS_%s", field)
		}
	}
}

func TestStructuralContextV2ResourceDiagnosticDirectGatewayParity(t *testing.T) {
	observed, suggested := 125, 125
	diagnostic := &transientstructural.ResourceDiagnostic{Reason: transientstructural.ResourceReasonRegexMaxDocumentBytes, Field: transientstructural.ResourceFieldRegexMaxDocumentBytes, Allowed: 100, Observed: &observed, MaximumAllowed: 16777216, SuggestedLimit: &suggested}
	executor := &structuralContextRecordingExecutor{failure: &operation.Failure{Code: string(transientstructural.StateResourceLimit), Err: &transientstructural.DomainFailure{Phase: transientstructural.PhasePreflight, State: transientstructural.StateResourceLimit, ResourceDiagnostic: diagnostic}}}
	server := &Server{Registry: NewRegistryWithProfile(false, ToolProfileFull), Executors: map[ExecutorFamily]Executor{StructuralContextV2ExecutorFamily: executor}}
	args := structuralContextV2Args()
	direct := server.callContext(context.Background(), response{JSONRPC: "2.0", ID: float64(1)}, mustCallParams(t, mcpcontract.StructuralContextV2Tool, args))
	gateway := server.callContext(context.Background(), response{JSONRPC: "2.0", ID: float64(2)}, mustCallParams(t, "lsp_trace_v1_execute", map[string]any{"request": map[string]any{"operation": mcpcontract.StructuralContextV2Tool, "arguments": args}}))
	if direct.Error != nil || gateway.Error != nil {
		t.Fatalf("ASSERT_STRUCTURAL_CONTEXT_V2_RESOURCE_DIRECT_GATEWAY: direct=%v gateway=%v", direct.Error, gateway.Error)
	}
	outer := gateway.Result.(callResult).StructuredContent
	var delegated envelope
	if err := json.Unmarshal([]byte(outer.DelegatedEnvelope), &delegated); err != nil {
		t.Fatal(err)
	}
	for route, env := range map[string]envelope{"direct": direct.Result.(callResult).StructuredContent, "gateway": delegated} {
		raw, _ := json.Marshal(env)
		if env.Phase != "PREFLIGHT" || env.State != "RESOURCE_LIMIT" || env.EnvelopeSchemaID != mcpcontract.StructuralContextTraversalDomainErrorID || env.ResourceDiagnostic == nil || env.Diagnostic != nil || strings.Contains(string(raw), "file:///") {
			t.Fatalf("ASSERT_STRUCTURAL_CONTEXT_V2_RESOURCE_%s: %s", route, raw)
		}
		if err := mcpcontract.ValidateStructuralContextV2EnvelopeExclusive(raw); err != nil {
			t.Fatalf("ASSERT_STRUCTURAL_CONTEXT_V2_RESOURCE_SCHEMA_%s: %v", route, err)
		}
	}
}

func TestStructuralContextV2GenericResourceFailureRemainsGeneric(t *testing.T) {
	executor := &structuralContextRecordingExecutor{failure: &operation.Failure{Code: string(transientstructural.StateResourceLimit), Err: &transientstructural.DomainFailure{Phase: transientstructural.PhasePreflight, State: transientstructural.StateResourceLimit}}}
	server := &Server{Registry: NewRegistryWithProfile(false, ToolProfileFull), Executors: map[ExecutorFamily]Executor{StructuralContextV2ExecutorFamily: executor}}
	response := server.callContext(context.Background(), response{JSONRPC: "2.0", ID: float64(1)}, mustCallParams(t, mcpcontract.StructuralContextV2Tool, structuralContextV2Args()))
	env := response.Result.(callResult).StructuredContent
	if env.State != "RESOURCE_LIMIT" || env.ResourceDiagnostic != nil || env.Diagnostic != nil {
		t.Fatalf("ASSERT_STRUCTURAL_CONTEXT_V2_GENERIC_RESOURCE: %+v", env)
	}
}

func TestStructuralContextV2TypedLocatorFailureUsesV4Envelope(t *testing.T) {
	executor := &structuralContextRecordingExecutor{failure: &operation.Failure{Code: string(transientstructural.StateTargetNotFound), Err: &transientstructural.DomainFailure{Phase: transientstructural.PhasePreflight, State: transientstructural.StateTargetNotFound}}}
	server := &Server{Registry: NewRegistryWithProfile(false, ToolProfileFull), Executors: map[ExecutorFamily]Executor{StructuralContextV2ExecutorFamily: executor}}
	args := structuralContextV2Args()
	delete(args, "symbol")
	args["uri"] = "file:///workspace/store.go"
	args["line"] = 123
	args["character"] = 5
	direct := server.callContext(context.Background(), response{JSONRPC: "2.0", ID: float64(1)}, mustCallParams(t, mcpcontract.StructuralContextV2Tool, args))
	gateway := server.callContext(context.Background(), response{JSONRPC: "2.0", ID: float64(2)}, mustCallParams(t, "lsp_trace_v1_execute", map[string]any{"request": map[string]any{"operation": mcpcontract.StructuralContextV2Tool, "arguments": args}}))
	if direct.Error != nil {
		t.Fatalf("ASSERT_STRUCTURAL_CONTEXT_V2_DOMAIN_ENVELOPE_direct: rpc error=%v", direct.Error)
	}
	directEnvelope := direct.Result.(callResult).StructuredContent
	directRaw, _ := json.Marshal(directEnvelope)
	_ = json.Unmarshal(directRaw, &directEnvelope)
	if directEnvelope.EnvelopeSchemaID != mcpcontract.StructuralContextTraversalDomainErrorID || directEnvelope.Phase != "PREFLIGHT" || directEnvelope.State != "TARGET_NOT_FOUND" || directEnvelope.Diagnostic != nil {
		t.Fatalf("ASSERT_STRUCTURAL_CONTEXT_V2_DOMAIN_ENVELOPE_direct: %+v", directEnvelope)
	}
	if directEnvelope.Target != nil || strings.Contains(string(directRaw), "file:///workspace/store.go") || strings.Contains(string(directRaw), `"line":123`) || strings.Contains(string(directRaw), `"character":5`) {
		t.Fatalf("ASSERT_STRUCTURAL_CONTEXT_V2_DOMAIN_TARGET_PRIVATE_direct: %s", directRaw)
	}
	if gateway.Error != nil {
		t.Fatalf("ASSERT_STRUCTURAL_CONTEXT_V2_DOMAIN_ENVELOPE_gateway: rpc error=%v", gateway.Error)
	}
	outer := gateway.Result.(callResult).StructuredContent
	var delegated envelope
	if json.Unmarshal([]byte(outer.DelegatedEnvelope), &delegated) != nil || delegated.EnvelopeSchemaID != mcpcontract.StructuralContextTraversalDomainErrorID || delegated.Phase != "PREFLIGHT" || delegated.State != "TARGET_NOT_FOUND" || delegated.Diagnostic != nil {
		t.Fatalf("ASSERT_STRUCTURAL_CONTEXT_V2_DOMAIN_ENVELOPE_gateway: %+v", outer)
	}
	delegatedRaw, _ := json.Marshal(delegated)
	if delegated.Target != nil || strings.Contains(string(delegatedRaw), "file:///workspace/store.go") || strings.Contains(string(delegatedRaw), `"line":123`) || strings.Contains(string(delegatedRaw), `"character":5`) {
		t.Fatalf("ASSERT_STRUCTURAL_CONTEXT_V2_DOMAIN_TARGET_PRIVATE_gateway: %s", delegatedRaw)
	}
}

func TestStructuralContextV2ProjectionFailureUsesSchemaValidPrivateV4Envelope(t *testing.T) {
	executor := &structuralContextRecordingExecutor{failure: &operation.Failure{Code: "SOURCE_PROJECTION_FAILED"}}
	server := &Server{Registry: NewRegistryWithProfile(false, ToolProfileFull), Executors: map[ExecutorFamily]Executor{StructuralContextV2ExecutorFamily: executor}}
	args := structuralContextV2Args()
	delete(args, "symbol")
	args["uri"] = "file:///workspace/store.go"
	args["line"] = 123
	args["character"] = 5

	direct := server.callContext(context.Background(), response{JSONRPC: "2.0", ID: float64(1)}, mustCallParams(t, mcpcontract.StructuralContextV2Tool, args))
	if direct.Error != nil {
		t.Fatalf("ASSERT_STRUCTURAL_CONTEXT_V2_PROJECTION_FAILURE_ENVELOPE: rpc error=%v", direct.Error)
	}
	env := direct.Result.(callResult).StructuredContent
	raw, _ := json.Marshal(env)
	if env.EnvelopeSchemaID != mcpcontract.StructuralContextTraversalDomainErrorID || env.State != "INVALID_SERVER_RESPONSE" || env.Target != nil {
		t.Fatalf("ASSERT_STRUCTURAL_CONTEXT_V2_PROJECTION_FAILURE_ENVELOPE: env=%+v raw=%s", env, raw)
	}
	if err := mcpcontract.ValidateStructuralContextV2EnvelopeExclusive(raw); err != nil {
		t.Fatalf("ASSERT_STRUCTURAL_CONTEXT_V2_PROJECTION_FAILURE_SCHEMA: %v\n%s", err, raw)
	}
	for _, forbidden := range []string{"file:///workspace/store.go", `"line":123`, `"character":5`} {
		if strings.Contains(string(raw), forbidden) {
			t.Fatalf("ASSERT_STRUCTURAL_CONTEXT_V2_PROJECTION_FAILURE_PRIVATE: leaked %q in %s", forbidden, raw)
		}
	}
}

func TestStructuralContextV2ProjectionDirectGatewayParity(t *testing.T) {
	executor := &structuralContextRecordingExecutor{artifact: unifiedStructuralContextV2Artifact()}
	server := &Server{Registry: NewRegistryWithProfile(false, ToolProfileFull), Executors: map[ExecutorFamily]Executor{StructuralContextV2ExecutorFamily: executor}}
	args := structuralContextV2Args()
	args["projection"] = map[string]any{"mode": "TARGET", "body": "OMIT", "include_relation_occurrences": false, "include_ancillary": false, "display_range_policy": "FULL_DEFINITION", "limits": map[string]any{"max_objects": 1, "max_ranges": 1, "max_source_bytes": 0, "max_work": 1, "max_response_bytes": 4096, "max_additional_documents": 0, "max_document_requests": 1, "max_document_bytes": 1024, "max_total_document_bytes": 1024, "max_document_messages": 1, "max_document_acquisition_work": 1, "max_display_resolution_work": 1}, "privacy_policy_id": "public"}
	direct := server.callContext(context.Background(), response{JSONRPC: "2.0", ID: float64(1)}, mustCallParams(t, mcpcontract.StructuralContextV2Tool, args))
	gateway := server.callContext(context.Background(), response{JSONRPC: "2.0", ID: float64(2)}, mustCallParams(t, "lsp_trace_v1_execute", map[string]any{"request": map[string]any{"operation": mcpcontract.StructuralContextV2Tool, "arguments": args}}))
	if direct.Error != nil || gateway.Error != nil || len(executor.calls) != 2 {
		t.Fatalf("ASSERT_STRUCTURAL_CONTEXT_V2_PROJECTION_TRANSPORT_PARITY: direct=%v gateway=%v calls=%d", direct.Error, gateway.Error, len(executor.calls))
	}
	var a, b map[string]any
	if json.Unmarshal(executor.calls[0].Input, &a) != nil || json.Unmarshal(executor.calls[1].Input, &b) != nil || !reflect.DeepEqual(a, b) || a["projection"] == nil {
		t.Fatalf("ASSERT_STRUCTURAL_CONTEXT_V2_PROJECTION_TRANSPORT_PARITY: a=%v b=%v", a, b)
	}
}

func TestStructuralContextV2DirectGatewayParity(t *testing.T) {
	executor := &structuralContextRecordingExecutor{artifact: unifiedStructuralContextV2Artifact()}
	server := &Server{Registry: NewRegistryWithProfile(false, ToolProfileFull), Executors: map[ExecutorFamily]Executor{StructuralContextV2ExecutorFamily: executor}}
	args := structuralContextV2Args()
	direct := server.callContext(context.Background(), response{JSONRPC: "2.0", ID: float64(1)}, mustCallParams(t, mcpcontract.StructuralContextV2Tool, args))
	gateway := server.callContext(context.Background(), response{JSONRPC: "2.0", ID: float64(2)}, mustCallParams(t, "lsp_trace_v1_execute", map[string]any{"request": map[string]any{"operation": mcpcontract.StructuralContextV2Tool, "arguments": args}}))
	if direct.Error != nil || gateway.Error != nil || len(executor.calls) != 2 {
		t.Fatalf("ASSERT_STRUCTURAL_CONTEXT_V2_DIRECT_GATEWAY: direct=%v gateway=%v calls=%d", direct.Error, gateway.Error, len(executor.calls))
	}
	var a, b map[string]any
	_ = json.Unmarshal(executor.calls[0].Input, &a)
	_ = json.Unmarshal(executor.calls[1].Input, &b)
	if !reflect.DeepEqual(a, b) || executor.calls[0].Name != operation.Name("structural_context_v2") || executor.calls[1].Name != operation.Name("structural_context_v2") {
		t.Fatal("ASSERT_STRUCTURAL_CONTEXT_V2_SHARED_DISPATCH")
	}
}
