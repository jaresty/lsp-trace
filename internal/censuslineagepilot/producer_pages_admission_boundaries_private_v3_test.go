//go:build darwin || linux

package censuslineagepilot

import (
	"context"
	"errors"
	"testing"

	"lsp-trace/internal/publication"
)

func TestProducerPagesPrivateV3AdmissionParserBoundary(t *testing.T) {
	ctx := context.Background()
	p := pageOwnerCountsPrivateV3(t, 16384, 1, 0)
	r, _, e := p.Advance(ctx, 1, pageAllowancePrivateV3(), "")
	if e != nil {
		t.Fatal(e)
	}
	c, e := p.SelectedBatchReader(ctx, r, 0, pageAllowancePrivateV3())
	if e != nil {
		t.Fatal(e)
	}
	var n uint64
	base := 7 + len(p.state.frozen.state.files)
	for j := 0; j < 4; j++ {
		n += uint64(len(p.state.objects[base+j].raw))
	}
	g := selectedObjectCostPrivateV3(n)
	g, _ = g.plus(sourceCheckCostRequiredPrivateV3(t, p))
	g, _ = g.plus(revisionReadCostPrivateV3())
	max := p.state.policy.PayloadBytes + producerPageHeaderPrivateV3 + 4
	for i := uint64(0); i < r.Cursor.PageCount; i++ {
		g, _ = g.plus(PageCostPrivateV3{2 * (max + 1), 8 * (max + 1), 0, producerPacketTopologyWorkPrivateV2 + 16})
	}
	for i := uint64(0); i < r.Cursor.PageCount+3; i++ {
		g, _ = g.plus(pageLedgerOverheadPrivateV3)
	}
	read := p.state.ops.read
	reads := 0
	p.state.ops.read = func(ctx context.Context, root *publication.ContentRetentionRoot, id string, max int64) ([]byte, error) {
		reads++
		return read(ctx, root, id, max)
	}
	report, b, e := c.Advance(ctx, 1, g)
	if !errors.Is(e, ErrPagesPausedPrivateV3) || b.valid() || report.PagesVerified != r.Cursor.PageCount {
		t.Fatal("ASSERT_V3_PARSER_BOUNDARY_PAUSE", report, e)
	}
	before := reads
	report, b, e = c.Advance(ctx, 2, pageAllowancePrivateV3())
	if e != nil || !b.valid() || reads-before != 2 {
		t.Fatal("ASSERT_V3_PARSER_CONTINUES_WITHOUT_PREFIX_REPLAY", report, reads-before, e)
	}
}
func TestProducerPagesPrivateV3AdmissionUnitRefusal(t *testing.T) {
	p := pageOwnerCountsPrivateV3(t, 16384, 1, 0)
	s := p.state
	ctx := context.Background()
	policy := s.policy
	policy.ComputationUnitBytes = 1
	policy.ConfigSHA256 = Digest([]byte("explicit tiny unit test"))
	small, e := NewProducerPagesPrivateV3(ctx, s.frozen, s.origin, s.captures, s.roots, policy, s.ledger)
	if e != nil {
		t.Fatal(e)
	}
	r, _, e := small.Advance(ctx, 1, pageAllowancePrivateV3(), "")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = small.SelectedBatchReader(ctx, r, 0, pageAllowancePrivateV3()); !errors.Is(e, ErrPagesUnitPrivateV3) {
		t.Fatal("ASSERT_V3_OVERSIZED_SELECTED_UNIT_REFUSED", e)
	}
}
