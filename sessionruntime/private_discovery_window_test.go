package sessionruntime

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"lsp-trace/internal/lsp"
	"lsp-trace/internal/lspwire"
	"lsp-trace/internal/managedprocess"
	"lsp-trace/internal/runtimeprofile"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func discoveryLimitsTest() PrivateDiscoveryLimits {
	return PrivateDiscoveryLimits{SourceBytes: 16 * (MaxDocumentSupplyBytes + 1), RequestBytes: 32 << 20, ResponseBytes: 16 << 20, WireBytes: 48 << 20, RetainedBytes: 512 << 20, Work: 1 << 32, Messages: 256, Requests: 128, FrameBytes: 1 << 20, HeaderBytes: 8192}
}
func TestPrivateDiscoveryWindowSourceBeforeSupply(t *testing.T) {
	for _, cached := range []bool{false, true} {
		t.Run(fmt.Sprint(cached), func(t *testing.T) {
			root, e := filepath.EvalSymlinks(t.TempDir())
			if e != nil {
				t.Fatal(e)
			}
			file := filepath.Join(root, "x.go")
			if e = os.WriteFile(file, []byte("abcde"), 0600); e != nil {
				t.Fatal(e)
			}
			p, e := runtimeprofile.Validate(runtimeprofile.Selector{TrustDomain: "test", Workspace: root, Profile: "go", EnvironmentReference: "local"})
			if e != nil {
				t.Fatal(e)
			}
			writer := &supplyWriter{}
			m, e := New(Config{Limits: Limits{MaxSessions: 1, MaxRequests: 8, MaxChildren: 2, MaxCancels: 2, MaxTombstones: 8, MaxObservations: 16}, Starter: oneChildStarter{supplyChild{writer}}})
			if e != nil {
				t.Fatal(e)
			}
			started := m.Start(context.Background(), StartRequest{Profile: runtimeprofile.Resolve(p), LanguageID: "go"})
			if started.Failure != "" {
				t.Fatal(started)
			}
			m.ObserveInitialization(started.SessionID, started.Generation, true)
			req := DocumentRequest{SessionID: started.SessionID, Generation: started.Generation, URI: (&url.URL{Scheme: "file", Path: file}).String(), LanguageID: "go", CaptureSupply: true, PreparedNoFollow: true}
			if cached {
				first := m.PrepareDocument(context.Background(), req)
				if first.Failure != "" {
					t.Fatal(first)
				}
			}
			before := writer.Len()
			req.PrivateSourceByteLimit = 4
			if r := m.PrepareDocument(context.Background(), req); r.Failure == "" || r.Supply != nil || writer.Len() != before {
				t.Fatal("ASSERT_DISCOVERY_SOURCE_LIMIT_BEFORE_CLONE_OR_WRITE")
			}
		})
	}
	t.Log("ASSERT_DISCOVERY_SOURCE_BEFORE_SUPPLY PASS")
}

type discoveryCountReader struct {
	io.Reader
	calls, bytes, requested int
}

func (r *discoveryCountReader) Read(p []byte) (int, error) {
	r.calls++
	r.requested += len(p)
	n, e := r.Reader.Read(p)
	r.bytes += n
	return n, e
}

type discoveryStderrErrorReader struct{}

func (discoveryStderrErrorReader) Read([]byte) (int, error) {
	return 0, fmt.Errorf("synthetic nonpublic reader error")
}

// Actual ManagedStarter path, deliberately invalid empty process spec: command
// construction refuses before pipes/exec. The scripted reader drives the same
// primitive used by production copyStderr, using the context installed by Start.
func TestPrivateDiscoveryWindowManagedStderr(t *testing.T) {
	backend, e := managedprocess.NewLocalDarwinSupervisor(managedprocess.Options{StderrLimit: 4096})
	if e != nil {
		t.Skip("Darwin backend unavailable; no portable qualification")
	}
	deadline := time.Now().Add(10 * time.Second)
	ctx, cancel := context.WithDeadline(context.Background(), deadline)
	defer cancel()
	limits := discoveryLimitsTest()
	w := &privateDiscoveryWindow{ctx: ctx, limits: limits, remaining: limits}
	w.ctx = context.WithValue(ctx, privateDiscoveryContextKey{}, w)
	if !w.prepayBackend() {
		t.Fatal("ASSERT_STDERR_PREPAY")
	}
	starter := ManagedStarter{Manager: backend}
	_, started := starter.Start(w.ctx, managedprocess.Spec{})
	if started.Kind != managedprocess.StartFailed {
		t.Fatal("ASSERT_STDERR_NO_PROCESS", started)
	}
	p := backend.PrivateProviderWindowProfile()
	reader := &discoveryCountReader{Reader: bytes.NewReader(make([]byte, 128<<10))}
	buffer := make([]byte, 8192)
	n, e := backend.PrivateProviderWindowReadStderr(w.backendCtx, p, deadline, reader, buffer)
	if e != nil || n != 4096 || reader.requested != 4096 {
		t.Fatal("ASSERT_STDERR_ACTUAL_MANAGED_BINDING", n, reader.requested, e)
	}
	// Invalid admissions are checked while the real pool still has capacity.
	invalidCalls := reader.calls
	badProfile := p
	badProfile.StderrRetention++
	for _, bad := range []struct {
		ctx context.Context
		p   managedprocess.PrivateProviderWindowProfile
		d   time.Time
	}{{ctx, p, deadline}, {w.backendCtx, badProfile, deadline}, {w.backendCtx, p, deadline.Add(time.Second)}} {
		if _, e := backend.PrivateProviderWindowReadStderr(bad.ctx, bad.p, bad.d, reader, buffer); e == nil || reader.calls != invalidCalls {
			t.Fatal("ASSERT_STDERR_INVALID_LIVE_POOL_NO_READ")
		}
	}
	otherLive, _ := managedprocess.NewLocalDarwinSupervisor(managedprocess.Options{StderrLimit: 4096})
	if _, e := otherLive.PrivateProviderWindowReadStderr(w.backendCtx, p, deadline, reader, buffer); e == nil || reader.calls != invalidCalls {
		t.Fatal("ASSERT_STDERR_OWNER_LIVE_POOL_NO_READ")
	}
	dead, stopDead := context.WithCancel(w.backendCtx)
	stopDead()
	if _, e := backend.PrivateProviderWindowReadStderr(dead, p, deadline, reader, buffer); e == nil || reader.calls != invalidCalls {
		t.Fatal("ASSERT_STDERR_CANCEL_LIVE_POOL_NO_READ")
	}
	// An error burns its full requested chunk, exposes only the closed error, and
	// does not refund the shared pool. Restore the same scripted stream afterward.
	originalReader := reader.Reader
	reader.Reader = discoveryStderrErrorReader{}
	if n, e := backend.PrivateProviderWindowReadStderr(w.backendCtx, p, deadline, reader, buffer); n != 0 || e != managedprocess.ErrPrivateProviderWindow {
		t.Fatal("ASSERT_STDERR_CLOSED_ERROR_NO_REFUND", n, e)
	}
	reader.Reader = originalReader
	// Five startup attempts, five operation attempts, six cleanup attempts share 64KiB.
	readCtx := w.backendCtx
	for i := 2; i < 16; i++ {
		if i == 5 || i == 10 {
			copyStarter := starter
			_, o := copyStarter.Start(context.WithValue(w.ctx, struct{ phase int }{i}, true), managedprocess.Spec{})
			if o.Kind != managedprocess.StartFailed {
				t.Fatal("no process")
			}
			readCtx, e = backend.PrivateProviderWindowContext(context.WithValue(w.backendCtx, struct{ phase int }{i}, true), p, deadline)
			if e != nil {
				t.Fatal("ASSERT_STDERR_CONTEXT_REUSE", e)
			}
		}
		if _, e := backend.PrivateProviderWindowReadStderr(readCtx, p, deadline, reader, buffer); e != nil {
			t.Fatal("ASSERT_STDERR_SHARED_POOL", i, e)
		}
	}
	before := reader.calls
	if _, e := backend.PrivateProviderWindowReadStderr(w.backendCtx, p, deadline, reader, buffer); e == nil || reader.calls != before || reader.requested != 64<<10 || reader.bytes != 60<<10 {
		t.Fatal("ASSERT_STDERR_READ_BOUND_BEFORE_SYSCALL", reader, e)
	}
	if _, e := backend.PrivateProviderWindowContext(w.backendCtx, p, deadline.Add(time.Second)); e == nil {
		t.Fatal("ASSERT_STDERR_NO_DEADLINE_RENEWAL")
	}
	if _, e := backend.PrivateProviderWindowReadStderr(ctx, p, deadline, reader, buffer); e == nil || reader.calls != before {
		t.Fatal("ASSERT_STDERR_MISSING_CONTEXT_NO_READ")
	}
	other, _ := managedprocess.NewLocalDarwinSupervisor(managedprocess.Options{StderrLimit: 4096})
	if _, e := other.PrivateProviderWindowReadStderr(w.backendCtx, p, deadline, reader, buffer); e == nil || reader.calls != before {
		t.Fatal("ASSERT_STDERR_WRONG_OWNER_NO_READ")
	}
	cancelled, stop := context.WithCancel(w.backendCtx)
	stop()
	if _, e := backend.PrivateProviderWindowReadStderr(cancelled, p, deadline, reader, buffer); e == nil || reader.calls != before {
		t.Fatal("ASSERT_STDERR_CANCELLED_NO_READ")
	}
	t.Log("ASSERT_STDERR_ACTUAL_MANAGED_BINDING PASS; ASSERT_STDERR_READ_BOUND_BEFORE_SYSCALL PASS; requested=65536 returned=61440 retention=4096 (error chunk not refunded)")
}
func TestPrivateDiscoveryWindowRawBeforeDecodeAndWrite(t *testing.T) {
	limits := discoveryLimitsTest()
	limits.FrameBytes = 128
	w := &privateDiscoveryWindow{ctx: context.Background(), limits: limits, remaining: limits}
	reader := &discoveryCountReader{Reader: bytes.NewBufferString("Content-Length: 1000000\r\n\r\n" + string(bytes.Repeat([]byte{'x'}, 1024)))}
	child := discoveryRawChild{reader: io.NopCloser(reader)}
	if _, _, _, _, e := w.frame(child); e == nil || reader.bytes > 32 {
		t.Fatal("ASSERT_DISCOVERY_RAW_REJECT_BEFORE_BODY_ALLOCATION", reader.bytes, e)
	}
	var dst bytes.Buffer
	limits.RequestBytes = 2
	limits.WireBytes = 2
	w = &privateDiscoveryWindow{ctx: context.Background(), limits: limits, remaining: limits}
	if _, e := (privateDiscoveryWriter{WriteCloser: discoveryWriteCloser{&dst}, w: w}).Write([]byte("abc")); e == nil || dst.Len() != 0 {
		t.Fatal("ASSERT_DISCOVERY_WRITE_RESERVED_BEFORE_EFFECT")
	}
	t.Log("ASSERT_DISCOVERY_RAW_PREDECODE_WRITE PASS")
}

type discoveryWriteCloser struct{ io.Writer }

func (discoveryWriteCloser) Close() error { return nil }

type discoveryRawChild struct{ reader io.ReadCloser }

func (c discoveryRawChild) Stdin() io.WriteCloser { return discoveryWriteCloser{io.Discard} }
func (c discoveryRawChild) Stdout() io.ReadCloser { return c.reader }
func (discoveryRawChild) Close() managedprocess.ResourceObservation {
	return managedprocess.ResourceObservation{Kind: managedprocess.ResourcesClosed}
}
func (discoveryRawChild) Teardown(context.Context) managedprocess.TeardownObservation {
	return managedprocess.TeardownObservation{}
}

type discoveryPipeChild struct {
	input  *io.PipeReader
	stdin  *io.PipeWriter
	stdout *io.PipeReader
	output *io.PipeWriter
	once   sync.Once
}

func newDiscoveryPipeChild() *discoveryPipeChild {
	input, stdin := io.Pipe()
	stdout, output := io.Pipe()
	c := &discoveryPipeChild{input: input, stdin: stdin, stdout: stdout, output: output}
	go func() {
		reader := lspwire.NewReader(input, lspwire.DefaultLimits())
		writer := lspwire.NewWriter(output, lspwire.DefaultLimits())
		for {
			message, e := reader.Read()
			if e != nil {
				return
			}
			if message.Method == "exit" {
				return
			}
			if len(message.ID) == 0 {
				continue
			}
			result := json.RawMessage(`[]`)
			if message.Method == "initialize" {
				result = json.RawMessage(`{"capabilities":{"documentSymbolProvider":true,"callHierarchyProvider":true}}`)
			}
			if message.Method == "shutdown" {
				result = json.RawMessage(`null`)
			}
			if writer.Write(lspwire.Message{JSONRPC: lspwire.Version, ID: message.ID, Result: result}) != nil {
				return
			}
		}
	}()
	return c
}
func (c *discoveryPipeChild) Stdin() io.WriteCloser { return c.stdin }
func (c *discoveryPipeChild) Stdout() io.ReadCloser { return c.stdout }
func (c *discoveryPipeChild) Teardown(context.Context) managedprocess.TeardownObservation {
	c.once.Do(func() { c.stdin.Close(); c.input.Close(); c.output.Close(); c.stdout.Close() })
	return managedprocess.TeardownObservation{Death: managedprocess.DeathObservation{Kind: managedprocess.DeathExited, Reap: managedprocess.ReapObservation{Kind: managedprocess.ReapComplete}}, Census: managedprocess.GroupCensusObservation{Bounded: true}}
}
func (c *discoveryPipeChild) Close() managedprocess.ResourceObservation {
	c.Teardown(context.Background())
	return managedprocess.ResourceObservation{Kind: managedprocess.ResourcesClosed}
}

type discoveryStderrErrorCleanup struct{ *discoveryPipeChild }

func (c discoveryStderrErrorCleanup) Teardown(ctx context.Context) managedprocess.TeardownObservation {
	o := c.discoveryPipeChild.Teardown(ctx)
	o.Death.StderrDelivery.Err = managedprocess.ErrPrivateProviderWindow
	return o
}

type discoveryUnknownCleanup struct{ *discoveryPipeChild }

func (c discoveryUnknownCleanup) Teardown(ctx context.Context) managedprocess.TeardownObservation {
	o := c.discoveryPipeChild.Teardown(ctx)
	o.Census.Bounded = false
	return o
}
func TestPrivateDiscoveryWindowSixteenMembersOneLifetime(t *testing.T) {
	root, e := filepath.EvalSymlinks(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	p, e := runtimeprofile.Validate(runtimeprofile.Selector{TrustDomain: "test", Workspace: root, Profile: "go", EnvironmentReference: "local"})
	if e != nil {
		t.Fatal(e)
	}
	cfg := PrivateDiscoveryConstruction{Start: StartRequest{Profile: runtimeprofile.Resolve(p), LanguageID: "go"}, Deadline: time.Now().Add(10 * time.Second), Limits: discoveryLimitsTest()}
	for i := 0; i < 16; i++ {
		path := filepath.Join(root, fmt.Sprintf("x%d.go", i))
		if e = os.WriteFile(path, []byte("package x\n"), 0600); e != nil {
			t.Fatal(e)
		}
		cfg.Members = append(cfg.Members, PrivateDiscoveryMember{Path: filepath.Base(path), URI: (&url.URL{Scheme: "file", Path: path}).String(), LanguageID: "go"})
	}
	starts := 0
	cfg.NewStarter = func() (Starter, error) { starts++; return oneChildStarter{newDiscoveryPipeChild()}, nil }
	supplied := 0
	observation, e := RunPrivateDiscovery(context.Background(), cfg, func(ctx context.Context, s *PrivateDiscoverySession) error {
		for _, f := range cfg.Members {
			if e := s.Supply(ctx, f.URI, f.LanguageID); e != nil {
				return e
			}
			supplied++
			if _, e := s.DocumentSymbols(ctx, lsp.DocumentSymbolParams{TextDocument: lsp.TextDocumentIdentifier{URI: f.URI}}); e != nil {
				return e
			}
		}
		return nil
	})
	if e != nil || !observation.CleanupJoined || observation.Failed || starts != 1 || supplied != 16 {
		t.Fatalf("ASSERT_DISCOVERY_SINGLE_LIFETIME_ALL_MEMBERS starts=%d supplied=%d observation=%+v err=%v", starts, supplied, observation, e)
	}
	partial, e := RunPrivateDiscovery(context.Background(), cfg, func(ctx context.Context, s *PrivateDiscoverySession) error {
		return s.Supply(ctx, cfg.Members[0].URI, "go")
	})
	if e == nil || !partial.Failed || !partial.CleanupJoined {
		t.Fatal("ASSERT_DISCOVERY_FIRST_CALLABLE_NOT_PARTITION_SUCCESS", partial, e)
	}
	cfg.NewStarter = func() (Starter, error) { return oneChildStarter{discoveryUnknownCleanup{newDiscoveryPipeChild()}}, nil }
	unknown, e := RunPrivateDiscovery(context.Background(), cfg, func(ctx context.Context, s *PrivateDiscoverySession) error {
		for _, f := range cfg.Members {
			if e := s.Supply(ctx, f.URI, f.LanguageID); e != nil {
				return e
			}
		}
		return nil
	})
	if e == nil || unknown.CleanupJoined || !unknown.Failed {
		t.Fatal("ASSERT_DISCOVERY_UNKNOWN_CLEANUP_SUPPRESSES", unknown, e)
	}
	cfg.NewStarter = func() (Starter, error) {
		return oneChildStarter{discoveryStderrErrorCleanup{newDiscoveryPipeChild()}}, nil
	}
	stderrFailed, e := RunPrivateDiscovery(context.Background(), cfg, func(ctx context.Context, s *PrivateDiscoverySession) error {
		for _, f := range cfg.Members {
			if e := s.Supply(ctx, f.URI, f.LanguageID); e != nil {
				return e
			}
		}
		return nil
	})
	if e == nil || !stderrFailed.Failed || stderrFailed.CleanupJoined {
		t.Fatal("ASSERT_STDERR_JOINED_ERROR_SUPPRESSES_WITHOUT_TRUNCATION", stderrFailed, e)
	}
	cfg.Limits.ResponseBytes = 64 << 10
	cfg.NewStarter = func() (Starter, error) { t.Fatal("ASSERT_STDERR_PREFUND_BEFORE_FACTORY"); return nil, nil }
	if _, e := RunPrivateDiscovery(context.Background(), cfg, func(context.Context, *PrivateDiscoverySession) error { return nil }); e == nil {
		t.Fatal("ASSERT_STDERR_UNFUNDED_CONSTRUCTOR_REFUSED")
	}
	cfg.Limits = discoveryLimitsTest()
	cfg.NewStarter = func() (Starter, error) { return oneChildStarter{newDiscoveryPipeChild()}, nil }
	cfg.Limits.Requests = 2
	firstSucceeded := false
	residual, e := RunPrivateDiscovery(context.Background(), cfg, func(ctx context.Context, s *PrivateDiscoverySession) error {
		for _, f := range cfg.Members {
			if e := s.Supply(ctx, f.URI, f.LanguageID); e != nil {
				return e
			}
		}
		for i := 0; i < 2; i++ {
			_, e := s.DocumentSymbols(ctx, lsp.DocumentSymbolParams{TextDocument: lsp.TextDocumentIdentifier{URI: cfg.Members[0].URI}})
			if e != nil {
				return e
			}
			firstSucceeded = true
		}
		return nil
	})
	if e == nil || !firstSucceeded || !residual.Failed {
		t.Fatal("ASSERT_DISCOVERY_CUMULATIVE_REQUESTS_NOT_RENEWED", residual, e)
	}
	cfg.Deadline = time.Now().Add(-time.Second)
	cfg.NewStarter = func() (Starter, error) { t.Fatal("ASSERT_DISCOVERY_EXPIRED_NO_FACTORY"); return nil, nil }
	if _, e := RunPrivateDiscovery(context.Background(), cfg, func(context.Context, *PrivateDiscoverySession) error { return nil }); e == nil {
		t.Fatal("ASSERT_DISCOVERY_ORIGINAL_EXPIRED_DEADLINE_REFUSED")
	}
	t.Log("ASSERT_DISCOVERY_SINGLE_LIFETIME_ALL_MEMBERS PASS")
}
