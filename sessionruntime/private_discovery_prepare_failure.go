package sessionruntime

import (
	"context"
	"encoding/json"
	"errors"

	"lsp-trace/internal/discoveryfailure"
	"lsp-trace/internal/session"
)

// Static error values carry no RPC message, code, URI, or native error. Returning
// a pointer into this fixed table allocates no owned error object in a failed
// window; it neither spends residual currency nor changes existing precharges.
// The original ErrPrivateDiscovery identity and closed Error text are preserved.
type privateDiscoveryPrepareError struct{ cause discoveryfailure.Cause }

func (*privateDiscoveryPrepareError) Error() string                                 { return "private discovery enclosure refused" }
func (*privateDiscoveryPrepareError) Unwrap() error                                 { return ErrPrivateDiscovery }
func (e *privateDiscoveryPrepareError) PrepareFailureCause() discoveryfailure.Cause { return e.cause }
func (e *privateDiscoveryPrepareError) Is(target error) bool {
	return target == context.Canceled && e.cause == discoveryfailure.Canceled || target == context.DeadlineExceeded && e.cause == discoveryfailure.Deadline
}

var privateDiscoveryPrepareErrors = [...]privateDiscoveryPrepareError{
	{discoveryfailure.Unknown}, {discoveryfailure.Canceled}, {discoveryfailure.Deadline},
	{discoveryfailure.RPCServer}, {discoveryfailure.SessionUnavailable}, {discoveryfailure.SessionCrashed},
	{discoveryfailure.EnclosureBudget}, {discoveryfailure.InvalidResponse},
}

func privateDiscoveryPrepareCauseError(cause discoveryfailure.Cause) error {
	for i := range privateDiscoveryPrepareErrors {
		if privateDiscoveryPrepareErrors[i].cause == cause {
			return &privateDiscoveryPrepareErrors[i]
		}
	}
	return &privateDiscoveryPrepareErrors[0]
}

// Map only source-known closed failures. A nonempty unrecognized Failure wins
// over ServerError presence; it cannot silently become proof of a server error.
func privateDiscoveryPrepareRPCFailure(failure session.Failure, serverError bool) error {
	cause := discoveryfailure.Unknown
	switch failure {
	case session.RequestCancelled:
		cause = discoveryfailure.Canceled
	case session.RequestTimeout:
		cause = discoveryfailure.Deadline
	case session.SessionNotFound, session.StaleGeneration, session.SessionStopping, session.SessionPoisoned:
		cause = discoveryfailure.SessionUnavailable
	case session.SessionCrashed:
		cause = discoveryfailure.SessionCrashed
	case "":
		if serverError {
			cause = discoveryfailure.RPCServer
		}
	}
	return privateDiscoveryPrepareCauseError(cause)
}

// Snapshot only the existing first refusal. It is not a new recorder or an
// accounting action. Known amounts distinguish budget failure from generic
// enclosure refusal; an unknown amount never becomes an observed zero.
func (w *privateDiscoveryWindow) prepareFailure(ctxErr, err error) error {
	w.mu.Lock()
	refusal := w.refusal
	w.mu.Unlock()
	return privateDiscoveryPrepareFailure(ctxErr, err, refusal)
}
func privateDiscoveryPrepareFailure(ctxErr, err error, refusal PrivateDiscoveryRefusal) error {
	if refusal.Valid() && refusal.AmountsAvailable {
		return privateDiscoveryPrepareCauseError(discoveryfailure.EnclosureBudget)
	}
	if errors.Is(ctxErr, context.Canceled) {
		return privateDiscoveryPrepareCauseError(discoveryfailure.Canceled)
	}
	if errors.Is(ctxErr, context.DeadlineExceeded) {
		return privateDiscoveryPrepareCauseError(discoveryfailure.Deadline)
	}
	if typed, ok := err.(*privateDiscoveryPrepareError); ok {
		return typed
	}
	// These are the exact concrete errors returned by the json.Unmarshal call
	// below this boundary. No recursive error container or raw message is kept.
	switch err.(type) {
	case *json.SyntaxError, *json.UnmarshalTypeError:
		return privateDiscoveryPrepareCauseError(discoveryfailure.InvalidResponse)
	}
	return privateDiscoveryPrepareCauseError(discoveryfailure.Unknown)
}
