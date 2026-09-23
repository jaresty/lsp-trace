package sessionruntime

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"
	"time"

	"lsp-trace/internal/session"
)

func ownedPairHookManager(t *testing.T, mode string, hook func(OwnedMethodPair)) (*Manager, StartResult) {
	t.Helper()
	m, err := New(Config{
		Limits:  Limits{MaxSessions: 1, MaxRequests: 1, MaxChildren: 2, MaxCancels: 2, MaxTombstones: 4, MaxObservations: 64, MaxOperations: 2},
		Starter: oneChildStarter{newRoundTripChild(mode)}, ownedMethodPairTestHook: hook,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = m.Shutdown(context.Background()) })
	s := m.Start(context.Background(), StartRequest{Profile: profile(t)})
	if ready := m.ObserveInitialization(s.SessionID, s.Generation, true); ready.State != session.Ready {
		t.Fatalf("fixture not READY: %+v", ready)
	}
	return m, s
}

func ownedPairHookRequest(s StartResult) RoundTripRequest {
	return RoundTripRequest{SessionID: s.SessionID, Generation: s.Generation, Method: "textDocument/references",
		Params:   json.RawMessage(`{"textDocument":{"uri":"file:///fixture/main.go"},"position":{"line":1,"character":0},"context":{"includeDeclaration":false}}`),
		Deadline: time.Now().Add(time.Second), MaxMessages: 3, MaxBytes: 4096, CaptureOwnedMethodPair: true}
}

func TestOwnedMethodPairManagerHookCopiesAndSurvivesPanic(t *testing.T) {
	calls := 0
	m, s := ownedPairHookManager(t, "references-repeated", func(p OwnedMethodPair) {
		calls++
		p.Params[0] = 'x'
		p.Result[0] = 'x'
		panic("observer panic must not break transport")
	})
	req := ownedPairHookRequest(s)
	original := append([]byte(nil), req.Params...)
	got := m.RoundTrip(context.Background(), req)
	req.Params[0] = 'z'
	got.Result[0] = 'z'
	if got.Failure != "" || got.ServerError != nil || calls != 1 {
		t.Fatalf("ASSERT_OWNED_PAIR_HOOK_SUCCESS: calls=%d failure=%s server=%v", calls, got.Failure, got.ServerError)
	}
	pair, ok := got.CompletedOwnedMethodPair()
	if !ok || !bytes.Equal(pair.Params, original) || len(pair.Result) == 0 || pair.Result[0] != '[' {
		t.Fatalf("ASSERT_OWNED_PAIR_HOOK_COPY: ok=%v pair=%+v", ok, pair)
	}
}

func TestOwnedMethodPairManagerHookSourceCopy(t *testing.T) {
	m, s, _, source := ownedDocumentFixture(t, true)
	t.Cleanup(func() { _ = m.Shutdown(context.Background()) })
	calls := 0
	m.ownedMethodPairTestHook = func(p OwnedMethodPair) {
		calls++
		if p.Source == nil || p.Source.URI != source.URI {
			t.Fatalf("ASSERT_OWNED_PAIR_HOOK_SOURCE: %+v", p.Source)
		}
		p.Source.URI = "file:///changed.go"
	}
	got := m.RoundTrip(context.Background(), methodForOwnedDocument(s, source))
	pair, ok := got.CompletedOwnedMethodPair()
	if calls != 1 || !ok || pair.Source == nil || pair.Source.URI != source.URI {
		t.Fatalf("ASSERT_OWNED_PAIR_HOOK_SOURCE_COPY: calls=%d ok=%v source=%+v failure=%s", calls, ok, pair.Source, got.Failure)
	}
}

func TestOwnedMethodPairManagerHookIneligible(t *testing.T) {
	for _, tc := range []struct {
		name, mode string
		change     func(*RoundTripRequest)
		off        bool
		checkPair  bool
	}{
		{"default-off", "references-empty", nil, true, false},
		{"capture-disabled", "references-empty", func(r *RoundTripRequest) { r.CaptureOwnedMethodPair = false }, false, true},
		{"non-dr", "success", func(r *RoundTripRequest) { r.Method = "test/method" }, false, false},
		{"server-error", "server-error", nil, false, false},
		{"unmatched-timeout", "unmatched-only", func(r *RoundTripRequest) { r.Deadline = time.Now().Add(30 * time.Millisecond) }, false, false},
		{"invalid-byte-bound", "references-empty", func(r *RoundTripRequest) { r.MaxBytes = 1<<20 + 1 }, false, false},
		{"invalid-message-bound", "references-empty", func(r *RoundTripRequest) { r.MaxMessages = 65 }, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			var hook func(OwnedMethodPair)
			if !tc.off {
				hook = func(OwnedMethodPair) { calls++ }
			}
			m, s := ownedPairHookManager(t, tc.mode, hook)
			req := ownedPairHookRequest(s)
			if tc.change != nil {
				tc.change(&req)
			}
			got := m.RoundTrip(context.Background(), req)
			if calls != 0 {
				t.Fatalf("ASSERT_OWNED_PAIR_HOOK_INELIGIBLE: calls=%d failure=%s", calls, got.Failure)
			}
			if tc.checkPair {
				if _, ok := got.CompletedOwnedMethodPair(); ok {
					t.Fatal("ASSERT_OWNED_PAIR_HOOK_CAPTURE_DISABLED_PAIR: completed pair despite capture disabled")
				}
			}
		})
	}
}
