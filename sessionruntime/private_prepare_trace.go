package sessionruntime

import (
	"context"
	"runtime/trace"

	"lsp-trace/internal/session"
)

// Only fixed literals enter the trace. In particular, unknown typed failures
// and opaque errors are never formatted, unwrapped or compared via methods.
func privatePrepareTraceFailure(failure session.Failure) string {
	switch failure {
	case "":
		return "OK"
	case session.ToolNotImplemented:
		return "TOOL_NOT_IMPLEMENTED"
	case session.ProcessContainmentUnavailable:
		return "PROCESS_CONTAINMENT_UNAVAILABLE"
	case session.LiveLSPDisabled:
		return "LIVE_LSP_DISABLED"
	case session.SessionNotFound:
		return "SESSION_NOT_FOUND"
	case session.ListCursorInvalid:
		return "LIST_CURSOR_INVALID"
	case session.StaleGeneration:
		return "STALE_GENERATION"
	case session.LifecycleConflict:
		return "LIFECYCLE_CONFLICT"
	case session.SessionReapIncomplete:
		return "SESSION_REAP_INCOMPLETE"
	case session.SessionStopping:
		return "SESSION_STOPPING"
	case session.SessionPoisoned:
		return "SESSION_POISONED"
	case session.ResourceExhausted:
		return "RESOURCE_EXHAUSTED"
	case session.SpawnFailure:
		return "SPAWN_FAILED"
	case session.PipeSetupFailure:
		return "PIPE_SETUP_FAILED"
	case session.InitializationFailure:
		return "INITIALIZATION_FAILED"
	case session.InitializationTimeout:
		return "INITIALIZATION_TIMEOUT"
	case session.SessionCrashed:
		return "SESSION_CRASHED"
	case session.RequestCancelled:
		return "REQUEST_CANCELLED"
	case session.RequestTimeout:
		return "REQUEST_TIMEOUT"
	default:
		return "UNKNOWN"
	}
}

func privatePrepareTraceError(err error) string {
	switch err {
	case nil:
		return "NONE"
	case context.Canceled:
		return "CANCELED"
	case context.DeadlineExceeded:
		return "DEADLINE"
	case errSessionLeaseConflict:
		return "LEASE_CONFLICT"
	default:
		return "UNKNOWN"
	}
}

func privatePrepareTracePresence(ctx context.Context, serverError, pair bool) {
	if serverError {
		trace.Log(ctx, "private.prepare.server_error", "PRESENT")
	} else {
		trace.Log(ctx, "private.prepare.server_error", "ABSENT")
	}
	if pair {
		trace.Log(ctx, "private.prepare.owned_pair", "PRESENT")
	} else {
		trace.Log(ctx, "private.prepare.owned_pair", "ABSENT")
	}
}

func privatePrepareTraceLeaseCancel(failure session.Failure, serverError, pair bool) {
	if !trace.IsEnabled() {
		return
	}
	ctx := context.Background()
	// This position is ABOUT_TO_CANCEL, not evidence cancellation won a race.
	trace.Log(ctx, "private.prepare.lease", "ABOUT_TO_CANCEL")
	trace.Log(ctx, "private.prepare.failure", privatePrepareTraceFailure(failure))
	privatePrepareTracePresence(ctx, serverError, pair)
}

// privatePrepareTraceMessages emits at most one fixed-layout annotation at the
// capture checkpoint, including failed captures. It shares the existing opt-in
// trace and its cap; it creates no logger, transport, store, or default artifact.
func privatePrepareTraceMessages(ctx context.Context, result RoundTripResult) {
	if !trace.IsEnabled() {
		return
	}
	trace.Log(ctx, "private.prepare.messages", privateProviderMessageSummaryJSON(result))
}

func privatePrepareTraceTransaction(ctx context.Context, sampledErr error, failure session.Failure, serverError, pair bool) {
	if !trace.IsEnabled() {
		return
	}
	// The context_err field uses the original passed ctx.Err sample. Separately,
	// context.Cause samples via the standard API, which may itself call Err.
	// These adjacent samples are non-atomic and do not prove causal ordering.
	cause := context.Cause(ctx)
	trace.Log(ctx, "private.prepare.transaction", "RETURN")
	trace.Log(ctx, "private.prepare.failure", privatePrepareTraceFailure(failure))
	trace.Log(ctx, "private.prepare.context_err", privatePrepareTraceError(sampledErr))
	trace.Log(ctx, "private.prepare.context_cause", privatePrepareTraceError(cause))
	privatePrepareTracePresence(ctx, serverError, pair)
}
