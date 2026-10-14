package incomingops

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"lsp-trace/internal/acquisition"
	"lsp-trace/internal/directionalraw"
	"lsp-trace/internal/operation"
	"lsp-trace/internal/session"
	"lsp-trace/sessionruntime"
	"time"
)

type directionalPrivateRuntime interface {
	RoundTripDirectionalPrivate(context.Context, sessionruntime.RoundTripRequest) (sessionruntime.RoundTripResult, directionalraw.Lease)
}

func directionalPrivateEnabled(ctx context.Context, method string) bool {
	_, ok := directionalraw.FromContext(ctx)
	return ok && directionalraw.Allowed(method)
}

// ExecuteDirectionalPrivate is opt-in process-local ingress only. Caller owns
// the returned reservation's Release, which converges on the generic owner.
func (e *Executor) ExecuteDirectionalPrivate(parent context.Context, op operation.Request, transactionID string) (operation.Result, *operation.Failure, directionalraw.Reservation) {
	r := directionalraw.NewReservation(transactionID)
	result, failure := e.Execute(r.Context(parent), op)
	return result, failure, r
}
func (c *SessionClient) callDirectionalPrivate(ctx context.Context, method string, params, target any) (bool, error) {
	runtime, ok := c.runtime.(directionalPrivateRuntime)
	if !ok {
		return false, errors.New("directional runtime unavailable")
	}
	raw, err := json.Marshal(params)
	if err != nil {
		return false, err
	}
	deadline := time.Now().Add(c.requestTimeout)
	if d, ok := ctx.Deadline(); ok && d.Before(deadline) {
		deadline = d
	}
	observed, lease := runtime.RoundTripDirectionalPrivate(ctx, sessionruntime.RoundTripRequest{SessionID: c.sessionID, Generation: c.generation, Method: method, Params: raw, Deadline: deadline, MaxMessages: c.wireLimits.MaxMessages, MaxBytes: c.wireLimits.MaxBytes})
	if observed.Failure != "" {
		switch observed.Failure {
		case session.RequestCancelled:
			return false, context.Canceled
		case session.RequestTimeout:
			return false, context.DeadlineExceeded
		default:
			return false, fmt.Errorf("%s", observed.Failure)
		}
	}
	if observed.ServerError != nil {
		return false, &rpcError{code: observed.ServerError.Code, message: observed.ServerError.Message}
	}
	if err = acquisition.BindDirectionalResponse(lease); err != nil {
		return false, err
	}
	if lease.Null() {
		return true, nil
	}
	return false, lease.Decode(target)
}
