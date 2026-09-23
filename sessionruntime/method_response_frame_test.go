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

func referenceResultBody() json.RawMessage {
	location := `{"uri":"file:///fixture/main.go","range":{"start":{"line":1,"character":0},"end":{"line":1,"character":1}}}`
	return json.RawMessage(`[` + location + `,` + location + `]`)
}

func TestMethodFrameCaptureReferencesEmptyAndRepeated(t *testing.T) {
	for _, tc := range []struct {
		name, mode string
		result     json.RawMessage
		messages   int
		members    int
		isNull     bool
	}{
		{"empty", "references-empty", json.RawMessage(`[]`), 1, 0, false},
		{"null", "references-null", json.RawMessage(`null`), 1, 0, true},
		{"repeated", "references-repeated", referenceResultBody(), 3, 2, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			frame := definitionFixtureFrame(t, lspwire.Message{JSONRPC: lspwire.Version, ID: json.RawMessage(`1`), Result: tc.result})
			m, started, child := roundTripManager(t, tc.mode)
			t.Cleanup(func() { _ = m.Shutdown(context.Background()) })
			params := json.RawMessage(`{"textDocument":{"uri":"file:///fixture/main.go"},"position":{"line":1,"character":2},"context":{"includeDeclaration":false}}`)
			got := m.RoundTrip(context.Background(), RoundTripRequest{
				SessionID: started.SessionID, Generation: started.Generation, Method: "textDocument/references", Params: params,
				Deadline: time.Now().Add(time.Second), MaxMessages: tc.messages, MaxBytes: 4096,
				CaptureReferencesResponseFrameMaxBytes: int64(len(frame)),
			})
			if got.Failure != "" || got.ServerError != nil || got.Key.ID != 1 || got.Messages != tc.messages || !bytes.Equal(got.Result, tc.result) {
				t.Fatalf("fixture result: failure=%s error=%v key=%+v messages=%d result=%s", got.Failure, got.ServerError, got.Key, got.Messages, got.Result)
			}
			requests, _ := child.snapshot()
			if len(requests) != 1 || !bytes.Equal(requests[0].Params, params) {
				t.Fatalf("fixture request: count=%d params=%q", len(requests), requests[0].Params)
			}
			var members []json.RawMessage
			if err := json.Unmarshal(got.Result, &members); err != nil || len(members) != tc.members || (members == nil) != tc.isNull {
				t.Fatalf("ASSERT_ADR0011_REFERENCES_MEMBER_SHAPE: err=%v members=%d nil=%v", err, len(members), members == nil)
			}
			raw, present := got.CompletedReferencesResponseFrame()
			methodRaw, methodPresent := got.CompletedMethodResponseFrame()
			observed, readPresent := got.CompletedResponseRead()
			if !present || !methodPresent || !readPresent || !bytes.Equal(raw, frame) || !bytes.Equal(methodRaw, frame) || observed.FrameBytes != int64(len(frame)) || observed.FrameSHA256 != fmt.Sprintf("sha256:%x", sha256.Sum256(raw)) {
				t.Fatalf("ASSERT_ADR0011_REFERENCES_ACCEPTED_FRAME: case=%s references=%v method=%v read=%v rawEqual=%v observed=%+v", tc.name, present, methodPresent, readPresent, bytes.Equal(raw, frame), observed)
			}
			if old, ok := got.CompletedDefinitionResponseFrame(); ok || old != nil {
				t.Fatal("ASSERT_ADR0011_REFERENCES_DEFINITION_ACCESSOR_ISOLATED: references reached definition accessor")
			}
			raw[0] ^= 1
			methodRaw[0] ^= 1
			again, ok := got.CompletedReferencesResponseFrame()
			if !ok || !bytes.Equal(again, frame) {
				t.Fatal("ASSERT_ADR0011_REFERENCES_FRAME_COPY: accessor aliased stored bytes")
			}
			encoded, err := json.Marshal(got)
			if err != nil || strings.Contains(string(encoded), "Content-Length:") || strings.Contains(string(encoded), "referencesResponseFrame") {
				t.Fatalf("ASSERT_ADR0011_REFERENCES_PRIVATE_JSON: err=%v encoded=%q", err, encoded)
			}
			t.Log("ASSERT_ADR0011_REFERENCES_ACCEPTED_FRAME: PASS")
		})
	}
}

func TestMethodFrameCaptureReferencesWithheldAndDefinitionParity(t *testing.T) {
	repeatedFrame := definitionFixtureFrame(t, lspwire.Message{JSONRPC: lspwire.Version, ID: json.RawMessage(`1`), Result: referenceResultBody()})
	for _, tc := range []struct {
		name, mode, method string
		cap                int64
		maxMessages        int
		maxBytes           int64
		stale              bool
		failure            session.Failure
	}{
		{"accepted-over-cap", "references-repeated", "textDocument/references", int64(len(repeatedFrame) - 1), 3, 4096, false, ""},
		{"default-off", "references-empty", "textDocument/references", 0, 1, 4096, false, ""},
		{"wrong-method", "references-empty", "textDocument/definition", 4096, 1, 4096, false, ""},
		{"server-error", "server-error", "textDocument/references", 4096, 1, 4096, false, ""},
		{"byte-limit", "references-empty", "textDocument/references", 4096, 1, 1, false, session.ResourceExhausted},
		{"malformed", "malformed", "textDocument/references", 4096, 1, 4096, false, session.SessionPoisoned},
		{"timeout", "hang", "textDocument/references", 4096, 1, 4096, false, session.RequestTimeout},
		{"stale", "references-empty", "textDocument/references", 4096, 1, 4096, true, session.StaleGeneration},
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
				CaptureReferencesResponseFrameMaxBytes: tc.cap,
			})
			if got.Failure != tc.failure {
				t.Fatalf("fixture: case=%s failure=%s want=%s", tc.name, got.Failure, tc.failure)
			}
			if raw, ok := got.CompletedReferencesResponseFrame(); ok || raw != nil {
				t.Fatalf("ASSERT_ADR0011_REFERENCES_WITHHELD: case=%s present=%v raw=%q", tc.name, ok, raw)
			}
			if raw, ok := got.CompletedMethodResponseFrame(); ok || raw != nil {
				t.Fatalf("ASSERT_ADR0011_REFERENCES_METHOD_WITHHELD: case=%s present=%v raw=%q", tc.name, ok, raw)
			}
			if tc.name == "accepted-over-cap" && (got.Messages != 3 || !bytes.Equal(got.Result, referenceResultBody())) {
				t.Fatalf("ASSERT_ADR0011_REFERENCES_UNSELECTED_CONTINUES: messages=%d result=%s", got.Messages, got.Result)
			}
			t.Log("ASSERT_ADR0011_REFERENCES_WITHHELD: PASS")
		})
	}
	// The old definition-only accessor and the new method-neutral accessor must
	// return independent copies of the same previously qualified frame.
	m, started, _ := roundTripManager(t, "success")
	t.Cleanup(func() { _ = m.Shutdown(context.Background()) })
	definition := definitionFixtureResponse(t)
	got := m.RoundTrip(context.Background(), RoundTripRequest{
		SessionID: started.SessionID, Generation: started.Generation, Method: "textDocument/definition",
		Deadline: time.Now().Add(time.Second), MaxMessages: 3, MaxBytes: 4096,
		CaptureDefinitionResponseFrameMaxBytes: int64(len(definition)),
	})
	old, oldOK := got.CompletedDefinitionResponseFrame()
	shared, sharedOK := got.CompletedMethodResponseFrame()
	if !oldOK || !sharedOK || !bytes.Equal(old, definition) || !bytes.Equal(shared, definition) {
		t.Fatal("ASSERT_ADR0011_DEFINITION_CAPTURE_PARITY: definition-only and method-neutral accessors diverged")
	}
	t.Log("ASSERT_ADR0011_DEFINITION_CAPTURE_PARITY: PASS")
}
