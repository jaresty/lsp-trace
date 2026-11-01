//go:build darwin || linux

package privatehostproductv3

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"
)

func TestCohortNavigationExactReviewedRevisions(t *testing.T) {
	c, items := cohortInputs(t)
	r, e := RunCohort(context.Background(), c, items, &cohortFixture{})
	if e != nil {
		t.Fatal(e)
	}
	c.Navigation = true
	task := bindNavigation(c, navigationCandidates(r.Outcomes), []CohortNavigation{}, "interpreter", 5)
	raw, e := navigationFixture(mustJSON(task))
	if e != nil {
		t.Fatal(e)
	}
	response, e := DecodeCohortNavigation(raw)
	if e != nil || validateCohortNavigation(task, response) != nil {
		t.Fatal("ASSERT_NAV_VALID_EXACT_REVISIONS", e)
	}
	for _, kind := range []string{"foreign", "revision", "digest", "duplicate", "omitted", "mixed", "alias", "escaped"} {
		t.Run(kind, func(t *testing.T) {
			response, _ := DecodeCohortNavigation(raw)
			changedTask := task
			switch kind {
			case "foreign":
				response.Groups[0].Members[0].ItemID = "other-source-item"
			case "revision":
				response.Groups[0].Members[0].Revision++
			case "digest":
				response.Groups[0].Members[0].ReviewSHA256 = hash([]byte("foreign review"))
			case "duplicate":
				response.Groups[0].Members[1] = response.Groups[0].Members[0]
			case "omitted":
				changedTask.Role = "reviewer"
				changedTask.Proposal = response.Groups
				response.Groups = []CohortNavigation{}
			case "mixed":
				response.Protocol = CohortProtocol
			case "alias":
				payload := bytes.Replace(raw, []byte(`"Supported":`), []byte(`"supported":false,"Supported":`), 1)
				if _, e := DecodeCohortNavigation(payload); e == nil {
					t.Fatal("ASSERT_NAV_STRICT_RECURSIVE_KEYS")
				}
				return
			case "escaped":
				payload := bytes.Replace(raw, []byte(`"ItemID":`), []byte(`"Item\u0049D":"foreign","ItemID":`), 1)
				if _, e := DecodeCohortNavigation(payload); e == nil {
					t.Fatal("ASSERT_NAV_STRICT_RECURSIVE_KEYS")
				}
				return
			}
			if validateCohortNavigation(changedTask, response) == nil {
				t.Fatal("ASSERT_NAV_FOREIGN_OR_OMITTED_REVIEW_REFUSED")
			}
		})
	}
}
func TestCohortNavigationUnsupportedAndNoNewGrant(t *testing.T) {
	c, items := cohortInputs(t)
	c.Navigation = true
	f := &cohortFixture{}
	r, e := RunCohort(context.Background(), c, items, cohortBackendFunc(func(ctx context.Context, raw []byte) ([]byte, error) {
		var h struct{ Protocol, Role string }
		json.Unmarshal(raw, &h)
		if h.Protocol != CohortNavigationProtocol {
			return f.ExchangeCohort(ctx, raw)
		}
		out, e := navigationFixture(raw)
		if h.Role == "reviewer" {
			var r CohortNavigationResponse
			json.Unmarshal(out, &r)
			r.Groups[0].Supported = false
			out = mustJSON(r)
		}
		return out, e
	}))
	if e != nil || r.Reviewed != 20 || r.Calls != 6 || r.NavigationHistory[1].Groups[0].Supported {
		t.Fatal("ASSERT_NAV_REVIEWER_UNRESOLVED_NOT_FEATURE_ACCEPTED", e)
	}
	if _, e = LatestCohort(c); e != nil {
		t.Fatal("ASSERT_NAV_UNSUPPORTED_HISTORY", e)
	}
	c, items = cohortInputs(t)
	c.Navigation = true
	c.MaxCalls = 4
	f = &cohortFixture{}
	r, e = RunCohort(context.Background(), c, items, f)
	if e != nil || r.Reviewed != 20 || r.Calls != 4 || r.NavigationStatus != "UNAVAILABLE_REMAINING_GRANT" {
		t.Fatal("ASSERT_NAV_NO_SILENT_GRANT_RESET", r.NavigationStatus, e)
	}
}
