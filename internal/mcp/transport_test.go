package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"lsp-trace/internal/operation"
)

type fakeExecutor struct {
	calls    []operation.Name
	artifact []byte
}

func (f *fakeExecutor) Execute(_ context.Context, request operation.Request) (operation.Result, *operation.Failure) {
	f.calls = append(f.calls, request.Name)
	switch request.Name {
	case operation.Capabilities:
		return operation.Result{Value: NewRegistry(false).Capabilities()}, nil
	case operation.Verify:
		artifact := f.artifact
		if artifact == nil {
			artifact = []byte(`{"schema_version":"lsp-trace.graph.v3"}`)
		}
		return operation.Result{Artifact: artifact}, nil
	default:
		return operation.Result{}, &operation.Failure{Code: operation.FailureInvalidInput}
	}
}

func runMessages(t *testing.T, input string) []map[string]any {
	t.Helper()
	return runServerMessages(t, &Server{Registry: NewRegistry(false), Executor: &fakeExecutor{}}, input)
}

func runServerMessages(t *testing.T, server *Server, input string) []map[string]any {
	t.Helper()
	var out bytes.Buffer
	if err := server.Serve(strings.NewReader(input), &out); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	responses := make([]map[string]any, 0, len(lines))
	for _, line := range lines {
		var response map[string]any
		if err := json.Unmarshal([]byte(line), &response); err != nil {
			t.Fatalf("invalid response %q: %v", line, err)
		}
		responses = append(responses, response)
	}
	return responses
}

func TestCapabilitiesAndReservedDispatch(t *testing.T) {
	const capabilityAssertion = "capabilities returns immutable metadata for all twelve canonical tools"
	const precedenceAssertion = "reserved canonical and alias names validate before availability and emit canonical not-implemented envelopes"
	t.Log("ASSERTION: " + capabilityAssertion)
	t.Log("ASSERTION: " + precedenceAssertion)
	input := strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"lsp_trace_capabilities","arguments":{}}}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"lsp_trace_incoming","arguments":{}}}`,
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"lsp_trace_v1_incoming","arguments":{"unexpected":true}}}`,
	}, "\n") + "\n"
	got := runMessages(t, input)
	capCall := got[0]["result"].(map[string]any)
	capEnvelope := capCall["structuredContent"].(map[string]any)
	capResult := capEnvelope["result"].(map[string]any)
	tools := capResult["tools"].([]any)
	if len(tools) != 12 {
		t.Errorf("%s: got %d tools", capabilityAssertion, len(tools))
	}
	for _, raw := range tools {
		tool := raw.(map[string]any)
		if _, ok := tool["artifact_schema_ids"].([]any); !ok {
			t.Errorf("%s: %s artifact_schema_ids is not an array", capabilityAssertion, tool["name"])
		}
	}
	reservedCall := got[1]["result"].(map[string]any)
	reservedEnvelope := reservedCall["structuredContent"].(map[string]any)
	if reservedEnvelope["tool"] != "lsp_trace_v1_incoming" || reservedEnvelope["code"] != "TOOL_NOT_IMPLEMENTED" || reservedEnvelope["outcome"] != "DOMAIN_ERROR" || reservedEnvelope["operation_status"] != "FAILED" {
		t.Errorf("%s: %v", precedenceAssertion, reservedEnvelope)
	}
	if _, ok := got[2]["error"]; !ok {
		t.Errorf("%s: malformed reserved call reached availability gate: %v", precedenceAssertion, got[2])
	}
}

func TestOversizedArtifactRequiresSelector(t *testing.T) {
	const assertion = "selector-free artifacts larger than the advertised inline limit fail closed without content while equality remains inline"
	t.Log("ASSERTION: " + assertion)
	for _, tc := range []struct {
		name string
		size int
		over bool
	}{{"at-limit", inlineByteLimit, false}, {"over-limit", inlineByteLimit + 1, true}} {
		t.Run(tc.name, func(t *testing.T) {
			artifact := []byte(`{"schema_version":"lsp-trace.graph.v3"}`)
			artifact = append(artifact, bytes.Repeat([]byte(" "), tc.size-len(artifact))...)
			server := &Server{Registry: NewRegistry(false), Executor: &fakeExecutor{artifact: artifact}}
			responses := runServerMessages(t, server, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"lsp_trace_v1_verify","arguments":{"input":{}}}}`+"\n")
			call := responses[0]["result"].(map[string]any)
			env := call["structuredContent"].(map[string]any)
			if tc.over {
				if env["outcome"] != "DOMAIN_ERROR" || env["operation_status"] != "FAILED" || env["code"] != "OUTPUT_REQUIRES_SELECTOR" || env["isError"] != true {
					t.Errorf("ASSERT_OUTPUT_REQUIRES_SELECTOR: envelope=%v", env)
				}
				if _, ok := env["content"]; ok {
					t.Errorf("ASSERT_OVERSIZED_HAS_NO_CONTENT: envelope=%v", env)
				}
				return
			}
			if env["outcome"] != "COMPLETE" || env["operation_status"] != "SUCCEEDED" || env["content"] == nil || env["isError"] != false {
				t.Errorf("ASSERT_INLINE_LIMIT_INCLUSIVE: envelope=%v", env)
			}
		})
	}
}

func TestEmittedArtifactIdentityMustBelongToManifestTool(t *testing.T) {
	const assertion = "runtime rejects an artifact schema identity not declared for the canonical tool"
	t.Log("ASSERTION: " + assertion)
	tool, ok := NewRegistry(false).Resolve("lsp_trace_v1_filter")
	if !ok {
		t.Fatal("filter tool missing")
	}
	content := "{}\n"
	env := envelope{
		EnvelopeVersion: "1", EnvelopeSchemaID: artifactEnvelopeSchemaID, Tool: tool.Name, RequestID: "offline-1",
		Outcome: "COMPLETE", OperationStatus: "SUCCEEDED", Content: &content,
		ArtifactSchemaID: "https://jaresty.github.io/lsp-trace/schemas/lsp-trace.graph.v3.schema.json",
	}
	raw, err := json.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}
	if err := validateEmittedEnvelope(tool, env, raw); err == nil {
		t.Fatalf("%s: envelope=%s", assertion, raw)
	}
}

func TestTransportContract(t *testing.T) {
	const transportAssertion = "stdio JSON-RPC emits one response per request with no alternate transport"
	const listAssertion = "tools/list advertises only six enabled canonical names"
	const bindingAssertion = "tools/call binds the canonical operation envelope once in structuredContent with empty content and equal error state"
	const unknownAssertion = "unknown tool calls use native MCP unknown-tool errors without an operation envelope"
	for _, a := range []string{transportAssertion, listAssertion, bindingAssertion, unknownAssertion} {
		t.Log("ASSERTION: " + a)
	}

	input := strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}`,
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"lsp_trace_verify","arguments":{"input":{}}}}`,
		`{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"lsp_trace_v2_verify","arguments":{}}}`,
	}, "\n") + "\n"
	got := runMessages(t, input)
	if len(got) != 4 {
		t.Fatalf("%s: got %d responses", transportAssertion, len(got))
	}
	result, _ := got[1]["result"].(map[string]any)
	tools, _ := result["tools"].([]any)
	if len(tools) != 6 {
		t.Errorf("%s: got %d tools", listAssertion, len(tools))
	}
	call, _ := got[2]["result"].(map[string]any)
	content, _ := call["content"].([]any)
	if content == nil || len(content) != 0 {
		t.Errorf("%s: outer content is not empty", bindingAssertion)
	}
	envelope, _ := call["structuredContent"].(map[string]any)
	if envelope["tool"] != "lsp_trace_v1_verify" || call["isError"] != envelope["isError"] {
		t.Errorf("%s: call=%v envelope=%v", bindingAssertion, call, envelope)
	}
	if _, ok := got[3]["error"]; !ok || got[3]["result"] != nil {
		t.Errorf("%s: %v", unknownAssertion, got[3])
	}
}
