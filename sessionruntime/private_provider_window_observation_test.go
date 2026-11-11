package sessionruntime

import (
	"context"
	"errors"
	"lsp-trace/internal/managedprocess"
	"lsp-trace/internal/session"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func observationWindowV1() *privateProviderWindow {
	return &privateProviderWindow{ctx: context.Background(), deadline: time.Now().Add(time.Minute), cancel: func() {}, changed: make(chan struct{})}
}
func observationAwaitV1(t *testing.T, ch <-chan struct{}) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(5 * time.Second):
		t.Fatal("ASSERT_WINDOW_BARRIER: FAIL")
	}
}

func TestPrivateProviderWindowObservationAdmission(t *testing.T) {
	for _, mode := range []string{"failed-active-zero", "active", "finishing"} {
		t.Run(mode, func(t *testing.T) {
			w := observationWindowV1()
			switch mode {
			case "failed-active-zero":
				w.failed = true
			case "active":
				w.active = 1
			case "finishing":
				w.finishing = true
			}
			before := w.active
			called := 0
			changed := w.changed
			err, obs := privateProviderWindowLeaseRun(w, context.Background(), func(context.Context) error { called++; return nil })
			if err != ErrPrivateProviderWindowUnjoined || called != 0 || w.active != before || w.changed != changed || obs.Boundary() != PrivateLeaseBoundaryAdmissionRejected || obs.ReturnedResultObserved() || obs.InnerResult() != PrivateLeaseResultUnobserved {
				t.Fatal("ASSERT_WINDOW_ADMISSION: FAIL")
			}
			t.Log("ASSERT_WINDOW_ADMISSION: PASS")
		})
	}
}

func TestPrivateProviderWindowObservationContextLateFreeze(t *testing.T) {
	w := observationWindowV1()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	entered, release, wrapperReturned := make(chan struct{}), make(chan struct{}), make(chan struct{})
	var once sync.Once
	unblock := func() { once.Do(func() { close(release) }) }
	t.Cleanup(unblock)
	var canceled atomic.Int32
	w.cancel = func() { canceled.Add(1) }
	changed := w.changed
	var got error
	var obs PrivateLeaseBoundaryObservation
	go func() {
		got, obs = privateProviderWindowLeaseRun(w, ctx, func(inner context.Context) error {
			if !privateProviderWindowInner(inner) {
				t.Error("ASSERT_WINDOW_INNER_KEY: FAIL")
			}
			close(entered)
			<-release
			return context.DeadlineExceeded
		})
		close(wrapperReturned)
	}()
	observationAwaitV1(t, entered)
	cancel()
	observationAwaitV1(t, wrapperReturned)
	frozen := obs
	w.mu.Lock()
	active, expired := w.active, w.expired
	w.mu.Unlock()
	if got != ErrPrivateProviderWindowUnjoined || active != 1 || !expired || canceled.Load() != 1 || obs.Boundary() != PrivateLeaseBoundaryContextDone || obs.ReturnedResultObserved() || obs.InnerResult() != PrivateLeaseResultUnobserved {
		t.Fatal("ASSERT_WINDOW_CONTEXT_FREEZE: FAIL")
	}
	unblock()
	observationAwaitV1(t, changed)
	w.mu.Lock()
	active = w.active
	w.mu.Unlock()
	// notify/active==0 precedes result-buffer delivery; the caller never receives
	// that buffer on the context arm and must not infer observation from activity.
	if active != 0 || obs != frozen || obs.ReturnedResultObserved() {
		t.Fatal("ASSERT_WINDOW_CONTEXT_FREEZE: FAIL late worker")
	}
	t.Log("ASSERT_WINDOW_CONTEXT_FREEZE: PASS")
}

func TestPrivateProviderWindowObservationResultReplaced(t *testing.T) {
	w := observationWindowV1()
	changed := w.changed
	err, obs := privateProviderWindowLeaseRun(w, context.Background(), func(context.Context) error {
		w.mu.Lock()
		w.failed = true
		w.mu.Unlock()
		return context.DeadlineExceeded
	})
	if err != ErrPrivateProviderWindowUnjoined || obs.Boundary() != PrivateLeaseBoundaryResultReplaced || !obs.ReturnedResultObserved() || obs.InnerResult() != PrivateLeaseResultDeadline || w.active != 0 {
		t.Fatal("ASSERT_WINDOW_REPLACED: FAIL")
	}
	observationAwaitV1(t, changed)
	t.Log("ASSERT_WINDOW_REPLACED: PASS")
}

func TestPrivateProviderWindowObservationResultReturned(t *testing.T) {
	for _, item := range []struct {
		err  error
		want PrivateLeaseResult
	}{{nil, PrivateLeaseResultOK}, {context.Canceled, PrivateLeaseResultCanceled}, {context.DeadlineExceeded, PrivateLeaseResultDeadline}, {ErrPrivateProviderWindowUnjoined, PrivateLeaseResultUnjoined}, {errSessionLeaseConflict, PrivateLeaseResultConflict}} {
		e := item.err
		w := observationWindowV1()
		changed := w.changed
		calls := 0
		err, obs := privateProviderWindowLeaseRun(w, context.Background(), func(inner context.Context) error {
			calls++
			if !privateProviderWindowInner(inner) {
				t.Error("ASSERT_WINDOW_INNER_KEY: FAIL")
			}
			return e
		})
		if err != e || calls != 1 || obs.Boundary() != PrivateLeaseBoundaryResultReturned || !obs.ReturnedResultObserved() || obs.InnerResult() != item.want || w.active != 0 {
			t.Fatal("ASSERT_WINDOW_RETURNED: FAIL")
		}
		observationAwaitV1(t, changed)
	}
	t.Log("ASSERT_WINDOW_RETURNED: PASS")
}

type observationOpaqueV1 struct {
	payload []byte
	calls   *atomic.Int32
}

func (e observationOpaqueV1) Error() string { e.calls.Add(1); panic("opaque Error") }
func (e observationOpaqueV1) Is(error) bool { e.calls.Add(1); panic("opaque Is") }
func (e observationOpaqueV1) Unwrap() error { e.calls.Add(1); panic("opaque Unwrap") }
func TestPrivateProviderWindowObservationOpaque(t *testing.T) {
	defer func() {
		if recover() != nil {
			t.Error("ASSERT_WINDOW_OPAQUE: FAIL method invoked")
		}
	}()
	var calls atomic.Int32
	e := observationOpaqueV1{[]byte("private-never-output"), &calls}
	nodes := make([]error, 4096)
	for i := range nodes {
		nodes[i] = e
	}
	for _, err := range []error{e, errors.Join(nodes...)} {
		if privateLeaseResult(err) != PrivateLeaseResultUnknown {
			t.Fatal("ASSERT_WINDOW_OPAQUE: FAIL classification")
		}
	}
	if calls.Load() != 0 || PrivateLeaseBoundary(255).String() != "UNKNOWN" || PrivateLeaseResult(255).String() != "UNKNOWN" {
		t.Fatal("ASSERT_WINDOW_OPAQUE: FAIL")
	}
	t.Log("ASSERT_WINDOW_OPAQUE: PASS")
}

func observationManagerV1() (*Manager, *privateProviderWindow, context.Context, SessionLeaseRequest) {
	w := observationWindowV1()
	m := &Manager{limits: Limits{MaxChildren: 2}, starter: ManagedStarter{}, sessions: make(map[string]*runtimeSession), workerDone: make(chan struct{}, 1)}
	r := &runtimeSession{record: Record{SessionID: "unit", Generation: 1, State: session.Ready}}
	w.spec = &privateProviderWindowSpec{manager: m, limits: m.limits, configured: m.wire, readiness: m.readinessTimeout, controlled: m.starter}
	w.ownedRecord = r
	w.child = &privateProviderWindowChild{window: w}
	r.process = w.child
	m.sessions["unit"] = r
	w.start = StartResult{SessionID: "unit", Generation: 1, Start: managedprocess.StartObservation{Kind: managedprocess.StartStarted}}
	return m, w, context.WithValue(context.Background(), privateProviderWindowKey{}, w), SessionLeaseRequest{SessionID: "unit", Generation: 1}
}
func TestPrivateProviderWindowObservationSiblingEffects(t *testing.T) {
	for _, private := range []bool{false, true} {
		for _, observed := range []bool{false, true} {
			m, w, ctx, req := observationManagerV1()
			if !private {
				ctx = context.Background()
			}
			calls := 0
			callback := func(inner context.Context) error {
				calls++
				m.mu.Lock()
				leased := m.workers == 1 && m.sessions["unit"].leaseCount == 1
				m.mu.Unlock()
				if !leased || (private && !privateProviderWindowInner(inner)) {
					t.Error("ASSERT_WINDOW_SINGLE_EFFECT: FAIL lease")
				}
				return context.Canceled
			}
			var err error
			var obs PrivateLeaseBoundaryObservation
			if observed {
				err, obs = m.WithReadySessionLeaseObserved(ctx, req, callback)
			} else {
				err = m.WithReadySessionLease(ctx, req, callback)
			}
			if err != context.Canceled || calls != 1 || m.workers != 0 || m.sessions["unit"].leaseCount != 0 || m.sessions["unit"].leaseNonce != 2 || w.active != 0 {
				t.Fatal("ASSERT_WINDOW_SINGLE_EFFECT: FAIL")
			}
			if observed && ((!private && obs.Boundary() != PrivateLeaseBoundaryNotApplicable) || (private && (obs.Boundary() != PrivateLeaseBoundaryResultReturned || !obs.ReturnedResultObserved()))) {
				t.Fatal("ASSERT_WINDOW_SINGLE_EFFECT: FAIL boundary")
			}
		}
	}
	t.Log("ASSERT_WINDOW_SINGLE_EFFECT: PASS")
}
func TestPrivateProviderWindowObservationSiblingRefusals(t *testing.T) {
	m, _, ctx, req := observationManagerV1()
	req.Generation++
	calls := 0
	err, obs := m.WithReadySessionLeaseObserved(ctx, req, func(context.Context) error { calls++; return nil })
	if err != errSessionLeaseConflict || calls != 0 || obs.Boundary() != PrivateLeaseBoundaryOwnerRejected || obs.ReturnedResultObserved() {
		t.Fatal("ASSERT_WINDOW_OWNER: FAIL")
	}
	for _, ctx := range []context.Context{nil, context.Background()} {
		original := m.WithReadySessionLease(ctx, req, nil)
		got, v := m.WithReadySessionLeaseObserved(ctx, req, nil)
		if original != errSessionLeaseConflict || got != original || v.Boundary() != PrivateLeaseBoundaryNotApplicable {
			t.Fatal("ASSERT_WINDOW_COMPAT: FAIL")
		}
	}
	t.Log("ASSERT_WINDOW_OWNER: PASS; ASSERT_WINDOW_COMPAT: PASS")
}
func TestPrivateProviderWindowObservationGenericContextEffects(t *testing.T) {
	trace := func(observed bool) [6]int {
		w := observationWindowV1()
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		entered, release, returned := make(chan struct{}), make(chan struct{}), make(chan struct{})
		var once sync.Once
		unblock := func() { once.Do(func() { close(release) }) }
		defer unblock()
		t.Cleanup(unblock)
		var cancels atomic.Int32
		w.cancel = func() { cancels.Add(1) }
		changed := w.changed
		calls := 0
		var result error
		work := func(context.Context) error { calls++; close(entered); <-release; return nil }
		go func() {
			if observed {
				result, _ = privateProviderWindowLeaseRun(w, ctx, work)
			} else {
				result = privateProviderWindowRun(w, ctx, error(ErrPrivateProviderWindowUnjoined), work)
			}
			close(returned)
		}()
		observationAwaitV1(t, entered)
		cancel()
		observationAwaitV1(t, returned)
		w.mu.Lock()
		active, expired := w.active, w.expired
		w.mu.Unlock()
		unblock()
		observationAwaitV1(t, changed)
		w.mu.Lock()
		joined := w.active
		w.mu.Unlock()
		out := [6]int{active, int(cancels.Load()), calls, joined}
		if expired {
			out[4] = 1
		}
		if result == ErrPrivateProviderWindowUnjoined {
			out[5] = 1
		}
		return out
	}
	old, current := trace(false), trace(true)
	if old != current || current != [6]int{1, 1, 1, 0, 1, 1} {
		t.Fatal("ASSERT_WINDOW_GENERIC_CONTEXT_EFFECTS: FAIL", old, current)
	}
	t.Log("ASSERT_WINDOW_GENERIC_CONTEXT_EFFECTS: PASS")
}

func TestPrivateProviderWindowObservationGenericEffects(t *testing.T) {
	for _, replace := range []bool{false, true} {
		trace := func(observed bool) (error, int, bool, bool, int) {
			w := observationWindowV1()
			var cancels atomic.Int32
			w.cancel = func() { cancels.Add(1) }
			changed := w.changed
			calls := 0
			work := func(inner context.Context) error {
				calls++
				if replace {
					w.mu.Lock()
					w.failed = true
					w.mu.Unlock()
				}
				return context.Canceled
			}
			var e error
			if observed {
				e, _ = privateProviderWindowLeaseRun(w, context.Background(), work)
			} else {
				e = privateProviderWindowRun(w, context.Background(), error(ErrPrivateProviderWindowUnjoined), work)
			}
			notified := false
			select {
			case <-changed:
				notified = true
			default:
			}
			return e, w.active, w.expired, notified, calls + int(cancels.Load())
		}
		a, b, c, d, e := trace(false)
		f, g, h, i, j := trace(true)
		if a != f || b != g || c != h || d != i || e != j {
			t.Fatal("ASSERT_WINDOW_GENERIC_EFFECTS: FAIL")
		}
	}
	t.Log("ASSERT_WINDOW_GENERIC_EFFECTS: PASS")
}
