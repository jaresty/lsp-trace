//go:build darwin || linux

package censuslineagepilot_test

import (
	"context"
	"testing"

	prep "lsp-trace/internal/privatedescribeprep"
)

func ownedPostLimits() prep.PostPreparationLimits {
	return prep.PostPreparationLimits{MaxConstituents: 64, MaxLedgerTargets: 4096, MaxSubmissions: 256, MaxReviews: 256, MaxClaimsPerItem: 16, MaxCitationsPerClaim: 32, MaxStringBytes: 4096, MaxItemBytes: 1 << 20, MaxInputBytes: 16 << 20, MaxOutputBytes: 16 << 20, MaxEvidenceRecords: 262144, MaxWork: 10000000}
}
func ownedPostCeilings() prep.PrivateCeilings {
	return prep.PrivateCeilings{FeatureIdentity: "UNRESOLVED", SourceCompleteness: "UNKNOWN", OrdinaryNative: "NOT_ATTEMPTED", Describe: "NOT_ATTEMPTED", Workflow: "NOT_ATTEMPTED"}
}
func ownedPostSubmission(t *testing.T, h prep.PostPreparation, n int) prep.CallerSubmission {
	t.Helper()
	manifests, err := h.Manifest()
	if err != nil {
		t.Fatal(err)
	}
	m := manifests[n]
	sha, err := h.PreparationSHA256()
	if err != nil {
		t.Fatal(err)
	}
	handle := ""
	for _, ref := range m.Handles {
		if ref.Kind == "RETAINED_SOURCE_AST_DECLARATION" {
			handle = ref.Handle
			break
		}
	}
	if handle == "" {
		t.Fatal("no declaration")
	}
	return prep.CallerSubmission{PrivateCeilings: ownedPostCeilings(), PreparationSHA256: sha, InputSHA256: m.InputSHA256, Target: m.Target, PacketID: m.PacketID, OrderedPartSHA256: m.OrderedPartSHA256, AuthorTag: "caller-producer", Disposition: "DESCRIBED", WorkingLabel: "root", TechnicalDescription: "Retained root declaration", Claims: []prep.TechnicalClaim{{Text: "Declares root", Citations: []prep.Citation{{Handle: handle, SourceQuote: "func root()"}}}}, Limitations: "Synthetic owned fixture, source incomplete", Alternatives: "May be unused", RejectionCondition: "A contradicting retained declaration"}
}
func TestOwnedPrivatePostPreparation(t *testing.T) {
	ctx := context.Background()
	input, traffic := preparationOwnedInput(t)
	before, bp := traffic()
	limits := prep.Limits{MaxTargets: 64, MaxPartBytes: 64 << 10, MaxTotalBytes: 8 << 20, MaxParts: 256, MaxSourceBytes: 2 << 20}
	for _, n := range []int{1, 4, 25} {
		in := input
		_, carrier, err := input.Projection.PreparationEvidencePrivate()
		if err != nil {
			t.Fatal(err)
		}
		for _, target := range carrier.Evidence.Acquisition.Targets[:n] {
			in.Targets = append(in.Targets, target.Requested.ID)
		}
		h, err := prep.NewPostPreparation(ctx, []prep.Input{in}, limits, ownedPostLimits())
		if err != nil {
			t.Fatal(err)
		}
		pending, err := h.Reconcile(ctx, nil, nil)
		if err != nil || len(pending.Targets) != n || pending.Coverage[0].Prepared != 1 || pending.Coverage[0].SourceGap != n-1 || pending.Coverage[0].Pending != 1 {
			t.Fatal(err, pending)
		}
		v := ownedPostSubmission(t, h, 0)
		first, err := h.Reconcile(ctx, []prep.CallerSubmission{v}, nil)
		if err != nil {
			t.Fatal(err)
		}
		review := prep.CallerReview{PrivateCeilings: ownedPostCeilings(), PreparationSHA256: v.PreparationSHA256, InputSHA256: v.InputSHA256, Target: v.Target, SubmissionSHA256: first.Targets[0].SubmissionSHA256, ReviewerAuthorTag: "caller-reviewer", Verdict: "SUPPORT_WITH_LIMITS", Findings: "Caller asserts only this synthetic declaration is supported"}
		out, err := h.Reconcile(ctx, []prep.CallerSubmission{v}, []prep.CallerReview{review})
		if err != nil || len(out.Leads) != 1 || out.Coverage[0].Submitted != 1 || out.Coverage[0].SourceGap != n-1 || out.Leads[0].Accepted {
			t.Fatal(err, out)
		}
		t.Logf("owned selected=%d prepared=1 source_gaps=%d leads=1 provisional only", n, n-1)
	}
	after, ap := traffic()
	if before != after || bp != ap {
		t.Fatal("post-preparation performed traffic")
	}
}
func TestIndependentOwnedPrivatePostPreparation(t *testing.T) {
	ctx := context.Background()
	a, ta := preparationOwnedInput(t)
	b, tb := preparationOwnedInput(t)
	ba, bap := ta()
	bb, bbp := tb()
	// NIL selection preserves every original nomination in both namespaces.
	limits := prep.Limits{MaxTargets: 64, MaxPartBytes: 64 << 10, MaxTotalBytes: 8 << 20, MaxParts: 256, MaxSourceBytes: 2 << 20}
	h, err := prep.NewPostPreparation(ctx, []prep.Input{a, b}, limits, ownedPostLimits())
	if err != nil {
		t.Fatal(err)
	}
	pending, err := h.Reconcile(ctx, nil, nil)
	if err != nil || len(pending.Targets) != 50 || len(pending.Coverage) != 2 {
		t.Fatal(err, pending)
	}
	for _, c := range pending.Coverage {
		if c.Requested != 25 || c.Prepared != 1 || c.SourceGap != 24 || c.Pending != 1 {
			t.Fatal(c)
		}
	}
	va, vb := ownedPostSubmission(t, h, 0), ownedPostSubmission(t, h, 1)
	if va.InputSHA256 == vb.InputSHA256 || va.WorkingLabel != vb.WorkingLabel || va.Target != "root" || vb.Target != "root" {
		t.Fatal("namespace identity")
	}
	first, err := h.Reconcile(ctx, []prep.CallerSubmission{vb, va}, nil)
	if err != nil || first.Coverage[0].Submitted != 1 || first.Coverage[1].Submitted != 1 {
		t.Fatal(err, first)
	}
	foreign := va
	foreign.Claims = []prep.TechnicalClaim{{Text: "foreign", Citations: vb.Claims[0].Citations}}
	rejected, err := h.Reconcile(ctx, []prep.CallerSubmission{foreign}, nil)
	if err != nil || rejected.Coverage[0].Rejected != 1 || rejected.Coverage[1].Pending != 1 {
		t.Fatal("borrowed citation", err, rejected)
	}
	review := prep.CallerReview{PrivateCeilings: ownedPostCeilings(), PreparationSHA256: va.PreparationSHA256, InputSHA256: va.InputSHA256, Target: va.Target, SubmissionSHA256: first.Targets[25].SubmissionSHA256, ReviewerAuthorTag: "other", Verdict: "SUPPORT_WITH_LIMITS", Findings: "wrong namespace pin"}
	rejected, err = h.Reconcile(ctx, []prep.CallerSubmission{va, vb}, []prep.CallerReview{review})
	if err != nil || rejected.Targets[0].ReviewStatus != "REJECTED" || len(rejected.Leads) != 0 {
		t.Fatal("foreign review pin", err, rejected)
	}
	mixed := a
	mixed.Instability = b.Instability
	if _, err := prep.NewPostPreparation(ctx, []prep.Input{mixed}, limits, ownedPostLimits()); err == nil {
		t.Fatal("mixed assessment accepted")
	}
	aa, aap := ta()
	ab, abp := tb()
	if ba != aa || bap != aap || bb != ab || bbp != abp {
		t.Fatal("post calls performed traffic")
	}
	t.Log("50 indexed nominations; two prepared and 48 source gaps; independent equal root labels not merged")
}
