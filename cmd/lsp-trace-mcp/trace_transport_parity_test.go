package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"lsp-trace/internal/lspwire"
	"lsp-trace/internal/managedprocess"
	"lsp-trace/internal/mcp"
	"lsp-trace/internal/mcpcontract"
	"lsp-trace/internal/operation"
	"lsp-trace/internal/runtimeprofile"
	"lsp-trace/sessionruntime"
)

type parityProcess struct {
	in      *io.PipeReader
	stdin   *io.PipeWriter
	out     *io.PipeWriter
	stdout  *io.PipeReader
	uri     string
	hang    bool
	mu      sync.Mutex
	methods []string
}

func newParityProcess(uri string, hang bool) *parityProcess {
	in, stdin := io.Pipe()
	stdout, out := io.Pipe()
	p := &parityProcess{in: in, stdin: stdin, out: out, stdout: stdout, uri: uri, hang: hang}
	go p.serve()
	return p
}

func (p *parityProcess) serve() {
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
		if p.hang && (msg.Method == "textDocument/prepareCallHierarchy" || msg.Method == "callHierarchy/outgoingCalls") {
			continue
		}
		result := json.RawMessage(`[]`)
		switch msg.Method {
		case "initialize":
			result = json.RawMessage(`{"capabilities":{"callHierarchyProvider":true,"documentSymbolProvider":true,"positionEncoding":"utf-16"}}`)
		case "initialized", "textDocument/didOpen", "textDocument/didChange", "$/cancelRequest":
			continue
		case "textDocument/prepareCallHierarchy":
			result = p.items()
		case "callHierarchy/outgoingCalls":
			var items []any
			_ = json.Unmarshal(p.items(), &items)
			result, _ = json.Marshal([]any{map[string]any{"to": items[0], "fromRanges": []any{map[string]any{"start": map[string]int{"line": 1, "character": 2}, "end": map[string]int{"line": 1, "character": 3}}}}})
		case "callHierarchy/incomingCalls":
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

func (p *parityProcess) items() json.RawMessage {
	item := map[string]any{"name": "F", "kind": 12, "uri": p.uri,
		"range":          map[string]any{"start": map[string]int{"line": 1, "character": 0}, "end": map[string]int{"line": 1, "character": 6}},
		"selectionRange": map[string]any{"start": map[string]int{"line": 1, "character": 5}, "end": map[string]int{"line": 1, "character": 6}}}
	raw, _ := json.Marshal([]any{item})
	return raw
}
func (p *parityProcess) Methods() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]string{}, p.methods...)
}
func (p *parityProcess) Stdin() io.WriteCloser { return p.stdin }
func (p *parityProcess) Stdout() io.ReadCloser { return p.stdout }
func (p *parityProcess) Teardown(context.Context) managedprocess.TeardownObservation {
	_ = p.stdin.Close()
	_ = p.in.Close()
	_ = p.out.Close()
	return managedprocess.TeardownObservation{Death: managedprocess.DeathObservation{Reap: managedprocess.ReapObservation{Kind: managedprocess.ReapComplete}}}
}
func (p *parityProcess) Close() managedprocess.ResourceObservation {
	_ = p.stdout.Close()
	return managedprocess.ResourceObservation{Kind: managedprocess.ResourcesClosed}
}

type parityStarter struct {
	uri     string
	hang    bool
	process *parityProcess
}

func (s *parityStarter) Start(context.Context, managedprocess.Spec) (sessionruntime.Child, managedprocess.StartObservation) {
	s.process = newParityProcess(s.uri, s.hang)
	return s.process, managedprocess.StartObservation{Kind: managedprocess.StartStarted}
}

type countingTraceExecutor struct {
	delegate *traceExecutor
	calls    int
}

func (e *countingTraceExecutor) Execute(ctx context.Context, r operation.Request) (operation.Result, *operation.Failure) {
	e.calls++
	return e.delegate.Execute(ctx, r)
}

type parityObservation struct {
	delegated             string
	outcome, status, code string
	diagnostics           []string
	isError               bool
	calls                 int
	methods               []string
	lifecycle             []string
}

type parityCase struct {
	name                              string
	alias                             bool
	ready                             bool
	omitGeneration                    bool
	stale                             bool
	finalDrift                        bool
	hang                              bool
	cancel                            bool
	requestTimeout                    int
	wantOutcome, wantStatus, wantCode string
	wantError                         bool
	wantMethods                       []string
	wantLifecycle                     []string
	wantDiagnostics                   []string
}

var operation33ParityDimensions = []string{"delegated-envelope-bytes", "outcome", "operation-status", "code", "diagnostics", "isError", "executor-calls", "lsp-methods", "lifecycle-state-failure"}

var operation33ParityCases = []parityCase{
	{name: "alias-resolution", alias: true, ready: true, wantOutcome: "COMPLETE", wantStatus: "SUCCEEDED", wantMethods: []string{"initialize", "initialized", "textDocument/didOpen", "textDocument/prepareCallHierarchy", "callHierarchy/outgoingCalls"}, wantLifecycle: []string{"startup:INITIALIZING:", "readiness:READY:", "document:READY:", "request:READY:", "response:READY:", "request:READY:", "response:READY:"}},
	{name: "non-ready-session", alias: true, omitGeneration: true, wantOutcome: "DOMAIN_ERROR", wantStatus: "FAILED", wantCode: "SESSION_NOT_READY", wantError: true, wantMethods: []string{}, wantLifecycle: []string{"startup:INITIALIZING:"}},
	{name: "missing-generation", ready: true, omitGeneration: true, wantOutcome: "COMPLETE", wantStatus: "SUCCEEDED", wantMethods: []string{"initialize", "initialized", "textDocument/didOpen", "textDocument/prepareCallHierarchy", "callHierarchy/outgoingCalls"}, wantLifecycle: []string{"startup:INITIALIZING:", "readiness:READY:", "document:READY:", "request:READY:", "response:READY:", "request:READY:", "response:READY:"}},
	{name: "stale-generation", ready: true, stale: true, wantOutcome: "DOMAIN_ERROR", wantStatus: "FAILED", wantCode: "STALE_GENERATION", wantError: true, wantMethods: []string{"initialize", "initialized"}, wantLifecycle: []string{"startup:INITIALIZING:", "readiness:READY:"}},
	{name: "final-ready-drift", ready: true, finalDrift: true, wantOutcome: "DOMAIN_ERROR", wantStatus: "FAILED", wantCode: "LIFECYCLE_CONFLICT", wantError: true, wantMethods: []string{"initialize", "initialized"}, wantLifecycle: []string{"startup:INITIALIZING:", "readiness:READY:", "crash:CRASHED:SESSION_CRASHED"}},
	{name: "request-timeout", ready: true, hang: true, requestTimeout: 5, wantOutcome: "PARTIAL", wantStatus: "PARTIAL", wantMethods: []string{"initialize", "initialized", "textDocument/didOpen", "textDocument/prepareCallHierarchy", "$/cancelRequest"}, wantLifecycle: []string{"startup:INITIALIZING:", "readiness:READY:", "document:READY:", "request:READY:", "cancel:READY:REQUEST_CANCELLED", "response:POISONED:REQUEST_TIMEOUT"}},
	{name: "caller-cancellation", ready: true, cancel: true, wantOutcome: "DOMAIN_ERROR", wantStatus: "FAILED", wantCode: "REQUEST_CANCELLED", wantError: true, wantMethods: []string{"initialize", "initialized"}, wantLifecycle: []string{"startup:INITIALIZING:", "readiness:READY:"}},
}

func runOperation33ParityRoute(t *testing.T, workspace string, tc parityCase, gateway bool) parityObservation {
	t.Helper()
	path := filepath.Join(workspace, "code.go")
	uri := (&url.URL{Scheme: "file", Path: filepath.ToSlash(path)}).String()
	profile, err := runtimeprofile.Validate(runtimeprofile.Selector{TrustDomain: "trace-parity", Workspace: workspace, Profile: "fixture", EnvironmentReference: "test"})
	if err != nil {
		t.Fatal(err)
	}
	starter := &parityStarter{uri: uri, hang: tc.hang}
	var manager *sessionruntime.Manager
	var started sessionruntime.StartResult
	config := sessionruntime.Config{Limits: sessionruntime.Limits{MaxSessions: 1, MaxRequests: 8, MaxChildren: 2, MaxCancels: 8, MaxTombstones: 8, MaxObservations: 128, MaxOperations: 8}, Starter: starter}
	if tc.finalDrift {
		config.DocumentFinalHook = func() { manager.ObserveCrash(started.SessionID, started.Generation) }
	}
	manager, err = sessionruntime.New(config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = manager.Shutdown(context.Background()) })
	started = manager.Start(context.Background(), sessionruntime.StartRequest{Profile: runtimeprofile.Resolve(profile), BootstrapAlias: "trace-parity", LanguageID: "go"})
	if tc.ready {
		pending := manager.BeginReadiness(context.Background(), started.SessionID, started.Generation, time.Now().Add(time.Second))
		ready, ok := manager.WaitReadiness(context.Background(), pending.ID)
		if !ok || ready.State != sessionruntime.ReadinessReady {
			t.Fatalf("ASSERT_TRACE_PARITY_READY: %+v", ready)
		}
	}
	runtime := newHostSelectorRuntime(manager, []bootstrapSession{{Alias: "trace-alias", SessionID: started.SessionID}})
	exec := &countingTraceExecutor{delegate: &traceExecutor{runtime: runtime}}
	server := &mcp.Server{Registry: mcp.NewRegistryWithProfile(false, mcp.ToolProfileFull), Executors: map[mcp.ExecutorFamily]mcp.Executor{mcp.TraceExecutorFamily: exec}}
	id := started.SessionID
	if tc.alias {
		id = "trace-alias"
	}
	generation := started.Generation
	if tc.omitGeneration {
		generation = 0
	}
	if tc.stale {
		generation++
	}
	args := map[string]any{"session_id": id, "uri": uri, "line": float64(1), "character": float64(5), "down_depth": float64(2), "up_depth": float64(2)}
	if generation != 0 {
		args["generation"] = generation
	}
	if tc.requestTimeout != 0 {
		args["request_timeout_ms"] = tc.requestTimeout
	}
	name := mcpcontract.TraceTool
	if gateway {
		name = "lsp_trace_v1_execute"
		args = map[string]any{"request": map[string]any{"operation": mcpcontract.TraceTool, "arguments": args}}
	}
	params, _ := json.Marshal(map[string]any{"name": name, "arguments": args})
	request := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":` + string(params) + `}` + "\n"
	ctx := context.Background()
	if tc.cancel {
		canceled, cancel := context.WithCancel(ctx)
		cancel()
		ctx = canceled
	}
	var out bytes.Buffer
	if err := server.ServeContext(ctx, strings.NewReader(request), &out); err != nil {
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
		t.Fatalf("ASSERT_TRACE_PARITY_RPC_SUCCESS: %v", wire.Error)
	}
	delegated := string(wire.Result.Structured)
	if gateway {
		var outer map[string]any
		if err := json.Unmarshal(wire.Result.Structured, &outer); err != nil {
			t.Fatal(err)
		}
		delegated, _ = outer["delegated_envelope"].(string)
	}
	var env struct {
		Outcome         string   `json:"outcome"`
		OperationStatus string   `json:"operation_status"`
		Code            string   `json:"code"`
		Diagnostics     []string `json:"diagnostics"`
		IsError         bool     `json:"isError"`
	}
	if err := json.Unmarshal([]byte(delegated), &env); err != nil {
		t.Fatal(err)
	}
	methods := []string{}
	if starter.process != nil {
		methods = starter.process.Methods()
	}
	lifecycle := []string{}
	for _, observation := range manager.Observations() {
		lifecycle = append(lifecycle, fmt.Sprintf("%s:%s:%s", observation.Kind, observation.State, observation.Failure))
	}
	return parityObservation{delegated: delegated, outcome: env.Outcome, status: env.OperationStatus, code: env.Code, diagnostics: env.Diagnostics, isError: env.IsError, calls: exec.calls, methods: methods, lifecycle: lifecycle}
}

func TestOperation33RealRegistryGatewayParity(t *testing.T) {
	for _, tc := range operation33ParityCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			workspace := t.TempDir()
			if err := os.WriteFile(filepath.Join(workspace, "code.go"), []byte("package p\nfunc F() {}\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			direct := runOperation33ParityRoute(t, workspace, tc, false)
			canonical := runOperation33ParityRoute(t, workspace, tc, true)
			if direct.delegated != canonical.delegated {
				t.Fatalf("ASSERT_OPERATION33_DELEGATED_ENVELOPE_BYTES: direct=%s canonical=%s", direct.delegated, canonical.delegated)
			}
			for route, got := range map[string]parityObservation{"direct": direct, "canonical": canonical} {
				if got.outcome != tc.wantOutcome || got.status != tc.wantStatus || got.code != tc.wantCode || got.isError != tc.wantError {
					t.Errorf("ASSERT_OPERATION33_ENVELOPE_DIMENSIONS_%s: got=%+v", route, got)
				}
				if got.calls != 1 {
					t.Errorf("ASSERT_OPERATION33_EXECUTOR_ONCE_%s: %d", route, got.calls)
				}
				if !reflect.DeepEqual(got.methods, tc.wantMethods) {
					t.Errorf("ASSERT_OPERATION33_LSP_SIDE_EFFECTS_%s: got=%v want=%v", route, got.methods, tc.wantMethods)
				}
				if !reflect.DeepEqual(got.lifecycle, tc.wantLifecycle) {
					t.Errorf("ASSERT_OPERATION33_LIFECYCLE_SIDE_EFFECTS_%s: got=%v want=%v", route, got.lifecycle, tc.wantLifecycle)
				}
				if !reflect.DeepEqual(got.diagnostics, tc.wantDiagnostics) {
					t.Errorf("ASSERT_OPERATION33_DIAGNOSTICS_%s: got=%v want=%v", route, got.diagnostics, tc.wantDiagnostics)
				}
			}
		})
	}
}

func TestOperation33ParityTableCoverage(t *testing.T) {
	want := []string{"alias-resolution", "non-ready-session", "missing-generation", "stale-generation", "final-ready-drift", "request-timeout", "caller-cancellation"}
	wantDimensions := []string{"delegated-envelope-bytes", "outcome", "operation-status", "code", "diagnostics", "isError", "executor-calls", "lsp-methods", "lifecycle-state-failure"}
	if !reflect.DeepEqual(operation33ParityDimensions, wantDimensions) {
		t.Fatalf("ASSERT_OPERATION33_PARITY_DIMENSION_COVERAGE: got=%v want=%v", operation33ParityDimensions, wantDimensions)
	}
	if len(operation33ParityCases) != len(want) {
		t.Fatalf("ASSERT_OPERATION33_PARITY_ROW_COUNT: got=%d want=%d", len(operation33ParityCases), len(want))
	}
	for i, name := range want {
		tc := operation33ParityCases[i]
		if tc.name != name {
			t.Fatalf("ASSERT_OPERATION33_PARITY_ROW_%d: got=%q want=%q", i, tc.name, name)
		}
		if tc.wantOutcome == "" || tc.wantStatus == "" || tc.wantMethods == nil || tc.wantLifecycle == nil {
			t.Fatalf("ASSERT_OPERATION33_PARITY_DIMENSIONS_%s: %+v", name, tc)
		}
	}
}
