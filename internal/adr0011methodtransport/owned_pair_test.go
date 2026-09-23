package adr0011methodtransport

import (
	"context"
	"encoding/json"
	"testing"

	"lsp-trace/sessionruntime"
)

func TestOptedInManagedPairMissingFailsClosed(t *testing.T) {
	f := &fakeRuntime{metadata: sessionruntime.SessionMetadata{ReferencesSupport: true}, result: sessionruntime.RoundTripResult{Result: json.RawMessage(`[]`), Messages: 1, Bytes: 32}}
	req := request(MethodReferences)
	req.CaptureOwnedMethodPair = true
	got := New(f).Execute(context.Background(), req)
	if !f.wire.CaptureOwnedMethodPair || got.Outcome() != OutcomeTransportFailure || got.Raw() != nil {
		t.Fatalf("ASSERT_OPTED_PAIR_REJECTS_MISSING_MANAGER_OBSERVATION: forwarded=%v outcome=%s raw=%q", f.wire.CaptureOwnedMethodPair, got.Outcome(), got.Raw())
	}
	if _, ok := got.OwnedPair(); ok {
		t.Fatal("ASSERT_OPTED_PAIR_WITHHELD_ON_FAILURE")
	}
}

func TestDefaultMethodTransportDoesNotRequirePair(t *testing.T) {
	f := &fakeRuntime{metadata: sessionruntime.SessionMetadata{ReferencesSupport: true}, result: sessionruntime.RoundTripResult{Result: json.RawMessage(`[]`), Messages: 1, Bytes: 32}}
	got := New(f).Execute(context.Background(), request(MethodReferences))
	if f.wire.CaptureOwnedMethodPair || got.Outcome() != OutcomeTransportSuccess || string(got.Raw()) != `[]` {
		t.Fatalf("ASSERT_DEFAULT_TRANSPORT_UNCHANGED: forwarded=%v outcome=%s raw=%q", f.wire.CaptureOwnedMethodPair, got.Outcome(), got.Raw())
	}
	if _, ok := got.OwnedPair(); ok {
		t.Fatal("ASSERT_DEFAULT_HAS_NO_PAIR")
	}
}
