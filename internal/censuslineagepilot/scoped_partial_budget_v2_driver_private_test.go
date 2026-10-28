//go:build darwin || linux

package censuslineagepilot_test

import (
	"context"
	"encoding/json"
	pilot "lsp-trace/internal/censuslineagepilot"
	"testing"
)

func TestScopedPartialBudgetV2DriverPrivate(t *testing.T) {
	h, m, cfg, f := pilot.NewScopedPartialBudgetV2RehearsalFixturePrivate(t)
	// Old full-purpose driver refuses v4 before readiness or semantic traffic.
	if r, e := executeScopedRehearsalPrivate(context.Background(), h, m, cfg); e == nil || r.AcquireCalls != 0 || len(r.Records) != 0 {
		t.Fatal("ASSERT_FULL_V4_REFUSAL")
	}
	if sem, prep := f.Traffic(); sem != 0 || prep != 0 {
		t.Fatal("ASSERT_FULL_V4_ZERO_TRAFFIC")
	}
	f.Fault("rehearsal-empty-directions")
	r, e := executeScopedPartialRehearsalPrivate(context.Background(), h, m, cfg)
	if e != nil || r.Outcome != "PARTIAL_INSPECTION_RETAINED" || r.AcquireCalls != 1 || r.Constituents != 0 || r.Authority != 0 || r.Accepted || len(r.Records) != 7 || r.OutcomeReceipt == nil || !r.OutcomeReceipt.Verified {
		t.Fatalf("ASSERT_PARTIAL_V4_DRIVER: %+v %v", r, e)
	}
	var budget struct {
		Output                                                   pilot.ScopedRehearsalBudgetPrivate
		ResponseBytes, LifetimeSeconds, RequestSeconds, Requests int
	}
	raw, encodeErr := json.Marshal(r.EffectiveBudget)
	if encodeErr != nil || json.Unmarshal(raw, &budget) != nil {
		t.Fatal("budget observation encoding")
	}
	if r.DiscoveryPolicy == nil || r.DiscoveryPolicy.Name != "one-partial-inspection-private-v2" || budget.Output != pilot.ScopedRehearsalBudgetPartialV2Private() || budget.ResponseBytes != 2<<20 || budget.LifetimeSeconds != 30 || budget.RequestSeconds != 5 || budget.Requests != 256 {
		t.Fatal("ASSERT_PARTIAL_V4_EFFECTIVE_PROFILE", r)
	}
	for _, rec := range r.Records {
		if !rec.Verified {
			t.Fatal("ASSERT_V4_PAIRED_RECORD", rec)
		}
	}
	t.Log("ASSERT_PARTIAL_V4_DRIVER: PASS; records=8; output-object=4194304; response=2097152; core_admitted_constituents=0")
}
