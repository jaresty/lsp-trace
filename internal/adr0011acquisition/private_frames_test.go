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

func TestPrivateFrameDuplicateJSONFieldRejected(t *testing.T) {
	if _, ok := exactJSONField([]byte(`{"result":null,"result":null}`), "result"); ok {
		t.Fatal("ASSERT_PRIVATE_FRAME_DUPLICATE_REJECT")
	}
}
