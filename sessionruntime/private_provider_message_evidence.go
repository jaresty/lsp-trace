package sessionruntime

import (
	"encoding/json"
	"math"

	"lsp-trace/internal/lspwire"
)

// This is private observation, never provider/native admission or packet authority.
// Only the RoundTrip goroutine mutates it, including after joining its reader.
// Presence means an admission/refusal/read was observed, not all physical output.
// With tracing disabled the value lives only in RoundTripResult; persistence
// requires the existing opt-in private trace transport. No new sink is created.
//
// Schema v1 closed numeric vocabulary (zero means absent/not observed):
// class: 1 progress, 2 diagnostics, 3 log/show/telemetry, 4 other notification,
//
//	5 server request, 6 response.
//
// disposition: 0 decoded, 1 dispatched, 2 selected, 3 refused,
//
//	4 JOINED_NOT_DISPATCHED.
//
// boundary: 0 none, 1 FRAME_ADMISSION_REFUSED, 2 READ_FAILED, 3 REQUEST_REFUSED,
//
//	4 WORK_REFUSED, 5 BYTE_REFUSED, 6 CLASS_REFUSED, 7 PROTOCOL_REFUSED,
//	8 RUNTIME_REFUSED, 9 CANCELLED_JOIN.
//
// join: 0 no join observed, 1 JOINED_NOT_DISPATCHED (decoded),
//
//	2 JOINED_UNCLASSIFIED (read failed; bytes unknown/incomplete).
const (
	privateMessageFrameAdmissionRefused uint8 = 1 + iota
	privateMessageReadFailed
	privateMessageRequestRefused
	privateMessageWorkRefused
	privateMessageByteRefused
	privateMessageClassRefused
	privateMessageProtocolRefused
	privateMessageRuntimeRefused
	privateMessageCancelledJoin
)

type privateProviderMessageEvent struct {
	Ordinal     uint8 `json:"n"`
	Class       uint8 `json:"c"`
	Disposition uint8 `json:"d"`
}

type privateProviderMessageEvidence struct {
	enabled    bool
	Present    bool                           `json:"present"`
	Admissions uint64                         `json:"admissions"`
	Decoded    [6]uint64                      `json:"decoded"`
	FrameBytes uint64                         `json:"frame_bytes"`
	Dispatched uint64                         `json:"dispatched"`
	Selected   bool                           `json:"selected"`
	Boundary   uint8                          `json:"boundary"`
	Join       uint8                          `json:"join"`
	Incomplete bool                           `json:"bytes_incomplete"`
	Overflow   bool                           `json:"overflow"`
	Truncated  bool                           `json:"truncated"`
	EventCount uint8                          `json:"event_count"`
	Events     [8]privateProviderMessageEvent `json:"events"`
}

func (e *privateProviderMessageEvidence) add(dst *uint64, n uint64) {
	if n > math.MaxUint64-*dst {
		*dst = math.MaxUint64
		e.Overflow = true
		return
	}
	*dst += n
}

func (e *privateProviderMessageEvidence) admitted() {
	if !e.enabled {
		return
	}
	e.Present = true
	e.add(&e.Admissions, 1)
}

func privateProviderMessageClass(message lspwire.Message) uint8 {
	switch message.Kind() {
	case lspwire.KindNotification:
		switch message.Method {
		case "$/progress":
			return 1
		case "textDocument/publishDiagnostics":
			return 2
		case "window/logMessage", "window/showMessage", "telemetry/event":
			return 3
		default:
			return 4
		}
	case lspwire.KindRequest:
		return 5
	case lspwire.KindSuccessResponse, lspwire.KindErrorResponse:
		return 6
	default:
		return 0
	}
}

func (e *privateProviderMessageEvidence) decoded(message lspwire.Message, frameBytes int64, joined bool) {
	if !e.enabled {
		return
	}
	e.Present = true
	class := privateProviderMessageClass(message)
	if class != 0 {
		e.add(&e.Decoded[class-1], 1)
	}
	if frameBytes > 0 {
		e.add(&e.FrameBytes, uint64(frameBytes))
	} else {
		// A decoded message without an original frame observation is not a
		// complete zero-byte frame. Never substitute re-marshaled JSON length.
		e.Incomplete = true
	}
	disposition := uint8(0)
	if joined {
		e.Join = 1
		disposition = 4
	}
	if e.EventCount < uint8(len(e.Events)) {
		e.Events[e.EventCount] = privateProviderMessageEvent{Ordinal: e.EventCount + 1, Class: class, Disposition: disposition}
		e.EventCount++
	} else {
		e.Truncated = true
	}
}

func (e *privateProviderMessageEvidence) stop(boundary uint8) {
	if !e.enabled {
		return
	}
	e.Present = true
	e.Boundary = boundary
	if boundary == privateMessageReadFailed {
		e.Incomplete = true
	}
	// Do not rewrite the previous event on a pre-read admission or read failure.
	if boundary >= privateMessageRequestRefused && boundary <= privateMessageRuntimeRefused && !e.Truncated && e.EventCount > 0 {
		e.Events[e.EventCount-1].Disposition = 3
	}
}

func (e *privateProviderMessageEvidence) dispatched(selected bool) {
	if !e.enabled {
		return
	}
	e.add(&e.Dispatched, 1)
	if selected {
		e.Selected = true
	}
	if !e.Truncated && e.EventCount > 0 {
		disposition := uint8(1)
		if selected {
			disposition = 2
		}
		e.Events[e.EventCount-1].Disposition = disposition
	}
}

func (e *privateProviderMessageEvidence) joined(message lspwire.Message, frameBytes int64, readFailed bool) {
	if !e.enabled {
		return
	}
	e.stop(privateMessageCancelledJoin)
	if readFailed {
		e.Join = 2
		e.Incomplete = true
		return
	}
	e.decoded(message, frameBytes, true)
}

// Detached observations are copied through existing accessors. Their string
// enums are mapped to closed integer codes, never serialized verbatim. Unknown
// values map to 0, not opaque error/method/payload text.
type privateProviderCompositionSummary struct {
	Present bool      `json:"present"`
	Counts  [8]uint64 `json:"counts"`
	Outcome uint8     `json:"outcome"`
	Invalid uint8     `json:"invalid"`
}

type privateProviderTransactionSummary struct {
	Present    bool      `json:"present"`
	Completion uint64    `json:"completion"`
	Processed  [3]uint64 `json:"processed"`
	Limits     [2]uint64 `json:"limits"`
	Flags      [5]bool   `json:"flags"`
	Policy     uint8     `json:"policy"`
	Lifecycle  uint8     `json:"lifecycle"`
}

type privateProviderMessageSummary struct {
	Version     uint8                             `json:"v"`
	Evidence    privateProviderMessageEvidence    `json:"e"`
	Composition privateProviderCompositionSummary `json:"c"`
	Transaction privateProviderTransactionSummary `json:"t"`
}

func privateProviderMessageSummaryValue(r RoundTripResult) privateProviderMessageSummary {
	s := privateProviderMessageSummary{Version: 1, Evidence: r.privateProviderMessages}
	if c, ok := r.ProviderMessageComposition(); ok {
		s.Composition.Present = true
		s.Composition.Counts = [8]uint64{c.ProgressNotifications, c.LogShowTelemetryNotifications, c.PublishDiagnosticsNotifications, c.OtherNotifications, c.ServerRequests, c.UnmatchedLateInvalidResponses, c.AcceptedTerminalResponses, c.InvalidProtocolMessages}
		switch c.Outcome {
		case ProviderMessageOutcomeAcceptedTerminalResponse:
			s.Composition.Outcome = 1
		case ProviderMessageOutcomeMessageCeiling:
			s.Composition.Outcome = 2
		case ProviderMessageOutcomeByteLimit:
			s.Composition.Outcome = 3
		case ProviderMessageOutcomeCancelled:
			s.Composition.Outcome = 4
		case ProviderMessageOutcomeTimedOut:
			s.Composition.Outcome = 5
		case ProviderMessageOutcomeReadOrTransportFailure:
			s.Composition.Outcome = 6
		case ProviderMessageOutcomeInvalidProtocolMessage:
			s.Composition.Outcome = 7
		case ProviderMessageOutcomeOtherRuntimeFailure:
			s.Composition.Outcome = 8
		}
		switch c.InvalidProtocolCategory {
		case ProviderInvalidProtocolMalformedJSON:
			s.Composition.Invalid = 1
		case ProviderInvalidProtocolWrongVersion:
			s.Composition.Invalid = 2
		case ProviderInvalidProtocolInvalidMessage:
			s.Composition.Invalid = 3
		case ProviderInvalidProtocolInvalidContentLength:
			s.Composition.Invalid = 4
		case ProviderInvalidProtocolDuplicateContentLength:
			s.Composition.Invalid = 5
		case ProviderInvalidProtocolMissingContentLength:
			s.Composition.Invalid = 6
		case ProviderInvalidProtocolEmptyHeaderBlock:
			s.Composition.Invalid = 7
		case ProviderInvalidProtocolNonemptyHeaderMissingLength:
			s.Composition.Invalid = 8
		}
	}
	if t, ok := r.ProviderTransactionObservation(); ok {
		s.Transaction.Present = true
		s.Transaction.Completion = t.CompletionSequence
		s.Transaction.Processed = [3]uint64{t.ProviderMessagesProcessed, t.NotificationMessagesProcessed, t.ControlMessagesProcessed}
		s.Transaction.Limits = [2]uint64{t.ConfiguredControlCeiling, t.ConfiguredMaxBytes}
		s.Transaction.Flags = [5]bool{t.SelectedResponseObserved, t.RequestWriteCompleted, t.SelectedResponseReadCompleted, t.FailurePostEffect, t.TerminalPoisonObserved}
		switch t.NotificationCeilingPolicy {
		case ProviderNotificationCeilingConfiguredCount:
			s.Transaction.Policy = 1
		case ProviderNotificationCeilingByteTimeBounded:
			s.Transaction.Policy = 2
		}
		switch t.TerminalLifecycle {
		case ProviderTransactionLifecycleReady:
			s.Transaction.Lifecycle = 1
		case ProviderTransactionLifecyclePoisoned:
			s.Transaction.Lifecycle = 2
		}
	}
	return s
}

func privateProviderMessageSummaryJSON(r RoundTripResult) string {
	// Fixed struct: no maps, interfaces, raw JSON, custom marshalers or opaque
	// errors. This can only format bounded scalars and eight fixed records.
	b, err := json.Marshal(privateProviderMessageSummaryValue(r))
	if err != nil || len(b) > 2048 {
		return `{"v":1,"state":"UNAVAILABLE"}`
	}
	return string(b)
}
