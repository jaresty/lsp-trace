package adr0011methodtransport_test

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"

	transport "lsp-trace/internal/adr0011methodtransport"
	"lsp-trace/internal/lspwire"
	"lsp-trace/internal/session"
	"lsp-trace/sessionruntime"
)

type outcomeRuntime struct {
	metadata      sessionruntime.SessionMetadata
	result        sessionruntime.RoundTripResult
	params        json.RawMessage
	calls         int
	metadataCalls int
}

func (r *outcomeRuntime) Metadata(string, uint64) (sessionruntime.SessionMetadata, session.Failure) {
	r.metadataCalls++
	return r.metadata, ""
}
func (r *outcomeRuntime) RoundTrip(_ context.Context, req sessionruntime.RoundTripRequest) sessionruntime.RoundTripResult {
	r.calls++
	r.params = append(json.RawMessage(nil), req.Params...)
	return r.result
}

func TestMethodParameterFieldAliasesCannotChangeQuery(t *testing.T) {
	for _, tc := range []struct {
		name, method, params string
	}{
		{"root alias", transport.MethodDefinition, `{"TextDocument":{"uri":"file:///w/a.go"},"position":{"line":0,"character":0}}`},
		{"URI alias", transport.MethodDefinition, `{"textDocument":{"URI":"file:///w/a.go"},"position":{"line":0,"character":0}}`},
		{"URI competing keys", transport.MethodDefinition, `{"textDocument":{"uri":"file:///w/a.go","URI":"file:///w/b.go"},"position":{"line":0,"character":0}}`},
		{"line alias", transport.MethodDefinition, `{"textDocument":{"uri":"file:///w/a.go"},"position":{"Line":0,"character":0}}`},
		{"line competing keys", transport.MethodDefinition, `{"textDocument":{"uri":"file:///w/a.go"},"position":{"line":0,"Line":1,"character":0}}`},
		{"character alias", transport.MethodDefinition, `{"textDocument":{"uri":"file:///w/a.go"},"position":{"line":0,"Character":0}}`},
		{"references context alias", transport.MethodReferences, `{"textDocument":{"uri":"file:///w/a.go"},"position":{"line":0,"character":0},"context":{"IncludeDeclaration":true}}`},
		{"references context competing keys", transport.MethodReferences, `{"textDocument":{"uri":"file:///w/a.go"},"position":{"line":0,"character":0},"context":{"includeDeclaration":true,"IncludeDeclaration":false}}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			runtime := &outcomeRuntime{}
			runtime.metadata.DefinitionSupport, runtime.metadata.ReferencesSupport = true, true
			got := transport.New(runtime).Execute(context.Background(), transport.Request{
				SessionID: "exact", Generation: 1, Method: tc.method,
				Params: json.RawMessage(tc.params), Deadline: time.Now().Add(time.Second),
				MaxMessages: 5, MaxBytes: 8192,
			})
			if got.Outcome() != transport.OutcomePreflightFailure || runtime.metadataCalls != 0 || runtime.calls != 0 || !strings.Contains(got.FailureText(), "non-canonical") || len(got.Raw()) != 0 || strings.Contains(got.FailureText(), "file:///") {
				t.Fatalf("ASSERT_PARAM_ALIAS_PRECHECK: outcome=%s metadata=%d calls=%d failure=%q rawLen=%d", got.Outcome(), runtime.metadataCalls, runtime.calls, got.FailureText(), len(got.Raw()))
			}
			t.Log("ASSERT_PARAM_ALIAS_PRECHECK: PASS")
		})
	}
}

func TestCanonicalMethodParameterNamesPreserveExactWireBytes(t *testing.T) {
	for _, tc := range []struct{ method, params string }{
		{transport.MethodDefinition, ` {"textDocument":{"ur\u0069":"file:///w/a.go"} , "position":{"line":0,"character":0}} `},
		{transport.MethodReferences, ` {"textDocument":{"uri":"file:///w/a.go"}, "position":{"line":0,"character":0}, "context":{"includeDeclaration":false}} `},
	} {
		t.Run(tc.method, func(t *testing.T) {
			runtime := &outcomeRuntime{result: sessionruntime.RoundTripResult{Result: json.RawMessage(`[]`)}}
			runtime.metadata.DefinitionSupport, runtime.metadata.ReferencesSupport = true, true
			got := transport.New(runtime).Execute(context.Background(), transport.Request{
				SessionID: "exact", Generation: 1, Method: tc.method,
				Params: json.RawMessage(tc.params), Deadline: time.Now().Add(time.Second),
				MaxMessages: 5, MaxBytes: 8192,
			})
			if got.Outcome() != transport.OutcomeTransportSuccess || runtime.metadataCalls != 1 || runtime.calls != 1 || string(runtime.params) != tc.params || string(got.Raw()) != `[]` {
				t.Fatalf("ASSERT_CANONICAL_PARAMS_EXACT_WIRE: outcome=%s metadata=%d calls=%d params=%q want=%q raw=%q", got.Outcome(), runtime.metadataCalls, runtime.calls, runtime.params, tc.params, got.Raw())
			}
			t.Log("ASSERT_CANONICAL_PARAMS_EXACT_WIRE: PASS")
		})
	}
}

func TestDuplicateMethodParameterKeysArePreflightFailures(t *testing.T) {
	for _, tc := range []struct {
		name, method, params string
	}{
		{"definition top-level", transport.MethodDefinition, `{"textDocument":{"uri":"file:///w/a.go"},"position":{"line":0,"character":0},"position":{"line":1,"character":0}}`},
		{"definition nested URI", transport.MethodDefinition, `{"textDocument":{"uri":"file:///w/a.go","uri":"file:///w/b.go"},"position":{"line":0,"character":0}}`},
		{"definition escaped URI", transport.MethodDefinition, `{"textDocument":{"uri":"file:///w/a.go","ur\u0069":"file:///w/b.go"},"position":{"line":0,"character":0}}`},
		{"definition nested position", transport.MethodDefinition, `{"textDocument":{"uri":"file:///w/a.go"},"position":{"line":0,"line":1,"character":0}}`},
		{"references context", transport.MethodReferences, `{"textDocument":{"uri":"file:///w/a.go"},"position":{"line":0,"character":0},"context":{"includeDeclaration":true,"includeDeclaration":false}}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			runtime := &outcomeRuntime{}
			runtime.metadata.DefinitionSupport, runtime.metadata.ReferencesSupport = true, true
			got := transport.New(runtime).Execute(context.Background(), transport.Request{
				SessionID: "exact", Generation: 1, Method: tc.method,
				Params: json.RawMessage(tc.params), Deadline: time.Now().Add(time.Second),
				MaxMessages: 5, MaxBytes: 8192,
			})
			code, present := got.ServerErrorCode()
			if got.Outcome() != transport.OutcomePreflightFailure || runtime.metadataCalls != 0 || runtime.calls != 0 || !strings.Contains(got.FailureText(), "duplicate") || strings.Contains(got.FailureText(), "file:///") || code != 0 || present || len(got.Raw()) != 0 {
				t.Fatalf("ASSERT_DUPLICATE_PARAM_PRECHECK: outcome=%s metadata=%d calls=%d failure=%q code=%d present=%t rawLen=%d", got.Outcome(), runtime.metadataCalls, runtime.calls, got.FailureText(), code, present, len(got.Raw()))
			}
			t.Log("ASSERT_DUPLICATE_PARAM_PRECHECK: PASS")
		})
	}
}

func TestDuplicateParamDiagnosticWithholdsCallerKey(t *testing.T) {
	const marker = "private_MUST_NOT_ECHO_7b1"
	for _, tc := range []struct{ name, params string }{
		{"plain key", `{"textDocument":{"uri":"file:///w/a.go","private_MUST_NOT_ECHO_7b1":0,"private_MUST_NOT_ECHO_7b1":1},"position":{"line":0,"character":0}}`},
		{"escaped key", `{"textDocument":{"uri":"file:///w/a.go","private_MUST_NOT_ECHO_7b1":0,"priv\u0061te_MUST_NOT_ECHO_7b1":1},"position":{"line":0,"character":0}}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			runtime := &outcomeRuntime{}
			runtime.metadata.DefinitionSupport = true
			got := transport.New(runtime).Execute(context.Background(), transport.Request{
				SessionID: "exact", Generation: 1, Method: transport.MethodDefinition,
				Params: json.RawMessage(tc.params), Deadline: time.Now().Add(time.Second),
				MaxMessages: 5, MaxBytes: 8192,
			})
			if got.Outcome() != transport.OutcomePreflightFailure || runtime.metadataCalls != 0 || runtime.calls != 0 || !strings.Contains(got.FailureText(), "duplicate") || strings.Contains(got.FailureText(), marker) || len(got.Raw()) != 0 {
				t.Fatalf("ASSERT_DUPLICATE_KEY_PRIVACY: outcome=%s metadata=%d calls=%d failure=%q rawLen=%d", got.Outcome(), runtime.metadataCalls, runtime.calls, got.FailureText(), len(got.Raw()))
			}
			t.Log("ASSERT_DUPLICATE_KEY_PRIVACY: PASS")
		})
	}
}

func TestServerErrorBeyondReportedBoundsCannotExposeCode(t *testing.T) {
	for _, tc := range []struct {
		name   string
		result sessionruntime.RoundTripResult
	}{
		{"excess messages", sessionruntime.RoundTripResult{ServerError: &lspwire.RPCError{Code: -32601, Message: "not found"}, Messages: 6, Bytes: 100}},
		{"negative messages", sessionruntime.RoundTripResult{ServerError: &lspwire.RPCError{Code: -32601, Message: "not found"}, Messages: -1, Bytes: 100}},
		{"excess reported bytes", sessionruntime.RoundTripResult{ServerError: &lspwire.RPCError{Code: -32601, Message: "not found"}, Messages: 1, Bytes: 8193}},
		{"negative reported bytes", sessionruntime.RoundTripResult{ServerError: &lspwire.RPCError{Code: -32601, Message: "not found"}, Messages: 1, Bytes: -1}},
		{"excess raw result bytes", sessionruntime.RoundTripResult{ServerError: &lspwire.RPCError{Code: -32601, Message: "not found"}, Result: json.RawMessage(`"` + strings.Repeat("x", 8193) + `"`), Messages: 1, Bytes: 100}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			runtime := &outcomeRuntime{result: tc.result}
			runtime.metadata.DefinitionSupport = true
			got := transport.New(runtime).Execute(context.Background(), transport.Request{
				SessionID: "exact", Generation: 1, Method: transport.MethodDefinition,
				Params:   json.RawMessage(`{"textDocument":{"uri":"file:///w/a.go"},"position":{"line":0,"character":0}}`),
				Deadline: time.Now().Add(time.Second), MaxMessages: 5, MaxBytes: 8192,
			})
			code, present := got.ServerErrorCode()
			if runtime.calls != 1 || got.Outcome() != transport.OutcomeTransportFailure || code != 0 || present || got.ServerErrorText() != "" || len(got.Raw()) != 0 || got.FailureText() == "" || got.Messages() != tc.result.Messages || got.Bytes() != tc.result.Bytes {
				t.Fatalf("ASSERT_SERVER_ERROR_BOUNDS_PRECEDENCE: outcome=%s code=%d present=%t serverText=%q rawLen=%d failure=%q messages=%d bytes=%d calls=%d", got.Outcome(), code, present, got.ServerErrorText(), len(got.Raw()), got.FailureText(), got.Messages(), got.Bytes(), runtime.calls)
			}
			t.Log("ASSERT_SERVER_ERROR_BOUNDS_PRECEDENCE: PASS")
		})
	}
}

func TestServerErrorMessageRespectsSelectedBoundBeforeErrorEvidence(t *testing.T) {
	const maxBytes int64 = 8192
	for _, tc := range []struct {
		name    string
		result  sessionruntime.RoundTripResult
		outcome transport.Outcome
		codeOK  bool
	}{
		{"oversized underreported message", sessionruntime.RoundTripResult{ServerError: &lspwire.RPCError{Code: -32601, Message: strings.Repeat("x", int(maxBytes)+1)}, Messages: 1, Bytes: 100}, transport.OutcomeTransportFailure, false},
		{"bounded code zero at exact message length", sessionruntime.RoundTripResult{ServerError: &lspwire.RPCError{Code: 0, Message: strings.Repeat("x", int(maxBytes))}, Messages: 1, Bytes: maxBytes}, transport.OutcomeServerError, true},
		{"typed timeout precedes oversized message", sessionruntime.RoundTripResult{Failure: session.RequestTimeout, ServerError: &lspwire.RPCError{Code: -32601, Message: strings.Repeat("x", int(maxBytes)+1)}, Messages: 1, Bytes: 100}, transport.OutcomeTimeout, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			runtime := &outcomeRuntime{result: tc.result}
			runtime.metadata.DefinitionSupport = true
			got := transport.New(runtime).Execute(context.Background(), transport.Request{
				SessionID: "exact", Generation: 1, Method: transport.MethodDefinition,
				Params:   json.RawMessage(`{"textDocument":{"uri":"file:///w/a.go"},"position":{"line":0,"character":0}}`),
				Deadline: time.Now().Add(time.Second), MaxMessages: 5, MaxBytes: maxBytes,
			})
			code, present := got.ServerErrorCode()
			wantCode := 0
			wantServerText := ""
			if tc.codeOK {
				wantServerText = tc.result.ServerError.Message
			}
			if got.Outcome() != tc.outcome || runtime.calls != 1 || got.Messages() != tc.result.Messages || got.Bytes() != tc.result.Bytes || code != wantCode || present != tc.codeOK || got.ServerErrorText() != wantServerText || len(got.Raw()) != 0 || (tc.outcome != transport.OutcomeServerError && got.FailureText() == "") {
				t.Fatalf("ASSERT_SERVER_MESSAGE_BOUND_PRECEDENCE: outcome=%s calls=%d messages=%d bytes=%d code=%d present=%t serverTextLen=%d failure=%q rawLen=%d", got.Outcome(), runtime.calls, got.Messages(), got.Bytes(), code, present, len(got.ServerErrorText()), got.FailureText(), len(got.Raw()))
			}
			t.Log("ASSERT_SERVER_MESSAGE_BOUND_PRECEDENCE: PASS")
		})
	}
}

func TestExternalCallerObservesServerErrorCodePresence(t *testing.T) {
	for _, tc := range []struct {
		name     string
		result   sessionruntime.RoundTripResult
		support  bool
		wantCode int
		wantOK   bool
		want     transport.Outcome
	}{
		{"method not found", sessionruntime.RoundTripResult{ServerError: &lspwire.RPCError{Code: -32601, Message: "not found"}, Result: json.RawMessage(`{"untrusted":true}`)}, true, -32601, true, transport.OutcomeServerError},
		{"zero code still present", sessionruntime.RoundTripResult{ServerError: &lspwire.RPCError{Code: 0, Message: "zero"}}, true, 0, true, transport.OutcomeServerError},
		{"zero code at exact limits", sessionruntime.RoundTripResult{ServerError: &lspwire.RPCError{Code: 0, Message: "zero"}, Messages: 5, Bytes: 8192}, true, 0, true, transport.OutcomeServerError},
		{"successful result has no code", sessionruntime.RoundTripResult{Result: json.RawMessage(`[]`)}, true, 0, false, transport.OutcomeTransportSuccess},
		{"unsupported capability has no code", sessionruntime.RoundTripResult{}, false, 0, false, transport.OutcomeUnsupportedCapability},
		{"failed transport has no code", sessionruntime.RoundTripResult{Failure: session.ResourceExhausted, ServerError: &lspwire.RPCError{Code: -32601}}, true, 0, false, transport.OutcomeTransportFailure},
	} {
		t.Run(tc.name, func(t *testing.T) {
			runtime := &outcomeRuntime{result: tc.result}
			runtime.metadata.DefinitionSupport = tc.support
			result := transport.New(runtime).Execute(context.Background(), transport.Request{
				SessionID: "exact", Generation: 1, Method: transport.MethodDefinition,
				Params:   json.RawMessage(`{"textDocument":{"uri":"file:///w/a.go"},"position":{"line":0,"character":0}}`),
				Deadline: time.Now().Add(time.Second), MaxMessages: 5, MaxBytes: 8192,
			})
			method, ok := reflect.TypeOf(result).MethodByName("ServerErrorCode")
			if !ok || method.Type.NumOut() != 2 || method.Type.Out(0).Kind() != reflect.Int || method.Type.Out(1).Kind() != reflect.Bool {
				t.Fatal("ASSERT_SERVER_ERROR_CODE_EVIDENCE: external Result must expose (int, bool) ServerErrorCode")
			}
			values := reflect.ValueOf(result).MethodByName("ServerErrorCode").Call(nil)
			code, present := int(values[0].Int()), values[1].Bool()
			if code != tc.wantCode || present != tc.wantOK || result.Outcome() != tc.want || len(result.Raw()) != 0 && tc.want != transport.OutcomeTransportSuccess {
				t.Fatalf("ASSERT_SERVER_ERROR_CODE_EVIDENCE: code=%d present=%t outcome=%s raw=%q wantCode=%d wantPresent=%t wantOutcome=%s", code, present, result.Outcome(), result.Raw(), tc.wantCode, tc.wantOK, tc.want)
			}
			t.Logf("ASSERT_SERVER_ERROR_CODE_EVIDENCE: PASS code=%d present=%t outcome=%s", code, present, result.Outcome())
		})
	}
}

func TestExternalCallerObservesTypedTransportOutcome(t *testing.T) {
	for _, tc := range []struct {
		name         string
		support      bool
		generation   uint64
		result       sessionruntime.RoundTripResult
		want         string
		wantCalls    int
		wantRaw      string
		wantMessages int
	}{
		{"success", true, 1, sessionruntime.RoundTripResult{Result: json.RawMessage(`[]`), Messages: 1, Bytes: 30}, "TRANSPORT_SUCCESS", 1, `[]`, 1},
		{"preflight", true, 0, sessionruntime.RoundTripResult{}, "PREFLIGHT_FAILURE", 0, "", 0},
		{"unsupported", false, 1, sessionruntime.RoundTripResult{}, "UNSUPPORTED_CAPABILITY", 0, "", 0},
		{"transport failure", true, 1, sessionruntime.RoundTripResult{Failure: session.ResourceExhausted, Result: json.RawMessage(`{"unsafe":true}`), Messages: 7, Bytes: 10000}, "TRANSPORT_FAILURE", 1, "", 7},
		{"server error", true, 1, sessionruntime.RoundTripResult{ServerError: &lspwire.RPCError{Message: "invalid"}, Result: json.RawMessage(`{"unsafe":true}`), Messages: 1, Bytes: 50}, "SERVER_ERROR", 1, "", 1},
		{"timeout", true, 1, sessionruntime.RoundTripResult{Failure: session.RequestTimeout, Messages: 2}, "TIMEOUT", 1, "", 2},
		{"cancel", true, 1, sessionruntime.RoundTripResult{Failure: session.RequestCancelled, Messages: 3}, "CANCELLED", 1, "", 3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			runtime := &outcomeRuntime{result: tc.result}
			runtime.metadata.DefinitionSupport = tc.support
			result := transport.New(runtime).Execute(context.Background(), transport.Request{
				SessionID: "exact", Generation: tc.generation, Method: transport.MethodDefinition,
				Params:   json.RawMessage(`{"textDocument":{"uri":"file:///w/a.go"},"position":{"line":0,"character":0}}`),
				Deadline: time.Now().Add(time.Second), MaxMessages: 5, MaxBytes: 8192,
			})
			method, ok := reflect.TypeOf(result).MethodByName("Outcome")
			if !ok {
				t.Fatal("ASSERT_TYPED_OUTCOME_ACCESSOR: external Result has no Outcome method")
			}
			if method.Type.NumOut() != 1 || method.Type.Out(0).Name() != "Outcome" || method.Type.Out(0).PkgPath() != "lsp-trace/internal/adr0011methodtransport" || method.Type.Out(0).Kind() != reflect.String {
				t.Fatalf("ASSERT_TYPED_OUTCOME_ACCESSOR: result type=%v", method.Type)
			}
			got := reflect.ValueOf(result).MethodByName("Outcome").Call(nil)[0].String()
			if got != tc.want || runtime.calls != tc.wantCalls || string(result.Raw()) != tc.wantRaw || result.Messages() != tc.wantMessages {
				t.Fatalf("ASSERT_TYPED_OUTCOME_%s: outcome=%q calls=%d raw=%q messages=%d", tc.name, got, runtime.calls, result.Raw(), result.Messages())
			}
			t.Logf("ASSERT_TYPED_OUTCOME_%s: PASS", tc.name)
		})
	}
}
