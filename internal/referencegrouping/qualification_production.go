package referencegrouping

import (
	"fmt"
	"math"
	"reflect"
)

// qualificationOwner is a private artifact consumer, not an acquisition backend.
// It does not authenticate a producer or resist privileged same-process forgery.
type qualificationOwner struct {
	limits    productionQualificationLimits
	resolver  qualificationArtifactResolver
	approvals []qualificationApproval
}

// Implementations must return immutable artifact bytes for the exact role and
// commitment, without acquisition, network, checkout, or fallback resolution.
// Returned bytes remain untrusted; only the code-owned declarations authorize.
type qualificationArtifactResolver interface {
	ResolveQualificationArtifact(role, digest string, byteLength int) ([]byte, error)
}

type productionQualificationLimits struct {
	Receipt                                                          QualificationParseLimits
	MaxArtifactBytes, MaxTotalBytes, MaxTokens, MaxDepth, MaxRecords int
}

// ProductionQualificationLimits supplies finite bounds for private production
// qualification verification. No default bounds or authority are implied.
type ProductionQualificationLimits = productionQualificationLimits

// ProductionQualificationArtifactResolver supplies only immutable artifact bytes;
// it cannot supply maintainer authorization or acquire missing evidence.
type ProductionQualificationArtifactResolver = qualificationArtifactResolver

// NewProductionQualificationOwner constructs an opaque verifier with code-owned,
// currently UNISSUED authority. Its only inputs are limits and an artifact resolver.
func NewProductionQualificationOwner(l ProductionQualificationLimits, resolver ProductionQualificationArtifactResolver) (*qualificationOwner, error) {
	return newQualificationOwner(l, resolver)
}

type qualificationFailure struct{ Outcome, Reason string }

func (e *qualificationFailure) Error() string     { return fmt.Sprintf("%s: %s", e.Outcome, e.Reason) }
func qualificationIncomplete(reason string) error { return &qualificationFailure{"INCOMPLETE", reason} }

func newQualificationOwner(l productionQualificationLimits, resolver qualificationArtifactResolver) (*qualificationOwner, error) {
	if err := validateProductionQualificationLimits(l, resolver); err != nil {
		return nil, err
	}
	return &qualificationOwner{limits: l, resolver: resolver, approvals: issuedQualificationApprovals()}, nil
}
func validateProductionQualificationLimits(l productionQualificationLimits, resolver qualificationArtifactResolver) error {
	if resolver == nil || l.MaxArtifactBytes <= 0 || l.MaxTotalBytes <= 0 || l.MaxTokens <= 0 || l.MaxDepth <= 0 || l.MaxRecords <= 0 || l.Receipt.MaxRawBytes <= 0 || l.Receipt.MaxCases <= 0 || l.Receipt.MaxMetrics <= 0 || l.Receipt.MaxCriteria <= 0 || l.Receipt.MaxReviewers <= 0 || l.Receipt.MaxStringBytes <= 0 {
		return qualificationIncomplete("finite explicit limits and artifact resolver required")
	}
	return nil
}

func (o *qualificationOwner) Verify(receipt GroupingPolicyQualificationReceipt, policy GroupingPolicy, input AdmittedInput, epoch uint64) (VerifiedQualification, error) {
	fail := func(reason string) (VerifiedQualification, error) {
		return VerifiedQualification{}, qualificationIncomplete(reason)
	}
	if o == nil {
		return fail("owner missing")
	}
	// Bounds precede parsing, canonical copies, artifact resolution, and authority work.
	l := o.limits
	if err := validateProductionQualificationLimits(l, o.resolver); err != nil {
		return VerifiedQualification{}, err
	}
	d := receipt.doc
	if len(receipt.canonical) == 0 || len(receipt.canonical) > l.Receipt.MaxRawBytes || policy.ByteLength() == 0 || input.ByteLength() == 0 || len(d.Cases) > l.Receipt.MaxCases || len(d.Metrics) > l.Receipt.MaxMetrics || len(d.Criteria) > l.Receipt.MaxCriteria || len(d.Reviewers) > l.Receipt.MaxReviewers || len(o.approvals) > l.MaxRecords {
		return fail("preflight bounds")
	}
	commitments := [5]qualificationCommitment{d.EvaluationPlan, d.CorpusManifest, d.Implementation, d.Environment, d.Report}
	roles := [5]string{"evaluation-plan", "corpus-manifest", "implementation", "environment", "report"}
	total, ok := qualificationCheckedSum(len(receipt.canonical), policy.ByteLength(), input.ByteLength())
	if !ok || total > l.MaxTotalBytes {
		return fail("total byte bound")
	}
	for j, c := range commitments {
		if c.ID != roles[j] || c.ByteLength <= 0 || c.ByteLength > l.MaxArtifactBytes || !validDigest(c.Digest) {
			return fail("artifact commitment/bound")
		}
		total, ok = qualificationCheckedSum(total, c.ByteLength)
		if !ok || total > l.MaxTotalBytes {
			return fail("total byte bound")
		}
	}
	if !qualificationAccountingChecked(d) {
		return fail("checked accounting")
	}
	if len(d.Cases) != len(requiredArms) || len(d.Metrics) != len(requiredMetricCodes) || len(d.Criteria) != len(requiredMetricCodes) || len(d.Cases) > l.MaxRecords {
		return fail("complete bounded case/metric/criteria inventory")
	}
	if err := qualificationJSONPreflight(receipt.canonical, l); err != nil {
		return VerifiedQualification{}, err
	}
	parsed, err := ParseGroupingPolicyQualificationReceipt(receipt.canonical, l.Receipt)
	if err != nil || !reflect.DeepEqual(parsed.doc, d) {
		return fail("receipt representation")
	}
	if d.Policy != (qualificationCommitment{policy.ID(), policy.ByteLength(), policy.Digest()}) || d.GroupingInput != (qualificationCommitment{input.ID(), input.ByteLength(), input.Digest()}) || epoch == 0 || epoch < d.ValidFromEpoch || epoch > d.ValidThroughEpoch {
		return fail("subject or current validity")
	}
	approval, err := o.authorize(parsed, commitments)
	if err != nil {
		return VerifiedQualification{}, err
	}
	artifacts := [5][]byte{}
	for j, c := range commitments {
		raw, e := o.resolver.ResolveQualificationArtifact(roles[j], c.Digest, c.ByteLength)
		if e != nil || len(raw) != c.ByteLength {
			return fail("independent artifact commitment")
		}
		// Copy only after the received length and all declared budgets are checked.
		artifacts[j] = append([]byte(nil), raw...)
		if receiptDigest(artifacts[j]) != c.Digest {
			return fail("independent artifact commitment")
		}
	}
	if err = verifyProductionQualificationArtifacts(parsed.doc, artifacts, approval, l); err != nil {
		return VerifiedQualification{}, err
	}
	switch parsed.outcome {
	case "REJECTED":
		return VerifiedQualification{}, &qualificationFailure{"REJECTED", "completed evaluation or maintainer decision declined"}
	case "QUALIFIED":
	default:
		return fail("existing qualification criteria/case semantics")
	}
	sum := receiptDigest(parsed.canonical)
	return VerifiedQualification{policyID: policy.ID(), policyDigest: policy.Digest(), policyByteLength: policy.ByteLength(), inputID: input.ID(), inputDigest: input.Digest(), inputByteLength: input.ByteLength(), validFromEpoch: d.ValidFromEpoch, validThroughEpoch: d.ValidThroughEpoch, receiptID: sum, receiptDigest: sum, receiptByteLength: len(parsed.canonical)}, nil
}

func (o *qualificationOwner) authorize(r GroupingPolicyQualificationReceipt, evidence [5]qualificationCommitment) (qualificationApproval, error) {
	var found qualificationApproval
	count := 0
	rc := qualificationCommitment{"receipt", len(r.canonical), receiptDigest(r.canonical)}
	for _, a := range o.approvals {
		if a.Receipt == rc {
			found = a
			count++
		}
	}
	d := r.doc
	if count != 1 || found.Policy != d.Policy || found.Input != d.GroupingInput || found.Evidence != evidence || found.RequiredClass != d.RequiredReviewerClass || found.RequiredAcceptances != d.RequiredAcceptances || found.RequiredAcceptances <= 0 || found.ValidFromEpoch != d.ValidFromEpoch || found.ValidThroughEpoch != d.ValidThroughEpoch || found.DecisionUnix != d.DecisionUnix || found.FrozenUnix <= 0 || found.FrozenUnix >= found.DecisionUnix || len(found.Reviewers) > o.limits.Receipt.MaxReviewers || !reflect.DeepEqual(found.Reviewers, d.Reviewers) {
		return qualificationApproval{}, qualificationIncomplete("independent maintainer authorization missing or mismatched")
	}
	return found, nil
}

func qualificationCheckedSum(values ...int) (int, bool) {
	sum := 0
	for _, v := range values {
		if v < 0 || v > math.MaxInt-sum {
			return 0, false
		}
		sum += v
	}
	return sum, true
}
func qualificationAccountingChecked(d qualificationDocument) bool {
	var totals [6]int
	for _, c := range d.Cases {
		values := [6]int{c.Candidates, c.Evaluated, c.Passed, c.Rejected, c.Excluded, c.Failed}
		evaluated, ok := qualificationCheckedSum(c.Passed, c.Rejected)
		if !ok || evaluated != c.Evaluated {
			return false
		}
		candidates, ok := qualificationCheckedSum(c.Evaluated, c.Excluded, c.Failed)
		if !ok || candidates != c.Candidates {
			return false
		}
		for j, v := range values {
			n, ok := qualificationCheckedSum(totals[j], v)
			if !ok {
				return false
			}
			totals[j] = n
		}
	}
	return totals == [6]int{d.Accounting.Candidates, d.Accounting.Evaluated, d.Accounting.Passed, d.Accounting.Rejected, d.Accounting.Excluded, d.Accounting.Failed}
}
