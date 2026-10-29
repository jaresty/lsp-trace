package sessionruntime

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"testing"
	"time"

	"lsp-trace/internal/lspwire"
	"lsp-trace/internal/session"
)

// Only the first read is gated. Existing worker teardown interrupts the pipe;
// releasing this gate remains the test's responsibility even on failed assertions.
type budgetReaderPrivateV2 struct {
	io.ReadCloser
	entered, release chan struct{}
	first            bool
}

func (r *budgetReaderPrivateV2) Read(b []byte) (int, error) {
	if !r.first {
		r.first = true
		close(r.entered)
		<-r.release
	}
	return r.ReadCloser.Read(b)
}

type budgetChildPrivateV2 struct {
	*shutdownLoopChild
	reader io.ReadCloser
}

func (c *budgetChildPrivateV2) Stdout() io.ReadCloser { return c.reader }

type enteredReaderPrivateV2 struct {
	io.ReadCloser
	entered chan struct{}
	first   bool
}

func (r *enteredReaderPrivateV2) Read(b []byte) (int, error) {
	if !r.first {
		r.first = true
		close(r.entered)
	}
	return r.ReadCloser.Read(b)
}

func budgetManagerPrivateV2(t *testing.T, child Child) (*Manager, string) {
	t.Helper()
	m, e := New(Config{Limits: Limits{MaxSessions: 1, MaxRequests: 1, MaxChildren: 1, MaxCancels: 1, MaxTombstones: 2, MaxObservations: 4}, Wire: lspwire.Limits{MaxBodyBytes: 512, MaxHeaderBytes: 1024}, Starter: oneChildStarter{child}})
	if e != nil {
		t.Fatal(e)
	}
	s := m.Start(context.Background(), StartRequest{Profile: profile(t)})
	if s.Failure != "" {
		t.Fatal(s)
	}
	return m, s.SessionID
}
func budgetJoinPrivateV2(t *testing.T, m *Manager, id string) OperationSnapshot {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if e := m.Shutdown(ctx); e != nil {
		t.Fatal(e)
	}
	op, ok := m.Operation(id)
	if !ok {
		t.Fatal("missing operation")
	}
	return op
}
func mustBudgetPrivateV2(t *testing.T, d time.Duration) ShutdownResponseBudgetPrivateV2 {
	t.Helper()
	b, e := NewShutdownResponseBudgetPrivateV2(d)
	if e != nil {
		t.Fatal(e)
	}
	return b
}

func TestShutdownResponseBudgetPrivateV2Delayed(t *testing.T) {
	for _, private := range []bool{false, true} {
		t.Run(map[bool]string{false: "ordinary", true: "private"}[private], func(t *testing.T) {
			c := newShutdownLoopChild("exact")
			r := &budgetReaderPrivateV2{ReadCloser: c.stdout, entered: make(chan struct{}), release: make(chan struct{})}
			m, s := budgetManagerPrivateV2(t, &budgetChildPrivateV2{c, r})
			var stop session.LifecycleResult
			if private {
				stop = m.StopWithShutdownResponseBudgetPrivateV2(context.Background(), s, "budget", mustBudgetPrivateV2(t, time.Second))
			} else {
				stop = m.Stop(context.Background(), s, "budget")
			}
			<-r.entered
			// The gate—not provider scheduling—holds response delivery past the ordinary
			// 250ms limit. A generous private budget separates the two allowed outcomes.
			timer := time.NewTimer(350 * time.Millisecond)
			<-timer.C
			close(r.release)
			op := budgetJoinPrivateV2(t, m, stop.IntentID)
			f, _ := op.CleanupObservationPrivateV2()
			if private {
				if op.State != OperationComplete || f.ShutdownRead != ShutdownMatchedPrivateV2 || f.ResponseBudgetSource != "PRIVATE_V2" || f.DeclaredResponseBudget != time.Second || f.EffectiveResponseReadBudget <= 250*time.Millisecond {
					t.Fatalf("ASSERT_PRIVATE_DELAYED_SUCCESS: %+v %+v", op, f)
				}
			} else if op.State != OperationFailed || f.ShutdownRead != ShutdownTimeoutPrivateV2 {
				t.Fatalf("ASSERT_ORDINARY_250MS: %+v %+v", op, f)
			}
			if c.teardowns.Load() != 1 || c.closes.Load() != 1 || c.shutdowns.Load() != 1 {
				t.Fatal("ASSERT_SINGLE_OWNER", f)
			}
		})
	}
}

func TestShutdownResponseBudgetPrivateV2Admission(t *testing.T) {
	for _, d := range []time.Duration{1, MaxShutdownResponseBudgetPrivateV2} {
		if _, e := NewShutdownResponseBudgetPrivateV2(d); e != nil {
			t.Fatal("positive boundary refused", d, e)
		}
	}
	for _, d := range []time.Duration{0, -1, MaxShutdownResponseBudgetPrivateV2 + 1, time.Duration(1<<63 - 1)} {
		if _, e := NewShutdownResponseBudgetPrivateV2(d); e == nil {
			t.Fatal("ASSERT_INVALID_CONSTRUCTOR", d)
		}
	}
	c := newShutdownLoopChild("exact")
	m, s := budgetManagerPrivateV2(t, c)
	good := mustBudgetPrivateV2(t, time.Second)
	raw, _ := json.Marshal(good)
	var restored ShutdownResponseBudgetPrivateV2
	if e := json.Unmarshal(raw, &restored); e != nil {
		t.Fatal(e)
	}
	for _, b := range []ShutdownResponseBudgetPrivateV2{{}, restored} {
		before := m.Census()
		if got := m.StopWithShutdownResponseBudgetPrivateV2(context.Background(), s, "invalid", b); got.Failure == "" || got.IntentID != "" || m.Census() != before {
			t.Fatal("ASSERT_REFUSE_BEFORE_ADMISSION", got)
		}
	}
	stop := m.StopWithShutdownResponseBudgetPrivateV2(context.Background(), s, "budget", good)
	op := budgetJoinPrivateV2(t, m, stop.IntentID)
	if op.State != OperationComplete {
		t.Fatal(op)
	}
	for _, b := range []ShutdownResponseBudgetPrivateV2{mustBudgetPrivateV2(t, 2*time.Second), {}} {
		if got := m.StopWithShutdownResponseBudgetPrivateV2(context.Background(), s, "budget", b); got.Failure == "" {
			t.Fatal("ASSERT_INCOMPATIBLE_REPLAY", got)
		}
	}
	if got := m.Stop(context.Background(), s, "budget"); got.Failure == "" {
		t.Fatal("ordinary replay changed private source")
	}
	f, _ := op.CleanupObservationPrivateV2()
	f.ResponseBudgetSource = "changed"
	f.DeclaredResponseBudget = 0
	again, _ := op.CleanupObservationPrivateV2()
	if again.ResponseBudgetSource != "PRIVATE_V2" || again.DeclaredResponseBudget != time.Second {
		t.Fatal("ASSERT_IMMUTABLE_FACTS")
	}
	encoded, _ := json.Marshal(op)
	expected, _ := json.Marshal(struct {
		ID, SessionID, CallerID string
		Generation              uint64
		Restart                 bool
		State                   OperationState
		Failure                 session.Failure
	}{op.ID, op.SessionID, op.CallerID, op.Generation, op.Restart, op.State, op.Failure})
	if !bytes.Equal(encoded, expected) {
		t.Fatal("ASSERT_ORDINARY_OPERATION_JSON_UNCHANGED", string(encoded))
	}
	if bytes.Contains(encoded, []byte("Budget")) || bytes.Contains(encoded, []byte("PRIVATE_V2")) || bytes.Contains(encoded, []byte("cleanup")) {
		t.Fatal("ASSERT_PRIVATE_JSON", string(encoded))
	}
}

func TestShutdownResponseBudgetPrivateV2CancellationAndJoin(t *testing.T) {
	for _, cancelled := range []bool{false, true} {
		t.Run(map[bool]string{false: "expires", true: "cancelled"}[cancelled], func(t *testing.T) {
			c := newShutdownLoopChild("timeout")
			r := &heldShutdownReader{ReadCloser: c.stdout, interrupted: make(chan struct{}), release: make(chan struct{})}
			entered := make(chan struct{})
			m, s := budgetManagerPrivateV2(t, &budgetChildPrivateV2{c, &enteredReaderPrivateV2{ReadCloser: r, entered: entered}})
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			b := mustBudgetPrivateV2(t, 30*time.Millisecond)
			if cancelled {
				b = mustBudgetPrivateV2(t, time.Second)
			}
			stop := m.StopWithShutdownResponseBudgetPrivateV2(ctx, s, "budget", b)
			<-entered
			if cancelled {
				cancel()
			}
			<-r.interrupted
			m.mu.Lock()
			accepted := m.operations[stop.IntentID].shutdownResponsePrivateV2
			m.mu.Unlock()
			joined := m.StopWithShutdownResponseBudgetPrivateV2(context.Background(), s, "budget", b)
			if joined.IntentID != stop.IntentID || !joined.Replayed {
				t.Fatal("ASSERT_SAME_BUDGET_REPLAY", joined)
			}
			m.mu.Lock()
			retained := m.operations[stop.IntentID].shutdownResponsePrivateV2
			m.mu.Unlock()
			if retained != accepted || retained.deadline != accepted.deadline || retained.ctx != ctx {
				t.Fatal("ASSERT_JOIN_DEADLINE_IMMUTABLE")
			}
			// A joiner cannot replace or extend accepted state, even with another caller.
			if got := m.StopWithShutdownResponseBudgetPrivateV2(context.Background(), s, "other", mustBudgetPrivateV2(t, 2*time.Second)); got.Failure != "" && got.Failure != session.LifecycleConflict {
				t.Fatal(got)
			} else if got.Failure == "" {
				t.Fatal("ASSERT_JOIN_EXTENSION_REFUSED", got)
			}
			short, done := context.WithCancel(context.Background())
			done()
			if e := m.Shutdown(short); e == nil {
				t.Fatal("ASSERT_NONJOIN_FAILURE")
			}
			pending, _ := m.Operation(stop.IntentID)
			if pending.State != OperationPending {
				t.Fatal("ASSERT_WORKER_OWNS_READJOIN", pending)
			}
			close(r.release)
			op := budgetJoinPrivateV2(t, m, stop.IntentID)
			f, _ := op.CleanupObservationPrivateV2()
			want := ShutdownTimeoutPrivateV2
			if cancelled {
				want = ShutdownCancelledPrivateV2
			}
			if op.State != OperationFailed || f.ShutdownRead != want || f.ExitWrite != "" || c.teardowns.Load() != 1 || c.closes.Load() != 1 {
				t.Fatal("ASSERT_PRIVATE_END_REFUSES_SUCCESS", op, f)
			}
		})
	}
}

func TestShutdownResponseBudgetPrivateV2PendingOwnership(t *testing.T) {
	c := newShutdownLoopChild("exact")
	var pending *lspwire.Pending
	pending = lspwire.NewPendingObserved(2, func(e lspwire.Event) {
		if e.Stage == lspwire.EventPendingBegin {
			pending.Accept(lspwire.ResponseKey{Generation: 1, ID: 1})
		}
	})
	b := mustBudgetPrivateV2(t, time.Second)
	m := &Manager{limits: Limits{MaxObservations: 4}, wire: lspwire.DefaultLimits()}
	observed := m.gracefulShutdownWithResponsePrivateV2(c, pending, 1, &acceptedShutdownResponsePrivateV2{budget: b, deadline: time.Now().Add(time.Second), ctx: context.Background()})
	_ = c.Teardown(context.Background())
	_ = c.Close()
	if observed.acknowledged || observed.exitNotified || observed.facts.ShutdownRead != ShutdownUnexpectedPrivateV2 {
		t.Fatal("ASSERT_PRIVATE_PENDING_OWNERSHIP", observed.facts)
	}
}

func TestShutdownResponseBudgetPrivateV2LimitsAndOwnership(t *testing.T) {
	for _, mode := range []string{"coalesced", "wrong-id", "count", "bytes-plus"} {
		t.Run(mode, func(t *testing.T) {
			c := newShutdownLoopChild(mode)
			m, s := budgetManagerPrivateV2(t, c)
			stop := m.StopWithShutdownResponseBudgetPrivateV2(context.Background(), s, "budget", mustBudgetPrivateV2(t, time.Second))
			op := budgetJoinPrivateV2(t, m, stop.IntentID)
			f, _ := op.CleanupObservationPrivateV2()
			want := map[string]ShutdownReadPrivateV2{"coalesced": ShutdownMatchedPrivateV2, "wrong-id": ShutdownUnexpectedPrivateV2, "count": ShutdownCountLimitPrivateV2, "bytes-plus": ShutdownByteLimitPrivateV2}[mode]
			if f.ShutdownRead != want || c.shutdowns.Load() != 1 || c.teardowns.Load() != 1 || c.closes.Load() != 1 {
				t.Fatal("ASSERT_PRIVATE_LIMITS_UNCHANGED", f)
			}
		})
	}
}
