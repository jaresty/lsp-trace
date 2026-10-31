package privatedescribeprep

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

type cohortTestBackend struct {
	id string
	fn func(CohortRequestPrivateV2) ([]byte, error)
}

func (b cohortTestBackend) Identity() string { return b.id }
func (b cohortTestBackend) ExchangeCohort(ctx context.Context, r CohortRequestPrivateV2, cap int) ([]byte, error) {
	return b.fn(r)
}
func cohortTestLimits() AutomaticLimitsPrivateV2 {
	return AutomaticLimitsPrivateV2{1, 16, 1 << 20, 64 << 10, 8 << 20, 4 << 20}
}
func cohortTestPair() (cohortTestBackend, cohortTestBackend) {
	i := cohortTestBackend{"interpreter", func(r CohortRequestPrivateV2) ([]byte, error) {
		if r.Navigation != nil {
			return automaticTestEncode([]NavigationPrivateV2{}), nil
		}
		out := CohortResponsePrivateV2{Protocol: r.Protocol, Stage: r.Stage, PreparationSHA256: r.PreparationSHA256}
		for j, item := range r.Items {
			v := automaticTestInterpret(AutomaticRequestPrivateV2{Bodies: item.Bodies})
			out.Items = append(out.Items, CohortResponseItemPrivateV2{Binding: r.ItemBindings[j], Interpretation: &v})
		}
		return automaticTestEncode(out), nil
	}}
	v := cohortTestBackend{"reviewer", func(r CohortRequestPrivateV2) ([]byte, error) {
		if r.Navigation != nil {
			return automaticTestEncode(AutomaticReviewPrivateV2{Digest: r.Navigation.RevisionDigest, Verdict: "APPROVE", Findings: "fixture bounded navigation review"}), nil
		}
		out := CohortResponsePrivateV2{Protocol: r.Protocol, Stage: r.Stage, PreparationSHA256: r.PreparationSHA256}
		for j, item := range r.Items {
			v := AutomaticReviewPrivateV2{Digest: item.InterpretationDigest, Verdict: "APPROVE", Findings: "fixture own body independently reviewed"}
			out.Items = append(out.Items, CohortResponseItemPrivateV2{Binding: r.ItemBindings[j], Review: &v})
		}
		return automaticTestEncode(out), nil
	}}
	return i, v
}
func TestCohortSixInFour(t *testing.T) {
	h := automaticTestOwner(t, 6)
	i, v := cohortTestPair()
	a, e := AutomateCohortPrivateV2(context.Background(), h, cohortTestLimits(), 6, i, v)
	if e != nil || a.Inventory().Status != "PROVISIONAL_COMPLETE" || a.Inventory().Calls != 4 || len(a.Inventory().Candidates) != 6 || a.owner != h.state {
		t.Fatalf("ASSERT_COHORT_SIX_IN_FOUR: FAIL error=%v inventory=%+v", e, a.Inventory())
	}
	t.Log("ASSERT_COHORT_SIX_IN_FOUR: PASS")
}

func TestCohortBindingVector(t *testing.T) {
	for _, kind := range []string{"missing", "extra", "duplicate", "reorder", "swapped", "foreign", "stale", "wrong-stage", "wrong-preparation", "null"} {
		t.Run(kind, func(t *testing.T) {
			h := automaticTestOwner(t, 3)
			i, v := cohortTestPair()
			old := i.fn
			i.fn = func(r CohortRequestPrivateV2) ([]byte, error) {
				raw, e := old(r)
				if r.Stage != "INTERPRET_COHORT" {
					return raw, e
				}
				var out CohortResponsePrivateV2
				json.Unmarshal(raw, &out)
				switch kind {
				case "missing":
					out.Items = out.Items[:2]
				case "extra":
					out.Items = append(out.Items, out.Items[0])
				case "duplicate":
					out.Items[1] = out.Items[0]
				case "reorder":
					out.Items[0], out.Items[1] = out.Items[1], out.Items[0]
				case "swapped":
					out.Items[0].Binding.Target, out.Items[1].Binding.Target = out.Items[1].Binding.Target, out.Items[0].Binding.Target
				case "foreign":
					out.Items[2].Binding.InputSHA256 = "foreign"
				case "stale":
					out.Items[2].Binding.PredecessorDigest = "stale"
				case "wrong-stage":
					out.Stage = "REVIEW_TECHNICAL_COHORT"
				case "wrong-preparation":
					out.PreparationSHA256 = "foreign"
				case "null":
					out.Items = nil
				}
				return automaticTestEncode(out), nil
			}
			a, e := AutomateCohortPrivateV2(context.Background(), h, cohortTestLimits(), 3, i, v)
			if e == nil || a.Inventory().Status != "FAILED" || len(a.Inventory().Candidates) != 0 || a.owner != nil || len(a.Inventory().Rows) != 3 || a.Inventory().Calls != 1 {
				t.Fatalf("ASSERT_COHORT_VECTOR_ATOMIC: FAIL %s %v %+v", kind, e, a.Inventory())
			}
			for _, row := range a.Inventory().Rows {
				if len(row.Revisions) != 0 {
					t.Fatal("ASSERT_COHORT_VECTOR_ATOMIC: FAIL accepted prefix")
				}
			}
			t.Log("ASSERT_COHORT_VECTOR_ATOMIC: PASS")
		})
	}
}
func TestCohortOwnBodiesAndMixedCorrections(t *testing.T) {
	h := automaticTestOwner(t, 3)
	i, v := cohortTestPair()
	oldI, oldV := i.fn, v.fn
	var interpreted, reviewed [][]string
	i.fn = func(r CohortRequestPrivateV2) ([]byte, error) {
		raw, e := oldI(r)
		if r.Navigation != nil {
			return raw, e
		}
		var targets []string
		for _, it := range r.Items {
			targets = append(targets, it.Target)
			if it.Attempt == 1 && (it.PredecessorDigest == "" || len(it.PriorRevisions) != 1 || it.Feedback == "") {
				t.Fatal("ASSERT_COHORT_CORRECTION_CHAIN: FAIL")
			}
		}
		interpreted = append(interpreted, targets)
		var out CohortResponsePrivateV2
		json.Unmarshal(raw, &out)
		for j := range out.Items {
			if r.Items[j].Attempt == 0 && r.Items[j].Target == "target-1" {
				out.Items[j].Interpretation.Claims[0].Citations[0].Handle = r.Items[0].Bodies[0].Handle
			}
		}
		return automaticTestEncode(out), nil
	}
	v.fn = func(r CohortRequestPrivateV2) ([]byte, error) {
		raw, e := oldV(r)
		if r.Navigation != nil {
			if len(r.Navigation.Candidates) != 3 {
				t.Fatal("ASSERT_COHORT_NAVIGATION_EXACT: FAIL")
			}
			return raw, e
		}
		var targets []string
		for _, it := range r.Items {
			targets = append(targets, it.Target)
		}
		reviewed = append(reviewed, targets)
		var out CohortResponsePrivateV2
		json.Unmarshal(raw, &out)
		for j := range out.Items {
			if r.Items[j].Attempt == 0 && r.Items[j].Target == "target-2" {
				out.Items[j].Review.Verdict = "REJECT"
			}
		}
		return automaticTestEncode(out), nil
	}
	a, e := AutomateCohortPrivateV2(context.Background(), h, cohortTestLimits(), 3, i, v)
	if e != nil || a.Inventory().Status != "PROVISIONAL_COMPLETE" || a.Inventory().Calls != 6 || len(a.Inventory().Candidates) != 3 || len(interpreted) != 2 || strings.Join(interpreted[1], ",") != "target-1,target-2" || strings.Join(reviewed[0], ",") != "target-0,target-2" {
		t.Fatalf("ASSERT_COHORT_CORRECTION_CHAIN: FAIL %v %v %v %+v", e, interpreted, reviewed, a.Inventory())
	}
	rows := a.Inventory().Rows
	if len(rows[0].Revisions) != 1 || len(rows[1].Revisions) != 2 || rows[1].Revisions[0].Status != "MALFORMED" || rows[2].Revisions[0].Status != "REJECTED" || rows[1].Revisions[1].PredecessorDigest != rows[1].Revisions[0].Digest {
		t.Fatal("ASSERT_COHORT_CORRECTION_CHAIN: FAIL revisions")
	}
	checkOwnedCohortObservationTest(t, h, a)
	t.Log("ASSERT_COHORT_CORRECTION_CHAIN: PASS")
}
func TestCohortReviewDigestAndTerminalBudget(t *testing.T) {
	for _, kind := range []string{"digest", "calls", "response", "cancel", "terminal"} {
		t.Run(kind, func(t *testing.T) {
			h := automaticTestOwner(t, 2)
			i, v := cohortTestPair()
			l := cohortTestLimits()
			old := v.fn
			ctx := context.Background()
			switch kind {
			case "digest":
				l.MaxCorrections = 0
				v.fn = func(r CohortRequestPrivateV2) ([]byte, error) {
					raw, e := old(r)
					if r.Navigation != nil {
						return raw, e
					}
					var out CohortResponsePrivateV2
					json.Unmarshal(raw, &out)
					out.Items[0].Review.Digest = "foreign"
					return automaticTestEncode(out), nil
				}
			case "calls":
				l.MaxCalls = 2
			case "response":
				i.fn = func(CohortRequestPrivateV2) ([]byte, error) { return make([]byte, l.MaxResponseBytes+1), nil }
			case "cancel":
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			case "terminal":
				v.fn = func(CohortRequestPrivateV2) ([]byte, error) { return []byte("prefix"), cohortTestTerminal{} }
			}
			a, e := AutomateCohortPrivateV2(ctx, h, l, 2, i, v)
			if kind == "digest" {
				if e != nil || len(a.Inventory().Candidates) != 1 || a.Inventory().Rows[0].Status != "MALFORMED" {
					t.Fatal("ASSERT_COHORT_REVIEW_DIGEST: FAIL", e, a.Inventory())
				}
				t.Log("ASSERT_COHORT_REVIEW_DIGEST: PASS")
				return
			}
			if e == nil || a.owner != nil || a.Inventory().Status != "FAILED" || len(a.Inventory().Candidates) != 0 || len(a.Inventory().Navigation) != 0 || a.Inventory().Calls > l.MaxCalls || a.Inventory().Bytes > l.MaxTotalBytes {
				t.Fatal("ASSERT_COHORT_TERMINAL_CLEAR: FAIL", e, a.Inventory())
			}
			if kind == "response" && a.Inventory().Bytes < l.MaxResponseBytes {
				t.Fatal("ASSERT_COHORT_PREFIX_CHARGED: FAIL")
			}
			t.Log("ASSERT_COHORT_TERMINAL_CLEAR: PASS")
		})
	}
}

type cohortTestTerminal struct{}

func (cohortTestTerminal) Error() string           { return "controlled terminal" }
func (cohortTestTerminal) TerminalHandoffFailure() {}
func TestCohortPackingExactAndOversize(t *testing.T) {
	i, _ := cohortTestPair()
	l := cohortTestLimits()
	b := automaticBudget{limits: l}
	items := []CohortItemPrivateV2{{InputSHA256: "a", Target: "one", Bodies: []OwnBodyPrivateV2{{"h", strings.Repeat("x", 1000)}}}, {InputSHA256: "a", Target: "two", Bodies: []OwnBodyPrivateV2{{"h2", strings.Repeat("y", 1000)}}}}
	both := cohortRequest("INTERPRET_COHORT", "sha", items, mechanicalLimits())
	size, _ := cohortSize(i, both, l.MaxResponseBytes)
	b.limits.MaxRequestBytes = size
	req, n, e := b.cohortPack(i, both.Stage, both.PreparationSHA256, items, 2, both.PostLimits)
	if e != nil || n != 2 || len(automaticTestEncode(req)) != size {
		t.Fatal("ASSERT_COHORT_PACK_EXACT: FAIL", e, n)
	}
	b.limits.MaxRequestBytes = size - 1
	_, n, e = b.cohortPack(i, both.Stage, both.PreparationSHA256, items, 2, both.PostLimits)
	if e != nil || n != 1 {
		t.Fatal("ASSERT_COHORT_PACK_EXACT: FAIL boundary", e, n)
	}
	v := automaticTestInterpret(AutomaticRequestPrivateV2{Bodies: items[0].Bodies})
	items[0].Interpretation = &v
	items[0].InterpretationDigest = "revision"
	items[1].Interpretation = &v
	items[1].InterpretationDigest = "revision2"
	review := cohortRequest("REVIEW_TECHNICAL_COHORT", "sha", items, both.PostLimits)
	reviewSize, _ := cohortSize(i, review, l.MaxResponseBytes)
	if reviewSize <= size {
		t.Fatal("ASSERT_COHORT_REVIEW_LARGER: FAIL")
	}
	singleReview := cohortRequest(review.Stage, "sha", items[:1], both.PostLimits)
	singleReviewSize, _ := cohortSize(i, singleReview, l.MaxResponseBytes)
	b.limits.MaxRequestBytes = max(size, singleReviewSize)
	_, n, e = b.cohortPack(i, review.Stage, "sha", items, 2, both.PostLimits)
	if e != nil || n != 1 {
		t.Fatal("ASSERT_COHORT_REVIEW_REPACK: FAIL", e, n)
	}
	b.limits.MaxRequestBytes = 1
	_, n, e = b.cohortPack(i, both.Stage, "sha", items, 2, both.PostLimits)
	var be AutomaticBackendErrorPrivateV2
	if n != 0 || !errors.As(e, &be) || be.Class != "BUDGET_EXHAUSTED" || b.calls != 0 {
		t.Fatal("ASSERT_COHORT_OVERSIZE_NO_CALL: FAIL", e, n)
	}
	t.Log("ASSERT_COHORT_PACK_EXACT: PASS")
	t.Log("ASSERT_COHORT_REVIEW_REPACK: PASS")
	t.Log("ASSERT_COHORT_OVERSIZE_NO_CALL: PASS")
}
