package mcp

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"lsp-trace/internal/mcpcontract"
	"lsp-trace/internal/operation"
	"lsp-trace/internal/session"
	"lsp-trace/internal/transientstructural"
	"lsp-trace/internal/transientstructuralresult"
	"lsp-trace/sessionruntime"
	"lsp-trace/structuralcontextsymbolops"
)

type structuralContextRecordingExecutor struct {
	calls    []operation.Request
	artifact []byte
	failure  *operation.Failure
}

func (e *structuralContextRecordingExecutor) Execute(_ context.Context, r operation.Request) (operation.Result, *operation.Failure) {
	e.calls = append(e.calls, r)
	return operation.Result{Artifact: e.artifact}, e.failure
}

func structuralContextArgs() map[string]any {
	return map[string]any{"session_id": "s", "generation": float64(1), "uri": "file:///w/a.go", "symbol": "A", "down_depth": float64(2), "up_depth": float64(2), "max_nodes": float64(100), "timeout_ms": float64(5000), "request_timeout_ms": float64(1000), "max_messages": float64(64), "max_bytes": float64(4194304), "analysis": map[string]any{"kind": "NEIGHBORHOOD"}}
}

func TestStructuralContextOperation35RegistrationProfilesAndSchema(t *testing.T) {
	full := NewRegistryWithProfile(false, ToolProfileFull)
	compact := NewRegistryWithProfile(false, ToolProfileCompact)
	tool, ok := full.ResolveCanonical(mcpcontract.StructuralContextTool)
	if !ok || len(full.Tools()) != 41 || len(full.Advertised()) != 41 || len(compact.Tools()) != 41 || len(compact.Advertised()) != 12 {
		t.Fatalf("ASSERT_STRUCTURAL_CONTEXT_OPERATION35_CARDINALITY: ok=%t full=%d/%d compact=%d/%d", ok, len(full.Tools()), len(full.Advertised()), len(compact.Tools()), len(compact.Advertised()))
	}
	if tool.ExecutorFamily != StructuralContextExecutorFamily || tool.InputSchemaID != mcpcontract.StructuralContextInputID || !strings.Contains(tool.Description, "transient live CALLS-only") {
		t.Fatalf("ASSERT_STRUCTURAL_CONTEXT_CONTRACT: %+v", tool)
	}
	for _, n := range toolNames(compact.Advertised()) {
		if n == mcpcontract.StructuralContextTool {
			t.Fatal("ASSERT_STRUCTURAL_CONTEXT_COMPACT_HIDDEN")
		}
	}
	if _, ok := compact.ResolveCanonical(mcpcontract.StructuralContextTool); !ok {
		t.Fatal("ASSERT_STRUCTURAL_CONTEXT_COMPACT_CALLABLE")
	}
	if validateArguments(tool, structuralContextArgs()) != nil {
		t.Fatal("ASSERT_STRUCTURAL_CONTEXT_VALID_INPUT")
	}
	for _, field := range []string{"output_selector", "publication", "custody", "hydration", "replay", "source_supply", "retained_input"} {
		bad := structuralContextArgs()
		bad[field] = true
		if validateArguments(tool, bad) == nil {
			t.Fatalf("ASSERT_STRUCTURAL_CONTEXT_FORBIDDEN_%s", field)
		}
	}
	bad := structuralContextArgs()
	bad["request_timeout_ms"] = float64(5001)
	invalid := (&Server{Registry: full}).callContext(context.Background(), response{JSONRPC: "2.0", ID: float64(1)}, mustCallParams(t, mcpcontract.StructuralContextTool, bad))
	if invalid.Error == nil || invalid.Error.Code != -32602 {
		t.Fatalf("ASSERT_STRUCTURAL_CONTEXT_TIMEOUT_RELATION: %+v", invalid)
	}
}

func TestStructuralContextOperation35PreservesTypedDomainFailures(t *testing.T) {
	legal := map[transientstructural.Phase][]transientstructural.TerminalState{
		transientstructural.PhasePreflight: {transientstructural.StateUnsupported, transientstructural.StateAmbiguousTarget, transientstructural.StateTargetNotFound, transientstructural.StateResourceLimit, transientstructural.StateTimeout, transientstructural.StateCancelled, transientstructural.StateGenerationChanged, transientstructural.StateInvalidServerResponse},
		transientstructural.PhaseTraversal: {transientstructural.StatePartial, transientstructural.StateTruncated, transientstructural.StateResourceLimit, transientstructural.StateTimeout, transientstructural.StateCancelled, transientstructural.StateGenerationChanged, transientstructural.StateInvalidServerResponse},
		transientstructural.PhaseAdmission: {transientstructural.StateResourceLimit, transientstructural.StateCancelled, transientstructural.StateGenerationChanged, transientstructural.StateInvalidServerResponse},
		transientstructural.PhaseAnalysis:  {transientstructural.StateResourceLimit, transientstructural.StateTimeout, transientstructural.StateCancelled, transientstructural.StateGenerationChanged, transientstructural.StateAnalysisFailed},
	}
	for phase, states := range legal {
		for _, state := range states {
			t.Run(string(phase)+"/"+string(state), func(t *testing.T) {
				executor := &structuralContextRecordingExecutor{failure: &operation.Failure{Code: string(state), Err: &transientstructural.DomainFailure{Phase: phase, State: state}}}
				server := &Server{Registry: NewRegistryWithProfile(false, ToolProfileFull), Executors: map[ExecutorFamily]Executor{StructuralContextExecutorFamily: executor}}
				response := server.callContext(context.Background(), response{JSONRPC: "2.0", ID: float64(1)}, mustCallParams(t, mcpcontract.StructuralContextTool, structuralContextArgs()))
				call := response.Result.(callResult)
				if call.StructuredContent.Phase != string(phase) || call.StructuredContent.State != string(state) {
					t.Fatalf("ASSERT_STRUCTURAL_CONTEXT_PRESERVES_TYPED_DOMAIN_FAILURE: %+v", call.StructuredContent)
				}
				if state == transientstructural.StateTruncated {
					raw, _ := json.Marshal(call.StructuredContent)
					var projected map[string]any
					_ = json.Unmarshal(raw, &projected)
					if projected["diagnostic"] == nil {
						t.Fatalf("ASSERT_STRUCTURAL_CONTEXT_TRUNCATION_ACTIONABLE: %s", raw)
					}
				}
			})
		}
	}
}

func TestStructuralContextV2TraversalFailureReasonProjection(t *testing.T) {
	executor := &structuralContextRecordingExecutor{failure: &operation.Failure{Code: "INVALID_SERVER_RESPONSE"}}
	server := &Server{Registry: NewRegistryWithProfile(false, ToolProfileFull), Executors: map[ExecutorFamily]Executor{StructuralContextV2ExecutorFamily: executor}}
	response := server.callContext(context.Background(), response{JSONRPC: "2.0", ID: float64(1)}, mustCallParams(t, mcpcontract.StructuralContextV2Tool, structuralContextV2Args()))
	env := response.Result.(callResult).StructuredContent
	raw, _ := json.Marshal(env)
	var projected map[string]any
	_ = json.Unmarshal(raw, &projected)
	if env.Phase != "TRAVERSAL" || env.State != "INVALID_SERVER_RESPONSE" || projected["error"].(map[string]any)["code"] != "INVALID_SERVER_RESPONSE" || projected["reason"] != "TRAVERSAL_FAILED" {
		t.Fatalf("ASSERT_STRUCTURAL_CONTEXT_V2_GENERIC_FAILURE_REASON: %s", raw)
	}
	if err := mcpcontract.ValidateJSON(mcpcontract.StructuralContextTraversalDomainErrorID, raw); err != nil {
		t.Fatalf("ASSERT_STRUCTURAL_CONTEXT_V2_GENERIC_FAILURE_REASON_SCHEMA: %v\\n%s", err, raw)
	}
}

func TestStructuralContextV2TraversalDiagnosticsDirectGatewayParity(t *testing.T) {
	depthZero, depthOne := 0, 1
	cases := []struct {
		name       string
		diagnostic *transientstructural.TraversalDiagnostic
		want       map[string]any
	}{
		{name: "prepare", diagnostic: &transientstructural.TraversalDiagnostic{Stage: transientstructural.TraversalStagePrepare, Method: "textDocument/prepareCallHierarchy"}, want: map[string]any{"stage": "PREPARE", "method": "textDocument/prepareCallHierarchy"}},
		{name: "outgoing", diagnostic: &transientstructural.TraversalDiagnostic{Stage: transientstructural.TraversalStageOutgoing, Method: "callHierarchy/outgoingCalls", Direction: transientstructural.DirectionOutgoing, Depth: &depthOne}, want: map[string]any{"stage": "OUTGOING", "method": "callHierarchy/outgoingCalls", "direction": "OUTGOING", "depth": float64(1)}},
		{name: "incoming", diagnostic: &transientstructural.TraversalDiagnostic{Stage: transientstructural.TraversalStageIncoming, Method: "callHierarchy/incomingCalls", Direction: transientstructural.DirectionIncoming, Depth: &depthZero}, want: map[string]any{"stage": "INCOMING", "method": "callHierarchy/incomingCalls", "direction": "INCOMING", "depth": float64(0)}},
		{name: "malformed-response", diagnostic: &transientstructural.TraversalDiagnostic{Stage: transientstructural.TraversalStageOutgoing, Method: "callHierarchy/outgoingCalls", Direction: transientstructural.DirectionOutgoing, ProviderMethod: "callHierarchy/outgoingCalls", FailedField: "response", FailedInvariant: "ARRAY_RESULT", ProviderVariant: "NON_ARRAY", ProjectionEntered: func() *bool { value := false; return &value }(), Guidance: "RETRY_PROVIDER_OR_REPORT_MALFORMED_RESPONSE"}, want: map[string]any{"stage": "OUTGOING", "method": "callHierarchy/outgoingCalls", "direction": "OUTGOING", "provider_method": "callHierarchy/outgoingCalls", "failed_field": "response", "failed_invariant": "ARRAY_RESULT", "provider_variant": "NON_ARRAY", "projection_entered": false, "guidance": "RETRY_PROVIDER_OR_REPORT_MALFORMED_RESPONSE"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			executor := &structuralContextRecordingExecutor{failure: &operation.Failure{Code: "INVALID_SERVER_RESPONSE", Err: &transientstructural.DomainFailure{Phase: transientstructural.PhaseTraversal, State: transientstructural.StateInvalidServerResponse, TraversalDiagnostic: tc.diagnostic}}}
			server := &Server{Registry: NewRegistryWithProfile(false, ToolProfileFull), Executors: map[ExecutorFamily]Executor{StructuralContextV2ExecutorFamily: executor}}
			args := structuralContextV2Args()
			direct := server.callContext(context.Background(), response{JSONRPC: "2.0", ID: float64(1)}, mustCallParams(t, mcpcontract.StructuralContextV2Tool, args))
			gateway := server.callContext(context.Background(), response{JSONRPC: "2.0", ID: float64(2)}, mustCallParams(t, "lsp_trace_v1_execute", map[string]any{"request": map[string]any{"operation": mcpcontract.StructuralContextV2Tool, "arguments": args}}))
			if direct.Error != nil || gateway.Error != nil {
				t.Fatalf("ASSERT_STRUCTURAL_CONTEXT_%s_TRANSPORT: direct=%v gateway=%v", strings.ToUpper(tc.name), direct.Error, gateway.Error)
			}
			directEnvelope := direct.Result.(callResult).StructuredContent
			outer := gateway.Result.(callResult).StructuredContent
			var gatewayEnvelope envelope
			if err := json.Unmarshal([]byte(outer.DelegatedEnvelope), &gatewayEnvelope); err != nil {
				t.Fatalf("ASSERT_STRUCTURAL_CONTEXT_%s_GATEWAY_DECODE: %v", strings.ToUpper(tc.name), err)
			}
			for label, env := range map[string]envelope{"direct": directEnvelope, "gateway": gatewayEnvelope} {
				raw, _ := json.Marshal(env.Diagnostic)
				var got map[string]any
				_ = json.Unmarshal(raw, &got)
				if !reflect.DeepEqual(got, tc.want) {
					t.Fatalf("ASSERT_STRUCTURAL_CONTEXT_%s_%s_DIAGNOSTIC: got=%v want=%v", strings.ToUpper(tc.name), strings.ToUpper(label), got, tc.want)
				}
				if env.EnvelopeSchemaID != mcpcontract.StructuralContextTraversalDomainErrorID {
					t.Fatalf("ASSERT_STRUCTURAL_CONTEXT_ADDITIVE_V4_%s: %v", strings.ToUpper(label), env.EnvelopeSchemaID)
				}
			}
		})
	}
}

type ambiguousStructuralContextSymbolRuntime struct{}

func (ambiguousStructuralContextSymbolRuntime) Metadata(string, uint64) (sessionruntime.SessionMetadata, session.Failure) {
	return sessionruntime.SessionMetadata{WorkspaceSymbolSupport: true}, ""
}

func (ambiguousStructuralContextSymbolRuntime) Records() []sessionruntime.Record {
	return []sessionruntime.Record{{SessionID: "s", Generation: 1, State: session.Ready, Routing: sessionruntime.RoutingMetadata{WorkspaceRoot: "/workspace"}}}
}

func (ambiguousStructuralContextSymbolRuntime) RoundTrip(context.Context, sessionruntime.RoundTripRequest) sessionruntime.RoundTripResult {
	return sessionruntime.RoundTripResult{Result: json.RawMessage(`[{"name":"run","kind":12,"location":{"uri":"file:///workspace/a.go","range":{"start":{"line":1,"character":0},"end":{"line":1,"character":3}}}},{"name":"run","kind":12,"location":{"uri":"file:///workspace/b.go","range":{"start":{"line":2,"character":0},"end":{"line":2,"character":3}}}}]`)}
}

func TestStructuralContextV2RealSymbolLocatorAmbiguityDirectGatewayParityAndPrivacy(t *testing.T) {
	delegate := &structuralContextRecordingExecutor{}
	executor := structuralcontextsymbolops.NewUnifiedV2Executor(ambiguousStructuralContextSymbolRuntime{}, delegate)
	server := &Server{Registry: NewRegistryWithProfile(false, ToolProfileFull), Executors: map[ExecutorFamily]Executor{StructuralContextV2ExecutorFamily: executor}}
	args := map[string]any{"symbol": "run", "session_id": "s", "generation": float64(1), "up_depth": float64(0), "down_depth": float64(0), "max_nodes": float64(10), "analysis": map[string]any{"kind": "NEIGHBORHOOD"}, "request_timeout_ms": float64(15000), "timeout_ms": float64(30000)}
	direct := server.callContext(context.Background(), response{JSONRPC: "2.0", ID: float64(1)}, mustCallParams(t, mcpcontract.StructuralContextV2Tool, args))
	gateway := server.callContext(context.Background(), response{JSONRPC: "2.0", ID: float64(2)}, mustCallParams(t, "lsp_trace_v1_execute", map[string]any{"request": map[string]any{"operation": mcpcontract.StructuralContextV2Tool, "arguments": args}}))
	if direct.Error != nil || gateway.Error != nil {
		t.Fatalf("ASSERT_REAL_SYMBOL_AMBIGUITY_TYPED_TRANSPORT: direct=%v gateway=%v", direct.Error, gateway.Error)
	}
	directEnvelope := direct.Result.(callResult).StructuredContent
	outer := gateway.Result.(callResult).StructuredContent
	var gatewayEnvelope envelope
	if err := json.Unmarshal([]byte(outer.DelegatedEnvelope), &gatewayEnvelope); err != nil {
		t.Fatalf("ASSERT_REAL_SYMBOL_AMBIGUITY_GATEWAY_DECODE: %v", err)
	}
	for label, env := range map[string]envelope{"direct": directEnvelope, "gateway": gatewayEnvelope} {
		raw, _ := json.Marshal(env)
		if env.EnvelopeSchemaID != mcpcontract.StructuralContextTraversalDomainErrorID || env.Phase != "PREFLIGHT" || env.State != "AMBIGUOUS_TARGET" || env.TargetDiagnostic == nil {
			t.Fatalf("ASSERT_REAL_SYMBOL_AMBIGUITY_%s_V4: %s", strings.ToUpper(label), raw)
		}
		if err := mcpcontract.ValidateJSON(mcpcontract.StructuralContextTraversalDomainErrorID, raw); err != nil {
			t.Fatalf("ASSERT_REAL_SYMBOL_AMBIGUITY_%s_SCHEMA: %v\n%s", strings.ToUpper(label), err, raw)
		}
		if !strings.Contains(string(raw), `"candidate_accounting"`) || strings.Count(string(raw), `"uri":"file:///workspace/`) != 2 {
			t.Fatalf("ASSERT_REAL_SYMBOL_AMBIGUITY_%s_BOUNDED_CANDIDATES: %s", strings.ToUpper(label), raw)
		}
		for _, forbidden := range []string{`"target"`, "source", "selector", "provider", "request_timeout_ms"} {
			if strings.Contains(string(raw), forbidden) {
				t.Fatalf("ASSERT_REAL_SYMBOL_AMBIGUITY_%s_PRIVACY: leaked %q in %s", strings.ToUpper(label), forbidden, raw)
			}
		}
	}
	if len(delegate.calls) != 0 {
		t.Fatalf("ASSERT_REAL_SYMBOL_AMBIGUITY_NO_CANDIDATE_SELECTION_OR_RETRY: calls=%d", len(delegate.calls))
	}
}

func TestStructuralContextV2TruncatedRecoveryDirectGatewayParity(t *testing.T) {
	entered := false
	zero := 0
	diagnostic := &transientstructural.TargetDiagnostic{
		ExactMatches: 0, TotalSymbols: 9, OmittedSymbols: 1,
		Action:         transientstructural.TargetActionEnumerationTruncated,
		ProviderMethod: "textDocument/documentSymbol", NormalizationStage: "EXACT_MATCH",
		FailedField: "range", FailedInvariant: "POSITION_CONTAINMENT_PRESENT", ProjectionEntered: &entered,
		Completeness: "UNKNOWN",
		Recoveries: []transientstructural.TargetRecovery{
			{Kind: "POSITION_LOCATOR_TEMPLATE", RequiredFields: []string{"uri", "line", "character"}},
			{Kind: "REGEX_LOCATOR_TEMPLATE", RequiredPattern: true, MatchIndex: &zero, RequiredFields: []string{"uri", "pattern", "match_index"}, Limits: map[string]int{"max_document_bytes": 60 * 1024, "max_pattern_bytes": 4 * 1024, "max_work": 64 * 1024, "max_matches": 100}},
		},
	}
	executor := &structuralContextRecordingExecutor{failure: &operation.Failure{Code: "ENUMERATION_TRUNCATED", Err: &transientstructural.DomainFailure{Phase: transientstructural.PhasePreflight, State: transientstructural.StateTargetNotFound, TargetDiagnostic: diagnostic}}}
	server := &Server{Registry: NewRegistryWithProfile(false, ToolProfileFull), Executors: map[ExecutorFamily]Executor{StructuralContextV2ExecutorFamily: executor}}
	args := structuralContextV2Args()
	direct := server.callContext(context.Background(), response{JSONRPC: "2.0", ID: float64(1)}, mustCallParams(t, mcpcontract.StructuralContextV2Tool, args))
	gateway := server.callContext(context.Background(), response{JSONRPC: "2.0", ID: float64(2)}, mustCallParams(t, "lsp_trace_v1_execute", map[string]any{"request": map[string]any{"operation": mcpcontract.StructuralContextV2Tool, "arguments": args}}))
	if direct.Error != nil || gateway.Error != nil {
		t.Fatalf("ASSERT_TRUNCATED_RECOVERY_TYPED_TRANSPORT: direct=%v gateway=%v", direct.Error, gateway.Error)
	}
	directEnvelope := direct.Result.(callResult).StructuredContent
	outer := gateway.Result.(callResult).StructuredContent
	var gatewayEnvelope envelope
	if err := json.Unmarshal([]byte(outer.DelegatedEnvelope), &gatewayEnvelope); err != nil {
		t.Fatalf("ASSERT_TRUNCATED_RECOVERY_GATEWAY_DECODE: %v", err)
	}
	for label, env := range map[string]envelope{"direct": directEnvelope, "gateway": gatewayEnvelope} {
		raw, _ := json.Marshal(env)
		for _, required := range []string{`"action":"ENUMERATION_TRUNCATED"`, `"completeness":"UNKNOWN"`, `"provider_method":"textDocument/documentSymbol"`, `"normalization_stage":"EXACT_MATCH"`, `"failed_field":"range"`, `"failed_invariant":"POSITION_CONTAINMENT_PRESENT"`, `"kind":"POSITION_LOCATOR_TEMPLATE"`, `"kind":"REGEX_LOCATOR_TEMPLATE"`} {
			if !strings.Contains(string(raw), required) {
				t.Fatalf("ASSERT_TRUNCATED_RECOVERY_%s_PARITY missing %s: %s", strings.ToUpper(label), required, raw)
			}
		}
		if err := mcpcontract.ValidateJSON(mcpcontract.StructuralContextTraversalDomainErrorID, raw); err != nil {
			t.Fatalf("ASSERT_TRUNCATED_RECOVERY_%s_SCHEMA: %v\n%s", strings.ToUpper(label), err, raw)
		}
	}
}

func TestStructuralContextV2UnsupportedRecoveryFragmentDirectGatewayParityAndPrivacy(t *testing.T) {
	complete := false
	diagnostic := &transientstructural.TargetDiagnostic{
		Action: transientstructural.TargetActionFailUnsupported, ProviderMethod: "textDocument/prepareCallHierarchy", LocatorScope: "URI_POSITION",
		Recovery: &transientstructural.TargetRecovery{Kind: "SOURCE_ONLY_REQUEST_TEMPLATE", Complete: &complete, OmittedFields: []string{"projection.privacy_policy_id"}, RequestFragment: map[string]any{
			"session_id": "s", "generation": uint64(1), "uri": "file:///workspace/a.go", "line": 2, "character": 3, "up_depth": 0, "down_depth": 0,
			"projection": map[string]any{"mode": "TARGET", "body": "INCLUDE", "include_relation_occurrences": false},
		}},
	}
	executor := &structuralContextRecordingExecutor{failure: &operation.Failure{Code: "UNSUPPORTED", Err: &transientstructural.DomainFailure{Phase: transientstructural.PhasePreflight, State: transientstructural.StateUnsupported, TargetDiagnostic: diagnostic}}}
	server := &Server{Registry: NewRegistryWithProfile(false, ToolProfileFull), Executors: map[ExecutorFamily]Executor{StructuralContextV2ExecutorFamily: executor}}
	args := structuralContextV2Args()
	direct := server.callContext(context.Background(), response{JSONRPC: "2.0", ID: float64(1)}, mustCallParams(t, mcpcontract.StructuralContextV2Tool, args))
	gateway := server.callContext(context.Background(), response{JSONRPC: "2.0", ID: float64(2)}, mustCallParams(t, "lsp_trace_v1_execute", map[string]any{"request": map[string]any{"operation": mcpcontract.StructuralContextV2Tool, "arguments": args}}))
	if direct.Error != nil || gateway.Error != nil {
		t.Fatalf("ASSERT_UNSUPPORTED_RECOVERY_TRANSPORT: direct=%v gateway=%v", direct.Error, gateway.Error)
	}
	directEnvelope := direct.Result.(callResult).StructuredContent
	outer := gateway.Result.(callResult).StructuredContent
	var gatewayEnvelope envelope
	if err := json.Unmarshal([]byte(outer.DelegatedEnvelope), &gatewayEnvelope); err != nil {
		t.Fatal(err)
	}
	directRaw, _ := json.Marshal(directEnvelope)
	gatewayRaw, _ := json.Marshal(gatewayEnvelope)
	for label, raw := range map[string][]byte{"direct": directRaw, "gateway": gatewayRaw} {
		for _, required := range []string{`"kind":"SOURCE_ONLY_REQUEST_TEMPLATE"`, `"omitted_fields":["projection.privacy_policy_id"]`, `"complete":false`, `"mode":"TARGET"`, `"up_depth":0`, `"down_depth":0`} {
			if !strings.Contains(string(raw), required) {
				t.Fatalf("ASSERT_UNSUPPORTED_RECOVERY_%s_PARITY missing %s: %s", label, required, raw)
			}
		}
		if err := mcpcontract.ValidateJSON(mcpcontract.StructuralContextTraversalDomainErrorID, raw); err != nil {
			t.Fatalf("ASSERT_UNSUPPORTED_RECOVERY_%s_SCHEMA: %v\n%s", label, err, raw)
		}
		for _, forbidden := range []string{"SECRET_SOURCE", "textDocument/definition", "textDocument/references"} {
			if strings.Contains(string(raw), forbidden) {
				t.Fatalf("ASSERT_UNSUPPORTED_RECOVERY_%s_PRIVACY: leaked %q", label, forbidden)
			}
		}
	}
}

func TestStructuralContextV2TargetDiagnosticsDirectGatewayParityAndPrivacy(t *testing.T) {
	diagnostic := &transientstructural.TargetDiagnostic{ExactMatches: 2, TotalSymbols: 9, OmittedSymbols: 1, Action: transientstructural.TargetActionFailAmbiguous}
	executor := &structuralContextRecordingExecutor{failure: &operation.Failure{Code: "AMBIGUOUS_TARGET", Err: &transientstructural.DomainFailure{Phase: transientstructural.PhasePreflight, State: transientstructural.StateAmbiguousTarget, TargetDiagnostic: diagnostic}}}
	server := &Server{Registry: NewRegistryWithProfile(false, ToolProfileFull), Executors: map[ExecutorFamily]Executor{StructuralContextV2ExecutorFamily: executor}}
	args := structuralContextV2Args()
	direct := server.callContext(context.Background(), response{JSONRPC: "2.0", ID: float64(1)}, mustCallParams(t, mcpcontract.StructuralContextV2Tool, args))
	gateway := server.callContext(context.Background(), response{JSONRPC: "2.0", ID: float64(2)}, mustCallParams(t, "lsp_trace_v1_execute", map[string]any{"request": map[string]any{"operation": mcpcontract.StructuralContextV2Tool, "arguments": args}}))
	if direct.Error != nil || gateway.Error != nil {
		t.Fatalf("ASSERT_STRUCTURAL_CONTEXT_TARGET_DIAGNOSTIC_TRANSPORT: direct=%v gateway=%v", direct.Error, gateway.Error)
	}
	directEnvelope := direct.Result.(callResult).StructuredContent
	outer := gateway.Result.(callResult).StructuredContent
	var gatewayEnvelope envelope
	if err := json.Unmarshal([]byte(outer.DelegatedEnvelope), &gatewayEnvelope); err != nil {
		t.Fatalf("ASSERT_STRUCTURAL_CONTEXT_TARGET_DIAGNOSTIC_GATEWAY_DECODE: %v", err)
	}
	want := map[string]any{"exact_matches": float64(2), "total_symbols": float64(9), "omitted_symbols": float64(1), "action": "FAIL_AMBIGUOUS"}
	for label, env := range map[string]envelope{"direct": directEnvelope, "gateway": gatewayEnvelope} {
		rawDiagnostic, _ := json.Marshal(env.TargetDiagnostic)
		var got map[string]any
		_ = json.Unmarshal(rawDiagnostic, &got)
		if !reflect.DeepEqual(got, want) || env.Diagnostic != nil || env.Target != nil || env.Phase != "PREFLIGHT" || env.State != "AMBIGUOUS_TARGET" || env.EnvelopeSchemaID != mcpcontract.StructuralContextTraversalDomainErrorID {
			t.Fatalf("ASSERT_STRUCTURAL_CONTEXT_TARGET_DIAGNOSTIC_%s_PARITY: env=%+v diagnostic=%v", strings.ToUpper(label), env, got)
		}
		raw, _ := json.Marshal(env)
		for _, forbidden := range []string{`"target"`, `"symbol"`, `"A"`, "file:///", "/w/", `"line"`, `"character"`, "source", "selector", "provider", "environment", "privacy_policy"} {
			if strings.Contains(string(raw), forbidden) {
				t.Fatalf("ASSERT_STRUCTURAL_CONTEXT_TARGET_DIAGNOSTIC_%s_FORBIDDEN_FIELDS_ABSENT: leaked %q in %s", strings.ToUpper(label), forbidden, raw)
			}
		}
	}
}

// Lane D synthetic UX-12: validation is exercised on both existing transports.
// corrected is a test-only suggestion, not a product diagnostic or new API.
func TestLaneDUX12ValidationVsDomainDirectGateway(t *testing.T) {
	executor := &structuralContextRecordingExecutor{failure: &operation.Failure{Code: "TARGET_NOT_FOUND", Err: &transientstructural.DomainFailure{Phase: transientstructural.PhasePreflight, State: transientstructural.StateTargetNotFound}}}
	server := &Server{Registry: NewRegistryWithProfile(false, ToolProfileFull), Executors: map[ExecutorFamily]Executor{StructuralContextV2ExecutorFamily: executor}}
	valid := structuralContextV2Args()
	invalid := structuralContextV2Args()
	delete(invalid, "session_id")
	corrected := map[string]any{"session_id": "s"} // synthetic correction; not returned by MCP
	if corrected["session_id"] != valid["session_id"] {
		t.Fatal("UX12_TEST_ONLY_CORRECTED_FRAGMENT")
	}
	for _, tc := range []struct {
		name    string
		args    map[string]any
		gateway bool
	}{
		{"direct-invalid", invalid, false}, {"gateway-invalid", invalid, true},
		{"direct-domain", valid, false}, {"gateway-domain", valid, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			method := mcpcontract.StructuralContextV2Tool
			args := tc.args
			if tc.gateway {
				method = "lsp_trace_v1_execute"
				args = map[string]any{"request": map[string]any{"operation": mcpcontract.StructuralContextV2Tool, "arguments": tc.args}}
			}
			got := server.callContext(context.Background(), response{JSONRPC: "2.0", ID: float64(1)}, mustCallParams(t, method, args))
			if strings.Contains(tc.name, "invalid") {
				if got.Error == nil || got.Error.Code != -32602 || got.Result != nil {
					t.Fatalf("UX12_VALIDATION_STAGE_%s: %+v", tc.name, got)
				}
			} else {
				if got.Error != nil || got.Result == nil {
					t.Fatalf("UX12_DOMAIN_NOT_VALIDATION_%s: %+v", tc.name, got)
				}
				call, ok := got.Result.(callResult)
				if !ok {
					t.Fatalf("UX12_DOMAIN_RESULT_SHAPE_%s: %T", tc.name, got.Result)
				}
				domain := call.StructuredContent
				if tc.gateway {
					if domain.DelegatedEnvelope == "" {
						t.Fatalf("UX12_GATEWAY_DELEGATED_ENVELOPE_MISSING: %+v", domain)
					}
					if err := json.Unmarshal([]byte(domain.DelegatedEnvelope), &domain); err != nil {
						t.Fatalf("UX12_GATEWAY_DELEGATED_ENVELOPE_DECODE: %v", err)
					}
				}
				if domain.EnvelopeSchemaID != mcpcontract.StructuralContextTraversalDomainErrorID || domain.Phase != "PREFLIGHT" || domain.State != "TARGET_NOT_FOUND" {
					t.Fatalf("UX12_TYPED_DOMAIN_CODE_STAGE_%s: %+v", tc.name, domain)
				}
			}
		})
	}
	if len(executor.calls) != 2 {
		t.Fatalf("UX12_OBSERVED_EXECUTOR_CALLS_VALID_ONLY: %d", len(executor.calls))
	}
}

func TestStructuralContextOperation35UnknownUntypedFailureFailsClosed(t *testing.T) {
	executor := &structuralContextRecordingExecutor{failure: &operation.Failure{Code: "UNKNOWN_FUTURE_CODE"}}
	server := &Server{Registry: NewRegistryWithProfile(false, ToolProfileFull), Executors: map[ExecutorFamily]Executor{StructuralContextExecutorFamily: executor}}
	response := server.callContext(context.Background(), response{JSONRPC: "2.0", ID: float64(1)}, mustCallParams(t, mcpcontract.StructuralContextTool, structuralContextArgs()))
	call := response.Result.(callResult)
	if call.StructuredContent.Phase != "TRAVERSAL" || call.StructuredContent.State != "INVALID_SERVER_RESPONSE" {
		t.Fatalf("ASSERT_STRUCTURAL_CONTEXT_UNKNOWN_FAILURE_FAILS_CLOSED: %+v", call.StructuredContent)
	}
}

func TestStructuralContextOperation35DirectGatewayParity(t *testing.T) {
	reasons := transientstructuralresult.EmptyReasonMap()
	accounting := transientstructuralresult.Accounting{RequestAttempted: 1, RequestSucceeded: 1, PreparedAttempted: 1, PreparedReturned: 1, NodeObserved: 1, NodeAdmitted: 1, FrontierObserved: 1, FrontierExpanded: 1, RequestOmissionReasons: reasons, NodeOmissionReasons: transientstructuralresult.EmptyReasonMap(), OccurrenceOmissionReasons: transientstructuralresult.EmptyReasonMap(), FrontierOmissionReasons: transientstructuralresult.EmptyReasonMap()}
	q := transientstructuralresult.Request{Generation: 1, DownDepth: 2, UpDepth: 0, MaxNodes: 100, TimeoutMS: 5000, RequestTimeoutMS: 1000, MaxMessages: 64, MaxBytes: 4194304, Analysis: transientstructuralresult.AnalysisRequest{Kind: transientstructuralresult.Neighborhood}}
	node, _ := transientstructuralresult.NodeID("ts_0123456789abcdef0123456789abcdef", 1, "A")
	result := transientstructuralresult.NewResult("ts_0123456789abcdef0123456789abcdef", 1, node, "utf-16", q, accounting, transientstructuralresult.NeighborhoodResult{RootNodeID: node, Nodes: []transientstructuralresult.Node{{ID: node}}, Edges: []transientstructuralresult.Edge{}})
	artifact, _ := json.Marshal(result)
	if err := mcpcontract.ValidateJSON(mcpcontract.StructuralContextResultID, artifact); err != nil {
		t.Fatalf("ASSERT_STRUCTURAL_CONTEXT_FIXTURE_SCHEMA: %v\n%s", err, artifact)
	}
	executor := &structuralContextRecordingExecutor{artifact: artifact}
	server := &Server{Registry: NewRegistryWithProfile(false, ToolProfileFull), Executors: map[ExecutorFamily]Executor{StructuralContextExecutorFamily: executor}}
	args := structuralContextArgs()
	direct := server.callContext(context.Background(), response{JSONRPC: "2.0", ID: float64(1)}, mustCallParams(t, mcpcontract.StructuralContextTool, args))
	gateway := server.callContext(context.Background(), response{JSONRPC: "2.0", ID: float64(2)}, mustCallParams(t, "lsp_trace_v1_execute", map[string]any{"request": map[string]any{"operation": mcpcontract.StructuralContextTool, "arguments": args}}))
	if direct.Error != nil || gateway.Error != nil || len(executor.calls) != 2 {
		t.Fatalf("ASSERT_STRUCTURAL_CONTEXT_DIRECT_GATEWAY: direct=%v gateway=%v calls=%d", direct.Error, gateway.Error, len(executor.calls))
	}
	var a, b map[string]any
	_ = json.Unmarshal(executor.calls[0].Input, &a)
	_ = json.Unmarshal(executor.calls[1].Input, &b)
	if !reflect.DeepEqual(a, b) {
		t.Fatal("ASSERT_STRUCTURAL_CONTEXT_ARGUMENT_PARITY")
	}
	directCall := direct.Result.(callResult)
	raw, _ := json.Marshal(directCall.StructuredContent)
	if !strings.Contains(string(raw), `"outcome":"EMPTY"`) || !strings.Contains(string(raw), `"authority":0`) || strings.Contains(string(raw), "output_selector") {
		t.Fatalf("ASSERT_STRUCTURAL_CONTEXT_DIRECT_RESULT: %s", raw)
	}
	raw, _ = json.Marshal(gateway.Result)
	if !strings.Contains(string(raw), `"delegated_outcome":"EMPTY"`) {
		t.Fatalf("ASSERT_STRUCTURAL_CONTEXT_GATEWAY_RESULT: %s", raw)
	}
}
