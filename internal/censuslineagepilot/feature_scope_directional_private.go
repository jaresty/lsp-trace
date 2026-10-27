package censuslineagepilot

import (
	"context"
	"errors"
	"fmt"
	"io"

	"lsp-trace/internal/acquisition"
	"lsp-trace/internal/directionalraw"
	"lsp-trace/sessionruntime"
)

// Called only inside scopeNativeClient's invocation lock, which remains held
// through WireClient binding, shape checks and typed decoding. No reservation,
// lease, ordinary-result custody, or terminal authority is synthesized here.
func (a *scopeRuntimeAdapter) nativeDirectionalRoundTripLocked(ctx context.Context, r acquisition.WireRequest) (directionalraw.Lease, error) {
	var empty directionalraw.Lease
	if a == nil || ctx == nil || r.Context.SessionID != a.identity.SessionID || r.Context.Generation != a.identity.Generation || !directionalraw.Allowed(r.Method) {
		return empty, ErrInvalid
	}
	reservation, ok := directionalraw.FromContext(ctx)
	if !ok || reservation.IdentityPrivate() == "" || reservation.Closed() {
		return empty, ErrInvalid
	}
	bounds, err := a.wireBoundsLocked(ctx, r.MaxMessages, r.MaxBytes, 0)
	if err != nil {
		return empty, err
	}
	if len(r.Params) == 0 || len(r.Params) > scopeAdapterParams {
		return empty, ErrInvalid
	}
	if err = a.check(ctx); err != nil {
		return empty, err
	}
	a.mu.Lock()
	if a.requests >= scopeAdapterRequests || a.captured >= scopeAdapterCapture {
		a.mu.Unlock()
		return empty, ErrInvalid
	}
	a.requests++
	a.mu.Unlock()
	ctx, cancel := context.WithDeadline(ctx, bounds.deadline)
	defer cancel()
	observed, lease := a.manager.RoundTripDirectionalPrivate(ctx, sessionruntime.RoundTripRequest{SessionID: a.identity.SessionID, Generation: a.identity.Generation, RetainedScopePrivate: a.guard, Method: r.Method, Params: r.Params, Deadline: bounds.deadline, MaxMessages: bounds.messages, MaxBytes: int64(bounds.bytes)})
	if a.afterResponsePrivateTest != nil && observed.Failure == "" {
		a.afterResponsePrivateTest()
	}
	if observed.Failure != "" {
		err = scopeAdapterFailure(observed.Failure, "")
	} else if observed.ServerError != nil {
		err = fmt.Errorf("private scope json-rpc error %d", observed.ServerError.Code)
	}
	// Completed metadata is Manager-owned byte evidence, never an ordinary result
	// or receipt granting semantic/source authority. Charge it even on failure.
	metadata, completed := lease.CompletedMetadataPrivate()
	a.mu.Lock()
	if completed {
		n := metadata.ResultLengthPrivate
		if n > scopeAdapterResponse || n > uint64(scopeAdapterCapture-a.captured) {
			err = errors.Join(err, ErrInvalid)
		} else {
			a.captured += int(n)
		}
	}
	if err != nil {
		a.terminal = err
	}
	a.mu.Unlock()
	if err != nil {
		return lease, err
	}
	if !completed {
		a.mu.Lock()
		a.terminal = ErrInvalid
		a.mu.Unlock()
		return lease, ErrInvalid
	}
	// The stream copies into ephemeral decoder scratch, not retained raw custody.
	// The original lease alone crosses back to WireClient, which binds exactly once.
	raw, err := io.ReadAll(io.LimitReader(lease.ReaderPrivate(), scopeAdapterResponse+1))
	defer clear(raw)
	if err == nil {
		_, _, err = validateScopeResponse(r.Method, raw, "")
	}
	return lease, err
}
