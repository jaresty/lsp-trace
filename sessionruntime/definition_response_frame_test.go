package sessionruntime

import (
	"bytes"
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

func definitionFixtureFrame(t *testing.T, message lspwire.Message) []byte {
	t.Helper()
	body, err := json.Marshal(message)
	if err != nil {
		t.Fatal(err)
	}
	return []byte(fmt.Sprintf("Content-Length: %d\r\n\r\n%s", len(body), body))
}

func definitionFixtureResponse(t *testing.T) []byte {
	t.Helper()
	return definitionFixtureFrame(t, lspwire.Message{
		JSONRPC: lspwire.Version, ID: json.RawMessage(`1`), Result: json.RawMessage(`{"ok":true}`),
	})
}

func TestDefinitionFrameCaptureBindsOnlyAcceptedSuccessfulResponse(t *testing.T) {
	frame := definitionFixtureResponse(t)
	notification := definitionFixtureFrame(t, lspwire.Message{
		JSONRPC: lspwire.Version, Method: "window/logMessage", Params: json.RawMessage(`{"type":3}`),
	})
	if len(notification) <= len(frame) {
		t.Fatal("fixture: unselected notification must exceed raw capture cap")
	}
	m, started, _ := roundTripManager(t, "success")
	t.Cleanup(func() { _ = m.Shutdown(context.Background()) })
	got := m.RoundTrip(context.Background(), RoundTripRequest{
		SessionID: started.SessionID, Generation: started.Generation, Method: "textDocument/definition",
		Deadline: time.Now().Add(time.Second), MaxMessages: 3, MaxBytes: 4096,
		CaptureDefinitionResponseFrameMaxBytes: int64(len(frame)),
	})
	if got.Failure != "" || got.Key.ID != 1 || got.Messages != 3 || len(got.Notifications) != 1 || len(got.Responses) != 1 || string(got.Result) != `{"ok":true}` {
		t.Fatalf("fixture: %+v", got)
	}
	raw, ok := got.CompletedDefinitionResponseFrame()
	observed, readOK := got.CompletedResponseRead()
	if !ok || !readOK || !bytes.Equal(raw, frame) || observed.FrameBytes != int64(len(frame)) || observed.FrameSHA256 != fmt.Sprintf("sha256:%x", sha256.Sum256(raw)) {
		t.Fatalf("ASSERT_ADR0011_MANAGER_DEFINITION_ACCEPTED_RAW: present=%v read=%v equal=%v observation=%+v", ok, readOK, bytes.Equal(raw, frame), observed)
	}
	raw[0] ^= 1
	again, ok := got.CompletedDefinitionResponseFrame()
	if !ok || !bytes.Equal(again, frame) {
		t.Fatal("ASSERT_ADR0011_MANAGER_DEFINITION_RAW_COPY: returned bytes alias stored frame")
	}
	encoded, err := json.Marshal(got)
	if err != nil || strings.Contains(string(encoded), "Content-Length:") || strings.Contains(string(encoded), "definitionResponseFrame") {
		t.Fatalf("ASSERT_ADR0011_MANAGER_DEFINITION_PRIVATE_JSON: err=%v encoded=%q", err, encoded)
	}
	if _, ok := got.CompletedRequestWrite(); !ok {
		t.Fatal("ASSERT_ADR0011_MANAGER_DEFINITION_PRESERVES_WRITE: missing")
	}
	t.Log("ASSERT_ADR0011_MANAGER_DEFINITION_ACCEPTED_RAW: PASS")
}

func TestDefinitionFrameCaptureWithholdsUnselectedAndTerminalFailures(t *testing.T) {
	frame := definitionFixtureResponse(t)
	// All three fixture responses fit the ordinary decoded-response budget except
	// the last byte of the accepted response in the matched-byte-bound case.
	budget := int64(0)
	for _, message := range []lspwire.Message{
		{JSONRPC: lspwire.Version, Method: "window/logMessage", Params: json.RawMessage(`{"type":3}`)},
		{JSONRPC: lspwire.Version, ID: json.RawMessage(`999`), Result: json.RawMessage(`null`)},
		{JSONRPC: lspwire.Version, ID: json.RawMessage(`1`), Result: json.RawMessage(`{"ok":true}`)},
	} {
		body, err := json.Marshal(message)
		if err != nil {
			t.Fatal(err)
		}
		budget += int64(len(body))
	}
	for _, tc := range []struct {
		name, mode, method string
		cap                int64
		maxMessages        int
		maxBytes           int64
		stale              bool
		failure            session.Failure
	}{
		{"accepted-over-raw-cap", "success", "textDocument/definition", int64(len(frame) - 1), 3, 4096, false, ""},
		{"default-off", "success", "textDocument/definition", 0, 3, 4096, false, ""},
		{"other-method", "success", "textDocument/references", int64(len(frame)), 3, 4096, false, ""},
		{"server-error", "server-error", "textDocument/definition", int64(len(frame) + 200), 1, 4096, false, ""},
		{"matched-byte-bound", "success", "textDocument/definition", int64(len(frame)), 3, budget - 1, false, session.ResourceExhausted},
		{"malformed-frame", "malformed", "textDocument/definition", int64(len(frame)), 1, 4096, false, session.SessionPoisoned},
		{"timeout", "hang", "textDocument/definition", int64(len(frame)), 1, 4096, false, session.RequestTimeout},
		{"stale", "server-error", "textDocument/definition", int64(len(frame)), 1, 4096, true, session.StaleGeneration},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m, started, _ := roundTripManager(t, tc.mode)
			t.Cleanup(func() { _ = m.Shutdown(context.Background()) })
			generation := started.Generation
			if tc.stale {
				generation++
			}
			got := m.RoundTrip(context.Background(), RoundTripRequest{
				SessionID: started.SessionID, Generation: generation, Method: tc.method,
				Deadline: time.Now().Add(50 * time.Millisecond), MaxMessages: tc.maxMessages, MaxBytes: tc.maxBytes,
				CaptureDefinitionResponseFrameMaxBytes: tc.cap,
			})
			if got.Failure != tc.failure {
				t.Fatalf("fixture failure=%s want=%s", got.Failure, tc.failure)
			}
			if raw, ok := got.CompletedDefinitionResponseFrame(); ok || raw != nil {
				t.Fatalf("ASSERT_ADR0011_MANAGER_DEFINITION_WITHHELD: case=%s present=%v raw=%q", tc.name, ok, raw)
			}
			if tc.name == "accepted-over-raw-cap" && (got.Messages != 3 || string(got.Result) != `{"ok":true}`) {
				t.Fatalf("ASSERT_ADR0011_MANAGER_DEFINITION_NONSELECTED_CONTINUES: messages=%d result=%q", got.Messages, got.Result)
			}
			if tc.name == "server-error" && got.ServerError == nil {
				t.Fatal("fixture: server error missing")
			}
			t.Log("ASSERT_ADR0011_MANAGER_DEFINITION_WITHHELD: PASS")
		})
	}
}
