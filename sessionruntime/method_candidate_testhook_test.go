package sessionruntime

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"
	"time"

	"lsp-trace/internal/session"
)

func methodCandidateManager(t *testing.T, mode string, hook func(methodCandidateObservation)) (*Manager, StartResult) {
	t.Helper()
	child := newRoundTripChild(mode)
	m, err := New(Config{
		Limits:  Limits{MaxSessions: 1, MaxRequests: 1, MaxChildren: 2, MaxCancels: 2, MaxTombstones: 4, MaxObservations: 64, MaxOperations: 2},
		Starter: oneChildStarter{child}, methodCandidateTestHook: hook,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = m.Shutdown(context.Background()) })
	started := m.Start(context.Background(), StartRequest{Profile: profile(t)})
	if ready := m.ObserveInitialization(started.SessionID, started.Generation, true); ready.State != session.Ready {
		t.Fatalf("fixture not READY: %+v", ready)
	}
	return m, started
}

func methodCandidateRequest(started StartResult, method string, messages int) RoundTripRequest {
	return RoundTripRequest{
		SessionID: started.SessionID, Generation: started.Generation, Method: method,
		Params:   json.RawMessage(`{"textDocument":{"uri":"file:///fixture/main.go"},"position":{"line":1,"character":2},"context":{"includeDeclaration":false}}`),
		Deadline: time.Now().Add(time.Second), MaxMessages: messages, MaxBytes: 4096,
		CaptureMethodRequestFrameMaxBytes:      4096,
		CaptureDefinitionResponseFrameMaxBytes: 4096,
		CaptureReferencesResponseFrameMaxBytes: 4096,
	}
}

func TestManagerOnlyEmitsCandidateAfterDRSuccess(t *testing.T) {
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
			var captured []methodCandidateObservation
			m, started := methodCandidateManager(t, tc.mode, func(c methodCandidateObservation) { captured = append(captured, c) })
			req := methodCandidateRequest(started, tc.method, tc.messages)
			got := m.RoundTrip(context.Background(), req)
			if got.Failure != "" || got.ServerError != nil {
				t.Fatalf("fixture failure=%s error=%v", got.Failure, got.ServerError)
			}
			if len(captured) != 1 {
				t.Fatalf("ASSERT_ADR0011_OWNER_CANDIDATE_AFTER_SUCCESS: case=%s callbacks=%d", tc.name, len(captured))
			}
			c := captured[0]
			request, requestOK := got.CompletedMethodRequestFrame()
			response, responseOK := got.CompletedMethodResponseFrame()
			write, writeOK := got.CompletedRequestWrite()
			read, readOK := got.CompletedResponseRead()
			if !requestOK || !responseOK || !writeOK || !readOK || c.SessionID != req.SessionID || c.Generation != req.Generation || c.Method != req.Method || c.Key != got.Key || c.RequestWrite != write || c.ResponseRead != read || !bytes.Equal(c.RequestFrame, request) || !bytes.Equal(c.ResponseFrame, response) {
				t.Fatalf("ASSERT_ADR0011_OWNER_CANDIDATE_BINDINGS: case=%s request=%v response=%v write=%v read=%v candidate=%+v", tc.name, requestOK, responseOK, writeOK, readOK, c.Key)
			}
			if err := VerifyMethodFrameCorrespondence(req, got); err != nil {
				t.Fatalf("ASSERT_ADR0011_OWNER_CANDIDATE_LOCAL_BYTES: %v", err)
			}
			c.RequestFrame[0] ^= 1
			c.ResponseFrame[0] ^= 1
			restoredRequest, _ := got.CompletedMethodRequestFrame()
			restoredResponse, _ := got.CompletedMethodResponseFrame()
			if !bytes.Equal(restoredRequest, request) || !bytes.Equal(restoredResponse, response) {
				t.Fatal("ASSERT_ADR0011_OWNER_CANDIDATE_COPY: hook modified returned result")
			}
			t.Log("ASSERT_ADR0011_OWNER_CANDIDATE_AFTER_SUCCESS: PASS")
		})
	}
}

func TestManagerCandidateWithheldWithoutSelectedSuccess(t *testing.T) {
	for _, tc := range []struct {
		name, mode, method string
		messages           int
		change             func(*RoundTripRequest)
		failure            session.Failure
	}{
		{"no-request-cap", "references-empty", "textDocument/references", 1, func(r *RoundTripRequest) { r.CaptureMethodRequestFrameMaxBytes = 0 }, ""},
		{"no-response-cap", "references-empty", "textDocument/references", 1, func(r *RoundTripRequest) { r.CaptureReferencesResponseFrameMaxBytes = 0 }, ""},
		{"other-method", "references-empty", "textDocument/hover", 1, func(*RoundTripRequest) {}, ""},
		{"request-over-cap", "references-empty", "textDocument/references", 1, func(r *RoundTripRequest) { r.CaptureMethodRequestFrameMaxBytes = 1 }, ""},
		{"response-over-cap", "references-repeated", "textDocument/references", 3, func(r *RoundTripRequest) { r.CaptureReferencesResponseFrameMaxBytes = 1 }, ""},
		{"server-error", "server-error", "textDocument/references", 1, func(*RoundTripRequest) {}, ""},
		{"unmatched-only", "unmatched-only", "textDocument/references", 2, func(r *RoundTripRequest) { r.Deadline = time.Now().Add(50 * time.Millisecond) }, session.RequestTimeout},
		{"timeout", "hang", "textDocument/references", 1, func(r *RoundTripRequest) { r.Deadline = time.Now().Add(50 * time.Millisecond) }, session.RequestTimeout},
		{"stale", "references-empty", "textDocument/references", 1, func(r *RoundTripRequest) { r.Generation++ }, session.StaleGeneration},
		{"malformed", "malformed", "textDocument/references", 1, func(*RoundTripRequest) {}, session.SessionPoisoned},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var captured []methodCandidateObservation
			m, started := methodCandidateManager(t, tc.mode, func(c methodCandidateObservation) { captured = append(captured, c) })
			req := methodCandidateRequest(started, tc.method, tc.messages)
			tc.change(&req)
			got := m.RoundTrip(context.Background(), req)
			if got.Failure != tc.failure {
				t.Fatalf("fixture: failure=%s want=%s", got.Failure, tc.failure)
			}
			if len(captured) != 0 {
				t.Fatalf("ASSERT_ADR0011_OWNER_CANDIDATE_WITHHELD: case=%s callbacks=%d", tc.name, len(captured))
			}
			t.Log("ASSERT_ADR0011_OWNER_CANDIDATE_WITHHELD: PASS")
		})
	}
}

func TestManagerCandidateShortWriteDoesNotInvokeHook(t *testing.T) {
	child := &shortWriteChild{newRoundTripChild("hang")}
	calls := 0
	m, err := New(Config{
		Limits:  Limits{MaxSessions: 1, MaxRequests: 1, MaxChildren: 2, MaxCancels: 2, MaxTombstones: 4, MaxObservations: 64, MaxOperations: 2},
		Starter: oneChildStarter{child}, methodCandidateTestHook: func(methodCandidateObservation) { calls++ },
	})
	if err != nil {
		t.Fatal(err)
	}
	started := m.Start(context.Background(), StartRequest{Profile: profile(t)})
	if ready := m.ObserveInitialization(started.SessionID, started.Generation, true); ready.State != session.Ready {
		t.Fatalf("fixture not READY: %+v", ready)
	}
	got := m.RoundTrip(context.Background(), methodCandidateRequest(started, "textDocument/references", 1))
	if got.Failure != session.SessionPoisoned || calls != 0 {
		t.Fatalf("ASSERT_ADR0011_OWNER_CANDIDATE_SHORT_WRITE: failure=%s calls=%d", got.Failure, calls)
	}
	t.Log("ASSERT_ADR0011_OWNER_CANDIDATE_SHORT_WRITE: PASS")
}
