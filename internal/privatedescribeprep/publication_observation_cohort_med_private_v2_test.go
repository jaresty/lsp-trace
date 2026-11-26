//go:build darwin || linux

package privatedescribeprep

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

// Retain through the original opaque creator continuation, then decode ONLY
// detached observations. The mechanical preparation is not source qualification;
// these tests target the cohort seam, not the separate source admission gate.
func retainedCohortMEDFixture(t *testing.T, mode string) (Result, postPublicationEnvelopePrivateV2, PostPreparationLimits, []CohortRequestPrivateV2) {
	t.Helper()
	h := automaticTestOwner(t, 4)
	if mode == "gaps" {
		// Construct a fresh original mechanical fixture with no prepared packets;
		// never restore an owner from retained native observations.
		r := mechanicalPreparation(0)
		r.RequestType = RequestTypePrivateV2
		r.Coverage[0].OriginalTargets, r.Coverage[0].Requested = 2, 2
		r.Coverage[0].SourceGap, r.Coverage[0].ResourceGap = 1, 1
		r.Coverage[0].Selection = []string{"source-gap", "resource-gap"}
		for j, status := range []string{"SOURCE_GAP", "RESOURCE_GAP"} {
			r.Index = append(r.Index, TargetIndex{InputSHA256: r.Coverage[0].InputSHA256, Target: r.Coverage[0].Selection[j], Status: status, Gap: "CONTROLLED_GAP", OriginalOrdinal: j})
		}
		original, err := postFromPreparedProfile(context.Background(), r, mechanicalLimits(), RequestTypePrivateV2)
		if err != nil {
			t.Fatal(err)
		}
		h = PostPreparationPrivateV2{state: original.state}
	}
	outs, ins := postPublicationRoots(t)
	_, other := postPublicationRoots(t)
	ins = append(ins, other[0])
	for _, root := range ins {
		path, id, e := root.CanonicalIdentityPrivateV2()
		if e != nil {
			t.Fatal(e)
		}
		h.roots = append(h.roots, producerRootBindingPrivateV2{path, id.Device, id.Inode})
	}
	i, v := cohortTestPair()
	oldI, oldV := i.fn, v.fn
	var requests []CohortRequestPrivateV2
	i.fn = func(r CohortRequestPrivateV2) ([]byte, error) {
		requests = append(requests, postClone(r))
		raw, e := oldI(r)
		if mode == "mixed" && r.Navigation == nil {
			var out CohortResponsePrivateV2
			if json.Unmarshal(raw, &out) != nil {
				t.Fatal("interpret fixture")
			}
			for j := range out.Items {
				if r.Items[j].Target == "target-3" {
					out.Items[j].Interpretation = &AutomaticInterpretationPrivateV2{Disposition: "UNSUPPORTED", Limitations: "Controlled unsupported own body"}
				}
			}
			raw = automaticTestEncode(out)
		}
		return raw, e
	}
	v.fn = func(r CohortRequestPrivateV2) ([]byte, error) {
		requests = append(requests, postClone(r))
		raw, e := oldV(r)
		if r.Navigation != nil {
			var review AutomaticReviewPrivateV2
			json.Unmarshal(raw, &review)
			if mode == "nav-abstain" {
				review.Verdict = "ABSTAIN"
			} else if mode == "nav-correct" && r.Navigation.Attempt == 0 {
				review.Verdict = "REJECT"
			}
			return automaticTestEncode(review), e
		}
		if mode == "mixed" {
			var out CohortResponsePrivateV2
			json.Unmarshal(raw, &out)
			for j := range out.Items {
				switch r.Items[j].Target {
				case "target-1":
					out.Items[j].Review.Verdict = "ABSTAIN"
				case "target-2":
					out.Items[j].Review.Verdict = "REJECT"
				}
			}
			raw = automaticTestEncode(out)
		}
		return raw, e
	}
	a, e := AutomateCohortPrivateV2(context.Background(), h, cohortTestLimits(), 6, i, v)
	if e != nil {
		t.Fatal(e)
	}
	out, e := RetainAutomaticPrivateV2(context.Background(), h, a, outs, ins)
	if e != nil {
		t.Fatal(e)
	}
	var env postPublicationEnvelopePrivateV2
	if e = observationClosedPrivateV2(out.EnvelopeBytes(), &env); e != nil {
		t.Fatal(e)
	}
	var p Result
	if e = observationClosedPrivateV2(env.Preparation, &p); e != nil {
		t.Fatal(e)
	}
	if e = observationCohortPrivateV2(context.Background(), p, *env.Inventory, env.Ledger, h.state.limits); e != nil {
		t.Fatal("valid original creator output", e)
	}
	return p, env, h.state.limits, requests
}

func TestCohortObservationNavigationAbstainTerminalMED(t *testing.T) {
	t.Run("valid-terminal-abstain", func(t *testing.T) {
		_, env, _, _ := retainedCohortMEDFixture(t, "nav-abstain")
		if env.Inventory.Status != "PROVISIONAL_WITH_UNRESOLVED" || len(env.Inventory.NavigationRevisions) != 1 || env.Inventory.NavigationRevisions[0].Status != "ABSTAINED" {
			t.Fatal("creator terminal abstention fixture")
		}
	})
	t.Run("abstain-then-approve-impossible", func(t *testing.T) {
		p, env, post, requests := retainedCohortMEDFixture(t, "nav-correct")
		inv := env.Inventory
		if len(inv.NavigationRevisions) != 2 || inv.NavigationRevisions[0].Status != "REJECTED" || inv.NavigationRevisions[1].Status != "APPROVED" {
			t.Fatal("creator correction fixture")
		}
		first := &inv.NavigationRevisions[0]
		first.Status = "ABSTAINED"
		first.Review.Verdict = "ABSTAIN"
		for j := range inv.Cohort.Frames {
			f := &inv.Cohort.Frames[j]
			if f.Stage == "REVIEW_NAVIGATION" {
				f.Response = automaticTestEncode(first.Review)
				f.ResponseSHA256 = postPublicationDigest(f.Response)
				f.ResponseByteLength = len(f.Response)
				break
			}
		}
		// Recompute the direct fixture's exact request+response charge, including
		// the changed feedback and prior navigation revision in attempt 1 requests.
		inv.Bytes = 0
		for j := range requests {
			r := &requests[j]
			if r.Navigation != nil && r.Navigation.Attempt == 1 {
				r.Navigation.Feedback = "ABSTAINED: " + first.Findings
				r.Navigation.PriorNavigationRevisions[0] = postClone(*first)
			}
			inv.Bytes += len(automaticTestEncode(*r)) + len(inv.Cohort.Frames[j].Response)
		}
		env.HumanInventory = renderAutomatic(*inv)
		raw := automaticTestEncode(env)
		var detached postPublicationEnvelopePrivateV2
		if e := observationClosedPrivateV2(raw, &detached); e != nil {
			t.Fatal("canonical detached counterexample", e)
		}
		err := observationCohortPrivateV2(context.Background(), p, *detached.Inventory, detached.Ledger, post)
		if err == nil || !strings.Contains(err.Error(), "navigation after abstention") {
			t.Fatalf("ASSERT_MED_NAV_ABSTAIN_TERMINAL: want navigation after abstention, got %v", err)
		}
		t.Log("ASSERT_MED_NAV_ABSTAIN_TERMINAL: PASS")
	})
}

func TestCohortObservationGapLedgerMED(t *testing.T) {
	p, env, post, _ := retainedCohortMEDFixture(t, "gaps")
	if env.Inventory.Status != "PROVISIONAL_WITH_UNRESOLVED" || len(env.Inventory.Candidates) != 0 {
		t.Fatal("gap creator state")
	}
	for j := range env.Ledger.Targets {
		if env.Ledger.Targets[j].SubmissionStatus != "" || env.Ledger.Targets[j].ReviewStatus != "PENDING" {
			t.Fatal("gap reconciliation state")
		}
		e := postClone(env)
		e.Ledger.Targets[j].SubmissionStatus = "PENDING"
		err := observationCohortPrivateV2(context.Background(), p, *e.Inventory, e.Ledger, post)
		if err == nil || !strings.Contains(err.Error(), "unapproved ledger correspondence") {
			t.Fatalf("ASSERT_MED_GAP_LEDGER_%d: %v", j, err)
		}
	}
	t.Log("ASSERT_MED_GAP_LEDGER: PASS original retained source/resource gaps, empty submission status required")
}

func TestCohortObservationUnapprovedLedgerMED(t *testing.T) {
	p, env, post, _ := retainedCohortMEDFixture(t, "mixed")
	if env.Inventory.Status != "PROVISIONAL_WITH_UNRESOLVED" || env.Inventory.Rows[1].Status != "ABSTAINED" || env.Inventory.Rows[2].Status != "REJECTED" || len(env.Inventory.Rows[2].Revisions) != 2 || env.Inventory.Rows[3].Status != "UNSUPPORTED" {
		t.Fatal("mixed creator fixture")
	}
	for _, kind := range []string{"recorded-review-without-review", "review-status-empty", "review-reason", "independence", "submission-status", "submission-status-empty", "submission-sha", "submission-reason", "submission-pointer", "review-pointer"} {
		t.Run(kind, func(t *testing.T) {
			e := postClone(env)
			row := &e.Ledger.Targets[1]
			switch kind {
			case "recorded-review-without-review":
				row.ReviewStatus = "RECORDED_SUPPORT_WITH_LIMITS"
			case "review-status-empty":
				row.ReviewStatus = ""
			case "review-reason":
				row.ReviewReason = "invented"
			case "independence":
				row.Independence = "CALLER_ASSERTED_INDEPENDENCE_NOT_AUTHENTICATED"
			case "submission-status":
				row.SubmissionStatus = "ABSTAINED"
			case "submission-status-empty":
				row.SubmissionStatus = ""
			case "submission-sha":
				row.SubmissionSHA256 = postPublicationDigest([]byte("invented"))
			case "submission-reason":
				row.SubmissionReason = "invented"
			case "submission-pointer":
				row.Submission = postClone(e.Ledger.Targets[0].Submission)
			case "review-pointer":
				row.Review = postClone(e.Ledger.Targets[0].Review)
			}
			// Cohort human output renders inventory, not these ledger diagnostics.
			// Preserve its exact creator bytes; re-encode a canonical detached envelope.
			var detached postPublicationEnvelopePrivateV2
			if err := observationClosedPrivateV2(automaticTestEncode(e), &detached); err != nil {
				t.Fatal(err)
			}
			err := observationCohortPrivateV2(context.Background(), p, *detached.Inventory, detached.Ledger, post)
			if err == nil || !strings.Contains(err.Error(), "unapproved ledger correspondence") {
				t.Fatalf("ASSERT_MED_UNAPPROVED_LEDGER_%s: want unapproved ledger correspondence, got %v", kind, err)
			}
			t.Logf("ASSERT_MED_UNAPPROVED_LEDGER_%s: PASS", kind)
		})
	}
}
