package sessionruntime

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	"lsp-trace/internal/lspwire"
	"lsp-trace/internal/session"
)

func expectedMethodRequestFrame(t *testing.T, req RoundTripRequest, id uint64) []byte {
	t.Helper()
	return definitionFixtureFrame(t, lspwire.Message{
		JSONRPC: lspwire.Version, ID: json.RawMessage(strconv.FormatUint(id, 10)), Method: req.Method, Params: req.Params,
	})
}

func TestMethodRequestFrameAcceptedExactWrite(t *testing.T) {
	for _, tc := range []struct {
		name, mode, method, params string
		messages                   int
	}{
		{"definition", "success", "textDocument/definition", `{"textDocument":{"uri":"file:///a.go"},"position":{"line":1,"character":2}}`, 3},
		{"references-spaced-params", "references-empty", "textDocument/references", `{ "textDocument" : { "uri" : "file:///a.go" }, "position" : { "line": 1, "character": 2 }, "context" : { "includeDeclaration" : false } }`, 1},
		{"references-repeated", "references-repeated", "textDocument/references", `{"textDocument":{"uri":"file:///a.go"},"position":{"line":1,"character":2},"context":{"includeDeclaration":false}}`, 3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m, started, _ := roundTripManager(t, tc.mode)
			t.Cleanup(func() { _ = m.Shutdown(context.Background()) })
			req := RoundTripRequest{SessionID: started.SessionID, Generation: started.Generation, Method: tc.method,
				Params: json.RawMessage(tc.params), Deadline: time.Now().Add(time.Second), MaxMessages: tc.messages, MaxBytes: 4096}
			// The cap is bound to this single reconstructed fixture frame, not the result body.
			req.CaptureMethodRequestFrameMaxBytes = int64(len(expectedMethodRequestFrame(t, req, 1)))
			got := m.RoundTrip(context.Background(), req)
			if got.Failure != "" || got.ServerError != nil || got.Key.ID != 1 {
				t.Fatalf("fixture failed: failure=%s server=%v key=%+v", got.Failure, got.ServerError, got.Key)
			}
			frame := expectedMethodRequestFrame(t, req, got.Key.ID)
			observed, written := got.CompletedRequestWrite()
			raw, present := got.CompletedMethodRequestFrame()
			if !written || !present || !bytes.Equal(raw, frame) || observed.FrameBytes != int64(len(frame)) || observed.FrameSHA256 != fmt.Sprintf("sha256:%x", sha256.Sum256(raw)) {
				t.Fatalf("ASSERT_ADR0011_DR_OWNER_REQUEST_EXACT: case=%s written=%v present=%v equal=%v observation=%+v", tc.name, written, present, bytes.Equal(raw, frame), observed)
			}
			raw[0] ^= 1
			again, ok := got.CompletedMethodRequestFrame()
			if !ok || !bytes.Equal(again, frame) {
				t.Fatal("ASSERT_ADR0011_DR_OWNER_REQUEST_DEFENSIVE_COPY: request-frame bytes aliased")
			}
			encoded, err := json.Marshal(got)
			if err != nil || strings.Contains(string(encoded), "Content-Length:") || strings.Contains(string(encoded), "methodRequestFrame") {
				t.Fatalf("ASSERT_ADR0011_DR_OWNER_REQUEST_PRIVATE: err=%v json=%s", err, encoded)
			}
			t.Log("ASSERT_ADR0011_DR_OWNER_REQUEST_EXACT: PASS")
		})
	}
}

func TestMethodRequestFrameWithheldUntilAcceptedSuccess(t *testing.T) {
	for _, tc := range []struct {
		name, mode, method string
		messages           int
		cap                int64
		maxBytes           int64
		stale              bool
		failure            session.Failure
	}{
		{"default-off", "references-empty", "textDocument/references", 1, 0, 4096, false, ""},
		{"other-method", "references-empty", "textDocument/hover", 1, 4096, 4096, false, ""},
		{"over-cap-interleaved", "references-repeated", "textDocument/references", 3, 1, 4096, false, ""},
		{"cap-minus-one", "references-empty", "textDocument/references", 1, -1, 4096, false, ""},
		{"over-ceiling", "references-empty", "textDocument/references", 1, (2 << 20) + 1, 4096, false, ""},
		{"server-error", "server-error", "textDocument/references", 1, 4096, 4096, false, ""},
		{"malformed", "malformed", "textDocument/references", 1, 4096, 4096, false, session.SessionPoisoned},
		{"timeout", "hang", "textDocument/references", 1, 4096, 4096, false, session.RequestTimeout},
		{"unmatched-only", "unmatched-only", "textDocument/references", 2, 4096, 4096, false, session.RequestTimeout},
		{"stale", "references-empty", "textDocument/references", 1, 4096, 4096, true, session.StaleGeneration},
		{"response-byte-limit", "references-empty", "textDocument/references", 1, 4096, 1, false, session.ResourceExhausted},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m, started, _ := roundTripManager(t, tc.mode)
			t.Cleanup(func() { _ = m.Shutdown(context.Background()) })
			generation := started.Generation
			if tc.stale {
				generation++
			}
			req := RoundTripRequest{
				SessionID: started.SessionID, Generation: generation, Method: tc.method,
				Deadline: time.Now().Add(50 * time.Millisecond), MaxMessages: tc.messages, MaxBytes: tc.maxBytes,
				CaptureMethodRequestFrameMaxBytes: tc.cap,
			}
			if tc.cap == -1 {
				req.CaptureMethodRequestFrameMaxBytes = int64(len(expectedMethodRequestFrame(t, req, 1)) - 1)
			}
			got := m.RoundTrip(context.Background(), req)
			if got.Failure != tc.failure {
				t.Fatalf("fixture: case=%s failure=%s want=%s", tc.name, got.Failure, tc.failure)
			}
			if raw, ok := got.CompletedMethodRequestFrame(); ok || raw != nil {
				t.Fatalf("ASSERT_ADR0011_DR_OWNER_REQUEST_WITHHELD: case=%s present=%v raw=%q", tc.name, ok, raw)
			}
			t.Log("ASSERT_ADR0011_DR_OWNER_REQUEST_WITHHELD: PASS")
		})
	}
}

func TestMethodRequestFrameShortWriteCannotExposeBytes(t *testing.T) {
	child := &shortWriteChild{newRoundTripChild("hang")}
	m, err := New(Config{Limits: Limits{MaxSessions: 1, MaxRequests: 1, MaxChildren: 2, MaxCancels: 2, MaxTombstones: 4, MaxObservations: 64, MaxOperations: 2}, Starter: oneChildStarter{child}})
	if err != nil {
		t.Fatal(err)
	}
	started := m.Start(context.Background(), StartRequest{Profile: profile(t)})
	if ready := m.ObserveInitialization(started.SessionID, started.Generation, true); ready.State != session.Ready {
		t.Fatalf("fixture not READY: %+v", ready)
	}
	got := m.RoundTrip(context.Background(), RoundTripRequest{
		SessionID: started.SessionID, Generation: started.Generation, Method: "textDocument/references",
		Deadline: time.Now().Add(time.Second), MaxMessages: 1, MaxBytes: 4096, CaptureMethodRequestFrameMaxBytes: 4096,
	})
	if got.Failure != session.SessionPoisoned || got.RequestBytes == 0 {
		t.Fatalf("fixture short write: failure=%s requestBytes=%d", got.Failure, got.RequestBytes)
	}
	if _, ok := got.CompletedRequestWrite(); ok {
		t.Fatal("ASSERT_ADR0011_DR_OWNER_SHORT_WRITE_WITHHELD: write observation exposed")
	}
	if raw, ok := got.CompletedMethodRequestFrame(); ok || raw != nil {
		t.Fatal("ASSERT_ADR0011_DR_OWNER_SHORT_WRITE_WITHHELD: raw frame exposed")
	}
	t.Log("ASSERT_ADR0011_DR_OWNER_SHORT_WRITE_WITHHELD: PASS")
}
