//go:build darwin

package inventorysuccessor

import (
	"encoding/binary"
	"testing"
)

// Synthetic, non-executable filesystem metadata only. No directory ACL changes.
func TestLocalStageDarwinACLMetadata(t *testing.T) {
	fixture := func(kind uint32) []byte {
		b := make([]byte, 80)
		put := binary.NativeEndian.PutUint32
		put(b, 80)
		put(b[4:], 8)
		put(b[8:], 68)
		put(b[12:], 0x012cc16d)
		put(b[48:], 1)
		put(b[72:], kind)
		return b
	}
	for _, c := range []struct {
		name string
		b    []byte
		safe bool
	}{
		{"deny-only", fixture(2), true},
		{"grant", fixture(1), false},
		{"unknown-kind", fixture(3), false},
		{"truncated", fixture(2)[:12], false},
		{"unknown-empty", make([]byte, 12), false},
	} {
		t.Run(c.name, func(t *testing.T) {
			if got := admitLocalDarwinACLBytesV1(c.b) == nil; got != c.safe {
				t.Fatal("ASSERT_STAGE_ACL_METADATA", got, c.safe)
			}
		})
	}
}
