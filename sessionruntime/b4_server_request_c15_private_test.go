package sessionruntime

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"testing"

	"lsp-trace/internal/lspwire"
	"lsp-trace/internal/session"
)

type privateB4FailWriter struct {
	written  int
	limit    int
	flushErr error
}

func (w *privateB4FailWriter) Write(p []byte) (int, error) {
	remaining := w.limit - w.written
	if remaining <= 0 {
		return 0, io.ErrClosedPipe
	}
	if len(p) > remaining {
		w.written += remaining
		return remaining, io.ErrClosedPipe
	}
	w.written += len(p)
	return len(p), nil
}

func (w *privateB4FailWriter) Flush() error { return w.flushErr }

func TestPrivateB4ServerRequestResponseExactAccountingAndAtomicRefusal(t *testing.T) {
	account, failure := newPrivateB4ByteAccountV2()
	if failure != "" {
		t.Fatal(failure)
	}
	requestBody := []byte(`{"jsonrpc":"2.0","id":"exact-id","method":"workspace/unknown"}`)
	header := []byte(fmt.Sprintf("Content-Length: %d\r\n\r\n", len(requestBody)))
	frame := append(append([]byte(nil), header...), requestBody...)
	idStart := bytes.Index(requestBody, []byte(`"exact-id"`))
	observation := lspwire.SuccessorReadObservation{BodyOffset: uint64(len(header)), RequestIDStart: uint64(idStart), RequestIDEnd: uint64(idStart + len(`"exact-id"`))}
	expectedBody := []byte(`{"jsonrpc":"2.0","id":"exact-id","error":{"code":-32601,"message":"Method not found"}}`)
	expectedFrame := append([]byte(fmt.Sprintf("Content-Length: %d\r\n\r\n", len(expectedBody))), expectedBody...)

	before := account.snapshot()
	var output bytes.Buffer
	writer := lspwire.NewWriter(&output, lspwire.DefaultLimits())
	if got, poison := writePrivateB4UnsupportedResponse(context.Background(), &ownedTransport{}, writer, account, frame, observation); got != "" || poison {
		t.Fatalf("ASSERT_C15_SERVER_REQUEST_WRITE failure=%q poison=%v", got, poison)
	}
	if !bytes.Equal(output.Bytes(), expectedFrame) {
		t.Fatalf("ASSERT_C15_SERVER_REQUEST_EXACT_FRAME got=%q want=%q", output.Bytes(), expectedFrame)
	}
	after := account.snapshot()
	required := uint64(len(expectedFrame))
	if after.Live != before.Live || after.Cumulative != before.Cumulative+required {
		t.Fatalf("ASSERT_C15_SERVER_REQUEST_EXACT_ACCOUNTING before=%+v after=%+v required=%d", before, after, required)
	}

	headerLength := len(expectedFrame) - len(expectedBody)
	for _, tc := range []struct {
		name     string
		limit    int
		flushErr error
	}{
		{name: "header", limit: 5},
		{name: "body", limit: headerLength + 5},
		{name: "flush", limit: len(expectedFrame), flushErr: io.ErrClosedPipe},
	} {
		t.Run(tc.name, func(t *testing.T) {
			failing := &privateB4FailWriter{limit: tc.limit, flushErr: tc.flushErr}
			if got, poison := writePrivateB4UnsupportedResponse(context.Background(), &ownedTransport{}, lspwire.NewWriter(failing, lspwire.DefaultLimits()), account, frame, observation); got != session.SessionPoisoned || !poison {
				t.Fatalf("ASSERT_C15_SERVER_REQUEST_WRITE_FAILURE failure=%q poison=%v", got, poison)
			}
			if failing.written == 0 || account.snapshot().Live != before.Live {
				t.Fatalf("ASSERT_C15_SERVER_REQUEST_WRITE_FAILURE_RELEASE written=%d snapshot=%+v", failing.written, account.snapshot())
			}
		})
	}

	for _, malformed := range []lspwire.SuccessorReadObservation{
		{BodyOffset: ^uint64(0), RequestIDStart: 1, RequestIDEnd: 2},
		{BodyOffset: 1, RequestIDStart: ^uint64(0), RequestIDEnd: 1},
		{BodyOffset: 1, RequestIDStart: 2, RequestIDEnd: ^uint64(0)},
	} {
		var malformedOutput bytes.Buffer
		if got, poison := writePrivateB4UnsupportedResponse(context.Background(), &ownedTransport{}, lspwire.NewWriter(&malformedOutput, lspwire.DefaultLimits()), account, frame, malformed); got != session.SessionPoisoned || !poison || malformedOutput.Len() != 0 || account.snapshot().Live != before.Live {
			t.Fatalf("ASSERT_C15_SERVER_REQUEST_MALFORMED_SPAN failure=%q poison=%v bytes=%d snapshot=%+v", got, poison, malformedOutput.Len(), account.snapshot())
		}
	}

	fillerCapacity := privateB4MaxOwnedBytes - after.Live - required + 1
	filler, fillFailure := account.reserve(fillerCapacity)
	if fillFailure != "" {
		t.Fatal(fillFailure)
	}
	defer filler.release()
	full := account.snapshot()
	output.Reset()
	if got, poison := writePrivateB4UnsupportedResponse(context.Background(), &ownedTransport{}, writer, account, frame, observation); got != session.ResourceExhausted || poison {
		t.Fatalf("ASSERT_C15_SERVER_REQUEST_PLUS_ONE failure=%q poison=%v", got, poison)
	}
	if output.Len() != 0 || account.snapshot() != full {
		t.Fatalf("ASSERT_C15_SERVER_REQUEST_REFUSAL_ATOMIC bytes=%d before=%+v after=%+v", output.Len(), full, account.snapshot())
	}
}

func TestPrivateB4ServerRequestsReceiveCanonicalMethodNotFoundAndDoNotPublish(t *testing.T) {
	requests := []lspwire.Message{
		{JSONRPC: lspwire.Version, ID: json.RawMessage(`"request-id"`), Method: "workspace/unknown", Params: json.RawMessage(`{"x":1}`)},
		{JSONRPC: lspwire.Version, ID: json.RawMessage(`17`), Method: "workspace/unknown"},
		{JSONRPC: lspwire.Version, ID: json.RawMessage(`17`), Method: "workspace/unknown"},
	}
	f := newFullP1FixtureWithResponses(t, requests)
	f.req.MaxMessages = len(requests) + 1
	result, _ := f.m.RoundTripPrivateB4(context.Background(), f.req, f.owner)
	if result.Failure != "" || result.ServerError != nil {
		t.Fatalf("ASSERT_C15_SERVER_REQUEST_TRANSACTION failure=%q error=%+v", result.Failure, result.ServerError)
	}
	if len(result.Notifications) != 0 || len(result.Responses) != 0 {
		t.Fatalf("ASSERT_C15_SERVER_REQUEST_NOT_PUBLISHED notifications=%d responses=%d", len(result.Notifications), len(result.Responses))
	}
	f.child.mu.Lock()
	replies := append([]lspwire.Message(nil), f.child.serverReplies...)
	f.child.mu.Unlock()
	if len(replies) != len(requests) {
		t.Fatalf("ASSERT_C15_SERVER_REQUEST_ONE_REPLY_PER_OCCURRENCE got=%d", len(replies))
	}
	for i := range replies {
		if string(replies[i].ID) != string(requests[i].ID) || replies[i].JSONRPC != lspwire.Version || replies[i].Error == nil || replies[i].Error.Code != -32601 || replies[i].Error.Message != "Method not found" || replies[i].Error.Data != nil || replies[i].Result != nil || replies[i].Method != "" {
			t.Fatalf("ASSERT_C15_SERVER_REQUEST_CANONICAL_REPLY_%d got=%+v", i, replies[i])
		}
	}
}
