package censuscontinuation

import (
	"context"
	"errors"
	"lsp-trace/internal/programcrepresentative"
)

// QualificationArtifactResolverV2 resolves only bounded, identified bytes. It
// cannot supply actors, classifications, mechanics, or a qualification handle.
type QualificationArtifactResolverV2 interface {
	ResolveQualificationArtifactV2(context.Context, programcrepresentative.ArtifactBindingDTO) ([]byte, error)
}

var ErrQualificationProvenanceV2 = errors.New("v2 independent registered review provenance refused")
var ErrQualificationUnavailableV2 = errors.New("v2 Census qualification unavailable")

// VerifiedQualificationV2 is a production capability issued only from this
// owner's private verified receipt inventory. Its zero value is invalid.
// A checked calculation, DTO, or embedded interface cannot manufacture it.
type VerifiedQualificationV2 struct{ state *verifiedQualificationStateV2 }
type verifiedQualificationStateV2 struct {
	calculation programcrepresentative.CheckedQualificationCalculationV2
}

type ownerQualificationSourceV2 struct {
	evidence  ProgramCMultiSeedQualificationEvidenceV2
	artifacts QualificationArtifactResolverV2
	bindings  []programcrepresentative.ReviewedHarnessActorBindingV2
}

func (s ownerQualificationSourceV2) ResolveQualificationArtifactV2(ctx context.Context, b programcrepresentative.ArtifactBindingDTO) ([]byte, error) {
	if ctx == nil || s.artifacts == nil || b.ByteLength == 0 || b.ByteLength > programcrepresentative.MaxArtifactCanonicalBytesV2 {
		return nil, ErrInvalidProgramCTransition
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	raw, err := s.artifacts.ResolveQualificationArtifactV2(ctx, b)
	if err != nil {
		return nil, err
	}
	if err = programcrepresentative.VerifyRawArtifactBindingV2(b, raw); err != nil {
		return nil, err
	}
	if err = ctx.Err(); err != nil {
		return nil, err
	}
	return raw, nil
}
func (s ownerQualificationSourceV2) RecordedReviewActorV2(ctx context.Context, kind, id string) (programcrepresentative.RecordedReviewActorV2, error) {
	for _, b := range s.bindings {
		if b.Actor.Kind == kind && b.Actor.ID == id {
			raw, err := s.ResolveQualificationArtifactV2(ctx, b.Projection)
			if err != nil {
				return programcrepresentative.RecordedReviewActorV2{}, err
			}
			actor := b.Actor
			actor.Projection = raw
			return actor, nil
		}
	}
	return programcrepresentative.RecordedReviewActorV2{}, ErrQualificationProvenanceV2
}
func (s ownerQualificationSourceV2) ReconstructQualifiedMechanicsV2(ctx context.Context, d programcrepresentative.QualificationSubjectV2) ([]programcrepresentative.MechanicsV2, []programcrepresentative.CustodyIdentityDTO, error) {
	return s.evidence.ReconstructQualifiedMechanicsV2(ctx, d)
}
func (s ownerQualificationSourceV2) ReconstructCheckpointConstituentsV2(ctx context.Context, d programcrepresentative.QualificationSubjectV2) ([]programcrepresentative.ConstituentV2, map[programcrepresentative.ArtifactBindingDTO][]byte, error) {
	return s.evidence.ReconstructCheckpointConstituentsV2(ctx, d)
}

func (e ProgramCMultiSeedQualificationEvidenceV2) VerifyQualificationEnvelopeV2(ctx context.Context, expected programcrepresentative.QualificationSubjectExpectedContextV2, envelopeRaw []byte, artifacts QualificationArtifactResolverV2) (VerifiedQualificationV2, error) {
	zero := VerifiedQualificationV2{}
	if ctx == nil || artifacts == nil || len(e.inventory) == 0 || len(e.preflight.adaptations) != len(e.inventory) {
		return zero, ErrInvalidProgramCTransition
	}
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	// Check the caller's equality expectations against this concrete owner's
	// inventory before consulting any caller-resolved envelope bytes.
	if err := e.checkQualificationSubjectV2(programcrepresentative.QualificationSubjectV2{CorpusIdentity: expected.CorpusIdentity, TerminalProgramCHead: expected.TerminalProgramCHead, ReceiptInventory: expected.ReceiptInventory, AdapterContract: expected.AdapterContract}); err != nil {
		return zero, err
	}
	bindings, err := programcrepresentative.ReviewedHarnessActorBindingsV2()
	if err != nil {
		return zero, err
	}
	if len(bindings) != 2 {
		return zero, ErrQualificationProvenanceV2
	}
	for i, b := range bindings {
		kind := "PI_SESSION"
		if i == 1 {
			kind = "PI_ISOLATED_TASK"
		}
		if b.Actor.Kind != kind || b.Actor.IsolationKind != kind || b.Actor.ID == "" || len(b.Actor.Projection) != 0 || b.Projection.SchemaVersion != "lsp-trace.program-c-representative-review-transcript-jsonl.private.v2" || b.Projection.Identity != "" || b.Projection.ByteLength == 0 || b.Projection.ByteLength > programcrepresentative.MaxArtifactCanonicalBytesV2 {
			return zero, ErrQualificationProvenanceV2
		}
	}
	if bindings[0].Actor.ParentID != "" || bindings[0].Actor.ID == bindings[1].Actor.ID || bindings[1].Actor.ParentID != bindings[0].Actor.ID {
		return zero, ErrQualificationProvenanceV2
	}
	// Only this private adapter supplies the mechanics/classification callbacks.
	// Review closure checks exact registered projection bytes, parent, isolation,
	// subject and scope; artifact resolution alone cannot replace those owners.
	source := ownerQualificationSourceV2{evidence: e, artifacts: artifacts, bindings: bindings}
	calculation, err := programcrepresentative.CheckQualificationCalculationV2(ctx, expected, envelopeRaw, source)
	if err != nil {
		return zero, err
	}
	if err = ctx.Err(); err != nil {
		return zero, err
	}
	return VerifiedQualificationV2{state: &verifiedQualificationStateV2{calculation: calculation}}, nil
}
func (q VerifiedQualificationV2) checked() (programcrepresentative.CheckedQualificationCalculationV2, error) {
	if q.state == nil {
		return programcrepresentative.CheckedQualificationCalculationV2{}, ErrQualificationUnavailableV2
	}
	return q.state.calculation, nil
}
func (q VerifiedQualificationV2) SubjectDTOv2() (programcrepresentative.QualificationSubjectV2, error) {
	c, e := q.checked()
	if e != nil {
		return programcrepresentative.QualificationSubjectV2{}, e
	}
	return c.SubjectDTOv2()
}
func (q VerifiedQualificationV2) SubjectBindingV2() (programcrepresentative.ArtifactBindingDTO, error) {
	c, e := q.checked()
	if e != nil {
		return programcrepresentative.ArtifactBindingDTO{}, e
	}
	return c.SubjectBindingV2()
}
func (q VerifiedQualificationV2) EnvelopeBindingV2() (programcrepresentative.ArtifactBindingDTO, error) {
	c, e := q.checked()
	if e != nil {
		return programcrepresentative.ArtifactBindingDTO{}, e
	}
	return c.EnvelopeBindingV2()
}
func (q VerifiedQualificationV2) RetainedQualificationArtifactsV2() (map[programcrepresentative.ArtifactBindingDTO][]byte, error) {
	c, e := q.checked()
	if e != nil {
		return nil, e
	}
	return c.RetainedQualificationArtifactsV2()
}
func (q VerifiedQualificationV2) BuildDenominatorArtifactV2() (programcrepresentative.VerifiedDenominatorV2, error) {
	c, e := q.checked()
	if e != nil {
		return programcrepresentative.VerifiedDenominatorV2{}, e
	}
	return programcrepresentative.BuildDenominatorArtifactV2(c)
}
func (q VerifiedQualificationV2) VerifyDenominatorArtifactV2(raw []byte) (programcrepresentative.VerifiedDenominatorV2, error) {
	c, e := q.checked()
	if e != nil {
		return programcrepresentative.VerifiedDenominatorV2{}, e
	}
	return programcrepresentative.VerifyDenominatorArtifactV2(raw, c)
}
func (q VerifiedQualificationV2) BuildLineageArtifactV2(d programcrepresentative.VerifiedDenominatorV2) (programcrepresentative.VerifiedLineageV2, error) {
	c, e := q.checked()
	if e != nil {
		return programcrepresentative.VerifiedLineageV2{}, e
	}
	return programcrepresentative.BuildLineageArtifactV2(d, c)
}
func (q VerifiedQualificationV2) VerifyLineageArtifactV2(ctx context.Context, raw []byte, d programcrepresentative.VerifiedDenominatorV2, r programcrepresentative.CheckpointArtifactResolverV2) (programcrepresentative.VerifiedLineageV2, error) {
	c, e := q.checked()
	if e != nil {
		return programcrepresentative.VerifiedLineageV2{}, e
	}
	return programcrepresentative.VerifyLineageArtifactV2(ctx, raw, d, c, r)
}
func (q VerifiedQualificationV2) ProjectCheckpointV2(d programcrepresentative.VerifiedDenominatorV2, l programcrepresentative.VerifiedLineageV2) (programcrepresentative.CheckpointProjectionV2, error) {
	c, e := q.checked()
	if e != nil {
		return programcrepresentative.CheckpointProjectionV2{}, e
	}
	return programcrepresentative.ProjectVerifiedCheckpointV2(c, d, l)
}
