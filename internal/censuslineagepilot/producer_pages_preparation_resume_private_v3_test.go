//go:build darwin || linux

package censuslineagepilot_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	pilot "lsp-trace/internal/censuslineagepilot"
	prep "lsp-trace/internal/privatedescribeprep"
	derived "lsp-trace/internal/retainedpartialprogramc"
)

func prepGrantV3() pilot.PageCostPrivateV3 {
	return pilot.PageCostPrivateV3{LogicalBytes: 1 << 40, ReadHashBytes: 1 << 40, WriteBytes: 1 << 34, Work: 1 << 32}
}
func TestProducerPagesPrivateV3PreparedPrefixResume(t *testing.T) {
	ctx := context.Background()
	g := prepGrantV3()
	owner, r := pilot.ControlledMatchedPagesPrivateV3(t, g)
	reader, e := owner.SelectedBatchReader(ctx, r, 0, g)
	if e != nil {
		t.Fatal(e)
	}
	_, b, e := reader.Advance(ctx, 1, g)
	if e != nil {
		t.Fatal(e)
	}
	p, e := derived.ProjectPrivateV3(ctx, b, g)
	if e != nil {
		t.Fatal(e)
	}
	a, e := derived.ComputeInstabilityPrivateV3(ctx, p, []uint64{3, 7}, g)
	if e != nil {
		t.Fatal(e)
	}
	if _, err := prep.NewOwnASTReaderPrivateV3(ctx, p, a, prep.Limits{MaxTargets: 64, MaxPartBytes: 2048, MaxTotalBytes: 2048, MaxParts: 1, MaxSourceBytes: 2 << 20}, g); !errors.Is(err, pilot.ErrPagesUnitPrivateV3) {
		t.Fatal("ASSERT_V3_PREPARED_HEADER_UNIT_PREFLIGHT", err)
	}
	c, e := prep.NewOwnASTReaderPrivateV3(ctx, p, a, prep.Limits{MaxTargets: 64, MaxPartBytes: 64 << 10, MaxTotalBytes: 8 << 20, MaxParts: 256, MaxSourceBytes: 2 << 20}, g)
	if e != nil {
		t.Fatal(e)
	}
	before := p.BudgetSpent()
	if _, e = p.Read(ctx, g); e != nil {
		t.Fatal(e)
	}
	after := p.BudgetSpent()
	one := pilot.PageCostPrivateV3{LogicalBytes: after.LogicalBytes - before.LogicalBytes, ReadHashBytes: after.ReadHashBytes - before.ReadHashBytes, WriteBytes: after.WriteBytes - before.WriteBytes, Work: after.Work - before.Work}
	two := pilot.PageCostPrivateV3{LogicalBytes: 2 * one.LogicalBytes, ReadHashBytes: 2 * one.ReadHashBytes, WriteBytes: 2 * one.WriteBytes, Work: 2 * one.Work}
	status, empty, e := c.Advance(ctx, 1, two)
	if !errors.Is(e, pilot.ErrPagesPausedPrivateV3) || status.Prepared != 1 || status.Denominator != 2 {
		t.Fatal("ASSERT_V3_PREPARED_PREFIX_PAUSE", status, e)
	}
	if _, e = empty.Read(ctx, g); e == nil {
		t.Fatal("ASSERT_V3_PARTIAL_BODY_NOT_EXPOSED")
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	status, _, e = c.Advance(canceled, 2, g)
	if !errors.Is(e, context.Canceled) || status.Prepared != 1 {
		t.Fatal("ASSERT_V3_PREPARED_PREFIX_CANCEL_PRESERVED", status, e)
	}
	status, prepared, e := c.Advance(ctx, 3, g)
	if e != nil || status.Prepared != 2 {
		t.Fatal("ASSERT_V3_PREPARED_PREFIX_RESUMED", status, e)
	}
	raw, e := prepared.Read(ctx, g)
	if e != nil {
		t.Fatal(e)
	}
	var result prep.OwnASTReportPrivateV3
	if json.Unmarshal(raw, &result) != nil || len(result.Bodies) != 2 || result.Bodies[0].Declaration.Body != "func A0() {}" || result.Bodies[1].Declaration.Body != "func A1() {}" {
		t.Fatal("ASSERT_V3_OWN_DISTINCT_BODIES", string(raw))
	}
	// A single exchange pays for a partial publication, not an automatic reset.
	pub, e := prepared.Publication(ctx, g)
	if e != nil {
		t.Fatal(e)
	}
	// Failed zero-grant publication leaves the existing prepared owner intact.
	ps, e := pub.Advance(ctx, 1, pilot.PageCostPrivateV3{})
	if !errors.Is(e, pilot.ErrPagesPausedPrivateV3) || ps.VerifiedReferences != 0 {
		t.Fatal("ASSERT_V3_PUBLICATION_ZERO_PAUSE", ps, e)
	}
	v, e := p.Read(ctx, g)
	if e != nil {
		t.Fatal(e)
	}
	before = p.BudgetSpent()
	if _, e = p.RetainPreparedBytes(ctx, v.DerivedCanonicalV5, g); e != nil {
		t.Fatal(e)
	}
	after = p.BudgetSpent()
	firstReference := pilot.PageCostPrivateV3{LogicalBytes: after.LogicalBytes - before.LogicalBytes, ReadHashBytes: after.ReadHashBytes - before.ReadHashBytes, WriteBytes: after.WriteBytes - before.WriteBytes, Work: after.Work - before.Work}
	ps, e = pub.Advance(ctx, 2, firstReference)
	if !errors.Is(e, pilot.ErrPagesPausedPrivateV3) || ps.VerifiedReferences != 1 || ps.CanonicalSHA256 != result.Source.DerivedCanonicalV5SHA256 {
		t.Fatal("ASSERT_V3_VERIFIED_REFERENCE_PREFIX_PAUSE", ps, e)
	}
	ps, e = pub.Advance(ctx, 3, g)
	if e != nil || ps.VerifiedReferences != 3 || ps.PreparedSHA256 != pilot.Digest(raw) {
		t.Fatal("ASSERT_V3_PUBLICATION_RESUMES", ps, e)
	}
}
