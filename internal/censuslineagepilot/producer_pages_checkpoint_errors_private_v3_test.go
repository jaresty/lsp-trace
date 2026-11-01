//go:build darwin || linux

package censuslineagepilot

import (
	"context"
	"errors"
	"testing"
)

func TestRetainedCheckpointPrivateV3NestedPauseIdentity(t *testing.T) {
	x := retainedFixtureV3(t)
	ctx := context.Background()
	owner, e := x.admit(ctx, x.raw, x.d)
	if e != nil {
		t.Fatal(e)
	}
	grant := x.l.state.allowance
	b, e := owner.SelectedBatch(ctx, 0, grant)
	if e != nil {
		t.Fatal(e)
	}
	cost, e := retainedSourceReservationPrivateV3(x.fresh)
	if e != nil {
		t.Fatal(e)
	}
	cost, _ = cost.plus(pageLedgerOverheadPrivateV3)
	first, _ := owner.state.stageCost().plus(pageLedgerOverheadPrivateV3)
	exchange, _ := first.plus(cost)
	if _, e = b.Read(ctx, exchange); !errors.Is(e, ErrPagesPausedPrivateV3) {
		t.Fatal("ASSERT_RETAINED_NESTED_PAUSE_IDENTITY", e)
	}
	if _, e = b.Read(ctx, grant); e != nil {
		t.Fatal("ASSERT_RETAINED_PAUSE_RETRY", e)
	}
	t.Log("ASSERT_RETAINED_NESTED_PAUSE_IDENTITY: PASS")
}
func TestRetainedCheckpointPrivateV3AdmissionPauseIdentity(t *testing.T) {
	x := retainedFixtureV3(t)
	ctx := context.Background()
	g, _, _ := x.l.Tip()
	cost := checkpointReadCostPrivateV3(4 * scopeMetadataPrivateV2)
	cost, _ = cost.plus(pageLedgerOverheadPrivateV3)
	each := checkpointReadCostPrivateV3(producerPageHeaderPrivateV3)
	count := g + 1
	chain := PageCostPrivateV3{each.LogicalBytes * count, each.ReadHashBytes * count, each.WriteBytes * count, each.Work * count}
	chain, _ = chain.plus(pageLedgerOverheadPrivateV3)
	cost, _ = cost.plus(chain)
	source, _ := retainedSourceReservationPrivateV3(x.fresh)
	source, _ = source.plus(pageLedgerOverheadPrivateV3)
	cost, _ = cost.plus(source)
	_, e := AdmitRetainedPageCheckpointPrivateV3(ctx, x.raw, x.fresh, x.p.state.roots, x.l, x.d, cost)
	if !errors.Is(e, ErrPagesPausedPrivateV3) {
		t.Fatal("ASSERT_RETAINED_ADMISSION_PAUSE_IDENTITY", e)
	}
	if _, e = x.admit(ctx, x.raw, x.d); e != nil {
		t.Fatal("ASSERT_RETAINED_ADMISSION_PAUSE_RETRY", e)
	}
	t.Log("ASSERT_RETAINED_ADMISSION_PAUSE_IDENTITY: PASS")
}
