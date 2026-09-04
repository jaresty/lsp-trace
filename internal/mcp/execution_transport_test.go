package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"lsp-trace/internal/operation"
	"lsp-trace/internal/publication"
)

const (
	executionToolName  = "lsp_trace_v1_execute"
	executionToolAlias = "lsp_trace_execute"
)

type executionTransportExecutor struct {
	calls    []operation.Request
	artifact []byte
	digest   string
}

func (e *executionTransportExecutor) Execute(_ context.Context, request operation.Request) (operation.Result, *operation.Failure) {
	e.calls = append(e.calls, request)
	return operation.Result{Artifact: append([]byte(nil), e.artifact...), LogicalDigest: e.digest}, nil
}

func TestExecutionToolRegistryDiscoveryAndClosedSchema(t *testing.T) {
	const assertion = "ASSERT_MCP_EXECUTION_REGISTRY_DISCOVERY_AND_CLOSED_SCHEMA"
	t.Log("ASSERTION: " + assertion)
	registry := NewRegistryWithPublication(false, true)
	canonical, ok := registry.Resolve(executionToolName)
	if !ok {
		t.Fatalf("%s: canonical execution tool missing", assertion)
	}
	alias, aliasOK := registry.Resolve(executionToolAlias)
	properties, _ := canonical.InputSchema["properties"].(map[string]any)
	if !aliasOK || alias.Name != executionToolName || canonical.Availability != Enabled || canonical.Description == "" || properties["request"] == nil || properties["output_selector"] == nil || canonical.InputSchema["additionalProperties"] != false {
		t.Fatalf("%s: canonical=%+v alias=%+v aliasOK=%v", assertion, canonical, alias, aliasOK)
	}
	advertised := false
	for _, tool := range registry.Advertised() {
		advertised = advertised || tool.Name == executionToolName
	}
	if !advertised {
		t.Fatalf("%s: canonical execution tool not advertised", assertion)
	}
}

func TestExecutionTransportCanonicalDispatchAndLogicalDigestParity(t *testing.T) {
	const assertion = "ASSERT_MCP_EXECUTION_CANONICAL_DISPATCH_AND_LOGICAL_DIGEST_PARITY"
	t.Log("ASSERTION: " + assertion)
	artifact := []byte(`{"$id":"https://jaresty.github.io/lsp-trace/schemas/lsp-trace.execution.v1.schema.json","execution_version":"1","result":{}}`)
	executor := &executionTransportExecutor{artifact: artifact, digest: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}
	server := &Server{Registry: NewRegistry(false), Executor: executor}
	response := runServerMessages(t, server, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"lsp_trace_execute","arguments":{"request":{}}}}`+"\n")[0]
	env := decodeEnvelopeForAssertion(t, assertion, response)
	if len(executor.calls) != 1 || executor.calls[0].Name != operation.Name("execute") {
		t.Fatalf("%s: calls=%+v", assertion, executor.calls)
	}
	if env["tool"] != executionToolName || env["logical_digest"] != executor.digest || env["content"] != string(artifact) {
		t.Fatalf("%s: envelope=%v", assertion, env)
	}
	var input map[string]any
	if err := json.Unmarshal(executor.calls[0].Input, &input); err != nil || input["request"] == nil || input["output_selector"] != nil {
		t.Fatalf("%s: executor input=%s err=%v", assertion, executor.calls[0].Input, err)
	}
}

func TestExecutionTransportOversizedResultIsPublishedExactlyOrFailsWithoutContent(t *testing.T) {
	const (
		publicationAssertion = "ASSERT_MCP_EXECUTION_OVERSIZED_IMMUTABLE_EXACT_PUBLICATION"
		failureAssertion     = "ASSERT_MCP_EXECUTION_OVERSIZED_EXPLICIT_NON_TRUNCATING_FAILURE"
	)
	t.Log("ASSERTION: " + publicationAssertion)
	t.Log("ASSERTION: " + failureAssertion)
	artifact := []byte(`{"$id":"https://jaresty.github.io/lsp-trace/schemas/lsp-trace.execution.v1.schema.json","execution_version":"1","result":"` + strings.Repeat("x", inlineByteLimit) + `"}`)
	executor := &executionTransportExecutor{artifact: artifact, digest: "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"}
	rootPath := t.TempDir()
	root, err := publication.OpenRoot(rootPath)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	server := &Server{Registry: NewRegistryWithPublication(false, true), Executor: executor, PublicationRoot: root}
	responses := runServerMessages(t, server, strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"lsp_trace_v1_execute","arguments":{"request":{}}}}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"lsp_trace_v1_execute","arguments":{"request":{},"output_selector":"execution.json"}}}`,
	}, "\n")+"\n")
	failure := decodeEnvelopeForAssertion(t, failureAssertion, responses[0])
	if failure["code"] != "OUTPUT_REQUIRES_SELECTOR" || failure["content"] != nil || failure["publication_receipt"] != nil {
		t.Fatalf("%s: envelope=%v", failureAssertion, failure)
	}
	published := decodeEnvelopeForAssertion(t, publicationAssertion, responses[1])
	if published["logical_digest"] != executor.digest || published["publication_receipt"] == nil || published["content"] != nil {
		t.Fatalf("%s: envelope=%v", publicationAssertion, published)
	}
	got, err := os.ReadFile(filepath.Join(rootPath, "execution.json"))
	if err != nil || !bytes.Equal(got, artifact) {
		t.Fatalf("%s: exact bytes=%v err=%v", publicationAssertion, bytes.Equal(got, artifact), err)
	}
}
