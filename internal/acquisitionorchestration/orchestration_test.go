package acquisitionorchestration

import "testing"

func TestCustodyPoliciesAreDistinctAndClosed(t *testing.T) {
	cases := []struct {
		route                         string
		caller, retain, prepareSource bool
	}{
		{RouteExplicitTrace, true, false, true},
		{RouteSeedFile, true, false, false},
		{RouteAutomaticFile, true, true, false},
		{RouteLegacyManifest, false, false, false},
	}
	seen := map[string]bool{}
	for _, tc := range cases {
		policy, ok := policyForRoute(tc.route)
		if !ok || seen[tc.route] || policy.callerLocal != tc.caller || policy.retainSeedSpec != tc.retain || policy.prepareSource != tc.prepareSource {
			t.Fatalf("ASSERT_FOUR_MODE_CUSTODY_POLICY_%s: ok=%t policy=%+v", tc.route, ok, policy)
		}
		seen[tc.route] = true
	}
	if _, ok := policyForRoute("ordinary-acquisition"); ok {
		t.Fatal("ASSERT_ORDINARY_ACQUISITION_HAS_NO_TRUSTED_ROUTE_POLICY")
	}
}
