package inventorysuccessor

import (
	"lsp-trace/internal/graphprovenance"
	"testing"
)

func TestPrivateNativeConstructorDiscriminatorPrecopyOverflow(t *testing.T) {
	for _, q := range []struct {
		name           string
		n, scale, base uint64
		want           bool
	}{
		{"ordinary", 65536, 8, 65536, true},
		{"equality", 0, 1, uint64(graphprovenance.MaxEnvelopeBytesV5), true},
		{"above", 1, 1, uint64(graphprovenance.MaxEnvelopeBytesV5), false},
		{"multiply-overflow", uint64(1) << 63, 2, 0, false},
		{"add-overflow", 1, 1, ^uint64(0), false},
	} {
		t.Run(q.name, func(t *testing.T) {
			_, ok := privateNativeEncodedSizeV2(q.n, q.scale, q.base)
			if ok != q.want {
				t.Fatalf("ASSERT_PRECOPY_%s: got=%v want=%v", q.name, ok, q.want)
			}
			t.Logf("ASSERT_PRECOPY_%s: PASS", q.name)
		})
	}
}
