package mcp

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"
)

func TestExecuteGatewayCanonicalDelegationAndEvidence(t *testing.T) {
	const assertion = "ASSERT_MCP_EXECUTE_GATEWAY_CANONICAL_BYTE_EXACT_DELEGATION"
	t.Log("ASSERTION: " + assertion)
	server := &Server{Registry: NewRegistry(false), Executor: &fakeExecutor{}}
	response := runServerMessages(t, server, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"lsp_trace_v1_execute","arguments":{"request":{"tool":"lsp_trace_v1_capabilities","arguments":{}}}}}`+"\n")[0]
	env := decodeEnvelopeForAssertion(t, assertion, response)
	delegated, ok := env["delegated_envelope"].(string)
	if !ok || delegated == "" {
		t.Fatalf("%s: missing delegated envelope: %v", assertion, env)
	}
	sum := sha256.Sum256([]byte(delegated))
	if env["requested_tool"] != "lsp_trace_v1_capabilities" || env["delegated_digest"] != "sha256:"+hex.EncodeToString(sum[:]) || env["delegated_outcome"] != "COMPLETE" || env["delegated_is_error"] != false {
		t.Fatalf("%s: envelope=%v", assertion, env)
	}
	var delegatedEnv map[string]any
	if err := json.Unmarshal([]byte(delegated), &delegatedEnv); err != nil || delegatedEnv["tool"] != "lsp_trace_v1_capabilities" {
		t.Fatalf("%s: delegated=%q err=%v", assertion, delegated, err)
	}
}

func TestExecuteGatewayRejectsAliasUnknownRecursiveAndMalformed(t *testing.T) {
	const assertion = "ASSERT_MCP_EXECUTE_GATEWAY_REJECTS_NONCANONICAL_TARGETS"
	server := &Server{Registry: NewRegistry(false), Executor: &fakeExecutor{}}
	inputs := []string{
		`{"tool":"lsp_trace_capabilities","arguments":{}}`,
		`{"tool":"lsp_trace_v9_missing","arguments":{}}`,
		`{"tool":"lsp_trace_v1_execute","arguments":{"request":{}}}`,
		`{"tool":"lsp_trace_v1_capabilities"}`,
	}
	for i, input := range inputs {
		response := runServerMessages(t, server, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"lsp_trace_v1_execute","arguments":{"request":`+input+`}}}`+"\n")[0]
		if response["error"] == nil || !strings.Contains(response["error"].(map[string]any)["message"].(string), "Invalid tool arguments") {
			t.Fatalf("%s[%d]: %v", assertion, i, response)
		}
	}
}

func TestExecuteGatewayPreservesLegacyCustodyPath(t *testing.T) {
	const assertion = "ASSERT_MCP_EXECUTE_GATEWAY_LEGACY_CUSTODY_UNCHANGED"
	artifact := []byte(`{"$id":"https://jaresty.github.io/lsp-trace/schemas/lsp-trace.execution.v1.schema.json","execution_version":"1","result":{}}`)
	executor := &executionTransportExecutor{artifact: artifact, digest: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}
	server := &Server{Registry: NewRegistry(false), Executor: executor}
	response := runServerMessages(t, server, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"lsp_trace_v1_execute","arguments":{"request":{}}}}`+"\n")[0]
	env := decodeEnvelopeForAssertion(t, assertion, response)
	if len(executor.calls) != 1 || env["content"] != string(artifact) || env["delegated_envelope"] != nil {
		t.Fatalf("%s: calls=%v envelope=%v", assertion, executor.calls, env)
	}
}
