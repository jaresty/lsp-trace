package main

import (
	"lsp-trace/internal/censusacquisition"
	"lsp-trace/internal/censussourcecontinuation"
	"testing"
)

func TestPrivateDiscoveryContinuationTerminalExact(t *testing.T) {
	p := terminalPlanFixture(t, 3)
	d := validClosedTerminalDiscovery(t, p)
	x, e := newPrivateConstituentDiscoveryTerminal(p, d)
	if e != nil {
		t.Fatal(e)
	}
	got, e := privateDiscoveryContinuationTerminal(p, x)
	if e != nil || got.PlanIdentity != p.Identity || got.Authority != 0 || got.Ordinal != 0 || got.Outcome != censussourcecontinuation.OutcomeCompleteNonempty || got.Discovery.Targets[0].CanonicalSeedV2 == nil {
		t.Fatalf("ASSERT_DISCOVERY_RESUME_ADAPTER_EXACT got=%+v err=%v", got, e)
	}
	x.planIdentity = "foreign"
	if _, e = privateDiscoveryContinuationTerminal(p, x); e == nil {
		t.Fatal("ASSERT_DISCOVERY_RESUME_ADAPTER_MISMATCH_REJECTED")
	}
}
func TestPrivateDiscoveryContinuationTerminalNil(t *testing.T) {
	p := terminalPlanFixture(t, 3)
	x := &privateConstituentDiscoveryTerminal{planIdentity: p.Identity, authority: 0, ordinal: 0, members: p.Partitions[0].Members, discovery: censusacquisition.Discovery{}, outcome: privateDiscoveryUnknown}
	got, e := privateDiscoveryContinuationTerminal(p, x)
	if e != nil || got.Discovery.Targets != nil || got.Outcome != censussourcecontinuation.OutcomeUnknown {
		t.Fatalf("ASSERT_DISCOVERY_RESUME_ADAPTER_NIL_PRESERVED got=%+v err=%v", got, e)
	}
}
