//go:build darwin || linux

package censuslineagepilot_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"lsp-trace/internal/censusacquisition"
	pilot "lsp-trace/internal/censuslineagepilot"
	"lsp-trace/internal/lsp"
	"lsp-trace/internal/lspwire"
	"lsp-trace/internal/managedprocess"
	"lsp-trace/internal/publication"
	"lsp-trace/internal/runtimeprofile"
	"lsp-trace/sessionruntime"
)

// Explicit owned-only backend for legacy protocol fixtures whose CodeRoot is
// intentionally not a Git checkout. Real callers cannot select this backend.
func variableOwnedCodeInputs(ctx context.Context, root, head string) (variableCodeWitness, error) {
	if ctx == nil {
		return variableCodeWitness{}, errors.New("owned nil context")
	}
	if e := ctx.Err(); e != nil {
		return variableCodeWitness{}, e
	}
	return variableCodeWitness{Profile: "OWNED_FAKE_REPOSITORY_INPUTS_ONLY", CodeRoot: root, CodeHEAD: head, Module: "owned", InputsSHA256: pilot.Digest([]byte(root + "\n" + head)), Files: 1, Bytes: 1, Packages: 1, Limits: variableCodePolicy()}, nil
}
func variableOwnedRun(ctx context.Context, mode, path string, git variableGitRunner, factory variableManagerFactory, closeFile func(*os.File) error) (variableCallerReport, error) {
	return variableRunWithCode(ctx, mode, path, git, factory, closeFile, variableOwnedCodeInputs)
}

func variableOwnedConfig(t *testing.T, nom, con int) (variableCallerConfig, string, variableGitRunner) {
	t.Helper()
	home, e := os.UserHomeDir()
	if e != nil {
		t.Fatal(e)
	}
	base, e := os.MkdirTemp(home, ".owned-variable-v2-caller-")
	if e != nil {
		t.Fatal(e)
	}
	base, e = filepath.EvalSymlinks(base)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() {
		if e := os.RemoveAll(base); e != nil {
			t.Error(e)
		}
	})
	dir := func(name string) string {
		p := filepath.Join(base, name)
		if e := os.Mkdir(p, 0700); e != nil {
			t.Fatal(e)
		}
		return p
	}
	source := dir("source")
	code := dir("code")
	op := dir("operation")
	server := filepath.Join(base, "owned-server")
	body := []byte("owned fake executable pin, never executed\n")
	if e := os.WriteFile(server, body, 0700); e != nil {
		t.Fatal(e)
	}
	spec := pilot.ScopeSpecPrivateV2{Root: source, RevisionAssertion: strings.Repeat("b", 40), PartitionSize: 1, EffectivePolicy: pilot.ScopeEffectivePolicyPrivateV2{MaxNominationFiles: 8, MaxContextFiles: 8, MaxMembers: 16, FileBytes: 2 << 20, TotalBytes: 8 << 20, MaxPathDepth: 32, Discovery: censusacquisition.DiscoveryLimits{MaxFiles: 8, MaxSymbols: 256, MaxDepth: 64, MaxStringBytes: 1 << 20}, MaxTargets: 63, Planning: censusacquisition.PlanningConfig{DownDepth: 1, UpDepth: 1, MaxBatchTargets: 1}, MaxNodes: 128, MaxRequests: 256, MaxEvidenceBytes: 4 << 20, MaxPathWork: 100000, TimeoutMS: 30000, RequestTimeoutMS: 5000, MaxResponseBytes: 2 << 20, MaxMessages: 64}}
	for i := 0; i < nom+con; i++ {
		rel := fmt.Sprintf("owned%d.go", i)
		b := []byte(fmt.Sprintf("package owned\n\nfunc A%d() {}\n", i))
		role := "NOMINATION"
		if i >= nom {
			role = "CONTEXT_ONLY"
		}
		if e := os.WriteFile(filepath.Join(source, rel), b, 0600); e != nil {
			t.Fatal(e)
		}
		spec.Members = append(spec.Members, pilot.ScopeMemberPrivateV2{CanonicalRelativePath: rel, Role: role, LanguageID: "go", ExpectedSHA256: pilot.Digest(b), ByteLength: uint64(len(b))})
	}
	selector := runtimeprofile.Selector{TrustDomain: "test", Workspace: source, Profile: "go", EnvironmentReference: "owned"}
	v, e := runtimeprofile.Validate(selector)
	if e != nil {
		t.Fatal(e)
	}
	c := variableCallerConfig{Schema: variableCallerSchema, Purpose: variableCallerPurpose, CodeRoot: code, CodeHEAD: strings.Repeat("a", 40), Scope: spec, Session: censusacquisition.SessionIdentity{SessionID: runtimeprofile.Resolve(v).SessionKey().String(), Generation: 1}, Selector: selector, Server: server, ServerSHA256: pilot.Digest(body), Args: []string{}, Environment: []string{"OWNED_VARIABLE_V2=true"}, PrepLimits: prepLimitsV2(), PostLimits: postLimitsV2(), Seeds: []uint64{42, 43}, OperationTimeoutMS: 30000, ReadinessTimeoutMS: 1000, ShutdownTimeoutMS: 1000, ShutdownResponseTimeoutMS: 250, OperationDirectory: op, ProducerOutputs: [2]string{dir("producer0"), dir("producer1")}, PostOutputs: [2]string{dir("post0"), dir("post1")}, ProtectedRoots: []string{}, AttemptMarkerPath: filepath.Join(op, "attempt.json"), ReportPath: filepath.Join(op, "report.json")}
	path := filepath.Join(base, "config.json")
	variableSave(t, path, c)
	git := func(ctx context.Context, root string, args ...string) ([]byte, error) {
		if ctx == nil {
			return nil, errors.New("nil git context")
		}
		if e := ctx.Err(); e != nil {
			return nil, e
		}
		switch strings.Join(args, " ") {
		case "rev-parse --show-toplevel":
			return []byte(c.CodeRoot + "\n"), nil
		case "rev-parse HEAD":
			if root == c.CodeRoot {
				return []byte(c.CodeHEAD + "\n"), nil
			}
			return []byte(c.Scope.RevisionAssertion + "\n"), nil
		case "diff --cached --quiet", "ls-files --others --exclude-standard":
			return nil, nil
		}
		return nil, errors.New("unexpected owned git command")
	}
	return c, path, git
}
func variableSave(t *testing.T, path string, c variableCallerConfig) {
	t.Helper()
	b, e := json.Marshal(c)
	if e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(path, b, 0600); e != nil {
		t.Fatal(e)
	}
}
func variableClose(f *os.File) error { return f.Close() }
func variableEmpty(t *testing.T, c variableCallerConfig) {
	t.Helper()
	for _, p := range append([]string{c.OperationDirectory}, append(c.ProducerOutputs[:], c.PostOutputs[:]...)...) {
		entries, e := os.ReadDir(p)
		if e != nil || len(entries) != 0 {
			t.Fatalf("ASSERT_PREFLIGHT_READONLY: %s entries=%d err=%v", p, len(entries), e)
		}
	}
}

func TestVariableCallerV2ModeBoundary(t *testing.T) {
	for _, m := range []string{"", "bogus", "execute", "EXECUTE-ONCE"} {
		mode, _, e := variableEnvironment(func(k string) string {
			if k == "LSP_TRACE_PRIVATE_V2_CALLER" {
				return m
			}
			return "/no/config"
		})
		if m == "" {
			if e != nil || mode != "" {
				t.Fatal("default-off")
			}
		} else if e == nil {
			t.Fatal("invalid sentinel accepted")
		}
	}
	for _, m := range []string{"preflight", "execute-once"} {
		if _, _, e := variableEnvironment(func(k string) string {
			if k == "LSP_TRACE_PRIVATE_V2_CALLER" {
				return m
			}
			return ""
		}); e == nil {
			t.Fatal("missing config")
		}
	}
	if _, e := variableOwnedRun(nil, "preflight", "/no/config", variableGit, nil, nil); e == nil {
		t.Fatal("nil context")
	}
	starts := 0
	for _, m := range []string{"", "wrong"} {
		if _, e := variableOwnedRun(context.Background(), m, "/no/config", variableGit, func() (*sessionruntime.Manager, error) { starts++; return nil, nil }, variableClose); e == nil {
			t.Fatal("mode boundary")
		}
	}
	if starts != 0 {
		t.Fatal("mode started provider")
	}
}
func TestVariableCallerV2ReadonlyPreflight(t *testing.T) {
	c, path, git := variableOwnedConfig(t, 2, 1)
	starts := 0
	r, e := variableOwnedRun(context.Background(), "preflight", path, git, func() (*sessionruntime.Manager, error) { starts++; return nil, errors.New("must not start") }, variableClose)
	if e != nil || r.Phase != "READONLY_PREFLIGHT_COMPLETE" || starts != 0 {
		t.Fatalf("ASSERT_PREFLIGHT_READONLY: %s starts=%d err=%v", r.Phase, starts, e)
	}
	variableEmpty(t, c)
	t.Log("ASSERT_PREFLIGHT_READONLY: PASS; owned starts=0 writes=0")
}
func TestVariableCallerV2StrictConfig(t *testing.T) {
	c, path, git := variableOwnedConfig(t, 1, 0)
	original, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	cases := map[string][]byte{"duplicate": append([]byte(`{"schema":"wrong",`), original[1:]...), "case": bytes.Replace(original, []byte(`"schema"`), []byte(`"Schema"`), 1), "unknown": append([]byte(`{"extra":1,`), original[1:]...), "trailing": append(bytes.Clone(original), []byte(`{}`)...), "oversized": bytes.Repeat([]byte(" "), variableCallerConfigCap+1), "malformed": []byte(`{"schema":`)}
	for name, b := range cases {
		t.Run(name, func(t *testing.T) {
			if e := os.WriteFile(path, b, 0600); e != nil {
				t.Fatal(e)
			}
			if _, e := variablePreflightRead(context.Background(), path, git); e == nil {
				t.Fatal("strict config accepted")
			}
			variableEmpty(t, c)
		})
	}
}
func TestVariableCallerV2NegativePreflight(t *testing.T) {
	for _, kind := range []string{"purpose", "head", "serverhash", "sourcehash", "sourcebytes", "language", "zero-limit", "timeout", "seed-duplicate", "args", "env-duplicate", "alias", "output-temp", "output-mode", "output-source", "root-ancestry", "marker-collision", "report-collision", "existing-marker", "existing-report", "dirty"} {
		t.Run(kind, func(t *testing.T) {
			c, path, git := variableOwnedConfig(t, 2, 1)
			switch kind {
			case "purpose":
				c.Purpose = "ORDINARY"
			case "head":
				c.CodeHEAD = strings.Repeat("f", 40)
			case "serverhash":
				c.ServerSHA256 = pilot.Digest([]byte("wrong"))
			case "sourcehash":
				c.Scope.Members[0].ExpectedSHA256 = pilot.Digest([]byte("wrong"))
			case "sourcebytes":
				c.Scope.Members[0].ByteLength++
			case "language":
				c.Scope.Members[0].LanguageID = "typescript"
			case "zero-limit":
				c.PrepLimits.MaxTargets = 0
			case "timeout":
				c.OperationTimeoutMS = 240001
			case "seed-duplicate":
				c.Seeds = []uint64{1, 1}
			case "args":
				c.Args = []string{"-remote=auto"}
			case "env-duplicate":
				c.Environment = []string{"A=1", "A=2"}
			case "alias":
				link := filepath.Join(filepath.Dir(path), "server-link")
				if e := os.Symlink(c.Server, link); e != nil {
					t.Fatal(e)
				}
				c.Server = link
			case "output-temp":
				c.PostOutputs[0] = t.TempDir()
			case "output-mode":
				if e := os.Chmod(c.PostOutputs[0], 0755); e != nil {
					t.Fatal(e)
				}
			case "output-source":
				c.PostOutputs[0] = c.Scope.Root
			case "root-ancestry":
				c.PostOutputs[0] = filepath.Dir(c.ProducerOutputs[0])
			case "marker-collision":
				c.AttemptMarkerPath = c.ReportPath
			case "report-collision":
				c.ReportPath = path
			case "existing-marker":
				if e := os.WriteFile(c.AttemptMarkerPath, []byte("spent failed"), 0600); e != nil {
					t.Fatal(e)
				}
			case "existing-report":
				if e := os.WriteFile(c.ReportPath, []byte("prior"), 0600); e != nil {
					t.Fatal(e)
				}
			case "dirty":
				old := git
				git = func(ctx context.Context, r string, a ...string) ([]byte, error) {
					if strings.Join(a, " ") == "diff --cached --quiet" {
						return nil, errors.New("owned dirty checkout")
					}
					return old(ctx, r, a...)
				}
			}
			variableSave(t, path, c)
			starts := 0
			_, e := variableOwnedRun(context.Background(), "execute-once", path, git, func() (*sessionruntime.Manager, error) { starts++; return nil, errors.New("forbidden start") }, variableClose)
			if e == nil || starts != 0 {
				t.Fatalf("negative accepted/start: %s starts=%d err=%v", kind, starts, e)
			}
		})
	}
}

type variableOwnedChild struct {
	input                                         *io.PipeReader
	stdin                                         *io.PipeWriter
	stdout                                        *io.PipeReader
	output                                        *io.PipeWriter
	once                                          sync.Once
	done                                          chan struct{}
	traffic                                       atomic.Int64
	kind                                          string
	closed                                        atomic.Bool
	writesAfterClose, shutdownRequests, teardowns atomic.Int64
	teardownEntered                               chan struct{}
	teardownRelease                               chan struct{}
	incomingCount                                 atomic.Int64
	fixtureMu                                     sync.Mutex
	fixtureEvents                                 []variableFixtureEvent
	fixtureReaders, fixtureMaxReaders             int
	fixtureReadBytes                              int64
	fixtureFinalID, fixtureShutdownID             string
	fixtureCoalescedBytes                         int
}

type variableFixtureEvent struct {
	Kind, ID      string
	ActiveReaders int
	Bytes         int64
}

type variableFixtureStdout struct{ child *variableOwnedChild }

func (r variableFixtureStdout) Read(p []byte) (int, error) {
	c := r.child
	c.fixtureMu.Lock()
	c.fixtureReaders++
	if c.fixtureReaders > c.fixtureMaxReaders {
		c.fixtureMaxReaders = c.fixtureReaders
	}
	c.fixtureEvents = append(c.fixtureEvents, variableFixtureEvent{Kind: "READ_BEGIN", ActiveReaders: c.fixtureReaders})
	c.fixtureMu.Unlock()
	n, e := c.stdout.Read(p)
	c.fixtureMu.Lock()
	c.fixtureReadBytes += int64(n)
	c.fixtureReaders--
	c.fixtureEvents = append(c.fixtureEvents, variableFixtureEvent{Kind: "READ_END", ActiveReaders: c.fixtureReaders, Bytes: int64(n)})
	c.fixtureMu.Unlock()
	return n, e
}
func (r variableFixtureStdout) Close() error { return r.child.stdout.Close() }

type variableOwnedStdin struct{ child *variableOwnedChild }

func (w variableOwnedStdin) Write(b []byte) (int, error) {
	if w.child.closed.Load() {
		w.child.writesAfterClose.Add(1)
	}
	if (w.child.kind == "shutdown-write" && bytes.Contains(b, []byte(`"method":"shutdown"`))) || (w.child.kind == "exit-write" && bytes.Contains(b, []byte(`"method":"exit"`))) {
		return 0, io.ErrClosedPipe
	}
	return w.child.stdin.Write(b)
}
func (w variableOwnedStdin) Close() error           { return w.child.stdin.Close() }
func (c *variableOwnedChild) Stdin() io.WriteCloser { return variableOwnedStdin{c} }
func (c *variableOwnedChild) Stdout() io.ReadCloser {
	if strings.HasPrefix(c.kind, "integrated-") {
		return variableFixtureStdout{c}
	}
	return c.stdout
}
func (c *variableOwnedChild) Close() managedprocess.ResourceObservation {
	c.once.Do(func() { c.closed.Store(true); c.input.Close(); c.stdin.Close(); c.stdout.Close(); c.output.Close() })
	if c.kind == "resource-unknown" {
		return managedprocess.ResourceObservation{}
	}
	if c.kind == "shutdown-close" {
		return managedprocess.ResourceObservation{Kind: managedprocess.ResourcesCloseFailed, Err: errors.New("owned shutdown close failure")}
	}
	return managedprocess.ResourceObservation{Kind: managedprocess.ResourcesClosed}
}
func (c *variableOwnedChild) Teardown(context.Context) managedprocess.TeardownObservation {
	c.teardowns.Add(1)
	if c.teardownEntered != nil {
		close(c.teardownEntered)
		<-c.teardownRelease
	}
	c.Close()
	death, reap := managedprocess.DeathExited, managedprocess.ReapComplete
	switch c.kind {
	case "exit-write":
		death = managedprocess.DeathSignaled
	case "death-unknown":
		death = managedprocess.DeathUnknown
	case "reap-failed":
		reap = managedprocess.ReapFailed
	case "reap-unknown":
		reap = ""
	}
	return managedprocess.TeardownObservation{Death: managedprocess.DeathObservation{Kind: death, Reap: managedprocess.ReapObservation{Kind: reap}}}
}
func (c *variableOwnedChild) serve() {
	defer close(c.done)
	r := lspwire.NewReader(c.input, lspwire.DefaultLimits())
	w := lspwire.NewWriter(c.output, lspwire.DefaultLimits())
	for {
		m, e := r.Read()
		if e != nil {
			return
		}
		c.traffic.Add(1)
		if m.Method == "initialized" || strings.HasPrefix(m.Method, "textDocument/did") {
			continue
		}
		var value any = []any{}
		switch m.Method {
		case "exit":
			return
		case "shutdown":
			c.shutdownRequests.Add(1)
			if strings.HasPrefix(c.kind, "integrated-") {
				c.fixtureMu.Lock()
				c.fixtureShutdownID = string(m.ID)
				c.fixtureEvents = append(c.fixtureEvents, variableFixtureEvent{Kind: "OWN_SHUTDOWN_READ", ID: string(m.ID), ActiveReaders: c.fixtureReaders})
				c.fixtureMu.Unlock()
			}
			switch c.kind {
			case "shutdown-notifications":
				var coalesced bytes.Buffer
				cw := lspwire.NewWriter(&coalesced, lspwire.DefaultLimits())
				for i := 0; i < 3; i++ {
					_ = cw.Write(lspwire.Message{JSONRPC: lspwire.Version, Method: "withheld", Params: json.RawMessage(`{}`)})
				}
				_ = cw.Write(lspwire.Message{JSONRPC: lspwire.Version, ID: m.ID, Result: json.RawMessage(`null`)})
				if _, e := c.output.Write(coalesced.Bytes()); e != nil {
					return
				}
				continue
			case "shutdown-wrong-id":
				m.ID = json.RawMessage(`999999`)
			case "shutdown-error":
				if w.Write(lspwire.Message{JSONRPC: lspwire.Version, ID: m.ID, Error: &lspwire.RPCError{Code: -32603, Message: "withheld"}}) != nil {
					return
				}
				continue
			case "shutdown-request":
				if w.Write(lspwire.Message{JSONRPC: lspwire.Version, ID: json.RawMessage(`77`), Method: "withheld", Params: json.RawMessage(`{}`)}) != nil {
					return
				}
				continue
			case "shutdown-malformed":
				_, _ = io.WriteString(c.output, "Content-Length: nope\r\n\r\n")
				continue
			case "shutdown-timeout":
				<-c.doneNever()
				return
			}
		case "initialize":
			if c.kind == "blocked" {
				<-c.doneNever()
				return
			}
			value = map[string]any{"capabilities": map[string]any{"positionEncoding": "utf-16", "documentSymbolProvider": true, "callHierarchyProvider": true}}
		case "textDocument/documentSymbol":
			value = []lsp.DocumentSymbol{{Name: "A0", Kind: 12, Range: lsp.Range{Start: lsp.Position{Line: 2}, End: lsp.Position{Line: 2, Character: 12}}, SelectionRange: lsp.Range{Start: lsp.Position{Line: 2, Character: 5}, End: lsp.Position{Line: 2, Character: 7}}}}
			if c.kind == "malformed" {
				value = "invalid symbols"
			}
		case "textDocument/prepareCallHierarchy":
			var p lsp.PrepareCallHierarchyParams
			_ = json.Unmarshal(m.Params, &p)
			value = []lsp.CallHierarchyItem{{Name: "A0", Kind: 12, URI: p.TextDocument.URI, Range: lsp.Range{Start: lsp.Position{Line: 2}, End: lsp.Position{Line: 2, Character: 12}}, SelectionRange: lsp.Range{Start: lsp.Position{Line: 2, Character: 5}, End: lsp.Position{Line: 2, Character: 7}}}}
		case "callHierarchy/incomingCalls":
			ordinal := c.incomingCount.Add(1)
			if c.kind == "capture-second-failed" && ordinal == 2 {
				if w.Write(lspwire.Message{JSONRPC: lspwire.Version, ID: m.ID, Error: &lspwire.RPCError{Code: -32603, Message: "owned failure"}}) != nil {
					return
				}
				continue
			}
			value = []lsp.CallHierarchyIncomingCall{}
		case "callHierarchy/outgoingCalls":
			value = []lsp.CallHierarchyOutgoingCall{}
		}
		raw, _ := json.Marshal(value)
		if strings.HasPrefix(c.kind, "integrated-") && m.Method == "callHierarchy/incomingCalls" {
			var coalesced bytes.Buffer
			cw := lspwire.NewWriter(&coalesced, lspwire.DefaultLimits())
			_ = cw.Write(lspwire.Message{JSONRPC: lspwire.Version, Method: "owned-prefix", Params: json.RawMessage(`{}`)})
			_ = cw.Write(lspwire.Message{JSONRPC: lspwire.Version, ID: m.ID, Result: raw})
			if c.kind == "integrated-request-suffix" {
				_ = cw.Write(lspwire.Message{JSONRPC: lspwire.Version, ID: json.RawMessage(`77001`), Method: "owned-unsupported", Params: json.RawMessage(`{}`)})
			} else {
				_ = cw.Write(lspwire.Message{JSONRPC: lspwire.Version, Method: "owned-suffix", Params: json.RawMessage(`{}`)})
			}
			c.fixtureMu.Lock()
			c.fixtureFinalID, c.fixtureCoalescedBytes = string(m.ID), coalesced.Len()
			c.fixtureEvents = append(c.fixtureEvents, variableFixtureEvent{Kind: "FINAL_RESPONSE_COALESCED", ID: string(m.ID), Bytes: int64(coalesced.Len())})
			c.fixtureMu.Unlock()
			if _, e := c.output.Write(coalesced.Bytes()); e != nil {
				return
			}
			continue
		}
		if strings.HasPrefix(c.kind, "integrated-") && m.Method == "shutdown" {
			c.fixtureMu.Lock()
			c.fixtureEvents = append(c.fixtureEvents, variableFixtureEvent{Kind: "ACK_AFTER_OWN_SHUTDOWN", ID: string(m.ID)})
			c.fixtureMu.Unlock()
		}
		if e := w.Write(lspwire.Message{JSONRPC: lspwire.Version, ID: m.ID, Result: raw}); e != nil {
			return
		}
	}
}

// Blocking readiness is canceled by closing its input; this is owned pipe IO,
// never a live provider or a sleep/restart loop.
func (c *variableOwnedChild) doneNever() <-chan struct{} {
	ch := make(chan struct{})
	go func() { var b [1]byte; c.input.Read(b[:]); close(ch) }()
	return ch
}

type variableOwnedStarter struct {
	child  *variableOwnedChild
	starts *atomic.Int64
	fail   bool
	spec   *managedprocess.Spec
}

func (s variableOwnedStarter) Start(_ context.Context, p managedprocess.Spec) (sessionruntime.Child, managedprocess.StartObservation) {
	s.starts.Add(1)
	*s.spec = p
	if s.fail {
		return nil, managedprocess.StartObservation{Kind: managedprocess.StartFailed, Reason: "owned startup failure"}
	}
	return s.child, managedprocess.StartObservation{Kind: managedprocess.StartStarted}
}
func variableOwnedFactory(t *testing.T, kind string) (variableManagerFactory, *atomic.Int64, *variableOwnedChild, *managedprocess.Spec) {
	t.Helper()
	in, stdin := io.Pipe()
	stdout, out := io.Pipe()
	child := &variableOwnedChild{input: in, stdin: stdin, stdout: stdout, output: out, done: make(chan struct{}), kind: kind}
	go child.serve()
	starts := &atomic.Int64{}
	spec := &managedprocess.Spec{}
	t.Cleanup(func() {
		child.Close()
		select {
		case <-child.done:
		case <-time.After(time.Second):
			t.Error("owned child did not close")
		}
	})
	return func() (*sessionruntime.Manager, error) {
		return sessionruntime.New(sessionruntime.Config{Starter: variableOwnedStarter{child, starts, kind == "startup-fail", spec}, Limits: variableRuntimeLimits()})
	}, starts, child, spec
}
func TestVariableCallerV2OwnedFullPath(t *testing.T) {
	for _, counts := range [][2]int{{1, 0}, {2, 1}} {
		t.Run(fmt.Sprint(counts), func(t *testing.T) {
			c, path, git := variableOwnedConfig(t, counts[0], counts[1])
			factory, starts, child, spec := variableOwnedFactory(t, "")
			report, e := variableOwnedRun(context.Background(), "execute-once", path, git, factory, variableClose)
			if e != nil {
				t.Fatalf("owned full path: %v phase=%s errors=%v", e, report.Phase, report.Errors)
			}
			if starts.Load() != 1 || report.Phase != "PENDING_INTERPRETATION_PRIVATE_V2" || report.Outcome == nil || report.Outcome.Status != "VERIFIED_PAIR_PRIVATE_WITH_LIMITS" || report.Ledger == nil || len(report.Ledger.Leads) != 0 || report.Accepted || report.Authority != 0 {
				t.Fatal("owned pending report", report.Phase, starts.Load())
			}
			if spec.Path != c.Server || spec.Dir != c.Scope.Root || len(spec.Args) != 0 || len(spec.Env) != 1 || spec.Env[0] != c.Environment[0] {
				t.Fatal("exact explicit startup")
			}
			if len(report.Coverage) != counts[0] {
				t.Fatal("all actual batches not retained", len(report.Coverage), counts[0])
			}
			total := 0
			for _, coverage := range report.Coverage {
				total += coverage.Requested
				if coverage.Requested != coverage.Prepared+coverage.SourceGap+coverage.ResourceGap {
					t.Fatal("coverage conservation")
				}
			}
			if total != counts[0] {
				t.Fatal("all actual targets not retained", total)
			}
			if e := variableTerminalCleanup(report.Cleanup); e != nil || report.Cleanup.Status != "JOINED_STOP_VERIFIED" || child.shutdownRequests.Load() != 1 || child.teardowns.Load() != 1 || child.writesAfterClose.Load() != 0 {
				t.Fatal("exact joined cleanup", e, report.Cleanup)
			}
			before := child.traffic.Load()
			if before < 5 {
				t.Fatal("actual API traffic absent")
			}
			marker, e := os.ReadFile(c.AttemptMarkerPath)
			if e != nil || pilot.Digest(marker) != report.MarkerSHA256 {
				t.Fatal("marker lineage", e)
			}
			raw, e := os.ReadFile(c.ReportPath)
			if e != nil {
				t.Fatal(e)
			}
			expected, _ := json.Marshal(report)
			if !bytes.Equal(raw, expected) {
				t.Fatal("ASSERT_REPORT_EXACT_FINAL_BYTES")
			}
			var final variableCallerReport
			if json.Unmarshal(raw, &final) != nil || final.ConfigSHA256 == "" || len(final.InputSourceWitness) == 0 || len(final.Qualification) == 0 || final.PreparationSHA256 == "" || final.ProducerCopies[0].Receipt == nil || final.ProducerCopies[1].Receipt == nil {
				t.Fatal("report lineage incomplete")
			}
			for _, p := range c.PostOutputs {
				r, e := publication.OpenContentRetentionRoot(context.Background(), p)
				if e != nil {
					t.Fatal(e)
				}
				b, e := r.ReadPostPreparationPrivateV2(context.Background(), report.Outcome.EnvelopeSHA256, int64(report.Outcome.ByteLength))
				ce := r.Close()
				if e != nil || ce != nil || !bytes.Equal(b, report.Outcome.EnvelopeBytes()) {
					t.Fatal("physical final copy", e, ce)
				}
			}
			if _, e := variableOwnedRun(context.Background(), "execute-once", path, git, factory, variableClose); e == nil || starts.Load() != 1 || child.traffic.Load() != before {
				t.Fatal("spent attempt replayed")
			}
			t.Logf("OWNED_COMPLETE_PENDING: nom=%d ctx=%d starts=1 batches=%d; no provider after shutdown", counts[0], counts[1], len(report.Coverage))
		})
	}
}
func TestVariableCallerV2SpentFailures(t *testing.T) {
	for _, kind := range []string{"startup-fail", "blocked", "malformed", "source-gap", "resource-gap", "report-close", "shutdown-close"} {
		t.Run(kind, func(t *testing.T) {
			c, path, git := variableOwnedConfig(t, 1, 0)
			if kind == "blocked" {
				c.ReadinessTimeoutMS = 30
			}
			if kind == "resource-gap" {
				c.PostLimits.MaxInputBytes = 1
			}
			if kind == "source-gap" {
				c.PrepLimits.MaxSourceBytes = 1
			}
			variableSave(t, path, c)
			factory, starts, _, _ := variableOwnedFactory(t, kind)
			closeFile := variableClose
			if kind == "report-close" {
				closeFile = func(f *os.File) error { return errors.Join(f.Close(), errors.New("owned report close failure")) }
			}
			report, e := variableOwnedRun(context.Background(), "execute-once", path, git, factory, closeFile)
			if kind == "source-gap" {
				if e != nil {
					t.Fatal(e)
				}
			} else if e == nil {
				t.Fatal("owned failure not propagated", report.Phase)
			}
			if starts.Load() != 1 {
				t.Fatal("single startup count", starts.Load())
			}
			if _, e := os.Stat(c.AttemptMarkerPath); e != nil {
				t.Fatal("attempt not spent", e)
			}
			if _, e := os.Stat(c.ReportPath); e != nil {
				t.Fatal("failure report absent", e)
			}
			if _, e := variableOwnedRun(context.Background(), "execute-once", path, git, factory, variableClose); e == nil || starts.Load() != 1 {
				t.Fatal("failed attempt replayed")
			}
		})
	}
}
