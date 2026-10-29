package sessionruntime

import (
	"context"
	"io"
	"testing"
	"time"
)

type shutdownWriteGatePrivateV2 struct {
	io.WriteCloser
	entered, release chan struct{}
	first            bool
}

func (w *shutdownWriteGatePrivateV2) Write(b []byte) (int, error) {
	if !w.first {
		w.first = true
		close(w.entered)
		<-w.release
	}
	return w.WriteCloser.Write(b)
}

type shutdownWriteChildPrivateV2 struct {
	*shutdownLoopChild
	writer *shutdownWriteGatePrivateV2
}

func (c *shutdownWriteChildPrivateV2) Stdin() io.WriteCloser { return c.writer }

func TestShutdownResponseBudgetPrivateV2WriteConsumesDeadline(t *testing.T) {
	c := newShutdownLoopChild("exact")
	w := &shutdownWriteGatePrivateV2{WriteCloser: c.stdin, entered: make(chan struct{}), release: make(chan struct{})}
	m, s := budgetManagerPrivateV2(t, &shutdownWriteChildPrivateV2{c, w})
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Millisecond)
	defer cancel()
	stop := m.StopWithShutdownResponseBudgetPrivateV2(ctx, s, "write", mustBudgetPrivateV2(t, time.Second))
	<-w.entered
	<-ctx.Done()
	close(w.release)
	op := budgetJoinPrivateV2(t, m, stop.IntentID)
	f, _ := op.CleanupObservationPrivateV2()
	if op.State != OperationFailed || f.ShutdownRead != ShutdownTimeoutPrivateV2 || f.EffectiveResponseReadBudget != 0 || f.ReadFrames != 0 || f.ExitWrite != "" {
		t.Fatal("ASSERT_WRITE_CONSUMES_DEADLINE", op, f)
	}
}

func TestShutdownResponseBudgetPrivateV2PriorCleanupTimeConsumesAllowance(t *testing.T) {
	// This is the same cleanup context supplied after a readiness wait. Its spent
	// deadline must not be replaced by the fresh private response allowance.
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	c := newShutdownLoopChild("exact")
	m, s := budgetManagerPrivateV2(t, c)
	// Isolate WaitReadiness's join allowance from transport retirement. No
	// readiness worker or provider is started by this synthetic pending record.
	m.mu.Lock()
	m.readiness["owned-wait"] = &readinessOperation{snapshot: ReadinessSnapshot{State: ReadinessPending}, done: make(chan struct{})}
	m.mu.Unlock()
	waited, ok := m.WaitReadiness(ctx, "owned-wait")
	if !ok || waited.State != ReadinessPending || ctx.Err() != context.DeadlineExceeded {
		t.Fatal("readiness wait did not consume allowance")
	}
	stop := m.StopWithShutdownResponseBudgetPrivateV2(ctx, s, "spent-readiness", mustBudgetPrivateV2(t, time.Second))
	op := budgetJoinPrivateV2(t, m, stop.IntentID)
	f, _ := op.CleanupObservationPrivateV2()
	if op.State != OperationFailed || f.ShutdownRead != ShutdownTimeoutPrivateV2 || f.EffectiveResponseReadBudget != 0 || f.ReadFrames != 0 {
		t.Fatal("ASSERT_SPENT_CLEANUP_CONTEXT", op, f)
	}
}
