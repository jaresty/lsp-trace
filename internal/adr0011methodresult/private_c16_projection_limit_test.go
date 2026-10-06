package adr0011methodresult

import (
	"bytes"
	"fmt"
	transport "lsp-trace/internal/adr0011methodtransport"
	"testing"
)

func c16Raw(n int) []byte {
	var b bytes.Buffer
	b.WriteByte('[')
	for i := 0; i < n; i++ {
		if i > 0 {
			b.WriteByte(',')
		}
		fmt.Fprintf(&b, `{"uri":"file:///t","range":{"start":{"line":%d,"character":0},"end":{"line":%d,"character":1}}}`, i, i)
	}
	b.WriteByte(']')
	return b.Bytes()
}
func TestPrivateC16ProjectionLimit(t *testing.T) {
	a := &c16RecordingAdmitter{}
	p, f := parseRawUntrustedWithAdmission(transport.MethodDefinition, c16Raw(4096), 4097, a)
	if f != nil || len(p.Items) != 4096 || a.callbacks != 4096 {
		t.Fatalf("ASSERT_C16_4096 failure=%+v items=%d callbacks=%d", f, len(p.Items), a.callbacks)
	}
	source := bytes.Repeat([]byte("x\n"), 4097)
	active := true
	d, ok := evaluatePrivateC16(privateC16View{p.Items, &active}, map[string][]byte{"file:///t": source})
	active = false
	if !ok || d.Count != 4096 {
		t.Fatalf("ASSERT_C16_4096_PROJECTION %+v %t", d, ok)
	}
	a = &c16RecordingAdmitter{failAt: 4097}
	p, f = parseRawUntrustedWithAdmission(transport.MethodDefinition, c16Raw(4097), 4097, a)
	if f == nil || len(p.Items) != 0 || a.callbacks != 4096 {
		t.Fatalf("ASSERT_C16_4097 failure=%+v items=%d callbacks=%d", f, len(p.Items), a.callbacks)
	}
}
