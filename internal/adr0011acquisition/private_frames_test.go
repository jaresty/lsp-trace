package adr0011acquisition

import (
	"bytes"
	"fmt"
	"testing"
)

func TestPrivateFrameExactBodyRejectsPartialAndOvercap(t *testing.T) {
	body := []byte(`{"jsonrpc":"2.0","id":1,"result":null}`)
	frame := []byte(fmt.Sprintf("Content-Length: %d\r\n\r\n%s", len(body), body))
	if got, ok := exactFrameBody(frame); !ok || !bytes.Equal(got, body) {
		t.Fatal("ASSERT_PRIVATE_FRAME_EXACT_PASS")
	}
	for _, bad := range [][]byte{frame[:len(frame)-1], append(append([]byte(nil), frame...), byte('x')), []byte("Content-Length: 2\r\n\r\n{}"), bytes.Repeat([]byte("x"), 1<<20+1)} {
		if _, ok := exactFrameBody(bad); ok {
			t.Fatal("ASSERT_PRIVATE_FRAME_PARTIAL_OVERCAP_REJECT")
		}
	}
}

// Padding is outside the raw result token; it exercises the frame allocation
// without relaxing the independent result-token ceiling.
func TestPrivateFrameSelectedBoundary(t *testing.T) {
	prefix := []byte(`{"jsonrpc":"2.0","id":31,"result":null`)
	for _, frameSize := range []int{1<<20 + 1, 2 << 20, 2<<20 + 1} {
		bodySize := frameSize - len(fmt.Sprintf("Content-Length: %d\r\n\r\n", frameSize))
		for {
			headerSize := len(fmt.Sprintf("Content-Length: %d\r\n\r\n", bodySize))
			if bodySize+headerSize == frameSize {
				break
			}
			bodySize = frameSize - headerSize
		}
		body := append(append([]byte(nil), prefix...), bytes.Repeat([]byte(" "), bodySize-len(prefix)-1)...)
		body = append(body, '}')
		frame := []byte(fmt.Sprintf("Content-Length: %d\r\n\r\n", len(body)))
		frame = append(frame, body...)
		if len(frame) != frameSize {
			t.Fatalf("fixture frame length %d != %d", len(frame), frameSize)
		}
		_, accepted := exactFrameBody(frame)
		if accepted != (frameSize <= 2<<20) {
			t.Fatalf("ASSERT_PRIVATE_FRAME_SELECTED_BOUNDARY size=%d accepted=%t", frameSize, accepted)
		}
		if accepted {
			span, ok := exactOwnedSpan(frame, "result", "textDocument/references", 31)
			if !ok || string(span.Value) != "null" {
				t.Fatal("ASSERT_PRIVATE_FRAME_SMALL_RESULT_SPAN")
			}
			slice, ok := privateBodySliceFromFrame(frame, span)
			if !ok || !replayPrivateBodySlice(body, slice, "result", "textDocument/references", 31, []byte("null")) {
				t.Fatal("ASSERT_PRIVATE_FRAME_BODY_REPLAY")
			}
		}
	}
}

func TestPrivateFrameDuplicateJSONFieldRejected(t *testing.T) {
	if _, ok := exactJSONField([]byte(`{"result":null,"result":null}`), "result"); ok {
		t.Fatal("ASSERT_PRIVATE_FRAME_DUPLICATE_REJECT")
	}
}
