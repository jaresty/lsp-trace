//go:build darwin || linux

package inventorysuccessor

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"lsp-trace/internal/publication"
)

func acquisitionFixtureV1(t *testing.T, pins [2]localAcquisitionPinV1, under int) (*PrivateManagedInputOwnerV2, PrivateLocalAcquisitionRequestV1, context.Context) {
	t.Helper()
	root, re := publication.OpenRoot(acquisitionProtectedFixtureV1(t))
	if re != nil {
		t.Fatal(re)
	}
	t.Cleanup(func() { _ = root.Close() })
	j, a, _ := fixtureV2(t)
	l := verificationLimitsV2{400, 4 << 20, 400}
	c, e := localAcquisitionQuoteV1(pins, l)
	if e != nil {
		t.Fatal(e)
	}
	c.ActiveNS = uint64(3 * time.Minute)
	if under >= 0 {
		v := c.vector()
		if v[under] == 0 {
			t.Fatalf("ASSERT_ACQUISITION_QUOTE_COORDINATE: zero %d", under)
		}
		v[under]--
		c = countersFromV2(v)
	}
	a.Ceiling = c
	a.MaxTranche = c
	a.MaxWindows = 1
	a.MaxWindowNS = uint64(3 * time.Minute)
	a.Expires = time.Now().Add(3 * time.Minute).UnixNano()
	jb, ab := sealTestV2(t, &j), sealTestV2(t, &a)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	t.Cleanup(cancel)
	init, e := InitializePrivateReadyV2(ctx, root, jb, ab, PrivateReadyDecisionAssertionV2{j.Identity, a.Identity, a.Decision}, PrivateManagedInputVerificationV2{400, 4 << 20, 400})
	if e != nil {
		t.Fatal(e)
	}
	o, e := OpenPrivateManagedInputOwnerV2(ctx, root, init.CanonicalJob(), init.ConfirmedHead(), PrivateManagedInputVerificationV2{400, 4 << 20, 400})
	if e != nil {
		t.Fatal(e)
	}
	storage := acquisitionProtectedFixtureV1(t)
	if e = os.Chmod(storage, 0700); e != nil {
		t.Fatal(e)
	}
	q := PrivateLocalAcquisitionRequestV1{storage, hashLabelV2(t.Name()), o.head, time.Now().Add(45 * time.Second)}
	return o, q, ctx
}
func acquisitionTinyPinsV1(url string) [2]localAcquisitionPinV1 {
	var p [2]localAcquisitionPinV1
	for i, s := range []string{"tiny public model fixture", "tiny public archive fixture"} {
		p[i] = localAcquisitionPinV1{fmt.Sprintf("%s/%d", url, i), fmt.Sprintf("fixture-%d", i), fmt.Sprintf("sha256:%x", sha256.Sum256([]byte(s))), uint64(len(s))}
	}
	return p
}
func TestPrivateLocalAcquisitionConnected(t *testing.T) {
	var o *PrivateManagedInputOwnerV2
	var q PrivateLocalAcquisitionRequestV1
	var requests atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		h, e := o.k.reopen(r.Context(), o.job, o.limits)
		if e != nil || h.Disposition != "PENDING" || h.Attempt != q.Attempt {
			t.Error("ASSERT_ACQUISITION_RESERVE_BEFORE_IO: no actual pending custody")
		}
		body := "tiny public model fixture"
		if r.URL.Path == "/1" {
			body = "tiny public archive fixture"
		}
		w.Header().Set("Content-Length", fmt.Sprint(len(body)))
		_, _ = w.Write([]byte(body))
	}))
	defer server.Close()
	pins := acquisitionTinyPinsV1(server.URL)
	var ctx context.Context
	o, q, ctx = acquisitionFixtureV1(t, pins, -1)
	// The old managed-source API cannot authorize this declaration; no Manager/source
	// custody is forged to make a network acquisition look like a source capture.
	if _, e := o.Reserve(ctx, hashLabelV2("not-a-source"), PrivateManagedInputBindingV2{Mode: "ACQUIRE", Deadline: q.Deadline}); e == nil {
		t.Fatal("ASSERT_ACQUISITION_OLD_CONTRACT_REFUSES")
	}
	got, e := o.acquireLocalCandidateV1(ctx, q, pins)
	if e != nil || got.state == nil {
		t.Fatalf("ASSERT_ACQUISITION_CONNECTED: state=%v err=%v", got.state, e)
	}
	if !got.state.budget.stopping {
		t.Fatal("ASSERT_REPAIR_FINAL_ADMISSION_CLOSED")
	}
	if requests.Load() != 2 {
		t.Fatal("ASSERT_ACQUISITION_ATTEMPTS: not exactly two requests")
	}
	h, e := o.k.reopen(ctx, o.job, o.limits)
	if e != nil || h.Disposition != "PENDING" || h.Windows != 1 {
		t.Fatal("ASSERT_ACQUISITION_PENDING_RETENTION", e)
	}
	quote, e := localAcquisitionQuoteV1(pins, o.limits)
	if e != nil {
		t.Fatal(e)
	}
	quote.ActiveNS = h.Charged.ActiveNS
	if h.Charged != quote {
		t.Fatal("ASSERT_ACQUISITION_EXACT_DEBIT")
	}
	paths, pe := got.Paths()
	if pe != nil {
		t.Fatal("ASSERT_ACQUISITION_PIN_IDENTITY", pe)
	}
	for i, p := range pins {
		path := filepath.Join(q.StorageRoot, "candidate-"+q.Attempt[7:], p.Name)
		if paths[i] != path {
			t.Fatal("ASSERT_ACQUISITION_PATH_CUSTODY", paths[i], path)
		}
		b, e := os.ReadFile(path)
		if e != nil || uint64(len(b)) != p.Bytes || fmt.Sprintf("sha256:%x", sha256.Sum256(b)) != p.SHA256 {
			t.Fatal("ASSERT_ACQUISITION_PIN_IDENTITY", e)
		}
		s, _ := os.Lstat(path)
		if s.Mode().Perm() != 0600 {
			t.Fatal("ASSERT_ACQUISITION_PRIVATE_MODE")
		}
	}
	copy := got
	if e = got.Retire(ctx); e != nil {
		t.Fatal("ASSERT_ACQUISITION_RETIRE", e)
	}
	if e = copy.Retire(ctx); e == nil || o.uncertain {
		t.Fatal("ASSERT_ACQUISITION_NO_REPLAY")
	}
	if _, e = os.Lstat(filepath.Join(q.StorageRoot, "candidate-"+q.Attempt[7:])); !os.IsNotExist(e) {
		t.Fatal("ASSERT_ACQUISITION_FIXED_DELETE", e)
	}
	after, e := o.k.reopen(ctx, o.job, o.limits)
	if e != nil || after.Disposition != "OBSERVED" || after.Charged != h.Charged || after.Windows != h.Windows {
		t.Fatal("ASSERT_ACQUISITION_NO_REFUND", e)
	}
	t.Log("ASSERT_ACQUISITION_CONNECTED A-pass; reserve-before-IO; exact pins/debit; PENDING retention; fixed cleanup; copied replay refused")
}
func TestPrivateLocalAcquisitionRefusalBeforeIO(t *testing.T) {
	var calls atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		http.Error(w, "unfunded must not reach server", 500)
	}))
	defer server.Close()
	pins := acquisitionTinyPinsV1(server.URL)
	archive := acquisitionArchiveFixtureV1(t, 2, 0)
	pins[1].Bytes = uint64(len(archive))
	pins[1].SHA256 = fmt.Sprintf("sha256:%x", sha256.Sum256(archive))
	for _, i := range []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13} {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			o, q, ctx := acquisitionFixtureV1(t, pins, i)
			before := o.head
			got, e := o.acquireLocalCandidateV1(ctx, q, pins)
			entries, re := os.ReadDir(q.StorageRoot)
			if e == nil || got.state != nil || calls.Load() != 0 || re != nil || len(entries) != 0 || o.head != before {
				t.Fatal("ASSERT_ACQUISITION_UNFUNDED_NO_IO")
			}
		})
	}
	if !t.Failed() {
		t.Log("ASSERT_ACQUISITION_UNFUNDED_NO_IO A-pass")
	}
}
func TestPrivateLocalAcquisitionMalformed(t *testing.T) {
	for _, kind := range []string{"hash", "length", "redirect", "status", "short", "interim"} {
		t.Run(kind, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				body := "tiny public model fixture"
				if r.URL.Path == "/1" {
					body = "tiny public archive fixture"
				}
				switch kind {
				case "interim":
					w.WriteHeader(http.StatusEarlyHints)
				case "hash":
					body = strings.Replace(body, "public", "PUBLIC", 1)
				case "length":
					body += "x"
				case "redirect":
					w.Header().Set("Location", "http://unapproved.invalid/")
					w.WriteHeader(302)
					return
				case "status":
					w.WriteHeader(503)
					return
				case "short":
					w.Header().Set("Content-Length", fmt.Sprint(len(body)))
					_, _ = w.Write([]byte(body[:2]))
					return
				}
				w.Header().Set("Content-Length", fmt.Sprint(len(body)))
				_, _ = w.Write([]byte(body))
			}))
			defer server.Close()
			pins := acquisitionTinyPinsV1(server.URL)
			o, q, ctx := acquisitionFixtureV1(t, pins, -1)
			got, e := o.acquireLocalCandidateV1(ctx, q, pins)
			if e == nil || got.state != nil {
				t.Fatal("ASSERT_ACQUISITION_BAD_CONTENT_REFUSED")
			}
			h, e := o.k.reopen(ctx, o.job, o.limits)
			if e != nil || h.Disposition != "FAILED" || h.Windows != 1 || h.Charged == (countersV2{}) {
				t.Fatal("ASSERT_ACQUISITION_FAILED_DEBIT_RETAINED", e)
			}
			entries, e := os.ReadDir(q.StorageRoot)
			if e != nil || len(entries) != 0 {
				t.Fatal("ASSERT_ACQUISITION_FAILURE_CLEANUP", e)
			}
		})
	}
	if !t.Failed() {
		t.Log("ASSERT_ACQUISITION_BAD_CONTENT_REFUSED A-pass")
	}
}

func acquisitionLiveFixtureV1(t *testing.T) (*PrivateManagedInputOwnerV2, PrivateLocalAcquisitionRequestV1, context.Context, context.CancelFunc, PrivateLocalAcquisitionV1) {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := "tiny public model fixture"
		if r.URL.Path == "/1" {
			body = "tiny public archive fixture"
		}
		w.Header().Set("Content-Length", fmt.Sprint(len(body)))
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(server.Close)
	pins := acquisitionTinyPinsV1(server.URL)
	o, q, base := acquisitionFixtureV1(t, pins, -1)
	ctx, cancel := context.WithCancel(base)
	t.Cleanup(cancel)
	got, e := o.acquireLocalCandidateV1(ctx, q, pins)
	if e != nil {
		t.Fatal("ASSERT_ACQUISITION_FIXTURE", e)
	}
	return o, q, ctx, cancel, got
}
func TestPrivateLocalAcquisitionOriginalContext(t *testing.T) {
	for _, kind := range []string{"cancelled", "replacement", "expired"} {
		t.Run(kind, func(t *testing.T) {
			o, q, ctx, cancel, got := acquisitionLiveFixtureV1(t)
			before, e := o.k.reopen(ctx, o.job, o.limits)
			if e != nil {
				t.Fatal(e)
			}
			use := ctx
			switch kind {
			case "cancelled":
				cancel()
			case "replacement":
				use = context.Background()
			case "expired":
				got.state.deadline = time.Now().Add(-time.Nanosecond)
			}
			if e = got.Retire(use); e == nil {
				t.Fatal("ASSERT_ACQUISITION_ORIGINAL_CONTEXT")
			}
			// Test-only readback uses the fixture's still-live base context, not execution
			// credit or a production replacement clock; the cancelled job stays PENDING.
			inspect, done := context.WithTimeout(context.Background(), time.Second)
			defer done()
			after, e := o.k.reopen(inspect, o.job, o.limits)
			if e != nil || after.Disposition != "PENDING" || after.Charged != before.Charged || !o.uncertain {
				t.Fatal("ASSERT_ACQUISITION_ORIGINAL_CONTEXT", e)
			}
			if _, e = got.Paths(); e == nil {
				t.Fatal("ASSERT_ACQUISITION_CONSUMED_CONTEXT")
			}
			if e = got.Retire(ctx); e == nil {
				t.Fatal("ASSERT_ACQUISITION_CONSUMED_CONTEXT")
			}
			if _, e = os.Lstat(filepath.Join(q.StorageRoot, "candidate-"+q.Attempt[7:])); !os.IsNotExist(e) {
				t.Fatal("ASSERT_ACQUISITION_CANCEL_CLOSE_FIXED_CLEANUP", e)
			}
			if got.state.root != nil || got.state.dir != nil {
				t.Fatal("ASSERT_ACQUISITION_CANCEL_CLOSE_FIXED_CLEANUP")
			}
		})
	}
	if !t.Failed() {
		t.Log("ASSERT_ACQUISITION_ORIGINAL_CONTEXT A-pass: cancelled/replacement/expired fail without renewed execution or refund; fixed owned cleanup attempted")
	}
}
func TestPrivateLocalAcquisitionCleanupUncertain(t *testing.T) {
	o, q, ctx, _, got := acquisitionLiveFixtureV1(t)
	dir := filepath.Join(q.StorageRoot, "candidate-"+q.Attempt[7:])
	foreign := filepath.Join(dir, "unrelated-fixture")
	if e := os.WriteFile(foreign, []byte("do not delete unknown names"), 0600); e != nil {
		t.Fatal(e)
	}
	if e := got.Retire(ctx); e == nil {
		t.Fatal("ASSERT_ACQUISITION_UNCERTAIN_CLEANUP")
	}
	h, e := o.k.reopen(ctx, o.job, o.limits)
	if e != nil || h.Disposition != "PENDING" || !o.uncertain || h.Windows != 1 {
		t.Fatal("ASSERT_ACQUISITION_UNCERTAIN_CLEANUP", e)
	}
	b, e := os.ReadFile(foreign)
	if e != nil || string(b) != "do not delete unknown names" {
		t.Fatal("ASSERT_ACQUISITION_UNKNOWN_NOT_DELETED", e)
	}
	if e = got.Retire(ctx); e == nil {
		t.Fatal("ASSERT_ACQUISITION_UNCERTAIN_NO_REPLAY")
	}
	if got.state.root != nil || got.state.dir != nil {
		t.Fatal("ASSERT_ACQUISITION_UNCERTAIN_FDS_CLOSED")
	}
	// Fixed test-owned disposal only, NOT a product recovery/resume interface.
	if e = os.Remove(foreign); e != nil {
		t.Fatal(e)
	}
	if e = os.Remove(dir); e != nil {
		t.Fatal(e)
	}
	t.Log("ASSERT_ACQUISITION_UNCERTAIN_CLEANUP A-pass: PENDING/debit retained; unknown file preserved; no retry; owned descriptors closed")
}
func TestPrivateLocalAcquisitionOpaqueHandles(t *testing.T) {
	var empty PrivateLocalAcquisitionV1
	if _, e := empty.Paths(); e == nil {
		t.Fatal("ASSERT_ACQUISITION_OPAQUE_HANDLE")
	}
	if e := empty.Retire(context.Background()); e == nil {
		t.Fatal("ASSERT_ACQUISITION_OPAQUE_HANDLE")
	}
	_, _, ctx, _, got := acquisitionLiveFixtureV1(t)
	b, e := json.Marshal(got)
	if e != nil || string(b) != "{}" {
		t.Fatal("ASSERT_ACQUISITION_OPAQUE_HANDLE", e)
	}
	var forged PrivateLocalAcquisitionV1
	if e = json.Unmarshal(b, &forged); e != nil {
		t.Fatal(e)
	}
	if _, e = forged.Paths(); e == nil {
		t.Fatal("ASSERT_ACQUISITION_OPAQUE_HANDLE")
	}
	if e = forged.Retire(ctx); e == nil {
		t.Fatal("ASSERT_ACQUISITION_OPAQUE_HANDLE")
	}
	if e = got.Retire(ctx); e != nil {
		t.Fatal(e)
	}
	t.Log("ASSERT_ACQUISITION_OPAQUE_HANDLE A-pass: zero and DTO-round-trip cannot gain custody")
}

type acquisitionCountingConnV1 struct {
	lastDeadline time.Time
	deadlines    atomic.Int64
	net.Conn
	reads, writes, closes atomic.Int64
	closeErr              error
}

func (c *acquisitionCountingConnV1) Read(p []byte) (int, error)  { c.reads.Add(1); return len(p), nil }
func (c *acquisitionCountingConnV1) Write(p []byte) (int, error) { c.writes.Add(1); return len(p), nil }
func (c *acquisitionCountingConnV1) Close() error                { c.closes.Add(1); return c.closeErr }
func (c *acquisitionCountingConnV1) SetDeadline(t time.Time) error {
	c.deadlines.Add(1)
	c.lastDeadline = t
	return nil
}
func (c *acquisitionCountingConnV1) SetReadDeadline(t time.Time) error  { return c.SetDeadline(t) }
func (c *acquisitionCountingConnV1) SetWriteDeadline(t time.Time) error { return c.SetDeadline(t) }
func TestPrivateLocalAcquisitionSocketDeadline(t *testing.T) {
	original := time.Unix(1234, 0)
	for _, kind := range []int{0, 1, 2} {
		for _, requested := range []time.Time{{}, original.Add(time.Hour), original.Add(-time.Second)} {
			b := newAcquisitionBudgetV1(acquisitionTinyPinsV1("http://127.0.0.1"))
			b.metadata = 1
			raw := &acquisitionCountingConnV1{}
			c := &acquisitionConnV1{Conn: raw, b: b, done: make(chan struct{}), deadline: original}
			want := requested
			if want.IsZero() || want.After(original) {
				want = original
			}
			var e error
			switch kind {
			case 0:
				e = c.SetDeadline(requested)
			case 1:
				e = c.SetReadDeadline(requested)
			case 2:
				e = c.SetWriteDeadline(requested)
			}
			if e != nil || !raw.lastDeadline.Equal(want) || raw.deadlines.Load() != 1 {
				t.Fatal("ASSERT_ACQUISITION_SOCKET_ORIGINAL_DEADLINE", e)
			}
			if e = c.SetDeadline(time.Time{}); e == nil || raw.deadlines.Load() != 1 {
				t.Fatal("ASSERT_ACQUISITION_SOCKET_ORIGINAL_DEADLINE")
			}
			if e = c.Close(); e != nil {
				t.Fatal(e)
			}
		}
	}
	t.Log("ASSERT_ACQUISITION_SOCKET_ORIGINAL_DEADLINE A-pass: all three deadline methods prepaid; zero/later never extend original")
}
func TestPrivateLocalAcquisitionAttemptBudget(t *testing.T) {
	for _, kind := range []string{"read", "write", "call-exhaustion", "wire-exhaustion"} {
		t.Run(kind, func(t *testing.T) {
			b := newAcquisitionBudgetV1(acquisitionTinyPinsV1("http://127.0.0.1"))
			b.wire = 3
			b.rawCalls = 1
			raw := &acquisitionCountingConnV1{}
			c := &acquisitionConnV1{Conn: raw, b: b, done: make(chan struct{})}
			if kind == "call-exhaustion" {
				b.rawCalls = 0
			}
			if kind == "wire-exhaustion" {
				b.wire = 0
			}
			var n int
			var e error
			if kind == "write" {
				n, e = c.Write(make([]byte, 9))
			} else {
				n, e = c.Read(make([]byte, 9))
			}
			if kind == "read" || kind == "write" {
				if n != 3 || e != nil || b.wire != 0 || b.rawCalls != 0 {
					t.Fatal("ASSERT_ACQUISITION_PREDEBIT_ATTEMPTS", n, e)
				}
			} else if n != 0 || e == nil || raw.reads.Load() != 0 || raw.writes.Load() != 0 {
				t.Fatal("ASSERT_ACQUISITION_PREDEBIT_ATTEMPTS", n, e)
			}
			before := raw.reads.Load() + raw.writes.Load()
			if _, e = c.Read(make([]byte, 1)); e == nil || raw.reads.Load()+raw.writes.Load() != before {
				t.Fatal("ASSERT_ACQUISITION_PREDEBIT_ATTEMPTS")
			}
			if e = c.Close(); e != nil {
				t.Fatal(e)
			}
			if e = c.Close(); e != nil || raw.closes.Load() != 1 {
				t.Fatal("ASSERT_ACQUISITION_ONE_ACTUAL_CLOSE", e)
			}
		})
	}
	// A zero-progress body is never silently retried; the ordinary Go reader
	// contract is tested separately from the counted raw-connection boundary.
	if !t.Failed() {
		t.Log("ASSERT_ACQUISITION_PREDEBIT_ATTEMPTS A-pass: capacities charged before read/write; exhausted attempt/byte envelope blocks actual methods")
	}
}
func TestPrivateLocalAcquisitionConnectionJoin(t *testing.T) {
	for _, bad := range []bool{false, true} {
		t.Run(fmt.Sprint(bad), func(t *testing.T) {
			b := newAcquisitionBudgetV1(acquisitionTinyPinsV1("http://127.0.0.1"))
			raw := &acquisitionCountingConnV1{}
			if bad {
				raw.closeErr = io.ErrClosedPipe
			}
			c := &acquisitionConnV1{Conn: raw, b: b, done: make(chan struct{})}
			b.connections = []*acquisitionConnV1{c}
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			s := &localAcquisitionStateV1{ctx: ctx, deadline: time.Now().Add(time.Second / 2), budget: b}
			ce := c.Close()
			e := s.joinConnections()
			if (e != nil) != bad || (ce != nil) != bad || raw.closes.Load() != 1 {
				t.Fatal("ASSERT_ACQUISITION_JOIN_CLOSE_TRUTH", e, ce)
			}
		})
	}
	if !t.Failed() {
		t.Log("ASSERT_ACQUISITION_JOIN_CLOSE_TRUTH A-pass: actual close error survives join")
	}
}
func TestPrivateLocalAcquisitionQuote(t *testing.T) {
	limits := PrivateManagedInputVerificationV2{400, 4 << 20, 400}
	got, e := QuotePrivateLocalCandidateAcquisitionV1(limits)
	if e != nil {
		t.Fatal(e)
	}
	// Independently evaluated fixed production operands, not a caller-selected
	// length or a second call into the production quote implementation.
	want := [14]uint64{2708173697710, 2698651043954, 4694787358, 654153816, 2689251787833, 8, 16, 2708828997805, 180000000000, 655300095, 131072, 4694459678, 9396783676, 2684102290718}
	if !reflect.DeepEqual(got, want) {
		t.Fatal("ASSERT_ACQUISITION_CHECKED_QUOTE", got, want)
	}
	for i, n := range got {
		if n == 0 {
			t.Fatal("ASSERT_ACQUISITION_CHECKED_QUOTE", i)
		}
	}
	for _, bad := range []PrivateManagedInputVerificationV2{{}, {400, ^uint64(0), 400}} {
		if _, e = QuotePrivateLocalCandidateAcquisitionV1(bad); e == nil {
			t.Fatal("ASSERT_ACQUISITION_CHECKED_QUOTE")
		}
	}
	pins := localAcquisitionPinsV1()
	pins[0].Bytes = ^uint64(0)
	if _, e = localAcquisitionQuoteV1(pins, verificationLimitsV2{400, 4 << 20, 400}); e == nil {
		t.Fatal("ASSERT_ACQUISITION_CHECKED_QUOTE")
	}
	maxPins := localAcquisitionPinsV1()
	maxPins[0].Bytes = 5 << 30
	maxPins[1].Bytes = 16 << 20
	maxQuote, me := localAcquisitionQuoteV1(maxPins, verificationLimitsV2{4096, 64 << 20, 16384})
	if me != nil {
		t.Fatal("ASSERT_ACQUISITION_CHECKED_QUOTE", me)
	}
	for i, n := range maxQuote.vector() {
		if n == 0 || n >= 1<<48 {
			t.Fatal("ASSERT_ACQUISITION_CHECKED_QUOTE: bounded arithmetic", i, n)
		}
	}
	for _, bad := range []verificationLimitsV2{{4097, 64 << 20, 16384}, {4096, (64 << 20) + 1, 16384}, {4096, 64 << 20, 16385}} {
		if _, e := localAcquisitionQuoteV1(maxPins, bad); e == nil {
			t.Fatal("ASSERT_ACQUISITION_CHECKED_QUOTE: invalid limits")
		}
	}
	maxPins[1].Bytes++
	if _, e := localAcquisitionQuoteV1(maxPins, verificationLimitsV2{4096, 64 << 20, 16384}); e == nil {
		t.Fatal("ASSERT_ACQUISITION_CHECKED_QUOTE: native input cap")
	}
	t.Log("ASSERT_ACQUISITION_CHECKED_QUOTE A-pass", got)
}

type acquisitionLateReadConnV1 struct {
	net.Conn
	started, release chan struct{}
}

func (c *acquisitionLateReadConnV1) Read([]byte) (int, error) {
	close(c.started)
	<-c.release
	return 0, io.ErrClosedPipe
}
func (c *acquisitionLateReadConnV1) Close() error { return nil }
func TestPrivateLocalAcquisitionJoinInFlightRead(t *testing.T) {
	b := newAcquisitionBudgetV1(acquisitionTinyPinsV1("http://127.0.0.1"))
	raw := &acquisitionLateReadConnV1{started: make(chan struct{}), release: make(chan struct{})}
	c := &acquisitionConnV1{Conn: raw, b: b, done: make(chan struct{})}
	b.connections = []*acquisitionConnV1{c}
	readDone := make(chan struct{})
	go func() { _, _ = c.Read(make([]byte, 1)); close(readDone) }()
	<-raw.started
	released := false
	defer func() {
		if !released {
			close(raw.release)
		}
		<-readDone
	}()
	if e := c.Close(); e != nil {
		t.Fatal(e)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	s := &localAcquisitionStateV1{ctx: ctx, deadline: time.Now().Add(time.Second / 2), budget: b}
	joined := make(chan error, 1)
	go func() { joined <- s.joinConnections() }()
	select {
	case e := <-joined:
		t.Fatal("ASSERT_ACQUISITION_JOIN_INFLIGHT_READ: returned before actual method", e)
	case <-time.After(30 * time.Millisecond):
	}
	close(raw.release)
	released = true
	<-readDone
	select {
	case e := <-joined:
		if e != nil {
			t.Fatal("ASSERT_ACQUISITION_JOIN_INFLIGHT_READ", e)
		}
	case <-ctx.Done():
		t.Fatal("ASSERT_ACQUISITION_JOIN_INFLIGHT_READ: did not finish")
	}
	t.Log("ASSERT_ACQUISITION_JOIN_INFLIGHT_READ A-pass: closure AND actual method return joined")
}

type acquisitionDialPauseV1 struct{ before, after, release chan struct{} }

func (p *acquisitionDialPauseV1) beforeDialAdmission() {
	if p.before != nil {
		close(p.before)
		<-p.release
	}
}
func (p *acquisitionDialPauseV1) afterDialAdmission() {
	if p.after != nil {
		close(p.after)
		<-p.release
	}
}
func TestPrivateLocalAcquisitionDialAdmission(t *testing.T) {
	for _, phase := range []string{"before", "after", "final"} {
		t.Run(phase, func(t *testing.T) {
			listener, e := net.Listen("tcp", "127.0.0.1:0")
			if e != nil {
				t.Fatal(e)
			}
			defer listener.Close()
			_ = listener.(*net.TCPListener).SetDeadline(time.Now().Add(time.Second))
			accepted := make(chan bool, 1)
			go func() {
				c, e := listener.Accept()
				if e == nil {
					_ = c.Close()
				}
				accepted <- e == nil
			}()
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			b := newAcquisitionBudgetV1(acquisitionTinyPinsV1("http://127.0.0.1"))
			s := &localAcquisitionStateV1{ctx: ctx, deadline: time.Now().Add(time.Second), budget: b}
			pause := &acquisitionDialPauseV1{release: make(chan struct{})}
			if phase == "before" {
				pause.before = make(chan struct{})
			}
			if phase == "after" {
				pause.after = make(chan struct{})
			}
			s.dialObserver = pause
			finished := make(chan error, 1)
			if phase == "final" {
				if e = s.stopConnections(); e != nil {
					t.Fatal(e)
				}
				_, e = s.dialContext(ctx, "tcp", listener.Addr().String())
				if e == nil {
					t.Fatal("ASSERT_REPAIR_DIAL_CLOSED")
				}
				b.mu.Lock()
				remaining := b.dials
				b.mu.Unlock()
				if remaining != 8 {
					t.Fatal("ASSERT_REPAIR_DIAL_CLOSED")
				}
				t.Log("ASSERT_REPAIR_DIAL_CLOSED A-pass")
				return
			}
			go func() {
				c, e := s.dialContext(ctx, "tcp", listener.Addr().String())
				if c != nil {
					_ = c.Close()
				}
				finished <- e
			}()
			if phase == "before" {
				<-pause.before
				if e = s.stopConnections(); e != nil {
					t.Fatal(e)
				}
				close(pause.release)
				<-finished
				// A real late DialContext successfully connects even though the old wrapper
				// subsequently returns an error. The listener makes that distinction visible.
				_ = listener.(*net.TCPListener).SetDeadline(time.Now().Add(40 * time.Millisecond))
				if <-accepted {
					t.Fatal("ASSERT_REPAIR_DIAL_NO_POST_STOP")
				}
				t.Log("ASSERT_REPAIR_DIAL_NO_POST_STOP A-pass")
			} else {
				<-pause.after
				joined := make(chan error, 1)
				go func() { joined <- s.stopConnections() }()
				select {
				case <-joined:
					t.Fatal("ASSERT_REPAIR_DIAL_REGISTERED_JOIN")
				case <-time.After(20 * time.Millisecond):
				}
				close(pause.release)
				<-finished
				if e = <-joined; e != nil {
					t.Fatal("ASSERT_REPAIR_DIAL_REGISTERED_JOIN", e)
				}
				if !<-accepted {
					t.Fatal("ASSERT_REPAIR_DIAL_REGISTERED_JOIN: actual loopback dial not exercised")
				}
				t.Log("ASSERT_REPAIR_DIAL_REGISTERED_JOIN A-pass")
			}
		})
	}
}

func acquisitionProtectedFixtureV1(t *testing.T) string {
	t.Helper()
	cwd, e := os.Getwd()
	if e != nil {
		t.Fatal(e)
	}
	p, e := os.MkdirTemp(cwd, ".acquisition-fixture-")
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { _ = os.RemoveAll(p) })
	return p
}
func TestPrivateLocalAcquisitionRootRefusal(t *testing.T) {
	for _, kind := range []string{"writable-ancestor", "alias-ancestor"} {
		t.Run(kind, func(t *testing.T) {
			var calls atomic.Int64
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls.Add(1); w.WriteHeader(503) }))
			defer server.Close()
			pins := acquisitionTinyPinsV1(server.URL)
			o, q, ctx := acquisitionFixtureV1(t, pins, -1)
			base := acquisitionProtectedFixtureV1(t)
			if kind == "writable-ancestor" {
				parent := filepath.Join(base, "parent")
				if e := os.Mkdir(parent, 0770); e != nil {
					t.Fatal(e)
				}
				if e := os.Chmod(parent, 0770); e != nil {
					t.Fatal(e)
				}
				q.StorageRoot = filepath.Join(parent, "storage")
				if e := os.Mkdir(q.StorageRoot, 0700); e != nil {
					t.Fatal(e)
				}
			} else {
				nested := filepath.Join(o.k.root.Path(), "fixture-storage")
				if e := os.Mkdir(nested, 0700); e != nil {
					t.Fatal(e)
				}
				alias := filepath.Join(base, "accounting-alias")
				if e := os.Symlink(o.k.root.Path(), alias); e != nil {
					t.Fatal(e)
				}
				q.StorageRoot = filepath.Join(alias, "fixture-storage")
			}
			got, e := o.acquireLocalCandidateV1(ctx, q, pins)
			if e == nil || got.state != nil || calls.Load() != 0 {
				t.Fatal("ASSERT_REPAIR_ROOT_BEFORE_CONTENT", kind, e, calls.Load())
			}
		})
	}
	if !t.Failed() {
		t.Log("ASSERT_REPAIR_ROOT_BEFORE_CONTENT A-pass")
	}
}

type acquisitionInterruptedNamespaceV1 struct {
	localOSIOV1
	calls atomic.Int64
}

func (x *acquisitionInterruptedNamespaceV1) namespaceFstat(uintptr, *syscall.Stat_t) error {
	x.calls.Add(1)
	return syscall.EINTR
}
func TestPrivateLocalAcquisitionRootContinuity(t *testing.T) {
	for _, kind := range []string{"permission", "single-EINTR"} {
		t.Run(kind, func(t *testing.T) {
			o, q, ctx, _, got := acquisitionLiveFixtureV1(t)
			var injected *acquisitionInterruptedNamespaceV1
			if kind == "permission" {
				if e := os.Chmod(q.StorageRoot, 0770); e != nil {
					t.Fatal(e)
				}
			} else {
				injected = &acquisitionInterruptedNamespaceV1{}
				got.state.namespaceIO = injected
			}
			e := got.Retire(ctx)
			if e == nil {
				t.Fatal("ASSERT_REPAIR_ROOT_CONTINUITY")
			}
			if injected != nil && injected.calls.Load() != 1 {
				t.Fatal("ASSERT_REPAIR_NAMESPACE_SINGLE_ATTEMPT", injected.calls.Load())
			}
			h, e := o.k.reopen(ctx, o.job, o.limits)
			if e != nil || h.Disposition != "PENDING" || !o.uncertain {
				t.Fatal("ASSERT_REPAIR_ROOT_CONTINUITY", e)
			}
		})
	}
	if !t.Failed() {
		t.Log("ASSERT_REPAIR_ROOT_CONTINUITY A-pass")
		t.Log("ASSERT_REPAIR_NAMESPACE_SINGLE_ATTEMPT A-pass")
	}
}

func acquisitionArchiveFixtureV1(t *testing.T, count int, size int64) []byte {
	t.Helper()
	var tarBytes bytes.Buffer
	tw := tar.NewWriter(&tarBytes)
	for i := 0; i < count; i++ {
		body := []byte("controlled native member, NOT executable")
		n := int64(len(body))
		if size > 0 {
			n = size
		}
		if e := tw.WriteHeader(&tar.Header{Name: fmt.Sprintf("libfixture-%d.dylib", i), Mode: 0600, Size: n, Format: tar.FormatUSTAR}); e != nil {
			t.Fatal(e)
		}
		if size > 0 {
			break
		}
		if _, e := tw.Write(body); e != nil {
			t.Fatal(e)
		}
	}
	if size == 0 {
		if e := tw.Close(); e != nil {
			t.Fatal(e)
		}
	}
	var out bytes.Buffer
	gw := gzip.NewWriter(&out)
	if _, e := gw.Write(tarBytes.Bytes()); e != nil {
		t.Fatal(e)
	}
	if e := gw.Close(); e != nil {
		t.Fatal(e)
	}
	return out.Bytes()
}
func acquisitionArchiveOwnerV1(t *testing.T, archive []byte) (*PrivateManagedInputOwnerV2, context.Context, context.CancelFunc, PrivateLocalAcquisitionV1) {
	t.Helper()
	bodies := [2][]byte{[]byte("tiny public model fixture"), archive}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		i := 0
		if r.URL.Path == "/1" {
			i = 1
		}
		w.Header().Set("Content-Length", fmt.Sprint(len(bodies[i])))
		_, _ = w.Write(bodies[i])
	}))
	t.Cleanup(server.Close)
	pins := acquisitionTinyPinsV1(server.URL)
	for i := range pins {
		pins[i].Bytes = uint64(len(bodies[i]))
		pins[i].SHA256 = fmt.Sprintf("sha256:%x", sha256.Sum256(bodies[i]))
	}
	o, q, base := acquisitionFixtureV1(t, pins, -1)
	ctx, cancel := context.WithCancel(base)
	t.Cleanup(cancel)
	got, e := o.acquireLocalCandidateV1(ctx, q, pins)
	if e != nil {
		t.Fatal("ASSERT_REPAIR_ARCHIVE_ACQUIRE", e)
	}
	return o, ctx, cancel, got
}
func TestPrivateLocalAcquisitionPaidArchive(t *testing.T) {
	o, ctx, _, got := acquisitionArchiveOwnerV1(t, acquisitionArchiveFixtureV1(t, 2, 0))
	lease, e := got.BeginNativeArchiveInspectionV1(ctx)
	if e != nil {
		t.Fatal("ASSERT_REPAIR_PAID_ARCHIVE_CONNECTED", e)
	}
	h, e := o.k.reopen(ctx, o.job, o.limits)
	if e != nil || h.Disposition != "PENDING" {
		t.Fatal("ASSERT_REPAIR_PAID_ARCHIVE_CONNECTED", e)
	}
	if e = got.Retire(ctx); e == nil {
		t.Fatal("ASSERT_REPAIR_ACTIVE_USE_BARRIER")
	}
	report, e := lease.Inspect(ctx)
	if e != nil || len(report.Members) != 2 || report.ArchiveSHA256 != got.state.pins[1].SHA256 || report.ExpandedBytes == 0 {
		t.Fatal("ASSERT_REPAIR_PAID_ARCHIVE_CONNECTED", report, e)
	}
	if _, e = lease.Inspect(ctx); e == nil {
		t.Fatal("ASSERT_REPAIR_INSPECTION_ONESHOT")
	}
	copy := lease
	got.state.budget.mu.Lock()
	readsBeforeClose := got.state.budget.archiveReads
	got.state.budget.mu.Unlock()
	if e = lease.Close(); e != nil {
		t.Fatal(e)
	}
	if e = copy.Close(); e == nil {
		t.Fatal("ASSERT_REPAIR_INSPECTION_ONESHOT")
	}
	if _, e = copy.Inspect(ctx); e == nil {
		t.Fatal("ASSERT_REPAIR_NO_CLOSED_DEBIT_READ")
	}
	if e = got.Retire(ctx); e != nil {
		t.Fatal("ASSERT_REPAIR_ACTIVE_USE_BARRIER", e)
	}
	after, e := o.k.reopen(ctx, o.job, o.limits)
	if e != nil || after.Disposition != "OBSERVED" || after.Charged != h.Charged || after.Windows != h.Windows {
		t.Fatal("ASSERT_REPAIR_NO_CLOSED_DEBIT_READ", e)
	}
	if _, e = copy.Inspect(ctx); e == nil {
		t.Fatal("ASSERT_REPAIR_NO_CLOSED_DEBIT_READ")
	}
	if _, e = got.BeginNativeArchiveInspectionV1(ctx); e == nil {
		t.Fatal("ASSERT_REPAIR_NO_CLOSED_DEBIT_READ")
	}
	got.state.budget.mu.Lock()
	readsAfterClose := got.state.budget.archiveReads
	got.state.budget.mu.Unlock()
	if readsAfterClose != readsBeforeClose {
		t.Fatal("ASSERT_REPAIR_NO_CLOSED_DEBIT_READ")
	}
	t.Log("ASSERT_REPAIR_PAID_ARCHIVE_CONNECTED A-pass")
	t.Log("ASSERT_REPAIR_ACTIVE_USE_BARRIER A-pass")
	t.Log("ASSERT_REPAIR_NO_CLOSED_DEBIT_READ A-pass")
	t.Log("ASSERT_REPAIR_INSPECTION_ONESHOT A-pass")
}
func TestPrivateLocalAcquisitionArchiveBounds(t *testing.T) {
	for _, kind := range []string{"members", "member-size", "not-gzip", "cancelled", "replacement"} {
		t.Run(kind, func(t *testing.T) {
			var b []byte
			switch kind {
			case "members":
				b = acquisitionArchiveFixtureV1(t, 17, 0)
			case "member-size":
				b = acquisitionArchiveFixtureV1(t, 1, (64<<20)+1)
			case "not-gzip":
				b = []byte("not gzip")
			default:
				b = acquisitionArchiveFixtureV1(t, 1, 0)
			}
			o, ctx, cancel, got := acquisitionArchiveOwnerV1(t, b)
			lease, e := got.BeginNativeArchiveInspectionV1(ctx)
			if e != nil {
				t.Fatal("ASSERT_REPAIR_ARCHIVE_LEASE", e)
			}
			if kind == "cancelled" {
				cancel()
			}
			use := ctx
			if kind == "replacement" {
				use = context.Background()
			}
			if _, e = lease.Inspect(use); e == nil {
				t.Fatal("ASSERT_REPAIR_ARCHIVE_BOUNDS")
			}
			if e = lease.Close(); e != nil {
				t.Fatal(e)
			}
			re := got.Retire(ctx)
			if kind == "cancelled" {
				if re == nil {
					t.Fatal("ASSERT_REPAIR_ARCHIVE_CANCELLED")
				}
			} else {
				if re != nil {
					t.Fatal(re)
				}
				h, e := o.k.reopen(ctx, o.job, o.limits)
				if e != nil || h.Disposition != "FAILED" {
					t.Fatal("ASSERT_REPAIR_ARCHIVE_BOUNDS", e)
				}
			}
		})
	}
	if !t.Failed() {
		t.Log("ASSERT_REPAIR_ARCHIVE_BOUNDS A-pass")
		t.Log("ASSERT_REPAIR_ARCHIVE_CANCELLED A-pass")
	}
}

type acquisitionArchivePauseV1 struct {
	calls            atomic.Int64
	entered, release chan struct{}
}

func (p *acquisitionArchivePauseV1) beforeArchiveRead() {
	if p.calls.Add(1) == 2 {
		close(p.entered)
		<-p.release
	}
}
func TestPrivateLocalAcquisitionArchiveReadJoinCancel(t *testing.T) {
	o, ctx, cancel, got := acquisitionArchiveOwnerV1(t, acquisitionArchiveFixtureV1(t, 2, 0))
	lease, e := got.BeginNativeArchiveInspectionV1(ctx)
	if e != nil {
		t.Fatal(e)
	}
	pause := &acquisitionArchivePauseV1{entered: make(chan struct{}), release: make(chan struct{})}
	lease.state.observer = pause
	inspectDone := make(chan error, 1)
	go func() { _, e := lease.Inspect(ctx); inspectDone <- e }()
	released := false
	defer func() {
		if !released {
			close(pause.release)
		}
	}()
	select {
	case <-pause.entered:
	case e := <-inspectDone:
		t.Fatal("ASSERT_REPAIR_READER_JOIN: second actual FD boundary not reached", e)
	case <-time.After(2 * time.Second):
		t.Fatal("ASSERT_REPAIR_READER_JOIN: stalled")
	}
	// The first callback has returned into a genuine file Read; the second is
	// paused before the next actual Read. No mock file or parser backend is used.
	got.state.budget.mu.Lock()
	reads := got.state.budget.archiveReads
	got.state.budget.mu.Unlock()
	if reads != acquisitionInspectionCallsV1-1 {
		t.Fatal("ASSERT_REPAIR_READER_JOIN", reads)
	}
	if e = got.Retire(ctx); e == nil {
		t.Fatal("ASSERT_REPAIR_ACTIVE_USE_BARRIER")
	}
	closed := make(chan error, 1)
	closing := make(chan struct{})
	go func() { close(closing); closed <- lease.Close() }()
	<-closing
	select {
	case e := <-closed:
		t.Fatal("ASSERT_REPAIR_READER_JOIN: Close returned before read method", e)
	case <-time.After(20 * time.Millisecond):
	}
	cancel()
	close(pause.release)
	released = true
	if e = <-inspectDone; e == nil {
		t.Fatal("ASSERT_REPAIR_ARCHIVE_CANCELLED")
	}
	if e = <-closed; e != nil {
		t.Fatal("ASSERT_REPAIR_READER_JOIN", e)
	}
	if e = got.Retire(ctx); e == nil {
		t.Fatal("ASSERT_REPAIR_ARCHIVE_CANCELLED")
	}
	got.state.budget.mu.Lock()
	after := got.state.budget.archiveReads
	got.state.budget.mu.Unlock()
	if after != reads {
		t.Fatal("ASSERT_REPAIR_NO_CLOSED_DEBIT_READ")
	}
	inspect, done := context.WithTimeout(context.Background(), time.Second)
	defer done()
	h, e := o.k.reopen(inspect, o.job, o.limits)
	if e != nil || h.Disposition != "PENDING" || !o.uncertain {
		t.Fatal("ASSERT_REPAIR_ARCHIVE_CANCELLED", e)
	}
	for _, f := range got.state.files {
		if f != nil {
			t.Fatal("ASSERT_REPAIR_READER_JOIN: owned FD not closed")
		}
	}
	t.Log("ASSERT_REPAIR_READER_JOIN A-pass")
	t.Log("ASSERT_REPAIR_ARCHIVE_CANCELLED A-pass")
	t.Log("ASSERT_REPAIR_NO_CLOSED_DEBIT_READ A-pass")
}
func TestPrivateLocalAcquisitionArchiveIdentity(t *testing.T) {
	_, ctx, _, got := acquisitionArchiveOwnerV1(t, acquisitionArchiveFixtureV1(t, 1, 0))
	if e := got.state.files[1].Chmod(0640); e != nil {
		t.Fatal(e)
	}
	got.state.budget.mu.Lock()
	before := got.state.budget.archiveReads
	got.state.budget.mu.Unlock()
	if _, e := got.BeginNativeArchiveInspectionV1(ctx); e == nil {
		t.Fatal("ASSERT_REPAIR_ARCHIVE_FD_IDENTITY")
	}
	got.state.budget.mu.Lock()
	after := got.state.budget.archiveReads
	got.state.budget.mu.Unlock()
	if before != after {
		t.Fatal("ASSERT_REPAIR_ARCHIVE_FD_IDENTITY")
	}
	// Restore only this fixture's exact owned mode so ordinary fixed cleanup can
	// run; this is not an operational substitution or recovery interface.
	if e := got.state.files[1].Chmod(0600); e != nil {
		t.Fatal(e)
	}
	if e := got.Retire(ctx); e != nil {
		t.Fatal(e)
	}
	t.Log("ASSERT_REPAIR_ARCHIVE_FD_IDENTITY A-pass")
}

func TestPrivateLocalAcquisitionRootIdentityAncestry(t *testing.T) {
	base := acquisitionProtectedFixtureV1(t)
	child := filepath.Join(base, "child")
	if e := os.Mkdir(child, 0700); e != nil {
		t.Fatal(e)
	}
	sibling := acquisitionProtectedFixtureV1(t)
	for _, item := range []struct {
		name, path string
		want       bool
	}{{"same-FD-object", base, false}, {"ancestor-child", child, false}, {"disjoint-sibling", sibling, true}} {
		t.Run(item.name, func(t *testing.T) {
			var scratch [4096]byte
			a, _, e := openProtectedLocalDirectoryV1(base, &scratch, nil)
			if e != nil {
				t.Fatal(e)
			}
			defer a.Close()
			b, _, e := openProtectedLocalDirectoryV1(item.path, &scratch, nil)
			if e != nil {
				t.Fatal(e)
			}
			defer b.Close()
			aa, e := acquisitionDirectoryAncestryV1(a, &scratch, nil)
			if e != nil {
				t.Fatal(e)
			}
			bb, e := acquisitionDirectoryAncestryV1(b, &scratch, nil)
			if e != nil {
				t.Fatal(e)
			}
			if acquisitionAncestryDisjointV1(aa, bb) != item.want {
				t.Fatal("ASSERT_REPAIR_ROOT_IDENTITY_ANCESTRY")
			}
		})
	}
	if !t.Failed() {
		t.Log("ASSERT_REPAIR_ROOT_IDENTITY_ANCESTRY A-pass")
	}
}

func TestPrivateLocalAcquisitionInspectionBudget(t *testing.T) {
	for _, kind := range []string{"compressed", "expanded", "hash"} {
		t.Run(kind, func(t *testing.T) {
			_, ctx, _, got := acquisitionArchiveOwnerV1(t, acquisitionArchiveFixtureV1(t, 1, 0))
			lease, e := got.BeginNativeArchiveInspectionV1(ctx)
			if e != nil {
				t.Fatal(e)
			}
			got.state.budget.mu.Lock()
			if kind == "compressed" {
				got.state.budget.archiveReads = 0
			} else if kind == "hash" {
				got.state.budget.archiveHashes = 0
			} else {
				got.state.budget.expandedReads = 0
			}
			got.state.budget.mu.Unlock()
			if _, e = lease.Inspect(ctx); e == nil {
				t.Fatal("ASSERT_REPAIR_INSPECTION_BUDGET")
			}
			if e = lease.Close(); e != nil {
				t.Fatal(e)
			}
			if e = got.Retire(ctx); e != nil {
				t.Fatal(e)
			}
		})
	}
	if !t.Failed() {
		t.Log("ASSERT_REPAIR_INSPECTION_BUDGET A-pass")
	}
}

func TestPrivateLocalAcquisitionArchiveContentContinuity(t *testing.T) {
	makeArchive := func(value byte) []byte {
		var raw, out bytes.Buffer
		tw := tar.NewWriter(&raw)
		body := bytes.Repeat([]byte{value}, 31)
		if e := tw.WriteHeader(&tar.Header{Name: "libfixture.dylib", Mode: 0600, Size: int64(len(body)), Format: tar.FormatUSTAR}); e != nil {
			t.Fatal(e)
		}
		if _, e := tw.Write(body); e != nil {
			t.Fatal(e)
		}
		if e := tw.Close(); e != nil {
			t.Fatal(e)
		}
		gz, e := gzip.NewWriterLevel(&out, gzip.NoCompression)
		if e != nil {
			t.Fatal(e)
		}
		if _, e = gz.Write(raw.Bytes()); e != nil {
			t.Fatal(e)
		}
		if e = gz.Close(); e != nil {
			t.Fatal(e)
		}
		return out.Bytes()
	}
	a, b := makeArchive('a'), makeArchive('b')
	if len(a) != len(b) || bytes.Equal(a, b) {
		t.Fatal("bad fixed content-continuity fixture")
	}
	_, ctx, _, got := acquisitionArchiveOwnerV1(t, a)
	if _, e := got.state.files[1].WriteAt(b, 0); e != nil {
		t.Fatal(e)
	}
	lease, e := got.BeginNativeArchiveInspectionV1(ctx)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = lease.Inspect(ctx); e == nil {
		t.Fatal("ASSERT_REPAIR_ARCHIVE_CONTENT_CONTINUITY")
	}
	if e = lease.Close(); e != nil {
		t.Fatal(e)
	}
	if e = got.Retire(ctx); e != nil {
		t.Fatal(e)
	}
	t.Log("ASSERT_REPAIR_ARCHIVE_CONTENT_CONTINUITY A-pass")
}

// Keep the fixed error surface check independent of transport/provider strings.
func TestPrivateLocalAcquisitionErrorIdentity(t *testing.T) {
	var o *PrivateManagedInputOwnerV2
	_, e := o.AcquireLocalCandidateV1(context.Background(), PrivateLocalAcquisitionRequestV1{})
	if !errors.Is(e, ErrPrivateLocalAcquisitionV1) {
		t.Fatal("ASSERT_ACQUISITION_OPAQUE_HANDLE", e)
	}
}
