package sessionruntime

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"lsp-trace/internal/managedprocess"
	"math"
	"strings"
	"sync"
	"testing"
	"unsafe"
)

func refusal53Window() *privateDiscoveryWindow {
	l := PrivateDiscoveryLimits{SourceBytes: 16 * (MaxDocumentSupplyBytes + 1), RequestBytes: 32 << 20, ResponseBytes: 16 << 20, WireBytes: 48 << 20, RetainedBytes: 512 << 20, Work: 1 << 32, Messages: 256, Requests: 128, FrameBytes: 1 << 20, HeaderBytes: 8192}
	return &privateDiscoveryWindow{ctx: context.Background(), limits: l, remaining: l}
}

type refusal53Child struct{ io.ReadCloser }

func (c refusal53Child) Stdout() io.ReadCloser                   { return c.ReadCloser }
func (refusal53Child) Stdin() io.WriteCloser                     { panic("unexpected write") }
func (refusal53Child) Close() managedprocess.ResourceObservation { panic("unexpected close") }
func (refusal53Child) Teardown(context.Context) managedprocess.TeardownObservation {
	panic("unexpected teardown")
}
func TestRefusal53SourceAccounting(t *testing.T) {
	// Independent oracle qualified for the supported 64-bit layouts. A layout
	// change requires reviewing the fixed ownership prices, not copying observed
	// remaining balances or calling the production pricing function as an oracle.
	if unsafe.Sizeof(uintptr(0)) != 8 || unsafe.Sizeof(privateDiscoveryFrame{}) != 304 {
		t.Fatal("ASSERT_REVIEW_64BIT_FRAME_LAYOUT")
	}
	const wire = "Content-Length: 36\r\n\r\n{\"jsonrpc\":\"2.0\",\"id\":1,\"result\":[]}"
	const sourceBytes, sourceRetained, sourceWork = 1048577, 16777232, 33620000
	// Header/hash + acquisition/raw + decoder depth/plan + decoded fields +
	// padded runtime frame record. This small frame's retention is NOT 4*cap.
	const frameRetained = (2*8192 + 175) + 2*58 + (10000 + 192) + 9 + 304
	for _, cap := range []uint64{1 << 20, 64 << 10} {
		t.Run(map[uint64]string{1 << 20: "cap-1MiB", 64 << 10: "cap-64KiB"}[cap], func(t *testing.T) {
			w := refusal53Window()
			want := PrivateDiscoveryLimits{SourceBytes: 16777232, RequestBytes: 33554432, ResponseBytes: 16777216, WireBytes: 50331648, RetainedBytes: 536870912, Work: 4294967296, Messages: 256, Requests: 128, FrameBytes: 1048576, HeaderBytes: 8192}
			if w.limits != want || w.remaining != want {
				t.Fatal("ASSERT_ORIGINAL_CEILINGS")
			}
			w.limits.FrameBytes = cap
			check := func(stage string) {
				t.Helper()
				if w.remaining != want || w.failed || w.refusal.Valid() {
					t.Fatalf("ASSERT_SOURCE_ACCOUNTING_VECTOR %s: got=%+v want=%+v refusal=%+v", stage, w.remaining, want, w.refusal)
				}
			}
			if !w.work(4<<20) || !w.spend(&w.remaining.RetainedBytes, 4<<20) {
				t.Fatal("fixture bookkeeping")
			}
			want.Work -= 4194304
			want.RetainedBytes -= 4194304
			check("bookkeeping")
			if !w.prepayBackend() {
				t.Fatal("fixture backend")
			}
			want.Work -= 5046272
			want.RetainedBytes -= 86016
			want.ResponseBytes -= 69632
			want.WireBytes -= 69632
			check("backend")
			for i := 0; i < 15; i++ {
				if w.sourceLimit() != 1048576 {
					t.Fatal("early source refusal")
				}
				want.SourceBytes -= sourceBytes
				want.RetainedBytes -= sourceRetained
				want.Work -= sourceWork
				check("source 1..15")
			}
			// A single genuine child owns the full inert stream. No request writes,
			// slots, selected-result copies, capture or source opens are performed.
			child, inert := refusal53BoundChild(w, strings.Repeat(wire, 65))
			for i := 0; i < 65; i++ {
				if !w.quoteRequest(0) {
					t.Fatal("request")
				}
				want.Requests--
				want.Work -= 65536
				check("request quote")
				message, observation, raw, available, err := w.frame(child)
				if err != nil || !available || string(raw) != wire || string(message.Result) != "[]" || observation.FrameBytes != 58 || inert.reads != (i+1)*58 {
					t.Fatalf("ASSERT_FRAME_WIRE_ACCOUNTING: frame=%d reads=%d err=%v", i, inert.reads, err)
				}
				want.Messages--
				want.ResponseBytes -= 58
				want.WireBytes -= 58
				want.RetainedBytes -= frameRetained
				want.Work -= 16*cap + 25417
				check("frame")
			}
			// Independent final table also checks the staged arithmetic above.
			before16 := PrivateDiscoveryLimits{SourceBytes: 1048577, RequestBytes: 33554432, ResponseBytes: 16703814, WireBytes: 50258246, RetainedBytes: 279165412, Work: 2684995735, Messages: 191, Requests: 63, FrameBytes: 1048576, HeaderBytes: 8192}
			if cap == 64<<10 {
				before16.Work = 3707357335
			}
			if want != before16 || w.remaining != before16 || inert.reads != 3770 {
				t.Fatal("ASSERT_BEFORE_SOURCE16_TABLE", w.remaining)
			}
			if w.sourceLimit() != 1048576 {
				t.Fatal("ASSERT_SOURCE16_FITS")
			}
			want.SourceBytes = 0
			want.RetainedBytes = 262388180
			want.Work = 2651375735
			if cap == 64<<10 {
				want.Work = 3673737335
			}
			check("source16")
			before := w.remaining
			if w.sourceLimit() != 0 {
				t.Fatal("ASSERT_SOURCE17_REFUSED")
			}
			d, ok := PrivateDiscoveryRefusalFromError(w.refusalError(PrivateDiscoveryAllSupplied, false))
			if !ok || d.Stage != PrivateDiscoverySource || d.Currency != PrivateDiscoverySourceBytes || !d.AmountsAvailable || d.Requested != 2 || d.Remaining != 0 || w.remaining != before {
				t.Fatalf("ASSERT_SOURCE17_PRE_EFFECT: %+v", d)
			}
			first := d
			w.work(math.MaxUint64)
			d, _ = PrivateDiscoveryRefusalFromError(w.refusalError(PrivateDiscoveryCleanup, true))
			first.CleanupUncertain = true
			if d != first || w.remaining != before {
				t.Fatal("ASSERT_FIRST_REFUSAL_CLEANUP_PRESERVED")
			}
		})
	}
}

func TestRefusal53SourceRetainedBoundary(t *testing.T) {
	// Separate minimal-capacity contract, not the 512-MiB trajectory above.
	for _, capacity := range []uint64{16777232, 16777231} {
		w := refusal53Window()
		w.limits.RetainedBytes = capacity
		w.remaining.RetainedBytes = capacity
		before := w.remaining
		n := w.sourceLimit()
		if capacity == 16777232 {
			want := before
			want.SourceBytes -= 1048577
			want.Work -= 33620000
			want.RetainedBytes = 0
			if n != 1048576 || w.remaining != want || w.failed || w.refusal.Valid() {
				t.Fatal("ASSERT_SOURCE_RETAINED_EQUALITY", w.remaining)
			}
			continue
		}
		d, ok := PrivateDiscoveryRefusalFromError(w.refusalError(PrivateDiscoveryUnknownStage, false))
		if n != 0 || !ok || d.Stage != PrivateDiscoverySource || d.Currency != PrivateDiscoveryRetainedBytes || !d.AmountsAvailable || d.Requested != 16777232 || d.Remaining != 16777231 || w.remaining != before {
			t.Fatalf("ASSERT_SOURCE_RETAINED_ATOMIC_REFUSAL: %+v before=%+v after=%+v", d, before, w.remaining)
		}
	}
}

func TestRefusal53SourceAdmissionBeforeFilesystem(t *testing.T) {
	// Existing inert manager fixture: oneChildStarter returns a memory transport,
	// never an OS process. External reader-entry instrumentation independently
	// proves the below-boundary case does not enter the filesystem reader.
	t.Setenv("TMPDIR", capture88PrivateDirectory(t))
	for _, name := range []string{"below", "equal"} {
		t.Run(name, func(t *testing.T) {
			content := []byte("package fixture\n")
			m, req, _, writer := supplyFixture(t, content)
			t.Cleanup(func() { m.Shutdown(context.Background()) })
			w := refusal53Window()
			w.manager = m
			w.remaining.RetainedBytes = 16777232
			if name == "below" {
				w.remaining.RetainedBytes--
			}
			req.PreparedNoFollow = true
			before := w.remaining
			ctx := context.WithValue(context.Background(), privateDiscoveryContextKey{}, w)
			got := m.PrepareDocument(ctx, req)
			if name == "equal" {
				if got.Failure != "" || got.Supply == nil || !bytes.Equal(got.Supply.Content, content) || writer.Len() == 0 || w.remaining.RetainedBytes != 0 {
					t.Fatalf("ASSERT_SOURCE_CALLER_EQUALITY: %+v", got)
				}
				return
			}
			d, ok := PrivateDiscoveryRefusalFromError(w.refusalError(PrivateDiscoveryUnknownStage, false))
			if got.Failure != DocumentSupplyUnavailable || got.Supply != nil || writer.Len() != 0 || w.remaining != before || !ok || d.Stage != PrivateDiscoverySource || d.Currency != PrivateDiscoveryRetainedBytes || d.Requested != 16777232 || d.Remaining != 16777231 || !d.AmountsAvailable {
				t.Fatalf("ASSERT_SOURCE_CALLER_PRE_EFFECT: result=%+v refusal=%+v", got, d)
			}
		})
	}
}
func TestRefusal53RealFinalizer(t *testing.T) {
	for _, n := range []int{15, 16} {
		w := refusal53Window()
		s := &PrivateDiscoverySession{client: privateDiscoveryClient{w: w}, members: make([]PrivateDiscoveryMember, 16), supplied: map[string]bool{}}
		for i := 0; i < n; i++ {
			s.supplied[string(rune('a'+i))] = true
		}
		e := s.finalize(nil)
		if !s.closed {
			t.Fatal("ASSERT_REAL_GUARD_CLOSED")
		}
		if n == 16 {
			if e != nil {
				t.Fatal("ASSERT_REAL_GUARD_CONTROL")
			}
			continue
		}
		d, ok := PrivateDiscoveryRefusalFromError(e)
		if !errors.Is(e, ErrPrivateDiscovery) || !ok || d.Stage != PrivateDiscoveryAllSupplied || d.AmountsAvailable {
			t.Fatalf("ASSERT_REAL_GUARD_DIAGNOSTIC: %+v %v", d, e)
		}
	}
}

// Genuine window/child identity with an inert counted transport. No Starter,
// process, native authority, or fabricated admission receipt is involved.
func refusal53BoundChild(w *privateDiscoveryWindow, body string) (*privateDiscoveryChild, *discoveryInertChild) {
	inert := &discoveryInertChild{in: io.NopCloser(strings.NewReader(body))}
	child := &privateDiscoveryChild{wireChild: inert, window: w}
	w.child = child
	return child, inert
}

func TestRefusal53ConcreteChildBinding(t *testing.T) {
	const wire = "Content-Length: 36\r\n\r\n{\"jsonrpc\":\"2.0\",\"id\":1,\"result\":[]}"
	for _, mode := range []string{"bound", "window-child", "child-window"} {
		t.Run(mode, func(t *testing.T) {
			w := refusal53Window()
			child, inert := refusal53BoundChild(w, wire)
			switch mode {
			case "window-child":
				w.child = nil
			case "child-window":
				child.window = refusal53Window()
			}
			before := w.remaining
			message, observation, raw, available, err := w.frame(child)
			if mode == "bound" {
				// Independently specified: 22 header bytes + 36 body bytes. The
				// concrete reader must debit actual requested wire/response bytes.
				if err != nil || !available || string(message.Result) != "[]" || string(raw) != wire || observation.FrameBytes != 58 || inert.reads != 58 ||
					w.remaining.ResponseBytes != before.ResponseBytes-58 || w.remaining.WireBytes != before.WireBytes-58 || w.remaining.Messages != before.Messages-1 || w.refusal.Valid() {
					t.Fatalf("ASSERT_BOUND_FRAME_ACCOUNTING: observation=%+v remaining=%+v reads=%d err=%v", observation, w.remaining, inert.reads, err)
				}
				return
			}
			d, ok := PrivateDiscoveryRefusalFromError(w.refusalError(PrivateDiscoveryUnknownStage, false))
			if !errors.Is(err, ErrPrivateDiscovery) || available || raw != nil || inert.reads != 0 || w.remaining != before || !ok || d.Stage != PrivateDiscoveryFrame || d.AmountsAvailable {
				t.Fatalf("ASSERT_BINDING_REFUSAL_BEFORE_READ: diagnostic=%+v reads=%d err=%v", d, inert.reads, err)
			}
		})
	}
}

func TestRefusal53PreEffectCurrencies(t *testing.T) {
	for _, stage := range []PrivateDiscoveryRefusalStage{PrivateDiscoveryBackend, PrivateDiscoveryFrame, PrivateDiscoveryWrite, PrivateDiscoveryRead, PrivateDiscoverySource} {
		w := refusal53Window()
		want := PrivateDiscoveryUnknownCurrency
		var requested uint64
		switch stage {
		case PrivateDiscoveryBackend:
			w.remaining.RetainedBytes = 0
			want = PrivateDiscoveryRetainedBytes
			requested = 86016
		case PrivateDiscoveryFrame:
			w.remaining.Messages = 0
			want = PrivateDiscoveryMessages
			requested = 1
		case PrivateDiscoveryWrite:
			w.remaining.RequestBytes = 0
			want = PrivateDiscoveryRequestBytes
			requested = 3
		case PrivateDiscoveryRead:
			w.remaining.ResponseBytes = 0
			want = PrivateDiscoveryResponseBytes
			requested = 1
		case PrivateDiscoverySource:
			w.remaining.SourceBytes = 1
			want = PrivateDiscoverySourceBytes
			requested = 2
		}
		before := w.remaining
		switch stage {
		case PrivateDiscoveryBackend:
			if w.prepayBackend() {
				t.Fatal("backend admitted")
			}
		case PrivateDiscoveryFrame:
			child, inert := refusal53BoundChild(w, "")
			if _, _, _, _, e := w.frame(child); e == nil {
				t.Fatal("frame admitted")
			}
			if inert.reads != 0 {
				t.Fatal("ASSERT_FRAME_CURRENCY_BEFORE_READ")
			}
		case PrivateDiscoveryWrite:
			if _, e := (privateDiscoveryWriter{w: w}).Write([]byte("abc")); e == nil {
				t.Fatal("write admitted")
			}
		case PrivateDiscoveryRead:
			if _, e := (privateDiscoveryReader{w: w}).Read(make([]byte, 8)); e == nil {
				t.Fatal("read admitted")
			}
		case PrivateDiscoverySource:
			if w.sourceLimit() != 0 {
				t.Fatal("source admitted")
			}
		}
		d, ok := PrivateDiscoveryRefusalFromError(w.refusalError(PrivateDiscoveryUnknownStage, false))
		if !ok || d.Stage != stage || d.Currency != want || d.Requested != requested || !d.AmountsAvailable || w.remaining != before {
			t.Fatalf("ASSERT_PRE_EFFECT_CURRENCY stage=%d diagnostic=%+v", stage, d)
		}
	}
}
func TestRefusal53OverflowAndConcurrentSnapshots(t *testing.T) {
	w := refusal53Window()
	w.limits.FrameBytes = math.MaxUint64
	w.remaining.ResponseBytes = math.MaxUint64
	w.remaining.WireBytes = math.MaxUint64
	before := w.remaining
	if _, _, _, _, e := w.frame(refusal53Child{}); !errors.Is(e, ErrPrivateDiscovery) {
		t.Fatal("ASSERT_OVERFLOW_REFUSED")
	}
	d, _ := PrivateDiscoveryRefusalFromError(w.refusalError(PrivateDiscoveryConsume, false))
	if d.Stage != PrivateDiscoveryFrame || d.AmountsAvailable || w.remaining != before {
		t.Fatal("ASSERT_OVERFLOW_NO_INVENTED_AMOUNT_OR_EFFECT")
	}
	w = refusal53Window()
	w.remaining.Work = 0
	var group sync.WaitGroup
	for i := 0; i < 16; i++ {
		group.Add(1)
		go func(n uint64) {
			defer group.Done()
			w.work(n)
			_, _ = PrivateDiscoveryRefusalFromError(w.refusalError(PrivateDiscoveryConsume, false))
		}(uint64(i + 1))
	}
	group.Wait()
	first, _ := PrivateDiscoveryRefusalFromError(w.refusalError(PrivateDiscoveryConsume, false))
	w.work(math.MaxUint64)
	last, _ := PrivateDiscoveryRefusalFromError(w.refusalError(PrivateDiscoveryConsume, false))
	if first != last || !first.AmountsAvailable || first.Requested < 1 || first.Requested > 16 {
		t.Fatal("ASSERT_CONCURRENT_FIRST_REFUSAL")
	}
}
func TestRefusal53CleanupAndNoRawLeakage(t *testing.T) {
	// The same closed forwarding used by Stop and deferred cleanup must retain
	// the guard's original non-budget refusal even without accounting failure.
	guardWindow := refusal53Window()
	guard := &PrivateDiscoverySession{client: privateDiscoveryClient{w: guardWindow}, members: make([]PrivateDiscoveryMember, 16), supplied: map[string]bool{}}
	guardErr := guard.finalize(nil)
	guardErr = guardWindow.refusalErrorFrom(guardErr, PrivateDiscoveryUnknownStage, false)
	guardErr = guardWindow.refusalErrorFrom(guardErr, PrivateDiscoveryCleanup, true)
	guardDiagnostic, known := PrivateDiscoveryRefusalFromError(guardErr)
	if !known || guardDiagnostic.Stage != PrivateDiscoveryAllSupplied || guardDiagnostic.AmountsAvailable || !guardDiagnostic.CleanupUncertain {
		t.Fatal("ASSERT_NON_BUDGET_GUARD_SURVIVES_STOP_CLEANUP")
	}
	w := refusal53Window()
	w.remaining.RetainedBytes = 0
	if w.sourceLimit() != 0 {
		t.Fatal("fixture")
	}
	s := &PrivateDiscoverySession{client: privateDiscoveryClient{w: w}, members: make([]PrivateDiscoveryMember, 16), supplied: map[string]bool{}}
	e := s.finalize(errors.New("SECRET file:///private/source.go opaque native payload"))
	e = w.refusalErrorFrom(e, PrivateDiscoveryCleanup, true)
	// Match the runner's outer error-sanitization defer after cleanup.
	e = w.refusalErrorFrom(e, PrivateDiscoveryUnknownStage, false)
	d, ok := PrivateDiscoveryRefusalFromError(e)
	if !ok || d.Stage != PrivateDiscoverySource || d.Currency != PrivateDiscoveryRetainedBytes || !d.CleanupUncertain || !errors.Is(e, ErrPrivateDiscovery) {
		t.Fatal("ASSERT_CLEANUP_PRESERVES_ORIGINAL")
	}
	raw, err := json.Marshal(d)
	if err != nil || strings.Contains(e.Error(), "SECRET") || strings.Contains(string(raw), "SECRET") || !strings.Contains(string(raw), "retained_bytes") || !strings.Contains(string(raw), "source_quote") {
		t.Fatal("ASSERT_CLOSED_LABELS_NO_RAW_LEAKAGE", err)
	}
	w = refusal53Window()
	s = &PrivateDiscoverySession{client: privateDiscoveryClient{w: w}, members: []PrivateDiscoveryMember{{}}, supplied: map[string]bool{"one": true}}
	e = s.finalize(errors.New("SECRET"))
	d, ok = PrivateDiscoveryRefusalFromError(e)
	if !ok || d.Stage != PrivateDiscoveryConsume || d.AmountsAvailable || strings.Contains(e.Error(), "SECRET") {
		t.Fatal("ASSERT_NON_BUDGET_NO_AMOUNTS")
	}
	if _, err := json.Marshal(PrivateDiscoveryRefusal{Stage: 255}); err == nil {
		t.Fatal("ASSERT_ENUM_SERIALIZATION_CLOSED")
	}
}

// Review54 reproduction: invoke the actual shutdown method only with its first
// quote guaranteed to refuse before pending/transport/native resource access.
func TestReview54FinalizerSurvivesLaterShutdownRefusal(t *testing.T) {
	w := refusal53Window()
	w.remaining.Requests = 0
	s := &PrivateDiscoverySession{client: privateDiscoveryClient{w: w}, members: make([]PrivateDiscoveryMember, 16), supplied: map[string]bool{}}
	original := s.finalize(nil)
	before, ok := PrivateDiscoveryRefusalFromError(original)
	if !ok || before.Stage != PrivateDiscoveryAllSupplied {
		t.Fatal("fixture guard", before)
	}
	if w.failed {
		t.Fatal("FINALIZATION_POISONED_CLEANUP")
	}
	child := &privateDiscoveryChild{window: w}
	_ = (&Manager{}).gracefulShutdownWithResponsePrivateV2(child, nil, 1, nil)
	afterError := w.refusalErrorFrom(original, PrivateDiscoveryCleanup, true)
	after, ok := PrivateDiscoveryRefusalFromError(afterError)
	expected := before
	expected.CleanupUncertain = true
	if !ok || after != expected {
		t.Fatalf("ORIGINAL_REFUSAL_REPLACED: before=%+v after=%+v", before, after)
	}
	if !w.failed {
		t.Fatal("ACTUAL_SHUTDOWN_PREFLIGHT_DID_NOT_REFUSE")
	}
	again, _ := PrivateDiscoveryRefusalFromError(w.refusalErrorFrom(afterError, PrivateDiscoveryUnknownStage, false))
	if again != expected {
		t.Fatal("CLEANUP_UNCERTAINTY_NOT_MONOTONIC")
	}
}
func TestReview54InvalidNonBudgetValueCannotSerialize(t *testing.T) {
	for _, d := range []PrivateDiscoveryRefusal{
		{Stage: PrivateDiscoveryAllSupplied, Currency: PrivateDiscoveryWork, Requested: 123456789, AmountsAvailable: true},
		{Stage: PrivateDiscoveryConsume, Requested: 123456789, AmountsAvailable: false},
		{Stage: PrivateDiscoveryConsume, Currency: PrivateDiscoveryWork, AmountsAvailable: false},
		{},
	} {
		if d.Valid() {
			t.Fatal("invalid fixture accepted by Valid")
		}
		raw, err := json.Marshal(d)
		if err == nil {
			t.Errorf("INVALID_NON_BUDGET_NUMBERS_SERIALIZED: %s", raw)
		} else if strings.Contains(err.Error(), "123456789") || len(raw) != 0 {
			t.Fatal("INVALID_AGGREGATE_ERROR_LEAKED_PAYLOAD")
		}
	}
}
func TestRepair55ChronologyAndCleanupAdmission(t *testing.T) {
	for _, all := range []bool{false, true} {
		w := refusal53Window()
		s := &PrivateDiscoverySession{client: privateDiscoveryClient{w: w}, members: []PrivateDiscoveryMember{{}}, supplied: map[string]bool{}}
		var cause error
		if all {
			s.supplied["one"] = true
			cause = errors.New("nonpublic consume payload")
		}
		original := s.finalize(cause)
		first, ok := PrivateDiscoveryRefusalFromError(original)
		if !ok || first.AmountsAvailable || w.failed {
			t.Fatal("NON_BUDGET_RECORDING_POISONED_CLEANUP")
		}
		before := w.remaining
		if !w.quoteRequest(1024) || w.remaining.Requests != before.Requests-1 || w.remaining.Work != before.Work-16*(1024+4096) {
			t.Fatal("BOUNDED_CLEANUP_QUOTE_BLOCKED")
		}
		w.remaining.Requests = 0
		_ = (&Manager{}).gracefulShutdownWithResponsePrivateV2(&privateDiscoveryChild{window: w}, nil, 1, nil)
		got, _ := PrivateDiscoveryRefusalFromError(w.refusalErrorFrom(original, PrivateDiscoveryCleanup, true))
		first.CleanupUncertain = true
		if got != first {
			t.Fatal("FIRST_NON_BUDGET_REFUSAL_LOST")
		}
	}
	// A prior argument is not automatically earlier than the window recorder.
	w := refusal53Window()
	w.remaining.Work = 0
	w.work(1)
	first, _ := PrivateDiscoveryRefusalFromError(w.refusalError(PrivateDiscoveryUnknownStage, false))
	later := privateDiscoveryRefusalError{PrivateDiscoveryRefusal{Stage: PrivateDiscoveryConsume}}
	got, _ := PrivateDiscoveryRefusalFromError(w.refusalErrorFrom(later, PrivateDiscoveryCleanup, true))
	first.CleanupUncertain = true
	if got != first {
		t.Fatal("EARLIER_ACCOUNTING_REPLACED_BY_LATER_ERROR")
	}
}
func TestRepair55ValidAggregateSerialization(t *testing.T) {
	for _, d := range []PrivateDiscoveryRefusal{
		{Stage: PrivateDiscoveryUnknownStage},
		{Stage: PrivateDiscoveryAllSupplied},
		{Stage: PrivateDiscoveryAccounting, Currency: PrivateDiscoveryWork, Requested: 1, Remaining: 0, AmountsAvailable: true},
	} {
		raw, err := json.Marshal(d)
		if err != nil {
			t.Fatal("VALID_AGGREGATE_REJECTED", err)
		}
		var got PrivateDiscoveryRefusal
		if json.Unmarshal(raw, &got) != nil || got != d {
			t.Fatal("VALID_AGGREGATE_ROUNDTRIP")
		}
	}
}
func TestRefusal53UnknownAndBoundary(t *testing.T) {
	w := refusal53Window()
	w.remaining.Work = math.MaxUint64
	if !w.work(math.MaxUint64) || w.remaining.Work != 0 {
		t.Fatal("ASSERT_EXACT_BOUNDARY")
	}
	if w.work(1) {
		t.Fatal("ASSERT_ZERO_REFUSES")
	}
	d, ok := PrivateDiscoveryRefusalFromError(w.refusalError(PrivateDiscoveryConsume, false))
	if !ok || !d.AmountsAvailable || d.Requested != 1 || d.Remaining != 0 || d.Currency != PrivateDiscoveryWork {
		t.Fatal("ASSERT_KNOWN_ZERO", d)
	}
	w = refusal53Window()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	w.ctx = ctx
	if w.work(1) {
		t.Fatal("ASSERT_CANCEL_REFUSES")
	}
	d, _ = PrivateDiscoveryRefusalFromError(w.refusalError(PrivateDiscoveryConsume, false))
	if d.AmountsAvailable || d.Currency != 0 {
		t.Fatal("ASSERT_NON_BUDGET_UNKNOWN", d)
	}
	for _, bad := range []PrivateDiscoveryRefusal{{Stage: 255}, {Stage: PrivateDiscoverySource, Currency: 255, AmountsAvailable: true, Requested: 1}, {Stage: PrivateDiscoverySource, Requested: 1}, {Stage: PrivateDiscoverySource, Currency: PrivateDiscoveryWork, AmountsAvailable: true, Requested: 1, Remaining: 1}} {
		if bad.Valid() {
			t.Fatal("ASSERT_CLOSED_LABELS")
		}
	}
}
