//go:build darwin || linux

package publication

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"testing"
)

func TestContentRetentionPartialV2Private(t *testing.T) {
	path, _, _ := observedRootFixture(t)
	r, e := OpenContentRetentionRoot(context.Background(), path)
	if e != nil {
		t.Fatal(e)
	}
	defer r.Close()
	for _, size := range []int{3487940, 4 << 20, (4 << 20) + 1} {
		t.Run(fmt.Sprint(size), func(t *testing.T) {
			raw := bytes.Repeat([]byte("x"), size)
			id := fmt.Sprintf("sha256:%x", sha256.Sum256(raw))
			if receipt, e := r.Publish(context.Background(), id, raw); e == nil || receipt != nil {
				t.Fatal("ASSERT_OLD_STORAGE_LIMIT_UNCHANGED")
			}
			receipt, e := r.PublishPartialInspectionV2Private(context.Background(), id, raw)
			if size > 4<<20 {
				if e == nil || receipt != nil {
					t.Fatal("ASSERT_NEW_STORAGE_PLUS_ONE")
				}
				if got, e := r.ReadPartialInspectionV2Private(context.Background(), id, int64(size)); e == nil || got != nil {
					t.Fatal("ASSERT_NEW_READ_PLUS_ONE")
				}
				return
			}
			if e != nil || receipt == nil || receipt.ByteLength != uint64(size) || receipt.Digest != id {
				t.Fatal("ASSERT_NEW_STORAGE_EXACT_RECEIPT", e)
			}
			got, e := r.ReadPartialInspectionV2Private(context.Background(), id, int64(size))
			if e != nil || !bytes.Equal(got, raw) {
				t.Fatal("ASSERT_NEW_STORAGE_EXACT_READ", e)
			}
			if got, e := r.Read(context.Background(), id, int64(size)); e == nil || got != nil {
				t.Fatal("ASSERT_OLD_READ_LIMIT_UNCHANGED")
			}
			if again, e := r.PublishPartialInspectionV2Private(context.Background(), id, raw); e == nil || again != nil {
				t.Fatal("ASSERT_NEW_NO_REPLACE")
			}
		})
	}
}
