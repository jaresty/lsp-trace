package sessionruntime

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"reflect"
	"sync"
	"testing"
	"time"
	"unsafe"

	"lsp-trace/internal/lsp"
	"lsp-trace/internal/lspwire"
)

func context80Session(w *privateDiscoveryWindow, c *privateDiscoveryChild) *PrivateDiscoverySession {
	m := response76Runtime(w, c)
	return &PrivateDiscoverySession{client: privateDiscoveryClient{m: m, w: w, id: "inert", generation: 1}, members: []PrivateDiscoveryMember{{URI: "private-a"}, {URI: "private-b"}}, supplied: map[string]bool{"private-a": true, "private-b": true}}
}
func TestContext80TwoRequests(t *testing.T) {
	wire := append(discoveryWire(`{"jsonrpc":"2.0","id":1,"error":{"code":0,"message":"SERVER_SECRET"}}`), discoveryWire(`{"jsonrpc":"2.0","id":2,"result":[]}`)...)
	w, c, _ := discoveryWindow(wire)
	w.diagnosticAnchor = time.Now()
	w.diagnosticDeadline = w.diagnosticAnchor.Add(time.Second)
	if !w.prepayResponseFacts() {
		t.Fatal("prepay")
	}
	s := context80Session(w, c)
	w.initializedEncoding(privateInitializationEncoding{Present: true, Value: "utf-16"})
	// This is the document owner's existing hash, not a new production hash pass.
	a := sha256.Sum256([]byte("exact\r\n\U0001f600 supplied text\n"))
	b := sha256.Sum256([]byte("other exact text"))
	ownerContext := context.WithValue(w.ctx, privateDiscoveryContextKey{}, w)
	ra, rb := DocumentResult{Supply: &DocumentSupply{}}, DocumentResult{Supply: &DocumentSupply{}}
	privateDiscoverySupplyDigest(ownerContext, &ra, a)
	privateDiscoverySupplyDigest(ownerContext, &rb, b)
	// A later Manager map value must not replace either exact result hash.
	s.client.m.sessions["inert"].documents = map[string]openDocument{"private-a": {digest: [32]byte{99}}, "private-b": {digest: [32]byte{98}}}
	s.mu.Lock()
	s.captureSuppliedDigestLocked("private-a", ra)
	s.captureSuppliedDigestLocked("private-b", rb)
	s.mu.Unlock()
	// Line 1 after the emoji plus space: UTF16 offset 3, not UTF8 offset 5
	// or rune offset 2. The runtime must preserve this trusted coordinate.
	pos := lsp.Position{Line: 1, Character: 3}
	ctx := s.DiscoveryDiagnosticContext(context.Background(), 0, 41)
	_, e := s.PrepareCallHierarchy(ctx, lsp.PrepareCallHierarchyParams{TextDocument: lsp.TextDocumentIdentifier{URI: "private-a"}, Position: pos})
	if e == nil {
		t.Fatal("server error missing")
	}
	before := s.ResponseFacts()
	ctx = s.DiscoveryDiagnosticContext(context.Background(), 1, 59)
	_, e = s.PrepareCallHierarchy(ctx, lsp.PrepareCallHierarchyParams{TextDocument: lsp.TextDocumentIdentifier{URI: "private-b"}, Position: lsp.Position{Line: 0, Character: 6}})
	if e != nil {
		t.Fatal(e)
	}
	d := s.ResponseFacts()
	if !d.Valid() || d.Context.Encoding != PrivateEncodingUTF16 || d.Context.EncodingOrigin != PrivateEncodingAdvertised || d.Count != 2 || d.Records[0] != before.Records[0] {
		t.Fatal("ASSERT80_IMMUTABLE_REQUESTS")
	}
	for i, want := range []struct {
		file, target, line, char int64
		hash                     [32]byte
	}{{0, 41, 1, 3, a}, {1, 59, 0, 6, b}} {
		r := d.Records[i]
		m := r.Correlation
		if r.Ordinal != uint16(i+1) || m.Method != PrivateMethodPrepareCallHierarchy || !m.FileAvailable || m.FileOrdinal != want.file || !m.TargetAvailable || m.TargetOrdinal != want.target || !m.PositionAvailable || m.Line != want.line || m.Character != want.char || !m.ContentAvailable || m.ContentSHA256 != want.hash {
			t.Fatal("ASSERT80_CORRELATION", i, m)
		}
		if !r.Sent || !r.SentAt.Available || !r.LastRead.FrameComplete || !r.LastRead.DecodeAttempted || r.LastRead.Outcome != PrivateDecodeSucceeded || !r.SelectedRead.Decoded.Available || !r.Classification.Observed || !r.Classification.At.Available {
			t.Fatal("ASSERT80_ACTUAL_STAGES", i, r)
		}
		if r.SentAt.ElapsedNS <= 0 {
			t.Fatal("ASSERT80_ORIGINAL_ANCHOR", r.SentAt)
		}
		if r.SentAt.ElapsedNS > r.SelectedRead.Received.ElapsedNS || r.SelectedRead.Received.ElapsedNS > r.SelectedRead.Decoded.ElapsedNS || r.SelectedRead.Decoded.ElapsedNS > r.Classification.At.ElapsedNS {
			t.Fatal("ASSERT80_STAGE_ORDER", i)
		}
		for _, at := range []PrivateDiscoveryEventTime{r.SentAt, r.SelectedRead.Received, r.SelectedRead.Decoded, r.Classification.At} {
			if at.ElapsedNS+at.RemainingNS != int64(time.Second) {
				t.Fatal("ASSERT80_ORIGINAL_ANCHOR", at)
			}
		}
	}
	if d.Records[0].Classification.Status != PrivateClassificationFailure || d.Records[0].Classification.Cause != 3 || d.Records[0].Classification.ServerCodeCategory != 12 || d.Records[1].Classification.Status != PrivateClassificationSuccess {
		t.Fatal("ASSERT80_CLASSIFIED_FINAL_SOURCE_CAUSE")
	}
	raw, e := json.Marshal(d)
	if e != nil {
		t.Fatal(e)
	}
	for _, secret := range []string{"private-a", "private-b", "SERVER_SECRET", "supplied text", "other exact text"} {
		if bytes.Contains(raw, []byte(secret)) {
			t.Fatal("ASSERT80_NO_PAYLOAD", secret)
		}
	}
	old := append([]byte(nil), raw...)
	d.Records[0].Correlation.ContentSHA256[0] ^= 1
	again, _ := json.Marshal(s.ResponseFacts())
	if !bytes.Equal(old, again) {
		t.Fatal("ASSERT80_SNAPSHOT_SERIALIZATION_STABLE")
	}
}
func TestContext80UnknownAndSourceOwnership(t *testing.T) {
	w, c, _ := discoveryWindow(discoveryWire(`{"jsonrpc":"2.0","id":1,"result":[]}`))
	w.prepayResponseFacts()
	client := privateDiscoveryClient{m: response76Runtime(w, c), w: w, id: "inert", generation: 1}
	_, e := client.PrepareCallHierarchy(context.Background(), lsp.PrepareCallHierarchyParams{TextDocument: lsp.TextDocumentIdentifier{URI: "private-a"}, Position: lsp.Position{Character: 9}})
	if e != nil {
		t.Fatal(e)
	}
	r := w.responseFactsSnapshot().Records[0]
	if r.Correlation.FileAvailable || r.Correlation.TargetAvailable || r.Correlation.ContentAvailable || !r.Correlation.PositionAvailable || r.Correlation.Character != 9 || r.SentAt.Available || r.LastRead.Decoded.Available {
		t.Fatal("ASSERT80_UNKNOWN_NOT_ZERO_ASSERTION", r)
	}
	s := context80Session(w, c)
	other := context80Session(w, c)
	foreign := other.DiscoveryDiagnosticContext(context.Background(), 4, 99)
	ctx := s.diagnosticContext(foreign, "private-a")
	m := privateCorrelation(ctx, "textDocument/documentSymbol")
	if !m.FileAvailable || m.FileOrdinal != 0 || m.TargetAvailable {
		t.Fatal("ASSERT80_FOREIGN_CONTEXT_NOT_ADOPTED", m)
	}
	for _, bad := range [][2]int{{-1, 0}, {16, 0}, {0, -2}, {0, 65536}} {
		ctx := s.DiscoveryDiagnosticContext(context.Background(), bad[0], bad[1])
		m := privateCorrelation(s.diagnosticContext(ctx, "private-a"), "textDocument/documentSymbol")
		if m.TargetAvailable {
			t.Fatal("ASSERT80_INVALID_ORDINAL", bad)
		}
	}
}

type context80Child struct {
	*discoveryInertChild
	partial bool
	before  func()
}
type context80Writer struct{ c *context80Child }

func (x context80Writer) Write(p []byte) (int, error) {
	if x.c.before != nil {
		x.c.before()
	}
	if x.c.partial {
		return len(p) / 2, nil
	}
	return len(p), nil
}
func (context80Writer) Close() error            { return nil }
func (c *context80Child) Stdin() io.WriteCloser { return context80Writer{c} }
func TestContext80WriteAndDecodeBoundaries(t *testing.T) {
	for _, tc := range []struct {
		name, body                 string
		partial, complete, decoded bool
		out                        PrivateDiscoveryDecodeOutcome
	}{
		{"full", `{"jsonrpc":"2.0","id":1,"result":[]}`, false, true, true, PrivateDecodeSucceeded},
		{"partial", `{"jsonrpc":"2.0","id":1,"result":[]}`, true, false, false, PrivateDecodeNotAttempted},
		{"malformed-json", `{"jsonrpc":`, false, true, true, PrivateDecodeRejected},
		{"invalid-code", `{"jsonrpc":"2.0","id":1,"error":{"code":true}}`, false, true, true, PrivateDecodeRejected},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w, c, inert := discoveryWindow(discoveryWire(tc.body))
			w.prepayResponseFacts()
			w.diagnosticAnchor = time.Now()
			w.diagnosticDeadline = w.diagnosticAnchor.Add(time.Second)
			observedBefore := false
			c.wireChild = &context80Child{discoveryInertChild: inert, partial: tc.partial, before: func() {
				d := w.responseFactsSnapshot()
				if d.Records[0].Sent || d.Records[0].Classification.Observed {
					t.Error("ASSERT80_NOT_SENT_BEFORE_WRITE")
				}
				observedBefore = true
			}}
			client := privateDiscoveryClient{m: response76Runtime(w, c), w: w, id: "inert", generation: 1}
			client.PrepareCallHierarchy(context.Background(), lsp.PrepareCallHierarchyParams{TextDocument: lsp.TextDocumentIdentifier{URI: "private-a"}})
			r := w.responseFactsSnapshot().Records[0]
			if !observedBefore || r.Sent == tc.partial {
				t.Fatal("ASSERT80_SENT_ONLY_COMPLETE_WRITE", r.Sent)
			}
			if r.LastRead.FrameComplete != tc.complete || r.LastRead.DecodeAttempted != tc.decoded || r.LastRead.Outcome != tc.out {
				t.Fatal("ASSERT80_RECEIVED_DECODED_DISTINCT", r.LastRead)
			}
			if tc.out != PrivateDecodeSucceeded && r.SelectedRead != (PrivateDiscoveryReadStages{}) {
				t.Fatal("ASSERT80_REJECTED_NOT_SELECTED")
			}
			if !r.Classification.Observed {
				t.Fatal("ASSERT80_FINAL_CLASSIFICATION_MISSING")
			}
		})
	}
}
func TestContext80EncodingPresence(t *testing.T) {
	for _, tc := range []struct {
		raw string
		e   PrivateDiscoveryEncoding
		o   PrivateDiscoveryEncodingOrigin
	}{
		{`{}`, PrivateEncodingUTF16, PrivateEncodingProtocolDefault},
		{`{"positionEncoding":"utf-16"}`, PrivateEncodingUTF16, PrivateEncodingAdvertised},
		{`{"positionEncoding":"utf-8"}`, PrivateEncodingUTF8, PrivateEncodingAdvertised},
		{`{"positionEncoding":"utf-32"}`, PrivateEncodingUTF32, PrivateEncodingAdvertised},
		{`{"positionEncoding":"server-secret-encoding"}`, PrivateEncodingUnsupported, PrivateEncodingAdvertised},
		{`{"positionEncoding":""}`, PrivateEncodingUnsupported, PrivateEncodingAdvertised},
		{`{"positionEncoding":null}`, PrivateEncodingUnsupported, PrivateEncodingInvalidField},
	} {
		t.Run(tc.raw, func(t *testing.T) {
			var v struct {
				Encoding privateInitializationEncoding `json:"positionEncoding"`
			}
			if json.Unmarshal([]byte(tc.raw), &v) != nil {
				t.Fatal("decode")
			}
			w, _, _ := discoveryWindow(nil)
			w.prepayResponseFacts()
			w.initializedEncoding(v.Encoding)
			d := w.responseFactsSnapshot()
			if !d.Valid() || d.Context.Encoding != tc.e || d.Context.EncodingOrigin != tc.o {
				t.Fatal("ASSERT80_ENCODING_PRESENCE", d.Context)
			}
			raw, _ := json.Marshal(d)
			if bytes.Contains(raw, []byte("server-secret")) {
				t.Fatal("ASSERT80_ENCODING_CLOSED")
			}
		})
	}
	var v struct {
		Encoding privateInitializationEncoding `json:"positionEncoding"`
	}
	if json.Unmarshal([]byte(`{"positionEncoding":4}`), &v) == nil {
		t.Fatal("ASSERT80_LEGACY_INVALID_ENCODING_REJECTED")
	}
}
func TestContext80FingerprintsAndGrowth(t *testing.T) {
	valid := PrivateDiscoveryFingerprints{Preverified: true, VersionAvailable: true, Major: 0, Minor: 17, Patch: 2, BinaryAvailable: true, BinarySHA256: [32]byte{9}, WorkspaceAvailable: true, WorkspaceSHA256: [32]byte{8}, ConfigAvailable: true, ConfigSHA256: [32]byte{7}}
	if !valid.Valid() || !(PrivateDiscoveryFingerprints{}).Valid() {
		t.Fatal("fingerprint validation")
	}
	fixture, _, _ := discoveryWindow(nil)
	for _, f := range []PrivateDiscoveryFingerprints{{Major: 1}, {BinaryAvailable: true}, {Preverified: true, ConfigSHA256: [32]byte{1}}} {
		effects := 0
		_, e := RunPrivateDiscovery(context.Background(), PrivateDiscoveryConstruction{Fingerprints: f, Limits: fixture.limits, Deadline: time.Now().Add(time.Second), Members: []PrivateDiscoveryMember{{URI: "private-a"}}, NewStarter: func() (Starter, error) { effects++; return nil, errors.New("must not run") }}, func(context.Context, *PrivateDiscoverySession) error { effects++; return nil })
		if e == nil || effects != 0 {
			t.Fatal("ASSERT80_INVALID_FINGERPRINT_PRESTART")
		}
	}
	if privateResponseFactsCharge < 8*uint64(unsafe.Sizeof(PrivateDiscoveryResponseFacts{}))+uint64(16*1024+128*512) {
		t.Fatal("ASSERT80_COMPILER_SIZED_GROWTH_CHARGE")
	}
	for _, anchor := range []time.Time{time.Now().Round(0), time.Now().Add(time.Hour)} {
		effects := 0
		_, e := RunPrivateDiscovery(context.Background(), PrivateDiscoveryConstruction{DiagnosticAnchor: anchor, Limits: fixture.limits, Deadline: time.Now().Add(time.Second), Members: []PrivateDiscoveryMember{{URI: "private-a"}}, NewStarter: func() (Starter, error) { effects++; return nil, errors.New("inert") }}, func(context.Context, *PrivateDiscoverySession) error { effects++; return nil })
		if e == nil || effects != 0 {
			t.Fatal("ASSERT80_BAD_ANCHOR_PRESTART")
		}
	}
	for _, fp := range []PrivateDiscoveryFingerprints{{}, {Preverified: true, VersionAvailable: true, Major: 1, Minor: 2, Patch: 3, BinaryAvailable: true, BinarySHA256: [32]byte{0xb1}, WorkspaceAvailable: true, WorkspaceSHA256: [32]byte{0x48}, ConfigAvailable: true, ConfigSHA256: [32]byte{0xc9}}} {
		effects := 0
		observation, e := RunPrivateDiscovery(context.Background(), PrivateDiscoveryConstruction{Fingerprints: fp, Limits: fixture.limits, Deadline: time.Now().Add(time.Second), Members: []PrivateDiscoveryMember{{URI: "private-a"}}, NewStarter: func() (Starter, error) { effects++; return nil, errors.New("inert") }}, func(context.Context, *PrivateDiscoverySession) error { effects++; return nil })
		got := observation.ResponseFacts
		if e == nil || effects != 1 || !got.Available || got.Context.Fingerprints != fp || got.Context.Encoding != PrivateEncodingUnknown {
			t.Fatal("ASSERT80_CALLER_FINGERPRINT_ERROR_SNAPSHOT", got)
		}
		fp.BinarySHA256[0]++
		if got.Context.Fingerprints == fp {
			t.Fatal("ASSERT80_FINGERPRINT_OWNED_COPY")
		}
	}
	w, _, _ := discoveryWindow(nil)
	w.remaining.RetainedBytes = privateResponseFactsCharge - 1
	if w.prepayResponseFacts() || w.responseFacts != nil {
		t.Fatal("ASSERT80_GROWTH_PREPAID")
	}
	w, _, _ = discoveryWindow(nil)
	w.prepayResponseFacts()
	w.responseFacts.Context.Fingerprints = valid
	before := w.responseFactsSnapshot()
	remaining := w.remaining
	w.beginResponseFacts()
	if w.prepayResponseFacts() || w.responseFactsSnapshot().Count != 1 || w.remaining != remaining || before.Count != 0 {
		t.Fatal("ASSERT80_NO_COUNT_RESET")
	}
	after := w.responseFactsSnapshot()
	after.Context.Fingerprints.BinarySHA256[0] = 0
	if w.responseFactsSnapshot().Context.Fingerprints.BinarySHA256[0] != 9 {
		t.Fatal("ASSERT80_FINGERPRINT_COPY")
	}
	t.Logf("paid fixed store/copy/context=%d record=%d header=%d", privateResponseFactsCharge, unsafe.Sizeof(PrivateDiscoveryResponseRecord{}), unsafe.Sizeof(PrivateDiscoverySessionContext{}))
}
func TestContext80RemainingAndConcurrentSnapshots(t *testing.T) {
	anchor := time.Unix(100, 0)
	deadline := anchor.Add(time.Second)
	for _, elapsed := range []time.Duration{0, time.Millisecond, time.Second, 2 * time.Second} {
		e := privateEventTime(anchor, deadline, elapsed)
		if !e.Available || e.ElapsedNS != int64(elapsed) || e.RemainingNS != int64(max(time.Second-elapsed, 0)) || !e.valid() {
			t.Fatal("ASSERT80_REMAINING_DEADLINE", e)
		}
	}
	if privateEventTime(time.Time{}, deadline, 0).Available || privateEventTime(anchor, time.Time{}, 0).Available {
		t.Fatal("ASSERT80_ANCHOR_UNKNOWN")
	}
	w, c, _ := discoveryWindow(discoveryWire(`{"jsonrpc":"2.0","id":1,"result":[]}`))
	w.prepayResponseFacts()
	s := context80Session(w, c)
	var wg sync.WaitGroup
	done := make(chan struct{})
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-done:
				return
			default:
				if !s.ResponseFacts().Valid() {
					t.Error("ASSERT80_CONCURRENT_SNAPSHOT")
					return
				}
			}
		}
	}()
	_, e := s.PrepareCallHierarchy(s.DiscoveryDiagnosticContext(context.Background(), 0, 7), lsp.PrepareCallHierarchyParams{TextDocument: lsp.TextDocumentIdentifier{URI: "private-a"}})
	close(done)
	wg.Wait()
	if e != nil {
		t.Fatal(e)
	}
}
func TestContext80ConcurrentRequestAndCancellation(t *testing.T) {
	w, c, _ := discoveryWindow(nil)
	pipe, writer := io.Pipe()
	defer writer.Close()
	inert := &response76BlockingChild{in: pipe, started: make(chan struct{})}
	c.wireChild = inert
	w.diagnosticAnchor = time.Now()
	w.diagnosticDeadline = w.diagnosticAnchor.Add(time.Second)
	w.prepayResponseFacts()
	s := context80Session(w, c)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	w.ctx = ctx
	joined := make(chan error, 1)
	go func() {
		_, e := s.PrepareCallHierarchy(s.DiscoveryDiagnosticContext(ctx, 0, 41), lsp.PrepareCallHierarchyParams{TextDocument: lsp.TextDocumentIdentifier{URI: "private-a"}})
		joined <- e
	}()
	<-inert.started
	_, e := s.PrepareCallHierarchy(s.DiscoveryDiagnosticContext(context.Background(), 1, 59), lsp.PrepareCallHierarchyParams{TextDocument: lsp.TextDocumentIdentifier{URI: "private-b"}})
	before := s.ResponseFacts()
	if e == nil || before.Count != 1 || before.Records[0].Correlation.TargetOrdinal != 41 || before.Records[0].Classification.Observed {
		t.Fatal("ASSERT80_CONCURRENT_REQUEST_NOT_MIXED", before.Records[0])
	}
	cancel()
	if <-joined == nil {
		t.Fatal("cancel result")
	}
	r := s.ResponseFacts().Records[0]
	if !r.Sent || !r.SentAt.Available || !r.Classification.Observed || r.Classification.Cause != 1 || !r.Classification.At.Available || r.Classification.At.RemainingNS <= 0 || r.SelectedRead != (PrivateDiscoveryReadStages{}) || r.LastDisposition != PrivateResponseJoined {
		t.Fatal("ASSERT80_CANCEL_REMAINING_AND_NOT_SELECTED", r)
	}
	if before.Records[0].Classification.Observed {
		t.Fatal("ASSERT80_BEFORE_SNAPSHOT_CHANGED")
	}
}

func TestContext80DocumentOwnerSeams(t *testing.T) {
	file, e := parser.ParseFile(token.NewFileSet(), "sessionruntime.go", nil, 0)
	if e != nil {
		t.Fatal(e)
	}
	count := 0
	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		id, ok := call.Fun.(*ast.Ident)
		if !ok || id.Name != "privateDiscoverySupplyDigest" {
			return true
		}
		count++
		if len(call.Args) != 3 {
			t.Fatal("ASSERT80_SOURCE_OWNER_HANDOFF_SEAMS")
		}
		digest, ok := call.Args[2].(*ast.Ident)
		if !ok || digest.Name != "digest" {
			t.Fatal("ASSERT80_SOURCE_OWNER_HANDOFF_SEAMS")
		}
		return true
	})
	if count != 2 {
		t.Fatal("ASSERT80_SOURCE_OWNER_HANDOFF_SEAMS", count)
	}
	ordinary := DocumentResult{Supply: &DocumentSupply{}}
	privateDiscoverySupplyDigest(context.Background(), &ordinary, [32]byte{7})
	if ordinary.privateDiscoveryDigestAvailable || ordinary.privateDiscoveryDigest != ([32]byte{}) {
		t.Fatal("ASSERT80_ORDINARY_DOCUMENT_UNCHANGED")
	}
	w, c, _ := discoveryWindow(nil)
	s := context80Session(w, c)
	s.captureSuppliedDigestLocked("private-a", ordinary)
	if s.digestAvailable[0] {
		t.Fatal("ASSERT80_MISSING_DIGEST_UNKNOWN")
	}
}

func TestContext80StrictMetadata(t *testing.T) {
	base := PrivateDiscoveryResponseFacts{Available: true, Count: 1}
	base.Records[0].Ordinal = 1
	for name, mutate := range map[string]func(*PrivateDiscoveryResponseFacts){
		"method": func(d *PrivateDiscoveryResponseFacts) { d.Records[0].Correlation.Method = 255 },
		"file": func(d *PrivateDiscoveryResponseFacts) {
			d.Records[0].Correlation.FileAvailable = true
			d.Records[0].Correlation.FileOrdinal = 16
		},
		"target": func(d *PrivateDiscoveryResponseFacts) {
			d.Records[0].Correlation.TargetAvailable = true
			d.Records[0].Correlation.TargetOrdinal = 65536
		},
		"position": func(d *PrivateDiscoveryResponseFacts) {
			d.Records[0].Correlation.PositionAvailable = true
			d.Records[0].Correlation.Line = -1
		},
		"unknown-digest": func(d *PrivateDiscoveryResponseFacts) { d.Records[0].Correlation.ContentSHA256[0] = 1 },
		"unknown-time":   func(d *PrivateDiscoveryResponseFacts) { d.Records[0].SentAt.ElapsedNS = 1 },
		"outcome":        func(d *PrivateDiscoveryResponseFacts) { d.Records[0].LastRead.Outcome = 255 },
		"category": func(d *PrivateDiscoveryResponseFacts) {
			d.Records[0].Classification = PrivateDiscoveryClassification{Observed: true, Status: PrivateClassificationFailure, Cause: 1, ServerCodeCategory: 12}
		},
		"fingerprint": func(d *PrivateDiscoveryResponseFacts) { d.Context.Fingerprints.Major = 1 },
		"encoding":    func(d *PrivateDiscoveryResponseFacts) { d.Context.Encoding = 255 },
	} {
		t.Run(name, func(t *testing.T) {
			d := base
			mutate(&d)
			if _, e := json.Marshal(d); e == nil {
				t.Fatal("ASSERT80_INVALID_METADATA_REFUSED", name)
			}
		})
	}
}

func TestContext80UnmatchedNotSelected(t *testing.T) {
	wire := append(discoveryWire(`{"jsonrpc":"2.0","id":99,"result":[]}`), discoveryWire(`{"jsonrpc":"2.0","id":1,"result":[]}`)...)
	w, c, _ := discoveryWindow(wire)
	w.prepayResponseFacts()
	m := response76Runtime(w, c)
	r := m.RoundTrip(context.Background(), RoundTripRequest{SessionID: "inert", Generation: 1, Method: "textDocument/prepareCallHierarchy", Params: json.RawMessage(`{}`), MaxMessages: 16, MaxBytes: 1 << 20})
	d := w.responseFactsSnapshot()
	if r.Failure != "" || d.Records[0].ReadAttempts != 2 || d.Records[0].Selected.Association != lspwire.ResponseIDMatched || !d.Records[0].SelectedRead.FrameComplete || d.Records[0].Classification.Observed {
		t.Fatal("ASSERT80_UNMATCHED_AND_UNCLASSIFIED", d.Records[0])
	}
}

// Independent review81 guard retained without weakening its assertions.
func TestReview81InvalidEncoding(t *testing.T) {
	for _, raw := range []string{`{"positionEncoding":4}`, `{"positionEncoding":true}`, `{"positionEncoding":{}}`, `{"positionEncoding":[]}`} {
		var v struct {
			Encoding privateInitializationEncoding `json:"positionEncoding"`
		}
		err := json.Unmarshal([]byte(raw), &v)
		if err == nil {
			t.Fatal("REVIEW81_WRONGTYPE_REJECTED")
		}
		w, _, _ := discoveryWindow(nil)
		w.prepayResponseFacts()
		// Probe scalar handoff separately: production returns before this handoff on
		// Unmarshal error (sessionruntime.go:3024-3029), leaving actual snapshot unknown.
		w.initializedEncoding(v.Encoding)
		got := w.responseFactsSnapshot().Context
		if got.EncodingOrigin != PrivateEncodingInvalidField {
			t.Errorf("REVIEW81_WRONGTYPE_INVALID_ORIGIN raw=%s origin=%d want=%d", raw, got.EncodingOrigin, PrivateEncodingInvalidField)
		}
	}
}

// Structural connection proof only: no Manager readiness/initialization is run.
func TestRepair83InitializerConnection(t *testing.T) {
	f, e := parser.ParseFile(token.NewFileSet(), "sessionruntime.go", nil, 0)
	if e != nil {
		t.Fatal(e)
	}
	calls, refusals, owners := 0, 0, 0
	var callPos, refusalPos token.Pos
	ast.Inspect(f, func(n ast.Node) bool {
		if c, ok := n.(*ast.CallExpr); ok {
			if id, ok := c.Fun.(*ast.Ident); ok && id.Name == "decodePrivateInitialization" {
				calls++
				callPos = c.Pos()
				if len(c.Args) != 2 {
					t.Fatal("REPAIR83_BOUNDARY_ARGUMENTS")
				}
				a, ok := c.Args[0].(*ast.SelectorExpr)
				if !ok || a.Sel.Name != "Result" {
					t.Fatal("REPAIR83_BOUNDARY_ARGUMENTS")
				}
				b, ok := c.Args[1].(*ast.Ident)
				if !ok || b.Name != "initializationWindow" {
					t.Fatal("REPAIR83_BOUNDARY_ARGUMENTS")
				}
			}
		}
		if a, ok := n.(*ast.AssignStmt); ok && len(a.Lhs) == 1 && len(a.Rhs) == 1 {
			if id, ok := a.Lhs[0].(*ast.Ident); ok && id.Name == "initializationWindow" {
				if sel, ok := a.Rhs[0].(*ast.SelectorExpr); ok && sel.Sel.Name == "owner" {
					owners++
				}
			}
		}
		if s, ok := n.(*ast.IfStmt); ok {
			if b, ok := s.Cond.(*ast.BinaryExpr); ok && b.Op == token.NEQ {
				if id, ok := b.X.(*ast.Ident); ok && id.Name == "decodeErr" {
					message, returned := false, false
					ast.Inspect(s.Body, func(x ast.Node) bool {
						if l, ok := x.(*ast.BasicLit); ok && l.Value == "\"sessionruntime: malformed initialize result\"" {
							message = true
						}
						if _, ok := x.(*ast.ReturnStmt); ok {
							returned = true
						}
						return true
					})
					if message && returned {
						refusals++
						refusalPos = s.Pos()
					}
				}
			}
		}
		return true
	})
	if calls != 1 || refusals != 1 || owners != 1 || callPos >= refusalPos {
		t.Fatal("REPAIR83_BOUNDARY_CONNECTED_BEFORE_ORIGINAL_REFUSAL", calls, refusals, owners)
	}
	t.Log("REPAIR83_BOUNDARY_CONNECTED_BEFORE_ORIGINAL_REFUSAL PASS (structural only)")
}

// Direct decoding uses the original anonymous schema and actual field type.
// Baseline field-decoder error signatures are separately compared in an
// isolated overlay; a differently named stand-in changes Go error type text.
func repair83OriginalDecode(raw []byte) error {
	var original struct {
		ServerInfo struct {
			Name    string `json:"name"`
			Version string `json:"version"`
		} `json:"serverInfo"`
		Capabilities struct {
			PositionEncoding        privateInitializationEncoding `json:"positionEncoding"`
			CallHierarchyProvider   json.RawMessage               `json:"callHierarchyProvider"`
			DocumentSymbolProvider  json.RawMessage               `json:"documentSymbolProvider"`
			WorkspaceSymbolProvider json.RawMessage               `json:"workspaceSymbolProvider"`
			DefinitionProvider      json.RawMessage               `json:"definitionProvider"`
			ReferencesProvider      json.RawMessage               `json:"referencesProvider"`
		} `json:"capabilities"`
	}
	return json.Unmarshal(raw, &original)
}

type repair83InitializationCase struct {
	name, raw string
	reject    bool
	encoding  PrivateDiscoveryEncoding
	origin    PrivateDiscoveryEncodingOrigin
}

func repair83InitializationCases() []repair83InitializationCase {
	return []repair83InitializationCase{
		{"missing", `{}`, false, PrivateEncodingUTF16, PrivateEncodingProtocolDefault},
		{"root-null", `null`, false, PrivateEncodingUTF16, PrivateEncodingProtocolDefault},
		{"capabilities-null", `{"capabilities":null}`, false, PrivateEncodingUTF16, PrivateEncodingProtocolDefault},
		{"null", `{"capabilities":{"positionEncoding":null}}`, false, PrivateEncodingUnsupported, PrivateEncodingInvalidField},
		{"utf8", `{"capabilities":{"positionEncoding":"utf-8"}}`, false, PrivateEncodingUTF8, PrivateEncodingAdvertised},
		{"utf16", `{"capabilities":{"positionEncoding":"utf-16"}}`, false, PrivateEncodingUTF16, PrivateEncodingAdvertised},
		{"utf32", `{"capabilities":{"positionEncoding":"utf-32"}}`, false, PrivateEncodingUTF32, PrivateEncodingAdvertised},
		{"unsupported", `{"serverInfo":{"name":"SERVER_SECRET83","version":"VERSION_SECRET83"},"capabilities":{"positionEncoding":"ENCODING_SECRET83"}}`, false, PrivateEncodingUnsupported, PrivateEncodingAdvertised},
		{"empty-string", `{"capabilities":{"positionEncoding":""}}`, false, PrivateEncodingUnsupported, PrivateEncodingAdvertised},
		{"number", `{"capabilities":{"positionEncoding":4}}`, true, PrivateEncodingUnsupported, PrivateEncodingInvalidField},
		{"boolean", `{"capabilities":{"positionEncoding":true}}`, true, PrivateEncodingUnsupported, PrivateEncodingInvalidField},
		{"object", `{"capabilities":{"positionEncoding":{}}}`, true, PrivateEncodingUnsupported, PrivateEncodingInvalidField},
		{"array", `{"capabilities":{"positionEncoding":[]}}`, true, PrivateEncodingUnsupported, PrivateEncodingInvalidField},
		{"unrelated-after", `{"capabilities":{"positionEncoding":"utf-8"},"serverInfo":{"version":4}}`, true, PrivateEncodingUTF8, PrivateEncodingAdvertised},
		{"unrelated-before", `{"serverInfo":{"name":false},"capabilities":{"positionEncoding":"utf-32"}}`, true, PrivateEncodingUTF32, PrivateEncodingAdvertised},
		{"missing-with-error", `{"serverInfo":{"version":4}}`, true, PrivateEncodingUnknown, PrivateEncodingOriginUnknown},
		{"bad-capabilities", `{"capabilities":true}`, true, PrivateEncodingUnknown, PrivateEncodingOriginUnknown},
		{"bad-root", `[]`, true, PrivateEncodingUnknown, PrivateEncodingOriginUnknown},
		{"syntax-after-field", `{"capabilities":{"positionEncoding":"utf-8"},`, true, PrivateEncodingUnknown, PrivateEncodingOriginUnknown},
		{"syntax-in-field", `{"capabilities":{"positionEncoding":{bad}}}`, true, PrivateEncodingUnknown, PrivateEncodingOriginUnknown},
		{"empty-input", ``, true, PrivateEncodingUnknown, PrivateEncodingOriginUnknown},
		{"duplicate-final-invalid", `{"capabilities":{"positionEncoding":"utf-8","positionEncoding":4}}`, true, PrivateEncodingUnsupported, PrivateEncodingInvalidField},
		{"aborted-before-later-field", `{"capabilities":{"positionEncoding":4,"positionEncoding":"utf-8"}}`, true, PrivateEncodingUnsupported, PrivateEncodingInvalidField},
	}
}
func TestRepair83OriginalErrorSignatures(t *testing.T) {
	var signatures []string
	for _, tc := range repair83InitializationCases() {
		_, err := decodePrivateInitialization([]byte(tc.raw), nil)
		if (err != nil) != tc.reject {
			t.Fatal("REPAIR83_ORIGINAL_REFUSAL", tc.name)
		}
		signature := tc.name + ":accepted"
		if err != nil {
			signature = tc.name + ":" + reflect.TypeOf(err).String() + ":" + err.Error()
		}
		signatures = append(signatures, signature)
	}
	raw, _ := json.Marshal(signatures)
	t.Logf("REPAIR83_ORIGINAL_ERROR_SIGNATURES %x", sha256.Sum256(raw))
}
func TestRepair83PureInitializationBoundary(t *testing.T) {
	for _, tc := range repair83InitializationCases() {
		t.Run(tc.name, func(t *testing.T) {
			w, _, _ := discoveryWindow(nil)
			if !w.prepayResponseFacts() {
				t.Fatal("prepay")
			}
			initialized, err := decodePrivateInitialization([]byte(tc.raw), w)
			originalErr := repair83OriginalDecode([]byte(tc.raw))
			without, withoutErr := decodePrivateInitialization([]byte(tc.raw), nil)
			if (err != nil) != tc.reject || !reflect.DeepEqual(err, originalErr) || !reflect.DeepEqual(err, withoutErr) || !reflect.DeepEqual(initialized, without) {
				t.Fatal("REPAIR83_ORIGINAL_REFUSAL", err, originalErr)
			}
			got := w.responseFactsSnapshot()
			if !got.Valid() || got.Context.Encoding != tc.encoding || got.Context.EncodingOrigin != tc.origin {
				t.Fatal("REPAIR83_PROVENANCE", got.Context, tc.encoding, tc.origin)
			}
			raw, e := json.Marshal(got)
			if e != nil {
				t.Fatal(e)
			}
			for _, s := range []string{"SERVER_SECRET83", "VERSION_SECRET83", "ENCODING_SECRET83"} {
				if bytes.Contains(raw, []byte(s)) {
					t.Fatal("REPAIR83_SNAPSHOT_PRIVACY")
				}
			}
			t.Log("REPAIR83_PROVENANCE PASS; REPAIR83_ORIGINAL_REFUSAL PASS; REPAIR83_SNAPSHOT_PRIVACY PASS")
		})
	}
}
