//go:build darwin || linux

package censuslineagepilot

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func ledgerFixturePrivateV3(t *testing.T, allowance PageCostPrivateV3) (PageLedgerPrivateV3, string, string) {
	t.Helper()
	path, e := filepath.EvalSymlinks(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	if e = os.Chmod(path, 0700); e != nil {
		t.Fatal(e)
	}
	app := Digest([]byte("owned application"))
	l, e := CreatePageLedgerPrivateV3(context.Background(), path, app, allowance)
	if e != nil {
		t.Fatal(e)
	}
	return l, path, app
}
func TestProducerPagesPrivateV3LedgerReopen(t *testing.T) {
	l, path, app := ledgerFixturePrivateV3(t, pageAllowancePrivateV3())
	left := pageAllowancePrivateV3()
	if e := l.state.reserve(PageCostPrivateV3{LogicalBytes: 100, ReadHashBytes: 200, Work: 1}, &left); e != nil {
		t.Fatal(e)
	}
	generation, tip, spent := l.Tip()
	if e := l.Close(); e != nil {
		t.Fatal(e)
	}
	resumed, e := OpenPageLedgerPrivateV3(context.Background(), path, app, pageAllowancePrivateV3(), generation, tip)
	if e != nil {
		t.Fatal(e)
	}
	defer resumed.Close()
	next, _, after := resumed.Tip()
	if next <= generation || !spent.fits(after) || spent == after {
		t.Fatal("ASSERT_DURABLE_REOPEN_CHARGED", generation, next, spent, after)
	}
	if _, e := OpenPageLedgerPrivateV3(context.Background(), path, app, pageAllowancePrivateV3(), generation, tip); e == nil {
		t.Fatal("ASSERT_SPENT_TIP_REPLAY_REFUSED")
	}
	info, e := os.Stat(filepath.Join(path, ledgerNamePrivateV3(next)))
	if e != nil || info.Mode().Perm() != 0600 {
		t.Fatal("ASSERT_PRIVATE_JOURNAL_MODE", e)
	}
}
func TestProducerPagesPrivateV3LedgerApplicationPause(t *testing.T) {
	// Initial journal, one reservation, then an exact application boundary.
	cost := PageCostPrivateV3{LogicalBytes: 100, ReadHashBytes: 200, WriteBytes: 10, Work: 1}
	charged, _ := cost.plus(pageLedgerOverheadPrivateV3)
	allowance, _ := charged.plus(pageLedgerOverheadPrivateV3)
	l, _, _ := ledgerFixturePrivateV3(t, allowance)
	defer l.Close()
	left := pageAllowancePrivateV3()
	if e := l.state.reserve(cost, &left); e != nil {
		t.Fatal(e)
	}
	g, tip, spent := l.Tip()
	left = pageAllowancePrivateV3() // new exchange must NOT reset the application ledger
	if e := l.state.reserve(cost, &left); !errors.Is(e, ErrPagesPausedPrivateV3) {
		t.Fatal("ASSERT_SHARED_APPLICATION_PAUSE", e)
	}
	gg, tt, ss := l.Tip()
	if gg != g || tt != tip || ss != spent {
		t.Fatal("ASSERT_DENIED_RESERVATION_DOES_NOT_REFUND_OR_SPEND")
	}
}
func TestProducerPagesPrivateV3LedgerTornReservation(t *testing.T) {
	l, path, app := ledgerFixturePrivateV3(t, pageAllowancePrivateV3())
	g, tip, _ := l.Tip()
	if e := os.WriteFile(filepath.Join(path, ledgerNamePrivateV3(g+1)), []byte("torn"), 0600); e != nil {
		t.Fatal(e)
	}
	l.Close()
	if _, e := OpenPageLedgerPrivateV3(context.Background(), path, app, pageAllowancePrivateV3(), g, tip); e == nil {
		t.Fatal("ASSERT_CRASH_GENERATION_NEVER_RESET")
	}
}
func TestProducerPagesPrivateV3LedgerRejectsNonempty(t *testing.T) {
	path, e := filepath.EvalSymlinks(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	os.Chmod(path, 0700)
	if e = os.WriteFile(filepath.Join(path, "protected"), []byte("unchanged"), 0600); e != nil {
		t.Fatal(e)
	}
	l, e := CreatePageLedgerPrivateV3(context.Background(), path, Digest([]byte("wrong root")), pageAllowancePrivateV3())
	if e == nil {
		l.Close()
		t.Fatal("ASSERT_LEDGER_NONEMPTY_ROOT_REFUSED")
	}
}
func TestProducerPagesPrivateV3LedgerSharedOrigins(t *testing.T) {
	first := pageOwnerPrivateV3(t, 16384)
	before := first.BudgetSpent()
	s := first.state
	second, e := NewProducerPagesPrivateV3(context.Background(), s.frozen, s.origin, s.captures, s.roots, s.policy, s.ledger)
	if e != nil {
		t.Fatal(e)
	}
	if first.BudgetSpent() != second.BudgetSpent() || first.BudgetSpent() == before {
		t.Fatal("ASSERT_NO_PER_ORIGIN_BUDGET_RESET")
	}
	if _, _, e = first.Advance(context.Background(), 1, pageAllowancePrivateV3(), ""); e != nil {
		t.Fatal(e)
	}
	if first.BudgetSpent() != second.BudgetSpent() {
		t.Fatal("ASSERT_ORIGINS_SHARE_FAILED_AND_SUCCESSFUL_SPEND")
	}
}
