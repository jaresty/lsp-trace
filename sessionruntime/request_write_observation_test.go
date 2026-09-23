package sessionruntime

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"testing"
	"time"

	"lsp-trace/internal/lspwire"
	"lsp-trace/internal/session"
)

func TestCompletedRequestWriteBindsActualFrameAndOwner(t *testing.T) {
	m, started, child := roundTripManager(t, "server-error")
	req := RoundTripRequest{SessionID: started.SessionID, Generation: started.Generation, Method: "textDocument/definition", Params: json.RawMessage(`{"textDocument":{"uri":"file:///a.go"},"position":{"line":1,"character":2}}`), Deadline: time.Now().Add(time.Second), MaxMessages: 1, MaxBytes: 4096}
	got := m.RoundTrip(context.Background(), req)
	if got.Failure != "" || got.ServerError == nil {
		t.Fatalf("ASSERT_WRITE_SERVER_ERROR_CONTROL: %+v", got)
	}
	observed, ok := got.CompletedRequestWrite()
	if !ok {
		t.Fatal("ASSERT_WRITE_COMPLETED_FRAME_OWNER: missing after completed writer")
	}
	if observed.SessionID != started.SessionID || observed.Generation != started.Generation || observed.Key != got.Key || observed.Method != req.Method {
		t.Fatalf("ASSERT_WRITE_COMPLETED_FRAME_OWNER: observation=%+v request=%+v", observed, got.Key)
	}
	requests, _ := child.snapshot()
	if len(requests) != 1 || requests[0].Method != req.Method || string(requests[0].Params) != string(req.Params) {
		t.Fatalf("ASSERT_WRITE_WIRE_REQUEST_CONTROL: %+v", requests)
	}
	body, err := json.Marshal(lspwire.Message{JSONRPC: lspwire.Version, ID: json.RawMessage(strconv.FormatUint(got.Key.ID, 10)), Method: req.Method, Params: req.Params})
	if err != nil {
		t.Fatal(err)
	}
	frame := append([]byte(fmt.Sprintf("Content-Length: %d\r\n\r\n", len(body))), body...)
	want := fmt.Sprintf("sha256:%x", sha256.Sum256(frame))
	if observed.FrameBytes != int64(len(frame)) || observed.FrameSHA256 != want {
		t.Fatalf("ASSERT_WRITE_COMPLETED_FRAME_DIGEST: got=(%d,%s) want=(%d,%s)", observed.FrameBytes, observed.FrameSHA256, len(frame), want)
	}
	observed.Method = "substituted"
	again, ok := got.CompletedRequestWrite()
	if !ok || again.Method != req.Method {
		t.Fatalf("ASSERT_WRITE_VALUE_COPY: %+v", again)
	}
	without := got
	without.requestWrite = nil
	encoded, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	baseline, err := json.Marshal(without)
	if err != nil {
		t.Fatal(err)
	}
	if string(encoded) != string(baseline) || strings.Contains(string(encoded), want) {
		t.Fatal("ASSERT_WRITE_JSON_OMITTED: private observation changed serialized result")
	}
}

type shortWritePipe struct{ io.WriteCloser }

func (w shortWritePipe) Write([]byte) (int, error) { return 0, nil }

type shortWriteChild struct{ *roundTripChild }

func (c *shortWriteChild) Stdin() io.WriteCloser { return shortWritePipe{c.roundTripChild.Stdin()} }

func TestCompletedRequestWriteRejectsShortWrite(t *testing.T) {
	child := &shortWriteChild{newRoundTripChild("hang")}
	m, err := New(Config{Limits: Limits{MaxSessions: 1, MaxRequests: 1, MaxChildren: 2, MaxCancels: 2, MaxTombstones: 4, MaxObservations: 64, MaxOperations: 2}, Starter: oneChildStarter{child}})
	if err != nil {
		t.Fatal(err)
	}
	started := m.Start(context.Background(), StartRequest{Profile: profile(t)})
	if ready := m.ObserveInitialization(started.SessionID, started.Generation, true); ready.State != session.Ready {
		t.Fatalf("ASSERT_WRITE_SHORT_READY_CONTROL: %+v", ready)
	}
	got := m.RoundTrip(context.Background(), RoundTripRequest{SessionID: started.SessionID, Generation: started.Generation, Method: "textDocument/definition", Params: json.RawMessage(`{"textDocument":{"uri":"file:///a.go"}}`), Deadline: time.Now().Add(time.Second), MaxMessages: 1, MaxBytes: 4096})
	if got.Failure != session.SessionPoisoned || got.RequestBytes == 0 {
		t.Fatalf("ASSERT_WRITE_SHORT_ATTEMPT_CONTROL: %+v", got)
	}
	if _, ok := got.CompletedRequestWrite(); ok {
		t.Fatal("ASSERT_WRITE_NO_SHORT_WRITE_OBSERVATION: short write cannot be completed")
	}
}

func TestCompletedRequestWriteDistinguishesAttemptAndLaterTimeout(t *testing.T) {
	m, started, _ := roundTripManager(t, "success")
	stale := m.RoundTrip(context.Background(), RoundTripRequest{SessionID: started.SessionID, Generation: started.Generation + 1, Method: "textDocument/references"})
	if stale.Failure != session.StaleGeneration {
		t.Fatalf("ASSERT_WRITE_STALE_CONTROL: %+v", stale)
	}
	if _, ok := stale.CompletedRequestWrite(); ok {
		t.Fatal("ASSERT_WRITE_NO_PRE_ADMISSION_OBSERVATION: stale generation")
	}

	m, started, _ = roundTripManager(t, "success")
	m.wire.MaxBodyBytes = 128
	tooLarge := m.RoundTrip(context.Background(), RoundTripRequest{SessionID: started.SessionID, Generation: started.Generation, Method: "textDocument/definition", Params: json.RawMessage(`{"payload":"` + strings.Repeat("x", 256) + `"}`), Deadline: time.Now().Add(time.Second), MaxMessages: 1, MaxBytes: 4096})
	if tooLarge.Failure == "" || tooLarge.RequestBytes == 0 || tooLarge.RequestMessages != 1 {
		t.Fatalf("ASSERT_WRITE_ATTEMPT_CONTROL: %+v", tooLarge)
	}
	if _, ok := tooLarge.CompletedRequestWrite(); ok {
		t.Fatal("ASSERT_WRITE_NO_FAILED_FRAME_OBSERVATION: attempted bytes are not completed write")
	}

	m, started, _ = roundTripManager(t, "hang")
	timedOut := m.RoundTrip(context.Background(), RoundTripRequest{SessionID: started.SessionID, Generation: started.Generation, Method: "textDocument/references", Params: json.RawMessage(`{"context":{"includeDeclaration":true}}`), Deadline: time.Now().Add(20 * time.Millisecond), MaxMessages: 1, MaxBytes: 4096})
	if timedOut.Failure != session.RequestTimeout {
		t.Fatalf("ASSERT_WRITE_TIMEOUT_CONTROL: %+v", timedOut)
	}
	if observed, ok := timedOut.CompletedRequestWrite(); !ok || observed.FrameBytes <= 0 || observed.FrameSHA256 == "" {
		t.Fatalf("ASSERT_WRITE_COMPLETION_SURVIVES_RESPONSE_TIMEOUT: %+v present=%v", observed, ok)
	}
}
