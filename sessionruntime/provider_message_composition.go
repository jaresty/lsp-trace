package sessionruntime

import (
	"errors"
	"math"

	"lsp-trace/internal/lspwire"
	"lsp-trace/internal/session"
)

// ProviderMessageOutcome is a closed, privacy-reduced terminal classification.
type ProviderMessageOutcome string

const (
	ProviderMessageOutcomeAcceptedTerminalResponse ProviderMessageOutcome = "ACCEPTED_TERMINAL_RESPONSE"
	ProviderMessageOutcomeMessageCeiling           ProviderMessageOutcome = "MESSAGE_CEILING"
	ProviderMessageOutcomeByteLimit                ProviderMessageOutcome = "BYTE_LIMIT"
	ProviderMessageOutcomeCancelled                ProviderMessageOutcome = "CANCELLED"
	ProviderMessageOutcomeTimedOut                 ProviderMessageOutcome = "TIMED_OUT"
	ProviderMessageOutcomeReadOrTransportFailure   ProviderMessageOutcome = "READ_OR_TRANSPORT_FAILURE"
	ProviderMessageOutcomeInvalidProtocolMessage   ProviderMessageOutcome = "INVALID_PROTOCOL_MESSAGE"
	ProviderMessageOutcomeOtherRuntimeFailure      ProviderMessageOutcome = "OTHER_RUNTIME_FAILURE"
)

// ProviderMessageComposition contains only bounded counters and a closed outcome.
type ProviderMessageComposition struct {
	ProgressNotifications           uint64                 `json:"-"`
	LogShowTelemetryNotifications   uint64                 `json:"-"`
	PublishDiagnosticsNotifications uint64                 `json:"-"`
	OtherNotifications              uint64                 `json:"-"`
	ServerRequests                  uint64                 `json:"-"`
	UnmatchedLateInvalidResponses   uint64                 `json:"-"`
	AcceptedTerminalResponses       uint64                 `json:"-"`
	InvalidProtocolMessages         uint64                 `json:"-"`
	Outcome                         ProviderMessageOutcome `json:"-"`
}

type providerMessageCompositionAccumulator struct {
	ProviderMessageComposition
	enabled, overflow, byteLimit, messageCeiling, invalidProtocol bool
}

func (a *providerMessageCompositionAccumulator) increment(value *uint64) {
	if !a.enabled || a.overflow {
		return
	}
	if *value == math.MaxUint64 {
		a.overflow = true
		return
	}
	*value++
}

func (a *providerMessageCompositionAccumulator) invalid(err error) {
	if !a.enabled || !isProviderInvalidProtocol(err) {
		return
	}
	a.invalidProtocol = true
	a.increment(&a.InvalidProtocolMessages)
}

func isProviderInvalidProtocol(err error) bool {
	return errors.Is(err, lspwire.ErrMalformedJSON) || errors.Is(err, lspwire.ErrWrongVersion) ||
		errors.Is(err, lspwire.ErrInvalidMessage) || errors.Is(err, lspwire.ErrInvalidContentLength) ||
		errors.Is(err, lspwire.ErrDuplicateContentLength) || errors.Is(err, lspwire.ErrMissingContentLength)
}

func (a *providerMessageCompositionAccumulator) dispatchDisposition(message lspwire.Message, disposition lspwire.ResponseDisposition) {
	a.dispatch(message, disposition == lspwire.ResponseAccepted)
}

func (a *providerMessageCompositionAccumulator) dispatch(message lspwire.Message, accepted bool) {
	if !a.enabled {
		return
	}
	switch message.Kind() {
	case lspwire.KindNotification:
		switch message.Method {
		case "$/progress":
			a.increment(&a.ProgressNotifications)
		case "window/logMessage", "window/showMessage", "telemetry/event":
			a.increment(&a.LogShowTelemetryNotifications)
		case "textDocument/publishDiagnostics":
			a.increment(&a.PublishDiagnosticsNotifications)
		default:
			a.increment(&a.OtherNotifications)
		}
	case lspwire.KindRequest:
		a.increment(&a.ServerRequests)
	default:
		if accepted {
			a.increment(&a.AcceptedTerminalResponses)
		} else {
			a.increment(&a.UnmatchedLateInvalidResponses)
		}
	}
}

func (a providerMessageCompositionAccumulator) finish(failure session.Failure) (ProviderMessageComposition, bool) {
	if !a.enabled {
		return ProviderMessageComposition{}, false
	}
	value := a.ProviderMessageComposition
	switch {
	case a.overflow:
		value.Outcome = ProviderMessageOutcomeOtherRuntimeFailure
	case value.AcceptedTerminalResponses == 1 && failure == "":
		value.Outcome = ProviderMessageOutcomeAcceptedTerminalResponse
	case a.invalidProtocol:
		value.Outcome = ProviderMessageOutcomeInvalidProtocolMessage
	case a.byteLimit:
		value.Outcome = ProviderMessageOutcomeByteLimit
	case a.messageCeiling:
		value.Outcome = ProviderMessageOutcomeMessageCeiling
	case failure == session.RequestCancelled:
		value.Outcome = ProviderMessageOutcomeCancelled
	case failure == session.RequestTimeout:
		value.Outcome = ProviderMessageOutcomeTimedOut
	case failure == session.SessionCrashed || failure == session.Failure("transport-unavailable"):
		value.Outcome = ProviderMessageOutcomeReadOrTransportFailure
	default:
		value.Outcome = ProviderMessageOutcomeOtherRuntimeFailure
	}
	return value, true
}
