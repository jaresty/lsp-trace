//go:build darwin || linux

package retainedpartialprogramc

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	pilot "lsp-trace/internal/censuslineagepilot"
	"lsp-trace/internal/graph"
	"lsp-trace/internal/programc"
	"lsp-trace/sessionruntime"
)

const ProfilePrivateV3 = "DERIVED_PAGED_RETAINED_PARTIAL_CALLS_PRIVATE_V3"
const ProfileDefinitionPrivateV3 = "private-paged-v3;original-owner-selected-admission-v3;per-batch;original-native-v3-exact;derived-canonical-v5;directed-unit-occurrences;typed-zero-site=zero-occurrences;single-node=unique-trivial-partition-not-Leiden;original-denominator;shared-ledger-stage-reservations;authority=0;accepted=false;partial=true;source=UNKNOWN;feature=UNRESOLVED;absence=PROHIBITED;no-ordinary-custody;projection-alone-not-AST-admission;no-history-authentication;no-cross-process-restoration"
const InstabilityPolicyPrivateV3 = "private-instability-v3;original-owner-paged-selected-batch-v3;page-revision-scope-origin-role-policy-runtime-originalV3-derivedV5-pins;ordered-unique-seeds-2..6;single-node-nonempty=unique-trivial-partition-not-Leiden;otherwise-neutral-CallsMath;one-run-per-seed;all-unordered-pairs<=15;RuleB-exact-rational-matching;m<63;n*2^m<=1000000;overflow-safe-division-preflight;rates<=0.05;VI<=0.10;minimum-Jaccard>=0.80;one-independent-source-per-batch;no-crossgraph-matching;no-replay;all-empty-zero-occurrence=EMPTY;empty-pairs=NOT_APPLICABLE_EMPTY;failure-or-cancel=unsealed;retain-all-declared-rows;report<=16777216;shared-original-ledger;authority=0;accepted=false;partial=true;source=UNKNOWN;feature=UNRESOLVED;absence=PROHIBITED"
const ClaimCeilingPrivateV3 = "Private original-owned paged observations and derived CALLS math only; authority=0; accepted=false; feature=UNRESOLVED; source=UNKNOWN; absence=PROHIBITED. No FULL_DEFINITION, ADR0012, ordinary Native admission, authenticated source/history, runtime claim, permission or production authority."

type SourceBindingPrivateV3 struct {
	Admission                                                                                   string
	Partial                                                                                     bool
	Profile, ProfileSHA256, ClaimCeiling                                                        string
	PageBindingSHA256, PageRevisionSHA256, SourceScopeSHA256, EnumerationOriginSHA256           string
	BatchOrdinal, FullBatchCount, FullTargetCount, BatchTargetCount, SourceMemberCount          int
	SelectedObjectSHA256                                                                        [4]string
	SourcePolicySHA256, RuntimeMetadataSHA256, OriginalNativeV3SHA256, DerivedCanonicalV5SHA256 string
	SessionID, WorkspaceURI, PositionEncoding, ProviderCommand                                  string
	Generation                                                                                  uint64
	Authority                                                                                   int
	Accepted                                                                                    bool
	FeatureIdentity, SourceCompleteness, AbsenceInference                                       string
}
type ProjectionPrivateV3 struct{ state *projectionStatePrivateV3 }
type projectionStatePrivateV3 struct {
	self              *projectionStatePrivateV3
	selected          pilot.AdmittedPageBatchPrivateV3
	source            SourceBindingPrivateV3
	original, derived []byte
	math              programc.CallsMathPrivate
}
type ProjectionEvidencePrivateV3 struct {
	Source                               SourceBindingPrivateV3
	Selected                             pilot.AdmittedPageBatchViewPrivateV3
	OriginalNativeV3, DerivedCanonicalV5 []byte
	Math                                 programc.CallsMathPrivate
}

func (ProjectionPrivateV3) MarshalJSON() ([]byte, error) {
	return nil, fmt.Errorf("opaque V3 projection")
}
func (p ProjectionPrivateV3) valid() bool { return p.state != nil && p.state.self == p.state }
func ProjectPrivateV3(ctx context.Context, b pilot.AdmittedPageBatchPrivateV3, exchange pilot.PageCostPrivateV3) (ProjectionPrivateV3, error) {
	v, e := b.ReadStage(ctx, exchange)
	if e != nil {
		return ProjectionPrivateV3{}, e
	}
	var meta sessionruntime.SessionMetadata
	if json.Unmarshal(v.RuntimeMetadata, &meta) != nil || meta.ServerCommand == "" {
		return ProjectionPrivateV3{}, fmt.Errorf("V3 provider metadata")
	}
	raw, e := graph.ProjectRetainedV3ToV5Private(v.OriginalNativeV3, v.Source.Policy.WorkspaceURI, meta.ServerCommand)
	if e != nil {
		return ProjectionPrivateV3{}, e
	}
	m, f := programc.ProjectNativeCallsMathPrivateV2(raw)
	if f != nil {
		return ProjectionPrivateV3{}, f
	}
	if len(m.NodeIdentities) > 10000 || len(m.Occurrences) > 100000 {
		return ProjectionPrivateV3{}, pilot.ErrPagesUnitPrivateV3
	}
	for _, o := range m.Occurrences {
		if o.Provider != meta.ServerCommand {
			return ProjectionPrivateV3{}, fmt.Errorf("V3 provider mismatch")
		}
	}
	policy, _ := json.Marshal(v.Source.Policy)
	s := SourceBindingPrivateV3{Admission: pilot.SelectedAdmissionProfilePrivateV3, Partial: true, Profile: ProfilePrivateV3, ProfileSHA256: pilot.Digest([]byte(ProfileDefinitionPrivateV3)), ClaimCeiling: ClaimCeilingPrivateV3, PageBindingSHA256: v.Binding, PageRevisionSHA256: v.RevisionSHA256, SourceScopeSHA256: v.ScopeSHA256, EnumerationOriginSHA256: v.OriginSHA256, BatchOrdinal: v.Ordinal, FullBatchCount: v.FullBatchCount, FullTargetCount: v.FullTargetCount, BatchTargetCount: len(v.Source.Evidence.Acquisition.Targets), SourceMemberCount: len(v.Source.Policy.Members), SelectedObjectSHA256: v.ObjectSHA256, SourcePolicySHA256: pilot.Digest(policy), RuntimeMetadataSHA256: v.RuntimeSHA256, OriginalNativeV3SHA256: pilot.Digest(v.OriginalNativeV3), DerivedCanonicalV5SHA256: pilot.Digest(raw), SessionID: v.Source.Policy.SessionID, WorkspaceURI: v.Source.Policy.WorkspaceURI, PositionEncoding: meta.PositionEncoding, ProviderCommand: meta.ServerCommand, Generation: v.Source.Policy.Generation, FeatureIdentity: "UNRESOLVED", SourceCompleteness: "UNKNOWN", AbsenceInference: "PROHIBITED"}
	if e = ctx.Err(); e != nil {
		return ProjectionPrivateV3{}, e
	}
	st := &projectionStatePrivateV3{selected: b, source: s, original: v.OriginalNativeV3, derived: raw, math: m}
	st.self = st
	return ProjectionPrivateV3{st}, nil
}

// Every detached read is explicitly charged under the same original ledger.
func (p ProjectionPrivateV3) Read(ctx context.Context, exchange pilot.PageCostPrivateV3) (ProjectionEvidencePrivateV3, error) {
	if !p.valid() {
		return ProjectionEvidencePrivateV3{}, fmt.Errorf("sealed V3 projection required")
	}
	v, e := p.state.selected.ReadStage(ctx, exchange)
	if e != nil {
		return ProjectionEvidencePrivateV3{}, e
	}
	if v.RevisionSHA256 != p.state.source.PageRevisionSHA256 || v.ObjectSHA256 != p.state.source.SelectedObjectSHA256 || !bytes.Equal(v.OriginalNativeV3, p.state.original) {
		return ProjectionEvidencePrivateV3{}, fmt.Errorf("V3 own source binding")
	}
	return ProjectionEvidencePrivateV3{p.state.source, v, bytes.Clone(p.state.original), bytes.Clone(p.state.derived), programc.CloneCallsMathPrivate(p.state.math)}, nil
}
func (p ProjectionPrivateV3) BudgetSpent() pilot.PageCostPrivateV3 {
	if !p.valid() {
		return pilot.PageCostPrivateV3{}
	}
	return p.state.selected.BudgetSpent()
}
func (p ProjectionPrivateV3) OwnSource(ctx context.Context, uri string, exchange pilot.PageCostPrivateV3) ([]byte, error) {
	if !p.valid() {
		return nil, pilot.ErrInvalid
	}
	return p.state.selected.OwnSource(ctx, uri, exchange)
}
func (p ProjectionPrivateV3) RetainPreparedBytes(ctx context.Context, raw []byte, exchange pilot.PageCostPrivateV3) (string, error) {
	if !p.valid() {
		return "", pilot.ErrInvalid
	}
	return p.state.selected.RetainOutput(ctx, raw, exchange)
}

type AssessmentPrivateV3 struct{ state *assessmentStatePrivateV3 }
type assessmentStatePrivateV3 struct {
	self       *assessmentStatePrivateV3
	projection *projectionStatePrivateV3
	raw        []byte
}
type InstabilityReportPrivateV3 struct {
	Source SourceBindingPrivateV3
	Math   InstabilityPrivate
}

func (AssessmentPrivateV3) MarshalJSON() ([]byte, error) {
	return nil, fmt.Errorf("opaque V3 assessment")
}
func assessmentMathPrivateV3(s SourceBindingPrivateV3, seeds []uint64) InstabilityPrivate {
	a := InstabilityPrivate{Profile: "DERIVED_PAGED_RETAINED_PARTIAL_INSTABILITY_PRIVATE_V3", PolicySHA256: pilot.Digest([]byte(InstabilityPolicyPrivateV3)), ProjectionProfile: ProfilePrivateV3, ProjectionDigest: s.ProfileSHA256, InputSHA256: s.PageRevisionSHA256, NativeGraphSHA256: s.DerivedCanonicalV5SHA256, ClaimCeiling: ClaimCeilingPrivateV3, Admission: pilot.SelectedAdmissionProfilePrivateV3, SourceCompleteness: "UNKNOWN", FeatureIdentity: "UNRESOLVED", Partial: true, IndependentSources: 1, Seeds: append([]uint64(nil), seeds...), ExpectedRuns: len(seeds), ExpectedPairs: len(seeds) * (len(seeds) - 1) / 2, Outcome: "INCOMPLETE"}
	a.RequestSHA256 = instabilityDigest(struct {
		Source SourceBindingPrivateV3
		Seeds  []uint64
	}{s, seeds})
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
func ComputeInstabilityPrivateV3(ctx context.Context, p ProjectionPrivateV3, seeds []uint64, exchange pilot.PageCostPrivateV3) (AssessmentPrivateV3, error) {
	v, e := p.Read(ctx, exchange)
	if e != nil {
		return AssessmentPrivateV3{}, e
	}
	if e = validSeeds(seeds); e != nil {
		return AssessmentPrivateV3{}, e
	}
	a := assessmentMathPrivateV3(v.Source, seeds)
	for i, seed := range seeds {
		if e = ctx.Err(); e != nil {
			return AssessmentPrivateV3{}, e
		}
		var m programc.PartitionMathPrivate
		if len(v.Math.NodeIdentities) == 1 && len(v.Math.Occurrences) > 0 {
			communities := []programc.Community{{Members: []string{v.Math.NodeIdentities[0]}}}
			m = programc.PartitionMathPrivate{Outcome: "COMPLETE", Algorithm: "TRIVIAL_SINGLE_NODE_PRIVATE_V3", Resolution: 1, Seed: seed, Communities: communities, LogicalDigest: programc.PartitionDigestMathPrivate(communities)}
		} else {
			var f *programc.Failure
			m, f = programc.PartitionCallsMathPrivate(v.Math, seed)
			if f != nil {
				return AssessmentPrivateV3{}, f
			}
		}
		a.Runs[i].Partition = m
		a.Runs[i].Status = m.Outcome
	}
	a, e = assessPrivateMath(ctx, v.Math, assessmentMathPrivateV3(v.Source, seeds), seeds, a.Runs)
	if e != nil {
		return AssessmentPrivateV3{}, e
	}
	raw, e := marshalAssessmentPrivateV3(ctx, InstabilityReportPrivateV3{v.Source, a}, nil)
	if e != nil {
		return AssessmentPrivateV3{}, e
	}
	if e = ctx.Err(); e != nil {
		return AssessmentPrivateV3{}, e
	}
	st := &assessmentStatePrivateV3{projection: p.state, raw: raw}
	st.self = st
	return AssessmentPrivateV3{st}, nil
}
func ValidateInstabilityPrivateV3(ctx context.Context, p ProjectionPrivateV3, a AssessmentPrivateV3, exchange pilot.PageCostPrivateV3) (InstabilityReportPrivateV3, []byte, error) {
	if !p.valid() || a.state == nil || a.state.self != a.state || a.state.projection != p.state {
		return InstabilityReportPrivateV3{}, nil, fmt.Errorf("sealed V3 projection-bound assessment required")
	}
	v, e := p.Read(ctx, exchange)
	if e != nil {
		return InstabilityReportPrivateV3{}, nil, e
	}
	var r InstabilityReportPrivateV3
	if json.Unmarshal(a.state.raw, &r) != nil || r.Math.Outcome == "INCOMPLETE" || validSeeds(r.Math.Seeds) != nil {
		return r, nil, pilot.ErrInvalid
	}
	assessed, e := assessPrivateMath(ctx, v.Math, assessmentMathPrivateV3(v.Source, r.Math.Seeds), r.Math.Seeds, r.Math.Runs)
	if e != nil {
		return r, nil, e
	}
	raw, e := marshalAssessmentPrivateV3(ctx, InstabilityReportPrivateV3{v.Source, assessed}, nil)
	if e != nil {
		return r, nil, e
	}
	if !bytes.Equal(raw, a.state.raw) {
		return r, nil, pilot.ErrInvalid
	}
	return r, bytes.Clone(raw), nil
}
