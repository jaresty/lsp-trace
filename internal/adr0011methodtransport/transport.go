// Package adr0011methodtransport provides a private, non-semantic transport for
// staged ADR 0011 definition and references requests.
package adr0011methodtransport

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"lsp-trace/internal/session"
	"lsp-trace/internal/strictjson"
	"lsp-trace/sessionruntime"
)

const (
	MethodDefinition = "textDocument/definition"
	MethodReferences = "textDocument/references"

	maxDeadline    = time.Minute
	maxMessages    = 64
	maxBytes       = 1 << 20
	maxParamsBytes = 64 << 10 // private provisional raw-parameter ceiling
)

// Runtime is the smallest managed-session surface this transport requires.
type Runtime interface {
	Metadata(id string, generation uint64) (sessionruntime.SessionMetadata, session.Failure)
	RoundTrip(context.Context, sessionruntime.RoundTripRequest) sessionruntime.RoundTripResult
}

// Request contains only caller-declared transport coordinates and bounds.
type Request struct {
	SessionID   string
	Generation  uint64
	Method      string
	Params      json.RawMessage
	Deadline    time.Time
	MaxMessages int
	MaxBytes    int64
	// Private default-off manager-owned query/result pair; not a receipt.
	CaptureOwnedMethodPair bool
	// Optional prepared-document expectation, verified by the managed owner.
	ExpectedOwnedDocument *sessionruntime.OwnedDocumentBinding
}

// Outcome classifies one private transport attempt, not a parsed LSP method result.
type Outcome string

// Preserve the internal test and classification name while exposing the typed API.
type outcome = Outcome

const (
	OutcomePreflightFailure      Outcome = "PREFLIGHT_FAILURE"
	OutcomeTransportFailure      Outcome = "TRANSPORT_FAILURE"
	OutcomeServerError           Outcome = "SERVER_ERROR"
	OutcomeTimeout               Outcome = "TIMEOUT"
	OutcomeCancelled             Outcome = "CANCELLED"
	OutcomeTransportSuccess      Outcome = "TRANSPORT_SUCCESS"
	OutcomeUnsupportedCapability Outcome = "UNSUPPORTED_CAPABILITY"
)

// Internal names retain the existing Execute/test classification sites.
const (
	preflightFailure      = OutcomePreflightFailure
	transportFailure      = OutcomeTransportFailure
	serverError           = OutcomeServerError
	timedOut              = OutcomeTimeout
	canceled              = OutcomeCancelled
	transportSuccess      = OutcomeTransportSuccess
	unsupportedCapability = OutcomeUnsupportedCapability
)

// Result is intentionally private to this package; success asserts transport only.
type Result struct {
	status      Outcome
	method      string
	raw         json.RawMessage
	failure     string
	server      string
	serverCode  int
	messages    int
	bytes       int64
	observation *TransactionObservation
	ownedPair   *sessionruntime.OwnedMethodPair
}

// Observation returns an unverified in-memory value, never a method receipt.
// A false presence value means the request did not pass private preflight.
func (r Result) Observation() (TransactionObservation, bool) {
	if r.observation == nil {
		return TransactionObservation{}, false
	}
	return *r.observation, true
}

func (r Result) Outcome() Outcome { return r.status }
func (r Result) Method() string   { return r.method }
func (r Result) Raw() json.RawMessage {
	if r.raw == nil {
		return nil
	}
	return append(json.RawMessage{}, r.raw...)
}
func (r Result) FailureText() string     { return r.failure }
func (r Result) ServerErrorText() string { return r.server }

// ServerErrorCode distinguishes an actual JSON-RPC error code of zero from
// the absence of a server error; it does not classify the request outcome.
func (r Result) ServerErrorCode() (int, bool) {
	if r.status != OutcomeServerError {
		return 0, false
	}
	return r.serverCode, true
}
func (r Result) Messages() int { return r.messages }
func (r Result) Bytes() int64  { return r.bytes }

// Transport privately gates and executes exactly one caller-bounded transaction.
type Transport struct{ runtime Runtime }

func New(runtime Runtime) *Transport { return &Transport{runtime: runtime} }

func (t *Transport) Execute(ctx context.Context, req Request) Result {
	// Reject caller-owned raw bytes before making the private copy or parsing.
	if len(req.Params) > maxParamsBytes {
		return Result{status: preflightFailure, method: req.Method, failure: "params exceed private byte limit"}
	}
	// Bind validation, the local digest, and the forwarded wire params to one
	// private copy rather than retaining a caller-owned mutable slice.
	req.Params = append(json.RawMessage(nil), req.Params...)
	if req.ExpectedOwnedDocument != nil {
		expected := *req.ExpectedOwnedDocument
		req.ExpectedOwnedDocument = &expected
	}
	if err := validate(ctx, t, req); err != nil {
		return Result{status: preflightFailure, method: req.Method, failure: err.Error()}
	}
	observation, err := newTransactionObservation(req)
	if err != nil {
		return Result{status: preflightFailure, method: req.Method, failure: "validated params could not be observed"}
	}
	out := Result{method: req.Method, observation: &observation}
	metadata, failure := t.runtime.Metadata(req.SessionID, req.Generation)
	if failure != "" {
		out.status, out.failure = preflightFailure, string(failure)
		return out
	}
	out.observation.MetadataObserved = true
	out.observation.ReportedPositionEncoding = metadata.PositionEncoding
	out.observation.ReportedProviderName = metadata.ProviderName
	out.observation.ReportedProviderVersion = metadata.ProviderVersion
	out.observation.ReportedMethodAdvertised = (req.Method == MethodDefinition && metadata.DefinitionSupport) ||
		(req.Method == MethodReferences && metadata.ReferencesSupport)
	if !out.observation.ReportedMethodAdvertised {
		out.status, out.failure = unsupportedCapability, "method capability unavailable"
		return out
	}

	wire := sessionruntime.RoundTripRequest{
		SessionID: req.SessionID, Generation: req.Generation, Method: req.Method,
		Params: append(json.RawMessage(nil), req.Params...), Deadline: req.Deadline,
		MaxMessages: req.MaxMessages, MaxBytes: req.MaxBytes,
		CaptureOwnedMethodPair: req.CaptureOwnedMethodPair,
		ExpectedOwnedDocument:  req.ExpectedOwnedDocument,
	}
	out.observation.RoundTripCalled = true
	result := t.runtime.RoundTrip(ctx, wire)
	out.observation.LocalWriteCorrespondence = classifyLocalWrite(req, result)
	// A runtime return is not proof of a wire write. Withhold any supplied raw
	// bytes unless the transaction passes every success and bound check below.
	out.observation.RawResultDisposition = RawResultWithheld
	out.observation.ReportedKey = result.Key
	out.observation.ReportedRequestMessages = result.RequestMessages
	out.observation.ReportedRequestBytes = result.RequestBytes
	out.observation.ReportedResponseMessages = result.Messages
	out.observation.ReportedResponseBytes = result.Bytes
	// Preserve observed accounting, including a failing runtime's over-limit
	// report. Never turn such a report into apparent success by clamping it.
	out.messages, out.bytes = result.Messages, result.Bytes
	if err := ctx.Err(); err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			out.status, out.failure = timedOut, string(session.RequestTimeout)
		} else {
			out.status, out.failure = canceled, string(session.RequestCancelled)
		}
		return out
	}
	switch result.Failure {
	case session.RequestTimeout:
		out.status, out.failure = timedOut, string(result.Failure)
		return out
	case session.RequestCancelled:
		out.status, out.failure = canceled, string(result.Failure)
		return out
	case "":
		// Only a failure-free response can carry a successful result.
	default:
		out.status, out.failure = transportFailure, string(result.Failure)
		return out
	}
	if result.Messages < 0 || result.Messages > req.MaxMessages ||
		result.Bytes < 0 || result.Bytes > req.MaxBytes ||
		int64(len(result.Result)) > req.MaxBytes ||
		(result.ServerError != nil && int64(len(result.ServerError.Message)) > req.MaxBytes) {
		out.status, out.failure = transportFailure, "managed transport reported bounds outside request"
		return out
	}
	if result.ServerError != nil {
		out.status = serverError
		out.server = result.ServerError.Message
		out.serverCode = result.ServerError.Code
		return out
	}
	if req.CaptureOwnedMethodPair {
		pair, present := result.CompletedOwnedMethodPair()
		if !present || !ownedPairMatches(req, result, pair, out.observation.LocalWriteCorrespondence) {
			out.status, out.failure = transportFailure, "managed method pair correspondence failed"
			return out
		}
		out.ownedPair = &pair
	}
	out.status = transportSuccess
	if result.Result == nil {
		out.observation.RawResultDisposition = RawResultAbsent
	} else {
		out.raw = append(json.RawMessage{}, result.Result...)
		out.observation.RawResultDisposition = RawResultRetained
		out.observation.RawResultBytes = len(out.raw)
		out.observation.RawResultSHA256 = fmt.Sprintf("sha256:%x", sha256.Sum256(out.raw))
	}
	return out
}

func validate(ctx context.Context, t *Transport, req Request) error {
	if t == nil || t.runtime == nil {
		return errors.New("runtime is required")
	}
	if ctx == nil {
		return errors.New("context is required")
	}
	if strings.TrimSpace(req.SessionID) == "" || strings.TrimSpace(req.SessionID) != req.SessionID {
		return errors.New("canonical session id is required")
	}
	if req.Generation == 0 {
		return errors.New("positive exact generation is required")
	}
	if req.Method != MethodDefinition && req.Method != MethodReferences {
		return errors.New("unsupported method")
	}
	now := time.Now()
	if req.Deadline.IsZero() || !req.Deadline.After(now) || req.Deadline.After(now.Add(maxDeadline)) {
		return errors.New("deadline must be finite, future, and within hard limit")
	}
	if deadline, ok := ctx.Deadline(); ok && req.Deadline.After(deadline) {
		return errors.New("request deadline exceeds context deadline")
	}
	if req.MaxMessages <= 0 || req.MaxMessages > maxMessages {
		return errors.New("message limit outside hard bounds")
	}
	if req.MaxBytes <= 0 || req.MaxBytes > maxBytes {
		return errors.New("byte limit outside hard bounds")
	}
	if err := validateParams(req.Method, req.Params); err != nil {
		return err
	}
	return validateOwnedSourceExpectation(req)
}

// ValidateMethodParams reuses private transport preflight during offline
// candidate replay; success validates syntax and coordinates, not custody.
func ValidateMethodParams(method string, raw json.RawMessage) error {
	if method != MethodDefinition && method != MethodReferences || len(raw) > maxParamsBytes {
		return errors.New("invalid bounded method parameters")
	}
	return validateParams(method, raw)
}

func validateParams(method string, raw json.RawMessage) error {
	if len(raw) == 0 || !json.Valid(raw) {
		return errors.New("params must be valid JSON")
	}
	if err := strictjson.RejectDuplicates(raw); err != nil {
		return errors.New("params contain duplicate JSON member")
	}
	if err := validateCanonicalParamKeys(method, raw); err != nil {
		return err
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if method == MethodDefinition {
		var p struct {
			TextDocument struct {
				URI string `json:"uri"`
			} `json:"textDocument"`
			Position *struct {
				Line      *uint32 `json:"line"`
				Character *uint32 `json:"character"`
			} `json:"position"`
		}
		if err := decodeOne(dec, &p); err != nil {
			return fmt.Errorf("invalid definition params: %w", err)
		}
		if p.TextDocument.URI == "" || p.Position == nil || p.Position.Line == nil || p.Position.Character == nil {
			return errors.New("definition params require textDocument.uri and position.line/character")
		}
		return nil
	}
	var p struct {
		TextDocument struct {
			URI string `json:"uri"`
		} `json:"textDocument"`
		Position *struct {
			Line      *uint32 `json:"line"`
			Character *uint32 `json:"character"`
		} `json:"position"`
		Context *struct {
			IncludeDeclaration *bool `json:"includeDeclaration"`
		} `json:"context"`
	}
	if err := decodeOne(dec, &p); err != nil {
		return fmt.Errorf("invalid references params: %w", err)
	}
	if p.TextDocument.URI == "" || p.Position == nil || p.Position.Line == nil || p.Position.Character == nil || p.Context == nil || p.Context.IncludeDeclaration == nil {
		return errors.New("references params require textDocument.uri, position, and context.includeDeclaration")
	}
	return nil
}

// validateCanonicalParamKeys closes encoding/json's case-insensitive struct-field
// matching without normalizing the caller's wire bytes. Required fields and value
// types remain the responsibility of the existing shape decoder.
func validateCanonicalParamKeys(method string, raw json.RawMessage) error {
	var root map[string]json.RawMessage
	if err := json.Unmarshal(raw, &root); err != nil {
		return errors.New("params must be an object")
	}
	for name, value := range root {
		var allowed []string
		switch name {
		case "textDocument":
			allowed = []string{"uri"}
		case "position":
			allowed = []string{"line", "character"}
		case "context":
			if method != MethodReferences {
				return errors.New("params contain non-canonical member")
			}
			allowed = []string{"includeDeclaration"}
		default:
			return errors.New("params contain non-canonical member")
		}
		var child map[string]json.RawMessage
		if err := json.Unmarshal(value, &child); err != nil {
			return errors.New("params contain non-object member")
		}
		if !onlyCanonicalKeys(child, allowed) {
			return errors.New("params contain non-canonical member")
		}
	}
	return nil
}

func onlyCanonicalKeys(object map[string]json.RawMessage, allowed []string) bool {
	for key := range object {
		found := false
		for _, name := range allowed {
			if key == name {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func decodeOne(dec *json.Decoder, dst any) error {
	if err := dec.Decode(dst); err != nil {
		return err
	}
	var extra any
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		return errors.New("params must contain one JSON value")
	}
	return nil
}
