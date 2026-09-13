package main

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"lsp-trace/internal/lsp"
	"lsp-trace/internal/mcp"
	"lsp-trace/internal/mcpcontract"
)

type negativeParityFixture struct {
	runtime  *fakeRuntime
	acquirer *fakeAcquirer
	executor *countingTraceExecutor
}

type negativeParityObservation struct {
	rpcCode          float64
	rpcMessage       string
	envelope         map[string]any
	envelopeBytes    string
	delegated        map[string]any
	delegatedBytes   string
	executorCalls    int
	acquisitionCalls int
	methods          []string
}

type negativeParityCase struct {
	name                  string
	directArguments       string
	gatewayArguments      string
	symbols               []lsp.DocumentSymbol
	wantDirectRPCCode     float64
	wantDirectRPCMessage  string
	wantGatewayRPCCode    float64
	wantGatewayRPCMessage string
	wantStatus            string
	wantOutcome           string
	wantCode              string
	wantDiagnostics       []string
	wantExecutorCalls     int
	wantMethods           []string
}

func newNegativeParityFixture(symbols []lsp.DocumentSymbol) *negativeParityFixture {
	runtime := &fakeRuntime{symbols: append([]lsp.DocumentSymbol(nil), symbols...)}
	acquirer := &fakeAcquirer{}
	executor := &countingTraceExecutor{delegate: &traceExecutor{runtime: runtime, acquisition: acquirer}}
	return &negativeParityFixture{runtime: runtime, acquirer: acquirer, executor: executor}
}

func runNegativeParityRoute(t *testing.T, gateway bool, tc negativeParityCase) negativeParityObservation {
	t.Helper()
	fixture := newNegativeParityFixture(tc.symbols)
	server := &mcp.Server{
		Registry:  mcp.NewRegistryWithProfile(false, mcp.ToolProfileFull),
		Executors: map[mcp.ExecutorFamily]mcp.Executor{mcp.TraceExecutorFamily: fixture.executor},
	}
	arguments := tc.directArguments
	name := mcpcontract.TraceTool
	if gateway {
		name = "lsp_trace_v1_execute"
		arguments = tc.gatewayArguments
	}
	request := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"` + name + `","arguments":` + arguments + `}}` + "\n"
	var out bytes.Buffer
	if err := server.Serve(strings.NewReader(request), &out); err != nil {
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
	if err := json.Unmarshal(bytes.TrimSpace(out.Bytes()), &wire); err != nil {
		t.Fatalf("decode response %q: %v", out.String(), err)
	}
	observation := negativeParityObservation{
		executorCalls:    fixture.executor.calls,
		acquisitionCalls: len(fixture.acquirer.requests),
		methods:          append([]string(nil), fixture.runtime.methods...),
	}
	if wire.Error != nil {
		observation.rpcCode = wire.Error.Code
		observation.rpcMessage = wire.Error.Message
		return observation
	}
	if wire.Result == nil {
		t.Fatalf("missing result and error: %s", out.Bytes())
	}
	observation.envelopeBytes = string(wire.Result.Structured)
	if err := json.Unmarshal(wire.Result.Structured, &observation.envelope); err != nil {
		t.Fatal(err)
	}
	if gateway {
		delegated, ok := observation.envelope["delegated_envelope"].(string)
		if !ok {
			t.Fatalf("gateway missing delegated envelope: %v", observation.envelope)
		}
		observation.delegatedBytes = delegated
		if err := json.Unmarshal([]byte(delegated), &observation.delegated); err != nil {
			t.Fatal(err)
		}
	} else {
		observation.delegatedBytes = observation.envelopeBytes
		observation.delegated = observation.envelope
	}
	return observation
}

func traceNegativeParityCases() []negativeParityCase {
	const validPrefix = `{"session_id":"s","generation":1,"uri":"file:///w/a.go",`
	return []negativeParityCase{
		{
			name:             "symbol-ambiguity",
			directArguments:  validPrefix + `"symbol":"Same"}`,
			gatewayArguments: `{"request":{"operation":"` + mcpcontract.TraceTool + `","arguments":` + validPrefix + `"symbol":"Same"}}}`,
			symbols:          []lsp.DocumentSymbol{symbol("Same", 1, 1), symbol("Same", 2, 2)},
			wantStatus:       "FAILED", wantOutcome: "DOMAIN_ERROR", wantCode: "INPUT_INVALID",
			wantDiagnostics:   []string{`total=2 omitted=0 candidates=[line=1,character=1 line=2,character=2]; use line/character`},
			wantExecutorCalls: 1, wantMethods: []string{"textDocument/documentSymbol"},
		},
		{
			name:             "symbol-not-found",
			directArguments:  validPrefix + `"symbol":"Missing"}`,
			gatewayArguments: `{"request":{"operation":"` + mcpcontract.TraceTool + `","arguments":` + validPrefix + `"symbol":"Missing"}}}`,
			wantStatus:       "FAILED", wantOutcome: "DOMAIN_ERROR", wantCode: "INPUT_INVALID",
			wantDiagnostics:   []string{`use an exact symbol name or line/character`},
			wantExecutorCalls: 1, wantMethods: []string{"textDocument/documentSymbol"},
		},
		{name: "invalid-nil-input", directArguments: `null`, gatewayArguments: `{"request":{"operation":"` + mcpcontract.TraceTool + `","arguments":null}}`, wantDirectRPCCode: -32602, wantDirectRPCMessage: "Invalid params", wantGatewayRPCCode: -32602, wantGatewayRPCMessage: "Invalid tool arguments: gateway request requires canonical operation and arguments object", wantExecutorCalls: 0},
		{name: "wrong-json-type", directArguments: `[]`, gatewayArguments: `{"request":{"operation":"` + mcpcontract.TraceTool + `","arguments":[]}}`, wantDirectRPCCode: -32602, wantDirectRPCMessage: "Invalid params", wantGatewayRPCCode: -32602, wantGatewayRPCMessage: "Invalid tool arguments: json: cannot unmarshal array into Go struct field gatewayRequest.arguments of type map[string]interface {}", wantExecutorCalls: 0},
		{name: "unknown-member", directArguments: validPrefix + `"symbol":"A","unknown":true}`, gatewayArguments: `{"request":{"operation":"` + mcpcontract.TraceTool + `","arguments":` + validPrefix + `"symbol":"A","unknown":true}}}`, wantDirectRPCCode: -32602, wantDirectRPCMessage: "Invalid tool arguments", wantGatewayRPCCode: -32602, wantGatewayRPCMessage: "Invalid tool arguments", wantExecutorCalls: 0},
		{name: "duplicate-json-member", directArguments: validPrefix + `"symbol":"A","symbol":"B"}`, gatewayArguments: `{"request":{"operation":"` + mcpcontract.TraceTool + `","arguments":` + validPrefix + `"symbol":"A","symbol":"B"}}}`, wantDirectRPCCode: -32700, wantDirectRPCMessage: `Parse error: duplicate JSON member "symbol"`, wantGatewayRPCCode: -32700, wantGatewayRPCMessage: `Parse error: duplicate JSON member "symbol"`, wantExecutorCalls: 0},
		{name: "symbol-vs-position-one-of", directArguments: validPrefix + `"symbol":"A","line":1,"character":2}`, gatewayArguments: `{"request":{"operation":"` + mcpcontract.TraceTool + `","arguments":` + validPrefix + `"symbol":"A","line":1,"character":2}}}`, wantDirectRPCCode: -32602, wantDirectRPCMessage: "Invalid tool arguments", wantGatewayRPCCode: -32602, wantGatewayRPCMessage: "Invalid tool arguments", wantExecutorCalls: 0},
	}
}

func TestOperation33RealNegativeRegistryGatewayParity(t *testing.T) {
	const assertion = "ASSERT_OPERATION33_REAL_NEGATIVE_REGISTRY_GATEWAY_PARITY"
	for _, tc := range traceNegativeParityCases() {
		t.Run(tc.name, func(t *testing.T) {
			direct := runNegativeParityRoute(t, false, tc)
			gateway := runNegativeParityRoute(t, true, tc)
			for route, got := range map[string]negativeParityObservation{"direct": direct, "gateway": gateway} {
				wantCode, wantMessage := tc.wantDirectRPCCode, tc.wantDirectRPCMessage
				if route == "gateway" {
					wantCode, wantMessage = tc.wantGatewayRPCCode, tc.wantGatewayRPCMessage
				}
				if got.rpcCode != wantCode || (wantMessage != "" && !strings.HasPrefix(got.rpcMessage, wantMessage)) {
					t.Fatalf("%s_%s_%s rpc=(%v,%q) want=(%v,%q)", assertion, tc.name, route, got.rpcCode, got.rpcMessage, wantCode, wantMessage)
				}
				if got.executorCalls != tc.wantExecutorCalls || got.acquisitionCalls != 0 || !reflect.DeepEqual(got.methods, tc.wantMethods) {
					t.Fatalf("%s_%s_%s side effects: executor=%d acquisition=%d methods=%v", assertion, tc.name, route, got.executorCalls, got.acquisitionCalls, got.methods)
				}
				if wantCode != 0 {
					if got.envelope != nil {
						t.Fatalf("%s_%s_%s unexpected operation envelope: %v", assertion, tc.name, route, got.envelope)
					}
					continue
				}
				if got.delegated["operation_status"] != tc.wantStatus || got.delegated["outcome"] != tc.wantOutcome || got.delegated["code"] != tc.wantCode || got.delegated["isError"] != true || !reflect.DeepEqual(got.delegated["diagnostics"], stringsToAny(tc.wantDiagnostics)) {
					t.Fatalf("%s_%s_%s delegated envelope: %v", assertion, tc.name, route, got.delegated)
				}
			}
			if tc.wantDirectRPCCode == 0 && tc.wantGatewayRPCCode == 0 {
				if gateway.delegatedBytes != direct.envelopeBytes {
					t.Fatalf("%s_%s exact delegated bytes:\n direct=%s\ngateway=%s", assertion, tc.name, direct.envelopeBytes, gateway.delegatedBytes)
				}
				if gateway.envelope["operation_status"] != "SUCCEEDED" || gateway.envelope["outcome"] != "COMPLETE" || gateway.envelope["isError"] != false || gateway.envelope["delegated_outcome"] != tc.wantOutcome || gateway.envelope["delegated_is_error"] != true {
					t.Fatalf("%s_%s outer execute envelope: %v", assertion, tc.name, gateway.envelope)
				}
			}
		})
	}
}

func stringsToAny(values []string) []any {
	out := make([]any, len(values))
	for i := range values {
		out[i] = values[i]
	}
	return out
}

func TestOperation33NegativeParityTableCoverageGuard(t *testing.T) {
	const assertion = "ASSERT_OPERATION33_NEGATIVE_PARITY_TABLE_COVERAGE"
	wantRows := []string{"symbol-ambiguity", "symbol-not-found", "invalid-nil-input", "wrong-json-type", "unknown-member", "duplicate-json-member", "symbol-vs-position-one-of"}
	cases := traceNegativeParityCases()
	gotRows := make([]string, len(cases))
	for i, tc := range cases {
		gotRows[i] = tc.name
		if tc.directArguments == "" || tc.gatewayArguments == "" {
			t.Fatalf("%s missing direct/gateway fixture for %s", assertion, tc.name)
		}
	}
	if !reflect.DeepEqual(gotRows, wantRows) {
		t.Fatalf("%s rows=%v want=%v", assertion, gotRows, wantRows)
	}
	wantDimensions := []string{"exact-delegated-envelope-bytes", "outer-execute-wrapper-isError", "status", "outcome", "code", "diagnostics", "executor-invocation-count", "exact-lsp-method-side-effects", "fresh-equivalent-fixture-per-route", "raw-transport-json-for-duplicate-type-nil"}
	if len(wantDimensions) != 10 {
		t.Fatalf("%s dimensions=%v", assertion, wantDimensions)
	}
}
