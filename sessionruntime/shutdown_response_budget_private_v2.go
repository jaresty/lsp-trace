package sessionruntime

import (
	"context"
	"errors"
	"time"

	"lsp-trace/internal/session"
)

// ShutdownResponseBudgetPrivateV2 is an in-process response allowance, not
// acquisition authority. Its zero value and JSON-restored values are invalid.
// No mutable exported fields or restoration API exist.
type ShutdownResponseBudgetPrivateV2 struct {
	duration    time.Duration
	constructed bool
}

// MaxShutdownResponseBudgetPrivateV2 matches the private caller cleanup ceiling.
const MaxShutdownResponseBudgetPrivateV2 = 10 * time.Second

func NewShutdownResponseBudgetPrivateV2(d time.Duration) (ShutdownResponseBudgetPrivateV2, error) {
	if d <= 0 || d > MaxShutdownResponseBudgetPrivateV2 {
		return ShutdownResponseBudgetPrivateV2{}, errors.New("sessionruntime: invalid private shutdown response budget")
	}
	return ShutdownResponseBudgetPrivateV2{duration: d, constructed: true}, nil
}

type acceptedShutdownResponsePrivateV2 struct {
	budget   ShutdownResponseBudgetPrivateV2
	deadline time.Time
	ctx      context.Context
}

// StopWithShutdownResponseBudgetPrivateV2 uses ordinary lifecycle admission and
// worker ownership. The response deadline is calculated once here, capped by the
// cleanup context. Time spent writing shutdown consumes it. Synchronous writes,
// teardown, close and reader join are NOT globally bounded by this allowance;
// Shutdown may return a nonjoined context failure while the worker still owns them.
func (m *Manager) StopWithShutdownResponseBudgetPrivateV2(ctx context.Context, id, caller string, b ShutdownResponseBudgetPrivateV2) session.LifecycleResult {
	if !b.constructed || b.duration <= 0 || b.duration > MaxShutdownResponseBudgetPrivateV2 || ctx == nil {
		return session.LifecycleResult{Failure: session.LifecycleConflict}
	}
	deadline := time.Now().Add(b.duration)
	if cap, ok := ctx.Deadline(); ok && cap.Before(deadline) {
		deadline = cap
	}
	return m.terminateWithShutdownResponsePrivateV2(ctx, id, caller, false, &acceptedShutdownResponsePrivateV2{budget: b, deadline: deadline, ctx: ctx})
}

func compatibleShutdownResponsePrivateV2(a, b *acceptedShutdownResponsePrivateV2) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	// The accepted operation retains its original context/deadline. A same-budget
	// join may observe it, never install the joiner's later allowance or cancellation.
	return a.budget == b.budget
}

func (a *acceptedShutdownResponsePrivateV2) ended() ShutdownReadPrivateV2 {
	if a.ctx.Err() == context.Canceled {
		return ShutdownCancelledPrivateV2
	}
	if a.ctx.Err() != nil || !time.Now().Before(a.deadline) {
		return ShutdownTimeoutPrivateV2
	}
	return ""
}
