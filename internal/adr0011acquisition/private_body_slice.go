package adr0011acquisition

import (
	"bytes"
	"crypto/sha256"
	"fmt"
)

// privateBodySlice holds a defensive complete original body and a selected raw
// value. Its offset is body-relative; this is not a publication receipt.
type privateBodySlice struct {
	body, value             []byte
	offset, length          int
	bodyDigest, valueDigest string
}

func privateDigest(b []byte) string { return fmt.Sprintf("sha256:%x", sha256.Sum256(b)) }

func privateBodySliceFromFrame(frame []byte, span ownedSpan) (privateBodySlice, bool) {
	body, ok := exactFrameBody(frame)
	if !ok || len(body) > 4194304 || span.FrameLength != len(frame) || span.FrameDigest != privateDigest(frame) || span.Length < 0 || span.Offset < 0 || span.Length > len(frame)-span.Offset {
		return privateBodySlice{}, false
	}
	start := len(frame) - len(body)
	if span.Offset < start || span.Length > len(body)-(span.Offset-start) || !bytes.Equal(frame[span.Offset:span.Offset+span.Length], span.Value) || span.ValueDigest != privateDigest(span.Value) {
		return privateBodySlice{}, false
	}
	return privateBodySlice{body: append([]byte(nil), body...), value: append([]byte(nil), span.Value...), offset: span.Offset - start, length: span.Length, bodyDigest: privateDigest(body), valueDigest: span.ValueDigest}, true
}

// replayPrivateBodySlice parses separately supplied body bytes using the strict
// owned-span parser. The synthetic frame is only a parser input, never evidence.
func replayPrivateBodySlice(body []byte, claimed privateBodySlice, field, method string, id uint64, expected []byte) bool {
	if len(body) == 0 || len(body) > 2<<20 || (field == "params" && len(expected) > 65536) || (field == "result" && len(expected) > 1048576) || claimed.bodyDigest != privateDigest(body) || !bytes.Equal(claimed.body, body) {
		return false
	}
	frame := []byte(fmt.Sprintf("Content-Length: %d\r\n\r\n", len(body)))
	frame = append(frame, body...)
	actual, ok := exactOwnedSpan(frame, field, method, id)
	if !ok {
		return false
	}
	offset := actual.Offset - (len(frame) - len(body))
	return claimed.offset == offset && claimed.length == actual.Length && claimed.length <= len(body)-offset && claimed.valueDigest == actual.ValueDigest && bytes.Equal(claimed.value, actual.Value) && bytes.Equal(actual.Value, expected)
}
