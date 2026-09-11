package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"lsp-trace/internal/mcpcontract"
	"lsp-trace/internal/operation"
	"lsp-trace/internal/publication"
)

type fakeExecutor struct {
	calls         []operation.Name
	artifact      []byte
	logicalDigest string
}

func (f *fakeExecutor) Execute(ctx context.Context, request operation.Request) (operation.Result, *operation.Failure) {
	f.calls = append(f.calls, request.Name)
	switch request.Name {
	case operation.Capabilities:
		return operation.Result{Value: NewRegistry(false).Capabilities()}, nil
	case operation.SchemaGet:
		return operation.SchemaGetHandler(ctx, request)
	case operation.Verify:
		artifact := f.artifact
		if artifact == nil {
			artifact = []byte(`{"schema_version":"lsp-trace.graph.v3"}`)
		}
		return operation.Result{Artifact: artifact, LogicalDigest: f.logicalDigest}, nil
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

type traversalFailureExecutor struct{}

func (traversalFailureExecutor) Execute(_ context.Context, _ operation.Request) (operation.Result, *operation.Failure) {
	return operation.Result{}, &operation.Failure{Code: "DOCUMENT_SYMBOL_ABSENT", Diagnostics: []string{`document symbol "Start" not found`}}
}

type targetFailureExecutor struct {
	failure *operation.Failure
}

func (e targetFailureExecutor) Execute(_ context.Context, _ operation.Request) (operation.Result, *operation.Failure) {
	return operation.Result{}, e.failure
}

func TestResolveTargetFailuresNormalizeToExclusivePublicEnvelopes(t *testing.T) {
	const assertion = "ASSERT_RESOLVE_TARGET_FAILURE_NORMALIZATION"
	t.Log("ASSERTION: " + assertion)
	tests := []struct {
		privateCode string
		publicCode  string
	}{
		{"DOCUMENT_SYMBOL_ABSENT", "INPUT_INVALID"},
		{"DOCUMENT_SYMBOL_AMBIGUOUS", "INPUT_INVALID"},
		{"DOCUMENT_SYMBOL_UNSUPPORTED", "UNSUPPORTED_CALL_HIERARCHY"},
		{"DOCUMENT_SYMBOL_FAILED", "OUTPUT_VALIDATION_FAILED"},
		{"DOCUMENT_SYMBOL_MALFORMED_RANGE", "OUTPUT_VALIDATION_FAILED"},
		{"DOCUMENT_SYMBOL_PREPARE_FAILED", "OUTPUT_VALIDATION_FAILED"},
		{"DOCUMENT_SYMBOL_UNPREPARABLE", "UNSUPPORTED_CALL_HIERARCHY"},
		{"CANCELLED", "REQUEST_CANCELLED"},
		{"REQUEST_TIMEOUT", "REQUEST_TIMEOUT"},
		{"RELATION_CUSTODY_FAILED", "RELATION_CUSTODY_FAILED"},
	}
	const arguments = `{"session_id":"project","uri":"file:///workspace/main.go","symbol":"Start"}`
	for _, test := range tests {
		t.Run(test.privateCode, func(t *testing.T) {
			diagnostics := []string{"resolve target: " + test.privateCode}
			server := &Server{
				Registry: NewRegistry(false),
				Executors: map[ExecutorFamily]Executor{
					IncomingExecutorFamily: targetFailureExecutor{failure: &operation.Failure{Code: test.privateCode, Diagnostics: diagnostics}},
				},
			}
			response := runServerMessages(t, server, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"lsp_trace_v1_incoming","arguments":`+arguments+`}}`+"\n")[0]
			envelope := decodeEnvelopeForAssertion(t, assertion+"_"+test.privateCode, response)
			if envelope["code"] != test.publicCode {
				t.Fatalf("%s_%s: code=%v want=%s", assertion, test.privateCode, envelope["code"], test.publicCode)
			}
			gotDiagnostics, ok := envelope["diagnostics"].([]any)
			if !ok || len(gotDiagnostics) != 1 || gotDiagnostics[0] != diagnostics[0] {
				t.Fatalf("%s_%s: diagnostics=%v want=%v", assertion, test.privateCode, envelope["diagnostics"], diagnostics)
			}
			raw, err := json.Marshal(envelope)
			if err != nil {
				t.Fatalf("%s_%s: marshal: %v", assertion, test.privateCode, err)
			}
			if err := mcpcontract.ValidateEnvelopeExclusive(raw); err != nil {
				t.Fatalf("%s_%s: exclusive validation: %v envelope=%s", assertion, test.privateCode, err, raw)
			}
		})
	}
}

func TestCompactResponsePublishesFullArtifactWithUsabilityMetadata(t *testing.T) {
	const (
		compat      = "ASSERT_MCP_FULL_BACKWARD_COMPATIBLE"
		custody     = "ASSERT_MCP_COMPACT_IMMUTABLE_FULL_RECEIPT"
		summary     = "ASSERT_MCP_COMPACT_CONCISE_SUMMARY"
		accounting  = "ASSERT_MCP_COMPACT_PUBLIC_ACCOUNTING"
		progress    = "ASSERT_MCP_COMPACT_OBSERVED_PROGRESS"
		cardinality = "ASSERT_MCP_EXECUTION_ADDS_EXACTLY_ONE_AUTHORITY"
	)
	for _, assertion := range []string{compat, custody, summary, accounting, progress, cardinality} {
		t.Log("ASSERTION: " + assertion)
	}
	artifact := []byte(`{"schema_version":"lsp-trace.graph.v3","nodes":[],"edges":[],"terminals":[],"frontier":[],"diagnostics":[{"phase":"prepare","method":"m1","category":"UNRESOLVED_CALL","node_id":"private-1","message":"secret one"},{"phase":"prepare","method":"m1","category":"UNRESOLVED_CALL","node_id":"private-2","message":"secret two"},{"phase":"incoming","method":"m2","message":"secret three"},{"phase":"outgoing","method":"m3","message":"secret four"},{"phase":"shutdown","method":"m4","message":"secret five"}],"summary":{"traversal_complete":true}}`)
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
	compactSummary := compact["summary"].(map[string]any)
	diagnosticSummary, ok := compactSummary["diagnostic_summary"].(map[string]any)
	if !ok || diagnosticSummary["total_count"] != float64(5) || diagnosticSummary["distinct_count"] != float64(4) || diagnosticSummary["retained_count"] != float64(3) || diagnosticSummary["repeated_count"] != float64(1) || diagnosticSummary["omitted_count"] != float64(1) {
		t.Fatalf("%s: %v", summary, compact)
	}
	examples, _ := diagnosticSummary["examples"].([]any)
	fullOutput, _ := compactSummary["full_output"].(map[string]any)
	if len(examples) != 3 || fullOutput["delivery"] != "publication_receipt" || fullOutput["output_selector"] != "full.json" {
		t.Fatalf("%s: examples=%v full_output=%v", summary, examples, fullOutput)
	}
	rawSummary, _ := json.Marshal(compactSummary)
	if bytes.Contains(rawSummary, []byte("secret")) || bytes.Contains(rawSummary, []byte("private-")) {
		t.Fatalf("%s leaked private diagnostic fields: %s", summary, rawSummary)
	}
	if compact["duration_ms"] == nil || compact["request_accounting"] == nil {
		t.Fatalf("%s: %v", accounting, compact)
	}
	if compact["progress"] != "completed" {
		t.Fatalf("%s: %v", progress, compact)
	}
	if got := len(server.Registry.Advertised()); got != 28 {
		t.Fatalf("%s: got %d", cardinality, got)
	}
}

func TestTraversalCompactResponsePublishesFullArtifact(t *testing.T) {
	const (
		compat      = "ASSERT_TRAVERSAL_DEFAULT_FULL_BACKWARD_COMPATIBLE"
		custody     = "ASSERT_TRAVERSAL_COMPACT_PUBLISHES_EXACT_FULL_BYTES"
		summary     = "ASSERT_TRAVERSAL_COMPACT_RETURNS_RECEIPT_AND_SUMMARY"
		cardinality = "ASSERT_TRAVERSAL_COMPACT_PRESERVES_THIRTEEN_TOOLS"
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
			if got := len(server.Registry.Advertised()); got != 28 {
				t.Fatalf("%s: got %d", cardinality, got)
			}
		})
	}
}

func TestIncompleteTraversalArtifactsReportPartialStatus(t *testing.T) {
	const assertion = "ASSERT_INCOMPLETE_TRAVERSAL_REPORTS_PARTIAL_STATUS"
	artifact := []byte(`{"schema_version":"lsp-trace.graph.v3","nodes":[],"edges":[],"terminals":[],"frontier":[],"diagnostics":[],"summary":{"traversal_complete":false}}`)
	for _, tc := range []struct {
		name      string
		arguments string
		family    ExecutorFamily
	}{
		{"lsp_trace_v1_incoming", `"session_id":"session","generation":1,"uri":"file:///workspace/main.go","line":0,"character":0`, IncomingExecutorFamily},
		{"lsp_trace_v1_slice", `"session_id":"session","generation":1,"start_mode":"at","uri":"file:///workspace/main.go","line":0,"character":0`, SliceExecutorFamily},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root, err := publication.OpenRoot(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			defer root.Close()
			server := &Server{Registry: NewRegistryWithPublication(false, true), Executors: map[ExecutorFamily]Executor{tc.family: traversalArtifactExecutor{artifact: artifact}}, PublicationRoot: root}
			responses := runServerMessages(t, server, strings.Join([]string{
				`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"` + tc.name + `","arguments":{` + tc.arguments + `}}}`,
				`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"` + tc.name + `","arguments":{` + tc.arguments + `,"detail":"compact","output_selector":"partial.json"}}}`,
			}, "\n")+"\n")
			for i, response := range responses {
				env := decodeEnvelopeForAssertion(t, assertion, response)
				if env["outcome"] != "PARTIAL" || env["operation_status"] != "PARTIAL" || env["isError"] != false {
					t.Fatalf("%s response[%d]=%v", assertion, i, env)
				}
			}
		})
	}
}

func TestRealTraversalEnvelopesValidateAcrossSuccessFailureAndPublication(t *testing.T) {
	const (
		fullAssertion        = "ASSERT_REAL_TRAVERSAL_FULL_ENVELOPE_EXCLUSIVE"
		failureAssertion     = "ASSERT_REAL_TRAVERSAL_FAILURE_DIAGNOSTIC_ENVELOPE_EXCLUSIVE"
		compactAssertion     = "ASSERT_REAL_TRAVERSAL_COMPACT_IMMUTABLE_PUBLICATION"
		cardinalityAssertion = "ASSERT_REAL_TRAVERSAL_CURRENT28_PRESERVES_HISTORICAL_TOOLS"
	)
	for _, assertion := range []string{fullAssertion, failureAssertion, compactAssertion, cardinalityAssertion} {
		t.Log("ASSERTION: " + assertion)
	}
	artifact := []byte(`{"schema_version":"lsp-trace.graph.v3","invocation":{"target":{"uri":"file:///workspace/main.go","line":7,"column":3},"limits":{"max_depth":4,"max_nodes":100,"timeout_ms":5000},"request_timeout_ms":1000},"capabilities":{"call_hierarchy_provider":true},"nodes":[{"id":"node-1","name":"Start","kind":12,"uri":"file:///workspace/main.go","range":{"start":{"line":7,"character":0},"end":{"line":9,"character":1}},"selection_range":{"start":{"line":7,"character":3},"end":{"line":7,"character":8}}}],"edges":[],"terminals":[],"frontier":[],"diagnostics":[],"summary":{"traversal_complete":true,"node_count":1,"edge_count":0}}` + "\n")
	rootPath := t.TempDir()
	root, err := publication.OpenRoot(rootPath)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	registry := NewRegistryWithPublication(false, true)
	arguments := `{"session_id":"project","uri":"file:///workspace/main.go","symbol":"Start"}`
	success := &Server{Registry: registry, Executors: map[ExecutorFamily]Executor{IncomingExecutorFamily: traversalArtifactExecutor{artifact: artifact}}, PublicationRoot: root}
	responses := runServerMessages(t, success, strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"lsp_trace_v1_incoming","arguments":` + arguments + `}}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"lsp_trace_v1_incoming","arguments":{"session_id":"project","uri":"file:///workspace/main.go","symbol":"Start","detail":"compact","output_selector":"incoming.json"}}}`,
	}, "\n")+"\n")
	for i, assertion := range []string{fullAssertion, compactAssertion} {
		call := decodeEnvelopeForAssertion(t, assertion, responses[i])
		raw, _ := json.Marshal(call)
		if err := mcpcontract.ValidateEnvelopeExclusive(raw); err != nil {
			t.Fatalf("%s: %v envelope=%s", assertion, err, raw)
		}
	}
	published, err := os.ReadFile(filepath.Join(rootPath, "incoming.json"))
	if err != nil || !bytes.Equal(published, artifact) {
		t.Fatalf("%s: bytes=%q err=%v", compactAssertion, published, err)
	}
	failure := &Server{Registry: registry, Executors: map[ExecutorFamily]Executor{IncomingExecutorFamily: traversalFailureExecutor{}}, PublicationRoot: root}
	failureResponse := runServerMessages(t, failure, `{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"lsp_trace_v1_incoming","arguments":`+arguments+`}}`+"\n")[0]
	failureEnvelope := decodeEnvelopeForAssertion(t, failureAssertion, failureResponse)
	if failureEnvelope["code"] != "INPUT_INVALID" || !strings.Contains(fmt.Sprint(failureEnvelope["diagnostics"]), "Start") {
		t.Fatalf("%s: envelope=%v", failureAssertion, failureEnvelope)
	}
	raw, _ := json.Marshal(failureEnvelope)
	if err := mcpcontract.ValidateEnvelopeExclusive(raw); err != nil {
		t.Fatalf("%s: %v envelope=%s", failureAssertion, err, raw)
	}
	if got := len(registry.Advertised()); got != 28 {
		t.Fatalf("%s: got %d", cardinalityAssertion, got)
	}
}

func decodeEnvelopeForAssertion(t *testing.T, assertion string, response map[string]any) map[string]any {
	t.Helper()
	if response["error"] != nil {
		t.Fatalf("%s: response=%v", assertion, response)
	}
	result, ok := response["result"].(map[string]any)
	if !ok {
		t.Fatalf("%s: result=%v", assertion, response)
	}
	env, ok := result["structuredContent"].(map[string]any)
	if !ok {
		t.Fatalf("%s: envelope=%v", assertion, result)
	}
	return env
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
	const capabilityAssertion = "capabilities returns immutable metadata for all 28 current canonical tools"
	t.Log("ASSERTION: " + capabilityAssertion)
	input := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"lsp_trace_capabilities","arguments":{}}}` + "\n"
	got := runMessages(t, input)
	capCall := got[0]["result"].(map[string]any)
	capEnvelope := capCall["structuredContent"].(map[string]any)
	capResult := capEnvelope["result"].(map[string]any)
	tools := capResult["tools"].([]any)
	if len(tools) != 28 {
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
				diagnostics, _ := env["diagnostics"].([]any)
				if len(diagnostics) != 1 {
					t.Fatalf("ASSERT_OVERSIZED_ACTIONABLE_DIAGNOSTIC: envelope=%v", env)
				}
				text, _ := diagnostics[0].(string)
				for _, phrase := range []string{"1048577 bytes", "inline limit is 1048576 bytes", "caller-supplied publication destination", "not the artifact or its schema identity", "--publication-root", "request.arguments.output_selector", "retry reacquires"} {
					if !strings.Contains(text, phrase) {
						t.Errorf("ASSERT_OVERSIZED_ACTIONABLE_DIAGNOSTIC: missing %q in %q", phrase, text)
					}
				}
				return
			}
			if env["outcome"] != "COMPLETE" || env["operation_status"] != "SUCCEEDED" || env["content"] == nil || env["isError"] != false {
				t.Errorf("ASSERT_INLINE_LIMIT_INCLUSIVE: envelope=%v", env)
			}
		})
	}
}

func TestCustodyLogicalDigestParityAcrossInlineAndPublication(t *testing.T) {
	const assertion = "ASSERT_MCP_CUSTODY_LOGICAL_DIGEST_PARITY"
	const digest = "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	artifact := []byte(`{"schema_version":"lsp-trace.graph.v3"}`)
	for _, tc := range []struct {
		name string
		args map[string]any
		root bool
	}{{"inline", map[string]any{"input": map[string]any{}}, false}, {"publication", map[string]any{"input": map[string]any{}, "output_selector": "custody.json"}, true}} {
		t.Run(tc.name, func(t *testing.T) {
			registry := NewRegistryWithPublication(false, tc.root)
			server := &Server{Registry: registry, Executor: &fakeExecutor{artifact: artifact, logicalDigest: digest}}
			if tc.root {
				root, err := publication.OpenRoot(t.TempDir())
				if err != nil {
					t.Fatal(err)
				}
				defer root.Close()
				server.PublicationRoot = root
			}
			arguments, _ := json.Marshal(tc.args)
			responses := runServerMessages(t, server, fmt.Sprintf(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"lsp_trace_v1_verify","arguments":%s}}`, arguments)+"\n")
			env := responses[0]["result"].(map[string]any)["structuredContent"].(map[string]any)
			if env["logical_digest"] != digest || env["outcome"] != "COMPLETE" {
				t.Fatalf("%s: envelope=%v", assertion, env)
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

func TestToolsCallAcceptsRequestMetadata(t *testing.T) {
	const assertion = "tools/call accepts standard MCP request metadata without changing tool arguments"
	t.Log("ASSERTION: " + assertion)

	responses := runMessages(t, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"lsp_trace_v1_capabilities","arguments":{"operation":"lsp_trace_v3_slice"},"_meta":{"progressToken":3}}}`+"\n")
	if len(responses) != 1 {
		t.Fatalf("%s: got %d responses", assertion, len(responses))
	}
	if rpcError := responses[0]["error"]; rpcError != nil {
		t.Fatalf("%s: error=%v", assertion, rpcError)
	}
	result, ok := responses[0]["result"].(map[string]any)
	if !ok || result["isError"] != false {
		t.Fatalf("%s: result=%v", assertion, responses[0]["result"])
	}
}

func TestToolsCallNormalizesOnlyOmittedArguments(t *testing.T) {
	const omittedAssertion = "tools/call normalizes omitted arguments to an empty object for tools whose schema accepts it"
	const strictAssertion = "tools/call still rejects explicit null arguments and missing required fields"
	for _, assertion := range []string{omittedAssertion, strictAssertion} {
		t.Log("ASSERTION: " + assertion)
	}

	input := strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"lsp_trace_v1_capabilities","_meta":{"progressToken":"capabilities"}}}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"lsp_session_v1_list","_meta":{"progressToken":"sessions"}}}`,
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"lsp_session_v1_list","arguments":null}}`,
		`{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"lsp_trace_v1_slice"}}`,
	}, "\n") + "\n"
	responses := runMessages(t, input)
	if len(responses) != 4 {
		t.Fatalf("%s: got %d responses", omittedAssertion, len(responses))
	}
	for i := 0; i < 2; i++ {
		if rpcError := responses[i]["error"]; rpcError != nil {
			t.Errorf("%s: response %d error=%v", omittedAssertion, i+1, rpcError)
		}
	}
	for i := 2; i < 4; i++ {
		rpcError, ok := responses[i]["error"].(map[string]any)
		if !ok || rpcError["code"] != float64(-32602) {
			t.Errorf("%s: response %d=%v", strictAssertion, i+1, responses[i])
		}
	}
}

func TestCompactSchemaGetIsInlineAndStructured(t *testing.T) {
	const assertion = "compact schema retrieval is inline and exposes a parsed schema object"
	t.Log("ASSERTION: " + assertion)
	responses := runMessages(t, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"lsp_trace_v1_schema_get","arguments":{"schema":{"family":"filter","version":"v1"},"detail":"compact"}}}`+"\n")
	if len(responses) != 1 || responses[0]["error"] != nil {
		t.Fatalf("%s: responses=%v", assertion, responses)
	}
	call, ok := responses[0]["result"].(map[string]any)
	if !ok {
		t.Fatalf("%s: call=%v", assertion, responses[0]["result"])
	}
	env, ok := call["structuredContent"].(map[string]any)
	if !ok || env["outcome"] != "COMPLETE" || env["operation_status"] != "SUCCEEDED" || env["code"] != nil {
		t.Fatalf("%s: envelope=%v", assertion, env)
	}
	result, ok := env["result"].(map[string]any)
	if !ok {
		t.Fatalf("%s: result=%v", assertion, env["result"])
	}
	if _, ok := result["schema"].(map[string]any); !ok {
		t.Fatalf("%s: schema=%T", assertion, result["schema"])
	}
	if result["schema_id"] != "https://jaresty.github.io/lsp-trace/schemas/lsp-trace.filter.v1.schema.json" {
		t.Fatalf("%s: schema_id=%v", assertion, result["schema_id"])
	}
	if env["content"] != nil {
		t.Fatalf("%s: compact result unexpectedly encoded schema as content", assertion)
	}
}

func TestTransportContract(t *testing.T) {
	const transportAssertion = "stdio JSON-RPC emits one response per request with no alternate transport"
	const listAssertion = "tools/list advertises the 28 current canonical names"
	const bindingAssertion = "tools/call binds one canonical operation envelope in structuredContent and one equal text content item"
	const unknownAssertion = "unknown tool calls use native MCP unknown-tool errors without an operation envelope"
	for _, a := range []string{transportAssertion, listAssertion, bindingAssertion, unknownAssertion} {
		t.Log("ASSERTION: " + a)
	}

	input := strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}`,
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"lsp_trace_verify","arguments":{"input":{}}}}`,
		`{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"lsp_trace_v99_verify","arguments":{}}}`,
	}, "\n") + "\n"
	got := runMessages(t, input)
	if len(got) != 4 {
		t.Fatalf("%s: got %d responses", transportAssertion, len(got))
	}
	result, _ := got[1]["result"].(map[string]any)
	tools, _ := result["tools"].([]any)
	if len(tools) != 28 {
		t.Errorf("%s: got %d tools", listAssertion, len(tools))
	}
	call, _ := got[2]["result"].(map[string]any)
	content, _ := call["content"].([]any)
	envelope, _ := call["structuredContent"].(map[string]any)
	var textEnvelope map[string]any
	if len(content) != 1 {
		t.Errorf("%s: content=%v", bindingAssertion, content)
	} else if item, ok := content[0].(map[string]any); !ok || item["type"] != "text" || json.Unmarshal([]byte(item["text"].(string)), &textEnvelope) != nil || !reflect.DeepEqual(textEnvelope, envelope) {
		t.Errorf("%s: content=%v envelope=%v", bindingAssertion, content, envelope)
	}
	if envelope["tool"] != "lsp_trace_v1_verify" || call["isError"] != envelope["isError"] {
		t.Errorf("%s: call=%v envelope=%v", bindingAssertion, call, envelope)
	}
	if _, ok := got[3]["error"]; !ok || got[3]["result"] != nil {
		t.Errorf("%s: %v", unknownAssertion, got[3])
	}
}
