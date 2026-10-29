//go:build darwin || linux

package censuslineagepilot_test

import (
	"context"
	"slices"
	"strings"
	"testing"

	pilot "lsp-trace/internal/censuslineagepilot"
	prep "lsp-trace/internal/privatedescribeprep"
)

func TestPostPreparationPrivateV2OwnedAggregateBounds(t *testing.T) {
	// Reuse the specified 5/2 fixture without widening any per-source producer
	// limit. Nested callbacks keep every actual producer root borrowed/alive.
	var collect func(int, []prep.InputPrivateV2)
	collect = func(n int, all []prep.InputPrivateV2) {
		if n < 9 {
			pilot.WithOwnedMatchedProducerPacketPrivateV2(t, 5, 2, false, func(proof pilot.DerivedProducerPacketPrivateV2, _ func() int) {
				collect(n+1, append(slices.Clone(all), inputsV2(t, proof)...))
			})
			return
		}
		ctx := context.Background()
		limits := prepLimitsV2()
		limits.MaxPartBytes = 128 << 10
		limits.MaxTotalBytes = 2 << 20
		// Each source has nine nominated targets and five retained batches.
		eight := all[:40]
		r, err := prep.PreparePrivateV2(ctx, eight, limits)
		if err != nil {
			t.Fatal(err)
		}
		if len(r.Index) != 72 || len(r.Packets) > 64 || len(r.Packets) == 0 || len(r.CanonicalBytes()) > limits.MaxTotalBytes {
			t.Fatal("ASSERT_V2_72_REQUESTED_COMPLETE_BOUNDED_INDEX", len(r.Index), len(r.Packets))
		}
		requested, prepared, sourceGap, resourceGap := 0, 0, 0, 0
		for _, c := range r.Coverage {
			if c.Requested != c.Prepared+c.SourceGap+c.ResourceGap {
				t.Fatal("ASSERT_V2_AGGREGATE_PER_BATCH_ACCOUNTING")
			}
			requested += c.Requested
			prepared += c.Prepared
			sourceGap += c.SourceGap
			resourceGap += c.ResourceGap
		}
		if requested != 72 || prepared != len(r.Packets) || sourceGap != 0 || resourceGap < 8 || requested != prepared+resourceGap {
			t.Fatal("ASSERT_V2_OVER_64_RESOURCE_GAPS_NOT_DROP", requested, prepared, sourceGap, resourceGap)
		}
		h, err := prep.NewPostPreparationPrivateV2(ctx, eight, limits, postLimitsV2())
		if err != nil {
			t.Fatal("ASSERT_V2_EIGHT_PACKET_PAIRS_ADMITTED", err)
		}
		ledger, err := h.Reconcile(ctx, nil, nil)
		if err != nil || len(ledger.Targets) != 72 || len(ledger.Leads) != 0 {
			t.Fatal("ASSERT_V2_AGGREGATE_PENDING_LEDGER", err)
		}
		// Nine real packet pairs require eighteen actual roots: refuse explicitly,
		// never silently omit a producer root to fit the 16-root private profile.
		if _, err := prep.NewPostPreparationPrivateV2(ctx, all, limits, postLimitsV2()); err == nil || !strings.Contains(err.Error(), "protected producer root count") {
			t.Fatal("ASSERT_V2_NINE_PACKET_PAIRS_EXPLICIT_ROOT_BOUND", err)
		}
		t.Log("ASSERT_V2_AGGREGATE_72_TARGETS_16_ROOTS_AND_ONE_OVER: PASS")
	}
	collect(0, nil)
}
