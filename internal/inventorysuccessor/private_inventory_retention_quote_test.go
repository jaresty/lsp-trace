//go:build darwin || linux

package inventorysuccessor

import (
	"context"
	"testing"
	"time"

	rg "lsp-trace/internal/referencegrouping"
	"lsp-trace/sessionruntime"
)

// Diagnostic only: these are the existing M/P/S child formulas, not a new
// complete profile. Selected bootstrap/control and caller retention are absent.
// In particular no initialization, native authorization or provider is invoked.
func TestInventoryRetentionChildQuoteNotGrant(t *testing.T) {
	now := time.Unix(1, 0)
	deadline := now.Add(15 * time.Second)
	m, e := privateRetainedMixedQuoteV1(now, deadline)
	if e != nil {
		t.Fatal(e)
	}
	p, e := selectedPreparationChargeV2(rg.AllModeMetadataPrivate{Input: rg.CompositionCommitment{ByteLength: 1 << 20}, Contributions: 1024, Custody: 2048, Predecessors: 1024}, now, deadline)
	if e != nil {
		t.Fatal(e)
	}
	// Largest accepted artifact/parent inputs to the existing S quote, not
	// an attestation that a selected contract has been constructed or admitted.
	s, e := selectedSourceChargeV2(1024, 64<<20, 64<<20, now, deadline)
	if e != nil {
		t.Fatal(e)
	}
	sum, e := addCountersV2(m, p)
	if e != nil {
		t.Fatal(e)
	}
	sum, e = addCountersV2(sum, s)
	if e != nil {
		t.Fatal(e)
	}
	for i, n := range sum.vector() {
		if n != m.vector()[i]+p.vector()[i]+s.vector()[i] {
			t.Fatal("CHILD_QUOTE_CONSERVATION", i)
		}
	}
	// M retains its existing one-second cleanup allowance even without a
	// provider. Model ActiveNS sums do not renew the shared original deadline.
	if sum.ProviderRequests != 0 || sum.ProviderMessages != 0 || sum.WireBytes != 0 || sum.ActiveNS != uint64(3*15*time.Second+time.Second) {
		t.Fatal("CHILD_QUOTE_NOT_PROVIDER_OR_RENEWAL")
	}
	discovery := NewInventoryNativeProfileV1()
	subtotal, e := addCountersV2(countersFromV2(discovery.Allocation), sum)
	if e != nil {
		t.Fatal(e)
	}
	two, e := addCountersV2(subtotal, subtotal)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = OpenInventoryFundedDiscoveryV1(context.Background(), InventoryNativePermitV1{}, sessionruntime.PrivateDiscoveryConstruction{}); e == nil {
		t.Fatal("QUOTE_MUST_NOT_MINT_GRANT")
	}
	t.Logf("INCOMPLETE_NATIVE_SUBTOTAL M=%v P=%v S=%v MPS=%v two_discovery_plus_two_MPS=%v; excludes selected bootstrap/control and caller retention/admission; no native initialization", m.vector(), p.vector(), s.vector(), sum.vector(), two.vector())
}
