package adr0011acquisition

import (
	"bytes"
	"testing"
)

func TestPrivateBodySliceReplay(t *testing.T) {
	for _, method := range []string{"textDocument/documentSymbol", "textDocument/references"} {
		for _, value := range []string{"null", "[]", `{ "a" : 1 }`} {
			for _, field := range []string{"params", "result"} {
				body := []byte(`{ "jsonrpc" : "2.0", "id" : 7, `)
				if field == "params" {
					body = append(body, []byte(`"method" : "`+method+`", `)...)
				}
				body = append(body, []byte(`"`+field+`" : `+value+` }`)...)
				frame := spanFrame(string(body))
				span, ok := exactOwnedSpan(frame, field, method, 7)
				if !ok {
					t.Fatalf("span %s %s %s", method, field, value)
				}
				selected, ok := privateBodySliceFromFrame(frame, span)
				if !ok || !bytes.Equal(selected.body, body) || !bytes.Equal(selected.value, []byte(value)) || !replayPrivateBodySlice(append([]byte(nil), body...), selected, field, method, 7, []byte(value)) {
					t.Fatalf("valid %s %s %s", method, field, value)
				}
				changed := selected
				changed.offset++
				if replayPrivateBodySlice(body, changed, field, method, 7, []byte(value)) {
					t.Fatal("shifted")
				}
				changed = selected
				changed.length++
				if replayPrivateBodySlice(body, changed, field, method, 7, []byte(value)) {
					t.Fatal("one past")
				}
				changed = selected
				changed.value = []byte(`false`)
				if replayPrivateBodySlice(body, changed, field, method, 7, []byte(value)) {
					t.Fatal("modified value")
				}
				swapped := append([]byte(nil), body...)
				swapped[len(swapped)-1] = ' '
				if replayPrivateBodySlice(swapped, selected, field, method, 7, []byte(value)) {
					t.Fatal("swapped body")
				}
				if replayPrivateBodySlice(body[:len(body)-1], selected, field, method, 7, []byte(value)) {
					t.Fatal("missing bytes")
				}
				for _, malformed := range [][]byte{append(append([]byte(nil), body...), []byte(` {}`)...), append(append([]byte(nil), body[:len(body)-1]...), []byte(`, "id":7 }`)...)} {
					changed = selected
					changed.body = malformed
					changed.bodyDigest = privateDigest(malformed)
					if replayPrivateBodySlice(malformed, changed, field, method, 7, []byte(value)) {
						t.Fatal("trailing or duplicate accepted")
					}
				}
				if _, ok := privateBodySliceFromFrame(frame[:len(frame)-1], span); ok {
					t.Fatal("incomplete frame")
				}
			}
		}
	}
}
