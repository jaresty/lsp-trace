//go:build darwin || linux

package censuslineagepilot

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"lsp-trace/internal/census"
)

// Reseal every nested policy binding; refusal must not depend on stale hashes.
func terminalRepairPolicyPrivateV2(t *testing.T, v *TerminalDiscoveryInspectionPrivateV2, budget int) {
	t.Helper()
	var spec ScopeSpecPrivateV2
	var manifest scopeManifestPrivateV2
	if json.Unmarshal(v.FrozenSpec, &spec) != nil || json.Unmarshal(v.FrozenManifest, &manifest) != nil {
		t.Fatal("policy setup")
	}
	spec.EffectivePolicy.Discovery.MaxStringBytes = budget
	v.EffectivePolicy, _ = json.Marshal(spec.EffectivePolicy)
	v.PolicySHA256 = Digest(v.EffectivePolicy)
	v.FrozenSpec, _ = json.Marshal(spec)
	manifest.SpecSHA256 = Digest(v.FrozenSpec)
	v.FrozenManifest, _ = json.Marshal(manifest)
	v.ScopeSHA256 = Digest(v.FrozenManifest)
}
func TestTerminalSixRetainedLedgerPrivateV2(t *testing.T) {
	for _, n := range []int{0, 84} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			_, run := terminalFixturePrivateV2(t, n, func(ctx context.Context, f FrozenScopePrivateV2, o TerminalDiscoveryOutcomePrivateV2) {
				var good TerminalDiscoveryInspectionPrivateV2
				if json.Unmarshal(o.state.raw, &good) != nil {
					t.Fatal("setup")
				}
				if _, e := InspectTerminalDiscoveryPrivateV2(o.state.raw); e != nil {
					t.Fatal("accepted control", e)
				}
				t.Log("ASSERT_SIX_LEDGER_CONTROL PASS")
				for _, kind := range []string{"omitted", "document-symbol-failed", "name-budget"} {
					t.Run(kind, func(t *testing.T) {
						var v TerminalDiscoveryInspectionPrivateV2
						if json.Unmarshal(o.state.raw, &v) != nil {
							t.Fatal("setup")
						}
						if kind == "name-budget" {
							terminalRepairPolicyPrivateV2(t, &v, 1)
						} else {
							v.Discovery.Accounting.Files[0].Disposition = census.FileDisposition(kind)
						}
						d, _ := json.Marshal(v.Discovery)
						v.DiscoverySHA256 = Digest(d)
						raw, _ := json.Marshal(v)
						// The perturbed record remains fully canonical and hash consistent.
						if _, e := InspectTerminalDiscoveryPrivateV2(raw); e == nil {
							t.Errorf("ASSERT_SIX_%s_REFUSED FAIL", kind)
						} else {
							t.Logf("ASSERT_SIX_%s_REFUSED PASS: %v", kind, e)
						}
					})
				}
			})
			_, _, _, e := run(false)
			if e != nil {
				t.Fatal(e)
			}
		})
	}
}
