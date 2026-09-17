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
	"lsp-trace/internal/transientstructuralresult"
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
