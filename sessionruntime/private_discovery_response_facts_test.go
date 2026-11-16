package sessionruntime

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"lsp-trace/internal/discoveryfailure"
	"lsp-trace/internal/lsp"
	"lsp-trace/internal/lspwire"
	"lsp-trace/internal/managedprocess"
	"lsp-trace/internal/session"
	"reflect"
	"sync"
	"testing"
	"time"
)

func TestResponse76OwnedFailureAndSnapshots(t *testing.T) {
	w, c, _ := discoveryWindow(discoveryWire(`{"jsonrpc":"2.0","id":7,"error":{"code":1e0,"message":"PRIVATE_SENTINEL"}}`))
	if !w.prepayResponseFacts() {
		t.Fatal("P4 prepay")
	}
	i, ok := w.beginResponseFacts()
	if !ok {
		t.Fatal("P4 slot")
	}
	f, e := w.frameRecord(c, 7)
	if e == nil || f == nil || f.facts.CodeState != lspwire.ResponseCodeNotInteger || f.message.Error != nil || f.raw != nil {
		t.Fatal("P1 failed decode facts lost", e)
	}
	w.observeResponseFacts(i, f, PrivateResponseDecodeRejected)
	d := w.responseFactsSnapshot()
	if !d.Valid() || d.Count != 1 || d.Records[0].LastFrame.CodeState != lspwire.ResponseCodeNotInteger || d.Records[0].Selected.State != lspwire.ResponseNotObserved {
		t.Fatal("P2 rejected frame selected or lost", d.Records[0])
	}
	f.facts.CodeState = lspwire.ResponseCodeAbsent
	if d.Records[0].LastFrame.CodeState != lspwire.ResponseCodeNotInteger {
		t.Fatal("P2 borrowed facts")
	}
	if _, e := json.Marshal(d); e != nil {
		t.Fatal("P5 marshal", e)
	}
	j, ok := w.beginResponseFacts()
	if !ok || j != 1 || w.responseFactsSnapshot().Records[j].LastFrame.State != lspwire.ResponseNotObserved {
		t.Fatal("P2 stale second request")
	}
}
func TestResponse76Association(t *testing.T) {
	wire := append(discoveryWire(`{"jsonrpc":"2.0","id":8,"result":[]}`), discoveryWire(`{"jsonrpc":"2.0","id":7,"error":{"code":0}}`)...)
	w, c, _ := discoveryWindow(wire)
	if !w.prepayResponseFacts() {
		t.Fatal("prepay")
	}
	i, _ := w.beginResponseFacts()
	f, e := w.frameRecord(c, 7)
	if e != nil {
		t.Fatal(e)
	}
	w.observeResponseFacts(i, f, PrivateResponseBoundary)
	w.observeResponseFacts(i, f, PrivateResponseUnmatched)
	if w.responseFactsSnapshot().Records[i].Selected.State != lspwire.ResponseNotObserved {
		t.Fatal("P2 unmatched selected")
	}
	f, e = w.frameRecord(c, 7)
	if e != nil {
		t.Fatal(e)
	}
	w.observeResponseFacts(i, f, PrivateResponseBoundary)
	w.observeResponseFacts(i, f, PrivateResponseSelected)
	d := w.responseFactsSnapshot()
	if !d.Valid() || d.Records[i].ReadAttempts != 2 || d.Records[i].Selected.CodeState != lspwire.ResponseCodeValid || d.Records[i].Selected.Code != 0 {
		t.Fatal("P2 selected zero lost", d.Records[i])
	}
	s := &PrivateDiscoverySession{client: privateDiscoveryClient{w: w}, closed: true}
	copy := s.ResponseFacts()
	copy.Records[i].Selected.Code = 9
	if s.ResponseFacts().Records[i].Selected.Code != 0 {
		t.Fatal("P2 session copy aliases")
	}
}
func TestResponse76PrepayAndBound(t *testing.T) {
	w, c, inert := discoveryWindow(discoveryWire(`{"jsonrpc":"2.0","id":7,"result":null}`))
	w.remaining.RetainedBytes = privateResponseFactsCharge - 1
	if w.prepayResponseFacts() || w.responseFacts != nil || inert.reads != 0 {
		t.Fatal("P4 allocated/read before admission")
	}
	w, c, inert = discoveryWindow(discoveryWire(`{"jsonrpc":"2.0","id":7,"result":null}`))
	if !w.prepayResponseFacts() {
		t.Fatal("prepay")
	}
	before := w.remaining
	for i := 0; i < PrivateDiscoveryResponseCapacity; i++ {
		if _, ok := w.beginResponseFacts(); !ok {
			t.Fatal("P2 premature cap")
		}
	}
	if _, ok := w.beginResponseFacts(); ok || !w.responseFactsSnapshot().Truncated || w.remaining != before || inert.reads != 0 {
		t.Fatal("P2 cap effects")
	}
	_ = c
	w, c, inert = discoveryWindow(discoveryWire(`{"jsonrpc":"2.0","id":7,"result":null}`))
	w.remaining.RetainedBytes = 0
	if f, e := w.frameRecord(c, 7); e == nil || f != nil || inert.reads != 0 {
		t.Fatal("P4 fixed frame slot not prepaid")
	}
}

// Direct in-memory transport fixture only: the Ready enum below is local
// protocol state, not kernel READY evidence, a reservation, or a grant. There is
// no Starter, New/Start, Initialize, source supply, provider, or native process.
func response76Runtime(w *privateDiscoveryWindow, c *privateDiscoveryChild) *Manager {
	r := &runtimeSession{record: Record{SessionID: "inert", Generation: 1, State: session.Ready}, process: c, pending: lspwire.NewPending(128), requests: make(map[lspwire.RequestKey]*Request)}
	return &Manager{limits: Limits{MaxRequests: 128, MaxChildren: 2, MaxObservations: 64}, wire: lspwire.Limits{MaxBodyBytes: 1 << 20, MaxHeaderBytes: 8192}, sessions: map[string]*runtimeSession{"inert": r}, now: time.Now, workerDone: make(chan struct{}, 1)}
}
func TestResponse76ActualRoundTrip(t *testing.T) {
	for _, tc := range []struct {
		name, wire string
		bad        bool
	}{
		{"zero", `{"jsonrpc":"2.0","id":1,"error":{"code":0}}`, false},
		{"decode", `{"jsonrpc":"2.0","id":1,"error":{"code":true}}`, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w, c, _ := discoveryWindow(discoveryWire(tc.wire))
			if !w.prepayResponseFacts() {
				t.Fatal("prepay")
			}
			m := response76Runtime(w, c)
			r := m.RoundTrip(context.Background(), RoundTripRequest{SessionID: "inert", Generation: 1, Method: "textDocument/prepareCallHierarchy", Params: json.RawMessage(`{}`), MaxMessages: 16, MaxBytes: 1 << 20})
			d := w.responseFactsSnapshot()
			if !d.Valid() || d.Count != 1 {
				t.Fatal("P2 actual RoundTrip missing record")
			}
			if tc.bad {
				if r.Failure == "" || r.ServerError != nil || d.Records[0].LastDisposition != PrivateResponseDecodeRejected || d.Records[0].LastFrame.CodeState != lspwire.ResponseCodeWrongType || d.Records[0].Selected.State != lspwire.ResponseNotObserved {
					t.Fatal("P2 actual rejected decode handoff", r.Failure, d.Records[0])
				}
			} else {
				if r.Failure != "" || r.ServerError == nil || d.Records[0].Selected.CodeState != lspwire.ResponseCodeValid || d.Records[0].Selected.Code != 0 {
					t.Fatal("P2 actual selected response", r.Failure, d.Records[0])
				}
				cause, category := discoveryfailure.DetailsFromError(privateDiscoveryPrepareRPCCodeFailure(r.Failure, r.ServerError))
				if cause != discoveryfailure.RPCServer || category != discoveryfailure.ServerCodeOther {
					t.Fatal("P3 zero classification changed")
				}
			}
		})
	}
	wire := append(discoveryWire(`{"jsonrpc":"2.0","id":9,"error":{"code":4}}`), discoveryWire(`{"jsonrpc":"2.0","id":1,"result":[]}`)...)
	w, c, _ := discoveryWindow(wire)
	if !w.prepayResponseFacts() {
		t.Fatal("prepay")
	}
	m := response76Runtime(w, c)
	r := m.RoundTrip(context.Background(), RoundTripRequest{SessionID: "inert", Generation: 1, Method: "textDocument/prepareCallHierarchy", Params: json.RawMessage(`{}`), MaxMessages: 16, MaxBytes: 1 << 20})
	d := w.responseFactsSnapshot()
	if r.Failure != "" || d.Records[0].ReadAttempts != 2 || d.Records[0].Selected.Error != lspwire.ResponseAbsent || d.Records[0].Selected.Result != lspwire.ResponseArray {
		t.Fatal("P2 unmatched frame selected", r.Failure, d.Records[0])
	}
}
func TestResponse76ActualClientNoStaleFacts(t *testing.T) {
	wire := append(discoveryWire(`{"jsonrpc":"2.0","id":1,"error":{"code":0}}`), discoveryWire(`{"jsonrpc":"2.0","id":2,"result":[]}`)...)
	w, c, _ := discoveryWindow(wire)
	if !w.prepayResponseFacts() {
		t.Fatal("prepay")
	}
	m := response76Runtime(w, c)
	client := privateDiscoveryClient{m: m, w: w, id: "inert", generation: 1}
	_, e := client.PrepareCallHierarchy(context.Background(), lsp.PrepareCallHierarchyParams{TextDocument: lsp.TextDocumentIdentifier{URI: "fixture"}})
	cause, category := discoveryfailure.DetailsFromError(e)
	if cause != discoveryfailure.RPCServer || category != discoveryfailure.ServerCodeOther {
		t.Fatal("P3 actual client zero category")
	}
	frozen := w.responseFactsSnapshot()
	_, e = client.PrepareCallHierarchy(context.Background(), lsp.PrepareCallHierarchyParams{TextDocument: lsp.TextDocumentIdentifier{URI: "fixture"}})
	d := w.responseFactsSnapshot()
	if e != nil || d.Count != 2 || d.Records[0] != frozen.Records[0] || d.Records[1].Selected.Error != lspwire.ResponseAbsent || d.Records[1].Selected.CodeState != lspwire.ResponseCodeAbsent {
		t.Fatal("P2 actual client cross-request mutation")
	}
}

type response76BlockingChild struct {
	in      *io.PipeReader
	started chan struct{}
	once    sync.Once
}

func (c *response76BlockingChild) Stdin() io.WriteCloser { return discoveryDiscard{} }

type response76BlockingReader struct{ c *response76BlockingChild }

func (r response76BlockingReader) Read(p []byte) (int, error) { return r.c.Read(p) }
func (r response76BlockingReader) Close() error               { return r.c.in.Close() }
func (c *response76BlockingChild) Stdout() io.ReadCloser      { return response76BlockingReader{c} }
func (c *response76BlockingChild) Read(p []byte) (int, error) {
	c.once.Do(func() { close(c.started) })
	return c.in.Read(p)
}
func (c *response76BlockingChild) Close() managedprocess.ResourceObservation {
	c.in.Close()
	return managedprocess.ResourceObservation{}
}
func (c *response76BlockingChild) Teardown(context.Context) managedprocess.TeardownObservation {
	c.in.Close()
	return managedprocess.TeardownObservation{}
}
func TestResponse76CancelJoin(t *testing.T) {
	w, c, _ := discoveryWindow(nil)
	pipe, write := io.Pipe()
	defer write.Close()
	inert := &response76BlockingChild{in: pipe, started: make(chan struct{})}
	c.wireChild = inert
	if !w.prepayResponseFacts() {
		t.Fatal("prepay")
	}
	m := response76Runtime(w, c)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { <-inert.started; cancel() }()
	r := m.RoundTrip(ctx, RoundTripRequest{SessionID: "inert", Generation: 1, Method: "textDocument/prepareCallHierarchy", Params: json.RawMessage(`{}`), MaxMessages: 16, MaxBytes: 1 << 20})
	d := w.responseFactsSnapshot()
	if r.Failure != session.RequestCancelled || !d.Valid() || d.Count != 1 || d.Records[0].LastDisposition != PrivateResponseJoined || d.Records[0].Selected.State != lspwire.ResponseNotObserved || d.Records[0].LastFrame.State != lspwire.ResponseNotObserved {
		t.Fatal("P2 cancellation join invented selected response", r.Failure, d.Records[0])
	}
}
func TestResponse76RunnerRefusesBeforeStarter(t *testing.T) {
	w, _, _ := discoveryWindow(nil)
	before := w.remaining.RetainedBytes
	if !w.prepayBackend() {
		t.Fatal("backend quote")
	}
	limits := w.limits
	limits.RetainedBytes = (4 << 20) + (before - w.remaining.RetainedBytes) + privateResponseFactsCharge - 1
	effects := 0
	observation, e := RunPrivateDiscovery(context.Background(), PrivateDiscoveryConstruction{Limits: limits, Deadline: time.Now().Add(time.Second), Members: []PrivateDiscoveryMember{{URI: "fixture"}}, NewStarter: func() (Starter, error) { effects++; return nil, ErrPrivateDiscovery }}, func(context.Context, *PrivateDiscoverySession) error { effects++; return nil })
	if e == nil || effects != 0 || observation.ResponseFacts.Available {
		t.Fatal("P4 runner effect before record admission")
	}
}

type response76LateReader struct {
	r        *bytes.Reader
	pending  *lspwire.Pending
	w        *privateDiscoveryWindow
	observed *PrivateDiscoveryResponseFacts
	once     sync.Once
}

func (r *response76LateReader) Read(p []byte) (int, error) {
	r.once.Do(func() { r.pending.Accept(lspwire.ResponseKey{Generation: 1, ID: 1}) })
	if r.r.Len() == 0 {
		*r.observed = r.w.responseFactsSnapshot()
	}
	return r.r.Read(p)
}
func (*response76LateReader) Close() error { return nil }
func TestResponse76LateIsNotSelected(t *testing.T) {
	wire := discoveryWire(`{"jsonrpc":"2.0","id":1,"error":{"code":0}}`)
	w, c, inert := discoveryWindow(nil)
	if !w.prepayResponseFacts() {
		t.Fatal("prepay")
	}
	m := response76Runtime(w, c)
	var observed PrivateDiscoveryResponseFacts
	inert.in = &response76LateReader{r: bytes.NewReader(wire), pending: m.sessions["inert"].pending, w: w, observed: &observed}
	r := m.RoundTrip(context.Background(), RoundTripRequest{SessionID: "inert", Generation: 1, Method: "textDocument/prepareCallHierarchy", Params: json.RawMessage(`{}`), MaxMessages: 16, MaxBytes: 1 << 20})
	if r.Failure != session.SessionCrashed || !observed.Valid() || observed.Count != 1 || observed.Records[0].LastDisposition != PrivateResponseLate || observed.Records[0].LastFrame.CodeState != lspwire.ResponseCodeValid || observed.Records[0].Selected.State != lspwire.ResponseNotObserved {
		t.Fatal("P2 late frame promoted or lost", r.Failure, observed.Records[0])
	}
}
func TestResponse76ConcurrentSnapshots(t *testing.T) {
	var wire []byte
	for id := 1; id <= 32; id++ {
		wire = append(wire, discoveryWire(fmt.Sprintf(`{"jsonrpc":"2.0","id":%d,"result":[]}`, id))...)
	}
	w, c, _ := discoveryWindow(wire)
	if !w.prepayResponseFacts() {
		t.Fatal("prepay")
	}
	m := response76Runtime(w, c)
	done, joined := make(chan struct{}), make(chan struct{})
	go func() {
		defer close(joined)
		for {
			select {
			case <-done:
				return
			default:
				if !w.responseFactsSnapshot().Valid() {
					t.Error("P2 concurrent torn snapshot")
					return
				}
			}
		}
	}()
	defer func() { close(done); <-joined }()
	for id := 1; id <= 32; id++ {
		r := m.RoundTrip(context.Background(), RoundTripRequest{SessionID: "inert", Generation: 1, Method: "textDocument/prepareCallHierarchy", Params: json.RawMessage(`{}`), MaxMessages: 16, MaxBytes: 1 << 20})
		if r.Failure != "" {
			t.Fatal("P2 concurrent fixture request failed")
		}
	}
	if w.responseFactsSnapshot().Count != 32 {
		t.Fatal("P2 concurrent record count")
	}
}
func TestResponse76NoPayloadShape(t *testing.T) {
	var inspect func(reflect.Type)
	inspect = func(typ reflect.Type) {
		switch typ.Kind() {
		case reflect.Struct:
			for i := 0; i < typ.NumField(); i++ {
				inspect(typ.Field(i).Type)
			}
		case reflect.Array:
			inspect(typ.Elem())
		case reflect.Bool, reflect.Uint8, reflect.Uint16, reflect.Int64:
		default:
			t.Fatal("P5 non-fixed payload shape", typ.Kind())
		}
	}
	inspect(reflect.TypeOf(PrivateDiscoveryResponseFacts{}))
}
func TestResponse76UnavailableAndStrict(t *testing.T) {
	if !((PrivateDiscoveryResponseFacts{}).Valid()) {
		t.Fatal("unavailable")
	}
	for _, d := range []PrivateDiscoveryResponseFacts{{Count: 129}, {Count: 1}, {Available: true, Count: 1}, {Truncated: true}} {
		if _, e := json.Marshal(d); e == nil {
			t.Fatal("P5 invalid handoff accepted")
		}
	}
	w, _, _ := discoveryWindow(nil)
	w.ctx = context.Background()
	if got := w.responseFactsSnapshot(); got.Available {
		t.Fatal("P2 unavailable invented")
	}
}
