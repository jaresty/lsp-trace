package censuslineagepilot

import (
	"context"

	"lsp-trace/internal/session"
	"lsp-trace/sessionruntime"
)

// ScopeFailureObservationPrivateV2 is detached, content-free accounting. It is
// neither a producer packet nor an origin, and grants no continuation authority.
// Stage is assigned at the failing operation, never inferred from ErrInvalid.
// Unknown ordinals are nil, not zero. No error text or provider data is retained.
type ScopeFailureObservationPrivateV2 struct {
	Stage                 string
	Class                 string
	Subcause              string
	DiscoveryAvailability string
	MemberOrdinal         *int `json:",omitempty"`
}

func (r ScopeDiscoveryReportPrivateV2) FailureObservationPrivateV2() (ScopeFailureObservationPrivateV2, bool) {
	if r.failure == nil {
		return ScopeFailureObservationPrivateV2{}, false
	}
	out := *r.failure
	if out.MemberOrdinal != nil {
		n := *out.MemberOrdinal
		out.MemberOrdinal = &n
	}
	return out, true
}

// ScopeFailureClassPrivateV2 does bounded work over owned error chains. It never
// calls Error or arbitrary Is/As methods. An unexposed cause remains UNKNOWN;
// exhausting the traversal budget is explicit rather than silently conclusive.
func ScopeFailureClassPrivateV2(err error) string {
	if err == nil {
		return "UNKNOWN"
	}
	pending := []error{err}
	for work := 0; len(pending) > 0 && work < 32; work++ {
		e := pending[0]
		pending = pending[1:]
		if e == nil {
			continue
		}
		switch e {
		case context.Canceled:
			return "CANCELLED"
		case context.DeadlineExceeded:
			return "DEADLINE_EXCEEDED"
		case ErrInvalid:
			return "INVALID_UNSPECIFIED"
		case ErrScopeDiscoveryIncompletePrivateV2:
			return "DISCOVERY_INCOMPLETE"
		case ErrScopeDiscoveryEmptyPrivateV2:
			return "DISCOVERY_EMPTY"
		}
		if f, ok := e.(scopeRuntimeFailurePrivate); ok {
			return scopeFailureRuntimeCodePrivateV2(f.Failure)
		}
		switch wrapped := e.(type) {
		case interface{ Unwrap() []error }:
			children := wrapped.Unwrap()
			if len(children) > 32-len(pending) {
				return "CAUSE_TRAVERSAL_BOUND"
			}
			pending = append(pending, children...)
		case interface{ Unwrap() error }:
			pending = append(pending, wrapped.Unwrap())
		}
	}
	if len(pending) > 0 {
		return "CAUSE_TRAVERSAL_BOUND"
	}
	return "UNKNOWN"
}

func scopeFailureRuntimeCodePrivateV2(f session.Failure) string {
	// Closed whitelist: a future runtime code cannot become a private message.
	switch f {
	case session.RequestCancelled:
		return "REQUEST_CANCELLED"
	case session.RequestTimeout:
		return "REQUEST_TIMEOUT"
	case session.ResourceExhausted:
		return "RESOURCE_EXHAUSTED"
	case sessionruntime.DocumentSupplyUnavailable:
		return "DOCUMENT_SUPPLY_UNAVAILABLE"
	case sessionruntime.DocumentSourceUnavailable:
		return "DOCUMENT_SOURCE_UNAVAILABLE"
	case sessionruntime.DocumentURIUnavailable:
		return "DOCUMENT_URI_UNAVAILABLE"
	case sessionruntime.DocumentOutsideWorkspace:
		return "DOCUMENT_OUTSIDE_WORKSPACE"
	case sessionruntime.LanguageIDUnavailable:
		return "LANGUAGE_ID_UNAVAILABLE"
	case session.LifecycleConflict:
		return "LIFECYCLE_CONFLICT"
	case session.SessionPoisoned:
		return "SESSION_POISONED"
	case session.SessionNotFound:
		return "SESSION_NOT_FOUND"
	case session.StaleGeneration:
		return "STALE_GENERATION"
	}
	return "RUNTIME_FAILURE_UNKNOWN"
}

func scopeFailureSubcausePrivateV2(sub sessionruntime.LifecycleSubcause) string {
	switch sub {
	case sessionruntime.LifecycleSubcauseManagerClosed:
		return "MANAGER_CLOSED"
	case sessionruntime.LifecycleSubcauseSessionNotReady:
		return "SESSION_NOT_READY"
	case sessionruntime.LifecycleSubcauseProtocolOwned:
		return "PROTOCOL_OWNED"
	case sessionruntime.LifecycleSubcauseLeaseClosing:
		return "LEASE_CLOSING"
	case sessionruntime.LifecycleSubcauseLeaseTokenMismatch:
		return "LEASE_TOKEN_MISMATCH"
	case sessionruntime.LifecycleSubcauseLanguageIDConflict:
		return "LANGUAGE_ID_CONFLICT"
	}
	return "UNKNOWN"
}
