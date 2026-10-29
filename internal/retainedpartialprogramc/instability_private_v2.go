package retainedpartialprogramc

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	pilot "lsp-trace/internal/censuslineagepilot"
	"lsp-trace/internal/programc"
)

const InstabilityProfilePrivateV2 = "DERIVED_RETAINED_PARTIAL_INSTABILITY_PRIVATE_V2"
const InstabilityPolicyPrivateV2 = "private-instability-v2;sealed-per-batch-projection-v2;packet-report-source-originalV3-derivedV5-seeds-origin-policy-pins;ordered-unique-seeds-2..6;one-run-per-seed;all-unordered-pairs<=15;RuleB-exact-rational-matching;m<63;n*2^m<=1000000;overflow-safe-division-preflight;rates<=0.05;VI<=0.10;minimum-Jaccard>=0.80;one-independent-source-per-batch;no-crossgraph-matching;no-replay;all-empty-zero-occurrence=EMPTY;empty-pairs=NOT_APPLICABLE_EMPTY;failure-or-cancel=unsealed-INCOMPLETE;retain-all-declared-rows;report<=16777216;authority=0;accepted=false;partial=true;source=UNKNOWN;feature=UNRESOLVED;absence=PROHIBITED"

func InstabilityPolicyDigestPrivateV2() string {
	return pilot.Digest([]byte(InstabilityPolicyPrivateV2))
}

// Report is detached finite diagnostic data, not a constructor or assessment seal.
// Source binds every packet/batch/raw/derived/metadata/policy pin. Multiple
// batches from the same origin do not become independent sources/workloads.
type InstabilityReportPrivateV2 struct {
	Source                                                                    SourceBindingPrivateV2
	Profile, PolicySHA256, RequestSHA256, Digest                              string
	ClaimCeiling, Admission, SourceCompleteness, FeatureIdentity              string
	Authority                                                                 int
	Accepted, Partial, CanonicalComplete, Replay                              bool
	IndependentSources                                                        int
	Seeds                                                                     []uint64
	Runs                                                                      []InstabilityRunPrivate
	Pairs                                                                     []InstabilityPairPrivate
	ExpectedRuns, CompleteRuns, EmptyRuns, FailedRuns, NotAttemptedRuns       int
	ExpectedPairs, CompletePairs, BlockedPairs, EmptyPairs, NotAttemptedPairs int
	Outcome, Error                                                            string
}

// Opaque immutable success capability, distinct from V1's retained DTO. Zero,
// JSON, a consistent report and a foreign projection state cannot mint it.
type AssessmentPrivateV2 struct{ state *assessmentStatePrivateV2 }
type assessmentStatePrivateV2 struct {
	self       *assessmentStatePrivateV2
	projection *projectionStatePrivateV2
	raw        []byte
	digest     string
}

func (AssessmentPrivateV2) MarshalJSON() ([]byte, error) {
	return nil, fmt.Errorf("opaque V2 assessment")
}
func (a AssessmentPrivateV2) valid() bool {
	return a.state != nil && a.state.self == a.state && len(a.state.raw) > 0 && len(a.state.raw) <= 16<<20 && a.state.digest == pilot.Digest(a.state.raw)
}
func (a AssessmentPrivateV2) Bytes() []byte {
	if !a.valid() {
		return nil
	}
	return bytes.Clone(a.state.raw)
}
func (a AssessmentPrivateV2) Report() InstabilityReportPrivateV2 {
	var r InstabilityReportPrivateV2
	if a.valid() {
		_ = json.Unmarshal(a.state.raw, &r)
	}
	return r
}
func newAssessmentMathPrivateV2(p ProjectionPrivateV2, seeds []uint64) InstabilityPrivate {
	s := p.state.evidence.Source
	a := InstabilityPrivate{Profile: InstabilityProfilePrivateV2, PolicySHA256: InstabilityPolicyDigestPrivateV2(), ProjectionProfile: ProfilePrivateV2, ProjectionDigest: ProfileDigestPrivateV2(), InputSHA256: s.PacketSHA256, NativeGraphSHA256: s.DerivedCanonicalV5SHA256, ClaimCeiling: ClaimCeilingPrivateV2, Admission: Admission, SourceCompleteness: "UNKNOWN", FeatureIdentity: "UNRESOLVED", Partial: true, IndependentSources: 1, Seeds: append([]uint64(nil), seeds...), ExpectedRuns: len(seeds), ExpectedPairs: len(seeds) * (len(seeds) - 1) / 2, Outcome: "INCOMPLETE"}
	a.RequestSHA256 = instabilityDigest(struct {
		Source SourceBindingPrivateV2
		Policy string
		Seeds  []uint64
	}{s, a.PolicySHA256, a.Seeds})
	for i, seed := range seeds {
		a.Runs = append(a.Runs, InstabilityRunPrivate{ID: fmt.Sprintf("run-%d-seed-%d", i, seed), Seed: seed, Status: "NOT_ATTEMPTED"})
	}
	for i := range seeds {
		for j := i + 1; j < len(seeds); j++ {
			a.Pairs = append(a.Pairs, InstabilityPairPrivate{Left: a.Runs[i].ID, Right: a.Runs[j].ID, Status: "NOT_ATTEMPTED"})
		}
	}
	return a
}
func reportPrivateV2(p ProjectionPrivateV2, a InstabilityPrivate) InstabilityReportPrivateV2 {
	a = CloneInstabilityPrivate(a)
	r := InstabilityReportPrivateV2{Source: p.Source(), Profile: a.Profile, PolicySHA256: a.PolicySHA256, RequestSHA256: a.RequestSHA256, ClaimCeiling: a.ClaimCeiling, Admission: a.Admission, SourceCompleteness: a.SourceCompleteness, FeatureIdentity: a.FeatureIdentity, Authority: a.Authority, Accepted: a.Accepted, Partial: a.Partial, CanonicalComplete: a.CanonicalComplete, Replay: a.Replay, IndependentSources: a.IndependentSources, Seeds: a.Seeds, Runs: a.Runs, Pairs: a.Pairs, ExpectedRuns: a.ExpectedRuns, CompleteRuns: a.CompleteRuns, EmptyRuns: a.EmptyRuns, FailedRuns: a.FailedRuns, NotAttemptedRuns: a.NotAttemptedRuns, ExpectedPairs: a.ExpectedPairs, CompletePairs: a.CompletePairs, BlockedPairs: a.BlockedPairs, EmptyPairs: a.EmptyPairs, NotAttemptedPairs: a.NotAttemptedPairs, Outcome: a.Outcome, Error: a.Error}
	r.Digest = instabilityDigest(r)
	return r
}

// Exactly one explicit run per requested seed, no retries or witness selection.
// Cancellation is cooperative between bounded operations, not a hard CPU limit.
func InstabilityComputePrivateV2(ctx context.Context, p ProjectionPrivateV2, seeds []uint64) (AssessmentPrivateV2, InstabilityReportPrivateV2, error) {
	return computeInstabilityPrivateV2(ctx, p, seeds, ComputeProjectionPrivateV2)
}
func ComputeInstabilityPrivateV2(ctx context.Context, p ProjectionPrivateV2, seeds []uint64) (AssessmentPrivateV2, InstabilityReportPrivateV2, error) {
	return InstabilityComputePrivateV2(ctx, p, seeds)
}

// Package-private dependency parameter is for controlled failure/cancel fixtures;
// it is not an exported compute injection or producer admission callback.
func computeInstabilityPrivateV2(ctx context.Context, p ProjectionPrivateV2, seeds []uint64, compute func(ProjectionPrivateV2, uint64) (OutcomePrivateV2, *programc.Failure)) (AssessmentPrivateV2, InstabilityReportPrivateV2, error) {
	if ctx == nil || !p.valid() {
		return AssessmentPrivateV2{}, InstabilityReportPrivateV2{}, fmt.Errorf("context and sealed V2 projection required")
	}
	if err := validSeeds(seeds); err != nil {
		return AssessmentPrivateV2{}, InstabilityReportPrivateV2{}, err
	}
	a := newAssessmentMathPrivateV2(p, seeds)
	fail := func(err error) (AssessmentPrivateV2, InstabilityReportPrivateV2, error) {
		return AssessmentPrivateV2{}, reportPrivateV2(p, finishPrivate(a, err)), err
	}
	for i, seed := range a.Seeds {
		if err := ctx.Err(); err != nil {
			return fail(err)
		}
		o, f := compute(p, seed)
		if f != nil {
			a.Runs[i].Status = "FAILED"
			a.Runs[i].Error = f.Error()
			return fail(f)
		}
		a.Runs[i].Partition = programc.ClonePartitionMathPrivate(o.Math)
		a.Runs[i].Status = o.Math.Outcome
		if err := ctx.Err(); err != nil {
			return fail(err)
		}
	}
	assessed, err := assessPrivateMath(ctx, p.state.math, newAssessmentMathPrivateV2(p, a.Seeds), a.Seeds, a.Runs)
	if err != nil {
		return AssessmentPrivateV2{}, reportPrivateV2(p, assessed), err
	}
	a = assessed
	if err = ctx.Err(); err != nil {
		return fail(err)
	}
	r := reportPrivateV2(p, a)
	raw, err := json.Marshal(r)
	if err != nil {
		return fail(err)
	}
	if len(raw) > 16<<20 {
		return fail(fmt.Errorf("V2 assessment byte cap"))
	}
	if err = ctx.Err(); err != nil {
		return fail(err)
	}
	st := &assessmentStatePrivateV2{projection: p.state, raw: bytes.Clone(raw), digest: pilot.Digest(raw)}
	st.self = st
	return AssessmentPrivateV2{st}, r, nil
}

// Recompute comparison metrics from retained partitions only. Never call Leiden,
// read a producer/root, or permit a JSON-consistent DTO to mint an assessment.
func ValidateInstabilityPrivateV2(ctx context.Context, p ProjectionPrivateV2, expected AssessmentPrivateV2, raw []byte) error {
	if ctx == nil || !p.valid() || !expected.valid() || expected.state.projection != p.state {
		return fmt.Errorf("sealed V2 projection-bound assessment required")
	}
	if len(raw) > 16<<20 || !bytes.Equal(raw, expected.state.raw) {
		return fmt.Errorf("V2 assessment canonical retained mismatch")
	}
	r := expected.Report()
	if r.Outcome == "INCOMPLETE" || len(r.Runs) > 6 || len(r.Pairs) > 15 || validSeeds(r.Seeds) != nil {
		return fmt.Errorf("completed V2 assessment required")
	}
	a, err := assessPrivateMath(ctx, p.state.math, newAssessmentMathPrivateV2(p, r.Seeds), r.Seeds, r.Runs)
	if err != nil {
		return err
	}
	want, err := json.Marshal(reportPrivateV2(p, a))
	if err != nil {
		return err
	}
	if !bytes.Equal(want, raw) {
		return fmt.Errorf("V2 assessment metric/batch binding mismatch")
	}
	return nil
}
