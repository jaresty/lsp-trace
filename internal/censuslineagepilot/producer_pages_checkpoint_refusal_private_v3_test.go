//go:build darwin || linux

package censuslineagepilot

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

type retainedAdmissionFixtureV3 struct {
	p     ProducerPagesPrivateV3
	raw   []byte
	m     RetainedPageCheckpointManifestPrivateV3
	l     ValidatedCurrentLedgerPrivateV3
	fresh FrozenScopePrivateV2
	d     RetainedCheckpointDisclosurePrivateV3
}

func retainedFixtureV3(t *testing.T) retainedAdmissionFixtureV3 {
	t.Helper()
	ctx := context.Background()
	grant := PageCostPrivateV3{1 << 40, 1 << 40, 1 << 34, 1 << 32}
	f, o, c, _ := packetFixtureConfiguredPrivateV2(t, 2, 0, true, "retained-negative-controlled-peer")
	roots, _, _ := packetRootsPrivateV2(t, f)
	l, path, app := ledgerFixturePrivateV3(t, grant)
	p, e := NewProducerPagesPrivateV3(ctx, f, o, c, roots, PagePolicyPrivateV3{16384, 1 << 20, Digest([]byte("negative-config"))}, l)
	if e != nil {
		t.Fatal(e)
	}
	r, _, e := p.Advance(ctx, 1, grant, "")
	if e != nil {
		t.Fatal(e)
	}
	raw, e := p.ExportRetainedCheckpointPrivateV3(ctx, r, Digest([]byte("negative-intent")), grant)
	if e != nil {
		t.Fatal(e)
	}
	var m RetainedPageCheckpointManifestPrivateV3
	if canonicalRetainedPrivateV3(raw, &m) != nil {
		t.Fatal("manifest")
	}
	if e = l.Close(); e != nil {
		t.Fatal(e)
	}
	current, e := openRetainedTestLedgerPrivateV3(ctx, path, app, grant, m.LedgerGeneration, m.LedgerTip)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { _ = current.Close() })
	fresh, e := FreezeRetainedCheckpointSourcePrivateV3(ctx, f.state.spec, current, grant)
	if e != nil {
		t.Fatal(e)
	}
	return retainedAdmissionFixtureV3{p, raw, m, current, fresh, RetainedCheckpointDisclosurePrivateV3{app, m.Revision.Policy.ConfigSHA256, m.Intent, RetainedCheckpointPreparationRevisionPrivateV3}}
}
func (x retainedAdmissionFixtureV3) admit(ctx context.Context, raw []byte, d RetainedCheckpointDisclosurePrivateV3) (RetainedPageCheckpointOwnerPrivateV3, error) {
	return AdmitRetainedPageCheckpointPrivateV3(ctx, raw, x.fresh, x.p.state.roots, x.l, d, x.l.state.allowance)
}
func TestRetainedCheckpointPrivateV3Refusals(t *testing.T) {
	for _, kind := range []string{"config", "application", "intent", "preparation-revision", "noncanonical", "duplicate-key", "unknown-key", "truncated-manifest", "foreign-index", "denominator", "missing-journal-middle", "partial-journal-middle", "source-changed", "corrupt-terminal-page", "foreign-pair-root"} {
		t.Run(kind, func(t *testing.T) {
			x := retainedFixtureV3(t)
			raw := x.raw
			d := x.d
			switch kind {
			case "config":
				d.ConfigSHA256 = Digest([]byte("different"))
			case "application":
				d.Application = Digest([]byte("different"))
			case "intent":
				d.ContinuationIntent = Digest([]byte("different"))
			case "preparation-revision":
				d.PreparationRevision = "other-preparation"
			case "noncanonical":
				raw = append([]byte(" "), raw...)
			case "duplicate-key":
				raw = append([]byte(`{"Schema":"x",`), raw[1:]...)
			case "unknown-key":
				raw = append([]byte(`{"foreign":true,`), raw[1:]...)
			case "truncated-manifest":
				raw = raw[:len(raw)-1]
			case "foreign-index":
				m := x.m
				m.Index = Digest([]byte("foreign"))
				raw, _ = json.Marshal(m)
			case "denominator":
				m := x.m
				m.Revision.BatchDenominator++
				raw, _ = json.Marshal(m)
			case "missing-journal-middle":
				if e := os.Remove(filepath.Join(x.l.state.path, ledgerNamePrivateV3(1))); e != nil {
					t.Fatal(e)
				}
			case "partial-journal-middle":
				if e := os.WriteFile(filepath.Join(x.l.state.path, ledgerNamePrivateV3(1)), []byte("{"), 0600); e != nil {
					t.Fatal(e)
				}
			case "source-changed":
				if e := os.WriteFile(filepath.Join(x.fresh.state.spec.Root, x.fresh.state.spec.Members[0].CanonicalRelativePath), bytes.Repeat([]byte("x"), 1<<20), 0600); e != nil {
					t.Fatal(e)
				}
			case "corrupt-terminal-page":
				if e := os.WriteFile(filepath.Join(x.m.Roots[0].Path, x.m.Revision.Cursor.Last), []byte("corrupt"), 0600); e != nil {
					t.Fatal(e)
				}
			case "foreign-pair-root":
				roots, _, _ := packetRootsPrivateV2(t, x.fresh)
				x.p.state.roots = roots
			}
			_, _, before := x.l.Tip()
			if _, e := x.admit(context.Background(), raw, d); e == nil {
				t.Fatal("ASSERT_RETAINED_REFUSAL", kind)
			}
			_, _, after := x.l.Tip()
			if !before.fits(after) || before == after {
				t.Fatal("ASSERT_RETAINED_FAILED_ADMISSION_CHARGED", kind)
			}
			t.Log("ASSERT_RETAINED_REFUSAL: PASS", kind)
		})
	}
}
func TestRetainedCheckpointPrivateV3ContinuationAndOpaqueGuards(t *testing.T) {
	x := retainedFixtureV3(t)
	ctx := context.Background()
	grant := x.l.state.allowance
	owner, e := x.admit(ctx, x.raw, x.d)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = json.Marshal(owner); e == nil {
		t.Fatal("ASSERT_RETAINED_OWNER_OPAQUE")
	}
	// Copy the capability-bearing fields with a fresh lock; the copied self
	// identity must still refuse. Copying sync.Mutex itself is invalid Go usage.
	copied := retainedCheckpointStatePrivateV3{self: owner.state, manifest: owner.state.manifest, checkpoint: owner.state.checkpoint, frozen: owner.state.frozen, roots: owner.state.roots, ledger: owner.state.ledger}
	if _, e = (RetainedPageCheckpointOwnerPrivateV3{&copied}).SelectedObject(ctx, 0, grant); e == nil {
		t.Fatal("ASSERT_RETAINED_SELF_GUARD")
	}
	before := owner.BudgetSpent()
	if _, e = owner.SelectedBatch(ctx, 0, PageCostPrivateV3{}); !errors.Is(e, ErrPagesPausedPrivateV3) {
		t.Fatal("ASSERT_RETAINED_ZERO_BUDGET_PAUSE", e)
	}
	if owner.BudgetSpent() != before {
		t.Fatal("ASSERT_RETAINED_DENIED_RESERVATION_NO_RESET")
	}
	dead, cancel := context.WithCancel(ctx)
	cancel()
	if _, e = owner.SelectedBatch(dead, 0, grant); e == nil {
		t.Fatal("ASSERT_RETAINED_DEAD_CONTEXT")
	}
	b, e := owner.SelectedBatch(ctx, 0, grant)
	if e != nil {
		t.Fatal(e)
	}
	v, e := b.Read(ctx, grant)
	if e != nil {
		t.Fatal(e)
	}
	if v.ObjectSHA256[2] != Digest(v.SourceArtifact) {
		t.Fatal("ASSERT_RETAINED_EXACT_SELECTED_OBJECT")
	}
	var dto RetainedSelectedBatchPrivateV3
	if json.Unmarshal(v.SourceArtifact, &dto) != nil {
		t.Fatal("DTO decode")
	}
	if _, e = dto.Read(ctx, grant); e == nil {
		t.Fatal("ASSERT_RETAINED_DTO_CANNOT_MINT_OWNER")
	}
	if _, e = b.OwnSource(ctx, "file:///not-authorized.go", grant); e == nil {
		t.Fatal("ASSERT_RETAINED_NO_SOURCE_FALLBACK")
	}
	raw, e := owner.CheckpointCompletedBatch(ctx, b, Digest([]byte("next")), grant)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = owner.CheckpointCompletedBatch(ctx, b, Digest([]byte("duplicate")), grant); e == nil {
		t.Fatal("ASSERT_RETAINED_DUPLICATE_COMPLETED_UNIT")
	}
	var next RetainedPageCheckpointManifestPrivateV3
	if canonicalRetainedPrivateV3(raw, &next) != nil || next.NextBatch != 1 {
		t.Fatal("ASSERT_RETAINED_DURABLE_SELECTED_CURSOR")
	}
	// Reopen with the ACTUAL latest tip. Already spent work remains; old checkpoint
	// continuation still refuses because its exclusive claim record already exists.
	g, tip, spent := x.l.Tip()
	if e = x.l.Close(); e != nil {
		t.Fatal(e)
	}
	reopened, e := openRetainedTestLedgerPrivateV3(ctx, x.l.state.path, x.m.Application, grant, g, tip)
	if e != nil {
		t.Fatal(e)
	}
	defer reopened.Close()
	if _, e = AdmitRetainedPageCheckpointPrivateV3(ctx, x.raw, x.fresh, x.p.state.roots, reopened, x.d, grant); e == nil {
		t.Fatal("ASSERT_RETAINED_CONTINUATION_REPLAY")
	}
	_, _, after := reopened.Tip()
	if !spent.fits(after) {
		t.Fatal("ASSERT_RETAINED_NO_REFUND_ON_REPLAY")
	}
	if _, e = OpenPageLedgerPrivateV3(ctx, x.l.state.path, x.m.Application, grant, x.m.LedgerGeneration, x.m.LedgerTip); e == nil {
		t.Fatal("ASSERT_RETAINED_STALE_TIP")
	}
}
func TestRetainedCheckpointPrivateV3UncheckpointedChargesAndTornTail(t *testing.T) {
	for _, torn := range []bool{false, true} {
		t.Run(map[bool]string{false: "preserve-latest-charge", true: "torn-tail-refuses"}[torn], func(t *testing.T) {
			x := retainedFixtureV3(t)
			ctx := context.Background()
			grant := x.l.state.allowance
			left := grant
			if e := x.l.state.reserve(PageCostPrivateV3{LogicalBytes: 12345, ReadHashBytes: 67890, Work: 99}, &left); e != nil {
				t.Fatal(e)
			}
			g, tip, spent := x.l.Tip()
			if e := x.l.Close(); e != nil {
				t.Fatal(e)
			}
			if torn {
				if e := os.WriteFile(filepath.Join(x.l.state.path, ledgerNamePrivateV3(g)), []byte("{"), 0600); e != nil {
					t.Fatal(e)
				}
			}
			l, e := openRetainedTestLedgerPrivateV3(ctx, x.l.state.path, x.m.Application, grant, g, tip)
			if torn {
				if e == nil {
					l.Close()
					t.Fatal("ASSERT_RETAINED_TORN_TAIL_REFUSED")
				}
				return
			}
			if e != nil {
				t.Fatal(e)
			}
			defer l.Close()
			owner, e := AdmitRetainedPageCheckpointPrivateV3(ctx, x.raw, x.fresh, x.p.state.roots, l, x.d, grant)
			if e != nil {
				t.Fatal("ASSERT_RETAINED_RESUME_LAST_DATA_WITH_LATEST_CHARGE", e)
			}
			if !spent.fits(owner.BudgetSpent()) || owner.state.manifest.NextBatch != 0 {
				t.Fatal("ASSERT_RETAINED_UNCHECKPOINTED_COST_NOT_PROGRESS")
			}
		})
	}
}
