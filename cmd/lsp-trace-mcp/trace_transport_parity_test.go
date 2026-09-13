package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/url"
	"os"
	"path/filepath"
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
	empty   bool
	mu      sync.Mutex
	methods []string
}

func newParityProcess(uri string, empty bool) *parityProcess {
	in, stdin := io.Pipe()
	stdout, out := io.Pipe()
	p := &parityProcess{in: in, stdin: stdin, out: out, stdout: stdout, uri: uri, empty: empty}
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
		result := json.RawMessage(`[]`)
		switch msg.Method {
		case "initialize":
			result = json.RawMessage(`{"capabilities":{"callHierarchyProvider":true,"documentSymbolProvider":true,"positionEncoding":"utf-16"}}`)
		case "initialized", "textDocument/didOpen", "textDocument/didChange":
			continue
		case "textDocument/prepareCallHierarchy":
			result = p.items()
		case "callHierarchy/outgoingCalls":
			if !p.empty {
				var items []any
				_ = json.Unmarshal(p.items(), &items)
				result, _ = json.Marshal([]any{map[string]any{"to": items[0], "fromRanges": []any{map[string]any{"start": map[string]int{"line": 1, "character": 2}, "end": map[string]int{"line": 1, "character": 3}}}}})
			}
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
	uri   string
	empty bool
}

func (s parityStarter) Start(context.Context, managedprocess.Spec) (sessionruntime.Child, managedprocess.StartObservation) {
	return newParityProcess(s.uri, s.empty), managedprocess.StartObservation{Kind: managedprocess.StartStarted}
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
	delegated string
	outcome   any
	isError   any
	calls     int
	methods   int
}

func runRealTraceTransport(t *testing.T, gateway, empty bool) parityObservation {
	t.Helper()
	workspace := t.TempDir()
	path := filepath.Join(workspace, "code.go")
	if err := os.WriteFile(path, []byte("package p\nfunc F() {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	uri := (&url.URL{Scheme: "file", Path: filepath.ToSlash(path)}).String()
	profile, err := runtimeprofile.Validate(runtimeprofile.Selector{TrustDomain: "trace-parity", Workspace: workspace, Profile: "fixture", EnvironmentReference: "test"})
	if err != nil {
		t.Fatal(err)
	}
	manager, err := sessionruntime.New(sessionruntime.Config{Limits: sessionruntime.Limits{MaxSessions: 1, MaxRequests: 8, MaxChildren: 2, MaxCancels: 8, MaxTombstones: 8, MaxObservations: 128, MaxOperations: 8}, Starter: parityStarter{uri: uri, empty: empty}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = manager.Shutdown(context.Background()) })
	started := manager.Start(context.Background(), sessionruntime.StartRequest{Profile: runtimeprofile.Resolve(profile), BootstrapAlias: "trace-parity", LanguageID: "go"})
	pending := manager.BeginReadiness(context.Background(), started.SessionID, started.Generation, time.Now().Add(time.Second))
	ready, ok := manager.WaitReadiness(context.Background(), pending.ID)
	if !ok || ready.State != sessionruntime.ReadinessReady {
		t.Fatalf("ASSERT_TRACE_PARITY_READY: %+v", ready)
	}
	exec := &countingTraceExecutor{delegate: &traceExecutor{runtime: manager}}
	server := &mcp.Server{Registry: mcp.NewRegistryWithProfile(false, mcp.ToolProfileFull), Executors: map[mcp.ExecutorFamily]mcp.Executor{mcp.TraceExecutorFamily: exec}}
	depth := float64(1)
	if empty {
		depth = 0
	}
	args := map[string]any{"session_id": started.SessionID, "generation": started.Generation, "uri": uri, "line": float64(1), "character": float64(5), "down_depth": depth, "up_depth": float64(0)}
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
		t.Fatalf("ASSERT_TRACE_PARITY_RPC_SUCCESS: %v", wire.Error)
	}
	var structured map[string]any
	if err := json.Unmarshal(wire.Result.Structured, &structured); err != nil {
		t.Fatal(err)
	}
	delegated := string(wire.Result.Structured)
	outcome, isError := structured["outcome"], structured["isError"]
	if gateway {
		delegated, _ = structured["delegated_envelope"].(string)
		outcome, isError = structured["delegated_outcome"], structured["delegated_is_error"]
	}
	return parityObservation{delegated: delegated, outcome: outcome, isError: isError, calls: exec.calls}
}

func TestOperation33RealRegistryTransportDirectCanonicalParity(t *testing.T) {
	for _, empty := range []bool{false, true} {
		name := "complete"
		if empty {
			name = "successful-empty"
		}
		t.Run(name, func(t *testing.T) {
			direct := runRealTraceTransport(t, false, empty)
			canonical := runRealTraceTransport(t, true, empty)
			if direct.delegated != canonical.delegated || direct.outcome != canonical.outcome || direct.isError != canonical.isError {
				t.Fatalf("ASSERT_OPERATION33_REAL_TRANSPORT_BYTE_PARITY: direct=%+v canonical=%+v", direct, canonical)
			}
			if direct.calls != 1 || canonical.calls != 1 {
				t.Fatalf("ASSERT_OPERATION33_ONE_EXECUTION_PER_ROUTE: direct=%d canonical=%d", direct.calls, canonical.calls)
			}
		})
	}
}
