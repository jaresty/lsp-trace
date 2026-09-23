package adr0011methodtransport

import (
	"context"
	"encoding/json"
	"testing"

	"lsp-trace/sessionruntime"
)

func TestSourceExpectationPreflightBeforeMetadata(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*Request)
	}{
		{"uncaptured", func(r *Request) { r.CaptureOwnedMethodPair = false }},
		{"wrong-query-uri", func(r *Request) { r.ExpectedOwnedDocument.URI = "file:///other.go" }},
		{"invalid-version", func(r *Request) { r.ExpectedOwnedDocument.Version = 0 }},
		{"invalid-digest", func(r *Request) { r.ExpectedOwnedDocument.SHA256 = "sha256:wrong" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := &fakeRuntime{metadata: sessionruntime.SessionMetadata{ReferencesSupport: true}}
			req := request(MethodReferences)
			req.CaptureOwnedMethodPair = true
			req.ExpectedOwnedDocument = &sessionruntime.OwnedDocumentBinding{URI: "file:///w/a.go", Version: 1, SHA256: "sha256:0000000000000000000000000000000000000000000000000000000000000000"}
			tc.change(&req)
			got := New(f).Execute(context.Background(), req)
			if got.Outcome() != OutcomePreflightFailure || f.metadataCalls != 0 || f.calls != 0 {
				t.Fatalf("ASSERT_SOURCE_PREFLIGHT_ZERO_IO: outcome=%s metadata=%d calls=%d", got.Outcome(), f.metadataCalls, f.calls)
			}
		})
	}
}

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
