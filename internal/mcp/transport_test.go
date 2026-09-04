package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"lsp-trace/internal/operation"
	"lsp-trace/internal/publication"
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

type orderedExecutor struct {
	events *[]string
}

func (e orderedExecutor) Execute(_ context.Context, _ operation.Request) (operation.Result, *operation.Failure) {
	*e.events = append(*e.events, "execute")
	return operation.Result{Artifact: []byte(`{"schema_version":"lsp-trace.graph.v3"}`)}, nil
}

type traversalArtifactExecutor struct {
	artifact []byte
}

func (e traversalArtifactExecutor) Execute(_ context.Context, _ operation.Request) (operation.Result, *operation.Failure) {
	return operation.Result{Artifact: e.artifact}, nil
}

func TestCompactResponsePublishesFullArtifactWithUsabilityMetadata(t *testing.T) {
	const (
		compat      = "ASSERT_MCP_FULL_BACKWARD_COMPATIBLE"
		custody     = "ASSERT_MCP_COMPACT_IMMUTABLE_FULL_RECEIPT"
		summary     = "ASSERT_MCP_COMPACT_CONCISE_SUMMARY"
		accounting  = "ASSERT_MCP_COMPACT_PUBLIC_ACCOUNTING"
		progress    = "ASSERT_MCP_COMPACT_OBSERVED_PROGRESS"
		cardinality = "ASSERT_MCP_EXACT_TWELVE_AUTHORITY_UNCHANGED"
	)
	for _, assertion := range []string{compat, custody, summary, accounting, progress, cardinality} {
		t.Log("ASSERTION: " + assertion)
	}
	artifact := []byte(`{"schema_version":"lsp-trace.graph.v3","nodes":[],"edges":[],"terminals":[],"frontier":[],"diagnostics":[],"summary":{"traversal_complete":true}}`)
	root, err := publication.OpenRoot(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	server := &Server{Registry: NewRegistryWithPublication(false, true), Executor: &fakeExecutor{artifact: artifact}, PublicationRoot: root}
	responses := runServerMessages(t, server, strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"lsp_trace_v1_verify","arguments":{"input":{},"detail":"full"}}}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"lsp_trace_v1_verify","arguments":{"input":{},"detail":"compact","output_selector":"full.json"}}}`,
	}, "\n")+"\n")
	if responses[0]["error"] != nil {
		t.Fatalf("%s: %v", compat, responses[0])
	}
	full := responses[0]["result"].(map[string]any)["structuredContent"].(map[string]any)
	if full["envelope_schema_id"] != artifactEnvelopeSchemaID || full["content"] != string(artifact) {
		t.Fatalf("%s: %v", compat, full)
	}
	if responses[1]["error"] != nil {
		t.Fatalf("%s: %v", custody, responses[1])
	}
	compact := responses[1]["result"].(map[string]any)["structuredContent"].(map[string]any)
	if compact["publication_receipt"] == nil || compact["content"] != nil {
		t.Fatalf("%s: %v", custody, compact)
	}
	if compact["summary"] == nil {
		t.Fatalf("%s: %v", summary, compact)
	}
	if compact["duration_ms"] == nil || compact["request_accounting"] == nil {
		t.Fatalf("%s: %v", accounting, compact)
	}
	if compact["progress"] != "completed" {
		t.Fatalf("%s: %v", progress, compact)
	}
	if got := len(server.Registry.Advertised()); got != 12 {
		t.Fatalf("%s: got %d", cardinality, got)
	}
}

func TestTraversalCompactResponsePublishesFullArtifact(t *testing.T) {
	const (
		compat      = "ASSERT_TRAVERSAL_DEFAULT_FULL_BACKWARD_COMPATIBLE"
		custody     = "ASSERT_TRAVERSAL_COMPACT_PUBLISHES_EXACT_FULL_BYTES"
		summary     = "ASSERT_TRAVERSAL_COMPACT_RETURNS_RECEIPT_AND_SUMMARY"
		cardinality = "ASSERT_TRAVERSAL_COMPACT_PRESERVES_TWELVE_TOOLS"
	)
	for _, assertion := range []string{compat, custody, summary, cardinality} {
		t.Log("ASSERTION: " + assertion)
	}
	artifact := []byte(`{"schema_version":"lsp-trace.graph.v3","nodes":[],"edges":[],"terminals":[],"frontier":[],"diagnostics":[],"summary":{"traversal_complete":true}}`)
	for _, tc := range []struct {
		name      string
		arguments string
		family    ExecutorFamily
	}{
		{"lsp_trace_v1_incoming", `"session_id":"session","generation":1,"uri":"file:///workspace/main.go","line":0,"character":0`, IncomingExecutorFamily},
		{"lsp_trace_v1_slice", `"session_id":"session","generation":1,"start_mode":"at","uri":"file:///workspace/main.go","line":0,"character":0`, SliceExecutorFamily},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rootPath := t.TempDir()
			root, err := publication.OpenRoot(rootPath)
			if err != nil {
				t.Fatal(err)
			}
			defer root.Close()
			server := &Server{
				Registry:        NewRegistryWithPublication(false, true),
				Executors:       map[ExecutorFamily]Executor{tc.family: traversalArtifactExecutor{artifact: artifact}},
				PublicationRoot: root,
			}
			responses := runServerMessages(t, server, strings.Join([]string{
				`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"` + tc.name + `","arguments":{` + tc.arguments + `}}}`,
				`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"` + tc.name + `","arguments":{` + tc.arguments + `,"detail":"compact","output_selector":"artifact.json"}}}`,
			}, "\n")+"\n")
			if responses[0]["error"] != nil {
				t.Fatalf("%s: %v", compat, responses[0])
			}
			full := responses[0]["result"].(map[string]any)["structuredContent"].(map[string]any)
			if full["envelope_schema_id"] != artifactEnvelopeSchemaID || full["content"] != string(artifact) {
				t.Fatalf("%s: %v", compat, full)
			}
			if responses[1]["error"] != nil {
				t.Fatalf("%s: %v", custody, responses[1])
			}
			compact := responses[1]["result"].(map[string]any)["structuredContent"].(map[string]any)
			if compact["publication_receipt"] == nil || compact["summary"] == nil || compact["content"] != nil {
				t.Fatalf("%s: %v", summary, compact)
			}
			published, err := os.ReadFile(filepath.Join(rootPath, "artifact.json"))
			if err != nil || !bytes.Equal(published, artifact) {
				t.Fatalf("%s: bytes=%q err=%v", custody, published, err)
			}
			if got := len(server.Registry.Advertised()); got != 12 {
				t.Fatalf("%s: got %d", cardinality, got)
			}
		})
	}
}

func TestStructuralThenSemanticThenExecutorFamily(t *testing.T) {
	const structuralAssertion = "structural validation rejects malformed input before semantic validation"
	const routeAssertion = "valid input runs optional semantic validation before the selected executor family"
	t.Log("ASSERTION: " + structuralAssertion)
	t.Log("ASSERTION: " + routeAssertion)

	events := []string{}
	registry := NewRegistryWithRouting(false, Routing{
		SemanticValidator: func(tool Tool) SemanticValidator {
			if tool.Name != "lsp_trace_v1_verify" {
				return nil
			}
			return func(_ context.Context, _ Tool, arguments map[string]any) error {
				events = append(events, "semantic")
				input, _ := arguments["input"].(map[string]any)
				if blocked, _ := input["blocked"].(bool); blocked {
					return errors.New("blocked semantically")
				}
				return nil
			}
		},
		ExecutorFamily: func(tool Tool) ExecutorFamily {
			if tool.Name == "lsp_trace_v1_verify" {
				return ExecutorFamily("analysis")
			}
			return OfflineExecutorFamily
		},
	})
	server := &Server{
		Registry: registry,
		Executor: orderedExecutor{events: &[]string{}},
		Executors: map[ExecutorFamily]Executor{
			ExecutorFamily("analysis"): orderedExecutor{events: &events},
		},
	}
	responses := runServerMessages(t, server, strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"lsp_trace_v1_verify","arguments":{"unexpected":true}}}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"lsp_trace_v1_verify","arguments":{"input":{"blocked":true}}}}`,
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"lsp_trace_v1_verify","arguments":{"input":{}}}}`,
	}, "\n")+"\n")
	if len(responses) != 3 || responses[0]["error"] == nil {
		t.Fatalf("%s: responses=%v", structuralAssertion, responses)
	}
	if semanticError, _ := responses[1]["error"].(map[string]any); semanticError["message"] != "Invalid tool arguments: blocked semantically" {
		t.Fatalf("%s: semantic error=%v", routeAssertion, responses[1])
	}
	if got := strings.Join(events, ","); got != "semantic,semantic,execute" {
		t.Fatalf("%s: events=%q", routeAssertion, got)
	}
}

type cancellationExecutor struct {
	started chan struct{}
	release chan struct{}
}

func (e cancellationExecutor) Execute(ctx context.Context, _ operation.Request) (operation.Result, *operation.Failure) {
	close(e.started)
	select {
	case <-ctx.Done():
		return operation.Result{}, &operation.Failure{Code: operation.FailureInternal, Err: ctx.Err()}
	case <-e.release:
		return operation.Result{}, &operation.Failure{Code: operation.FailureInternal}
	}
}

func TestRequestContextCancelsInFlightExecutor(t *testing.T) {
	const assertion = "request cancellation reaches the selected in-flight executor"
	t.Log("ASSERTION: " + assertion)
	started := make(chan struct{})
	release := make(chan struct{})
	server := &Server{Registry: NewRegistry(false), Executor: cancellationExecutor{started: started, release: release}}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan response, 1)
	go func() {
		done <- server.handleContext(ctx, request{
			JSONRPC: "2.0", ID: 1, Method: "tools/call",
			Params: json.RawMessage(`{"name":"lsp_trace_v1_verify","arguments":{"input":{}}}`),
		})
	}()
	<-started
	cancel()
	select {
	case <-done:
	case <-time.After(100 * time.Millisecond):
		close(release)
		t.Fatalf("%s: context cancellation did not reach executor", assertion)
	}
}

func TestServerShutdownCancelsInFlightExecutor(t *testing.T) {
	const assertion = "server shutdown coordinates cancellation of the selected in-flight executor"
	t.Log("ASSERTION: " + assertion)
	started := make(chan struct{})
	release := make(chan struct{})
	server := &Server{Registry: NewRegistry(false), Executor: cancellationExecutor{started: started, release: release}}
	reader, writer := io.Pipe()
	var output bytes.Buffer
	done := make(chan error, 1)
	go func() { done <- server.ServeContext(context.Background(), reader, &output) }()
	if _, err := io.WriteString(writer, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"lsp_trace_v1_verify","arguments":{"input":{}}}}`+"\n"); err != nil {
		t.Fatal(err)
	}
	<-started
	server.Shutdown()
	_ = writer.Close()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("%s: %v", assertion, err)
		}
	case <-time.After(time.Second):
		close(release)
		t.Fatalf("%s: serve did not stop", assertion)
	}
}

func TestCapabilitiesDispatch(t *testing.T) {
	const capabilityAssertion = "capabilities returns immutable metadata for all twelve canonical tools"
	t.Log("ASSERTION: " + capabilityAssertion)
	input := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"lsp_trace_capabilities","arguments":{}}}` + "\n"
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
	const listAssertion = "tools/list advertises the twelve always-local canonical names"
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
	if len(tools) != 12 {
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
