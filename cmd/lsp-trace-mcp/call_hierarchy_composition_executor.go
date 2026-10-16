package main

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"lsp-trace/internal/lsp"
	"lsp-trace/internal/mcpcontract"
	"lsp-trace/internal/operation"
	"lsp-trace/internal/session"
	"lsp-trace/sessionruntime"
)

const (
	callCompositionDefaultTimeout  = 30 * time.Second
	callCompositionDefaultMessages = 64
	callCompositionDefaultBytes    = int64(4 << 20)
)

type callHierarchyCompositionRuntime interface {
	ResolveSessionSelector(string, uint64) (string, uint64, session.Failure)
	ResolveSessionStatePrivate(string) (session.State, bool)
	Poison(string, uint64) session.LifecycleResult
	ExecuteCallHierarchyDirectionsPrivate(context.Context, sessionruntime.DirectionalCallHierarchyRequestPrivate) (sessionruntime.DirectionalCallHierarchyResultPrivate, error)
}
type callHierarchyCompositionExecutor struct {
	runtime callHierarchyCompositionRuntime
}
type callCompositionInput struct {
	ExperimentalVersion string  `json:"experimental_version"`
	SessionID           string  `json:"session_id"`
	URI                 string  `json:"uri"`
	LanguageID          string  `json:"language_id"`
	Symbol              string  `json:"symbol,omitempty"`
	Line                *uint32 `json:"line,omitempty"`
	Character           *uint32 `json:"character,omitempty"`
	TimeoutMS           int     `json:"timeout_ms,omitempty"`
	MaxMessages         int     `json:"max_messages,omitempty"`
	MaxBytes            int64   `json:"max_bytes,omitempty"`
}
type callCompositionObservation struct {
	Method    string `json:"method"`
	Ordinal   uint64 `json:"ordinal"`
	Length    uint64 `json:"length"`
	SHA256    string `json:"sha256"`
	Component string `json:"component"`
	Selected  bool   `json:"selected"`
	RequestID uint64 `json:"request_id"`
	Failure   string `json:"failure"`
}
type callCompositionReceipt struct {
	State          string                       `json:"state"`
	Method         string                       `json:"method,omitempty"`
	RequestID      uint64                       `json:"request_id,omitempty"`
	RequestLength  uint64                       `json:"request_length,omitempty"`
	ResponseLength uint64                       `json:"response_length,omitempty"`
	ResultLength   uint64                       `json:"result_length,omitempty"`
	RequestSHA256  string                       `json:"request_sha256,omitempty"`
	ResponseSHA256 string                       `json:"response_sha256,omitempty"`
	ResultSHA256   string                       `json:"result_sha256,omitempty"`
	Observations   []callCompositionObservation `json:"observations,omitempty"`
}
type callCompositionReceipts struct {
	Incoming callCompositionReceipt `json:"incoming"`
	Outgoing callCompositionReceipt `json:"outgoing"`
}
type callCompositionArtifact struct {
	SchemaVersion       string                          `json:"schema_version"`
	ExperimentalVersion string                          `json:"experimental_version"`
	Status              string                          `json:"status"`
	Phase               string                          `json:"phase"`
	Code                string                          `json:"code"`
	Incoming            []lsp.CallHierarchyIncomingCall `json:"incoming"`
	Outgoing            []lsp.CallHierarchyOutgoingCall `json:"outgoing"`
	IncomingNull        bool                            `json:"incoming_null"`
	OutgoingNull        bool                            `json:"outgoing_null"`
	Completeness        string                          `json:"completeness"`
	FeatureIdentity     string                          `json:"feature_identity"`
	Authority           int                             `json:"authority"`
	SessionPoisoned     bool                            `json:"session_poisoned"`
	RetryScope          string                          `json:"retry_scope"`
	Receipts            callCompositionReceipts         `json:"receipts"`
}

func unavailableCallReceipt() callCompositionReceipt {
	return callCompositionReceipt{State: "UNAVAILABLE"}
}
func newCallArtifact() callCompositionArtifact {
	return callCompositionArtifact{SchemaVersion: "lsp-trace.experimental.call-hierarchy-composition.v1", ExperimentalVersion: "1", Status: "REFUSED", Phase: "INTERNAL", Code: "INTERNAL", Incoming: []lsp.CallHierarchyIncomingCall{}, Outgoing: []lsp.CallHierarchyOutgoingCall{}, Completeness: "UNKNOWN", FeatureIdentity: "UNRESOLVED", Authority: 0, RetryScope: "NONE", Receipts: callCompositionReceipts{unavailableCallReceipt(), unavailableCallReceipt()}}
}
func receipt(r sessionruntime.DirectionalWireReceiptPrivate) callCompositionReceipt {
	obs := r.ObservationsPrivate()
	out := make([]callCompositionObservation, len(obs))
	for i, o := range obs {
		out[i] = callCompositionObservation{o.Method, o.Ordinal, o.Length, hex.EncodeToString(o.SHA256[:]), o.Component, o.Selected, o.RequestID, o.Failure}
	}
	rq, rs, rr := r.RequestSHA256Private(), r.ResponseSHA256Private(), r.ResultSHA256Private()
	if r.MethodPrivate() == "" || r.RequestIDPrivate() == 0 || r.RequestLengthPrivate() == 0 {
		return unavailableCallReceipt()
	}
	return callCompositionReceipt{State: "AUTHENTICATED", Method: r.MethodPrivate(), RequestID: r.RequestIDPrivate(), RequestLength: r.RequestLengthPrivate(), ResponseLength: r.ResponseLengthPrivate(), ResultLength: r.ResultLengthPrivate(), RequestSHA256: hex.EncodeToString(rq[:]), ResponseSHA256: hex.EncodeToString(rs[:]), ResultSHA256: hex.EncodeToString(rr[:]), Observations: out}
}
func cloneIncomingCalls(v []lsp.CallHierarchyIncomingCall) []lsp.CallHierarchyIncomingCall {
	out := make([]lsp.CallHierarchyIncomingCall, len(v))
	for i := range v {
		out[i] = v[i]
		out[i].From.Tags = append([]int(nil), v[i].From.Tags...)
		out[i].From.Data = append([]byte(nil), v[i].From.Data...)
		out[i].FromRanges = append([]lsp.Range(nil), v[i].FromRanges...)
	}
	return out
}
func cloneOutgoingCalls(v []lsp.CallHierarchyOutgoingCall) []lsp.CallHierarchyOutgoingCall {
	out := make([]lsp.CallHierarchyOutgoingCall, len(v))
	for i := range v {
		out[i] = v[i]
		out[i].To.Tags = append([]int(nil), v[i].To.Tags...)
		out[i].To.Data = append([]byte(nil), v[i].To.Data...)
		out[i].FromRanges = append([]lsp.Range(nil), v[i].FromRanges...)
	}
	return out
}
func classifyCallRefusal(f session.Failure, err error, r sessionruntime.DirectionalCallHierarchyResultPrivate) (phase, code string, poisoned bool, retryScope string) {
	retryScope = "NONE"
	if errors.Is(err, context.Canceled) {
		switch r.CancellationPhasePrivate {
		case "AFTER_OUTGOING_REQUEST_EFFECT":
			if r.RequestEffectObservedPrivate && r.TerminalPoisonObservedPrivate {
				return "CANCELLATION", "CANCELLED_AFTER_OUTGOING_REQUEST_EFFECT", true, "NEW_SESSION_REQUIRED"
			}
			return "CANCELLATION", "CANCELLED_AFTER_OUTGOING_REQUEST_EFFECT", false, retryScope
		case "AFTER_INCOMING_BEFORE_OUTGOING_EFFECT":
			if r.RequestEffectObservedPrivate && r.TerminalPoisonObservedPrivate {
				return "CANCELLATION", "CANCELLED_AFTER_INCOMING_BEFORE_OUTGOING_EFFECT", true, "NEW_SESSION_REQUIRED"
			}
			return "CANCELLATION", "CANCELLED_AFTER_INCOMING_BEFORE_OUTGOING_EFFECT", false, retryScope
		default:
			return "CANCELLATION", "CANCELLED_BEFORE_INCOMING_EFFECT", false, retryScope
		}
	}
	code = string(f)
	if code == "" && err != nil {
		switch err.Error() {
		case string(session.ResourceExhausted):
			code = string(session.ResourceExhausted)
		case string(session.LifecycleConflict):
			code = string(session.LifecycleConflict)
		case string(session.SessionPoisoned):
			code = string(session.SessionPoisoned)
		default:
			message := strings.ToLower(err.Error())
			if strings.Contains(message, "too large") || strings.Contains(message, "exceeds") || strings.Contains(message, "resource exhausted") {
				code = string(session.ResourceExhausted)
			} else {
				code = "CALL_HIERARCHY_FAILED"
			}
		}
	}
	if r.FailurePostEffect {
		poisoned = r.RequestEffectObservedPrivate && r.TerminalPoisonObservedPrivate
		if poisoned {
			retryScope = "NEW_SESSION_REQUIRED"
		}
		if code == string(session.ResourceExhausted) {
			return "POST_EFFECT_RESOURCE_REFUSAL", code, poisoned, retryScope
		}
		return "POST_EFFECT_FAILURE", code, poisoned, retryScope
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "RESOURCE_LIMIT", "TIMEOUT", false, retryScope
	}
	switch code {
	case "SESSION_NOT_FOUND", "AMBIGUOUS_SESSION_SELECTOR":
		return "SESSION_RESOLUTION", code, false, "REQUEST_CORRECTION"
	case "SESSION_POISONED", "SESSION_CRASHED":
		return "LIFECYCLE", code, true, "NEW_SESSION_REQUIRED"
	case "SESSION_NOT_READY", "STALE_GENERATION", "LIFECYCLE_CONFLICT":
		return "LIFECYCLE", code, false, "REQUEST_CORRECTION"
	case "RESOURCE_EXHAUSTED", "DIRECTIONAL_RELEASE_FAILED":
		return "RESOURCE_LIMIT", code, false, retryScope
	}
	if r.IncomingWire.RequestLengthPrivate() == 0 {
		return "PREPARATION", code, r.SessionPoisoned, retryScope
	}
	if r.OutgoingWire.RequestLengthPrivate() == 0 {
		return "INCOMING", code, r.SessionPoisoned, retryScope
	}
	return "OUTGOING", code, r.SessionPoisoned, retryScope
}
func (e callHierarchyCompositionExecutor) Execute(ctx context.Context, req operation.Request) (operation.Result, *operation.Failure) {
	var in callCompositionInput
	if err := json.Unmarshal(req.Input, &in); err != nil {
		return operation.Result{}, &operation.Failure{Code: operation.FailureInvalidInput, Err: err}
	}
	if in.TimeoutMS == 0 {
		in.TimeoutMS = int(callCompositionDefaultTimeout / time.Millisecond)
	}
	if in.MaxMessages == 0 {
		in.MaxMessages = callCompositionDefaultMessages
	}
	if in.MaxBytes == 0 {
		in.MaxBytes = callCompositionDefaultBytes
	}
	a := newCallArtifact()
	id, gen, f := e.runtime.ResolveSessionSelector(in.SessionID, 0)
	if f != "" {
		if state, ok := e.runtime.ResolveSessionStatePrivate(in.SessionID); ok && state == session.Poisoned {
			f = session.SessionPoisoned
		}
		a.Phase, a.Code, a.SessionPoisoned, a.RetryScope = classifyCallRefusal(f, errors.New(string(f)), sessionruntime.DirectionalCallHierarchyResultPrivate{Failure: f})
		return marshalCallArtifact(a)
	}
	deadline := time.Now().Add(time.Duration(in.TimeoutMS) * time.Millisecond)
	if d, ok := ctx.Deadline(); ok && d.Before(deadline) {
		deadline = d
	}
	r, err := e.runtime.ExecuteCallHierarchyDirectionsPrivate(ctx, sessionruntime.DirectionalCallHierarchyRequestPrivate{SessionID: id, Generation: gen, URI: in.URI, LanguageID: in.LanguageID, Symbol: in.Symbol, Line: in.Line, Character: in.Character, Deadline: deadline, MaxMessages: in.MaxMessages, MaxBytes: in.MaxBytes})
	if errors.Is(err, context.Canceled) && r.CancellationPhasePrivate != "BEFORE_INCOMING_EFFECT" && r.RequestEffectObservedPrivate && !r.TerminalPoisonObservedPrivate {
		terminal := e.runtime.Poison(id, gen)
		r.SessionPoisoned = terminal.State == session.Poisoned
		r.TerminalPoisonObservedPrivate = terminal.State == session.Poisoned
		r.TerminalLifecyclePrivate = terminal.State
	}
	a.Receipts = callCompositionReceipts{receipt(r.IncomingWire), receipt(r.OutgoingWire)}
	a.IncomingNull = r.IncomingNull
	a.OutgoingNull = r.OutgoingNull
	if err != nil || r.Failure != "" {
		a.Phase, a.Code, a.SessionPoisoned, a.RetryScope = classifyCallRefusal(r.Failure, err, r)
		return marshalCallArtifact(a)
	}
	a.Status = "SUCCEEDED"
	a.Phase = "COMPLETE"
	a.Code = "NONE"
	a.SessionPoisoned = false
	a.RetryScope = "NONE"
	a.Incoming = cloneIncomingCalls(r.Incoming)
	a.Outgoing = cloneOutgoingCalls(r.Outgoing)
	return marshalCallArtifact(a)
}
func marshalCallArtifact(a callCompositionArtifact) (operation.Result, *operation.Failure) {
	b, err := json.Marshal(a)
	if err != nil {
		return operation.Result{}, &operation.Failure{Code: operation.FailureInternal, Err: err}
	}
	if err = mcpcontract.ValidateJSON(mcpcontract.CallHierarchyCompositionResultID, b); err != nil {
		return operation.Result{}, &operation.Failure{Code: operation.FailureInternal, Err: err}
	}
	return operation.Result{Artifact: b, ArtifactSchemaID: mcpcontract.CallHierarchyCompositionResultID}, nil
}
