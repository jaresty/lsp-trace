package privatedescribeprep

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

// This helper tests the cohort-specific correspondence seam independently of
// the full publication gate. The mechanical fixture is NOT a qualified source
// and is never claimed to pass the separate preparation/source validator.
func checkOwnedCohortObservationTest(t *testing.T, h PostPreparationPrivateV2, a AutomaticResultPrivateV2) {
	t.Helper()
	var p Result
	if e := json.Unmarshal(h.state.prepared, &p); e != nil {
		t.Fatal(e)
	}
	ledger, e := h.Reconcile(context.Background(), a.submissions, a.reviews)
	if e != nil {
		t.Fatal(e)
	}
	if e = observationCohortPrivateV2(context.Background(), p, a.Inventory(), ledger, h.state.limits); e != nil {
		t.Fatal("owned cohort correspondence", e)
	}
}

func TestCohortObservationRevisionNavigationGuards(t *testing.T) {
	h := automaticTestOwner(t, 6)
	i, v := cohortTestPair()
	a, e := AutomateCohortPrivateV2(context.Background(), h, cohortTestLimits(), 6, i, v)
	if e != nil {
		t.Fatal(e)
	}
	checkOwnedCohortObservationTest(t, h, a)
	var p Result
	if e = json.Unmarshal(h.state.prepared, &p); e != nil {
		t.Fatal(e)
	}
	ledger, e := h.Reconcile(context.Background(), a.submissions, a.reviews)
	if e != nil {
		t.Fatal(e)
	}
	for _, kind := range []string{"candidate-revision", "candidate-claims", "row-revision", "navigation-omission", "navigation-review", "lead-claims", "context", "physical-declared-only"} {
		t.Run(kind, func(t *testing.T) {
			inv := a.Inventory()
			l := postClone(ledger)
			ctx := context.Background()
			want := ""
			switch kind {
			case "candidate-revision":
				inv.Candidates[0].RevisionDigest = "foreign"
				want = "candidate revision correspondence"
			case "candidate-claims":
				inv.Candidates[0].Claims[0].Text = "foreign"
				want = "candidate revision correspondence"
			case "row-revision":
				inv.Rows[0].Revisions[0].ResponseSHA256 = "foreign"
				want = "row revisions correspondence"
			case "navigation-omission":
				inv.NavigationRevisions = nil
				want = "navigation revisions bounds"
			case "navigation-review":
				inv.NavigationRevisions[0].Review.Digest = "foreign"
				want = "navigation review binding"
			case "lead-claims":
				l.Leads[0].Submission.Claims = nil
				l.Targets[0].Submission.Claims = nil
				want = "lead approved correspondence"
			case "context":
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
				want = "context canceled"
			case "physical-declared-only":
				// A well-shaped but unverified physical hash is not authenticated by this
				// checker. It cannot compare bytes that the creator wire does not retain.
				for j := range inv.Cohort.Frames {
					f := &inv.Cohort.Frames[j]
					f.PhysicalResponseSHA256 = postPublicationDigest([]byte("declared only"))
					f.PhysicalResponseByteLength = f.ResponseByteLength
				}
			}
			err := observationCohortPrivateV2(ctx, p, inv, l, h.state.limits)
			if want == "" {
				if err != nil {
					t.Fatal(err)
				}
				t.Log("ASSERT_PHYSICAL_HASH_DECLARATION_NOT_AUTHENTICATION: PASS")
				return
			}
			if err == nil || !strings.Contains(err.Error(), want) {
				t.Fatalf("assertion-specific guard %s: want %q got %v", kind, want, err)
			}
			t.Logf("ASSERT_COHORT_PURE_GUARD_%s: PASS %v", kind, err)
		})
	}
}
