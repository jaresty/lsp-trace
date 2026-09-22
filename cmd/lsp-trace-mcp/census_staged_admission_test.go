package main

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"lsp-trace/internal/censusrequest"
	"lsp-trace/internal/mcp"
	"lsp-trace/internal/mcpcontract"
	"lsp-trace/internal/operation"
)

func TestContinuationInternalBatchTargetsRequiresClosedContext(t *testing.T) {
	runtime, _, _ := censusRuntimeFixture(t, censusRuntimeOptions{
		ready: true, documentSymbolSupport: true, callHierarchySupport: true,
	})
	_, failure, reason := newCensusExecutor(runtime).execute(context.Background(), []byte(`{"session_id":"project","generation":1,"sources":["."],"batch_targets":16}`))
	if failure == nil {
		t.Fatalf("ASSERT_INTERNAL_BATCH_TARGETS_FAILS_CLOSED failure=nil reason=%s", reason)
	}
	if failure.stage != censusStageConfig || failure.code != censusCodeInvalidConfig || reason != reasonRequestUndecodable {
		t.Fatalf("ASSERT_INTERNAL_BATCH_TARGETS_FAILS_CLOSED stage=%v code=%v reason=%s", failure.stage, failure.code, reason)
	}
}

type censusAdmissionProbeRunner struct {
	executor *censusExecutor
	calls    int
	batch    int
	input    []byte
	admitted censusAdmittedSession
	reason   censusFailureReason
}

func (r *censusAdmissionProbeRunner) run(ctx context.Context, request operation.Request) censusCompletion {
	r.calls++
	r.batch = censusBatchTargetsFromContext(ctx)
	r.input = append([]byte(nil), request.Input...)
	admitted, failure, reason := r.executor.execute(ctx, request.Input)
	r.admitted, r.reason = admitted, reason
	if failure != nil {
		return censusFailureCompletion(failure, reason)
	}
	result := privateCensusSuccessFixture()
	return censusCompletion{Result: &result}
}

func runCensusAdmissionRoute(t *testing.T, gateway bool, input map[string]any) ([]byte, *censusAdmissionProbeRunner, *continuationMCPHostFixture, bool) {
	t.Helper()
	runtime, _, _ := censusRuntimeFixture(t, censusRuntimeOptions{ready: true, documentSymbolSupport: true, callHierarchySupport: true})
	probe := &censusAdmissionProbeRunner{executor: newCensusExecutor(runtime)}
	census := privateCensusSuccessFixture()
	host := &continuationMCPHostFixture{result: censusContinuationMCPResult{Census: &census, Descriptor: "g-" + strings.Repeat("a", 64) + ".selector.json", Status: "PAUSED"}}
	server := &mcp.Server{Registry: mcp.NewRegistryWithProfile(false, mcp.ToolProfileFull), Executors: map[mcp.ExecutorFamily]mcp.Executor{mcp.CensusExecutorFamily: newPrivateCensusMCPBinding(probe, host)}}
	name, arguments := mcpcontract.CensusTool, input
	if gateway {
		name = "lsp_trace_v1_execute"
		arguments = map[string]any{"request": map[string]any{"operation": mcpcontract.CensusTool, "arguments": input}}
	}
	params, err := json.Marshal(map[string]any{"name": name, "arguments": arguments})
	if err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err := server.Serve(strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":`+string(params)+"}\n"), &output); err != nil {
		t.Fatal(err)
	}
	var wire struct {
		Error *struct {
			Code    float64 `json:"code"`
			Message string  `json:"message"`
		} `json:"error"`
		Result *struct {
			Structured json.RawMessage `json:"structuredContent"`
		} `json:"result"`
	}
	if err := json.Unmarshal(bytes.TrimSpace(output.Bytes()), &wire); err != nil {
		t.Fatalf("decode response %q: %v", output.String(), err)
	}
	if wire.Error != nil {
		return nil, probe, host, true
	}
	if wire.Result == nil {
		t.Fatalf("missing result and error: %s", output.Bytes())
	}
	delegated := append([]byte(nil), wire.Result.Structured...)
	if gateway {
		var outer map[string]any
		if err := json.Unmarshal(delegated, &outer); err != nil {
			t.Fatal(err)
		}
		encoded, ok := outer["delegated_envelope"].(string)
		if !ok {
			t.Fatalf("gateway missing delegated envelope: %v", outer)
		}
		delegated = []byte(encoded)
	}
	return delegated, probe, host, false
}

func TestProductionGatewayContinuationAdmissionPropagatesEffectiveBatchTargets(t *testing.T) {
	base := map[string]any{
		"session_id": "project", "generation": 1,
		"sources":    []string{"internal/censuscontinuation/pipeline.go", "internal/censuscontinuation/capture.go"},
		"down_depth": 1, "up_depth": 0, "max_nodes": 10000,
		"timeout_ms": 60000, "request_timeout_ms": 30000,
		"continuation": map[string]any{"kind": "ADR_0007_FEATURE_CATALOG", "stop_after": "DESCRIBE_REQUESTS"},
	}
	cases := []struct {
		name  string
		batch any
		want  uint64
	}{
		{name: "explicit-16", batch: 16, want: 16},
		{name: "explicit-7", batch: 7, want: 7},
		{name: "omitted-default-16", want: 16},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			input := make(map[string]any, len(base)+1)
			for key, value := range base {
				input[key] = value
			}
			if tc.batch != nil {
				input["batch_targets"] = tc.batch
			}
			direct, directProbe, directHost, directRPCError := runCensusAdmissionRoute(t, false, input)
			gateway, gatewayProbe, gatewayHost, gatewayRPCError := runCensusAdmissionRoute(t, true, input)
			for route, observed := range map[string]struct {
				probe    *censusAdmissionProbeRunner
				host     *continuationMCPHostFixture
				rpcError bool
			}{"direct": {directProbe, directHost, directRPCError}, "gateway": {gatewayProbe, gatewayHost, gatewayRPCError}} {
				if observed.rpcError || observed.probe.calls != 1 || observed.probe.reason == reasonRequestUndecodable {
					t.Fatalf("ASSERT_CENSUS_PRIVATE_SANITIZED_PUBLIC_DECODERS_%s rpc_error=%t calls=%d reason=%s", route, observed.rpcError, observed.probe.calls, observed.probe.reason)
				}
				if uint64(observed.probe.batch) != tc.want || observed.probe.admitted.options.maxBatchTargets != tc.want || observed.probe.admitted.requestReceipt.Semantic.BatchTargets != tc.want {
					t.Fatalf("ASSERT_CENSUS_EFFECTIVE_BATCH_AGREEMENT_%s context=%d options=%d receipt=%d want=%d", route, observed.probe.batch, observed.probe.admitted.options.maxBatchTargets, observed.probe.admitted.requestReceipt.Semantic.BatchTargets, tc.want)
				}
				expected, err := censusrequest.New(observed.probe.admitted.requestReceipt.Refresh, observed.probe.admitted.requestReceipt.Semantic)
				if err != nil {
					t.Fatal(err)
				}
				if observed.probe.admitted.requestReceipt.Fingerprint != expected.Fingerprint || !bytes.Equal(observed.probe.admitted.requestReceipt.CanonicalJSON, expected.CanonicalJSON) {
					t.Fatalf("ASSERT_CENSUS_EFFECTIVE_BATCH_FINGERPRINT_%s got=%+v expected=%+v", route, observed.probe.admitted.requestReceipt, expected)
				}
				if observed.host.freshCalls != 1 || observed.host.freshStop != "DESCRIBE_REQUESTS" {
					t.Fatalf("ASSERT_CENSUS_CONTINUATION_ATTACHED_%s calls=%d stop=%q", route, observed.host.freshCalls, observed.host.freshStop)
				}
			}
			if !bytes.Equal(direct, gateway) {
				t.Fatalf("ASSERT_CENSUS_EFFECTIVE_BATCH_DIRECT_GATEWAY_PARITY direct=%s gateway=%s", direct, gateway)
			}
		})
	}

	malformed := make(map[string]any, len(base)+1)
	for key, value := range base {
		malformed[key] = value
	}
	malformed["batch_targets"] = "16"
	for _, gateway := range []bool{false, true} {
		_, probe, host, rpcError := runCensusAdmissionRoute(t, gateway, malformed)
		if !rpcError || probe.calls != 0 || host.freshCalls != 0 {
			t.Fatalf("ASSERT_CENSUS_MALFORMED_PRIVATE_BATCH_FAILS_CLOSED gateway=%t rpc_error=%t calls=%d continuation=%d", gateway, rpcError, probe.calls, host.freshCalls)
		}
	}
}

func TestStagedStopOnlyRequestPassesOfflineAdmission(t *testing.T) {
	runtime, _, started := censusRuntimeFixture(t, censusRuntimeOptions{
		ready: true, documentSymbolSupport: true, callHierarchySupport: true,
	})
	raw, err := json.Marshal(map[string]any{
		"session_id": "project", "generation": started.Generation,
		"sources":    []string{"internal/censuscontinuation/pipeline.go", "internal/censuscontinuation/capture.go"},
		"down_depth": 1, "up_depth": 0, "max_nodes": 10000, "batch_targets": 16,
		"timeout_ms": 60000, "request_timeout_ms": 30000,
		"continuation": map[string]any{"kind": "ADR_0007_FEATURE_CATALOG", "stop_after": "DESCRIBE_REQUESTS"},
	})
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := mcpcontract.DecodeFutureCensusRequestV2(raw)
	if err != nil {
		t.Fatalf("ASSERT_STAGED_REQUEST_V2_SCHEMA_ADMITTED: %v", err)
	}
	fresh := make(map[string]any, len(decoded)-1)
	for key, value := range decoded {
		if key != "continuation" {
			fresh[key] = value
		}
	}
	admissionRaw, err := json.Marshal(fresh)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.WithValue(context.Background(), censusBatchTargetsContextKey{}, 16)
	admitted, failure, reason := newCensusExecutor(runtime).execute(ctx, admissionRaw)
	if failure != nil {
		t.Fatalf("ASSERT_STAGED_REQUEST_OFFLINE_ADMISSION stage=%s code=%s reason=%s", failure.stage, failure.code, reason)
	}
	if admitted.options.maxBatchTargets != 16 || admitted.requestReceipt.Semantic.BatchTargets != 16 {
		t.Fatalf("ASSERT_STAGED_EFFECTIVE_BATCH_TARGETS_PRESERVED: options=%d receipt=%d want=16", admitted.options.maxBatchTargets, admitted.requestReceipt.Semantic.BatchTargets)
	}
}
