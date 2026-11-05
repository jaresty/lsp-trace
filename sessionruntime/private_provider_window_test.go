package sessionruntime

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
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
	"lsp-trace/internal/runtimeprofile"
)

type windowPipeStarter struct {
	child   *windowPipeChild
	starts  int
	delay   chan struct{}
	entered chan struct{}
}

func (*windowPipeStarter) privateProviderWindowControlledBackend() {}
func (s *windowPipeStarter) Start(context.Context, managedprocess.Spec) (Child, managedprocess.StartObservation) {
	s.starts++
	if s.entered != nil {
		close(s.entered)
	}
	if s.delay != nil {
		<-s.delay
	}
	return s.child, managedprocess.StartObservation{Kind: managedprocess.StartStarted}
}

type windowPipeChild struct {
	input                        *io.PipeWriter
	serverInput                  *io.PipeReader
	output                       *io.PipeReader
	serverOutput                 *io.PipeWriter
	done                         chan struct{}
	closed                       chan struct{}
	cleanupEntered               chan struct{}
	closeOnce                    sync.Once
	stall                        string
	release                      chan struct{}
	entered                      chan struct{}
	incomingFrame, outgoingFrame []byte
	uri                          string
	notifications                int
	serverRequest                bool
	oversized                    bool
	closeFailure                 bool
	capabilityUnavailable        bool
	emptyResults                 bool
	readJoinOnce                 sync.Once
	beforeWrite                  func(int)
}

func newWindowPipeChild(uri, stall string) *windowPipeChild {
	inR, inW := io.Pipe()
	outR, outW := io.Pipe()
	c := &windowPipeChild{input: inW, serverInput: inR, output: outR, serverOutput: outW, done: make(chan struct{}), closed: make(chan struct{}), cleanupEntered: make(chan struct{}), stall: stall, release: make(chan struct{}), entered: make(chan struct{}), uri: uri}
	return c
}
func (c *windowPipeChild) Stdin() io.WriteCloser { return windowBeforeWriter{c.input, c.beforeWrite} }
func (c *windowPipeChild) Stdout() io.ReadCloser {
	if c.stall == "reader-join" {
		return windowReadJoin{c}
	}
	return c.output
}

type windowBeforeWriter struct {
	io.WriteCloser
	before func(int)
}

func (w windowBeforeWriter) Write(p []byte) (int, error) {
	if w.before != nil {
		w.before(len(p))
	}
	return w.WriteCloser.Write(p)
}

type windowReadJoin struct{ c *windowPipeChild }

func (r windowReadJoin) Read(p []byte) (int, error) {
	n, e := r.c.output.Read(p)
	if n > 0 {
		r.c.readJoinOnce.Do(func() { close(r.c.entered) })
		<-r.c.release
	}
	return n, e
}
func (r windowReadJoin) Close() error { return r.c.output.Close() }
func (c *windowPipeChild) Close() managedprocess.ResourceObservation {
	if c.stall == "close" {
		select {
		case <-c.cleanupEntered:
		default:
			close(c.cleanupEntered)
		}
		select {
		case <-c.entered:
		default:
			close(c.entered)
		}
		<-c.release
	}
	c.closeOnce.Do(func() {
		close(c.closed)
		_ = c.input.Close()
		_ = c.output.Close()
		_ = c.serverInput.Close()
		_ = c.serverOutput.Close()
	})
	<-c.done
	if c.closeFailure {
		return managedprocess.ResourceObservation{Kind: managedprocess.ResourcesCloseFailed}
	}
	return managedprocess.ResourceObservation{Kind: managedprocess.ResourcesClosed}
}
func (c *windowPipeChild) Teardown(context.Context) managedprocess.TeardownObservation {
	if c.stall == "teardown" {
		select {
		case <-c.cleanupEntered:
		default:
			close(c.cleanupEntered)
		}
		select {
		case <-c.entered:
		default:
			close(c.entered)
		}
		<-c.release
	}
	_ = c.Close()
	return managedprocess.TeardownObservation{Death: managedprocess.DeathObservation{Kind: managedprocess.DeathExited, Reap: managedprocess.ReapObservation{Kind: managedprocess.ReapComplete}}}
}
func (c *windowPipeChild) serve() {
	defer close(c.done)
	defer c.serverOutput.Close()
	defer c.serverInput.Close()
	if c.stall == "initialize-write" {
		close(c.entered)
		<-c.release
		return
	}
	reader := lspwire.NewReader(c.serverInput, lspwire.DefaultLimits())
	item := fmt.Sprintf(`{"name":"f","kind":12,"uri":%q,"range":{"start":{"line":0,"character":0},"end":{"line":0,"character":1}},"selectionRange":{"start":{"line":0,"character":0},"end":{"line":0,"character":1}},"data":{"ordinal":0}}`, c.uri)
	for {
		msg, err := reader.Read()
		if err != nil {
			return
		}
		if msg.Method == "exit" {
			return
		}
		if len(msg.ID) == 0 {
			continue
		}
		if c.stall == msg.Method || (c.stall == "initialize-read" || c.stall == "close" || c.stall == "teardown") && msg.Method == "initialize" {
			close(c.entered)
			if c.stall == "teardown" {
				<-c.closed
			} else {
				<-c.release
			}
			return
		}
		raw := `[]`
		switch msg.Method {
		case "initialize":
			raw = `{"capabilities":{"documentSymbolProvider":true,"referencesProvider":true,"definitionProvider":true,"callHierarchyProvider":true}}`
		case "textDocument/documentSymbol":
			raw = `[{"name":"f","kind":12,"range":{"start":{"line":0,"character":0},"end":{"line":0,"character":1}},"selectionRange":{"start":{"line":0,"character":0},"end":{"line":0,"character":1}}}]`
		case "textDocument/references", "textDocument/definition":
			raw = fmt.Sprintf(`[{"uri":%q,"range":{"start":{"line":0,"character":0},"end":{"line":0,"character":1}}}]`, c.uri)
		case "textDocument/prepareCallHierarchy":
			raw = `[` + item + `]`
		case "callHierarchy/incomingCalls":
			raw = `[{"from":` + item + `,"fromRanges":[]},{"from":` + item + `,"fromRanges":[]}]`
		case "callHierarchy/outgoingCalls":
			raw = `[{"to":` + item + `,"fromRanges":[]},{"to":` + item + `,"fromRanges":[]}]`
		case "shutdown":
			raw = `null`
		}
		if c.capabilityUnavailable && msg.Method == "initialize" {
			raw = `{"capabilities":{"documentSymbolProvider":true,"definitionProvider":false,"referencesProvider":false,"callHierarchyProvider":false}}`
		}
		if c.emptyResults && msg.Method != "initialize" && msg.Method != "shutdown" {
			raw = `[]`
		}
		writer := lspwire.NewWriter(c.serverOutput, lspwire.DefaultLimits())
		if msg.Method == "textDocument/documentSymbol" {
			if c.oversized {
				_, _ = io.WriteString(c.serverOutput, "Content-Length: 131073\r\n\r\n")
				return
			}
			for i := 0; i < c.notifications; i++ {
				if writer.Write(lspwire.Message{JSONRPC: lspwire.Version, Method: "$/progress", Params: json.RawMessage(`{}`)}) != nil {
					return
				}
			}
			if c.serverRequest {
				_ = writer.Write(lspwire.Message{JSONRPC: lspwire.Version, ID: json.RawMessage(`"server"`), Method: "workspace/configuration", Params: json.RawMessage(`{}`)})
				return
			}
		}
		var capture bytes.Buffer
		writer = lspwire.NewWriter(io.MultiWriter(c.serverOutput, &capture), lspwire.DefaultLimits())
		if writer.Write(lspwire.Message{JSONRPC: lspwire.Version, ID: msg.ID, Result: json.RawMessage(raw)}) != nil {
			return
		}
		if msg.Method == "callHierarchy/incomingCalls" {
			c.incomingFrame = append([]byte(nil), capture.Bytes()...)
		}
		if msg.Method == "callHierarchy/outgoingCalls" {
			c.outgoingFrame = append([]byte(nil), capture.Bytes()...)
		}
		if c.stall == "shutdown-write" && msg.Method == "textDocument/definition" {
			close(c.entered)
			<-c.release
			return
		}
	}
}
func windowTestSetup(t *testing.T, plan PrivateProviderWindowPlan, stall string) (*Manager, StartRequest, context.Context, *windowPipeStarter) {
	t.Helper()
	root, rootErr := filepath.EvalSymlinks(t.TempDir())
	if rootErr != nil {
		t.Fatal(rootErr)
	}
	path := filepath.Join(root, "a.go")
	if err := os.WriteFile(path, []byte("package p\nfunc f() {}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	uri := (&url.URL{Scheme: "file", Path: path}).String()
	child := newWindowPipeChild(uri, stall)
	starter := &windowPipeStarter{child: child}
	m, err := New(Config{Limits: Limits{MaxSessions: 2, MaxRequests: 4, MaxChildren: 16, MaxCancels: 8, MaxTombstones: 16, MaxObservations: 64, MaxOperations: 8}, Starter: starter})
	if err != nil {
		t.Fatal(err)
	}
	selector, err := runtimeprofile.Validate(runtimeprofile.Selector{TrustDomain: "test", Workspace: root, Profile: "go", EnvironmentReference: "local"})
	if err != nil {
		t.Fatal(err)
	}
	start := StartRequest{Profile: runtimeprofile.Resolve(selector), Process: managedprocess.Spec{Path: "controlled-io.Pipe", Dir: root}, LanguageID: "go", Deadline: time.Now().Add(5 * time.Second)}
	envelope, err := m.PrivateProviderWindowEnvelope(start, plan)
	if err != nil {
		t.Fatal(err)
	}
	ctx, err := m.PrivateProviderWindowContext(context.Background(), envelope, start.Deadline)
	if err != nil {
		t.Fatal(err)
	}
	go child.serve()
	t.Cleanup(func() {
		select {
		case <-child.release:
		default:
			close(child.release)
		}
		_ = child.input.Close()
		_ = child.output.Close()
		_ = child.serverInput.Close()
		_ = child.serverOutput.Close()
		<-child.done
		if starter.delay != nil {
			select {
			case <-starter.delay:
			default:
				close(starter.delay)
			}
		}
		w := privateProviderWindowFrom(ctx)
		w.cancel()
		_, _ = m.FinishPrivateProviderWindow(ctx)
		select {
		case <-w.finishDone:
		case <-time.After(2 * time.Second):
			t.Error("controlled window cleanup did not join")
		}
	})
	return m, start, ctx, starter
}
func windowTestReady(t *testing.T, m *Manager, start StartRequest, ctx context.Context) StartResult {
	t.Helper()
	got := m.Start(ctx, start)
	if got.Failure != "" {
		t.Fatalf("start: %+v", got)
	}
	pending := m.BeginReadiness(ctx, got.SessionID, got.Generation, start.Deadline)
	ready, ok := m.WaitReadiness(ctx, pending.ID)
	if !ok || ready.State != ReadinessReady {
		t.Fatalf("readiness: %+v", ready)
	}
	return got
}
func windowTestQueries(t *testing.T, m *Manager, start StartRequest, ctx context.Context, child *windowPipeChild, plan PrivateProviderWindowPlan) DirectionalCallHierarchyResultPrivate {
	t.Helper()
	w := privateProviderWindowFrom(ctx)
	started := w.start
	supplied := m.PrepareDocument(ctx, DocumentRequest{SessionID: started.SessionID, Generation: started.Generation, URI: child.uri, LanguageID: "go"})
	if supplied.Failure != "" {
		t.Fatalf("supply: %+v", supplied)
	}
	methods := []string{"textDocument/documentSymbol"}
	if plan.References {
		methods = append(methods, "textDocument/references")
	}
	if plan.Definitions {
		methods = append(methods, "textDocument/definition")
	}
	for _, method := range methods {
		params := fmt.Sprintf(`{"textDocument":{"uri":%q},"position":{"line":0,"character":0},"context":{"includeDeclaration":true}}`, child.uri)
		got := m.RoundTrip(ctx, RoundTripRequest{SessionID: started.SessionID, Generation: started.Generation, Method: method, Params: json.RawMessage(params), Deadline: start.Deadline, MaxMessages: 4, MaxBytes: 128 << 10})
		if got.Failure != "" || got.ServerError != nil {
			t.Fatalf("query %s: %s %+v", method, got.Failure, got.ServerError)
		}
	}
	if !plan.Calls {
		return DirectionalCallHierarchyResultPrivate{}
	}
	line, ch := uint32(0), uint32(0)
	native, err := m.ExecuteCallHierarchyDirectionsPrivate(ctx, DirectionalCallHierarchyRequestPrivate{SessionID: started.SessionID, Generation: started.Generation, URI: child.uri, LanguageID: "go", Line: &line, Character: &ch, Deadline: start.Deadline, MaxMessages: 4, MaxBytes: 128 << 10})
	if err != nil {
		t.Fatalf("native: %v %+v", err, native)
	}
	return native
}
func TestPrivateProviderWindowAllSevenModesJoined(t *testing.T) {
	for mask := 1; mask < 8; mask++ {
		t.Run(fmt.Sprint(mask), func(t *testing.T) {
			plan := PrivateProviderWindowPlan{Calls: mask&1 != 0, References: mask&2 != 0, Definitions: mask&4 != 0}
			m, start, ctx, starter := windowTestSetup(t, plan, "")
			windowTestReady(t, m, start, ctx)
			native := windowTestQueries(t, m, start, ctx, starter.child, plan)
			got, err := m.FinishPrivateProviderWindow(ctx)
			if err != nil || !got.Completed {
				t.Fatalf("ASSERT_WINDOW_JOINED: completion=%+v err=%v census=%+v", got, err, m.Census())
			}
			if got.DeliveredReadBytes > got.ReservedReadBytes || got.DeliveredWriteBytes > got.ReservedWriteBytes {
				t.Fatal("ASSERT_WINDOW_PREDEBIT")
			}
			if m.Census().Workers != 0 || m.Census().Sessions != 0 {
				t.Fatalf("ASSERT_WINDOW_OWNER_RELEASE: %+v", m.Census())
			}
			if plan.Calls {
				if len(native.Incoming) != 2 || len(native.Outgoing) != 2 {
					t.Fatal("ASSERT_WINDOW_NATIVE_MULTIPLICITY")
				}
				if native.IncomingWire.ResponseSHA256Private() != sha256.Sum256(starter.child.incomingFrame) || native.OutgoingWire.ResponseSHA256Private() != sha256.Sum256(starter.child.outgoingFrame) {
					t.Fatal("ASSERT_WINDOW_ORIGINAL_NATIVE_RECEIPTS")
				}
				if privateProviderWindowFrom(ctx).preparations != 2 {
					t.Fatal("ASSERT_WINDOW_TWO_SOURCE_PREPARATIONS")
				}
			}
		})
	}
}
func TestPrivateProviderWindowActivationAndUnknownBackend(t *testing.T) {
	m, start, ctx, starter := windowTestSetup(t, PrivateProviderWindowPlan{Definitions: true}, "")
	w := privateProviderWindowFrom(ctx)
	e := PrivateProviderWindowEnvelope{w.spec}
	if _, err := m.PrivateProviderWindowContext(context.Background(), e, start.Deadline); err == nil {
		t.Fatal("ASSERT_WINDOW_NO_RESET")
	}
	again, err := m.PrivateProviderWindowContext(ctx, e, start.Deadline)
	if err != nil || privateProviderWindowFrom(again) != w {
		t.Fatal("ASSERT_WINDOW_SAME_OBJECT")
	}
	if _, err := m.PrivateProviderWindowContext(ctx, e, start.Deadline.Add(time.Second)); err == nil {
		t.Fatal("ASSERT_WINDOW_DEADLINE_BINDING")
	}
	other, _ := New(Config{Limits: m.limits, Starter: starter})
	if _, err := other.PrivateProviderWindowContext(ctx, e, start.Deadline); err == nil {
		t.Fatal("ASSERT_WINDOW_MANAGER_BINDING")
	}
	m.mu.Lock()
	m.wire.MaxBodyBytes = 1
	m.mu.Unlock()
	if _, err := m.PrivateProviderWindowContext(ctx, e, start.Deadline); err == nil {
		t.Fatal("ASSERT_WINDOW_CONFIG_BINDING")
	}
	m.mu.Lock()
	m.wire = w.spec.configured
	m.mu.Unlock()
	changed := start
	changed.Process.Path = "wrong-attempt-spec"
	if got := m.Start(ctx, changed); got.Failure == "" || starter.starts != 0 {
		t.Fatal("ASSERT_WINDOW_START_SPEC_ZERO_EFFECT")
	}
	unknown, _ := New(Config{Limits: m.limits, Starter: &referenceStarter{}})
	if _, err := unknown.PrivateProviderWindowEnvelope(start, PrivateProviderWindowPlan{Definitions: true}); err != ErrPrivateProviderWindowUnavailable {
		t.Fatal("ASSERT_WINDOW_UNKNOWN_BACKEND")
	}
	// No process started; retire the reserved owner without claiming completion.
	_, _ = m.FinishPrivateProviderWindow(ctx)
}
func TestPrivateProviderWindowStalledReadinessRetainsOwner(t *testing.T) {
	for _, stall := range []string{"initialize-write", "initialize-read", "close", "teardown"} {
		t.Run(stall, func(t *testing.T) {
			m, start, ctx, starter := windowTestSetup(t, PrivateProviderWindowPlan{Definitions: true}, stall)
			w := privateProviderWindowFrom(ctx)
			got := m.Start(ctx, start)
			if got.Failure != "" {
				t.Fatal(got.Failure)
			}
			op := m.BeginReadiness(ctx, got.SessionID, got.Generation, start.Deadline)
			<-starter.child.entered
			w.cancel()
			if stall == "close" || stall == "teardown" {
				<-starter.child.cleanupEntered
			}
			pendingResult, found := m.WaitReadiness(ctx, op.ID)
			if !found || pendingResult.State != ReadinessFailed || pendingResult.Failure != privateProviderWindowUnjoined {
				t.Fatal("ASSERT_WINDOW_READINESS_CALLER_RESULT")
			}
			if _, err := m.FinishPrivateProviderWindow(ctx); err != ErrPrivateProviderWindowUnjoined {
				t.Fatal("ASSERT_WINDOW_CANCEL_NO_COMPLETION")
			}
			if m.Census().Workers == 0 || m.Census().Sessions != 1 {
				t.Fatal("ASSERT_WINDOW_RETAINED_OWNER")
			}
			close(starter.child.release)
			select {
			case <-w.finishDone:
			case <-time.After(2 * time.Second):
				t.Fatalf("cleanup did not join: %+v op=%+v", m.Census(), op)
			}
			if got, err := m.FinishPrivateProviderWindow(ctx); err == nil || got.Completed {
				t.Fatal("ASSERT_WINDOW_EXPIRY_IRREVERSIBLE")
			}
		})
	}
}
func TestPrivateProviderWindowFrameAndServerRequestLimits(t *testing.T) {
	for _, kind := range []string{"notifications", "server-request", "oversized"} {
		t.Run(kind, func(t *testing.T) {
			m, start, ctx, s := windowTestSetup(t, PrivateProviderWindowPlan{Definitions: true}, "")
			windowTestReady(t, m, start, ctx)
			c := s.child
			if kind == "notifications" {
				c.notifications = 4
			}
			if kind == "server-request" {
				c.serverRequest = true
			}
			if kind == "oversized" {
				c.oversized = true
			}
			st := privateProviderWindowFrom(ctx).start
			m.PrepareDocument(ctx, DocumentRequest{SessionID: st.SessionID, Generation: st.Generation, URI: c.uri, LanguageID: "go"})
			got := m.RoundTrip(ctx, RoundTripRequest{SessionID: st.SessionID, Generation: st.Generation, Method: "textDocument/documentSymbol", Params: json.RawMessage(fmt.Sprintf(`{"textDocument":{"uri":%q}}`, c.uri)), Deadline: start.Deadline, MaxMessages: 4, MaxBytes: 128 << 10})
			if got.Failure == "" {
				t.Fatalf("ASSERT_WINDOW_BOUNDED_INGRESS: %s", kind)
			}
			_, _ = m.FinishPrivateProviderWindow(ctx)
		})
	}
}
func TestPrivateProviderWindowReadClippedBeforeBackend(t *testing.T) {
	m, start, ctx, s := windowTestSetup(t, PrivateProviderWindowPlan{Definitions: true}, "")
	windowTestReady(t, m, start, ctx)
	w := privateProviderWindowFrom(ctx)
	w.mu.Lock()
	w.remaining = 3
	w.mu.Unlock()
	probe := &windowReadProbe{}
	r := privateProviderWindowReader{w.child, probe}
	_, _ = r.Read(make([]byte, 100))
	if probe.maximum != 3 {
		t.Fatalf("ASSERT_WINDOW_READ_CLIPPED: backend=%d want=3", probe.maximum)
	}
	w.cancel()
	_, _ = m.FinishPrivateProviderWindow(ctx)
	_ = s
}

type windowReadProbe struct{ maximum int }

func (p *windowReadProbe) Read(b []byte) (int, error) { p.maximum = len(b); return 0, io.EOF }
func (*windowReadProbe) Close() error                 { return nil }
func TestPrivateProviderWindowSourceLimitBeforeSupply(t *testing.T) {
	m, start, ctx, s := windowTestSetup(t, PrivateProviderWindowPlan{Definitions: true}, "")
	windowTestReady(t, m, start, ctx)
	path := strings.TrimPrefix(s.child.uri, "file://")
	if err := os.WriteFile(path, bytes.Repeat([]byte("x"), (16<<10)+1), 0600); err != nil {
		t.Fatal(err)
	}
	st := privateProviderWindowFrom(ctx).start
	got := m.PrepareDocument(ctx, DocumentRequest{SessionID: st.SessionID, Generation: st.Generation, URI: s.child.uri, LanguageID: "go"})
	if got.Failure == "" || got.Supply != nil {
		t.Fatal("ASSERT_WINDOW_SOURCE_PREBOUND")
	}
	_, _ = m.FinishPrivateProviderWindow(ctx)
}
