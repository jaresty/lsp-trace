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

const ProfileRetainedCheckpointPrivateV3 = "DERIVED_REOPENED_RETAINED_OBSERVATIONAL_CALLS_PRIVATE_V3_1"
const ClaimCeilingRetainedCheckpointPrivateV3 = "REOPENED_RETAINED_OBSERVATIONS_NOT_ORIGINAL_CAPTURE_OWNERSHIP; authority=0; accepted=false; source=UNKNOWN; feature=UNRESOLVED; absence=PROHIBITED; retained runtime descriptor is observational, not a new provider response; hashes are not authentication/history/permission; no ordinary Native, ADR0012, FULL_DEFINITION or production authority"
const checkpointMathPolicyPrivateV3 = "reopened-retained-observational-v3.1;exact-native-v3;derived-v5;neutral-directed-occurrence-math;one-node=trivial-not-Leiden;seeds2..6;original-denominators;shared-durable-ledger;bounded-assessment-serializer-v3;not-original-capture-ownership"

// This distinct source DTO is not an original-owner projection or proof.
type SourceBindingRetainedCheckpointPrivateV3 struct {
	SourceBindingPrivateV3
	CheckpointSHA256 string
	InputFamily      string
}
type ProjectionRetainedCheckpointPrivateV3 struct {
	state *projectionCheckpointStatePrivateV3
}
type projectionCheckpointStatePrivateV3 struct {
	self              *projectionCheckpointStatePrivateV3
	selected          pilot.RetainedSelectedBatchPrivateV3
	source            SourceBindingRetainedCheckpointPrivateV3
	original, derived []byte
	math              programc.CallsMathPrivate
}
type ProjectionEvidenceRetainedCheckpointPrivateV3 struct {
	Source                               SourceBindingRetainedCheckpointPrivateV3
	Selected                             pilot.RetainedSelectedBatchViewPrivateV3
	OriginalNativeV3, DerivedCanonicalV5 []byte
	Math                                 programc.CallsMathPrivate
}

func (ProjectionRetainedCheckpointPrivateV3) MarshalJSON() ([]byte, error) {
	return nil, pilot.ErrInvalid
}
func (p ProjectionRetainedCheckpointPrivateV3) valid() bool {
	return p.state != nil && p.state.self == p.state
}
func ProjectRetainedCheckpointPrivateV3(ctx context.Context, b pilot.RetainedSelectedBatchPrivateV3, exchange pilot.PageCostPrivateV3) (ProjectionRetainedCheckpointPrivateV3, error) {
	v, e := b.Read(ctx, exchange)
	if e != nil {
		return ProjectionRetainedCheckpointPrivateV3{}, e
	}
	if v.Profile != pilot.RetainedCheckpointProfilePrivateV3 {
		return ProjectionRetainedCheckpointPrivateV3{}, pilot.ErrInvalid
	}
	var meta sessionruntime.SessionMetadata
	if json.Unmarshal(v.RuntimeMetadata, &meta) != nil || meta.ServerCommand == "" {
		return ProjectionRetainedCheckpointPrivateV3{}, fmt.Errorf("retained runtime descriptor")
	}
	raw, e := graph.ProjectRetainedV3ToV5Private(v.OriginalNativeV3, v.Source.Policy.WorkspaceURI, meta.ServerCommand)
	if e != nil {
		return ProjectionRetainedCheckpointPrivateV3{}, e
	}
	m, f := programc.ProjectNativeCallsMathPrivateV2(raw)
	if f != nil {
		return ProjectionRetainedCheckpointPrivateV3{}, f
	}
	if len(m.NodeIdentities) > 10000 || len(m.Occurrences) > 100000 {
		return ProjectionRetainedCheckpointPrivateV3{}, pilot.ErrPagesUnitPrivateV3
	}
	for _, o := range m.Occurrences {
		if o.Provider != meta.ServerCommand {
			return ProjectionRetainedCheckpointPrivateV3{}, pilot.ErrInvalid
		}
	}
	policy, _ := json.Marshal(v.Source.Policy)
	base := SourceBindingPrivateV3{Admission: pilot.RetainedCheckpointProfilePrivateV3, Partial: true, Profile: ProfileRetainedCheckpointPrivateV3, ProfileSHA256: pilot.Digest([]byte(checkpointMathPolicyPrivateV3)), ClaimCeiling: ClaimCeilingRetainedCheckpointPrivateV3, PageBindingSHA256: v.Binding, PageRevisionSHA256: v.RevisionSHA256, SourceScopeSHA256: v.ScopeSHA256, EnumerationOriginSHA256: v.OriginSHA256, BatchOrdinal: v.Ordinal, FullBatchCount: v.FullBatchCount, FullTargetCount: v.FullTargetCount, BatchTargetCount: len(v.Source.Evidence.Acquisition.Targets), SourceMemberCount: len(v.Source.Policy.Members), SelectedObjectSHA256: v.ObjectSHA256, SourcePolicySHA256: pilot.Digest(policy), RuntimeMetadataSHA256: v.RuntimeSHA256, OriginalNativeV3SHA256: pilot.Digest(v.OriginalNativeV3), DerivedCanonicalV5SHA256: pilot.Digest(raw), SessionID: v.Source.Policy.SessionID, WorkspaceURI: v.Source.Policy.WorkspaceURI, PositionEncoding: meta.PositionEncoding, ProviderCommand: meta.ServerCommand, Generation: v.Source.Policy.Generation, FeatureIdentity: "UNRESOLVED", SourceCompleteness: "UNKNOWN", AbsenceInference: "PROHIBITED"}
	s := &projectionCheckpointStatePrivateV3{selected: b, source: SourceBindingRetainedCheckpointPrivateV3{base, v.CheckpointSHA256, pilot.RetainedCheckpointProfilePrivateV3}, original: v.OriginalNativeV3, derived: raw, math: m}
	s.self = s
	if e = ctx.Err(); e != nil {
		return ProjectionRetainedCheckpointPrivateV3{}, e
	}
	return ProjectionRetainedCheckpointPrivateV3{s}, nil
}
func (p ProjectionRetainedCheckpointPrivateV3) Read(ctx context.Context, exchange pilot.PageCostPrivateV3) (ProjectionEvidenceRetainedCheckpointPrivateV3, error) {
	if !p.valid() {
		return ProjectionEvidenceRetainedCheckpointPrivateV3{}, pilot.ErrInvalid
	}
	s := p.state
	v, e := s.selected.Read(ctx, exchange)
	if e != nil {
		return ProjectionEvidenceRetainedCheckpointPrivateV3{}, e
	}
	if v.CheckpointSHA256 != s.source.CheckpointSHA256 || v.ObjectSHA256 != s.source.SelectedObjectSHA256 || !bytes.Equal(v.OriginalNativeV3, s.original) {
		return ProjectionEvidenceRetainedCheckpointPrivateV3{}, pilot.ErrInvalid
	}
	return ProjectionEvidenceRetainedCheckpointPrivateV3{s.source, v, bytes.Clone(s.original), bytes.Clone(s.derived), programc.CloneCallsMathPrivate(s.math)}, ctx.Err()
}
func (p ProjectionRetainedCheckpointPrivateV3) BudgetSpent() pilot.PageCostPrivateV3 {
	if !p.valid() {
		return pilot.PageCostPrivateV3{}
	}
	return p.state.selected.BudgetSpent()
}
func (p ProjectionRetainedCheckpointPrivateV3) OwnSource(ctx context.Context, uri string, exchange pilot.PageCostPrivateV3) ([]byte, error) {
	if !p.valid() {
		return nil, pilot.ErrInvalid
	}
	return p.state.selected.OwnSource(ctx, uri, exchange)
}
func (p ProjectionRetainedCheckpointPrivateV3) RetainPreparedBytes(ctx context.Context, raw []byte, exchange pilot.PageCostPrivateV3) (string, error) {
	if !p.valid() {
		return "", pilot.ErrInvalid
	}
	return p.state.selected.RetainOutput(ctx, raw, exchange)
}

type AssessmentRetainedCheckpointPrivateV3 struct {
	state *assessmentCheckpointStatePrivateV3
}
type assessmentCheckpointStatePrivateV3 struct {
	self       *assessmentCheckpointStatePrivateV3
	projection *projectionCheckpointStatePrivateV3
	raw        []byte
}
type InstabilityReportRetainedCheckpointPrivateV3 struct {
	Source SourceBindingRetainedCheckpointPrivateV3
	Math   InstabilityPrivate
}

func (AssessmentRetainedCheckpointPrivateV3) MarshalJSON() ([]byte, error) {
	return nil, pilot.ErrInvalid
}
func checkpointAssessmentSkeletonPrivateV3(s SourceBindingRetainedCheckpointPrivateV3, seeds []uint64) InstabilityPrivate {
	// This helper constructs detached math rows, never a projection owner.
	a := assessmentMathPrivateV3(s.SourceBindingPrivateV3, seeds)
	a.Profile = "DERIVED_REOPENED_RETAINED_OBSERVATIONAL_INSTABILITY_PRIVATE_V3_1"
	a.ProjectionProfile = ProfileRetainedCheckpointPrivateV3
	a.PolicySHA256 = pilot.Digest([]byte(checkpointMathPolicyPrivateV3))
	a.ClaimCeiling = ClaimCeilingRetainedCheckpointPrivateV3
	a.Admission = pilot.RetainedCheckpointProfilePrivateV3
	a.RequestSHA256 = instabilityDigest(struct {
		Source SourceBindingRetainedCheckpointPrivateV3
		Seeds  []uint64
	}{s, seeds})
	return a
}
func marshalCheckpointAssessmentPrivateV3(ctx context.Context, r InstabilityReportRetainedCheckpointPrivateV3) ([]byte, error) {
	// Apply the same pure conservative field-graph admission to the detached
	// common shape; this does not serialize or mint an original-owner report.
	if len(r.Source.CheckpointSHA256) > 71 || len(r.Source.InputFamily) > 128 || r.Source.InputFamily != pilot.RetainedCheckpointProfilePrivateV3 {
		return nil, pilot.ErrInvalid
	}
	// The old admitted bound may be conservative but must leave independent room
	// for the two new JSON string leaves before allocating the new encoded report.
	n, e := assessmentEncodedBoundPrivateV3(ctx, InstabilityReportPrivateV3{r.Source.SourceBindingPrivateV3, r.Math})
	if e != nil {
		return nil, e
	}
	extra := 128 + 6*len(r.Source.CheckpointSHA256) + 6*len(r.Source.InputFamily)
	if n > (16<<20)-extra {
		return nil, pilot.ErrPagesUnitPrivateV3
	}
	raw, e := json.Marshal(r)
	if e != nil {
		return nil, e
	}
	if len(raw) > 16<<20 {
		return nil, pilot.ErrPagesUnitPrivateV3
	}
	return raw, ctx.Err()
}
func ComputeInstabilityRetainedCheckpointPrivateV3(ctx context.Context, p ProjectionRetainedCheckpointPrivateV3, seeds []uint64, exchange pilot.PageCostPrivateV3) (AssessmentRetainedCheckpointPrivateV3, error) {
	v, e := p.Read(ctx, exchange)
	if e != nil {
		return AssessmentRetainedCheckpointPrivateV3{}, e
	}
	if e = validSeeds(seeds); e != nil {
		return AssessmentRetainedCheckpointPrivateV3{}, e
	}
	a := checkpointAssessmentSkeletonPrivateV3(v.Source, seeds)
	for i, seed := range seeds {
		if e = ctx.Err(); e != nil {
			return AssessmentRetainedCheckpointPrivateV3{}, e
		}
		var m programc.PartitionMathPrivate
		if len(v.Math.NodeIdentities) == 1 && len(v.Math.Occurrences) > 0 {
			cs := []programc.Community{{Members: []string{v.Math.NodeIdentities[0]}}}
			m = programc.PartitionMathPrivate{Outcome: "COMPLETE", Algorithm: "TRIVIAL_SINGLE_NODE_PRIVATE_V3", Resolution: 1, Seed: seed, Communities: cs, LogicalDigest: programc.PartitionDigestMathPrivate(cs)}
		} else {
			var f *programc.Failure
			m, f = programc.PartitionCallsMathPrivate(v.Math, seed)
			if f != nil {
				return AssessmentRetainedCheckpointPrivateV3{}, f
			}
		}
		a.Runs[i].Partition = m
		a.Runs[i].Status = m.Outcome
	}
	a, e = assessPrivateMath(ctx, v.Math, checkpointAssessmentSkeletonPrivateV3(v.Source, seeds), seeds, a.Runs)
	if e != nil {
		return AssessmentRetainedCheckpointPrivateV3{}, e
	}
	raw, e := marshalCheckpointAssessmentPrivateV3(ctx, InstabilityReportRetainedCheckpointPrivateV3{v.Source, a})
	if e != nil {
		return AssessmentRetainedCheckpointPrivateV3{}, e
	}
	s := &assessmentCheckpointStatePrivateV3{projection: p.state, raw: raw}
	s.self = s
	return AssessmentRetainedCheckpointPrivateV3{s}, nil
}
func ValidateInstabilityRetainedCheckpointPrivateV3(ctx context.Context, p ProjectionRetainedCheckpointPrivateV3, a AssessmentRetainedCheckpointPrivateV3, exchange pilot.PageCostPrivateV3) (InstabilityReportRetainedCheckpointPrivateV3, []byte, error) {
	_, r, raw, e := validateInstabilityEvidenceRetainedCheckpointPrivateV3(ctx, p, a, exchange)
	return r, raw, e
}

// The legacy core still performs exactly one fresh Read and preserves its report
// on error. Its detached evidence remains local until the combined exit passes.
func validateInstabilityEvidenceRetainedCheckpointPrivateV3(ctx context.Context, p ProjectionRetainedCheckpointPrivateV3, a AssessmentRetainedCheckpointPrivateV3, exchange pilot.PageCostPrivateV3) (ProjectionEvidenceRetainedCheckpointPrivateV3, InstabilityReportRetainedCheckpointPrivateV3, []byte, error) {
	if !p.valid() || a.state == nil || a.state.self != a.state || a.state.projection != p.state {
		return ProjectionEvidenceRetainedCheckpointPrivateV3{}, InstabilityReportRetainedCheckpointPrivateV3{}, nil, pilot.ErrInvalid
	}
	v, e := p.Read(ctx, exchange)
	if e != nil {
		return ProjectionEvidenceRetainedCheckpointPrivateV3{}, InstabilityReportRetainedCheckpointPrivateV3{}, nil, e
	}
	var r InstabilityReportRetainedCheckpointPrivateV3
	if json.Unmarshal(a.state.raw, &r) != nil || validSeeds(r.Math.Seeds) != nil || r.Math.Outcome == "INCOMPLETE" {
		return v, r, nil, pilot.ErrInvalid
	}
	assessed, e := assessPrivateMath(ctx, v.Math, checkpointAssessmentSkeletonPrivateV3(v.Source, r.Math.Seeds), r.Math.Seeds, r.Math.Runs)
	if e != nil {
		return v, r, nil, e
	}
	raw, e := marshalCheckpointAssessmentPrivateV3(ctx, InstabilityReportRetainedCheckpointPrivateV3{v.Source, assessed})
	if e != nil {
		return v, r, nil, e
	}
	if !bytes.Equal(raw, a.state.raw) {
		return v, r, nil, pilot.ErrInvalid
	}
	return v, r, bytes.Clone(raw), nil
}

// ValidateInstabilityWithEvidenceRetainedCheckpointPrivateV3 returns detached
// observations only after the real sealed batch has passed its mandatory exit.
// No caller-supplied DTO or callback can authorize disclosure through this API.
func ValidateInstabilityWithEvidenceRetainedCheckpointPrivateV3(ctx context.Context, p ProjectionRetainedCheckpointPrivateV3, a AssessmentRetainedCheckpointPrivateV3, exchange pilot.PageCostPrivateV3) (ProjectionEvidenceRetainedCheckpointPrivateV3, []byte, error) {
	if !p.valid() {
		return ProjectionEvidenceRetainedCheckpointPrivateV3{}, nil, pilot.ErrInvalid
	}
	return validateInstabilityWithEvidenceRetainedCheckpointPrivateV3(ctx, p, a, exchange, p.state.selected.RevalidateBindingPrivateV3)
}

// Per-call private seam permits fault injection between assessment and exit in
// package tests; production always supplies the bound real checker above.
func validateInstabilityWithEvidenceRetainedCheckpointPrivateV3(ctx context.Context, p ProjectionRetainedCheckpointPrivateV3, a AssessmentRetainedCheckpointPrivateV3, exchange pilot.PageCostPrivateV3, exit func(context.Context, pilot.RetainedSelectedBatchBindingPrivateV3, pilot.PageCostPrivateV3) error) (ProjectionEvidenceRetainedCheckpointPrivateV3, []byte, error) {
	zero := ProjectionEvidenceRetainedCheckpointPrivateV3{}
	if ctx == nil || exit == nil {
		return zero, nil, pilot.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return zero, nil, e
	}
	before := p.BudgetSpent()
	v, _, raw, e := validateInstabilityEvidenceRetainedCheckpointPrivateV3(ctx, p, a, exchange)
	if e != nil {
		return zero, nil, e
	}
	if e = ctx.Err(); e != nil {
		return zero, nil, e
	}
	left, e := pilot.RemainingPageExchangePrivateV3(before, p.BudgetSpent(), exchange)
	if e != nil {
		return zero, nil, e
	}
	s := p.state.source
	expected := pilot.RetainedSelectedBatchBindingPrivateV3{CheckpointSHA256: s.CheckpointSHA256, RuntimeSHA256: s.RuntimeMetadataSHA256, ObjectSHA256: s.SelectedObjectSHA256}
	if e = exit(ctx, expected, left); e != nil {
		return zero, nil, e
	}
	if e = ctx.Err(); e != nil {
		return zero, nil, e
	}
	return v, raw, nil
}
