package sliceops

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"lsp-trace/internal/graph"
	"lsp-trace/internal/graphprovenance"
	"lsp-trace/internal/lspwire"
	"lsp-trace/internal/managedprocess"
	"lsp-trace/internal/operation"
	"lsp-trace/internal/retainedcalls"
	"lsp-trace/internal/runtimeprofile"
	"lsp-trace/internal/session"
	"lsp-trace/sessionruntime"
)

type provenanceWire struct {
	in        *io.PipeReader
	stdin     *io.PipeWriter
	out       *io.PipeWriter
	stdout    *io.PipeReader
	uri       string
	mu        sync.Mutex
	texts     []string
	callsMode string
}

func newProvenanceWire(uri string, mode ...string) *provenanceWire {
	in, stdin := io.Pipe()
	stdout, out := io.Pipe()
	w := &provenanceWire{in: in, stdin: stdin, out: out, stdout: stdout, uri: uri}
	if len(mode) > 0 {
		w.callsMode = mode[0]
	}
	go w.serve()
	return w
}
func (w *provenanceWire) serve() {
	reader := lspwire.NewReader(w.in, lspwire.DefaultLimits())
	writer := lspwire.NewWriter(w.out, lspwire.DefaultLimits())
	for {
		msg, err := reader.Read()
		if err != nil {
			return
		}
		result := json.RawMessage(`[]`)
		switch msg.Method {
		case "initialize":
			result = json.RawMessage(`{"capabilities":{"callHierarchyProvider":true,"positionEncoding":"utf-16"}}`)
		case "textDocument/didOpen", "textDocument/didChange":
			var p struct {
				TextDocument struct {
					Text string `json:"text"`
				} `json:"textDocument"`
				Changes []struct {
					Text string `json:"text"`
				} `json:"contentChanges"`
			}
			_ = json.Unmarshal(msg.Params, &p)
			text := p.TextDocument.Text
			if len(p.Changes) > 0 {
				text = p.Changes[0].Text
			}
			w.mu.Lock()
			w.texts = append(w.texts, text)
			w.mu.Unlock()
			continue
		case "initialized":
			continue
		case "textDocument/prepareCallHierarchy":
			result, _ = json.Marshal([]any{map[string]any{"name": "F", "kind": 12, "uri": w.uri, "range": map[string]any{"start": map[string]int{"line": 1, "character": 0}, "end": map[string]int{"line": 1, "character": 11}}, "selectionRange": map[string]any{"start": map[string]int{"line": 1, "character": 5}, "end": map[string]int{"line": 1, "character": 6}}}})
		case "callHierarchy/outgoingCalls":
			if w.callsMode != "" {
				item := map[string]any{"name": "F", "kind": 12, "uri": w.uri, "range": map[string]any{"start": map[string]int{"line": 1, "character": 0}, "end": map[string]int{"line": 1, "character": 11}}, "selectionRange": map[string]any{"start": map[string]int{"line": 1, "character": 5}, "end": map[string]int{"line": 1, "character": 6}}}
				sites := []graph.Range{}
				if w.callsMode == "repeated" {
					sites = []graph.Range{{Start: graph.Position{Line: 1, Character: 7}, End: graph.Position{Line: 1, Character: 8}}, {Start: graph.Position{Line: 1, Character: 9}, End: graph.Position{Line: 1, Character: 10}}}
				}
				call := map[string]any{"to": item, "fromRanges": sites}
				result, _ = json.Marshal([]any{call, call, call})
			}
		case "shutdown":
			result = json.RawMessage(`null`)
		case "exit":
			return
		}
		if len(msg.ID) > 0 {
			if err := writer.Write(lspwire.Message{JSONRPC: lspwire.Version, ID: msg.ID, Result: result}); err != nil {
				return
			}
		}
	}
}
func (w *provenanceWire) Stdin() io.WriteCloser { return w.stdin }
func (w *provenanceWire) Stdout() io.ReadCloser { return w.stdout }
func (w *provenanceWire) Teardown(context.Context) managedprocess.TeardownObservation {
	_ = w.stdin.Close()
	_ = w.in.Close()
	_ = w.out.Close()
	return managedprocess.TeardownObservation{Death: managedprocess.DeathObservation{Reap: managedprocess.ReapObservation{Kind: managedprocess.ReapComplete}}}
}
func (w *provenanceWire) Close() managedprocess.ResourceObservation {
	_ = w.stdout.Close()
	return managedprocess.ResourceObservation{Kind: managedprocess.ResourcesClosed}
}

type provenanceStarter struct {
	uri       string
	mu        sync.Mutex
	children  []*provenanceWire
	callsMode string
}

func (s *provenanceStarter) Start(context.Context, managedprocess.Spec) (sessionruntime.Child, managedprocess.StartObservation) {
	w := newProvenanceWire(s.uri, s.callsMode)
	s.mu.Lock()
	s.children = append(s.children, w)
	s.mu.Unlock()
	return w, managedprocess.StartObservation{Kind: managedprocess.StartStarted}
}

type interleavedRuntime struct {
	*sessionruntime.Manager
	once  sync.Once
	after func()
}

func (r *interleavedRuntime) RoundTrip(ctx context.Context, req sessionruntime.RoundTripRequest) sessionruntime.RoundTripResult {
	result := r.Manager.RoundTrip(ctx, req)
	if req.Method == "callHierarchy/outgoingCalls" {
		r.once.Do(r.after)
	}
	return result
}
func TestGraphProvenanceActualWireMutationAndRestart(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "f.go")
	a := []byte("package p\nfunc F() {} // A\n")
	b := []byte("package p\nfunc F() {} // B\n")
	if err := os.WriteFile(file, a, 0600); err != nil {
		t.Fatal(err)
	}
	uri := (&url.URL{Scheme: "file", Path: filepath.ToSlash(file)}).String()
	v, err := runtimeprofile.Validate(runtimeprofile.Selector{TrustDomain: "wire", Workspace: root, Profile: "go", EnvironmentReference: "test"})
	if err != nil {
		t.Fatal(err)
	}
	starter := &provenanceStarter{uri: uri}
	m, err := sessionruntime.New(sessionruntime.Config{Limits: sessionruntime.Limits{MaxSessions: 1, MaxRequests: 2, MaxChildren: 2, MaxCancels: 2, MaxTombstones: 4, MaxObservations: 64}, Starter: starter})
	if err != nil {
		t.Fatal(err)
	}
	started := m.Start(context.Background(), sessionruntime.StartRequest{Profile: runtimeprofile.Resolve(v)})
	ready := m.BeginReadiness(context.Background(), started.SessionID, 1, time.Now().Add(time.Second))
	observed, _ := m.WaitReadiness(context.Background(), ready.ID)
	if observed.State != sessionruntime.ReadinessReady {
		t.Fatal(observed)
	}
	defer func() {
		m.Stop(context.Background(), started.SessionID, "test-end")
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = m.Shutdown(ctx)
	}()
	var competing *sessionruntime.DocumentSupply
	runtime := &interleavedRuntime{Manager: m, after: func() {
		if err := os.WriteFile(file, b, 0600); err != nil {
			t.Fatal(err)
		}
		other := m.PrepareDocument(context.Background(), sessionruntime.DocumentRequest{SessionID: started.SessionID, Generation: 1, URI: uri, LanguageID: "go", CaptureSupply: true})
		if other.Failure != "" {
			t.Fatal(other)
		}
		competing = other.Supply
	}}
	input := map[string]any{"session_id": started.SessionID, "generation": 1, "start_mode": "at", "uri": uri, "line": 1, "character": 5, "graph_provenance": true}
	run := func() graphprovenance.Evidence {
		raw, _ := json.Marshal(input)
		result, failed := NewExecutor(runtime).Execute(context.Background(), operation.Request{Name: OperationSlice, Input: raw})
		if failed != nil {
			t.Fatal(failed)
		}
		var e graphprovenance.Evidence
		if err := json.Unmarshal(result.Artifact, &e); err != nil {
			t.Fatal(err)
		}
		if _, err := graphprovenance.ValidateFor(result.Artifact, graphprovenance.Family, "v1"); err != nil {
			t.Fatal(err)
		}
		exported, err := retainedcalls.Export(result.Artifact)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = retainedcalls.ValidateFor(exported, retainedcalls.Family, "v1"); err != nil {
			t.Fatal(err)
		}
		return e
	}
	e := run()
	if e.Supply == nil || !bytes.Equal(e.Supply.Content, a) || !bytes.Equal(e.Captures[0].Content, b) || competing == nil || competing.DocumentVersion != 2 || e.Supply.Supply.Version != 1 || e.AnalyzedVersion != graphprovenance.Unverified {
		t.Fatalf("ASSERT_ACTUAL_WIRE_A_THEN_B_UNCERTAINTY: %+v competing=%+v", e, competing)
	}
	starter.mu.Lock()
	w := starter.children[0]
	starter.mu.Unlock()
	w.mu.Lock()
	texts := append([]string(nil), w.texts...)
	w.mu.Unlock()
	if len(texts) != 2 || texts[0] != string(a) || texts[1] != string(b) {
		t.Fatalf("ASSERT_SERVER_ACTUALLY_RECEIVED_A_THEN_B: %q", texts)
	}
	cached := run()
	if cached.Supply != nil || cached.SupplyStatus != "NO_NOTIFICATION_OBSERVATION" {
		t.Fatal("ASSERT_WIRE_CACHE_NO_MANUFACTURED_CHANGE")
	}
	accepted := m.Restart(context.Background(), started.SessionID, "restart")
	deadline := time.Now().Add(2 * time.Second)
	for {
		op, ok := m.Operation(accepted.IntentID)
		if ok && op.State != sessionruntime.OperationPending {
			if op.State != sessionruntime.OperationComplete {
				t.Fatal(op)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("restart deadline")
		}
		time.Sleep(time.Millisecond)
	}
	stale := m.PrepareDocument(context.Background(), sessionruntime.DocumentRequest{SessionID: started.SessionID, Generation: 1, URI: uri, LanguageID: "go", CaptureSupply: true})
	if stale.Failure != session.StaleGeneration || stale.Supply != nil {
		t.Fatal("ASSERT_SUCCESSFUL_RESTART_STALE_REJECT")
	}
	input["generation"] = 2
	fresh := run()
	if fresh.Generation != 2 || fresh.Supply == nil || fresh.Supply.Supply.Version != 1 || !bytes.Equal(fresh.Supply.Content, b) {
		t.Fatal("ASSERT_RESTART_OWN_SUPPLY_GENERATION")
	}
	if e.Supply.Supply.Generation != 1 || e.AnalyzedVersion != graphprovenance.Unverified {
		t.Fatal("ASSERT_HISTORICAL_SUPPLY_NOT_UPGRADED")
	}
}
