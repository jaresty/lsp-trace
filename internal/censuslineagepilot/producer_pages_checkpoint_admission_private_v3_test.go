//go:build darwin || linux

package censuslineagepilot

import (
	"context"
	"testing"
)

func TestRetainedCheckpointPrivateV3PublicExport(t *testing.T) {
	ctx := context.Background()
	f, o, c, _ := packetFixtureConfiguredPrivateV2(t, 2, 0, true, "retained-checkpoint-test")
	roots, _, _ := packetRootsPrivateV2(t, f)
	l, _, _ := ledgerFixturePrivateV3(t, pageAllowancePrivateV3())
	defer l.Close()
	p, e := NewProducerPagesPrivateV3(ctx, f, o, c, roots, PagePolicyPrivateV3{16384, 1 << 20, Digest([]byte("checkpoint-config"))}, l)
	if e != nil {
		t.Fatal(e)
	}
	r, _, e := p.Advance(ctx, 1, pageAllowancePrivateV3(), "")
	if e != nil {
		t.Fatal(e)
	}
	raw, e := p.ExportRetainedCheckpointPrivateV3(ctx, r, Digest([]byte("continue-observations")), pageAllowancePrivateV3())
	if e != nil || len(raw) == 0 {
		t.Fatal("ASSERT_RETAINED_CHECKPOINT_EXPORT", e)
	}
	t.Log("ASSERT_RETAINED_CHECKPOINT_EXPORT: PASS")
}
