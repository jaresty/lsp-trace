//go:build darwin || linux

package censuslineagepilot

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestRetainedCheckpointPrivateV3SelectedCursorGuards(t *testing.T) {
	x := retainedFixtureV3(t)
	ctx := context.Background()
	grant := x.l.state.allowance
	owner, e := x.admit(ctx, x.raw, x.d)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = x.admit(ctx, x.raw, x.d); e == nil {
		t.Fatal("ASSERT_RETAINED_EXCLUSIVE_CLAIM")
	}
	r, e := owner.SelectedBatchReader(ctx, 0, grant)
	if e != nil {
		t.Fatal(e)
	}
	clone := *r.state
	if _, _, e = (RetainedSelectedReaderPrivateV3{&clone}).Advance(ctx, 1, 1, grant); e == nil {
		t.Fatal("ASSERT_RETAINED_READER_SELF_GUARD")
	}
	p, empty, e := r.Advance(ctx, 1, 1, grant)
	if e != nil || p.VerifiedPages != 1 || p.Status != "PAUSED_PAGE_BOUND" {
		t.Fatal("ASSERT_RETAINED_SELECTED_FIRST_PAGE", p, e)
	}
	if _, e = empty.Read(ctx, grant); e == nil {
		t.Fatal("ASSERT_RETAINED_SELECTED_PARTIAL_NOT_COMPLETE")
	}
	p, _, e = r.Advance(ctx, 2, 1, PageCostPrivateV3{})
	if !errors.Is(e, ErrPagesPausedPrivateV3) || p.VerifiedPages != 1 {
		t.Fatal("ASSERT_RETAINED_SELECTED_BUDGET_PRESERVES_PROGRESS", p, e)
	}
	dead, cancel := context.WithCancel(ctx)
	cancel()
	p, _, e = r.Advance(dead, 3, 1, grant)
	if !errors.Is(e, context.Canceled) || p.VerifiedPages != 1 {
		t.Fatal("ASSERT_RETAINED_SELECTED_CANCEL_PRESERVES_PROGRESS", p, e)
	}
	p, _, e = r.Advance(ctx, 4, 1, grant)
	if e != nil || p.VerifiedPages != 2 {
		t.Fatal("ASSERT_RETAINED_SELECTED_FORWARD_PROGRESS", p, e)
	}
	if _, _, e = r.Advance(ctx, 4, 1, grant); e == nil {
		t.Fatal("ASSERT_RETAINED_SELECTED_SPENT_GENERATION")
	}
	saved, e := r.Checkpoint(ctx, Digest([]byte("saved-selected")), grant)
	if e != nil {
		t.Fatal(e)
	}
	var m RetainedPageCheckpointManifestPrivateV3
	if canonicalRetainedPrivateV3(saved, &m) != nil || m.SelectionSnapshot == "" || m.NextBatch != 0 {
		t.Fatal("ASSERT_RETAINED_SELECTED_CHECKPOINT_NOT_COMPLETE")
	}
	// A changed future page after a saved suffix cannot yield a typed selected batch.
	if r.state.role < 4 {
		id := r.state.last
		if id == "" {
			d, e := owner.state.findObject(ctx, 7+m.Members+uint64(r.state.role), &grant)
			if e != nil {
				t.Fatal(e)
			}
			id = d.LastPage
		}
		if e = os.WriteFile(filepath.Join(m.Roots[0].Path, id), []byte("changed future page"), 0600); e != nil {
			t.Fatal(e)
		}
		if _, _, e = r.Advance(ctx, 5, ^uint64(0), x.l.state.allowance); e == nil {
			t.Fatal("ASSERT_RETAINED_CORRUPT_SUFFIX_AFTER_PAUSE")
		}
	}
}
func TestRetainedCheckpointPrivateV3CorruptSpoolAdmission(t *testing.T) {
	x := retainedFixtureV3(t)
	ctx := context.Background()
	grant := x.l.state.allowance
	owner, e := x.admit(ctx, x.raw, x.d)
	if e != nil {
		t.Fatal(e)
	}
	r, e := owner.SelectedBatchReader(ctx, 0, grant)
	if e != nil {
		t.Fatal(e)
	}
	if _, _, e = r.Advance(ctx, 1, 1, grant); e != nil {
		t.Fatal(e)
	}
	raw, e := r.Checkpoint(ctx, Digest([]byte("spool-refusal")), grant)
	if e != nil {
		t.Fatal(e)
	}
	var m RetainedPageCheckpointManifestPrivateV3
	if canonicalRetainedPrivateV3(raw, &m) != nil {
		t.Fatal("manifest")
	}
	left := grant
	snapshot, e := owner.state.readPair(ctx, m.SelectionSnapshot, producerPageHeaderPrivateV3, &left)
	if e != nil {
		t.Fatal(e)
	}
	var spool retainedSelectedSnapshotPrivateV3
	if json.Unmarshal(snapshot, &spool) != nil {
		t.Fatal("snapshot")
	}
	if e = os.WriteFile(filepath.Join(m.Roots[0].Path, spool.Raw[0]), []byte("changed spool"), 0600); e != nil {
		t.Fatal(e)
	}
	g, tip, _ := x.l.Tip()
	if e = x.l.Close(); e != nil {
		t.Fatal(e)
	}
	l, e := openRetainedTestLedgerPrivateV3(ctx, x.l.state.path, m.Application, grant, g, tip)
	if e != nil {
		t.Fatal(e)
	}
	defer l.Close()
	d := RetainedCheckpointDisclosurePrivateV3{m.Application, m.Revision.Policy.ConfigSHA256, m.Intent, RetainedCheckpointPreparationRevisionPrivateV3}
	if _, e = AdmitRetainedPageCheckpointPrivateV3(ctx, raw, x.fresh, x.p.state.roots, l, d, grant); e == nil {
		t.Fatal("ASSERT_RETAINED_CORRUPT_SPOOL_REFUSED_AT_ADMISSION")
	}
}
func TestRetainedCheckpointPrivateV3FreshSourceReservation(t *testing.T) {
	x := retainedFixtureV3(t)
	ctx := context.Background()
	spec := x.fresh.state.spec
	if e := os.WriteFile(filepath.Join(spec.Root, spec.Members[0].CanonicalRelativePath), make([]byte, 1<<20), 0600); e != nil {
		t.Fatal(e)
	}
	_, _, before := x.l.Tip()
	if _, e := FreezeRetainedCheckpointSourcePrivateV3(ctx, spec, x.l, PageCostPrivateV3{}); !errors.Is(e, ErrPagesPausedPrivateV3) {
		t.Fatal("ASSERT_RETAINED_FRESH_SOURCE_DENIED_BEFORE_READ", e)
	}
	_, _, after := x.l.Tip()
	if before != after {
		t.Fatal("denied reservation unexpectedly changed")
	}
	if _, e := FreezeRetainedCheckpointSourcePrivateV3(ctx, spec, x.l, x.l.state.allowance); e == nil {
		t.Fatal("ASSERT_RETAINED_FRESH_SOURCE_CHANGED_REFUSED")
	}
	_, _, after = x.l.Tip()
	if !before.fits(after) || before == after {
		t.Fatal("ASSERT_RETAINED_FAILED_FRESH_SOURCE_CHARGED")
	}
}
