//go:build darwin || linux

package publication

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"testing"
)

func TestPostPreparationContentPrivateFixedProfiles(t *testing.T) {
	path, _, _ := observedRootFixture(t)
	r, e := OpenContentRetentionRoot(context.Background(), path)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() {
		if e := r.Close(); e != nil {
			t.Error(e)
		}
	})
	ctx := context.Background()
	for _, n := range []int{2<<20 + 1, 4<<20 + 1, 16 << 20, 16<<20 + 1} {
		raw := bytes.Repeat([]byte{'x'}, n)
		id := fmt.Sprintf("sha256:%x", sha256.Sum256(raw))
		if _, e = r.Publish(ctx, id, raw); e == nil {
			t.Fatal("legacy 2MiB relaxed")
		}
		if n > 4<<20 {
			if _, e = r.PublishPartialInspectionV2Private(ctx, id, raw); e == nil {
				t.Fatal("legacy 4MiB relaxed")
			}
		}
		receipt, e := r.PublishPostPreparationV1Private(ctx, id, raw)
		if n > 16<<20 {
			if e == nil || receipt != nil {
				t.Fatal("new one-over admitted")
			}
			continue
		}
		if e != nil || receipt == nil {
			t.Fatal(e)
		}
		got, e := r.ReadPostPreparationV1Private(ctx, id, int64(n))
		if e != nil || !bytes.Equal(got, raw) {
			t.Fatal("new exact read", e)
		}
		if _, e = r.Read(ctx, id, int64(n)); e == nil {
			t.Fatal("legacy read relaxed")
		}
	}
	if _, e = r.ReadPostPreparationV1Private(ctx, "bad", 16<<20+1); e == nil {
		t.Fatal("read ceiling")
	}
}
