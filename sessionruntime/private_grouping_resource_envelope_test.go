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
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"lsp-trace/internal/lspwire"
	"lsp-trace/internal/runtimeprofile"
	"lsp-trace/internal/session"
)

func TestPrivateGroupingResourceNormalization(t *testing.T) {
	for _, w := range []lspwire.Limits{{}, {MaxBodyBytes: -1, MaxHeaderBytes: -1}, lspwire.DefaultLimits(), {MaxBodyBytes: 1024, MaxHeaderBytes: 128}} {
		m := &Manager{wire: w}
		e, ok := m.PrivateGroupingResourceEnvelope()
		if !ok {
			t.Fatal("ASSERT_RESOURCE_NORMALIZATION", w)
		}
		b := e.Bounds()
		if b.SourceBytes != 16<<10 || b.SourceReadBytes != 16385 || b.BodyBytes == 0 || b.HeaderBytes == 0 || b.BodyBytes > 128<<10 || b.HeaderBytes > 4096 || b.WireBytes != b.RequestBytes+b.ResponseBytes {
			t.Fatal("ASSERT_RESOURCE_CONCRETE_BOUNDS", b)
		}
		if w.MaxBodyBytes == 1024 && (b.BodyBytes != 1024 || b.HeaderBytes != 128) {
			t.Fatal("ASSERT_RESOURCE_TIGHT_CONFIG_USED", b)
		}
	}
	for _, m := range []*Manager{nil, {wire: lspwire.Limits{MaxBodyBytes: 1 << 30}}, {wire: lspwire.Limits{MaxHeaderBytes: 1 << 30}}} {
		if _, ok := m.PrivateGroupingResourceEnvelope(); ok {
			t.Fatal("ASSERT_RESOURCE_UNKNOWN_CONFIG_REFUSED")
		}
	}
	if (PrivateGroupingResourceEnvelope{}).Bounds() != (PrivateGroupingResourceBounds{}) {
		t.Fatal("ASSERT_RESOURCE_ZERO_OPAQUE_REFUSED")
	}
}
func TestPrivateGroupingResourceDriftAndNoReset(t *testing.T) {
	m := &Manager{}
	e, _ := m.PrivateGroupingResourceEnvelope()
	deadline := time.Now().Add(time.Second)
	ctx, err := m.PrivateGroupingEnvelopeContext(context.Background(), e, deadline)
	if err != nil {
		t.Fatal(err)
	}
	s := privateGroupingResourcesFrom(ctx)
	s.remaining = 7
	again, err := m.PrivateGroupingEnvelopeContext(ctx, e, deadline)
	if err != nil || privateGroupingResourcesFrom(again) != s || s.remaining != 7 {
		t.Fatal("ASSERT_RESOURCE_NO_RESIDUAL_RESET")
	}
	m.wire.MaxBodyBytes = 1024
	if _, err := m.PrivateGroupingEnvelopeContext(ctx, e, deadline); err == nil {
		t.Fatal("ASSERT_RESOURCE_ENVELOPE_DRIFT")
	}
	req := RoundTripRequest{Method: "textDocument/documentSymbol", MaxMessages: 4, MaxBytes: 65536, Deadline: deadline, CaptureOwnedMethodPair: true, ExpectedOwnedDocument: &OwnedDocumentBinding{}, Params: json.RawMessage(`{}`)}
	if result := m.RoundTrip(ctx, req); result.Failure != session.ResourceExhausted {
		t.Fatal("ASSERT_RESOURCE_DRIFT_BEFORE_NATIVE", result.Failure)
	}
}
func TestPrivateGroupingResourceAggregateBeforeNextEffect(t *testing.T) {
	m := &Manager{}
	e, _ := m.PrivateGroupingResourceEnvelope()
	deadline := time.Now().Add(time.Second)
	ctx, _ := m.PrivateGroupingEnvelopeContext(context.Background(), e, deadline)
	s := privateGroupingResourcesFrom(ctx)
	s.supplied = true
	req := RoundTripRequest{Method: "textDocument/documentSymbol", MaxMessages: 4, MaxBytes: 65536, Deadline: deadline, CaptureOwnedMethodPair: true, ExpectedOwnedDocument: &OwnedDocumentBinding{}, Params: json.RawMessage(`{}`)}
	s.remaining = 6*(32<<10) + 8192 - 1
	if result := m.RoundTrip(ctx, req); result.Failure != session.ResourceExhausted {
		t.Fatal("ASSERT_RESOURCE_AGGREGATE_BEFORE_NEXT_SEND", result.Failure)
	}
	source := &privateResourceCountingReader{Reader: bytes.NewReader([]byte("abc"))}
	if n, err := (privateGroupingResourceReader{source, s, ctx}).Read(make([]byte, 4)); n != 0 || err == nil || source.n != 0 {
		t.Fatal("ASSERT_RESOURCE_AGGREGATE_BEFORE_NEXT_READ", n, err, source.n)
	}
	// A fresh operation is separate, not a reset of the prior residual. Once
	// exhausted, additional methods/frames/prepare cannot refresh this operation.
	ctx, _ = m.PrivateGroupingEnvelopeContext(context.Background(), e, deadline)
	s = privateGroupingResourcesFrom(ctx)
	s.frames = 27
	if _, ok := s.beginFrame(); ok {
		t.Fatal("ASSERT_RESOURCE_MESSAGE_AGGREGATE")
	}
	if s.prepare(ctx, m, DocumentRequest{URI: "file:///x", PreparedNoFollow: true, CaptureSupply: true}) {
		t.Fatal("ASSERT_RESOURCE_FAILED_STICKY")
	}
}

type privateResourceCountingReader struct {
	*bytes.Reader
	n int
}

func (r *privateResourceCountingReader) Read(p []byte) (int, error) {
	n, e := r.Reader.Read(p)
	r.n += n
	return n, e
}
func TestPrivateGroupingResourceResidualBeforeBodyAllocation(t *testing.T) {
	m := &Manager{}
	e, _ := m.PrivateGroupingResourceEnvelope()
	ctx, _ := m.PrivateGroupingEnvelopeContext(context.Background(), e, time.Now().Add(time.Second))
	s := privateGroupingResourcesFrom(ctx)
	wire := []byte("Content-Length: 100\r\n\r\n" + string(bytes.Repeat([]byte("x"), 100)))
	s.remaining = 50
	max, ok := s.beginFrame()
	if !ok || max != 50 {
		t.Fatal("ASSERT_RESOURCE_RESIDUAL_FRAME_CAP")
	}
	source := &privateResourceCountingReader{Reader: bytes.NewReader(wire)}
	_, _, _, _, err := lspwire.ReadPrivateBoundedFrame(privateGroupingResourceReader{source, s, ctx}, lspwire.Limits{MaxBodyBytes: 128 << 10, MaxHeaderBytes: 4096}, max, 0)
	if err == nil || source.n != len("Content-Length: 100\r\n\r\n") {
		t.Fatal("ASSERT_RESOURCE_RESIDUAL_BEFORE_BODY", err, source.n)
	}
}

// This is the same controlled io.Pipe Child contract as roundTripChild, with
// finite response specimens and an actual backend byte counter. No provider.
type privateGroupingObserverChild struct {
	*roundTripChild
	readBytes atomic.Int64
}
type privateGroupingObserverReadCloser struct {
	io.ReadCloser
	count *atomic.Int64
}

func (r privateGroupingObserverReadCloser) Read(p []byte) (int, error) {
	n, err := r.ReadCloser.Read(p)
	r.count.Add(int64(n))
	return n, err
}
func (c *privateGroupingObserverChild) Stdout() io.ReadCloser {
	return privateGroupingObserverReadCloser{c.stdout, &c.readBytes}
}

type privateGroupingObserverTrace struct {
	mu                                sync.Mutex
	notices                           []string
	entries, valid, invalid, retained int
}

func (s *privateGroupingObserverTrace) hooks(panics bool) (lspwire.Observer, func(), func()) {
	wire := func(e lspwire.Event) {
		if e.Stage != lspwire.EventHeaderRead && e.Stage != lspwire.EventBodyRead && e.Stage != lspwire.EventDecode {
			return
		}
		s.mu.Lock()
		s.notices = append(s.notices, fmt.Sprintf("wire:%d:%d:%d:%t", e.Stage, e.Bytes, e.Kind, e.Closed))
		if e.Stage == lspwire.EventDecode {
			if e.Kind == lspwire.KindInvalid {
				s.invalid++
			} else {
				s.valid++
			}
		}
		s.mu.Unlock()
		if panics {
			panic("controlled observer panic")
		}
	}
	entry := func() {
		s.mu.Lock()
		s.entries++
		s.notices = append(s.notices, "decode")
		s.mu.Unlock()
		if panics {
			panic("controlled decode panic")
		}
	}
	retain := func() {
		s.mu.Lock()
		s.retained++
		s.notices = append(s.notices, "retain")
		s.mu.Unlock()
		if panics {
			panic("controlled retain panic")
		}
	}
	return wire, entry, retain
}

func privateGroupingObserverTransaction(t *testing.T, body string, rawHeader bool, retain int64, hooks bool, panics bool) (RoundTripResult, *privateGroupingObserverTrace, int64, []byte) {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(root, "query.go")
	content := []byte("package fixture\nfunc Query() {}\n")
	if err = os.WriteFile(file, content, 0644); err != nil {
		t.Fatal(err)
	}
	profile, err := runtimeprofile.Validate(runtimeprofile.Selector{TrustDomain: "test", Workspace: root, Profile: "go", EnvironmentReference: "local"})
	if err != nil {
		t.Fatal(err)
	}
	in, stdin := io.Pipe()
	stdout, out := io.Pipe()
	child := &privateGroupingObserverChild{roundTripChild: &roundTripChild{input: in, stdin: stdin, output: out, stdout: stdout}}
	go func() {
		reader := lspwire.NewReader(in, lspwire.DefaultLimits())
		for {
			msg, err := reader.Read()
			if err != nil {
				return
			}
			if msg.Method != "textDocument/documentSymbol" {
				continue
			}
			wire := body
			if !rawHeader {
				wire = fmt.Sprintf("Content-Length: %d\r\n\r\n%s", len(body), body)
			}
			_, _ = io.WriteString(out, wire)
			return
		}
	}()
	m, err := New(Config{Limits: Limits{MaxSessions: 1, MaxRequests: 4, MaxChildren: 2, MaxCancels: 2, MaxTombstones: 8, MaxObservations: 64, MaxOperations: 2}, Starter: oneChildStarter{child}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { child.Close(); child.Teardown(context.Background()); m.Shutdown(context.Background()) })
	started := m.Start(context.Background(), StartRequest{Profile: runtimeprofile.Resolve(profile)})
	if started.Failure != "" {
		t.Fatal(started.Failure)
	}
	if result := m.ObserveInitialization(started.SessionID, started.Generation, true); result.Failure != "" {
		t.Fatal(result.Failure)
	}
	deadline := time.Now().Add(5 * time.Second)
	e, ok := m.PrivateGroupingResourceEnvelope()
	if !ok {
		t.Fatal("observer fixture envelope")
	}
	ctx, err := m.PrivateGroupingEnvelopeContext(context.Background(), e, deadline)
	if err != nil {
		t.Fatal(err)
	}
	uri := (&url.URL{Scheme: "file", Path: file}).String()
	trace := &privateGroupingObserverTrace{}
	var got RoundTripResult
	err = m.WithReadySessionLease(ctx, SessionLeaseRequest{SessionID: started.SessionID, Generation: started.Generation, Workspace: root}, func(ctx context.Context) error {
		d := m.PrepareDocument(ctx, DocumentRequest{SessionID: started.SessionID, Generation: started.Generation, URI: uri, LanguageID: "go", CaptureSupply: true, PreparedNoFollow: true})
		if d.Failure != "" || d.Supply == nil {
			return fmt.Errorf("observer fixture supply: %s", d.Failure)
		}
		source := OwnedDocumentBinding{URI: uri, Version: d.Version, SHA256: fmt.Sprintf("sha256:%x", sha256.Sum256(content))}
		req := documentSymbolOwnedRequest(started, source)
		req.MaxMessages = 4
		req.MaxBytes = 65536
		req.Deadline = deadline
		req.CaptureDocumentSymbolRequestFrameMaxBytes = 4096
		req.CaptureDocumentSymbolResponseFrameMaxBytes = retain
		if hooks {
			m.cWireObserver, m.cDecodeEntry, m.cRetainEntry = trace.hooks(panics)
		}
		got = m.RoundTrip(ctx, req)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	wire := []byte(body)
	if !rawHeader {
		wire = []byte(fmt.Sprintf("Content-Length: %d\r\n\r\n%s", len(body), body))
	}
	return got, trace, child.readBytes.Load(), wire
}

func TestPrivateGroupingResourceManagerObserversTraceAndIsolation(t *testing.T) {
	valid := `{"jsonrpc":"2.0","id":1,"result":[]}`
	for _, tc := range []struct {
		name, body string
		retain     int64
		panics     bool
	}{
		{"no-retention", valid, 0, false},
		{"retained", valid, 4096, false},
		{"over-retention-cap", valid, 30, false},
		{"raw-greater-than-64k-retained", valid + strings.Repeat(" ", 80<<10), 64 << 10, false},
		{"malformed-retained", `{"jsonrpc":"2.0","id":1,"result":`, 4096, false},
		{"panicking-hooks", valid, 4096, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, trace, n, wire := privateGroupingObserverTransaction(t, tc.body, false, tc.retain, true, tc.panics)
			expected := &privateGroupingObserverTrace{}
			observer, entry, retainEntry := expected.hooks(tc.panics)
			reader := lspwire.NewReaderObservedAtDecode(bytes.NewReader(wire), lspwire.Limits{MaxBodyBytes: 128 << 10, MaxHeaderBytes: 4096}, observer, entry, retainEntry)
			var message lspwire.Message
			var observation lspwire.ReadFrameObservation
			var raw []byte
			var retained bool
			var err error
			if tc.retain > 0 {
				message, observation, raw, retained, err = reader.ReadWithFrameIfWithin(tc.retain)
			} else {
				message, observation, err = reader.ReadWithFrame()
			}
			if !reflect.DeepEqual(trace.notices, expected.notices) || trace.entries != 1 || trace.valid != expected.valid || trace.invalid != expected.invalid || trace.retained != expected.retained {
				t.Fatalf("ASSERT_PRIVATE_MANAGER_OBSERVER_TRACE: got=%v want=%v", trace.notices, expected.notices)
			}
			_, pair := got.CompletedOwnedMethodPair()
			if err != nil {
				if got.Failure == "" || pair {
					t.Fatal("ASSERT_PRIVATE_OBSERVER_MALFORMED_UNCHANGED", got.Failure, pair)
				}
			} else {
				read, ok := got.CompletedResponseRead()
				frame, frameOK := got.CompletedDocumentSymbolResponseFrame()
				if got.Failure != "" || !pair || !bytes.Equal(got.Result, message.Result) || !ok || read.FrameBytes != observation.FrameBytes || read.FrameSHA256 != observation.FrameSHA256 || frameOK != retained || !bytes.Equal(frame, raw) {
					t.Fatalf("ASSERT_PRIVATE_OBSERVER_OUTCOME_UNCHANGED: failure=%s pair=%v read=%+v frame=%v", got.Failure, pair, read, frameOK)
				}
			}
			if n != int64(len(wire)) {
				t.Fatalf("ASSERT_PRIVATE_OBSERVER_NATIVE_EXACT: read=%d frame=%d", n, len(wire))
			}
		})
	}
}

func TestPrivateGroupingResourceManagerObserversFramingAndNil(t *testing.T) {
	for _, tc := range []struct {
		name, wire string
		read       int64
	}{
		{"header", strings.Repeat("x", 8192), 4097},
		{"body", "Content-Length: 131073\r\n\r\n", int64(len("Content-Length: 131073\r\n\r\n"))},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, trace, n, _ := privateGroupingObserverTransaction(t, tc.wire, true, 4096, true, false)
			_, pair := got.CompletedOwnedMethodPair()
			if got.Failure == "" || pair || trace.entries != 0 || trace.valid != 0 || trace.invalid != 0 || trace.retained != 0 || len(trace.notices) != 0 || n != tc.read {
				t.Fatalf("ASSERT_PRIVATE_OBSERVER_FRAMING_NO_DECODE: failure=%s pair=%v trace=%v read=%d want=%d", got.Failure, pair, trace.notices, n, tc.read)
			}
		})
	}
	t.Run("nil-hooks", func(t *testing.T) {
		got, trace, n, wire := privateGroupingObserverTransaction(t, `{"jsonrpc":"2.0","id":1,"result":[]}`, false, 4096, false, false)
		_, pair := got.CompletedOwnedMethodPair()
		if got.Failure != "" || !pair || trace.entries != 0 || len(trace.notices) != 0 || n != int64(len(wire)) {
			t.Fatal("ASSERT_PRIVATE_OBSERVER_NIL_COMPAT", got.Failure, pair, trace.notices, n)
		}
	})
}

func TestPrivateGroupingResourceManagerObservers(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		valid      bool
	}{
		{"valid", `{"jsonrpc":"2.0","id":1,"result":[]}`, true},
		{"malformed", `{"jsonrpc":"2.0","id":1,"result":`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, trace, read, wire := privateGroupingObserverTransaction(t, tc.body, false, 4096, true, false)
			_, pair := got.CompletedOwnedMethodPair()
			if tc.valid && (got.Failure != "" || !pair || string(got.Result) != "[]") {
				t.Fatalf("ASSERT_PRIVATE_OBSERVER_VALID_PAIR_CONTROL: failure=%s pair=%v", got.Failure, pair)
			}
			if !tc.valid && (got.Failure == "" || pair) {
				t.Fatal("ASSERT_PRIVATE_OBSERVER_MALFORMED_PAIR_CONTROL", got.Failure, pair)
			}
			if trace.entries != 1 || (tc.valid && trace.valid != 1) || (!tc.valid && trace.invalid != 1) {
				t.Fatalf("ASSERT_PRIVATE_MANAGER_OBSERVERS: valid=%v entry=%d decode=%d invalid=%d", tc.valid, trace.entries, trace.valid, trace.invalid)
			}
			if read != int64(len(wire)) {
				t.Fatalf("ASSERT_PRIVATE_OBSERVER_NO_NATIVE_PREFETCH: read=%d frame=%d", read, len(wire))
			}
		})
	}
}
