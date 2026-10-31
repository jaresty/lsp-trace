//go:build darwin || linux

package censuslineagepilot

import (
	"context"
	"errors"
	"os"
	"testing"

	"lsp-trace/internal/publication"
)

func sourceCheckCostRequiredPrivateV3(t *testing.T, p ProducerPagesPrivateV3) PageCostPrivateV3 {
	t.Helper()
	cost, err := p.state.sourceCheckCost()
	if err != nil {
		t.Fatal(err)
	}
	return cost
}
func onePageGrantPrivateV3(t *testing.T, p ProducerPagesPrivateV3) PageCostPrivateV3 {
	t.Helper()
	a, _ := pagePairCostPrivateV3(producerPageHeaderPrivateV3).plus(sourceCheckCostRequiredPrivateV3(t, p))
	a, _ = a.plus(pageJournalCostPrivateV3)
	a, _ = a.plus(pagePairCostPrivateV3(p.state.policy.PayloadBytes + producerPageHeaderPrivateV3 + 4))
	a, _ = a.plus(pageLedgerOverheadPrivateV3)
	a, _ = a.plus(pageLedgerOverheadPrivateV3)
	return a
}
func renewedOriginalPagesPrivateV3(t *testing.T, p ProducerPagesPrivateV3) ProducerPagesPrivateV3 {
	t.Helper()
	s := p.state
	q, e := NewProducerPagesPrivateV3(context.Background(), s.frozen, s.origin, s.captures, s.roots, s.policy, s.ledger)
	if e != nil {
		t.Fatal(e)
	}
	return q
}
func TestProducerPagesPrivateV3CheckpointOriginalResume(t *testing.T) {
	ctx := context.Background()
	p := pageOwnerPrivateV3(t, 4096)
	first, id, e := p.Advance(ctx, 1, onePageGrantPrivateV3(t, p), "")
	if !errors.Is(e, ErrPagesPausedPrivateV3) || first.Cursor.PageCount != 1 {
		t.Fatal("ASSERT_RESUME_FIXTURE_PARTIAL", e)
	}
	spent := p.BudgetSpent()
	q := renewedOriginalPagesPrivateV3(t, p)
	restored, e := q.ResumeOriginalCheckpoint(ctx, 1, pageAllowancePrivateV3())
	if e != nil || restored.Cursor != first.Cursor {
		t.Fatal("ASSERT_ORIGINAL_OWNER_FRESH_REVERIFICATION", e)
	}
	if !spent.fits(q.BudgetSpent()) || spent == q.BudgetSpent() {
		t.Fatal("ASSERT_RESUME_DOES_NOT_RESET_LEDGER")
	}
	if _, _, e = q.Advance(ctx, 1, pageAllowancePrivateV3(), id); e == nil {
		t.Fatal("ASSERT_RESUME_CANNOT_REPLAY_SPENT_GENERATION")
	}
	r, next, e := q.Advance(ctx, 2, pageAllowancePrivateV3(), id)
	if e != nil || r.Status != "COMPLETE_ORIGIN_PAGES" || next == id {
		t.Fatal("ASSERT_RESUME_CONTINUES_NEW_GENERATION", e)
	}
	stale := renewedOriginalPagesPrivateV3(t, p)
	if _, e = stale.ResumeOriginalCheckpoint(ctx, 1, pageAllowancePrivateV3()); e == nil {
		t.Fatal("ASSERT_OLD_CHECKPOINT_REFUSED_AFTER_LATER_ATTEMPT")
	}
	if e = q.VerifyPrefix(ctx, first, pageAllowancePrivateV3()); e != nil {
		t.Fatal("ASSERT_EARLIER_ADMITTED_PROGRESS_PRESERVED", e)
	}
}
func TestProducerPagesPrivateV3CheckpointCrashAndCancel(t *testing.T) {
	ctx := context.Background()
	p := pageOwnerPrivateV3(t, 4096)
	first, id, e := p.Advance(ctx, 1, onePageGrantPrivateV3(t, p), "")
	if !errors.Is(e, ErrPagesPausedPrivateV3) {
		t.Fatal(e)
	}
	// Crash-equivalent controlled stop at storage: durable attempt exists, but no
	// final revision/checkpoint can be published. Prior admitted prefix survives.
	p.state.ops.publish = func(context.Context, *publication.ContentRetentionRoot, string, []byte) (*publication.BoundFileReceipt, error) {
		return nil, os.ErrPermission
	}
	if _, _, e = p.Advance(ctx, 2, pageAllowancePrivateV3(), id); e == nil {
		t.Fatal("ASSERT_CRASH_FIXTURE_STORAGE_FAILURE")
	}
	q := renewedOriginalPagesPrivateV3(t, p)
	restored, e := q.ResumeOriginalCheckpoint(ctx, 2, pageAllowancePrivateV3())
	if e != nil || restored.Cursor != first.Cursor {
		t.Fatal("ASSERT_CRASH_RESUMES_PRIOR_ADMITTED_ONLY", e)
	}
	if _, _, e = q.Advance(ctx, 2, pageAllowancePrivateV3(), id); e == nil {
		t.Fatal("ASSERT_CRASH_ATTEMPT_SPENT")
	}
	r, _, e := q.Advance(ctx, 3, pageAllowancePrivateV3(), id)
	if e != nil || r.Status != "COMPLETE_ORIGIN_PAGES" {
		t.Fatal("ASSERT_CRASH_REPAIR_EXISTING_EXACT_PAGES", e)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, _, e = q.Advance(cancelled, 4, pageAllowancePrivateV3(), q.state.lastRevision); e == nil {
		t.Fatal("ASSERT_CANCEL_NO_WORK")
	}
}
