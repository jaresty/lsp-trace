package sessionruntime

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"lsp-trace/internal/adr0011querytarget"
	"lsp-trace/internal/lspwire"
	"lsp-trace/internal/session"
)

var ErrPrivateGroupingOwned = errors.New("private grouping owned handoff refused")

// PrivateGroupingWireWithinDefaultEnvelope is legacy read-only metadata, not
// resource-enforcement evidence. The fixed prepaid owner uses the opaque concrete
// PrivateGroupingResourceEnvelope instead. Preserve this older predicate's behavior.
func (m *Manager) PrivateGroupingWireWithinDefaultEnvelope() bool {
	if m == nil {
		return false
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	d := lspwire.DefaultLimits()
	return m.wire.MaxBodyBytes >= 0 && m.wire.MaxBodyBytes <= d.MaxBodyBytes && m.wire.MaxHeaderBytes >= 0 && m.wire.MaxHeaderBytes <= d.MaxHeaderBytes
}

// PrivateGroupingOwnedRequest selects methods, not authority. Revision remains a
// caller assertion. This private entry neither starts a session nor grants one.
type PrivateGroupingOwnedRequest struct {
	Session                              SessionLeaseRequest
	URI, OccurrenceID, Revision, Privacy string
	Line, Character                      uint32
	References, Definitions              bool
	Deadline                             time.Time
}

// PrivateGroupingOwnedHandoff cannot be constructed from mutable pair snapshots.
// Its lifetime is strictly the currently active Manager-owned session lease.
type PrivateGroupingOwnedHandoff struct{ capture *privateGroupingCapture }
type privateGroupingCapture struct {
	manager  *Manager
	token    *sessionLeaseToken
	request  PrivateGroupingOwnedRequest
	view     PrivateGroupingOwnedView
	consumed bool
	// These markers are Manager-owned and never derived from a borrowed view.
	sourceConsumedSuccessfully bool
	sourceTaken                bool
}

// PrivateGroupingOwnedView is a borrowed diagnostic copy, not an issuance token.
// No API accepts this value as proof that a transaction ran.
type PrivateGroupingOwnedView struct {
	Query             adr0011querytarget.Query
	Target            adr0011querytarget.Candidate
	Source            []byte
	Supply            DocumentSupply
	Metadata          SessionMetadata
	Revision, Privacy string
	Workspace         string
	Deadline          time.Time
	Symbol            OwnedMethodPair
	Methods           []OwnedMethodPair
	Unavailable       []string
}

// PrivateGroupingCaptureResult is a detached, closed return-family observation.
// It grants no authority, carries no error/text, and never proves elapsed time.
type PrivateGroupingCaptureResult uint8

const (
	PrivateGroupingCaptureUnobserved PrivateGroupingCaptureResult = iota
	PrivateGroupingCaptureOK
	PrivateGroupingCaptureUnknown
	PrivateGroupingCapturePrecheckRefused
	PrivateGroupingCaptureResourceEnvelopeRefused
	PrivateGroupingCaptureEnvelopeContextRefused
	PrivateGroupingCaptureActiveLeaseRefused
	PrivateGroupingCaptureMetadataRefused
	PrivateGroupingCapturePreparationFailure
	PrivateGroupingCapturePreparationSupplyInvalid
	PrivateGroupingCaptureSourceBoundRefused
	PrivateGroupingCaptureTargetSelectionRefused
	PrivateGroupingCaptureTargetBoundRefused
	PrivateGroupingCaptureFinalLeaseRefused
	PrivateGroupingCaptureFinalBoundRefused
	PrivateGroupingCaptureQueryGuardRefused
	PrivateGroupingCaptureSymbolContextCanceled
	PrivateGroupingCaptureSymbolContextDeadline
	PrivateGroupingCaptureSymbolToolNotImplemented
	PrivateGroupingCaptureSymbolProcessContainmentUnavailable
	PrivateGroupingCaptureSymbolLiveLSPDisabled
	PrivateGroupingCaptureSymbolSessionNotFound
	PrivateGroupingCaptureSymbolListCursorInvalid
	PrivateGroupingCaptureSymbolStaleGeneration
	PrivateGroupingCaptureSymbolLifecycleConflict
	PrivateGroupingCaptureSymbolSessionReapIncomplete
	PrivateGroupingCaptureSymbolSessionStopping
	PrivateGroupingCaptureSymbolSessionPoisoned
	PrivateGroupingCaptureSymbolResourceExhausted
	PrivateGroupingCaptureSymbolSpawnFailure
	PrivateGroupingCaptureSymbolPipeSetupFailure
	PrivateGroupingCaptureSymbolInitializationFailure
	PrivateGroupingCaptureSymbolInitializationTimeout
	PrivateGroupingCaptureSymbolSessionCrashed
	PrivateGroupingCaptureSymbolRequestCancelled
	PrivateGroupingCaptureSymbolRequestTimeout
	PrivateGroupingCaptureSymbolUnknownFailure
	PrivateGroupingCaptureSymbolServerError
	PrivateGroupingCaptureSymbolMissingPair
	PrivateGroupingCaptureSymbolInvalidPair
	PrivateGroupingCaptureRelationContextCanceled
	PrivateGroupingCaptureRelationContextDeadline
	PrivateGroupingCaptureRelationToolNotImplemented
	PrivateGroupingCaptureRelationProcessContainmentUnavailable
	PrivateGroupingCaptureRelationLiveLSPDisabled
	PrivateGroupingCaptureRelationSessionNotFound
	PrivateGroupingCaptureRelationListCursorInvalid
	PrivateGroupingCaptureRelationStaleGeneration
	PrivateGroupingCaptureRelationLifecycleConflict
	PrivateGroupingCaptureRelationSessionReapIncomplete
	PrivateGroupingCaptureRelationSessionStopping
	PrivateGroupingCaptureRelationSessionPoisoned
	PrivateGroupingCaptureRelationResourceExhausted
	PrivateGroupingCaptureRelationSpawnFailure
	PrivateGroupingCaptureRelationPipeSetupFailure
	PrivateGroupingCaptureRelationInitializationFailure
	PrivateGroupingCaptureRelationInitializationTimeout
	PrivateGroupingCaptureRelationSessionCrashed
	PrivateGroupingCaptureRelationRequestCancelled
	PrivateGroupingCaptureRelationRequestTimeout
	PrivateGroupingCaptureRelationUnknownFailure
	PrivateGroupingCaptureRelationServerError
	PrivateGroupingCaptureRelationMissingPair
	PrivateGroupingCaptureRelationInvalidPair
	PrivateGroupingCapturePreparationToolNotImplemented
	PrivateGroupingCapturePreparationProcessContainmentUnavailable
	PrivateGroupingCapturePreparationLiveLSPDisabled
	PrivateGroupingCapturePreparationSessionNotFound
	PrivateGroupingCapturePreparationListCursorInvalid
	PrivateGroupingCapturePreparationStaleGeneration
	PrivateGroupingCapturePreparationLifecycleConflict
	PrivateGroupingCapturePreparationSessionReapIncomplete
	PrivateGroupingCapturePreparationSessionStopping
	PrivateGroupingCapturePreparationSessionPoisoned
	PrivateGroupingCapturePreparationResourceExhausted
	PrivateGroupingCapturePreparationSpawnFailure
	PrivateGroupingCapturePreparationPipeSetupFailure
	PrivateGroupingCapturePreparationInitializationFailure
	PrivateGroupingCapturePreparationInitializationTimeout
	PrivateGroupingCapturePreparationSessionCrashed
	PrivateGroupingCapturePreparationRequestCancelled
	PrivateGroupingCapturePreparationRequestTimeout
	PrivateGroupingCapturePreparationDocumentSupplyUnavailable
	PrivateGroupingCapturePreparationLanguageIDUnavailable
	PrivateGroupingCapturePreparationDocumentURIUnavailable
	PrivateGroupingCapturePreparationDocumentOutsideWorkspace
	PrivateGroupingCapturePreparationDocumentSourceUnavailable
	PrivateGroupingCapturePreparationWindowUnjoined
)

func (r PrivateGroupingCaptureResult) String() string {
	switch r {
	case PrivateGroupingCaptureUnobserved:
		return "UNOBSERVED"
	case PrivateGroupingCaptureOK:
		return "OK"
	case PrivateGroupingCaptureUnknown:
		return "UNKNOWN"
	case PrivateGroupingCapturePrecheckRefused:
		return "PRECHECK_REFUSED"
	case PrivateGroupingCaptureResourceEnvelopeRefused:
		return "RESOURCE_ENVELOPE_REFUSED"
	case PrivateGroupingCaptureEnvelopeContextRefused:
		return "ENVELOPE_CONTEXT_REFUSED"
	case PrivateGroupingCaptureActiveLeaseRefused:
		return "ACTIVE_LEASE_REFUSED"
	case PrivateGroupingCaptureMetadataRefused:
		return "METADATA_REFUSED"
	case PrivateGroupingCapturePreparationFailure:
		return "PREPARATION_UNKNOWN_FAILURE"
	case PrivateGroupingCapturePreparationSupplyInvalid:
		return "PREPARATION_SUPPLY_INVALID"
	case PrivateGroupingCaptureSourceBoundRefused:
		return "SOURCE_BOUND_REFUSED"
	case PrivateGroupingCaptureTargetSelectionRefused:
		return "TARGET_SELECTION_REFUSED"
	case PrivateGroupingCaptureTargetBoundRefused:
		return "TARGET_BOUND_REFUSED"
	case PrivateGroupingCaptureFinalLeaseRefused:
		return "FINAL_LEASE_REFUSED"
	case PrivateGroupingCaptureFinalBoundRefused:
		return "FINAL_BOUND_REFUSED"
	case PrivateGroupingCaptureQueryGuardRefused:
		return "QUERY_GUARD_REFUSED"
	case PrivateGroupingCaptureSymbolContextCanceled:
		return "SYMBOL_CONTEXT_CANCELED"
	case PrivateGroupingCaptureSymbolContextDeadline:
		return "SYMBOL_CONTEXT_DEADLINE"
	case PrivateGroupingCaptureSymbolToolNotImplemented:
		return "SYMBOL_TOOL_NOT_IMPLEMENTED"
	case PrivateGroupingCaptureSymbolProcessContainmentUnavailable:
		return "SYMBOL_PROCESS_CONTAINMENT_UNAVAILABLE"
	case PrivateGroupingCaptureSymbolLiveLSPDisabled:
		return "SYMBOL_LIVE_LSP_DISABLED"
	case PrivateGroupingCaptureSymbolSessionNotFound:
		return "SYMBOL_SESSION_NOT_FOUND"
	case PrivateGroupingCaptureSymbolListCursorInvalid:
		return "SYMBOL_LIST_CURSOR_INVALID"
	case PrivateGroupingCaptureSymbolStaleGeneration:
		return "SYMBOL_STALE_GENERATION"
	case PrivateGroupingCaptureSymbolLifecycleConflict:
		return "SYMBOL_LIFECYCLE_CONFLICT"
	case PrivateGroupingCaptureSymbolSessionReapIncomplete:
		return "SYMBOL_SESSION_REAP_INCOMPLETE"
	case PrivateGroupingCaptureSymbolSessionStopping:
		return "SYMBOL_SESSION_STOPPING"
	case PrivateGroupingCaptureSymbolSessionPoisoned:
		return "SYMBOL_SESSION_POISONED"
	case PrivateGroupingCaptureSymbolResourceExhausted:
		return "SYMBOL_RESOURCE_EXHAUSTED"
	case PrivateGroupingCaptureSymbolSpawnFailure:
		return "SYMBOL_SPAWN_FAILED"
	case PrivateGroupingCaptureSymbolPipeSetupFailure:
		return "SYMBOL_PIPE_SETUP_FAILED"
	case PrivateGroupingCaptureSymbolInitializationFailure:
		return "SYMBOL_INITIALIZATION_FAILED"
	case PrivateGroupingCaptureSymbolInitializationTimeout:
		return "SYMBOL_INITIALIZATION_TIMEOUT"
	case PrivateGroupingCaptureSymbolSessionCrashed:
		return "SYMBOL_SESSION_CRASHED"
	case PrivateGroupingCaptureSymbolRequestCancelled:
		return "SYMBOL_REQUEST_CANCELLED"
	case PrivateGroupingCaptureSymbolRequestTimeout:
		return "SYMBOL_REQUEST_TIMEOUT"
	case PrivateGroupingCaptureSymbolUnknownFailure:
		return "SYMBOL_UNKNOWN_FAILURE"
	case PrivateGroupingCaptureSymbolServerError:
		return "SYMBOL_SERVER_ERROR"
	case PrivateGroupingCaptureSymbolMissingPair:
		return "SYMBOL_MISSING_PAIR"
	case PrivateGroupingCaptureSymbolInvalidPair:
		return "SYMBOL_INVALID_PAIR"
	case PrivateGroupingCaptureRelationContextCanceled:
		return "RELATION_CONTEXT_CANCELED"
	case PrivateGroupingCaptureRelationContextDeadline:
		return "RELATION_CONTEXT_DEADLINE"
	case PrivateGroupingCaptureRelationToolNotImplemented:
		return "RELATION_TOOL_NOT_IMPLEMENTED"
	case PrivateGroupingCaptureRelationProcessContainmentUnavailable:
		return "RELATION_PROCESS_CONTAINMENT_UNAVAILABLE"
	case PrivateGroupingCaptureRelationLiveLSPDisabled:
		return "RELATION_LIVE_LSP_DISABLED"
	case PrivateGroupingCaptureRelationSessionNotFound:
		return "RELATION_SESSION_NOT_FOUND"
	case PrivateGroupingCaptureRelationListCursorInvalid:
		return "RELATION_LIST_CURSOR_INVALID"
	case PrivateGroupingCaptureRelationStaleGeneration:
		return "RELATION_STALE_GENERATION"
	case PrivateGroupingCaptureRelationLifecycleConflict:
		return "RELATION_LIFECYCLE_CONFLICT"
	case PrivateGroupingCaptureRelationSessionReapIncomplete:
		return "RELATION_SESSION_REAP_INCOMPLETE"
	case PrivateGroupingCaptureRelationSessionStopping:
		return "RELATION_SESSION_STOPPING"
	case PrivateGroupingCaptureRelationSessionPoisoned:
		return "RELATION_SESSION_POISONED"
	case PrivateGroupingCaptureRelationResourceExhausted:
		return "RELATION_RESOURCE_EXHAUSTED"
	case PrivateGroupingCaptureRelationSpawnFailure:
		return "RELATION_SPAWN_FAILED"
	case PrivateGroupingCaptureRelationPipeSetupFailure:
		return "RELATION_PIPE_SETUP_FAILED"
	case PrivateGroupingCaptureRelationInitializationFailure:
		return "RELATION_INITIALIZATION_FAILED"
	case PrivateGroupingCaptureRelationInitializationTimeout:
		return "RELATION_INITIALIZATION_TIMEOUT"
	case PrivateGroupingCaptureRelationSessionCrashed:
		return "RELATION_SESSION_CRASHED"
	case PrivateGroupingCaptureRelationRequestCancelled:
		return "RELATION_REQUEST_CANCELLED"
	case PrivateGroupingCaptureRelationRequestTimeout:
		return "RELATION_REQUEST_TIMEOUT"
	case PrivateGroupingCaptureRelationUnknownFailure:
		return "RELATION_UNKNOWN_FAILURE"
	case PrivateGroupingCaptureRelationServerError:
		return "RELATION_SERVER_ERROR"
	case PrivateGroupingCaptureRelationMissingPair:
		return "RELATION_MISSING_PAIR"
	case PrivateGroupingCaptureRelationInvalidPair:
		return "RELATION_INVALID_PAIR"
	case PrivateGroupingCapturePreparationToolNotImplemented:
		return "PREPARATION_TOOL_NOT_IMPLEMENTED"
	case PrivateGroupingCapturePreparationProcessContainmentUnavailable:
		return "PREPARATION_PROCESS_CONTAINMENT_UNAVAILABLE"
	case PrivateGroupingCapturePreparationLiveLSPDisabled:
		return "PREPARATION_LIVE_LSP_DISABLED"
	case PrivateGroupingCapturePreparationSessionNotFound:
		return "PREPARATION_SESSION_NOT_FOUND"
	case PrivateGroupingCapturePreparationListCursorInvalid:
		return "PREPARATION_LIST_CURSOR_INVALID"
	case PrivateGroupingCapturePreparationStaleGeneration:
		return "PREPARATION_STALE_GENERATION"
	case PrivateGroupingCapturePreparationLifecycleConflict:
		return "PREPARATION_LIFECYCLE_CONFLICT"
	case PrivateGroupingCapturePreparationSessionReapIncomplete:
		return "PREPARATION_SESSION_REAP_INCOMPLETE"
	case PrivateGroupingCapturePreparationSessionStopping:
		return "PREPARATION_SESSION_STOPPING"
	case PrivateGroupingCapturePreparationSessionPoisoned:
		return "PREPARATION_SESSION_POISONED"
	case PrivateGroupingCapturePreparationResourceExhausted:
		return "PREPARATION_RESOURCE_EXHAUSTED"
	case PrivateGroupingCapturePreparationSpawnFailure:
		return "PREPARATION_SPAWN_FAILED"
	case PrivateGroupingCapturePreparationPipeSetupFailure:
		return "PREPARATION_PIPE_SETUP_FAILED"
	case PrivateGroupingCapturePreparationInitializationFailure:
		return "PREPARATION_INITIALIZATION_FAILED"
	case PrivateGroupingCapturePreparationInitializationTimeout:
		return "PREPARATION_INITIALIZATION_TIMEOUT"
	case PrivateGroupingCapturePreparationSessionCrashed:
		return "PREPARATION_SESSION_CRASHED"
	case PrivateGroupingCapturePreparationRequestCancelled:
		return "PREPARATION_REQUEST_CANCELLED"
	case PrivateGroupingCapturePreparationRequestTimeout:
		return "PREPARATION_REQUEST_TIMEOUT"
	case PrivateGroupingCapturePreparationDocumentSupplyUnavailable:
		return "PREPARATION_DOCUMENT_SUPPLY_UNAVAILABLE"
	case PrivateGroupingCapturePreparationLanguageIDUnavailable:
		return "PREPARATION_LANGUAGE_ID_UNAVAILABLE"
	case PrivateGroupingCapturePreparationDocumentURIUnavailable:
		return "PREPARATION_DOCUMENT_URI_UNAVAILABLE"
	case PrivateGroupingCapturePreparationDocumentOutsideWorkspace:
		return "PREPARATION_DOCUMENT_OUTSIDE_WORKSPACE"
	case PrivateGroupingCapturePreparationDocumentSourceUnavailable:
		return "PREPARATION_DOCUMENT_SOURCE_UNAVAILABLE"
	case PrivateGroupingCapturePreparationWindowUnjoined:
		return "PREPARATION_WINDOW_UNJOINED"
	default:
		return "UNKNOWN"
	}
}

// Only the already-read context result and typed transaction fields are used.
// Opaque errors and ServerError contents are never inspected or formatted.
func privateGroupingCaptureTransactionResult(symbol bool, ctxErr error, failure session.Failure, serverError, missingPair, invalidPair bool) PrivateGroupingCaptureResult {
	base := PrivateGroupingCaptureRelationContextCanceled
	if symbol {
		base = PrivateGroupingCaptureSymbolContextCanceled
	}
	if ctxErr != nil {
		if ctxErr == context.Canceled {
			return base
		}
		if ctxErr == context.DeadlineExceeded {
			return base + 1
		}
		return PrivateGroupingCaptureUnknown
	}
	if failure != "" {
		switch failure {
		case session.ToolNotImplemented:
			return base + 2
		case session.ProcessContainmentUnavailable:
			return base + 3
		case session.LiveLSPDisabled:
			return base + 4
		case session.SessionNotFound:
			return base + 5
		case session.ListCursorInvalid:
			return base + 6
		case session.StaleGeneration:
			return base + 7
		case session.LifecycleConflict:
			return base + 8
		case session.SessionReapIncomplete:
			return base + 9
		case session.SessionStopping:
			return base + 10
		case session.SessionPoisoned:
			return base + 11
		case session.ResourceExhausted:
			return base + 12
		case session.SpawnFailure:
			return base + 13
		case session.PipeSetupFailure:
			return base + 14
		case session.InitializationFailure:
			return base + 15
		case session.InitializationTimeout:
			return base + 16
		case session.SessionCrashed:
			return base + 17
		case session.RequestCancelled:
			return base + 18
		case session.RequestTimeout:
			return base + 19
		default:
			return base + 20
		}
	}
	if serverError {
		return base + 21
	}
	if missingPair {
		return base + 22
	}
	if invalidPair {
		return base + 23
	}
	return PrivateGroupingCaptureOK
}

func privateGroupingCapturePreparationResult(failure session.Failure) PrivateGroupingCaptureResult {
	switch failure {
	case "":
		return PrivateGroupingCapturePreparationSupplyInvalid
	case session.ToolNotImplemented:
		return PrivateGroupingCapturePreparationToolNotImplemented
	case session.ProcessContainmentUnavailable:
		return PrivateGroupingCapturePreparationProcessContainmentUnavailable
	case session.LiveLSPDisabled:
		return PrivateGroupingCapturePreparationLiveLSPDisabled
	case session.SessionNotFound:
		return PrivateGroupingCapturePreparationSessionNotFound
	case session.ListCursorInvalid:
		return PrivateGroupingCapturePreparationListCursorInvalid
	case session.StaleGeneration:
		return PrivateGroupingCapturePreparationStaleGeneration
	case session.LifecycleConflict:
		return PrivateGroupingCapturePreparationLifecycleConflict
	case session.SessionReapIncomplete:
		return PrivateGroupingCapturePreparationSessionReapIncomplete
	case session.SessionStopping:
		return PrivateGroupingCapturePreparationSessionStopping
	case session.SessionPoisoned:
		return PrivateGroupingCapturePreparationSessionPoisoned
	case session.ResourceExhausted:
		return PrivateGroupingCapturePreparationResourceExhausted
	case session.SpawnFailure:
		return PrivateGroupingCapturePreparationSpawnFailure
	case session.PipeSetupFailure:
		return PrivateGroupingCapturePreparationPipeSetupFailure
	case session.InitializationFailure:
		return PrivateGroupingCapturePreparationInitializationFailure
	case session.InitializationTimeout:
		return PrivateGroupingCapturePreparationInitializationTimeout
	case session.SessionCrashed:
		return PrivateGroupingCapturePreparationSessionCrashed
	case session.RequestCancelled:
		return PrivateGroupingCapturePreparationRequestCancelled
	case session.RequestTimeout:
		return PrivateGroupingCapturePreparationRequestTimeout
	case DocumentSupplyUnavailable:
		return PrivateGroupingCapturePreparationDocumentSupplyUnavailable
	case LanguageIDUnavailable:
		return PrivateGroupingCapturePreparationLanguageIDUnavailable
	case DocumentURIUnavailable:
		return PrivateGroupingCapturePreparationDocumentURIUnavailable
	case DocumentOutsideWorkspace:
		return PrivateGroupingCapturePreparationDocumentOutsideWorkspace
	case DocumentSourceUnavailable:
		return PrivateGroupingCapturePreparationDocumentSourceUnavailable
	case privateProviderWindowUnjoined:
		return PrivateGroupingCapturePreparationWindowUnjoined
	default:
		return PrivateGroupingCapturePreparationFailure
	}
}

// CapturePrivateGroupingOwned requires an existing exact active lease. It uses
// the ordinary bounded native transaction machinery, without enabling C16/C17
// public flags or accepting a caller-owned result as custody.
func (m *Manager) CapturePrivateGroupingOwned(ctx context.Context, req PrivateGroupingOwnedRequest) (PrivateGroupingOwnedHandoff, error) {
	return m.capturePrivateGroupingOwned(ctx, req, false)
}

// CapturePrivateGroupingQueryOwned performs genuine query/source selection for
// CALLS-only input, without inventing a references or definition request.
func (m *Manager) CapturePrivateGroupingQueryOwned(ctx context.Context, req PrivateGroupingOwnedRequest) (PrivateGroupingOwnedHandoff, error) {
	if req.References || req.Definitions {
		return PrivateGroupingOwnedHandoff{}, ErrPrivateGroupingOwned
	}
	return m.capturePrivateGroupingOwned(ctx, req, true)
}

// CapturePrivateGroupingOwnedObserved has the original arguments and effects.
func (m *Manager) CapturePrivateGroupingOwnedObserved(ctx context.Context, req PrivateGroupingOwnedRequest) (PrivateGroupingOwnedHandoff, error, PrivateGroupingCaptureResult) {
	return m.capturePrivateGroupingOwnedObserved(ctx, req, false)
}

// CapturePrivateGroupingQueryOwnedObserved preserves the legacy query guard.
func (m *Manager) CapturePrivateGroupingQueryOwnedObserved(ctx context.Context, req PrivateGroupingOwnedRequest) (PrivateGroupingOwnedHandoff, error, PrivateGroupingCaptureResult) {
	if req.References || req.Definitions {
		return PrivateGroupingOwnedHandoff{}, ErrPrivateGroupingOwned, PrivateGroupingCaptureQueryGuardRefused
	}
	return m.capturePrivateGroupingOwnedObserved(ctx, req, true)
}

func (m *Manager) capturePrivateGroupingOwned(ctx context.Context, req PrivateGroupingOwnedRequest, queryOnly bool) (PrivateGroupingOwnedHandoff, error) {
	h, err, _ := m.capturePrivateGroupingOwnedObserved(ctx, req, queryOnly)
	return h, err
}

func (m *Manager) capturePrivateGroupingOwnedObserved(ctx context.Context, req PrivateGroupingOwnedRequest, queryOnly bool) (PrivateGroupingOwnedHandoff, error, PrivateGroupingCaptureResult) {
	fail := func(result PrivateGroupingCaptureResult) (PrivateGroupingOwnedHandoff, error, PrivateGroupingCaptureResult) {
		return PrivateGroupingOwnedHandoff{}, ErrPrivateGroupingOwned, result
	}
	if !privateGroupingOwnedRequestFieldsValid(req) {
		return fail(PrivateGroupingCapturePrecheckRefused)
	}
	if m == nil || ctx == nil || req.URI == "" || req.OccurrenceID == "" || req.Revision == "" || strings.TrimSpace(req.Revision) != req.Revision || req.Privacy == "" || (!req.References && !req.Definitions && !queryOnly) || req.Deadline.IsZero() || !time.Now().Before(req.Deadline) || req.Deadline.After(time.Now().Add(15*time.Second)) {
		return fail(PrivateGroupingCapturePrecheckRefused)
	}
	envelope, ok := m.PrivateGroupingResourceEnvelope()
	if !ok {
		return fail(PrivateGroupingCaptureResourceEnvelopeRefused)
	}
	var envelopeErr error
	ctx, envelopeErr = m.PrivateGroupingEnvelopeContext(ctx, envelope, req.Deadline)
	if envelopeErr != nil {
		return fail(PrivateGroupingCaptureEnvelopeContextRefused)
	}
	if f := m.CheckActiveSessionLeasePrivateV2(ctx, req.Session); f != "" {
		return fail(PrivateGroupingCaptureActiveLeaseRefused)
	}
	metadata, f := m.Metadata(req.Session.SessionID, req.Session.Generation)
	if f != "" || !metadata.DocumentSymbolSupport || metadata.ProviderName == "" || metadata.ProviderVersion == "" || (metadata.PositionEncoding != "utf-8" && metadata.PositionEncoding != "utf-16" && metadata.PositionEncoding != "utf-32") {
		return fail(PrivateGroupingCaptureMetadataRefused)
	}
	// Empty language delegates to the existing managed document resolver; it is
	// not a Go/profile restriction and does not infer any semantic relation.
	language, _ := m.SessionLanguageID(req.Session.SessionID, req.Session.Generation)
	for _, field := range []string{metadata.PositionEncoding, metadata.ProviderName, metadata.ProviderVersion, metadata.ServerCommand, language} {
		if field != "" && !PrivateGroupingFieldValid(field) {
			return fail(PrivateGroupingCaptureMetadataRefused)
		}
	}
	d := m.PrepareDocument(ctx, DocumentRequest{SessionID: req.Session.SessionID, Generation: req.Session.Generation, URI: req.URI, LanguageID: language, CaptureSupply: true, PreparedNoFollow: true})
	if d.Failure != "" || d.Supply == nil || d.Version <= 0 || d.Supply.URI != req.URI || d.Supply.DocumentVersion != d.Version || d.Supply.SessionID != req.Session.SessionID || d.Supply.Generation != req.Session.Generation {
		return PrivateGroupingOwnedHandoff{}, fmt.Errorf("%w: document preparation: %s", ErrPrivateGroupingOwned, d.Failure), privateGroupingCapturePreparationResult(d.Failure)
	}
	// Charge actual supplied payload lengths before this adapter makes derived
	// source copies or sends semantic requests. PrepareDocument retains its own
	// existing source-read/supply bound; no physical RAM claim is made here.
	preflight := PrivateGroupingOwnedView{Source: d.Supply.Content, Supply: *d.Supply, Metadata: metadata, Revision: req.Revision, Privacy: req.Privacy, Workspace: req.Session.Workspace}
	if _, ok := PrivateGroupingQueryEncodedBound(preflight); !ok {
		return fail(PrivateGroupingCaptureSourceBoundRefused)
	}
	source := append([]byte(nil), d.Supply.Content...)
	binding := OwnedDocumentBinding{URI: req.URI, Version: d.Version, SHA256: fmt.Sprintf("sha256:%x", sha256.Sum256(source))}
	query := adr0011querytarget.Query{OccurrenceID: req.OccurrenceID, URI: req.URI, Encoding: metadata.PositionEncoding, DocumentVersion: strconv.Itoa(d.Version), SourceDigest: binding.SHA256, SessionID: req.Session.SessionID, Generation: req.Session.Generation, Line: req.Line, Character: req.Character}
	transact := func(method string, params []byte) (OwnedMethodPair, error, PrivateGroupingCaptureResult) {
		wire := RoundTripRequest{SessionID: req.Session.SessionID, Generation: req.Session.Generation, Method: method, Params: params, Deadline: req.Deadline, MaxMessages: 4, MaxBytes: 65536, CaptureOwnedMethodPair: true, ExpectedOwnedDocument: &binding, ADR0011PrivateP2: method != "textDocument/documentSymbol"}
		result := m.RoundTrip(ctx, wire)
		ctxErr := ctx.Err() // the original single read, at the original transaction checkpoint
		if ctxErr != nil || result.Failure != "" || result.ServerError != nil || result.ownedMethodPair == nil {
			return OwnedMethodPair{}, ErrPrivateGroupingOwned, privateGroupingCaptureTransactionResult(method == "textDocument/documentSymbol", ctxErr, result.Failure, result.ServerError != nil, result.ownedMethodPair == nil, false)
		}
		// Read the retained private pair, not the caller-visible Result snapshot.
		pair, ok := result.CompletedOwnedMethodPair()
		if !ok || !validOwnedMethodPair(wire, result, pair.Result) || pair.Source == nil || *pair.Source != binding {
			return OwnedMethodPair{}, ErrPrivateGroupingOwned, privateGroupingCaptureTransactionResult(method == "textDocument/documentSymbol", nil, "", false, false, true)
		}
		return pair, nil, PrivateGroupingCaptureOK
	}
	params, _ := json.Marshal(map[string]any{"textDocument": map[string]string{"uri": req.URI}})
	symbol, err, captureResult := transact("textDocument/documentSymbol", params)
	if err != nil {
		return PrivateGroupingOwnedHandoff{}, fmt.Errorf("%w: documentSymbol transaction", err), captureResult
	}
	target, err := adr0011querytarget.SelectDocumentSymbolCandidateV1(query, symbol.Result)
	if err != nil {
		return PrivateGroupingOwnedHandoff{}, fmt.Errorf("%w: canonical query target", err), PrivateGroupingCaptureTargetSelectionRefused
	}
	view := PrivateGroupingOwnedView{Query: query, Target: target, Source: source, Supply: *d.Supply, Metadata: metadata, Revision: req.Revision, Privacy: req.Privacy, Workspace: req.Session.Workspace, Deadline: req.Deadline, Symbol: symbol}
	if _, ok := PrivateGroupingQueryEncodedBound(view); !ok {
		return fail(PrivateGroupingCaptureTargetBoundRefused)
	}
	view.Supply.Content = append([]byte(nil), d.Supply.Content...)
	view.Supply.Params = append(json.RawMessage(nil), d.Supply.Params...)
	for _, method := range []struct {
		name                string
		selected, available bool
	}{{"textDocument/references", req.References, metadata.ReferencesSupport}, {"textDocument/definition", req.Definitions, metadata.DefinitionSupport}} {
		if !method.selected {
			continue
		}
		if !method.available {
			view.Unavailable = append(view.Unavailable, method.name)
			continue
		}
		fields := map[string]any{"textDocument": map[string]string{"uri": req.URI}, "position": map[string]uint32{"line": req.Line, "character": req.Character}}
		if method.name == "textDocument/references" {
			fields["context"] = map[string]bool{"includeDeclaration": false}
		}
		params, _ = json.Marshal(fields)
		pair, err, captureResult := transact(method.name, params)
		if err != nil {
			return fail(captureResult)
		}
		view.Methods = append(view.Methods, pair)
	}
	if f := m.CheckActiveSessionLeasePrivateV2(ctx, req.Session); f != "" {
		return fail(PrivateGroupingCaptureFinalLeaseRefused)
	}
	if _, ok := PrivateGroupingQueryEncodedBound(view); !ok {
		return fail(PrivateGroupingCaptureFinalBoundRefused)
	}
	token, _ := ctx.Value(sessionLeaseContextKey{}).(*sessionLeaseToken)
	return PrivateGroupingOwnedHandoff{&privateGroupingCapture{manager: m, token: token, request: req, view: view}}, nil, PrivateGroupingCaptureOK
}

// ConsumePrivateGroupingOwned atomically retires the opaque handle before its
// callback. Protocol ownership prevents source replacement during issuance;
// cancellation/lifecycle invalidation still prevents successful return. Failure
// never refunds or revives the handle. No hard CPU interruption is claimed.
func (m *Manager) ConsumePrivateGroupingOwned(ctx context.Context, h PrivateGroupingOwnedHandoff, consume func(PrivateGroupingOwnedView) error) error {
	if m == nil || ctx == nil || h.capture == nil || consume == nil {
		return ErrPrivateGroupingOwned
	}
	c := h.capture
	m.mu.Lock()
	r := m.sessions[c.request.Session.SessionID]
	token, _ := ctx.Value(sessionLeaseContextKey{}).(*sessionLeaseToken)
	// Only TAKE on this exact active lease can lend existing exclusion. The
	// token is opaque Manager custody, not a caller-authored ownership flag.
	borrowed := r != nil && token != nil && token == c.token && token.directionalTaken && r.protocolOwned && m.directionalContextPrivateV2(ctx, r, c.request.Deadline)
	valid := c.manager == m && !c.consumed && c.token != nil && token == c.token && r != nil && r.record.Generation == c.request.Session.Generation && m.validSessionLeaseLocked(ctx, r) && (!r.protocolOwned || borrowed) && !r.lifecycleOwned && ctx.Err() == nil && time.Now().Before(c.request.Deadline)
	if valid {
		_, valid = PrivateGroupingQueryEncodedBound(c.view)
	}
	if valid {
		for _, pair := range append([]OwnedMethodPair{c.view.Symbol}, c.view.Methods...) {
			req := RoundTripRequest{SessionID: pair.SessionID, Generation: pair.Generation, Method: pair.Method, Params: pair.Params, CaptureOwnedMethodPair: true, ExpectedOwnedDocument: pair.Source, MaxMessages: 4, MaxBytes: 65536}
			if !matchesOwnedDocument(req, r.documents) {
				valid = false
				break
			}
		}
	}
	if !valid {
		m.mu.Unlock()
		return ErrPrivateGroupingOwned
	}
	c.consumed = true
	r.protocolOwned = true
	m.mu.Unlock()
	defer func() {
		if !borrowed {
			m.mu.Lock()
			r.protocolOwned = false
			m.mu.Unlock()
		}
	}()
	// The consumer gets independent copies, never the retained backing slices.
	view := c.view
	view.Source = append([]byte(nil), view.Source...)
	view.Supply.Content = append([]byte(nil), view.Supply.Content...)
	view.Supply.Params = append(json.RawMessage(nil), view.Supply.Params...)
	view.Symbol = clonePrivateGroupingPair(view.Symbol)
	view.Unavailable = append([]string(nil), view.Unavailable...)
	view.Methods = make([]OwnedMethodPair, len(c.view.Methods))
	for i, p := range c.view.Methods {
		view.Methods[i] = clonePrivateGroupingPair(p)
	}
	if err := consume(view); err != nil {
		return err
	}
	m.mu.Lock()
	live := ctx.Err() == nil && m.validSessionLeaseLocked(ctx, r) && !m.closed && time.Now().Before(c.request.Deadline)
	if live {
		c.sourceConsumedSuccessfully = true
	}
	m.mu.Unlock()
	if !live {
		return ErrPrivateGroupingOwned
	}
	return nil
}

// Fixed private logical encoding contract, not a campaign grant or RAM bound.
const PrivateGroupingMaxFieldBytes = 4096
const PrivateGroupingMaxKinds = 3
const PrivateGroupingQueryMaxBytes uint64 = 1500000

func PrivateGroupingFieldValid(s string) bool {
	return len(s) > 0 && len(s) <= PrivateGroupingMaxFieldBytes && utf8.ValidString(s)
}
func privateGroupingOwnedRequestFieldsValid(req PrivateGroupingOwnedRequest) bool {
	for _, s := range []string{req.Session.SessionID, req.Session.Workspace, req.URI, req.OccurrenceID, req.Revision, req.Privacy} {
		if !PrivateGroupingFieldValid(s) {
			return false
		}
	}
	return req.Session.Generation != 0
}

// PrivateGroupingCheckedEncodedSize is the arithmetic primitive for this one
// private encoding contract. Reject before multiplying or adding; overflow and
// exceeding the fixed ceiling have the same fail-closed outcome.
func PrivateGroupingCheckedEncodedSize(total, n, multiplier, extra uint64) (uint64, bool) {
	if total > PrivateGroupingQueryMaxBytes || extra > PrivateGroupingQueryMaxBytes-total {
		return 0, false
	}
	remaining := PrivateGroupingQueryMaxBytes - total - extra
	if multiplier != 0 && n > remaining/multiplier {
		return 0, false
	}
	return total + n*multiplier + extra, true
}
func privateGroupingEncodedString(total uint64, s string) (uint64, bool) {
	if s != "" && !PrivateGroupingFieldValid(s) {
		return 0, false
	}
	return PrivateGroupingCheckedEncodedSize(total, uint64(len(s)), 6, 2)
}
func privateGroupingEncodedBytes(total uint64, n int) (uint64, bool) {
	if n < 0 {
		return 0, false
	}
	groups := uint64(n) / 3
	if uint64(n)%3 != 0 {
		groups++
	}
	return PrivateGroupingCheckedEncodedSize(total, groups, 4, 4)
}

// PrivateGroupingQueryEncodedBound covers ONLY PrivateGroupingOwnedView's closed
// query encoding. 16 KiB covers fixed field names, separators, all scalar
// numbers/ranges/booleans and the deadline; all variable strings and byte fields
// are enumerated below. No payload copies, maps or marshal calls occur here.
func PrivateGroupingQueryEncodedBound(v PrivateGroupingOwnedView) (uint64, bool) {
	if len(v.Methods) > 2 || len(v.Unavailable) > 2 {
		return 0, false
	}
	total := uint64(16 << 10)
	var ok bool
	strings := []string{v.Query.OccurrenceID, v.Query.URI, v.Query.Encoding, v.Query.DocumentVersion, v.Query.SourceDigest, v.Query.SessionID,
		v.Target.Version, v.Target.ID, v.Target.SymbolID, v.Target.QueryOccurrenceID, v.Target.SymbolName, v.Target.QueryURI, v.Target.SessionID, v.Target.Encoding, v.Target.DocumentVersion, v.Target.SourceDigest, v.Target.DocumentSymbolResultSHA256,
		v.Supply.Classification, v.Supply.SessionID, v.Supply.URI, v.Supply.Method,
		v.Metadata.PositionEncoding, v.Metadata.ProviderName, v.Metadata.ProviderVersion, v.Metadata.ServerCommand, v.Revision, v.Privacy, v.Workspace, "CALLER_ASSERTED"}
	for _, s := range strings {
		total, ok = privateGroupingEncodedString(total, s)
		if !ok {
			return 0, false
		}
	}
	for _, s := range v.Unavailable {
		total, ok = privateGroupingEncodedString(total, s)
		if !ok {
			return 0, false
		}
	}
	for _, n := range []int{len(v.Source), len(v.Supply.Content), len(v.Supply.Params)} {
		total, ok = privateGroupingEncodedBytes(total, n)
		if !ok {
			return 0, false
		}
	}
	pairBound := func(p OwnedMethodPair) bool {
		for _, s := range []string{p.Method, p.SessionID, p.Write.SessionID, p.Write.Method, p.Write.FrameSHA256, p.Read.SessionID, p.Read.FrameSHA256} {
			total, ok = privateGroupingEncodedString(total, s)
			if !ok {
				return false
			}
		}
		if p.Source != nil {
			for _, s := range []string{p.Source.URI, p.Source.SHA256} {
				total, ok = privateGroupingEncodedString(total, s)
				if !ok {
					return false
				}
			}
		}
		for _, n := range []int{len(p.Params), len(p.Result)} {
			total, ok = privateGroupingEncodedBytes(total, n)
			if !ok {
				return false
			}
		}
		return true
	}
	if !pairBound(v.Symbol) {
		return 0, false
	}
	for _, p := range v.Methods {
		if !pairBound(p) {
			return 0, false
		}
	}
	return total, true
}

func clonePrivateGroupingPair(p OwnedMethodPair) OwnedMethodPair {
	p.Params = append(json.RawMessage(nil), p.Params...)
	p.Result = append(json.RawMessage(nil), p.Result...)
	if p.Source != nil {
		s := *p.Source
		p.Source = &s
	}
	return p
}
