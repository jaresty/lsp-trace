package sessionruntime

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"lsp-trace/internal/lspwire"
)

func methodFrameTransaction(t *testing.T, mode, method string, maxMessages int) (RoundTripRequest, RoundTripResult) {
	t.Helper()
	m, started, _ := roundTripManager(t, mode)
	t.Cleanup(func() { _ = m.Shutdown(context.Background()) })
	req := RoundTripRequest{
		SessionID: started.SessionID, Generation: started.Generation, Method: method,
		Params:   json.RawMessage(`{"textDocument":{"uri":"file:///fixture/main.go"},"position":{"line":1,"character":2},"context":{"includeDeclaration":false}}`),
		Deadline: time.Now().Add(time.Second), MaxMessages: maxMessages, MaxBytes: 4096,
	}
	if method == "textDocument/definition" {
		req.CaptureDefinitionResponseFrameMaxBytes = 4096
	} else {
		req.CaptureReferencesResponseFrameMaxBytes = 4096
	}
	got := m.RoundTrip(context.Background(), req)
	if got.Failure != "" || got.ServerError != nil {
		t.Fatalf("fixture failed: failure=%s serverError=%v", got.Failure, got.ServerError)
	}
	if raw, ok := got.CompletedMethodResponseFrame(); !ok || len(raw) == 0 {
		t.Fatalf("fixture missing captured method frame: %v", ok)
	}
	return req, got
}

func TestMethodFrameCorrespondenceAcceptsLocalDefinitionAndReferences(t *testing.T) {
	for _, tc := range []struct {
		name, mode, method string
		messages           int
	}{
		{"definition", "success", "textDocument/definition", 3},
		{"references-empty", "references-empty", "textDocument/references", 1},
		{"references-null", "references-null", "textDocument/references", 1},
		{"references-repeated", "references-repeated", "textDocument/references", 3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req, got := methodFrameTransaction(t, tc.mode, tc.method, tc.messages)
			if err := VerifyMethodFrameCorrespondence(req, got); err != nil {
				t.Fatalf("ASSERT_ADR0011_DR_BYTE_CORRESPONDENCE: %v", err)
			}
			t.Log("ASSERT_ADR0011_DR_BYTE_CORRESPONDENCE: PASS")
		})
	}
}

func TestMethodFrameCorrespondenceRejectsDecodedResultSubstitution(t *testing.T) {
	req, got := methodFrameTransaction(t, "references-empty", "textDocument/references", 1)
	got.Result = json.RawMessage(`[{"uri":"file:///injected.go"}]`)
	if err := VerifyMethodFrameCorrespondence(req, got); !errors.Is(err, ErrMethodFrameCorrespondence) {
		t.Fatalf("ASSERT_ADR0011_DR_RESULT_SUBSTITUTION: expected fixed rejection, got %v", err)
	}
	t.Log("ASSERT_ADR0011_DR_RESULT_SUBSTITUTION: PASS")
}

func TestMethodFrameCorrespondenceRejectsIsolatedMismatches(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*RoundTripRequest, *RoundTripResult)
	}{
		{"missing-response-read", func(_ *RoundTripRequest, r *RoundTripResult) { r.responseRead = nil }},
		{"read-digest", func(_ *RoundTripRequest, r *RoundTripResult) { r.responseRead.FrameSHA256 = "sha256:changed" }},
		{"read-length", func(_ *RoundTripRequest, r *RoundTripResult) { r.responseRead.FrameBytes++ }},
		{"missing-request-write", func(_ *RoundTripRequest, r *RoundTripResult) { r.requestWrite = nil }},
		{"write-digest", func(_ *RoundTripRequest, r *RoundTripResult) { r.requestWrite.FrameSHA256 = "sha256:changed" }},
		{"write-length", func(_ *RoundTripRequest, r *RoundTripResult) { r.requestWrite.FrameBytes++ }},
		{"write-body-length", func(_ *RoundTripRequest, r *RoundTripResult) { r.RequestBytes++ }},
		{"write-method", func(_ *RoundTripRequest, r *RoundTripResult) { r.requestWrite.Method = "textDocument/references" }},
		{"wrong-key", func(_ *RoundTripRequest, r *RoundTripResult) { r.Key.ID++ }},
		{"wrong-session", func(_ *RoundTripRequest, r *RoundTripResult) { r.responseRead.SessionID = "other-session" }},
		{"wrong-generation", func(q *RoundTripRequest, _ *RoundTripResult) { q.Generation++ }},
		{"changed-request", func(q *RoundTripRequest, _ *RoundTripResult) { q.Params = json.RawMessage(`{"changed":true}`) }},
		{"wrong-request-cap", func(q *RoundTripRequest, _ *RoundTripResult) { q.CaptureDefinitionResponseFrameMaxBytes = 0 }},
		{"zero-message-bound", func(q *RoundTripRequest, _ *RoundTripResult) { q.MaxMessages = 0 }},
		{"zero-byte-bound", func(q *RoundTripRequest, _ *RoundTripResult) { q.MaxBytes = 0 }},
		{"too-many-messages", func(q *RoundTripRequest, _ *RoundTripResult) { q.MaxMessages = 1 }},
		{"too-few-bytes", func(q *RoundTripRequest, _ *RoundTripResult) { q.MaxBytes = 1 }},
		{"over-verification-ceiling", func(q *RoundTripRequest, _ *RoundTripResult) {
			q.CaptureDefinitionResponseFrameMaxBytes = (2 << 20) + 1
		}},
		{"server-error", func(_ *RoundTripRequest, r *RoundTripResult) {
			r.ServerError = &lspwire.RPCError{Code: -32603, Message: "private-secret"}
		}},
		{"changed-frame", func(_ *RoundTripRequest, r *RoundTripResult) { r.definitionResponseFrame[0] ^= 1 }},
		{"trailing-frame", func(q *RoundTripRequest, r *RoundTripResult) {
			frame := append(append([]byte(nil), r.definitionResponseFrame...), r.definitionResponseFrame...)
			r.definitionResponseFrame = frame
			q.CaptureDefinitionResponseFrameMaxBytes = int64(len(frame))
			r.responseRead.FrameBytes = int64(len(frame))
			r.responseRead.FrameSHA256 = methodFrameHash(frame)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req, got := methodFrameTransaction(t, "success", "textDocument/definition", 3)
			// Mutate only this test case; do not change the manager's stored pointers.
			if got.requestWrite != nil {
				copyWrite := *got.requestWrite
				got.requestWrite = &copyWrite
			}
			if got.responseRead != nil {
				copyRead := *got.responseRead
				got.responseRead = &copyRead
			}
			got.definitionResponseFrame = append([]byte(nil), got.definitionResponseFrame...)
			tc.change(&req, &got)
			err := VerifyMethodFrameCorrespondence(req, got)
			if !errors.Is(err, ErrMethodFrameCorrespondence) || err.Error() != "method frame correspondence failed" || strings.Contains(err.Error(), "private-secret") || strings.Contains(err.Error(), "file:///") {
				t.Fatalf("ASSERT_ADR0011_DR_MISMATCH_REJECTED: case=%s err=%v", tc.name, err)
			}
			t.Log("ASSERT_ADR0011_DR_MISMATCH_REJECTED: PASS")
		})
	}
}

func methodFrameHash(frame []byte) string {
	return fmt.Sprintf("sha256:%x", sha256.Sum256(frame))
}

func TestMethodFrameCorrespondenceDoesNotAuthenticateReplay(t *testing.T) {
	req, got := methodFrameTransaction(t, "success", "textDocument/definition", 3)
	// Same-size, internally consistent replacement. A caller can replace every
	// local observation; byte agreement alone cannot identify its producer.
	otherResult := json.RawMessage(`{"ok":null}`)
	frame := definitionFixtureFrame(t, lspwire.Message{JSONRPC: lspwire.Version, ID: json.RawMessage(`1`), Result: otherResult})
	if len(frame) != len(got.definitionResponseFrame) {
		t.Fatal("fixture replacement must preserve length")
	}
	got.definitionResponseFrame = append([]byte(nil), frame...)
	got.Result = append(json.RawMessage(nil), otherResult...)
	copyRead := *got.responseRead
	copyRead.FrameSHA256 = methodFrameHash(frame)
	got.responseRead = &copyRead
	if err := VerifyMethodFrameCorrespondence(req, got); err != nil {
		t.Fatalf("ASSERT_ADR0011_NO_AUTHENTICATION_ONLY_LOCAL: replay consistency=%v", err)
	}
	if bytes.Equal(got.Result, json.RawMessage(`{"ok":true}`)) {
		t.Fatal("fixture did not replace decoded result")
	}
	t.Log("ASSERT_ADR0011_NO_AUTHENTICATION_ONLY_LOCAL: PASS (self-consistent replacement remains possible)")
}
