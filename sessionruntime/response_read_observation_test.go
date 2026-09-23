package sessionruntime

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"lsp-trace/internal/lspwire"
	"lsp-trace/internal/session"
)

func expectManagedResponseFrame(t *testing.T, got RoundTripResult, sessionID string, generation uint64, response lspwire.Message) {
	t.Helper()
	observed, ok := got.CompletedResponseRead()
	if !ok || got.Failure != "" || observed.SessionID != sessionID || observed.Generation != generation ||
		observed.Key != got.Key || observed.Key.Generation != generation {
		t.Fatalf("ASSERT_MANAGED_RESPONSE_ACCEPTED_FRAME: present=%v failure=%s observed=%+v key=%+v", ok, got.Failure, observed, got.Key)
	}
	body, err := json.Marshal(response)
	if err != nil {
		t.Fatal(err)
	}
	frame := fmt.Sprintf("Content-Length: %d\r\n\r\n%s", len(body), body)
	wantDigest := fmt.Sprintf("sha256:%x", sha256.Sum256([]byte(frame)))
	if observed.FrameBytes != int64(len(frame)) || observed.FrameSHA256 != wantDigest {
		t.Fatalf("ASSERT_MANAGED_RESPONSE_EXACT_FRAME: got=(%d,%s) want=(%d,%s)", observed.FrameBytes, observed.FrameSHA256, len(frame), wantDigest)
	}
	observed.FrameSHA256 = "changed"
	again, ok := got.CompletedResponseRead()
	if !ok || again.FrameSHA256 != wantDigest {
		t.Fatalf("ASSERT_MANAGED_RESPONSE_VALUE_COPY: present=%v again=%+v", ok, again)
	}
	encoded, err := json.Marshal(got)
	if err != nil || strings.Contains(string(encoded), "FrameSHA256") || strings.Contains(string(encoded), "responseRead") || strings.Contains(string(encoded), wantDigest) {
		t.Fatalf("ASSERT_MANAGED_RESPONSE_JSON_UNCHANGED: err=%v encoded=%q", err, encoded)
	}
	t.Log("ASSERT_MANAGED_RESPONSE_ACCEPTED_FRAME: PASS")
	t.Log("ASSERT_MANAGED_RESPONSE_EXACT_FRAME: PASS")
}

func TestCompletedResponseReadBindsOnlyAcceptedManagerFrame(t *testing.T) {
	for _, tc := range []struct {
		name, mode string
		messages   int
		wantError  bool
	}{
		{"interleaved-success", "success", 3, false},
		{"server-error", "server-error", 1, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m, started, _ := roundTripManager(t, tc.mode)
			t.Cleanup(func() { _ = m.Shutdown(context.Background()) })
			got := m.RoundTrip(context.Background(), RoundTripRequest{
				SessionID: started.SessionID, Generation: started.Generation,
				Method: "textDocument/definition", Params: json.RawMessage(`{"textDocument":{"uri":"file:///a.go"}}`),
				Deadline: time.Now().Add(time.Second), MaxMessages: tc.messages, MaxBytes: 4096,
			})
			if got.Messages != tc.messages || (got.ServerError != nil) != tc.wantError || tc.name == "interleaved-success" && len(got.Responses) != 1 {
				t.Fatalf("fixture setup: messages=%d responses=%d serverError=%+v", got.Messages, len(got.Responses), got.ServerError)
			}
			response := lspwire.Message{JSONRPC: lspwire.Version, ID: json.RawMessage(fmt.Sprint(got.Key.ID))}
			if tc.wantError {
				response.Error = &lspwire.RPCError{Code: -32603, Message: "boom"}
			} else {
				response.Result = json.RawMessage(`{"ok":true}`)
			}
			expectManagedResponseFrame(t, got, started.SessionID, started.Generation, response)
			if _, ok := got.CompletedRequestWrite(); !ok {
				t.Fatal("ASSERT_MANAGED_RESPONSE_PRESERVES_COMPLETED_WRITE: missing")
			}
		})
	}
}

func TestCompletedResponseReadWithheldWithoutAcceptedBoundedResponse(t *testing.T) {
	for _, tc := range []struct {
		name, mode  string
		maxMessages int
		maxBytes    int64
		stale       bool
		want        session.Failure
	}{
		{"unmatched-message-limit", "success", 2, 4096, false, session.ResourceExhausted},
		{"matched-over-byte-limit", "server-error", 1, 1, false, session.ResourceExhausted},
		{"malformed-frame", "malformed", 1, 4096, false, session.SessionPoisoned},
		{"timeout", "hang", 1, 4096, false, session.RequestTimeout},
		{"stale-generation", "server-error", 1, 4096, true, session.StaleGeneration},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m, started, _ := roundTripManager(t, tc.mode)
			t.Cleanup(func() { _ = m.Shutdown(context.Background()) })
			generation := started.Generation
			if tc.stale {
				generation++
			}
			got := m.RoundTrip(context.Background(), RoundTripRequest{
				SessionID: started.SessionID, Generation: generation,
				Method: "textDocument/definition", Deadline: time.Now().Add(35 * time.Millisecond),
				MaxMessages: tc.maxMessages, MaxBytes: tc.maxBytes,
			})
			if got.Failure != tc.want {
				t.Fatalf("fixture setup: case=%s failure=%s want=%s", tc.name, got.Failure, tc.want)
			}
			if observed, ok := got.CompletedResponseRead(); ok || observed != (ResponseReadObservation{}) {
				t.Fatalf("ASSERT_MANAGED_RESPONSE_WITHHELD: case=%s present=%v observation=%+v", tc.name, ok, observed)
			}
			t.Log("ASSERT_MANAGED_RESPONSE_WITHHELD: PASS")
		})
	}
}
