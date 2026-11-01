//go:build darwin || linux

package censuslineagepilot

import (
	"context"
	"testing"
)

// This test-only seam creates fresh original source/discovery/capture owners;
// neither the whole-packet V2 qualifier nor an owner-restoration codec is used.
func ControlledMatchedPagesPrivateV3(t *testing.T, grant PageCostPrivateV3) (ProducerPagesPrivateV3, PageRevisionPrivateV3) {
	t.Helper()
	ctx := context.Background()
	f, o, c, _ := packetFixtureConfiguredPrivateV2(t, 2, 0, true, "controlled-matched-page-peer")
	roots, _, _ := packetRootsPrivateV2(t, f)
	ledger, _, _ := ledgerFixturePrivateV3(t, grant)
	t.Cleanup(func() {
		if e := ledger.Close(); e != nil {
			t.Error(e)
		}
	})
	p, e := NewProducerPagesPrivateV3(ctx, f, o, c, roots, PagePolicyPrivateV3{16384, 1 << 20, Digest([]byte("controlled matched pipeline"))}, ledger)
	if e != nil {
		t.Fatal(e)
	}
	r, _, e := p.Advance(ctx, 1, grant, "")
	if e != nil {
		t.Fatal(e)
	}
	return p, r
}
