//go:build darwin || linux

package publication

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

// Invocation-local observation only: storageLock calls Done after a real
// nonblocking flock returned EWOULDBLOCK/EAGAIN. Forward the actual context;
// neither manufacture expiration nor replace a production lock operation.
type quotaLockObservedContext struct {
	context.Context
	blocked chan struct{}
	once    sync.Once
}

func (c *quotaLockObservedContext) Done() <-chan struct{} {
	c.once.Do(func() { close(c.blocked) })
	return c.Context.Done()
}

func awaitQuotaBlocked(t *testing.T, blocked <-chan struct{}) bool {
	t.Helper()
	select {
	case <-blocked:
		t.Log("ACTUAL_FLOCK_CONTENTION_OBSERVED")
		return true
	case <-time.After(2 * time.Second):
		t.Error("FLOCK_CONTENTION_NOT_OBSERVED")
		return false
	}
}
func awaitQuotaResult(t *testing.T, done <-chan error) (error, bool) {
	t.Helper()
	select {
	case e := <-done:
		return e, true
	case <-time.After(2 * time.Second):
		t.Error("BOUNDED_LOCK_WORKER_DID_NOT_FINISH")
		return nil, false
	}
}

func TestStorageQuotaExpiredContextClassification(t *testing.T) {
	for _, kind := range []string{"pre-cancelled", "pre-deadline", "nil"} {
		t.Run(kind, func(t *testing.T) {
			r := quotaRoot(t, 2<<20)
			var ctx context.Context
			var want, wrong error
			switch kind {
			case "pre-cancelled":
				c, cancel := context.WithCancel(context.Background())
				cancel()
				ctx, want, wrong = c, context.Canceled, context.DeadlineExceeded
			case "pre-deadline":
				c, cancel := context.WithDeadline(context.Background(), time.Unix(1, 0))
				defer cancel()
				ctx, want, wrong = c, context.DeadlineExceeded, context.Canceled
			}
			if ctx != nil && ctx.Err() != want {
				t.Fatal("fixture is not already expired", ctx.Err())
			}
			before := *quotaStatus(t, r)
			q, e := ReserveStorage(ctx, r, 1)
			if q != nil {
				t.Error("expired/nil context admitted")
				if ce := q.Complete(false); ce != nil {
					t.Error(ce)
				}
			}
			if !errors.Is(e, ErrStorageIntegrity) {
				t.Error("INTEGRITY_WRAPPER_LOST", e)
			}
			if kind == "nil" {
				if errors.Is(e, context.Canceled) || errors.Is(e, context.DeadlineExceeded) {
					t.Error("NIL_CONTEXT_FABRICATED_EXPIRY", e)
				}
			} else if !errors.Is(e, want) || errors.Is(e, wrong) {
				t.Errorf("EXPIRED_CONTEXT_CLASSIFICATION kind=%s want=%v got=%v", kind, want, e)
			}
			// Classification failure must not bypass release/accounting qualification.
			assertCASRepairLocksReleased(t, r)
			statusCtx, statusCancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
			after, statusErr := ReadStorageStatus(statusCtx, r)
			statusCancel()
			if statusErr != nil || after == nil {
				t.Error("POST_EXPIRY_STATUS_UNAVAILABLE", statusErr)
			} else if after.ReservedBytes != before.ReservedBytes || after.FileBytes != before.FileBytes {
				t.Error("EXPIRED_CONTEXT_CHANGED_DEBT_OR_CONTENT", before, after)
			}
			t.Log("EXPIRED_CONTEXT_RELEASE_CHECKS_COMPLETED")
		})
	}
}

func TestStorageQuotaQueuedCancellation(t *testing.T) {
	r := quotaRoot(t, 2<<20)
	held, e := ReserveStorage(context.Background(), r, 0)
	if e != nil {
		t.Fatal(e)
	}
	defer func() {
		if held != nil {
			if e := held.Complete(true); e != nil {
				t.Error(e)
			}
		}
	}()
	base, cancel := context.WithCancel(context.Background())
	defer cancel()
	ctx := &quotaLockObservedContext{Context: base, blocked: make(chan struct{})}
	done := make(chan error, 1)
	go func() {
		q, e := ReserveStorage(ctx, r, 0)
		if q != nil {
			e = errors.Join(e, q.Complete(true))
		}
		done <- e
	}()
	awaitQuotaBlocked(t, ctx.blocked)
	cancel()
	e, finished := awaitQuotaResult(t, done)
	if finished && (!errors.Is(e, context.Canceled) || errors.Is(e, context.DeadlineExceeded)) {
		t.Error("QUEUED_CANCEL_CLASSIFICATION", e)
	}
	if e := held.Complete(true); e != nil {
		t.Error(e)
	}
	held = nil
	if !finished {
		awaitQuotaResult(t, done)
	}
	assertCASRepairLocksReleased(t, r)
	t.Log("QUEUED_CANCEL_RELEASE_CHECKS_COMPLETED")
}
