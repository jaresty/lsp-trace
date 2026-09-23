package adr0011methodtransport

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"lsp-trace/internal/lspwire"
	"lsp-trace/internal/session"
	"lsp-trace/sessionruntime"
)

type fakeRuntime struct {
	metadata           sessionruntime.SessionMetadata
	metadataFailure    session.Failure
	metadataID         string
	metadataGeneration uint64
	metadataCalls      int
	wire               sessionruntime.RoundTripRequest
	calls              int
	result             sessionruntime.RoundTripResult
}

func (f *fakeRuntime) Metadata(id string, generation uint64) (sessionruntime.SessionMetadata, session.Failure) {
	f.metadataCalls++
	f.metadataID, f.metadataGeneration = id, generation
	return f.metadata, f.metadataFailure
}
func (f *fakeRuntime) RoundTrip(_ context.Context, req sessionruntime.RoundTripRequest) sessionruntime.RoundTripResult {
	f.calls++
	f.wire = req
	return f.result
}

func request(method string) Request {
	params := `{"textDocument":{"uri":"file:///w/a.go"},"position":{"line":2,"character":3}`
	if method == MethodReferences {
		params += `,"context":{"includeDeclaration":true}`
	}
	params += `}`
	return Request{SessionID: "session-exact", Generation: 7, Method: method, Params: json.RawMessage(params), Deadline: time.Now().Add(20 * time.Second), MaxMessages: 5, MaxBytes: 8192}
}

func TestBothMethodsForwardExactWireRequest(t *testing.T) {
	for _, method := range []string{MethodDefinition, MethodReferences} {
		t.Run(method, func(t *testing.T) {
			f := &fakeRuntime{metadata: sessionruntime.SessionMetadata{DefinitionSupport: true, ReferencesSupport: true}, result: sessionruntime.RoundTripResult{Result: json.RawMessage(`[{}]`), Messages: 2, Bytes: 101}}
			req := request(method)
			got := New(f).Execute(context.Background(), req)
			if got.status != transportSuccess {
				t.Fatalf("ASSERT_TRANSPORT_SUCCESS: status=%v failure=%q", got.status, got.failure)
			}
			if f.calls != 1 {
				t.Fatalf("ASSERT_SINGLE_ROUNDTRIP: got %d calls", f.calls)
			}
			if f.wire.SessionID != req.SessionID || f.wire.Generation != req.Generation || f.wire.Method != method || string(f.wire.Params) != string(req.Params) || !f.wire.Deadline.Equal(req.Deadline) || f.wire.MaxMessages != req.MaxMessages || f.wire.MaxBytes != req.MaxBytes {
				t.Fatalf("ASSERT_EXACT_WIRE_FIELDS: got %#v", f.wire)
			}
			if string(got.Raw()) != `[{}]` {
				t.Fatalf("ASSERT_RAW_RESULT_PRESERVED: got %s", got.Raw())
			}
		})
	}
}

func TestUnsupportedCapabilitySkipsRoundTrip(t *testing.T) {
	f := &fakeRuntime{metadata: sessionruntime.SessionMetadata{DefinitionSupport: true}}
	got := New(f).Execute(context.Background(), request(MethodReferences))
	if got.status != unsupportedCapability {
		t.Fatalf("ASSERT_UNSUPPORTED_CAPABILITY: got %v", got.status)
	}
	if f.calls != 0 {
		t.Fatalf("ASSERT_UNSUPPORTED_ZERO_CALLS: got %d", f.calls)
	}
}

func TestMetadataFailurePreservesExactStaleGenerationAndSkipsRoundTrip(t *testing.T) {
	f := &fakeRuntime{metadataFailure: session.Failure("stale generation")}
	req := request(MethodDefinition)
	got := New(f).Execute(context.Background(), req)
	if got.status != preflightFailure || got.failure != "stale generation" {
		t.Fatalf("ASSERT_STALE_GENERATION_FAILURE: %#v", got)
	}
	if f.metadataCalls != 1 || f.metadataID != req.SessionID || f.metadataGeneration != req.Generation {
		t.Fatalf("ASSERT_EXACT_METADATA_LOOKUP: id=%q generation=%d calls=%d", f.metadataID, f.metadataGeneration, f.metadataCalls)
	}
	if f.calls != 0 {
		t.Fatalf("ASSERT_STALE_GENERATION_ZERO_CALLS: got %d", f.calls)
	}
}

func TestInvalidMethodAndBoundsArePreflightOnly(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*Request)
	}{
		{"method", func(r *Request) { r.Method = "textDocument/hover" }},
		{"generation", func(r *Request) { r.Generation = 0 }},
		{"deadline", func(r *Request) { r.Deadline = time.Time{} }},
		{"messages", func(r *Request) { r.MaxMessages = 0 }},
		{"message hard limit", func(r *Request) { r.MaxMessages = maxMessages + 1 }},
		{"bytes", func(r *Request) { r.MaxBytes = 0 }},
		{"byte hard limit", func(r *Request) { r.MaxBytes = maxBytes + 1 }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := &fakeRuntime{metadata: sessionruntime.SessionMetadata{DefinitionSupport: true}}
			req := request(MethodDefinition)
			tc.mutate(&req)
			got := New(f).Execute(context.Background(), req)
			if got.status != preflightFailure {
				t.Fatalf("ASSERT_PREFLIGHT_REJECTS_%s: got %v", tc.name, got.status)
			}
			if f.metadataCalls != 0 || f.calls != 0 {
				t.Fatalf("ASSERT_INVALID_ZERO_EXTERNAL_CALLS: metadata=%d roundtrip=%d", f.metadataCalls, f.calls)
			}
		})
	}
}

func TestDistinctTransportAndServerFailuresWithObservedAccounting(t *testing.T) {
	t.Run("transport", func(t *testing.T) {
		f := &fakeRuntime{metadata: sessionruntime.SessionMetadata{DefinitionSupport: true}, result: sessionruntime.RoundTripResult{Failure: session.ResourceExhausted, Messages: 99, Bytes: 999999}}
		got := New(f).Execute(context.Background(), request(MethodDefinition))
		if got.status != transportFailure || got.failure != string(session.ResourceExhausted) {
			t.Fatalf("ASSERT_TRANSPORT_FAILURE_DISTINCT: %#v", got)
		}
		if got.messages != 99 || got.bytes != 999999 {
			t.Fatalf("ASSERT_ACCOUNTING_OBSERVED: messages=%d bytes=%d", got.messages, got.bytes)
		}
	})
	t.Run("server", func(t *testing.T) {
		f := &fakeRuntime{metadata: sessionruntime.SessionMetadata{DefinitionSupport: true}, result: sessionruntime.RoundTripResult{
			ServerError: &lspwire.RPCError{Message: "bad request"}, Result: json.RawMessage(`{"untrusted":true}`),
		}}
		got := New(f).Execute(context.Background(), request(MethodDefinition))
		if got.status != serverError || got.server != "bad request" {
			t.Fatalf("ASSERT_JSONRPC_SERVER_ERROR: %#v", got)
		}
		if len(got.Raw()) != 0 {
			t.Fatalf("ASSERT_SERVER_ERROR_HAS_NO_RAW_RESULT: %s", got.Raw())
		}
	})
}

type terminalContext struct {
	context.Context
	err      error
	deadline time.Time
	done     chan struct{}
}

func (c terminalContext) Err() error                  { return c.err }
func (c terminalContext) Deadline() (time.Time, bool) { return c.deadline, true }
func (c terminalContext) Done() <-chan struct{}       { return c.done }

func TestContextTimeoutAndCancellationAreDistinct(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
		want outcome
	}{{"timeout", context.DeadlineExceeded, timedOut}, {"canceled", context.Canceled, canceled}} {
		t.Run(tc.name, func(t *testing.T) {
			deadline := time.Now().Add(20 * time.Second)
			done := make(chan struct{})
			close(done)
			ctx := terminalContext{Context: context.Background(), err: tc.err, deadline: deadline, done: done}
			req := request(MethodDefinition)
			req.Deadline = deadline
			f := &fakeRuntime{metadata: sessionruntime.SessionMetadata{DefinitionSupport: true}}
			got := New(f).Execute(ctx, req)
			if got.status != tc.want {
				t.Fatalf("ASSERT_CONTEXT_%s_DISTINCT: got %v", tc.name, got.status)
			}
		})
	}
}

func TestCanonicalParameterShapeRequired(t *testing.T) {
	cases := []string{`null`, `{}`, `{"textDocument":{"uri":"x"}}`, `{"textDocument":{"uri":"x"},"position":{"line":0,"character":0},"extra":true}`}
	for _, raw := range cases {
		t.Run(raw, func(t *testing.T) {
			f := &fakeRuntime{metadata: sessionruntime.SessionMetadata{DefinitionSupport: true}}
			req := request(MethodDefinition)
			req.Params = json.RawMessage(raw)
			got := New(f).Execute(context.Background(), req)
			if got.status != preflightFailure || f.calls != 0 {
				t.Fatalf("ASSERT_PARAMS_REJECTED_BEFORE_ROUNDTRIP: status=%v calls=%d", got.status, f.calls)
			}
		})
	}
}

func TestDeadlineAndContextAreBounded(t *testing.T) {
	f := &fakeRuntime{metadata: sessionruntime.SessionMetadata{DefinitionSupport: true}}
	req := request(MethodDefinition)
	req.Deadline = time.Now().Add(maxDeadline + time.Second)
	got := New(f).Execute(context.Background(), req)
	if got.status != preflightFailure || f.metadataCalls != 0 {
		t.Fatalf("ASSERT_EXCESS_DEADLINE_PRECHECK: status=%v metadata=%d", got.status, f.metadataCalls)
	}
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(time.Second))
	defer cancel()
	req = request(MethodDefinition)
	req.Deadline = time.Now().Add(2 * time.Second)
	got = New(f).Execute(ctx, req)
	if got.status != preflightFailure || f.metadataCalls != 0 {
		t.Fatalf("ASSERT_DEADLINE_WITHIN_CONTEXT: status=%v metadata=%d", got.status, f.metadataCalls)
	}
}

func TestNilDependenciesArePreflightErrors(t *testing.T) {
	got := New(nil).Execute(context.Background(), request(MethodDefinition))
	if got.status != preflightFailure {
		t.Fatalf("ASSERT_NIL_RUNTIME_PREFLIGHT: %#v", got)
	}
}

func TestTypedRuntimeFailureWithoutContextError(t *testing.T) {
	for _, tc := range []struct {
		name    string
		failure session.Failure
		want    outcome
	}{
		{"timeout", session.RequestTimeout, timedOut},
		{"cancelled", session.RequestCancelled, canceled},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := &fakeRuntime{
				metadata: sessionruntime.SessionMetadata{DefinitionSupport: true},
				result: sessionruntime.RoundTripResult{
					Failure: tc.failure, Result: json.RawMessage(`{"untrusted":true}`),
					Messages: 1, Bytes: 50,
				},
			}
			got := New(f).Execute(context.Background(), request(MethodDefinition))
			if got.status != tc.want {
				t.Fatalf("ASSERT_TYPED_MANAGED_FAILURE: got %v, want %v", got.status, tc.want)
			}
			if len(got.Raw()) != 0 {
				t.Fatalf("ASSERT_FAILURE_HAS_NO_RAW_RESULT: %s", got.Raw())
			}
		})
	}
}

func TestReportedBoundsFailClosed(t *testing.T) {
	ordinaryResult := json.RawMessage(`[{"untrusted":true}]`)
	oversizedResult := json.RawMessage(`"` + strings.Repeat("x", 8191) + `"`)
	for _, tc := range []struct {
		name      string
		messages  int
		bytes     int64
		result    json.RawMessage
		wantMsgs  int
		wantBytes int64
	}{
		{"messages", 6, 50, ordinaryResult, 6, 50},
		{"bytes", 1, 8193, ordinaryResult, 1, 8193},
		{"negative messages", -1, 50, ordinaryResult, -1, 50},
		{"negative bytes", 1, -1, ordinaryResult, 1, -1},
		{"raw exceeds declared bytes", 1, 50, oversizedResult, 1, 50},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := &fakeRuntime{
				metadata: sessionruntime.SessionMetadata{ReferencesSupport: true},
				result: sessionruntime.RoundTripResult{
					Result:   tc.result,
					Messages: tc.messages, Bytes: tc.bytes,
				},
			}
			got := New(f).Execute(context.Background(), request(MethodReferences))
			if got.status != transportFailure {
				t.Fatalf("ASSERT_OVERSHOOT_NOT_SUCCESS: got %v", got.status)
			}
			if len(got.Raw()) != 0 {
				t.Fatalf("ASSERT_OVERSHOOT_HAS_NO_RAW_RESULT: %s", got.Raw())
			}
			if got.Messages() != tc.wantMsgs || got.Bytes() != tc.wantBytes {
				t.Fatalf("ASSERT_OVERSHOOT_NOT_CLAMPED: got messages=%d bytes=%d", got.Messages(), got.Bytes())
			}
		})
	}
}
