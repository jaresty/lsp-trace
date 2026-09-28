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

func documentSymbolFrameRequest(started StartResult) RoundTripRequest {
	return RoundTripRequest{SessionID: started.SessionID, Generation: started.Generation, Method: "textDocument/documentSymbol", Params: json.RawMessage(`{"textDocument":{"uri":"file:///fixture/main.go"}}`), Deadline: time.Now().Add(time.Second), MaxMessages: 3, MaxBytes: 4096, CaptureDocumentSymbolRequestFrameMaxBytes: 4096, CaptureDocumentSymbolResponseFrameMaxBytes: 4096}
}

func TestDocumentSymbolFramesRequireOwnedDocument(t *testing.T) {
	m, started, _ := roundTripManager(t, "success")
	t.Cleanup(func() { _ = m.Shutdown(context.Background()) })
	req := documentSymbolFrameRequest(started)
	got := m.RoundTrip(context.Background(), req)
	if got.Failure != "" {
		t.Fatalf("fixture failure=%s", got.Failure)
	}
	if frame, ok := got.CompletedDocumentSymbolRequestFrame(); ok || frame != nil {
		t.Fatal("ASSERT_DOCUMENT_SYMBOL_UNOWNED_REQUEST_WITHHELD")
	}
	if frame, ok := got.CompletedDocumentSymbolResponseFrame(); ok || frame != nil {
		t.Fatal("ASSERT_DOCUMENT_SYMBOL_UNOWNED_RESPONSE_WITHHELD")
	}
}

func TestDocumentSymbolExactFrames(t *testing.T) {
	m, started, child, source := ownedDocumentFixture(t, true)
	t.Cleanup(func() { _ = m.Shutdown(context.Background()) })
	req := documentSymbolOwnedRequest(started, source)
	req.CaptureDocumentSymbolRequestFrameMaxBytes = 4096
	req.CaptureDocumentSymbolResponseFrameMaxBytes = 4096
	got := m.RoundTrip(context.Background(), req)
	if got.Failure != "" || got.ServerError != nil || got.Key.ID != 1 {
		t.Fatalf("fixture: failure=%s server=%v key=%+v", got.Failure, got.ServerError, got.Key)
	}
	requests, _ := child.snapshot()
	found := false
	for _, request := range requests {
		if request.Method == req.Method && bytes.Equal(request.Params, req.Params) {
			found = true
		}
	}
	if !found {
		t.Fatal("fixture query mismatch")
	}
	expectedRequest := expectedMethodRequestFrame(t, req, got.Key.ID)
	expectedResponse := definitionFixtureFrame(t, lspwire.Message{JSONRPC: lspwire.Version, ID: json.RawMessage(`1`), Result: got.Result})
	request, requestOK := got.CompletedDocumentSymbolRequestFrame()
	response, responseOK := got.CompletedDocumentSymbolResponseFrame()
	write, writeOK := got.CompletedRequestWrite()
	read, readOK := got.CompletedResponseRead()
	if !requestOK || !responseOK || !writeOK || !readOK || !bytes.Equal(request, expectedRequest) || !bytes.Equal(response, expectedResponse) || write.FrameBytes != int64(len(request)) || read.FrameBytes != int64(len(response)) || write.FrameSHA256 != fmt.Sprintf("sha256:%x", sha256.Sum256(request)) || read.FrameSHA256 != fmt.Sprintf("sha256:%x", sha256.Sum256(response)) {
		t.Fatalf("ASSERT_DOCUMENT_SYMBOL_EXACT_OWNER_FRAMES: request=%v response=%v write=%+v read=%+v", requestOK, responseOK, write, read)
	}
	request[0] ^= 1
	response[0] ^= 1
	againRequest, _ := got.CompletedDocumentSymbolRequestFrame()
	againResponse, _ := got.CompletedDocumentSymbolResponseFrame()
	if !bytes.Equal(againRequest, expectedRequest) || !bytes.Equal(againResponse, expectedResponse) {
		t.Fatal("ASSERT_DOCUMENT_SYMBOL_DEFENSIVE_COPY")
	}
	if old, ok := got.CompletedMethodRequestFrame(); ok || old != nil {
		t.Fatal("ASSERT_DOCUMENT_SYMBOL_DR_ISOLATION_REQUEST")
	}
	if old, ok := got.CompletedMethodResponseFrame(); ok || old != nil {
		t.Fatal("ASSERT_DOCUMENT_SYMBOL_DR_ISOLATION_RESPONSE")
	}
	encoded, err := json.Marshal(got)
	if err != nil || strings.Contains(string(encoded), "Content-Length:") || strings.Contains(string(encoded), "documentSymbolRequestFrame") {
		t.Fatalf("ASSERT_DOCUMENT_SYMBOL_PRIVATE: %v %s", err, encoded)
	}
	t.Log("ASSERT_DOCUMENT_SYMBOL_EXACT_OWNER_FRAMES: PASS")
}

func TestDocumentSymbolFramesFailClosed(t *testing.T) {
	for _, tc := range []struct {
		name, mode string
		change     func(*RoundTripRequest)
		failure    session.Failure
	}{
		{"disabled", "references-empty", func(r *RoundTripRequest) {
			r.CaptureDocumentSymbolRequestFrameMaxBytes = 0
			r.CaptureDocumentSymbolResponseFrameMaxBytes = 0
		}, ""},
		{"request-overcap", "references-empty", func(r *RoundTripRequest) { r.CaptureDocumentSymbolRequestFrameMaxBytes = 1 }, ""},
		{"response-overcap", "references-empty", func(r *RoundTripRequest) { r.CaptureDocumentSymbolResponseFrameMaxBytes = 1 }, ""},
		{"wrong-response-key", "unmatched-only", func(r *RoundTripRequest) { r.Deadline = time.Now().Add(50 * time.Millisecond) }, session.RequestTimeout},
		{"server-error", "server-error", func(*RoundTripRequest) {}, ""},
		{"generation-mismatch", "references-empty", func(r *RoundTripRequest) { r.Generation++ }, session.StaleGeneration},
		{"body-budget", "references-empty", func(r *RoundTripRequest) { r.MaxBytes = 1 }, session.ResourceExhausted},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m, started, _ := roundTripManager(t, tc.mode)
			t.Cleanup(func() { _ = m.Shutdown(context.Background()) })
			req := documentSymbolFrameRequest(started)
			tc.change(&req)
			got := m.RoundTrip(context.Background(), req)
			if got.Failure != tc.failure {
				t.Fatalf("fixture failure=%s want=%s", got.Failure, tc.failure)
			}
			request, requestOK := got.CompletedDocumentSymbolRequestFrame()
			response, responseOK := got.CompletedDocumentSymbolResponseFrame()
			if requestOK || responseOK || request != nil || response != nil {
				t.Fatalf("ASSERT_DOCUMENT_SYMBOL_FAIL_CLOSED: request=%v response=%v", requestOK, responseOK)
			}
			t.Log("ASSERT_DOCUMENT_SYMBOL_FAIL_CLOSED: PASS")
		})
	}
}

func TestDocumentSymbolPartialWriteWithheld(t *testing.T) {
	child := &shortWriteChild{newRoundTripChild("hang")}
	m, err := New(Config{Limits: Limits{MaxSessions: 1, MaxRequests: 1, MaxChildren: 2, MaxCancels: 2, MaxTombstones: 4, MaxObservations: 64, MaxOperations: 2}, Starter: oneChildStarter{child}})
	if err != nil {
		t.Fatal(err)
	}
	started := m.Start(context.Background(), StartRequest{Profile: profile(t)})
	if ready := m.ObserveInitialization(started.SessionID, started.Generation, true); ready.State != session.Ready {
		t.Fatalf("fixture: %+v", ready)
	}
	got := m.RoundTrip(context.Background(), documentSymbolFrameRequest(started))
	if got.Failure != session.SessionPoisoned {
		t.Fatalf("fixture: %s", got.Failure)
	}
	if _, ok := got.CompletedRequestWrite(); ok {
		t.Fatal("ASSERT_DOCUMENT_SYMBOL_PARTIAL_WRITE_OBSERVATION")
	}
	if _, ok := got.CompletedDocumentSymbolRequestFrame(); ok {
		t.Fatal("ASSERT_DOCUMENT_SYMBOL_PARTIAL_WRITE_REQUEST")
	}
	if _, ok := got.CompletedDocumentSymbolResponseFrame(); ok {
		t.Fatal("ASSERT_DOCUMENT_SYMBOL_PARTIAL_WRITE_RESPONSE")
	}
	t.Log("ASSERT_DOCUMENT_SYMBOL_PARTIAL_WRITE_WITHHELD: PASS")
}
