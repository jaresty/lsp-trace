//go:build darwin || linux

package privatehostproductv3

import (
	"context"
	"encoding/json"
	"testing"
)

func navigationFixture(raw []byte) ([]byte, error) {
	var task CohortNavigationTask
	if e := json.Unmarshal(raw, &task); e != nil {
		return nil, e
	}
	groups := task.Proposal
	if task.Role == "interpreter" {
		groups = []CohortNavigation{{ID: "nav-1", Label: "Reversible navigation", Explanation: "Predetermined grouping hypothesis only", Alternative: "Keep these items separate", Falsifier: "Responsibilities differ", Withdrawal: "Withdraw when falsified", Revisit: "Revisit after stakeholder correction", Members: []CohortRevision{task.Candidates[0].Revision, task.Candidates[1].Revision}, Supported: true}}
	}
	return json.Marshal(CohortNavigationResponse{Protocol: CohortNavigationProtocol, RequestDigest: task.RequestDigest, Groups: groups})
}
func TestCohortNavigationSameSixteenGrant(t *testing.T) {
	c, items := cohortInputs(t)
	c.Navigation = true
	f := &cohortFixture{reject: true}
	r, e := RunCohort(context.Background(), c, items, cohortBackendFunc(func(ctx context.Context, raw []byte) ([]byte, error) {
		var header struct{ Protocol string }
		json.Unmarshal(raw, &header)
		if header.Protocol == CohortNavigationProtocol {
			return navigationFixture(raw)
		}
		return f.ExchangeCohort(ctx, raw)
	}))
	if e != nil || r.Reviewed != 20 || r.Calls != 8 || len(r.NavigationHistory) != 2 || len(r.NavigationHistory[1].Groups) != 1 || !r.NavigationHistory[1].Groups[0].Supported {
		t.Fatalf("ASSERT_COHORT_REVIEWED_NAV_SAME_GRANT: calls=%d reviewed=%d histories=%d err=%v", r.Calls, r.Reviewed, len(r.NavigationHistory), e)
	}
	if _, e = LatestCohort(c); e != nil {
		t.Fatal("ASSERT_COHORT_NAV_PHYSICAL_HISTORY", e)
	}
}
