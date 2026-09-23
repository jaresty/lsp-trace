package adr0011methodresult

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"
	"time"

	transport "lsp-trace/internal/adr0011methodtransport"
	"lsp-trace/sessionruntime"
)

type tamperOwnedPairRuntime struct {
	*sessionruntime.Manager
	change func(*sessionruntime.RoundTripResult)
}

func (r tamperOwnedPairRuntime) RoundTrip(ctx context.Context, req sessionruntime.RoundTripRequest) sessionruntime.RoundTripResult {
	result := r.Manager.RoundTrip(ctx, req)
	r.change(&result)
	return result
}

func TestOptedInOwnedPairManagedPeerRejectsRuntimeSubstitution(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*sessionruntime.RoundTripResult)
	}{
		{"result", func(r *sessionruntime.RoundTripResult) { r.Result = json.RawMessage(`[]`) }},
		{"key", func(r *sessionruntime.RoundTripResult) { r.Key.ID++ }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			manager, started, _ := startManagedMethodPeer(t, "")
			req := fixtureRequest(transport.MethodDefinition, false)
			req.SessionID, req.Generation = started.SessionID, started.Generation
			req.Deadline = time.Now().Add(5 * time.Second)
			req.CaptureOwnedMethodPair = true
			got := transport.New(tamperOwnedPairRuntime{Manager: manager, change: tc.change}).Execute(context.Background(), req)
			if got.Outcome() != transport.OutcomeTransportFailure || got.Raw() != nil {
				t.Fatalf("ASSERT_OPTED_PAIR_REJECTS_SUBSTITUTION: %s result=%q", got.Outcome(), got.Raw())
			}
			if _, ok := got.OwnedPair(); ok {
				t.Fatal("ASSERT_OPTED_PAIR_NO_LEAK_AFTER_SUBSTITUTION")
			}
		})
	}
}

func TestOptedInOwnedPairManagedPeerNoFrames(t *testing.T) {
	for _, tc := range []struct{ method, raw string }{
		{transport.MethodDefinition, `null`},
		{transport.MethodReferences, `[]`},
	} {
		t.Run(tc.method, func(t *testing.T) {
			manager, started, _ := startManagedMethodPeer(t, "")
			req := fixtureRequest(tc.method, false)
			req.SessionID, req.Generation = started.SessionID, started.Generation
			req.Deadline = time.Now().Add(5 * time.Second)
			req.CaptureOwnedMethodPair = true
			observed := &methodWireObservedRuntime{Manager: manager}
			got := transport.New(observed).Execute(context.Background(), req)
			pair, ok := got.OwnedPair()
			if got.Outcome() != transport.OutcomeTransportSuccess || !ok || pair.Key.ID == 0 || pair.Key.Generation != req.Generation || pair.SessionID != req.SessionID || pair.Method != req.Method || !bytes.Equal(pair.Params, req.Params) || string(pair.Result) != tc.raw {
				t.Fatalf("ASSERT_OPTED_PAIR_REAL_MANAGED_SUCCESS: outcome=%s ok=%v pair=%+v", got.Outcome(), ok, pair)
			}
			if _, present := observed.last.CompletedMethodResponseFrame(); present {
				t.Fatal("ASSERT_OPTED_PAIR_NO_FRAME_REQUIREMENT")
			}
			pair.Result[0] = 'x'
			pair.Params[0] = 'x'
			replayed, present := got.OwnedPair()
			if !present || string(replayed.Result) != tc.raw || !bytes.Equal(replayed.Params, req.Params) {
				t.Fatal("ASSERT_OPTED_PAIR_COPY_ISOLATION")
			}
		})
	}
}
