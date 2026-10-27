package censuslineagepilot

import (
	"context"
	"encoding/json"
	"time"

	"lsp-trace/internal/acquisition"
	"lsp-trace/internal/session"
)

// Wire bytes and admitted result bytes are distinct ceilings. Only direct calls
// add the fixed envelope allowance to their internally owned result budget;
// native declared wire limits are never increased by that allowance.
const scopeAdapterWireBytes = scopeAdapterResponse + scopeAdapterResponseEnvelope
const scopeMinimumResponseBody = len(`{"jsonrpc":"2.0","id":0,"result":[]}`)

type scopeEffectiveWireBounds struct {
	messages int
	bytes    int
	deadline time.Time
}

func scopeIntersectWireBounds(now, lifetime, callerDeadline time.Time, remaining, messages, wireBytes, aggregateEnvelope int) (scopeEffectiveWireBounds, error) {
	if messages <= 0 || wireBytes <= 0 || remaining <= 0 || remaining > scopeAdapterCapture ||
		(aggregateEnvelope != 0 && aggregateEnvelope != scopeAdapterResponseEnvelope) {
		return scopeEffectiveWireBounds{}, ErrInvalid
	}
	b := scopeEffectiveWireBounds{messages: min(messages, scopeAdapterMessages), bytes: min(wireBytes, scopeAdapterWireBytes, remaining+aggregateEnvelope), deadline: now.Add(scopeAdapterRequestTimeout)}
	if lifetime.Before(b.deadline) {
		b.deadline = lifetime
	}
	if !callerDeadline.IsZero() && callerDeadline.Before(b.deadline) {
		b.deadline = callerDeadline
	}
	// Even an empty successful response needs a JSON-RPC envelope. Refuse an
	// unrepresentable wire budget before transport; do not silently inflate it.
	if b.bytes < scopeMinimumResponseBody {
		return scopeEffectiveWireBounds{}, ErrInvalid
	}
	if !b.deadline.After(now) {
		return scopeEffectiveWireBounds{}, context.DeadlineExceeded
	}
	return b, nil
}

// Requires invocationMu, not mu. No other invocation can consume aggregate
// allowance between deriving these bounds and complete response admission.
func (a *scopeRuntimeAdapter) wireBoundsLocked(ctx context.Context, messages, wireBytes, aggregateEnvelope int) (scopeEffectiveWireBounds, error) {
	if a == nil || ctx == nil {
		return scopeEffectiveWireBounds{}, ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return scopeEffectiveWireBounds{}, err
	}
	callerDeadline, _ := ctx.Deadline()
	a.mu.Lock()
	remaining := scopeAdapterCapture - a.captured
	a.mu.Unlock()
	return scopeIntersectWireBounds(time.Now(), a.deadline, callerDeadline, remaining, messages, wireBytes, aggregateEnvelope)
}

// The production native callback and fake-wire qualification share this exact
// seam. It accepts the declared WireRequest, never a caller-minted guard.
func (a *scopeRuntimeAdapter) nativeWireRoundTripLocked(ctx context.Context, r acquisition.WireRequest) (json.RawMessage, error) {
	if a == nil || r.Context.SessionID != a.identity.SessionID || r.Context.Generation != a.identity.Generation {
		return nil, ErrInvalid
	}
	bounds, err := a.wireBoundsLocked(ctx, r.MaxMessages, r.MaxBytes, 0)
	if err != nil {
		return nil, err
	}
	return a.roundTripWithBoundsLocked(ctx, r.Method, r.Params, bounds)
}

// Retain Manager's closed failure classification, without translating consumed
// budget failures into parser errors, lifecycle errors, or synthetic custody.
type scopeRuntimeFailurePrivate struct{ Failure session.Failure }

func (e scopeRuntimeFailurePrivate) Error() string { return string(e.Failure) }
