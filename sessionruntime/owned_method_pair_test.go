package sessionruntime

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"
	"time"
)

func TestOwnedMethodPairBoundedManagedSuccess(t *testing.T) {
	m, s, _ := roundTripManager(t, "references-repeated")
	params := json.RawMessage(`{"textDocument":{"uri":"file:///fixture/main.go"},"position":{"line":1,"character":0},"context":{"includeDeclaration":false}}`)
	original := append([]byte(nil), params...)
	got := m.RoundTrip(context.Background(), RoundTripRequest{SessionID: s.SessionID, Generation: s.Generation, Method: "textDocument/references", Params: params, Deadline: time.Now().Add(time.Second), MaxMessages: 3, MaxBytes: 4096, CaptureOwnedMethodPair: true})
	pair, ok := got.CompletedOwnedMethodPair()
	if got.Failure != "" || !ok || pair.SessionID != s.SessionID || pair.Generation != s.Generation || pair.Key != got.Key || pair.Method != "textDocument/references" || !bytes.Equal(pair.Params, original) || !bytes.Equal(pair.Result, got.Result) || pair.Write.Key != got.Key || pair.Read.Key != got.Key {
		t.Fatalf("ASSERT_OWNED_METHOD_PAIR_KEYED_SUCCESS: ok=%v pair=%+v failure=%v", ok, pair, got.Failure)
	}
	if _, framed := got.CompletedMethodResponseFrame(); framed {
		t.Fatal("ASSERT_OWNED_METHOD_PAIR_NO_FRAME_DEPENDENCY")
	}
	params[0] = 'x'
	got.Result[0] = 'x'
	pair.Params[0] = 'x'
	pair.Result[0] = 'x'
	copied, ok := got.CompletedOwnedMethodPair()
	if !ok || !bytes.Equal(copied.Params, original) || copied.Result[0] != '[' {
		t.Fatal("ASSERT_OWNED_METHOD_PAIR_INDEPENDENT_COPIES")
	}
}

func TestOwnedMethodPairRejectsNonEligibleResults(t *testing.T) {
	for _, tc := range []struct {
		name, mode, method    string
		capture               bool
		maxBytes, maxMessages int64
	}{
		{"default-off", "references-empty", "textDocument/references", false, 4096, 1},
		{"non-method", "success", "test/method", true, 4096, 1},
		{"server-error", "server-error", "textDocument/definition", true, 4096, 1},
		{"unmatched-timeout", "unmatched-only", "textDocument/references", true, 4096, 1},
		{"over-byte-policy", "references-empty", "textDocument/references", true, 1<<20 + 1, 1},
		{"over-message-policy", "references-empty", "textDocument/references", true, 4096, 65},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m, s, _ := roundTripManager(t, tc.mode)
			deadline := time.Now().Add(time.Second)
			if tc.mode == "unmatched-only" {
				deadline = time.Now().Add(30 * time.Millisecond)
			}
			got := m.RoundTrip(context.Background(), RoundTripRequest{SessionID: s.SessionID, Generation: s.Generation, Method: tc.method, Params: json.RawMessage(`{"textDocument":{"uri":"file:///fixture/main.go"},"position":{"line":1,"character":0},"context":{"includeDeclaration":false}}`), Deadline: deadline, MaxMessages: int(tc.maxMessages), MaxBytes: tc.maxBytes, CaptureOwnedMethodPair: tc.capture})
			if _, ok := got.CompletedOwnedMethodPair(); ok {
				t.Fatal("ASSERT_OWNED_METHOD_PAIR_WITHHELD")
			}
		})
	}
}
