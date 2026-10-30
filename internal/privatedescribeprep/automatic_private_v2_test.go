package privatedescribeprep

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	pilot "lsp-trace/internal/censuslineagepilot"
	"lsp-trace/internal/publication"
)

func automaticTestLimits() AutomaticLimitsPrivateV2 {
	return AutomaticLimitsPrivateV2{1, 32, 8 << 20, 64 << 10, 32 << 20, 8 << 20}
}

const automaticTestBody = "func sum(xs []int) int { total := 0; for _, x := range xs { if x > 0 { total += x } }; return total }"

func automaticTestOwner(t *testing.T, n int) PostPreparationPrivateV2 {
	return automaticTestOwnerBody(t, n, automaticTestBody)
}
func automaticTestOwnerBody(t *testing.T, n int, body string) PostPreparationPrivateV2 {
	t.Helper()
	r := mechanicalPreparation(n)
	r.RequestType = RequestTypePrivateV2
	r.PartBytes = 0
	for i := range r.Packets {
		p := &r.Packets[i]
		p.Parts[0].RequestType = RequestTypePrivateV2
		p.Parts[0].Records[0] = record("RETAINED_SOURCE_AST_DECLARATION", p.Parts[0].Records[0].Handle, Declaration{Body: body})
		raw, _ := json.Marshal(p.Parts[0])
		p.PartSHA256[0] = pilot.Digest(raw)
		r.PartBytes += len(raw)
	}
	h, e := postFromPreparedProfile(context.Background(), r, mechanicalLimits(), RequestTypePrivateV2)
	if e != nil {
		t.Fatal(e)
	}
	return PostPreparationPrivateV2{state: h.state}
}

type automaticTestBackend struct {
	id string
	fn func(AutomaticRequestPrivateV2) ([]byte, error)
}

func (b automaticTestBackend) Identity() string { return b.id }
func (b automaticTestBackend) Exchange(ctx context.Context, r AutomaticRequestPrivateV2, cap int) ([]byte, error) {
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	return b.fn(r)
}
func automaticTestEncode(v any) []byte { b, _ := json.Marshal(v); return b }
func automaticTestInterpret(r AutomaticRequestPrivateV2) AutomaticInterpretationPrivateV2 {
	body := r.Bodies[0]
	return AutomaticInterpretationPrivateV2{Disposition: "DESCRIBED", Label: "positive accumulation", Description: "Initializes total to zero, visits each input, adds only positive values, and returns the accumulated total.", Limitations: "Own AST only; helper and runtime behavior unknown", Alternatives: "A caller may never invoke it", RejectionCondition: "Retained body lacks the described positive predicate or accumulation", Claims: []AutomaticClaimPrivateV2{{Text: "Adds positive input elements to an initially zero accumulator", Citations: []ExactQuotePrivateV2{{Handle: body.Handle, Start: 0, End: len(body.Body), Quote: body.Body}}}}}
}
func automaticTestPair() (automaticTestBackend, automaticTestBackend) {
	i := automaticTestBackend{"interpreter", func(r AutomaticRequestPrivateV2) ([]byte, error) {
		switch r.Stage {
		case "INTERPRET":
			return automaticTestEncode(automaticTestInterpret(r)), nil
		case "NAVIGATE":
			return automaticTestEncode([]NavigationPrivateV2{{ID: "arithmetic", Candidates: []string{r.Candidates[0].ID}, Rationale: "Shared arithmetic is a navigation question, not feature identity", WithdrawalCondition: "Stakeholder rejects the relationship"}}), nil
		}
		return nil, errors.New("unexpected stage")
	}}
	v := automaticTestBackend{"reviewer", func(r AutomaticRequestPrivateV2) ([]byte, error) {
		return automaticTestEncode(AutomaticReviewPrivateV2{Digest: r.RevisionDigest, Verdict: "APPROVE", Findings: "Fixture independently checks positive predicate and accumulator; runtime unknown"}), nil
	}}
	return i, v
}

// Assertion procedures consume actual production results, not a stage-log oracle.
func automaticNominalGuard(a AutomaticResultPrivateV2) error {
	v := a.Inventory()
	if v.Status != "PROVISIONAL_COMPLETE" || len(v.Candidates) == 0 || a.owner == nil || !strings.Contains(a.Human(), "adds only positive") {
		return errors.New("ASSERT_AUTO_NOMINAL: FAIL")
	}
	return nil
}
func TestAutomaticNominal(t *testing.T) {
	t.Log("procedure: AutomatePrivateV2(sealed own fixture, independent bounded backends) -> ASSERT_AUTO_NOMINAL")
	h := automaticTestOwner(t, 3)
	i, v := automaticTestPair()
	a, e := AutomatePrivateV2(context.Background(), h, automaticTestLimits(), i, v)
	if e != nil {
		t.Fatal(e)
	}
	if e = automaticNominalGuard(a); e != nil {
		t.Fatal(e)
	}
	t.Log("ASSERT_AUTO_NOMINAL: PASS")
	got := a.Inventory()
	if len(got.Rows) != 3 {
		t.Fatal("ASSERT_AUTO_ACCOUNTING: FAIL")
	}
	for j, row := range got.Rows {
		if row.Status != "PROVISIONAL" || row.Target != fmt.Sprintf("target-%d", j) || row.CandidateID == "" {
			t.Fatal("ASSERT_AUTO_ACCOUNTING: FAIL")
		}
		c := got.Candidates[j]
		if !c.PrivateCeilings.valid() || c.ID != row.CandidateID || c.Claims[0].Citations[0].Quote != automaticTestBody {
			t.Fatal("ASSERT_AUTO_CEILING_OWN: FAIL")
		}
	}
	t.Log("ASSERT_AUTO_ACCOUNTING: PASS")
	t.Log("ASSERT_AUTO_CEILING_OWN: PASS")
	next, e := AutomatePrivateV2(context.Background(), h, automaticTestLimits(), i, v)
	if e != nil {
		t.Fatal(e)
	}
	for j, c := range next.Inventory().Candidates {
		if c.ID != got.Candidates[j].ID {
			t.Fatal("ASSERT_AUTO_STABLE: FAIL")
		}
	}
	t.Log("ASSERT_AUTO_STABLE: PASS")
	withdrawn := WithdrawNavigationPrivateV2(got, "arithmetic")
	if len(withdrawn.Navigation) != 0 || !bytes.Equal(automaticTestEncode(withdrawn.Candidates), automaticTestEncode(got.Candidates)) || len(got.Navigation) != 1 {
		t.Fatal("ASSERT_AUTO_WITHDRAW: FAIL")
	}
	t.Log("ASSERT_AUTO_WITHDRAW: PASS")
	// Detached observations cannot mutate the sealed automatic publication owner.
	got.Candidates[0].Accepted = true
	got.Rows[0].Revisions[0].Interpretation.Claims[0].Text = "foreign"
	if a.Inventory().Candidates[0].Accepted {
		t.Fatal("detachment")
	}
}
func TestAutomaticExactCitationRefusal(t *testing.T) {
	for _, kind := range []string{"foreign", "offset", "quote", "empty", "metadata", "authority-text"} {
		t.Run(kind, func(t *testing.T) {
			h := automaticTestOwner(t, 2)
			i, v := automaticTestPair()
			old := i.fn
			i.fn = func(r AutomaticRequestPrivateV2) ([]byte, error) {
				if r.Stage != "INTERPRET" {
					return old(r)
				}
				x := automaticTestInterpret(r)
				q := &x.Claims[0].Citations[0]
				switch kind {
				case "foreign":
					q.Handle = "another-batch/seed/target/declaration"
				case "offset":
					q.Start++
				case "quote":
					q.Quote = "return 0"
				case "empty":
					q.End = q.Start
				case "metadata":
					q.Handle = "TARGET"
				case "authority-text":
					return []byte(`{"Accepted":true}`), nil
				}
				return automaticTestEncode(x), nil
			}
			a, e := AutomatePrivateV2(context.Background(), h, automaticTestLimits(), i, v)
			if e != nil || len(a.Inventory().Candidates) != 0 || a.Inventory().Rows[0].Status != "MALFORMED" {
				t.Fatal("ASSERT_AUTO_EXACT_REFUSAL: FAIL", e, a.Inventory())
			}
			t.Log("ASSERT_AUTO_EXACT_REFUSAL: PASS")
		})
	}
}
func TestAutomaticCorrectionLineage(t *testing.T) {
	h := automaticTestOwner(t, 1)
	i, v := automaticTestPair()
	old := i.fn
	i.fn = func(r AutomaticRequestPrivateV2) ([]byte, error) {
		if r.Stage != "INTERPRET" {
			return old(r)
		}
		x := automaticTestInterpret(r)
		if r.Attempt == 0 {
			x.Claims[0].Citations[0].Start++
		} else if r.PredecessorDigest == "" || !strings.Contains(r.Feedback, "MALFORMED") {
			t.Fatal("correction feedback missing")
		}
		return automaticTestEncode(x), nil
	}
	a, e := AutomatePrivateV2(context.Background(), h, automaticTestLimits(), i, v)
	revs := a.Inventory().Rows[0].Revisions
	if e != nil || len(revs) != 2 || revs[0].Status != "MALFORMED" || revs[1].Status != "APPROVED" || revs[1].PredecessorDigest != revs[0].Digest || len(revs[0].Response) == 0 {
		t.Fatal("ASSERT_AUTO_CORRECTION: FAIL", e, revs)
	}
	for _, rev := range revs {
		if rev.ResponseSHA256 != pilot.Digest(rev.Response) || rev.ResponseByteLength != len(rev.Response) || rev.Digest != automaticDigest(struct{ Previous, RawDigest string }{rev.PredecessorDigest, rev.ResponseSHA256}) {
			t.Fatal("ASSERT_AUTO_CORRECTION: FAIL")
		}
	}
	t.Log("ASSERT_AUTO_CORRECTION: PASS")
}
func TestAutomaticLongOwnQuote(t *testing.T) {
	body := strings.TrimSuffix(automaticTestBody, "}") + "/* " + strings.Repeat("inert é comment ", 40) + "*/ }"
	h := automaticTestOwnerBody(t, 1, body)
	i, v := automaticTestPair()
	a, e := AutomatePrivateV2(context.Background(), h, automaticTestLimits(), i, v)
	if e != nil || a.Inventory().Status != "PROVISIONAL_COMPLETE" || len(a.Inventory().Candidates) != 1 {
		t.Fatal("ASSERT_AUTO_LONG_OWN_QUOTE: FAIL", e)
	}
	quote := a.Inventory().Candidates[0].Claims[0].Citations[0]
	if quote.Quote != body || quote.Start != 0 || quote.End != len(body) {
		t.Fatal("ASSERT_AUTO_LONG_OWN_QUOTE: FAIL")
	}
	ledger, e := h.Reconcile(context.Background(), a.submissions, a.reviews)
	if e != nil || len(ledger.Leads) != 1 {
		t.Fatal("ASSERT_AUTO_LONG_OWN_QUOTE: FAIL", e)
	}
	var joined strings.Builder
	for _, q := range ledger.Leads[0].Submission.Claims[0].Citations {
		if len(q.SourceQuote) > 240 || !utf8.ValidString(q.SourceQuote) {
			t.Fatal("ASSERT_AUTO_LONG_OWN_QUOTE: FAIL")
		}
		joined.WriteString(q.SourceQuote)
	}
	if joined.String() != body {
		t.Fatal("ASSERT_AUTO_LONG_OWN_QUOTE: FAIL")
	}
	t.Log("ASSERT_AUTO_LONG_OWN_QUOTE: PASS")
}
func TestAutomaticCorrectionContext(t *testing.T) {
	h := automaticTestOwner(t, 1)
	i, v := automaticTestPair()
	old := i.fn
	i.fn = func(r AutomaticRequestPrivateV2) ([]byte, error) {
		if r.Stage != "INTERPRET" {
			return old(r)
		}
		x := automaticTestInterpret(r)
		if r.Attempt == 0 {
			x.Claims[0].Citations[0].Start++
		} else if len(r.PriorRevisions) != 1 || r.PriorRevisions[0].Digest != r.PredecessorDigest || len(r.PriorRevisions[0].Response) == 0 || r.PostLimits.MaxClaimsPerItem < 1 || r.LedgerQuoteChunkBytes != 240 {
			t.Fatal("ASSERT_AUTO_CORRECTION_CONTEXT: FAIL")
		}
		return automaticTestEncode(x), nil
	}
	a, e := AutomatePrivateV2(context.Background(), h, automaticTestLimits(), i, v)
	if e != nil || a.Inventory().Status != "PROVISIONAL_COMPLETE" {
		t.Fatal("ASSERT_AUTO_CORRECTION_CONTEXT: FAIL", e)
	}
	t.Log("ASSERT_AUTO_CORRECTION_CONTEXT: PASS")
}
func TestAutomaticCorrectionLimit(t *testing.T) {
	for _, limit := range []int{0, 1, 3} {
		h := automaticTestOwner(t, 1)
		i, v := automaticTestPair()
		i.fn = func(AutomaticRequestPrivateV2) ([]byte, error) { return []byte(`{"bad":true}`), nil }
		l := automaticTestLimits()
		l.MaxCorrections = limit
		a, e := AutomatePrivateV2(context.Background(), h, l, i, v)
		if e != nil || len(a.Inventory().Rows[0].Revisions) != 1+limit {
			t.Fatal("ASSERT_AUTO_ATTEMPT_BOUND: FAIL", e)
		}
		t.Log("ASSERT_AUTO_ATTEMPT_BOUND: PASS")
	}
}
func TestAutomaticStableAcrossRevisionText(t *testing.T) {
	h := automaticTestOwner(t, 1)
	i, v := automaticTestPair()
	a, e := AutomatePrivateV2(context.Background(), h, automaticTestLimits(), i, v)
	if e != nil {
		t.Fatal(e)
	}
	old := i.fn
	i.fn = func(r AutomaticRequestPrivateV2) ([]byte, error) {
		if r.Stage == "INTERPRET" {
			x := automaticTestInterpret(r)
			x.Label = "revised label"
			x.Description += " This wording is revised."
			return automaticTestEncode(x), nil
		}
		return old(r)
	}
	b, e := AutomatePrivateV2(context.Background(), h, automaticTestLimits(), i, v)
	if e != nil || a.Inventory().Candidates[0].ID != b.Inventory().Candidates[0].ID || a.Inventory().Candidates[0].RevisionDigest == b.Inventory().Candidates[0].RevisionDigest {
		t.Fatal("ASSERT_AUTO_STABLE_REVISION: FAIL", e)
	}
	t.Log("ASSERT_AUTO_STABLE_REVISION: PASS")
}
func TestAutomaticPublicationGates(t *testing.T) {
	for _, kind := range []string{"pair", "same-root", "foreign-protected", "mode"} {
		t.Run(kind, func(t *testing.T) {
			outs, ins := postPublicationRoots(t)
			_, otherIns := postPublicationRoots(t)
			ins = append(ins, otherIns[0])
			h := automaticTestOwner(t, 1)
			for _, root := range ins {
				path, id, e := root.CanonicalIdentityPrivateV2()
				if e != nil {
					t.Fatal(e)
				}
				h.roots = append(h.roots, producerRootBindingPrivateV2{path, id.Device, id.Inode})
			}
			i, v := automaticTestPair()
			a, e := AutomatePrivateV2(context.Background(), h, automaticTestLimits(), i, v)
			if e != nil {
				t.Fatal(e)
			}
			if kind == "same-root" {
				outs[1] = outs[0]
			}
			if kind == "foreign-protected" {
				ins = ins[:1]
			}
			if kind == "mode" {
				path, _, e := ins[0].CanonicalIdentityPrivateV2()
				if e != nil {
					t.Fatal(e)
				}
				if e = os.Chmod(filepath.Join(filepath.Dir(path), "primary"), 0755); e != nil {
					t.Fatal(e)
				}
			}
			out, e := RetainAutomaticPrivateV2(context.Background(), h, a, outs, ins)
			if kind != "pair" {
				if e == nil {
					t.Fatal("ASSERT_AUTO_PUBLICATION_GATE: FAIL", kind)
				}
				t.Log("ASSERT_AUTO_PUBLICATION_GATE: PASS")
				return
			}
			if e != nil || out.Status != "VERIFIED_PAIR_PRIVATE_WITH_LIMITS" {
				t.Fatal("ASSERT_AUTO_PAIRED_BYTES: FAIL", e)
			}
			for _, root := range outs {
				b, e := root.ReadPostPreparationPrivateV2(context.Background(), out.EnvelopeSHA256, int64(out.ByteLength))
				if e != nil || !bytes.Equal(b, out.EnvelopeBytes()) {
					t.Fatal("ASSERT_AUTO_PAIRED_BYTES: FAIL", e)
				}
			}
			var envelope postPublicationEnvelopePrivateV2
			if json.Unmarshal(out.EnvelopeBytes(), &envelope) != nil || envelope.Inventory == nil || envelope.HumanInventory != a.Human() || len(envelope.Ledger.Leads) != 1 {
				t.Fatal("ASSERT_AUTO_PAIRED_BYTES: FAIL")
			}
			t.Log("ASSERT_AUTO_PAIRED_BYTES: PASS")
		})
	}
}
func TestAutomaticBackendOutcomes(t *testing.T) {
	for _, kind := range []string{"UNAVAILABLE", "MALFORMED", "BUDGET_EXHAUSTED", "UNSUPPORTED", "ABSTAINED", "REJECTED", "same-author", "call-budget", "total-budget", "output-budget", "cancelled"} {
		t.Run(kind, func(t *testing.T) {
			h := automaticTestOwner(t, 2)
			i, v := automaticTestPair()
			l := automaticTestLimits()
			ctx := context.Background()
			switch kind {
			case "UNAVAILABLE", "BUDGET_EXHAUSTED":
				i.fn = func(AutomaticRequestPrivateV2) ([]byte, error) { return nil, AutomaticBackendErrorPrivateV2{kind} }
			case "MALFORMED":
				i.fn = func(AutomaticRequestPrivateV2) ([]byte, error) { return []byte(`{"bad":true}`), nil }
			case "UNSUPPORTED":
				i.fn = func(r AutomaticRequestPrivateV2) ([]byte, error) {
					return automaticTestEncode(AutomaticInterpretationPrivateV2{Disposition: "UNSUPPORTED", Limitations: "Insufficient own evidence", Claims: []AutomaticClaimPrivateV2{}}), nil
				}
			case "ABSTAINED", "REJECTED":
				verdict := "ABSTAIN"
				if kind == "REJECTED" {
					verdict = "REJECT"
				}
				v.fn = func(r AutomaticRequestPrivateV2) ([]byte, error) {
					return automaticTestEncode(AutomaticReviewPrivateV2{r.RevisionDigest, verdict, "Not supported by own body"}), nil
				}
			case "same-author":
				v.id = i.id
			case "call-budget":
				l.MaxCalls = 1
			case "total-budget":
				l.MaxTotalBytes = 1
			case "output-budget":
				l.MaxOutputBytes = 1
			case "cancelled":
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			}
			a, e := AutomatePrivateV2(ctx, h, l, i, v)
			out := a.Inventory()
			if out.Status == "PROVISIONAL_COMPLETE" || len(out.Candidates) > 0 {
				t.Fatal("ASSERT_AUTO_FAILURE_CLASS: FAIL", kind, e, out)
			}
			if kind != "cancelled" && len(out.Rows) != 2 {
				t.Fatal("ASSERT_AUTO_ACCOUNTING: FAIL")
			}
			if e == nil && kind != "same-author" && kind != "call-budget" && kind != "total-budget" && kind != "output-budget" && kind != "cancelled" {
				if out.Rows[0].Status != kind {
					t.Fatal("ASSERT_AUTO_FAILURE_CLASS: FAIL", kind, out.Rows[0].Status)
				}
			}
			if out.Calls > l.MaxCalls {
				t.Fatal("ASSERT_AUTO_BUDGET: FAIL")
			}
			t.Log("ASSERT_AUTO_FAILURE_CLASS: PASS")
			t.Log("ASSERT_AUTO_BUDGET: PASS")
		})
	}
}
func TestAutomaticReviewAndNavigation(t *testing.T) {
	for _, kind := range []string{"stale-review", "nav-rejected", "nav-foreign", "nav-null", "vacuous"} {
		t.Run(kind, func(t *testing.T) {
			h := automaticTestOwner(t, 1)
			i, v := automaticTestPair()
			iv, vv := i.fn, v.fn
			i.fn = func(r AutomaticRequestPrivateV2) ([]byte, error) {
				if kind == "nav-null" && r.Stage == "NAVIGATE" {
					return []byte("null"), nil
				}
				if kind == "nav-foreign" && r.Stage == "NAVIGATE" {
					return automaticTestEncode([]NavigationPrivateV2{{"bad", []string{"foreign"}, "bad", "bad"}}), nil
				}
				if kind == "vacuous" && r.Stage == "INTERPRET" {
					x := automaticTestInterpret(r)
					x.Description = "Initializes and returns"
					return automaticTestEncode(x), nil
				}
				return iv(r)
			}
			v.fn = func(r AutomaticRequestPrivateV2) ([]byte, error) {
				if kind == "stale-review" {
					return automaticTestEncode(AutomaticReviewPrivateV2{"stale", "APPROVE", "stale"}), nil
				}
				if kind == "nav-rejected" && r.Stage == "REVIEW_NAVIGATION" || kind == "vacuous" && r.Stage == "REVIEW_TECHNICAL" {
					return automaticTestEncode(AutomaticReviewPrivateV2{r.RevisionDigest, "REJECT", "Description omits meaningful loop/predicate behavior"}), nil
				}
				return vv(r)
			}
			a, e := AutomatePrivateV2(context.Background(), h, automaticTestLimits(), i, v)
			if e != nil {
				t.Fatal(e)
			}
			out := a.Inventory()
			if out.Status != "PROVISIONAL_WITH_UNRESOLVED" {
				t.Fatal("ASSERT_AUTO_REVIEW: FAIL", out)
			}
			if kind == "stale-review" || kind == "vacuous" {
				if len(out.Candidates) != 0 {
					t.Fatal("ASSERT_AUTO_REVIEW: FAIL")
				}
			} else if len(out.Candidates) != 1 || len(out.Navigation) != 0 {
				t.Fatal("ASSERT_AUTO_NAV_REVIEW: FAIL")
			}
			t.Log("ASSERT_AUTO_REVIEW: PASS")
			t.Log("ASSERT_AUTO_NAV_REVIEW: PASS")
		})
	}
}
func TestAutomaticReviewedEmptyNavigation(t *testing.T) {
	h := automaticTestOwner(t, 1)
	i, v := automaticTestPair()
	iv, vv := i.fn, v.fn
	reviews := 0
	i.fn = func(r AutomaticRequestPrivateV2) ([]byte, error) {
		if r.Stage == "NAVIGATE" {
			return automaticTestEncode([]NavigationPrivateV2{}), nil
		}
		return iv(r)
	}
	v.fn = func(r AutomaticRequestPrivateV2) ([]byte, error) {
		if r.Stage == "REVIEW_NAVIGATION" {
			reviews++
			if r.Navigation == nil || len(r.Navigation) != 0 {
				t.Fatal("ASSERT_AUTO_EMPTY_NAVIGATION: FAIL")
			}
		}
		return vv(r)
	}
	a, e := AutomatePrivateV2(context.Background(), h, automaticTestLimits(), i, v)
	if e != nil || a.Inventory().Status != "PROVISIONAL_COMPLETE" || a.Inventory().Navigation == nil || reviews != 1 {
		t.Fatal("ASSERT_AUTO_EMPTY_NAVIGATION: FAIL", e)
	}
	t.Log("ASSERT_AUTO_EMPTY_NAVIGATION: PASS")
}
func TestAutomaticEmptyMixedAndOwnership(t *testing.T) {
	for _, n := range []int{0, 2} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			h := automaticTestOwner(t, n)
			if n > 0 {
				var r Result
				_ = json.Unmarshal(h.state.prepared, &r)
				r.Index[1].Status = "SOURCE_GAP"
				r.Index[1].Gap = "OWN_SOURCE_UNAVAILABLE"
				r.Index[1].PacketID = ""
				r.Coverage[0].Prepared--
				r.Coverage[0].SourceGap++
				r.Packets = r.Packets[:1]
				r.PartCount = 1
				raw, _ := json.Marshal(r.Packets[0].Parts[0])
				r.PartBytes = len(raw)
				sealed, e := postFromPreparedProfile(context.Background(), r, mechanicalLimits(), RequestTypePrivateV2)
				if e != nil {
					t.Fatal(e)
				}
				h = PostPreparationPrivateV2{state: sealed.state}
			}
			i, v := automaticTestPair()
			old := i.fn
			i.fn = func(r AutomaticRequestPrivateV2) ([]byte, error) {
				if r.Stage == "INTERPRET" {
					if r.Packet == nil || r.Packet.Target != r.Target || r.Packet.InputSHA256 != r.InputSHA256 || len(r.Bodies) != 1 || !strings.Contains(r.Bodies[0].Handle, "/seed/"+r.Target+"/") {
						t.Fatal("ASSERT_AUTO_OWN_REQUEST: FAIL")
					}
					for _, part := range r.Packet.Parts {
						if part.Target != r.Target || part.InputSHA256 != r.InputSHA256 {
							t.Fatal("ASSERT_AUTO_OWN_REQUEST: FAIL")
						}
					}
				}
				return old(r)
			}
			a, e := AutomatePrivateV2(context.Background(), h, automaticTestLimits(), i, v)
			out := a.Inventory()
			if e != nil || len(out.Rows) != n {
				t.Fatal("ASSERT_AUTO_ACCOUNTING: FAIL", e)
			}
			if n > 0 && (out.Status != "PROVISIONAL_WITH_UNRESOLVED" || out.Rows[1].Status != "SOURCE_GAP" || len(out.Candidates) != 1) {
				t.Fatal("ASSERT_AUTO_ACCOUNTING: FAIL")
			}
			if n == 0 && (len(out.Candidates) != 0 || out.Calls != 0) {
				t.Fatal("empty invented evidence")
			}
			t.Log("ASSERT_AUTO_ACCOUNTING: PASS")
			t.Log("ASSERT_AUTO_OWN_REQUEST: PASS")
		})
	}
	h := automaticTestOwner(t, 1)
	i, v := automaticTestPair()
	a, e := AutomatePrivateV2(context.Background(), h, automaticTestLimits(), i, v)
	if e != nil {
		t.Fatal(e)
	}
	var forged AutomaticResultPrivateV2
	_ = json.Unmarshal(automaticTestEncode(a.Inventory()), &forged)
	if _, e = RetainAutomaticPrivateV2(context.Background(), h, forged, [2]*publication.ContentRetentionRoot{}, nil); e == nil {
		t.Fatal("ASSERT_AUTO_OWNER: FAIL")
	}
	if _, e = RetainAutomaticPrivateV2(context.Background(), automaticTestOwner(t, 1), a, [2]*publication.ContentRetentionRoot{}, nil); e == nil {
		t.Fatal("ASSERT_AUTO_OWNER: FAIL")
	}
	t.Log("ASSERT_AUTO_OWNER: PASS")
}
func TestAutomaticNavigationCorrection(t *testing.T) {
	h := automaticTestOwner(t, 1)
	i, v := automaticTestPair()
	old := i.fn
	i.fn = func(r AutomaticRequestPrivateV2) ([]byte, error) {
		if r.Stage == "NAVIGATE" && r.Attempt == 0 {
			return automaticTestEncode([]NavigationPrivateV2{{"bad", []string{"foreign"}, "unsupported", "withdraw"}}), nil
		}
		if r.Stage == "NAVIGATE" && (r.PredecessorDigest == "" || !strings.Contains(r.Feedback, "MALFORMED")) {
			t.Fatal("navigation feedback missing")
		}
		return old(r)
	}
	a, e := AutomatePrivateV2(context.Background(), h, automaticTestLimits(), i, v)
	revs := a.Inventory().NavigationRevisions
	if e != nil || len(revs) != 2 || revs[0].Status != "MALFORMED" || revs[1].Status != "APPROVED" || revs[1].PredecessorDigest != revs[0].Digest || len(a.Inventory().Candidates) != 1 {
		t.Fatal("ASSERT_AUTO_NAV_CORRECTION: FAIL", e)
	}
	t.Log("ASSERT_AUTO_NAV_CORRECTION: PASS")
}
func TestAutomaticFormattingClasses(t *testing.T) {
	for _, kind := range []string{"unsupported-reason", "review-findings"} {
		t.Run(kind, func(t *testing.T) {
			h := automaticTestOwner(t, 1)
			i, v := automaticTestPair()
			if kind == "unsupported-reason" {
				i.fn = func(AutomaticRequestPrivateV2) ([]byte, error) {
					return automaticTestEncode(AutomaticInterpretationPrivateV2{Disposition: "UNSUPPORTED", Limitations: strings.Repeat("x", h.state.limits.MaxStringBytes+1), Claims: []AutomaticClaimPrivateV2{}}), nil
				}
			} else {
				v.fn = func(r AutomaticRequestPrivateV2) ([]byte, error) {
					return automaticTestEncode(AutomaticReviewPrivateV2{r.RevisionDigest, "APPROVE", strings.Repeat("x", h.state.limits.MaxStringBytes+1)}), nil
				}
			}
			a, e := AutomatePrivateV2(context.Background(), h, automaticTestLimits(), i, v)
			if e != nil || a.Inventory().Rows[0].Status != "MALFORMED" || len(a.Inventory().Candidates) != 0 {
				t.Fatal("ASSERT_AUTO_FORMATTING_CLASS: FAIL", e, a.Inventory().Rows[0].Status)
			}
			t.Log("ASSERT_AUTO_FORMATTING_CLASS: PASS")
		})
	}
}
func TestAutomaticHumanReviewSurface(t *testing.T) {
	h := automaticTestOwner(t, 1)
	i, v := automaticTestPair()
	a, e := AutomatePrivateV2(context.Background(), h, automaticTestLimits(), i, v)
	if e != nil {
		t.Fatal(e)
	}
	c := a.Inventory().Candidates[0]
	if !strings.Contains(a.Human(), c.Alternatives) || !strings.Contains(a.Human(), c.RejectionCondition) {
		t.Fatal("ASSERT_AUTO_HUMAN_REVIEW_SURFACE: FAIL")
	}
	t.Log("ASSERT_AUTO_HUMAN_REVIEW_SURFACE: PASS")
}
func TestAutomaticResourceEnvelope(t *testing.T) {
	t.Run("human", func(t *testing.T) {
		h := automaticTestOwner(t, 1)
		i, v := automaticTestPair()
		a, e := AutomatePrivateV2(context.Background(), h, automaticTestLimits(), i, v)
		if e != nil {
			t.Fatal(e)
		}
		l := automaticTestLimits()
		l.MaxOutputBytes = len(automaticTestEncode(a.Inventory()))
		a, e = AutomatePrivateV2(context.Background(), h, l, i, v)
		if e == nil || a.Inventory().Status != "FAILED" || a.Inventory().Failure != "BUDGET_EXHAUSTED" {
			t.Fatal("ASSERT_AUTO_HUMAN_BUDGET: FAIL")
		}
		t.Log("ASSERT_AUTO_HUMAN_BUDGET: PASS")
	})
}
