package main

import (
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"lsp-trace/internal/mcpcontract"
)

func callCompositionProcessFixture(t *testing.T, extraEnv ...string) (string, []string, string, string) {
	t.Helper()
	if runtime.GOOS != "darwin" {
		t.Skip("LOCAL_DARWIN_SUPERVISION_ONLY")
	}
	mcpBinary := buildMCPBinary(t)
	fakeBinary := buildBinary(t, "fake-lsp-call-composition", "./cmd/fake-lsp")
	workspace := t.TempDir()
	mainPath := filepath.Join(workspace, "main.go")
	if err := os.WriteFile(mainPath, []byte("package fixture\n\nfunc Target() {\n\tPeer()\n}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(workspace, "peer.go"), []byte("package fixture\n\nfunc Peer() {}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	trace := filepath.Join(t.TempDir(), "fake-lsp.trace")
	directionalOnce := filepath.Join(t.TempDir(), "directional-once")
	env := append([]string{"LSP_TRACE_FAKE_LSP_TRACE=" + trace, "LSP_TRACE_FAKE_LSP_DOCUMENT_SYMBOL=process-target", "LSP_TRACE_FAKE_LSP_DIRECTIONAL_ONCE=" + directionalOnce}, extraEnv...)
	process := func(alias string) map[string]any {
		return map[string]any{"alias": alias, "language_id": "go", "profile": map[string]any{"trust_domain": "call-composition-test-" + alias, "workspace": workspace, "profile": "fake-lsp-" + alias, "environment_reference": "hermetic"}, "execution": map[string]any{"path": fakeBinary, "directory": workspace, "environment": env}}
	}
	config := map[string]any{"version": 1, "processes": []any{process("composition"), process("composition-fresh")}}
	raw, _ := json.Marshal(config)
	configPath := filepath.Join(t.TempDir(), "bootstrap.json")
	if err := os.WriteFile(configPath, raw, 0600); err != nil {
		t.Fatal(err)
	}
	uri := (&url.URL{Scheme: "file", Path: mainPath}).String()
	return mcpBinary, []string{"--tool-profile", "experimental", "--bootstrap-config", configPath}, trace, uri
}
func callCompositionArgsForSession(uri, sessionID string) map[string]any {
	return map[string]any{"experimental_version": "1", "session_id": sessionID, "uri": uri, "language_id": "go", "symbol": "Target", "timeout_ms": 10000, "max_messages": 64, "max_bytes": 33554432}
}
func callCompositionArgs(uri string) map[string]any {
	return callCompositionArgsForSession(uri, "composition")
}
func compositionResult(t *testing.T, response map[string]any) map[string]any {
	t.Helper()
	result, ok := response["result"].(map[string]any)
	if !ok {
		t.Fatalf("response=%v", response)
	}
	env := result["structuredContent"].(map[string]any)
	artifact, ok := env["result"].(map[string]any)
	if !ok {
		t.Fatalf("envelope=%v", env)
	}
	return artifact
}
func traceCardinality(t *testing.T, path string) map[string]int {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	counts := map[string]int{}
	for _, event := range strings.Fields(string(raw)) {
		counts[event]++
	}
	return counts
}

func TestCallHierarchyCompositionRealManagedProcessSuccess(t *testing.T) {
	binary, args, trace, uri := callCompositionProcessFixture(t)
	requests := []map[string]any{{"jsonrpc": "2.0", "id": 1, "method": "tools/list", "params": map[string]any{}}, callRequest(2, mcpcontract.CallHierarchyCompositionTool, callCompositionArgs(uri))}
	responses := runMCPProcess(t, binary, args, requests)
	tools := responses[0]["result"].(map[string]any)["tools"].([]any)
	found := 0
	for _, raw := range tools {
		if raw.(map[string]any)["name"] == mcpcontract.CallHierarchyCompositionTool {
			found++
		}
	}
	if found != 1 {
		t.Fatalf("tool count=%d", found)
	}
	a := compositionResult(t, responses[1])
	incoming := a["incoming"].([]any)
	outgoing := a["outgoing"].([]any)
	if a["status"] != "SUCCEEDED" || len(incoming) != 0 || len(outgoing) != 1 || a["incoming_null"] != false || a["outgoing_null"] != false || a["completeness"] != "UNKNOWN" || a["feature_identity"] != "UNRESOLVED" || a["authority"] != float64(0) || a["session_poisoned"] != false || a["retry_scope"] != "NONE" {
		t.Fatalf("artifact=%v", a)
	}
	if outgoing[0].(map[string]any)["to"].(map[string]any)["name"] != "Peer" {
		t.Fatalf("outgoing=%v", outgoing)
	}
	if strings.Contains(string(mustJSON(t, a)), "generation") || strings.Contains(string(mustJSON(t, a)), "raw") || strings.Contains(string(mustJSON(t, a)), "prepared") {
		t.Fatalf("private state escaped: %v", a)
	}
	counts := traceCardinality(t, trace)
	for method, want := range map[string]int{"textDocument/prepareCallHierarchy": 1, "callHierarchy/incomingCalls": 1, "callHierarchy/outgoingCalls": 1} {
		if counts[method] != want {
			t.Fatalf("trace=%v method=%s", counts, method)
		}
	}
}

func TestCallHierarchyCompositionRealManagedProcessGatewaySuccess(t *testing.T) {
	binary, args, trace, uri := callCompositionProcessFixture(t)
	gateway := map[string]any{"request": map[string]any{"operation": mcpcontract.CallHierarchyCompositionTool, "arguments": callCompositionArgs(uri)}}
	response := runMCPProcess(t, binary, args, []map[string]any{callRequest(1, "lsp_trace_v1_execute", gateway)})[0]
	env := response["result"].(map[string]any)["structuredContent"].(map[string]any)
	delegated := env["delegated_envelope"].(string)
	var direct map[string]any
	if err := json.Unmarshal([]byte(delegated), &direct); err != nil {
		t.Fatal(err)
	}
	if direct["result"].(map[string]any)["status"] != "SUCCEEDED" {
		t.Fatalf("delegated=%v", direct)
	}
	counts := traceCardinality(t, trace)
	for method, want := range map[string]int{"textDocument/prepareCallHierarchy": 1, "callHierarchy/incomingCalls": 1, "callHierarchy/outgoingCalls": 1} {
		if counts[method] != want {
			t.Fatalf("trace=%v method=%s", counts, method)
		}
	}
}

func TestCallHierarchyCompositionRealDirectionalExactLimitSuccess(t *testing.T) {
	binary, args, trace, uri := callCompositionProcessFixture(t, "LSP_TRACE_FAKE_LSP_DIRECTIONAL_LIMIT=incoming-retained-exact")
	a := compositionResult(t, runMCPProcess(t, binary, args, []map[string]any{callRequest(1, mcpcontract.CallHierarchyCompositionTool, callCompositionArgs(uri))})[0])
	if a["status"] != "SUCCEEDED" || a["session_poisoned"] != false || a["retry_scope"] != "NONE" || len(a["outgoing"].([]any)) != 1 {
		t.Fatalf("artifact=%v", a)
	}
	incomingReceipt := a["receipts"].(map[string]any)["incoming"].(map[string]any)
	if got := uint64(incomingReceipt["request_length"].(float64) + incomingReceipt["response_length"].(float64)); got != 32<<20 {
		t.Fatalf("mechanical directional accounting got=%d want=%d receipt=%v", got, 32<<20, incomingReceipt)
	}
	counts := traceCardinality(t, trace)
	if counts["textDocument/prepareCallHierarchy"] != 1 || counts["callHierarchy/incomingCalls"] != 1 || counts["callHierarchy/outgoingCalls"] != 1 {
		t.Fatalf("trace=%v", counts)
	}
}

func TestCallHierarchyCompositionRealMalformedPostEffectPoisonsAndRequiresFreshSession(t *testing.T) {
	binary, args, trace, uri := callCompositionProcessFixture(t, "LSP_TRACE_FAKE_LSP_DIRECTIONAL_MALFORMED=incoming-object")
	requests := []map[string]any{
		callRequest(1, mcpcontract.CallHierarchyCompositionTool, callCompositionArgs(uri)),
		callRequest(2, mcpcontract.CallHierarchyCompositionTool, callCompositionArgs(uri)),
		callRequest(3, mcpcontract.CallHierarchyCompositionTool, callCompositionArgsForSession(uri, "composition-fresh")),
	}
	responses := runMCPProcess(t, binary, args, requests)
	malformed := compositionResult(t, responses[0])
	if malformed["status"] != "REFUSED" || malformed["phase"] != "POST_EFFECT_FAILURE" || malformed["session_poisoned"] != true || malformed["retry_scope"] != "NEW_SESSION_REQUIRED" {
		t.Fatalf("malformed=%v trace=%v", malformed, traceCardinality(t, trace))
	}
	retry := compositionResult(t, responses[1])
	if retry["code"] != "SESSION_POISONED" || retry["session_poisoned"] != true || retry["retry_scope"] != "NEW_SESSION_REQUIRED" {
		t.Fatalf("same-session retry=%v", retry)
	}
	fresh := compositionResult(t, responses[2])
	if fresh["status"] != "SUCCEEDED" || fresh["session_poisoned"] != false || fresh["retry_scope"] != "NONE" {
		t.Fatalf("fresh-session retry=%v", fresh)
	}
}

func TestCallHierarchyCompositionRealPostWriteTimeoutPoisonsAndRequiresFreshSession(t *testing.T) {
	for _, tc := range []struct {
		direction    string
		wantOutgoing int
	}{
		{direction: "incoming", wantOutgoing: 1},
		{direction: "outgoing", wantOutgoing: 2},
	} {
		t.Run(tc.direction, func(t *testing.T) {
			binary, args, trace, uri := callCompositionProcessFixture(t, "LSP_TRACE_FAKE_LSP_DIRECTIONAL_HANG_ONCE="+tc.direction)
			timeoutArgs := callCompositionArgs(uri)
			timeoutArgs["timeout_ms"] = 100
			requests := []map[string]any{
				callRequest(1, mcpcontract.CallHierarchyCompositionTool, timeoutArgs),
				callRequest(2, mcpcontract.CallHierarchyCompositionTool, callCompositionArgs(uri)),
				callRequest(3, mcpcontract.CallHierarchyCompositionTool, callCompositionArgsForSession(uri, "composition-fresh")),
			}
			responses := runMCPProcess(t, binary, args, requests)
			timedOut := compositionResult(t, responses[0])
			if timedOut["status"] != "REFUSED" || timedOut["phase"] != "POST_EFFECT_FAILURE" || timedOut["code"] != "REQUEST_TIMEOUT" || timedOut["session_poisoned"] != true || timedOut["retry_scope"] != "NEW_SESSION_REQUIRED" {
				t.Fatalf("%s timeout=%v trace=%v", tc.direction, timedOut, traceCardinality(t, trace))
			}
			retry := compositionResult(t, responses[1])
			if retry["status"] != "REFUSED" || retry["phase"] != "LIFECYCLE" || retry["code"] != "SESSION_POISONED" || retry["session_poisoned"] != true || retry["retry_scope"] != "NEW_SESSION_REQUIRED" {
				t.Fatalf("%s same-session retry=%v", tc.direction, retry)
			}
			fresh := compositionResult(t, responses[2])
			if fresh["status"] != "SUCCEEDED" || fresh["session_poisoned"] != false || fresh["retry_scope"] != "NONE" || len(fresh["outgoing"].([]any)) != 1 {
				t.Fatalf("%s fresh-session retry=%v", tc.direction, fresh)
			}
			counts := traceCardinality(t, trace)
			if counts["textDocument/prepareCallHierarchy"] != 2 || counts["callHierarchy/incomingCalls"] != 2 || counts["callHierarchy/outgoingCalls"] != tc.wantOutgoing || counts["$/cancelRequest"] != 1 {
				t.Fatalf("%s trace=%v", tc.direction, counts)
			}
		})
	}
}

func TestCallHierarchyCompositionRealDirectionalPlusOnePoisonsAndRequiresFreshSession(t *testing.T) {
	binary, args, trace, uri := callCompositionProcessFixture(t, "LSP_TRACE_FAKE_LSP_DIRECTIONAL_LIMIT=incoming-retained-plus-one")
	requests := []map[string]any{
		callRequest(1, mcpcontract.CallHierarchyCompositionTool, callCompositionArgs(uri)),
		callRequest(2, mcpcontract.CallHierarchyCompositionTool, callCompositionArgs(uri)),
		callRequest(3, mcpcontract.CallHierarchyCompositionTool, callCompositionArgsForSession(uri, "composition-fresh")),
	}
	responses := runMCPProcess(t, binary, args, requests)
	a := compositionResult(t, responses[0])
	if a["status"] != "REFUSED" || a["phase"] != "POST_EFFECT_RESOURCE_REFUSAL" || a["code"] != "RESOURCE_EXHAUSTED" || a["session_poisoned"] != true || a["retry_scope"] != "NEW_SESSION_REQUIRED" {
		t.Fatalf("artifact=%v trace=%v", a, traceCardinality(t, trace))
	}
	raw := string(mustJSON(t, a))
	if strings.Contains(raw, "33554433") || strings.Contains(raw, "fixture\"") || strings.Contains(raw, "padding") {
		t.Fatalf("raw response escaped: %s", raw)
	}
	retry := compositionResult(t, responses[1])
	if retry["status"] != "REFUSED" || retry["phase"] != "LIFECYCLE" || retry["code"] != "SESSION_POISONED" || retry["session_poisoned"] != true || retry["retry_scope"] != "NEW_SESSION_REQUIRED" {
		t.Fatalf("same-session retry=%v", retry)
	}
	fresh := compositionResult(t, responses[2])
	if fresh["status"] != "SUCCEEDED" || fresh["session_poisoned"] != false || fresh["retry_scope"] != "NONE" || len(fresh["outgoing"].([]any)) != 1 {
		t.Fatalf("fresh-session retry=%v", fresh)
	}
	counts := traceCardinality(t, trace)
	if counts["textDocument/prepareCallHierarchy"] != 2 || counts["callHierarchy/incomingCalls"] != 2 || counts["callHierarchy/outgoingCalls"] != 1 {
		t.Fatalf("trace=%v", counts)
	}
}
