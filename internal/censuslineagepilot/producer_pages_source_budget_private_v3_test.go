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

// Calculate the independent reservation floor from the original public spec,
// not sourceCheckCost. This is an accounting assertion against actual public
// operations and their durable ledger, not a mock reader or cost-function test.
func sourcePolicyFloorPrivateV3(t *testing.T, p ProducerPagesPrivateV3) (ScopeSpecPrivateV2, PageCostPrivateV3) {
	t.Helper()
	var spec ScopeSpecPrivateV2
	if e := json.Unmarshal(p.state.frozen.SpecBytes(), &spec); e != nil {
		t.Fatal(e)
	}
	members := uint64(len(spec.Members))
	maximum := uint64(spec.EffectivePolicy.TotalBytes)
	if files := members * uint64(spec.EffectivePolicy.FileBytes); files < maximum {
		maximum = files
	}
	// One sentinel per member conservatively bounds growth at the held descriptor;
	// the unchanged reader's shared-total limit can also reject an earlier prefix.
	unit := maximum + members + scopeMetadataPrivateV2
	return spec, PageCostPrivateV3{LogicalBytes: 24 * unit, ReadHashBytes: 32 * unit, Work: 2 * producerPacketTopologyWorkPrivateV2}
}

func TestProducerPagesPrivateV3GrownSourceReservations(t *testing.T) {
	for _, operation := range []string{"advance", "verify", "verifier", "batch", "batch-read", "resume"} {
		t.Run(operation, func(t *testing.T) {
			ctx := context.Background()
			p := pageOwnerPrivateV3(t, 16384)
			revision, id, e := p.Advance(ctx, 1, pageAllowancePrivateV3(), "")
			if e != nil {
				t.Fatal(e)
			}
			batch, e := p.Batch(ctx, revision, 0, pageAllowancePrivateV3())
			if e != nil {
				t.Fatal(e)
			}
			verifier, e := p.PrefixVerifier(revision)
			if e != nil {
				t.Fatal(e)
			}
			var resumed ProducerPagesPrivateV3
			if operation == "resume" {
				resumed = renewedOriginalPagesPrivateV3(t, p)
			}
			spec, floor := sourcePolicyFloorPrivateV3(t, p)
			grownBytes := spec.EffectivePolicy.FileBytes / 2
			if spec.Members[0].ByteLength >= uint64(grownBytes) || grownBytes > spec.EffectivePolicy.TotalBytes {
				t.Fatal("ASSERT_SMALL_FROZEN_LARGE_POLICY_FIXTURE")
			}
			// Only this disposable owned source is changed. These bytes are NOT admitted
			// source permission, a new capture, or a replacement success owner.
			file := filepath.Join(spec.Root, spec.Members[0].CanonicalRelativePath)
			if e = os.WriteFile(file, bytes.Repeat([]byte{'g'}, grownBytes), 0600); e != nil {
				t.Fatal(e)
			}
			for attempt := uint64(1); attempt <= 2; attempt++ {
				before := p.BudgetSpent()
				switch operation {
				case "advance":
					var r PageRevisionPrivateV3
					var published string
					r, published, e = p.Advance(ctx, attempt+1, pageAllowancePrivateV3(), id)
					if e == nil || published != "" || r.Status != "FAILED" {
						t.Fatal("ASSERT_CHANGED_SOURCE_NO_NEW_REVISION", r.Status, published, e)
					}
				case "verify":
					e = p.VerifyPrefix(ctx, revision, pageAllowancePrivateV3())
				case "verifier":
					var progress PageVerificationProgressPrivateV3
					progress, e = verifier.Advance(ctx, attempt, pageAllowancePrivateV3())
					if progress.Status != "FAILED" || progress.PagesVerified != 0 {
						t.Fatal("ASSERT_CHANGED_SOURCE_NOT_VERIFIED", progress)
					}
				case "batch":
					var out OriginalPageBatchPrivateV3
					out, e = p.Batch(ctx, revision, 0, pageAllowancePrivateV3())
					if out.state != nil {
						t.Fatal("ASSERT_CHANGED_SOURCE_NO_QUALIFIED_BATCH")
					}
				case "batch-read":
					var out OriginalPageBatchViewPrivateV3
					out, e = batch.Read(ctx, pageAllowancePrivateV3())
					if len(out.SourceArtifact) != 0 || len(out.NativeGraph) != 0 {
						t.Fatal("ASSERT_CHANGED_SOURCE_NO_BATCH_BYTES")
					}
				case "resume":
					_, e = resumed.ResumeOriginalCheckpoint(ctx, 1, pageAllowancePrivateV3())
					if resumed.state.generation != 0 {
						t.Fatal("ASSERT_CHANGED_SOURCE_NO_RESUME_ADMISSION")
					}
				}
				if e == nil || errors.Is(e, ErrPagesPausedPrivateV3) {
					t.Fatal("ASSERT_GROWN_SOURCE_REJECTED_AFTER_ADMITTED_CHECK", e)
				}
				after := p.BudgetSpent()
				if !before.fits(after) || !floor.fits(after.minus(before)) {
					t.Fatalf("ASSERT_GROWN_SOURCE_POLICY_RESERVATION: op=%s attempt=%d grown=%d floor=%+v delta=%+v", operation, attempt, grownBytes, floor, after.minus(before))
				}
				t.Logf("GROWN_SOURCE_RESERVED: op=%s attempt=%d members=%d file_policy=%d total_policy=%d grown=%d floor=%+v delta=%+v", operation, attempt, len(spec.Members), spec.EffectivePolicy.FileBytes, spec.EffectivePolicy.TotalBytes, grownBytes, floor, after.minus(before))
			}
		})
	}
}

func TestProducerPagesPrivateV3GrownSourceExhaustsWithoutReset(t *testing.T) {
	ctx := context.Background()
	p := pageOwnerPrivateV3(t, 16384)
	first, id, e := p.Advance(ctx, 1, pageAllowancePrivateV3(), "")
	if e != nil {
		t.Fatal(e)
	}
	spec, floor := sourcePolicyFloorPrivateV3(t, p)
	if e = os.WriteFile(filepath.Join(spec.Root, spec.Members[0].CanonicalRelativePath), bytes.Repeat([]byte{'g'}, spec.EffectivePolicy.FileBytes/2), 0600); e != nil {
		t.Fatal(e)
	}
	// The original explicit allowance is not increased. A finite number of failed
	// original-owner checks must exhaust it; each admitted failure is nonrefundable.
	maximum := pageAllowancePrivateV3().LogicalBytes/floor.LogicalBytes + 2
	for attempt := uint64(2); attempt <= maximum; attempt++ {
		before := p.BudgetSpent()
		generation := p.state.generation
		r, published, err := p.Advance(ctx, attempt, pageAllowancePrivateV3(), id)
		after := p.BudgetSpent()
		if errors.Is(err, ErrPagesPausedPrivateV3) {
			if after != before || p.state.generation != generation || published != "" || r.Cursor != first.Cursor {
				t.Fatal("ASSERT_EXHAUSTED_SOURCE_RESERVATION_BEFORE_WORK", r, err)
			}
			t.Logf("GROWN_SOURCE_APPLICATION_PAUSED: admitted_failures=%d cumulative=%+v", attempt-2, after)
			return
		}
		if err == nil || published != "" || !before.fits(after) || !floor.fits(after.minus(before)) {
			t.Fatal("ASSERT_REPEATED_SOURCE_FAILURE_CHARGED", err, before, after)
		}
	}
	t.Fatal("ASSERT_NO_APPLICATION_LEDGER_RESET_AFTER_FAILED_SOURCE_CHECKS")
}

func TestProducerPagesPrivateV3PolicyReservationPausePreservesPrefix(t *testing.T) {
	ctx := context.Background()
	p := pageOwnerPrivateV3(t, 4096)
	first, id, e := p.Advance(ctx, 1, onePageGrantPrivateV3(t, p), "")
	if !errors.Is(e, ErrPagesPausedPrivateV3) || id == "" {
		t.Fatal(e)
	}
	_, source := sourcePolicyFloorPrivateV3(t, p)
	grant, _ := pagePairCostPrivateV3(producerPageHeaderPrivateV3).plus(pageJournalCostPrivateV3)
	grant, _ = grant.plus(pageLedgerOverheadPrivateV3)
	grant, _ = grant.plus(source)
	// This exchange is exactly one logical byte short of the policy-max source
	// reservation. No allowance is increased and the application ledger is shared.
	grant.LogicalBytes--
	before := p.BudgetSpent()
	generation := p.state.generation
	paused, newID, e := p.Advance(ctx, 2, grant, id)
	if !errors.Is(e, ErrPagesPausedPrivateV3) || newID != "" || paused.Cursor != first.Cursor || p.BudgetSpent() != before || p.state.generation != generation {
		t.Fatal("ASSERT_POLICY_RESERVATION_BEFORE_SOURCE_WORK_PRESERVES_PREFIX", e, newID, paused)
	}
	if e = p.VerifyPrefix(ctx, first, pageAllowancePrivateV3()); e != nil {
		t.Fatal("ASSERT_PRIOR_ADMITTED_PREFIX_UNCHANGED", e)
	}
}
