//go:build darwin || linux

package censuslineagepilot

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"lsp-trace/internal/acquisition"
	"lsp-trace/internal/censusacquisition"
	"lsp-trace/internal/directionalraw"
	"lsp-trace/internal/lsp"
	"lsp-trace/internal/lspwire"
	"lsp-trace/internal/managedprocess"
	"lsp-trace/internal/runtimeprofile"
	"lsp-trace/sessionruntime"
)

type scopeAdapterChild struct {
	input         *io.PipeReader
	stdin         *io.PipeWriter
	stdout        *io.PipeReader
	output        *io.PipeWriter
	once          sync.Once
	mu            sync.Mutex
	messages      []lspwire.Message
	done          chan struct{}
	poison        bool
	response      func(lspwire.Message) json.RawMessage
	notifications int
	stdoutBytes   atomic.Int64
}

func (c *scopeAdapterChild) Stdin() io.WriteCloser { return c.stdin }
func (c *scopeAdapterChild) Stdout() io.ReadCloser { return scopeAdapterReadCounter{c} }

type scopeAdapterReadCounter struct{ child *scopeAdapterChild }

func (r scopeAdapterReadCounter) Read(p []byte) (int, error) {
	n, err := r.child.stdout.Read(p)
	r.child.stdoutBytes.Add(int64(n))
	return n, err
}
func (r scopeAdapterReadCounter) Close() error { return r.child.stdout.Close() }
func (c *scopeAdapterChild) Close() managedprocess.ResourceObservation {
	c.once.Do(func() { c.input.Close(); c.stdin.Close(); c.stdout.Close(); c.output.Close() })
	return managedprocess.ResourceObservation{Kind: managedprocess.ResourcesClosed}
}
func (c *scopeAdapterChild) Teardown(context.Context) managedprocess.TeardownObservation {
	c.Close()
	return managedprocess.TeardownObservation{Death: managedprocess.DeathObservation{Kind: managedprocess.DeathExited, Reap: managedprocess.ReapObservation{Kind: managedprocess.ReapComplete}}}
}
func (c *scopeAdapterChild) snapshot() []lspwire.Message {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]lspwire.Message(nil), c.messages...)
}

type scopeAdapterStarter struct{ child *scopeAdapterChild }

func (s scopeAdapterStarter) Start(context.Context, managedprocess.Spec) (sessionruntime.Child, managedprocess.StartObservation) {
	return s.child, managedprocess.StartObservation{Kind: managedprocess.StartStarted}
}
func scopeAdapterItem(uri string) lsp.CallHierarchyItem {
	return lsp.CallHierarchyItem{Name: "Query", Kind: 12, Detail: "opaque detail", Tags: []int{1}, URI: uri, Range: lsp.Range{End: lsp.Position{Line: 1}}, SelectionRange: lsp.Range{End: lsp.Position{Character: 5}}, Data: json.RawMessage(`{"URI":"opaque alias","nested":{"value":7}}`)}
}
func (c *scopeAdapterChild) serve() {
	defer close(c.done)
	reader := lspwire.NewReader(c.input, lspwire.DefaultLimits())
	writer := lspwire.NewWriter(c.output, lspwire.DefaultLimits())
	for {
		msg, err := reader.Read()
		if err != nil {
			return
		}
		c.mu.Lock()
		c.messages = append(c.messages, msg)
		poison := c.poison
		response := c.response
		notifications := c.notifications
		c.mu.Unlock()
		if msg.Method == "textDocument/didOpen" || msg.Method == "textDocument/didChange" || msg.Method == "initialized" {
			continue
		}
		if poison {
			c.Close()
			return
		}
		var value any = []any{}
		switch msg.Method {
		case "initialize":
			value = map[string]any{"capabilities": map[string]any{"positionEncoding": "utf-16", "documentSymbolProvider": true, "callHierarchyProvider": true}}
		case "textDocument/documentSymbol":
			value = []lsp.DocumentSymbol{{Name: "Query", Kind: 12, Range: lsp.Range{End: lsp.Position{Line: 1}}, SelectionRange: lsp.Range{End: lsp.Position{Character: 5}}}}
		case "textDocument/prepareCallHierarchy":
			var p lsp.PrepareCallHierarchyParams
			_ = json.Unmarshal(msg.Params, &p)
			value = []lsp.CallHierarchyItem{scopeAdapterItem(p.TextDocument.URI)}
		case "callHierarchy/incomingCalls":
			value = []lsp.CallHierarchyIncomingCall{{From: scopeAdapterItem("file:///external.go"), FromRanges: []lsp.Range{}}}
		case "callHierarchy/outgoingCalls":
			value = []lsp.CallHierarchyOutgoingCall{{To: scopeAdapterItem("file:///external.go"), FromRanges: []lsp.Range{}}}
		}
		if msg.Method != "initialize" {
			for i := 0; i < notifications; i++ {
				if writer.Write(lspwire.Message{JSONRPC: lspwire.Version, Method: "window/logMessage", Params: json.RawMessage(`{"type":3,"message":"private fixture"}`)}) != nil {
					return
				}
			}
		}
		raw, _ := json.Marshal(value)
		if response != nil && msg.Method != "initialize" {
			raw = response(msg)
		}
		if writer.Write(lspwire.Message{JSONRPC: lspwire.Version, ID: msg.ID, Result: raw}) != nil {
			return
		}
	}
}
func adapterFixture(t *testing.T) (scopeRestartResult, *sessionruntime.Manager, censusacquisition.SessionIdentity, *scopeAdapterChild) {
	t.Helper()
	// Disposable retained fixtures exercise the actual read-only reconstruction;
	// no caller DTO or fabricated scopeRestartResult substitutes for the reader.
	owner, _, locator := restartFixture(t, false)
	restart, err := readRetainedFeatureScope(context.Background(), owner.paths, locator)
	if err != nil {
		t.Fatal(err)
	}
	var manifest scopeManifest
	_ = json.Unmarshal(restart.Scope().ManifestBytes(), &manifest)
	input, stdin := io.Pipe()
	stdout, output := io.Pipe()
	child := &scopeAdapterChild{input: input, stdin: stdin, stdout: stdout, output: output, done: make(chan struct{})}
	go child.serve()
	m, err := sessionruntime.New(sessionruntime.Config{Starter: scopeAdapterStarter{child}, Limits: sessionruntime.Limits{MaxSessions: 1, MaxRequests: 1, MaxChildren: 2, MaxCancels: 2, MaxTombstones: 4, MaxObservations: 1024, MaxOperations: 2}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { child.Close(); <-child.done; m.Shutdown(context.Background()) })
	validated, err := runtimeprofile.Validate(runtimeprofile.Selector{TrustDomain: "test", Workspace: manifest.Root, Profile: "go", EnvironmentReference: "local"})
	if err != nil {
		t.Fatal(err)
	}
	start := m.Start(context.Background(), sessionruntime.StartRequest{Profile: runtimeprofile.Resolve(validated), LanguageID: "go"})
	if start.Failure != "" {
		t.Fatal(start)
	}
	r := m.BeginReadiness(context.Background(), start.SessionID, start.Generation, time.Now().Add(time.Second))
	r, ok := m.WaitReadiness(context.Background(), r.ID)
	if !ok || r.Failure != "" {
		t.Fatal(r, ok)
	}
	return restart, m, censusacquisition.SessionIdentity{SessionID: start.SessionID, Generation: start.Generation}, child
}
func semanticMessages(c *scopeAdapterChild) []lspwire.Message {
	out := []lspwire.Message{}
	for _, m := range c.snapshot() {
		switch m.Method {
		case "textDocument/documentSymbol", "textDocument/prepareCallHierarchy", "callHierarchy/incomingCalls", "callHierarchy/outgoingCalls":
			out = append(out, m)
		}
	}
	return out
}
func TestScopeAdapterPrivateFullPath(t *testing.T) {
	restart, m, id, child := adapterFixture(t)
	err := m.WithReadySessionLease(context.Background(), sessionruntime.SessionLeaseRequest{SessionID: id.SessionID, Generation: id.Generation, Workspace: restart.Scope().Selection().Workspace}, func(ctx context.Context) error {
		// A pre-existing captured document must be usable through the real cache.
		f := restart.Scope().Files()[0]
		cached := m.PrepareDocument(ctx, sessionruntime.DocumentRequest{SessionID: id.SessionID, Generation: id.Generation, URI: f.URI, LanguageID: "go", CaptureSupply: true})
		if cached.Failure != "" || cached.Supply == nil {
			t.Fatal(cached)
		}
		a, b, err := newScopeRuntimeAdapter(ctx, restart, m, id)
		if err != nil || b.adapter != a {
			t.Fatal(err)
		}
		if len(a.files) != 4 || len(a.guard.MembersPrivate()) != 6 || len(semanticMessages(child)) != 0 {
			t.Fatal("binding inventory/early semantics")
		}
		// Frozen supplier and semantic requests must not reopen even missing files.
		for _, f := range restart.Scope().Files() {
			var manifest scopeManifest
			_ = json.Unmarshal(restart.Scope().ManifestBytes(), &manifest)
			if err := os.Remove(filepath.Join(manifest.Root, f.Path)); err != nil {
				t.Fatal(err)
			}
		}
		contextFile := restart.Scope().Files()[5]
		if err := a.Supply(ctx, censusacquisition.SourceFile{Path: contextFile.Path, URI: contextFile.URI, LanguageID: "go"}); err == nil {
			t.Fatal("context nominated")
		}
		before := len(child.snapshot())
		for _, f := range a.files {
			if err := a.Supply(ctx, f); err != nil {
				t.Fatal(err)
			}
		}
		if len(child.snapshot()) != before {
			t.Fatal("supplier wrote")
		}
		d, err := a.Discover(ctx, id)
		if err != nil || !d.Complete || d.Accounting.FileDenominator != 4 || len(d.Targets) != 4 {
			t.Fatalf("discovery %+v %v", d, err)
		}
		item := scopeAdapterItem(a.files[0].URI)
		in, _, err := a.IncomingCalls(ctx, item)
		if err != nil || len(in) != 1 {
			t.Fatal(err)
		}
		out, _, err := a.OutgoingCalls(ctx, item)
		if err != nil || len(out) != 1 {
			t.Fatal(err)
		}
		msgs := semanticMessages(child)
		for _, msg := range msgs[len(msgs)-2:] {
			var p struct {
				Item lsp.CallHierarchyItem `json:"item"`
			}
			if json.Unmarshal(msg.Params, &p) != nil || !reflect.DeepEqual(p.Item, item) {
				t.Fatal("full item/data lost", string(msg.Params))
			}
		}
		opens := 0
		for _, msg := range child.snapshot() {
			if msg.Method == "textDocument/didOpen" {
				opens++
			}
			if msg.Method == "textDocument/documentSymbol" && opens != 6 {
				t.Fatal("not all six supplied first")
			}
		}
		if opens != 6 {
			t.Fatal(opens)
		}
		// Manager's real shared seam refuses an ordinary, unguarded bypass.
		p, _ := json.Marshal(lsp.DocumentSymbolParams{TextDocument: lsp.TextDocumentIdentifier{URI: a.files[0].URI}})
		before = len(child.snapshot())
		rr := m.RoundTrip(ctx, sessionruntime.RoundTripRequest{SessionID: id.SessionID, Generation: id.Generation, Method: "textDocument/documentSymbol", Params: p, Deadline: time.Now().Add(time.Second), MaxMessages: 64, MaxBytes: 2 << 20})
		if rr.Failure == "" || len(child.snapshot()) != before {
			t.Fatal("unguarded bypass")
		}
		external := item
		external.URI = "file:///external.go"
		if _, _, err = a.OutgoingCalls(ctx, external); err == nil || len(child.snapshot()) != before {
			t.Fatal("external write")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
func TestScopeAdapterPrivateSupplyRefusals(t *testing.T) {
	for _, kind := range []string{"zero", "empty-state", "bytes", "context-bytes", "stale", "cancel", "language", "linked-receipt", "metadata", "head", "retained-body"} {
		t.Run(kind, func(t *testing.T) {
			restart, m, id, child := adapterFixture(t)
			ctx := context.Background()
			switch kind {
			case "zero":
				restart = scopeRestartResult{}
			case "empty-state":
				restart = scopeRestartResult{state: &scopeRestartState{}}
			case "linked-receipt":
				restart.state.linked[0][0] = '!'
			case "metadata":
				restart.state.metadata[0] = '!'
			case "head":
				restart.state.head[0] = '!'
			case "retained-body":
				restart.state.scope.state.files[0].Bytes[0] = '!'
			case "bytes", "context-bytes":
				i := 0
				if kind == "context-bytes" {
					i = 5
				}
				f := restart.Scope().Files()[i]
				var manifest scopeManifest
				_ = json.Unmarshal(restart.Scope().ManifestBytes(), &manifest)
				if err := os.WriteFile(filepath.Join(manifest.Root, f.Path), []byte("changed"), 0600); err != nil {
					t.Fatal(err)
				}
			case "stale":
				id.Generation++
			case "cancel":
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			case "language":
				f := restart.Scope().Files()[5]
				r := m.PrepareDocument(ctx, sessionruntime.DocumentRequest{SessionID: id.SessionID, Generation: id.Generation, URI: f.URI, LanguageID: "other", CaptureSupply: true})
				if r.Failure != "" {
					t.Fatal(r)
				}
			}
			a, b, err := newScopeRuntimeAdapter(ctx, restart, m, id)
			if err == nil || a != nil || b.adapter != nil || len(semanticMessages(child)) != 0 {
				t.Fatal("refusal", err)
			}
		})
	}
}
func TestScopeAdapterPrivateSupplyEquality(t *testing.T) {
	restart, m, id, _ := adapterFixture(t)
	f := restart.Scope().Files()[0]
	r := m.PrepareDocument(context.Background(), sessionruntime.DocumentRequest{SessionID: id.SessionID, Generation: id.Generation, URI: f.URI, LanguageID: "go", CaptureSupply: true})
	if !scopeAdapterSupplyMatches(r, f, id) {
		t.Fatal("valid supply")
	}
	for _, kind := range []string{"nil", "uri", "language", "version", "session", "generation", "content", "params"} {
		t.Run(kind, func(t *testing.T) {
			copy := r
			s := *r.Supply
			copy.Supply = &s
			switch kind {
			case "nil":
				copy.Supply = nil
			case "uri":
				s.URI = "file:///other"
			case "language":
				copy.LanguageID = "other"
			case "version":
				s.DocumentVersion++
			case "session":
				s.SessionID = "other"
			case "generation":
				s.Generation++
			case "content":
				s.Content = []byte("other")
			case "params":
				s.Params = json.RawMessage(`{}`)
			}
			if scopeAdapterSupplyMatches(copy, f, id) {
				t.Fatal("mismatch admitted")
			}
		})
	}
}
func TestScopeAdapterPrivateLeaseCancellationAndPoison(t *testing.T) {
	for _, kind := range []string{"lease", "cancel", "poison"} {
		t.Run(kind, func(t *testing.T) {
			restart, m, id, child := adapterFixture(t)
			var a *scopeRuntimeAdapter
			var manifest scopeManifest
			_ = json.Unmarshal(restart.Scope().ManifestBytes(), &manifest)
			err := m.WithReadySessionLease(context.Background(), sessionruntime.SessionLeaseRequest{SessionID: id.SessionID, Generation: id.Generation, Workspace: manifest.Root}, func(ctx context.Context) error {
				var err error
				a, _, err = newScopeRuntimeAdapter(ctx, restart, m, id)
				if err != nil {
					return err
				}
				if kind == "lease" {
					return nil
				}
				if kind == "cancel" {
					var cancel context.CancelFunc
					ctx, cancel = context.WithCancel(ctx)
					cancel()
					if err = a.Supply(ctx, a.files[0]); !errors.Is(err, context.Canceled) {
						t.Fatal(err)
					}
					return nil
				}
				child.mu.Lock()
				child.poison = true
				child.mu.Unlock()
				_, err = a.DocumentSymbols(ctx, lsp.DocumentSymbolParams{TextDocument: lsp.TextDocumentIdentifier{URI: a.files[0].URI}})
				if err == nil || !errors.Is(err, censusacquisition.ErrSessionPoisoned) {
					t.Fatal("poison classification lost", err)
				}
				n := len(child.snapshot())
				_, _ = a.DocumentSymbols(ctx, lsp.DocumentSymbolParams{TextDocument: lsp.TextDocumentIdentifier{URI: a.files[0].URI}})
				if len(child.snapshot()) != n {
					t.Fatal("retry")
				}
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			if kind == "lease" {
				n := len(child.snapshot())
				if err := a.Supply(context.Background(), a.files[0]); err == nil || len(child.snapshot()) != n {
					t.Fatal("retired lease")
				}
			}
		})
	}
}

// Native request path reuses the guard, not the default unguarded session client.
func TestScopeAdapterPrivateNative(t *testing.T) {
	restart, m, id, child := adapterFixture(t)
	a, _, err := newScopeRuntimeAdapter(context.Background(), restart, m, id)
	if err != nil {
		t.Fatal(err)
	}
	zero := uint32(0)
	request := acquisition.Request{Mode: acquisition.Incoming, Context: acquisition.AcquisitionContext{ID: "private", SessionID: id.SessionID, Generation: id.Generation, PositionEncoding: "utf-16"}, Root: acquisition.Target{ID: "root", Locator: acquisition.Locator{URI: a.files[0].URI, LanguageID: "go", Line: &zero, Character: &zero}, DownDepth: 1, UpDepth: 1}, Limits: acquisition.Limits{MaxNodes: 8, MaxRequests: 8, MaxEvidenceBytes: 1 << 20, MaxPathWork: 100, Timeout: time.Second, RequestTimeout: time.Second, MaxResponseBytes: 2 << 20, MaxMessages: 64}}
	result, err := acquisition.Acquire(context.Background(), a.nativeClient(), request)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Targets) != 1 || result.Targets[0].Resolution.Status != acquisition.Resolved {
		t.Fatalf("native %+v", result)
	}
	// Directional custody must refuse before any request reaches Manager.
	n := len(child.snapshot())
	directionalResult, e := acquisition.Acquire(directionalraw.NewReservation("private-fake").Context(context.Background()), a.nativeClient(), request)
	if e != nil {
		t.Fatal(e)
	}
	if directionalResult.Targets[0].Resolution.Status == acquisition.Resolved || len(child.snapshot()) != n {
		t.Fatal("fabricated directional lease")
	}
	request.Mode = acquisition.Slice
	if _, err = acquisition.Acquire(context.Background(), a.nativeClient(), request); err != nil {
		t.Fatal(err)
	}
	methods := map[string]bool{}
	for _, msg := range semanticMessages(child) {
		methods[msg.Method] = true
	}
	if !methods["callHierarchy/incomingCalls"] || !methods["callHierarchy/outgoingCalls"] {
		t.Fatal(methods)
	}
	if m.Census().Requests != 0 {
		t.Fatal("pending request leak")
	}
}
