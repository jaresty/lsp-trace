//go:build darwin || linux

package censuslineagepilot

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"lsp-trace/internal/publication"
)

func TestProducerPagesPrivateV3SelectedAssembly(t *testing.T) {
	p := pageOwnerCountsPrivateV3(t, 16384, 1, 0)
	r, _, err := p.Advance(context.Background(), 1, pageAllowancePrivateV3(), "")
	if err != nil {
		t.Fatal(err)
	}
	// Select the largest original object, so at least one object crosses pages.
	object := uint64(0)
	for i, obj := range p.state.objects {
		if len(obj.raw) > len(p.state.objects[object].raw) {
			object = uint64(i)
		}
	}
	reader, err := p.ObjectReader(context.Background(), r, object, pageAllowancePrivateV3())
	if err != nil {
		t.Fatal(err)
	}
	max := p.state.policy.PayloadBytes + producerPageHeaderPrivateV3 + 4
	grant, _ := sourceCheckCostRequiredPrivateV3(t, p).plus(PageCostPrivateV3{2 * (max + 1), 8 * (max + 1), 0, producerPacketTopologyWorkPrivateV2 + 16})
	grant, _ = grant.plus(revisionReadCostPrivateV3())
	for i := 0; i < 3; i++ {
		grant, _ = grant.plus(pageLedgerOverheadPrivateV3)
	}
	reads := 0
	read := p.state.ops.read
	p.state.ops.read = func(ctx context.Context, root *publication.ContentRetentionRoot, id string, n int64) ([]byte, error) {
		reads++
		return read(ctx, root, id, n)
	}
	allocation := selectedObjectCostPrivateV3(uint64(len(p.state.objects[object].raw)))
	first, _ := grant.plus(allocation)
	first, _ = first.plus(pageLedgerOverheadPrivateV3)
	var progress OriginalPageObjectProgressPrivateV3
	var out []byte
	before := p.BudgetSpent()
	for generation := uint64(1); generation <= r.Cursor.PageCount; generation++ {
		exchange := grant
		if generation == 1 {
			exchange = first
		}
		progress, out, err = reader.Advance(context.Background(), generation, exchange)
		if err != nil && !errors.Is(err, ErrPagesPausedPrivateV3) {
			t.Fatal("ASSERT_SELECTED_FORWARD_PROGRESS", generation, err)
		}
		if progress.PagesVerified != generation {
			t.Fatal("ASSERT_SELECTED_FORWARD_PROGRESS", generation, progress)
		}
		if generation < r.Cursor.PageCount && out != nil {
			t.Fatal("ASSERT_NO_PARTIAL_BYTES_EXPOSED")
		}
	}
	if progress.Status != "COMPLETE_SELECTED_ORIGINAL_OBJECT" || !bytes.Equal(out, p.state.objects[object].raw) {
		t.Fatal("ASSERT_SELECTED_EXACT_ORIGINAL", progress)
	}
	if reads != 4*int(r.Cursor.PageCount) {
		t.Fatal("ASSERT_SELECTED_NO_PREFIX_REPLAY", reads, r.Cursor.PageCount)
	}
	if !before.fits(progress.Cumulative) || before == progress.Cumulative {
		t.Fatal("ASSERT_SELECTED_SHARED_LEDGER")
	}
	if _, _, err = reader.Advance(context.Background(), r.Cursor.PageCount, grant); err == nil {
		t.Fatal("ASSERT_SELECTED_SPENT_GENERATION")
	}
}

func TestProducerPagesPrivateV3SelectedStale(t *testing.T) {
	p := pageOwnerCountsPrivateV3(t, 16384, 1, 0)
	r, _, err := p.Advance(context.Background(), 1, pageAllowancePrivateV3(), "")
	if err != nil {
		t.Fatal(err)
	}
	reader, err := p.ObjectReader(context.Background(), r, 0, pageAllowancePrivateV3())
	if err != nil {
		t.Fatal(err)
	}
	// An empty exchange pauses without reading, but consumes this handle generation.
	_, _, err = reader.Advance(context.Background(), 1, PageCostPrivateV3{})
	if !errors.Is(err, ErrPagesPausedPrivateV3) {
		t.Fatal("ASSERT_SELECTED_ZERO_BUDGET_PAUSE", err)
	}
	if _, _, err = reader.Advance(context.Background(), 1, pageAllowancePrivateV3()); err == nil {
		t.Fatal("ASSERT_SELECTED_STALE_GENERATION")
	}
	copyState := *reader.state
	if _, _, err = (OriginalPageObjectReaderPrivateV3{&copyState}).Advance(context.Background(), 2, pageAllowancePrivateV3()); err == nil {
		t.Fatal("ASSERT_SELECTED_COPIED_STATE_REFUSED")
	}
	// Do not double-close the helper's independently cleaned up ledger.
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err = os.Chmod(dir, 0700); err != nil {
		t.Fatal(err)
	}
	closed, err := CreatePageLedgerPrivateV3(context.Background(), dir, Digest([]byte("closed selected reader test")), pageAllowancePrivateV3())
	if err != nil {
		t.Fatal(err)
	}
	p.state.ledger = closed
	if err = closed.Close(); err != nil {
		t.Fatal(err)
	}
	if _, _, err = reader.Advance(context.Background(), 2, pageAllowancePrivateV3()); err == nil {
		t.Fatal("ASSERT_SELECTED_CLOSED_LEDGER_REFUSED")
	}
}
