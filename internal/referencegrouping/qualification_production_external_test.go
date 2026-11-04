package referencegrouping_test

import (
	rg "lsp-trace/internal/referencegrouping"
	"testing"
)

type externalArtifactResolver struct{ calls int }

func (r *externalArtifactResolver) ResolveQualificationArtifact(role, digest string, byteLength int) ([]byte, error) {
	r.calls++
	return nil, nil
}

var _ rg.ProductionQualificationArtifactResolver = (*externalArtifactResolver)(nil)

func externalQualificationLimits() rg.ProductionQualificationLimits {
	return rg.ProductionQualificationLimits{
		Receipt:          rg.QualificationParseLimits{MaxRawBytes: 1 << 20, MaxCases: 32, MaxMetrics: 32, MaxCriteria: 32, MaxReviewers: 8, MaxStringBytes: 256},
		MaxArtifactBytes: 1 << 20, MaxTotalBytes: 8 << 20, MaxTokens: 100000, MaxDepth: 16, MaxRecords: 4096,
	}
}

// This imports the real package without overlays or private fixture authority.
// Construction and Verify accessibility are not evidence of actual qualification.
func TestProductionQualificationExternalConstructor(t *testing.T) {
	resolver := &externalArtifactResolver{}
	owner, err := rg.NewProductionQualificationOwner(externalQualificationLimits(), resolver)
	if err != nil || owner == nil {
		t.Fatalf("EXTERNAL_CONSTRUCTOR_FINITE_ARTIFACT_ONLY A-fail: owner=%v error=%v", owner, err)
	}
	policy, err := rg.BuildGroupingPolicy(rg.GroupingPolicyRequest{
		PolicyID: "CONTROLLED_EXTERNAL_CONSTRUCTION", Epoch: 108, EligibleKinds: []string{"CALLS", "REFERENCES_SYMBOL", "RESOLVES_TO_DEFINITION"},
		Rules: []rg.GroupingPolicyRule{
			{RelationKind: "CALLS", Action: "INCLUDE", RationaleCode: "CONTROLLED_INCLUDE", Weight: rg.Rational{Numerator: 1, Denominator: 1}},
			{RelationKind: "REFERENCES_SYMBOL", Action: "INCLUDE", RationaleCode: "CONTROLLED_INCLUDE", Weight: rg.Rational{Numerator: 1, Denominator: 1}},
			{RelationKind: "RESOLVES_TO_DEFINITION", Action: "INCLUDE", RationaleCode: "CONTROLLED_INCLUDE", Weight: rg.Rational{Numerator: 1, Denominator: 1}},
		},
		Limits: rg.GroupingPolicyLimits{MaxRules: 8, MaxCanonicalBytes: 1 << 16, MaxStringBytes: 256, MaxAbsoluteNumerator: 1024, MaxDenominator: 1024, MaxCap: 32},
	})
	if err != nil {
		t.Fatal(err)
	}
	q, err := owner.Verify(rg.GroupingPolicyQualificationReceipt{}, policy, rg.AdmittedInput{}, 108)
	if err == nil || q != (rg.VerifiedQualification{}) || resolver.calls != 0 || q.ValidAt(108) {
		t.Fatalf("EXTERNAL_CONSTRUCTOR_FINITE_ARTIFACT_ONLY A-fail: default issued or resolved missing input: q=%v error=%v calls=%d", q, err, resolver.calls)
	}
	if invalid, err := rg.NewProductionQualificationOwner(externalQualificationLimits(), nil); err == nil || invalid != nil {
		t.Fatal("EXTERNAL_CONSTRUCTOR_FINITE_ARTIFACT_ONLY A-fail: nil resolver capability")
	}
	limits := externalQualificationLimits()
	limits.MaxRecords = 0
	if invalid, err := rg.NewProductionQualificationOwner(limits, resolver); err == nil || invalid != nil {
		t.Fatal("EXTERNAL_CONSTRUCTOR_FINITE_ARTIFACT_ONLY A-fail: nonfinite limits")
	}
	t.Log("EXTERNAL_CONSTRUCTOR_FINITE_ARTIFACT_ONLY A-pass")
}
