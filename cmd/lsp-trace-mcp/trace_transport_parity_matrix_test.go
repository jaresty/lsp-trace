package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"lsp-trace/internal/graph"
	"lsp-trace/internal/graphprovenance"
	"lsp-trace/internal/lspwire"
	"lsp-trace/internal/manageddiagnostic"
	"lsp-trace/internal/managedprocess"
	"lsp-trace/internal/mcp"
	"lsp-trace/internal/mcpcontract"
	"lsp-trace/internal/operation"
	"lsp-trace/internal/publication"
	"lsp-trace/internal/runtimeprofile"
	"lsp-trace/sessionruntime"
)

type parityMatrixProcess struct {
	in            *io.PipeReader
	stdin         *io.PipeWriter
	out           *io.PipeWriter
	stdout        *io.PipeReader
	uri           string
	mu            sync.Mutex
	methods       []string
	teardownCalls int
	closeCalls    int
}

func newParityMatrixProcess(uri string) *parityMatrixProcess {
	in, stdin := io.Pipe()
	stdout, out := io.Pipe()
	p := &parityMatrixProcess{in: in, stdin: stdin, out: out, stdout: stdout, uri: uri}
	go p.serve()
	return p
}

func (p *parityMatrixProcess) serve() {
	r := lspwire.NewReader(p.in, lspwire.DefaultLimits())
	w := lspwire.NewWriter(p.out, lspwire.DefaultLimits())
	for {
		msg, err := r.Read()
		if err != nil {
			return
		}
		p.mu.Lock()
		p.methods = append(p.methods, msg.Method)
		p.mu.Unlock()
		result := json.RawMessage(`[]`)
		switch msg.Method {
		case "initialize":
			result = json.RawMessage(`{"capabilities":{"callHierarchyProvider":true,"documentSymbolProvider":true,"positionEncoding":"utf-16"}}`)
		case "initialized", "textDocument/didOpen", "textDocument/didChange":
			continue
		case "textDocument/prepareCallHierarchy":
			result = p.items()
		case "callHierarchy/outgoingCalls", "callHierarchy/incomingCalls":
			result = json.RawMessage(`[]`)
		case "shutdown":
			result = json.RawMessage(`null`)
		case "exit":
			return
		}
		if len(msg.ID) != 0 {
			if err := w.Write(lspwire.Message{JSONRPC: lspwire.Version, ID: msg.ID, Result: result}); err != nil {
				return
			}
		}
	}
}

func (p *parityMatrixProcess) items() json.RawMessage {
	item := map[string]any{"name": "F", "kind": 12, "uri": p.uri,
		"range":          map[string]any{"start": map[string]int{"line": 1, "character": 0}, "end": map[string]int{"line": 1, "character": 6}},
		"selectionRange": map[string]any{"start": map[string]int{"line": 1, "character": 5}, "end": map[string]int{"line": 1, "character": 6}}}
	raw, _ := json.Marshal([]any{item})
	return raw
}
func (p *parityMatrixProcess) Stdin() io.WriteCloser { return p.stdin }
func (p *parityMatrixProcess) Stdout() io.ReadCloser { return p.stdout }
func (p *parityMatrixProcess) Teardown(context.Context) managedprocess.TeardownObservation {
	p.mu.Lock()
	p.teardownCalls++
	p.mu.Unlock()
	_ = p.stdin.Close()
	_ = p.in.Close()
	_ = p.out.Close()
	return managedprocess.TeardownObservation{Death: managedprocess.DeathObservation{Reap: managedprocess.ReapObservation{Kind: managedprocess.ReapComplete}}}
}
func (p *parityMatrixProcess) Close() managedprocess.ResourceObservation {
	p.mu.Lock()
	p.closeCalls++
	p.mu.Unlock()
	_ = p.stdout.Close()
	return managedprocess.ResourceObservation{Kind: managedprocess.ResourcesClosed}
}
func (p *parityMatrixProcess) observations() ([]string, int, int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]string(nil), p.methods...), p.teardownCalls, p.closeCalls
}

type parityMatrixStarter struct {
	uri     string
	process *parityMatrixProcess
	starts  int
}

func (s *parityMatrixStarter) Start(context.Context, managedprocess.Spec) (sessionruntime.Child, managedprocess.StartObservation) {
	s.starts++
	s.process = newParityMatrixProcess(s.uri)
	return s.process, managedprocess.StartObservation{Kind: managedprocess.StartStarted}
}

type parityMatrixAcquisition struct {
	artifact []byte
	failure  *operation.Failure
	calls    int
}

func (a *parityMatrixAcquisition) ExecuteExplicitTrace(context.Context, operation.Request, []byte, string, sessionruntime.DocumentResult) (operation.Result, *operation.Failure) {
	a.calls++
	if a.failure != nil {
		return operation.Result{}, a.failure
	}
	return operation.Result{Artifact: append([]byte(nil), a.artifact...)}, nil
}

type parityMatrixObservation struct {
	delegated        string
	status           any
	outcome          any
	diagnostics      any
	isError          any
	executorCalls    int
	acquisitionCalls int
	methods          []string
	starts           int
	teardownCalls    int
	closeCalls       int
	publishedCount   int
	publishedBytes   []byte
}

type parityMatrixCase struct {
	name               string
	artifact           func(*testing.T) []byte
	selector           string
	occupySelector     bool
	readOnlyRoot       bool
	wantStatus         string
	wantOutcome        string
	wantCode           string
	wantDiagnostic     string
	wantError          bool
	wantPublishedCount int
	wantPublished      bool
}

func parityMatrixV5(t *testing.T, incomplete, truncated bool) []byte {
	t.Helper()
	uri := "file:///fixture/code.go"
	p := graph.Position{}
	node := graph.NewNode(graph.Item{Name: "F", Kind: 12, URI: uri, Range: graph.Range{Start: p, End: graph.Position{Character: 1}}, SelectionRange: graph.Range{Start: p, End: graph.Position{Character: 1}}})
	result := graph.Result{SchemaVersion: graph.SchemaVersionV5, Nodes: []graph.Node{node}, Targets: []string{node.ID}, Invocation: graph.Invocation{Target: graph.Target{URI: uri, Line: 0, Column: 0}, Server: graph.ServerInvocation{Command: "parity-matrix"}, Provenance: graph.InvocationProvenance{InvocationID: "parity-matrix", SourceRevision: graph.Unknown, ServerVersion: "fixture@1"}}, Summary: graph.Summary{Truncated: truncated}}
	if incomplete {
		result.Frontier = []graph.Boundary{{NodeID: node.ID, Reason: graph.RequestTimeout}}
	}
	native, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := graphprovenance.CaptureV5(native, "fixture-session", 1, manageddiagnostic.QueryResult{Status: manageddiagnostic.QueryUnavailable, Records: []manageddiagnostic.Record{}}, &graphprovenance.EvidenceV2{SchemaVersion: graphprovenance.VersionV2, Policy: graphprovenance.PolicyV2, WorkspaceURI: "file:///fixture", AnalyzedVersion: graphprovenance.Unverified, DependencyCompleteness: "UNKNOWN_INCOMPLETE", Supplies: []graphprovenance.SupplyReceiptV2{}, Captures: []graphprovenance.Receipt{}, Bindings: []graphprovenance.BindingV2{}})
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func parityMatrixEmptyV5(t *testing.T) []byte {
	t.Helper()
	result := graph.Result{
		SchemaVersion: graph.SchemaVersionV5,
		Nodes:         []graph.Node{},
		Targets:       []string{},
		Invocation: graph.Invocation{
			Target:     graph.Target{URI: "file:///fixture/code.go", Line: 0, Column: 0},
			Server:     graph.ServerInvocation{Command: "parity-matrix-empty"},
			Provenance: graph.InvocationProvenance{InvocationID: "parity-matrix-empty", SourceRevision: graph.Unknown, ServerVersion: "fixture@1"},
		},
		Summary: graph.Summary{},
	}
	native, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := graphprovenance.CaptureV5(native, "fixture-session-empty", 1, manageddiagnostic.QueryResult{Status: manageddiagnostic.QueryUnavailable, Records: []manageddiagnostic.Record{}}, &graphprovenance.EvidenceV2{SchemaVersion: graphprovenance.VersionV2, Policy: graphprovenance.PolicyV2, WorkspaceURI: "file:///fixture", AnalyzedVersion: graphprovenance.Unverified, DependencyCompleteness: "UNKNOWN_INCOMPLETE", Supplies: []graphprovenance.SupplyReceiptV2{}, Captures: []graphprovenance.Receipt{}, Bindings: []graphprovenance.BindingV2{}})
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func parityMatrixCases() []parityMatrixCase {
	valid := func(t *testing.T) []byte { return parityMatrixV5(t, false, false) }
	return []parityMatrixCase{
		{name: "successful-empty", artifact: parityMatrixEmptyV5, wantStatus: "SUCCEEDED", wantOutcome: "COMPLETE"},
		{name: "output-selector-publication-success", artifact: valid, selector: "trace.json", wantStatus: "SUCCEEDED", wantOutcome: "COMPLETE", wantPublishedCount: 4, wantPublished: true},
		{name: "selector-unsafe", artifact: valid, selector: "../escape.json", wantStatus: "FAILED", wantOutcome: "DOMAIN_ERROR", wantCode: "OUTPUT_SELECTOR_UNSAFE", wantDiagnostic: "output selector is unsafe", wantError: true},
		{name: "selector-conflict", artifact: valid, selector: "occupied.json", occupySelector: true, wantStatus: "FAILED", wantOutcome: "PUBLICATION_ERROR", wantCode: "PUBLICATION_FAILED", wantError: true},
		{name: "publication-failure", artifact: valid, selector: "read-only.json", readOnlyRoot: true, wantStatus: "FAILED", wantOutcome: "PUBLICATION_ERROR", wantCode: "PUBLICATION_FAILED", wantError: true},
		{name: "malformed-v5", artifact: func(*testing.T) []byte { return []byte(`{"schema_version":"lsp-trace.graph-provenance.v5"}`) }, wantStatus: "FAILED", wantOutcome: "DOMAIN_ERROR", wantCode: "OUTPUT_VALIDATION_FAILED", wantDiagnostic: "trace artifact is not a valid Graph Provenance V5 envelope with a valid native graph", wantError: true},
		{name: "legal-incomplete", artifact: func(t *testing.T) []byte { return parityMatrixV5(t, true, false) }, wantStatus: "PARTIAL", wantOutcome: "PARTIAL"},
		{name: "legal-truncated", artifact: func(t *testing.T) []byte { return parityMatrixV5(t, false, true) }, wantStatus: "PARTIAL", wantOutcome: "PARTIAL"},
	}
}

func runParityMatrixRoute(t *testing.T, tc parityMatrixCase, gateway bool) parityMatrixObservation {
	t.Helper()
	workspace := t.TempDir()
	path := filepath.Join(workspace, "code.go")
	if err := os.WriteFile(path, []byte("package fixture\nfunc F() {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	uri := (&url.URL{Scheme: "file", Path: filepath.ToSlash(path)}).String()
	profile, err := runtimeprofile.Validate(runtimeprofile.Selector{TrustDomain: "trace-parity-matrix", Workspace: workspace, Profile: "fixture", EnvironmentReference: "test"})
	if err != nil {
		t.Fatal(err)
	}
	starter := &parityMatrixStarter{uri: uri}
	manager, err := sessionruntime.New(sessionruntime.Config{Limits: sessionruntime.Limits{MaxSessions: 1, MaxRequests: 8, MaxChildren: 2, MaxCancels: 8, MaxTombstones: 8, MaxObservations: 128, MaxOperations: 8}, Starter: starter})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = manager.Shutdown(context.Background()) })
	started := manager.Start(context.Background(), sessionruntime.StartRequest{Profile: runtimeprofile.Resolve(profile), BootstrapAlias: "trace-parity-matrix", LanguageID: "go"})
	pending := manager.BeginReadiness(context.Background(), started.SessionID, started.Generation, time.Now().Add(time.Second))
	ready, ok := manager.WaitReadiness(context.Background(), pending.ID)
	if !ok || ready.State != sessionruntime.ReadinessReady {
		t.Fatalf("ASSERT_TRACE_PARITY_MATRIX_READY: %+v", ready)
	}
	acquisition := &parityMatrixAcquisition{artifact: tc.artifact(t)}
	executor := &countingTraceExecutor{delegate: &traceExecutor{runtime: manager, acquisition: acquisition}}
	rootPath := t.TempDir()
	root, err := publication.OpenRoot(rootPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = root.Close() })
	if tc.occupySelector {
		if err := os.WriteFile(filepath.Join(rootPath, tc.selector), []byte("caller-owned"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	before := parityMatrixPublishedFiles(t, rootPath)
	if tc.readOnlyRoot {
		if err := os.Chmod(rootPath, 0o500); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(rootPath, 0o700) })
	}
	server := &mcp.Server{Registry: mcp.NewRegistryWithProfile(false, mcp.ToolProfileFull), Executors: map[mcp.ExecutorFamily]mcp.Executor{mcp.TraceExecutorFamily: executor}, PublicationRoot: root}
	args := map[string]any{"session_id": started.SessionID, "generation": started.Generation, "uri": uri, "line": float64(1), "character": float64(5), "down_depth": float64(0), "up_depth": float64(0)}
	if tc.selector != "" {
		args["output_selector"] = tc.selector
	}
	name := mcpcontract.TraceTool
	if gateway {
		name = "lsp_trace_v1_execute"
		args = map[string]any{"request": map[string]any{"operation": mcpcontract.TraceTool, "arguments": args}}
	}
	params, _ := json.Marshal(map[string]any{"name": name, "arguments": args})
	request := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":` + string(params) + `}` + "\n"
	var out bytes.Buffer
	if err := server.Serve(strings.NewReader(request), &out); err != nil {
		t.Fatal(err)
	}
	var wire struct {
		Error  any `json:"error"`
		Result struct {
			Structured json.RawMessage `json:"structuredContent"`
		} `json:"result"`
	}
	if err := json.Unmarshal(bytes.TrimSpace(out.Bytes()), &wire); err != nil {
		t.Fatal(err)
	}
	if wire.Error != nil {
		t.Fatalf("ASSERT_TRACE_PARITY_MATRIX_RPC_SUCCESS: %v", wire.Error)
	}
	var outer map[string]any
	if err := json.Unmarshal(wire.Result.Structured, &outer); err != nil {
		t.Fatal(err)
	}
	delegated := string(wire.Result.Structured)
	if gateway {
		delegated, _ = outer["delegated_envelope"].(string)
	}
	var env map[string]any
	if err := json.Unmarshal([]byte(delegated), &env); err != nil {
		t.Fatalf("ASSERT_TRACE_PARITY_MATRIX_DELEGATED_JSON: %v: %q", err, delegated)
	}
	after := parityMatrixPublishedFiles(t, rootPath)
	methods, teardownCalls, closeCalls := starter.process.observations()
	published := []byte(nil)
	if tc.wantPublished {
		published, err = os.ReadFile(filepath.Join(rootPath, tc.selector))
		if err != nil {
			t.Fatal(err)
		}
	}
	return parityMatrixObservation{delegated: delegated, status: env["operation_status"], outcome: env["outcome"], diagnostics: env["diagnostics"], isError: env["isError"], executorCalls: executor.calls, acquisitionCalls: acquisition.calls, methods: methods, starts: starter.starts, teardownCalls: teardownCalls, closeCalls: closeCalls, publishedCount: len(after) - len(before), publishedBytes: published}
}

func parityMatrixPublishedFiles(t *testing.T, root string) []string {
	t.Helper()
	var files []string
	if err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() {
			files = append(files, path)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	sort.Strings(files)
	return files
}

func TestOperation33RealRegistryGatewayParityMatrix(t *testing.T) {
	cases := parityMatrixCases()
	wantNames := []string{"legal-incomplete", "legal-truncated", "malformed-v5", "output-selector-publication-success", "publication-failure", "selector-conflict", "selector-unsafe", "successful-empty"}
	gotNames := make([]string, len(cases))
	for i := range cases {
		gotNames[i] = cases[i].name
	}
	sort.Strings(gotNames)
	if !reflect.DeepEqual(gotNames, wantNames) {
		t.Fatalf("ASSERT_OPERATION33_PARITY_TABLE_COVERAGE: got=%v want=%v", gotNames, wantNames)
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			direct := runParityMatrixRoute(t, tc, false)
			canonical := runParityMatrixRoute(t, tc, true)
			if direct.delegated != canonical.delegated {
				t.Fatalf("ASSERT_OPERATION33_PARITY_EXACT_DELEGATED_BYTES: direct=%s canonical=%s", direct.delegated, canonical.delegated)
			}
			for route, got := range map[string]parityMatrixObservation{"direct": direct, "canonical": canonical} {
				if got.status != tc.wantStatus || got.outcome != tc.wantOutcome || got.isError != tc.wantError {
					t.Fatalf("ASSERT_OPERATION33_PARITY_STATUS_OUTCOME_ERROR_%s: got=%+v want=%s/%s/%t", route, got, tc.wantStatus, tc.wantOutcome, tc.wantError)
				}
				var env map[string]any
				_ = json.Unmarshal([]byte(got.delegated), &env)
				wantCode := any(nil)
				if tc.wantCode != "" {
					wantCode = tc.wantCode
				}
				if env["code"] != wantCode {
					t.Fatalf("ASSERT_OPERATION33_PARITY_CODE_%s: got=%v want=%v", route, env["code"], wantCode)
				}
				wantDiagnostics := any(nil)
				if tc.wantDiagnostic != "" {
					wantDiagnostics = []any{tc.wantDiagnostic}
				}
				if !reflect.DeepEqual(got.diagnostics, wantDiagnostics) {
					t.Fatalf("ASSERT_OPERATION33_PARITY_DIAGNOSTIC_%s: got=%v want=%v", route, got.diagnostics, wantDiagnostics)
				}
				if got.executorCalls != 1 || got.acquisitionCalls != 1 {
					t.Fatalf("ASSERT_OPERATION33_PARITY_INVOCATION_COUNT_%s: executor=%d acquisition=%d", route, got.executorCalls, got.acquisitionCalls)
				}
				if got.starts != 1 || got.teardownCalls != 0 || got.closeCalls != 0 {
					t.Fatalf("ASSERT_OPERATION33_PARITY_LIFECYCLE_SIDE_EFFECTS_%s: starts=%d teardown=%d close=%d", route, got.starts, got.teardownCalls, got.closeCalls)
				}
				wantMethods := []string{"initialize", "initialized", "textDocument/didOpen"}
				if !reflect.DeepEqual(got.methods, wantMethods) {
					t.Fatalf("ASSERT_OPERATION33_PARITY_LSP_METHOD_ORDER_%s: got=%v want=%v", route, got.methods, wantMethods)
				}
				if got.publishedCount != tc.wantPublishedCount {
					t.Fatalf("ASSERT_OPERATION33_PARITY_PUBLICATION_COUNT_%s: got=%d want=%d", route, got.publishedCount, tc.wantPublishedCount)
				}
				if tc.wantPublished && !bytes.Equal(got.publishedBytes, tc.artifact(t)) {
					t.Fatalf("ASSERT_OPERATION33_PARITY_PUBLICATION_BYTES_%s: got=%q want=%q", route, got.publishedBytes, tc.artifact(t))
				}
			}
		})
	}
}
