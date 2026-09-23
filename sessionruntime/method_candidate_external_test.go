package sessionruntime_test

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"lsp-trace/internal/adr0011methodtransport"
	"lsp-trace/internal/lspwire"
	"lsp-trace/internal/session"
	"lsp-trace/sessionruntime"
)

type fakeMethodRuntimeForOwnerProbe struct{}

func (fakeMethodRuntimeForOwnerProbe) Metadata(string, uint64) (sessionruntime.SessionMetadata, session.Failure) {
	return sessionruntime.SessionMetadata{ReferencesSupport: true}, ""
}
func (fakeMethodRuntimeForOwnerProbe) RoundTrip(_ context.Context, req sessionruntime.RoundTripRequest) sessionruntime.RoundTripResult {
	return sessionruntime.RoundTripResult{
		Key:    lspwire.RequestKey{Generation: req.Generation, ID: 1},
		Result: json.RawMessage(`[]`), Messages: 1, Bytes: 2, RequestMessages: 1, RequestBytes: 64,
	}
}

func TestFakeRuntimeTransportSuccessIsNotManagerOwnerCapture(t *testing.T) {
	field, ok := reflect.TypeOf(sessionruntime.Config{}).FieldByName("methodCandidateTestHook")
	if !ok || field.PkgPath == "" {
		t.Fatalf("ASSERT_ADR0011_OWNER_EXTERNAL_HOOK_UNEXPORTED: exists=%v pkg=%q", ok, field.PkgPath)
	}
	out := adr0011methodtransport.New(fakeMethodRuntimeForOwnerProbe{}).Execute(context.Background(), adr0011methodtransport.Request{
		SessionID: "fake-session", Generation: 1, Method: adr0011methodtransport.MethodReferences,
		Params:   json.RawMessage(`{"textDocument":{"uri":"file:///fixture/main.go"},"position":{"line":1,"character":2},"context":{"includeDeclaration":false}}`),
		Deadline: time.Now().Add(time.Second), MaxMessages: 1, MaxBytes: 4096,
	})
	if out.Outcome() != adr0011methodtransport.OutcomeTransportSuccess {
		t.Fatalf("fixture fake transport should remain substitutable: outcome=%s failure=%s", out.Outcome(), out.FailureText())
	}
	// This only checks the Go external-package surface: no concrete manager or
	// owner hook ran. It does NOT defend against privileged same-process forgery.
	t.Log("ASSERT_ADR0011_OWNER_EXTERNAL_HOOK_UNEXPORTED: PASS (fake transport success is not owner capture)")
}
