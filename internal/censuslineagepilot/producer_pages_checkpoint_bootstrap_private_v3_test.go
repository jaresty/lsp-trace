//go:build darwin || linux

package censuslineagepilot

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func openRetainedTestLedgerPrivateV3(ctx context.Context, path, app string, grant PageCostPrivateV3, g uint64, tip string) (ValidatedCurrentLedgerPrivateV3, error) {
	bootstrap := grant
	return OpenValidatedCurrentLedgerPrivateV3(ctx, path, app, grant, g, tip, &bootstrap)
}

func TestRetainedCheckpointPrivateV3BeforeFreezeDamagedMiddle(t *testing.T) {
	for _, damage := range []string{"missing", "partial", "noncanonical", "previous", "cumulative", "zero"} {
		t.Run(damage, func(t *testing.T) {
			ctx := context.Background()
			grant := PageCostPrivateV3{1 << 40, 1 << 40, 1 << 34, 1 << 32}
			root, e := filepath.EvalSymlinks(t.TempDir())
			if e != nil {
				t.Fatal(e)
			}
			spec := producerSpecPrivateV2(t, root, 1, 0)
			if _, e := FreezeScopePrivateV2(ctx, spec); e != nil {
				t.Fatal("source setup", e)
			}
			l, path, app := ledgerFixturePrivateV3(t, grant)
			left := grant
			for i := 0; i < 3; i++ {
				if e := l.state.reserve(PageCostPrivateV3{Work: 1}, &left); e != nil {
					t.Fatal(e)
				}
			}
			g, tip, spent := l.Tip()
			i := uint64(1)
			if damage == "zero" {
				i = 0
			}
			r, raw, e := readLedgerRecordPrivateV3(l.state.root, i)
			if e != nil {
				t.Fatal(e)
			}
			if e := l.Close(); e != nil {
				t.Fatal(e)
			}
			name := filepath.Join(path, ledgerNamePrivateV3(i))
			switch damage {
			case "missing", "zero":
				e = os.Remove(name)
			case "partial":
				e = os.WriteFile(name, []byte("{"), 0600)
			case "noncanonical":
				e = os.WriteFile(name, append([]byte(" "), raw...), 0600)
			case "previous":
				r.Previous = Digest([]byte("wrong"))
				raw, _ = json.Marshal(r)
				e = os.WriteFile(name, raw, 0600)
			case "cumulative":
				r.Cumulative.Work++
				raw, _ = json.Marshal(r)
				e = os.WriteFile(name, raw, 0600)
			}
			if e != nil {
				t.Fatal(e)
			}
			// The supplied tip is the ACTUAL intact terminal, not an old checkpoint tip.
			bootstrap := grant
			current, e := OpenValidatedCurrentLedgerPrivateV3(ctx, path, app, grant, g, tip, &bootstrap)
			if e == nil {
				defer current.Close()
				t.Fatal("ASSERT_NO_FRESH_SOURCE_ON_DAMAGED_CURRENT_CHAIN")
			}
			if bootstrap == grant {
				t.Fatal("ASSERT_BOOTSTRAP_FAILED_WORK_DEBITED")
			}
			calls := 0
			_, e = freezeRetainedCheckpointSourcePrivateV3(ctx, spec, current, grant, scopeReadOps{hook: func(string, string) error { calls++; return nil }})
			if e == nil || calls != 0 {
				t.Fatal("ASSERT_SOURCE_SPY_NEVER_CALLED", calls, e)
			}
			// Neither verification nor refusal appends to the unvalidated journal.
			if _, e = os.Stat(filepath.Join(path, ledgerNamePrivateV3(g+1))); !os.IsNotExist(e) {
				t.Fatal("ASSERT_UNVALIDATED_JOURNAL_NOT_SPENT", e)
			}
			if spent.Work == 0 {
				t.Fatal("setup")
			}
			t.Log("ASSERT_NO_FRESH_SOURCE_ON_DAMAGED_CURRENT_CHAIN: PASS")
		})
	}
}

func TestRetainedCheckpointPrivateV3BootstrapFrontier(t *testing.T) {
	for _, n := range []int{0, 1, 7} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			ctx := context.Background()
			grant := PageCostPrivateV3{1 << 40, 1 << 40, 1 << 34, 1 << 32}
			l, path, app := ledgerFixturePrivateV3(t, grant)
			left := grant
			for i := 0; i < n; i++ {
				if e := l.state.reserve(PageCostPrivateV3{LogicalBytes: 123, Work: 9}, &left); e != nil {
					t.Fatal(e)
				}
			}
			g, tip, spent := l.Tip()
			if e := l.Close(); e != nil {
				t.Fatal(e)
			}
			zero := PageCostPrivateV3{}
			if _, e := OpenValidatedCurrentLedgerPrivateV3(ctx, path, app, grant, g, tip, &zero); !errors.Is(e, ErrPagesPausedPrivateV3) {
				t.Fatal("zero", e)
			}
			dead, cancel := context.WithCancel(ctx)
			cancel()
			budget := grant
			if _, e := OpenValidatedCurrentLedgerPrivateV3(dead, path, app, grant, g, tip, &budget); e == nil || budget != grant {
				t.Fatal("cancel", e)
			}
			current, e := OpenValidatedCurrentLedgerPrivateV3(ctx, path, app, grant, g, tip, &budget)
			if e != nil {
				t.Fatal(e)
			}
			defer current.Close()
			_, _, after := current.Tip()
			if !spent.fits(after) || after == spent || budget == grant {
				t.Fatal("ASSERT_ALL_LATEST_CHARGES_PRESERVED")
			}
			if _, e := json.Marshal(current); e == nil {
				t.Fatal("opaque")
			}
			root, e := filepath.EvalSymlinks(t.TempDir())
			if e != nil {
				t.Fatal(e)
			}
			spec := producerSpecPrivateV2(t, root, 1, 0)
			calls := 0
			if _, e := freezeRetainedCheckpointSourcePrivateV3(ctx, spec, ValidatedCurrentLedgerPrivateV3{PageLedgerPrivateV3: current.PageLedgerPrivateV3}, grant, scopeReadOps{hook: func(string, string) error { calls++; return nil }}); e == nil || calls != 0 {
				t.Fatal("ASSERT_PLAIN_LEDGER_NOT_SOURCE_AUTHORIZATION")
			}
			if _, e := freezeRetainedCheckpointSourcePrivateV3(ctx, spec, current, grant, scopeReadOps{hook: func(string, string) error { calls++; return nil }}); e != nil || calls == 0 {
				t.Fatal("ASSERT_VALID_CHAIN_FRESH_FREEZE", calls, e)
			}
			if _, e := openRetainedTestLedgerPrivateV3(ctx, path, app, grant, g, tip); e == nil {
				t.Fatal("ASSERT_STALE_ACTUAL_TIP_REFUSED")
			}
		})
	}
}
