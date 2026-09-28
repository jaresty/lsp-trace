package adr0011acquisition

import (
	"bytes"
	"fmt"
	"testing"
)

func spanFrame(body string) []byte {
	return []byte(fmt.Sprintf("Content-Length: %d\r\n\r\n%s", len(body), body))
}

func TestPrivateExactSpans(t *testing.T) {
	for _, method := range []string{"textDocument/documentSymbol", "textDocument/references"} {
		params := ` { "textDocument" : {"uri":"file:///x"} } `
		req := spanFrame(`{"jsonrpc":"2.0","id":7,"method":"` + method + `","params":` + params + `}`)
		resp := spanFrame(`{"jsonrpc":"2.0","id":7,"result": [] }`)
		p, ok := exactOwnedSpan(req, "params", method, 7)
		if !ok || !bytes.Equal(req[p.Offset:p.Offset+p.Length], []byte(`{ "textDocument" : {"uri":"file:///x"} }`)) {
			t.Fatal("ASSERT_PRIVATE_PARAMS_EXACT", method)
		}
		r, ok := exactOwnedSpan(resp, "result", method, 7)
		if !ok || !bytes.Equal(resp[r.Offset:r.Offset+r.Length], []byte(`[]`)) {
			t.Fatal("ASSERT_PRIVATE_RESULT_EXACT", method)
		}
		if !replayOwnedSpan(req, p, "params", method, 7) || !replayOwnedSpan(resp, r, "result", method, 7) {
			t.Fatal("ASSERT_PRIVATE_SPAN_REPLAY", method)
		}
		p.Offset++
		if replayOwnedSpan(req, p, "params", method, 7) {
			t.Fatal("ASSERT_PRIVATE_ALTERED_OFFSET_REJECT")
		}
		r.Length++
		if replayOwnedSpan(resp, r, "result", method, 7) {
			t.Fatal("ASSERT_PRIVATE_ALTERED_LENGTH_REJECT")
		}
	}
}

func TestPrivateExactSpanRejections(t *testing.T) {
	method := "textDocument/references"
	for _, body := range []string{
		`{"jsonrpc":"2.0","id":7,"method":"textDocument/references","params":{},"params":[]}`,
		`{"jsonrpc":"2.0","id":7,"method":"textDocument/references","params":{},"extra":1}`,
		`{"jsonrpc":"2.0","id":8,"method":"textDocument/references","params":{}}`,
		`{"jsonrpc":"2.0","id":7,"method":"textDocument/documentSymbol","params":{}}`,
		`{"jsonrpc":"2.0","id":7,"method":"textDocument/references","params":{}} {}`,
	} {
		if _, ok := exactOwnedSpan(spanFrame(body), "params", method, 7); ok {
			t.Fatal("ASSERT_PRIVATE_MALFORMED_REJECT")
		}
	}
	for _, body := range []string{`{"jsonrpc":"2.0","id":7,"result":null,"result":[]}`, `{"jsonrpc":"2.0","id":7,"result":[],"extra":0}`, `{"jsonrpc":"2.0","id":7,"result":[],"error":null}`} {
		if _, ok := exactOwnedSpan(spanFrame(body), "result", method, 7); ok {
			t.Fatal("ASSERT_PRIVATE_RESPONSE_CLOSED_REJECT")
		}
	}
	frame := spanFrame(`{"jsonrpc":"2.0","id":7,"result":null}`)
	if _, ok := exactOwnedSpan(append(frame, 'x'), "result", method, 7); ok {
		t.Fatal("ASSERT_PRIVATE_EXTRA_FRAME_REJECT")
	}
	if _, ok := exactOwnedSpan(bytes.Repeat([]byte("x"), 1<<20+1), "result", method, 7); ok {
		t.Fatal("ASSERT_PRIVATE_OVERCAP_REJECT")
	}
	n, ok := exactOwnedSpan(frame, "result", method, 7)
	if !ok || string(frame[n.Offset:n.Offset+n.Length]) != "null" || replayOwnedSpan(spanFrame(`{"jsonrpc":"2.0","id":7,"result":[]}`), n, "result", method, 7) {
		t.Fatal("ASSERT_PRIVATE_NULL_NOT_EMPTY")
	}
}
