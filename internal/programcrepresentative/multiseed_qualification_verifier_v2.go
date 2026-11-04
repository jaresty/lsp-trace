package programcrepresentative

import (
	"bytes"
	"context"
	"fmt"
)

// RecordedReviewActorV2 is non-wire evidence read independently from harness
// session/task metadata. There is deliberately no trusted/qualified boolean.
type RecordedReviewActorV2 struct {
	Kind, ID, ParentID, IsolationKind string
	Projection                        []byte
}

// QualificationEvidenceSourceV2 supplies inputs to a nonauthoritative consistency
// calculation. Implementing it establishes no ownership or provenance. Resolve
// must enforce the declared byte bound before allocation. Production qualification
// is issued separately by the concrete Census evidence owner.
type QualificationEvidenceSourceV2 interface {
	ResolveQualificationArtifactV2(context.Context, ArtifactBindingDTO) ([]byte, error)
	RecordedReviewActorV2(context.Context, string, string) (RecordedReviewActorV2, error)
	ReconstructQualifiedMechanicsV2(context.Context, QualificationSubjectV2) ([]MechanicsV2, []CustodyIdentityDTO, error)
	ReconstructCheckpointConstituentsV2(context.Context, QualificationSubjectV2) ([]ConstituentV2, map[ArtifactBindingDTO][]byte, error)
}

func fixedReviewScopeV2() []string {
	return []string{"SUBJECT_CANONICAL_BYTES", "SUBJECT_EXPECTED_CONTEXT_RECONSTRUCTION", "IMPLEMENTATION_SEAM_MANIFEST", "STAGE6_POLICY", "STAGE6_LIMITS", "STAGE6_FINALIZATION", "CORPUS_AND_WORKFLOW_BINDINGS", "COMPLETE_RECEIPT_INVENTORY", "TERMINAL_PROGRAM_C_HEAD", "CLAIM_CEILINGS", "ACYCLICITY", "MULTISEED_ORDERED_BIJECTION", "GROUP_LOCAL_TERMINALS", "D_AND_G_CONSERVATION", "BOUNDED_TYPED_SIZE_ADMISSION"}
}

func verifyQualificationReviewClosureV2(ctx context.Context, subject ArtifactBindingDTO, review QualificationReviewV2, source QualificationEvidenceSourceV2) (map[ArtifactBindingDTO][]byte, error) {
	fail := func() (map[ArtifactBindingDTO][]byte, error) {
		return nil, fmt.Errorf("independent v2 review closure refused")
	}
	if ctx == nil || source == nil || ctx.Err() != nil || checkArtifactBindingV2(subject) != nil {
		return fail()
	}
	sealed, _, _, err := EncodeQualificationReviewV2(review)
	if err != nil || review.Identity != sealed.Identity || review.Subject != subject || review.Verdict != "ACCEPT" || review.SeparationMethod != "DISTINCT_HARNESS_ISOLATED_TASK" || !equalMechanicsStrings(review.ReviewScope, fixedReviewScopeV2()) || review.Writer.Identity == review.Reviewer.Identity || review.Writer.HarnessActorID == review.Reviewer.HarnessActorID || review.Writer.AttributionEvidence.Identity == review.Reviewer.AttributionEvidence.Identity {
		return fail()
	}
	objects := map[ArtifactBindingDTO][]byte{}
	read := func(b ArtifactBindingDTO) ([]byte, error) {
		if b.ByteLength == 0 || b.ByteLength > MaxArtifactCanonicalBytesV2 || !validDigest(b.SHA256) {
			return nil, fmt.Errorf("review artifact bound refused")
		}
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		raw, err := source.ResolveQualificationArtifactV2(ctx, b)
		if err != nil {
			return nil, err
		}
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if err := VerifyRawArtifactBindingV2(b, raw); err != nil {
			return nil, err
		}
		objects[b] = append([]byte{}, raw...)
		return raw, nil
	}
	for i, actor := range []ReviewActorV2{review.Writer, review.Reviewer} {
		kind, parent, verdict := "PI_SESSION", "", "SUBJECT_PRODUCED"
		if i == 1 {
			kind = "PI_ISOLATED_TASK"
			parent = review.Writer.HarnessActorID
			verdict = "ACCEPT"
		}
		root := actor
		root.Identity = ""
		n, e := CanonicalSizeReviewActorV2(root)
		pre, e := marshalSizedV2(root, n, e)
		if e != nil || actor.IdentityKind != kind || actor.HarnessActorID == "" || actor.Identity != multiSeedIdentityV2("lsp-trace:program-c-representative-review-actor:v2", pre) || actor.AttributionEvidence.SchemaVersion != "lsp-trace.program-c-representative-review-transcript-snapshot.private.v2" {
			return fail()
		}
		raw, e := read(actor.AttributionEvidence)
		if e != nil {
			return nil, e
		}
		snapshot, e := ParseReviewTranscriptSnapshotV2(raw)
		if e != nil || snapshot.Identity != actor.AttributionEvidence.Identity || snapshot.ActorKind != kind || snapshot.HarnessActorID != actor.HarnessActorID || snapshot.ParentActorID != parent || snapshot.AssignmentSubject != subject || snapshot.TerminalSubject != subject || snapshot.TerminalVerdict != verdict || !equalMechanicsStrings(snapshot.AssignmentScope, fixedReviewScopeV2()) || !equalMechanicsStrings(snapshot.TerminalScope, fixedReviewScopeV2()) || snapshot.RawTranscriptSelector != snapshot.RawTranscriptSHA256 {
			return fail()
		}
		rb := ArtifactBindingDTO{SchemaVersion: "lsp-trace.program-c-representative-review-transcript-jsonl.private.v2", SHA256: snapshot.RawTranscriptSHA256, ByteLength: snapshot.RawTranscriptByteLength}
		projection, e := read(rb)
		if e != nil {
			return nil, e
		}
		lines := bytes.Split(projection, []byte{'\n'})
		if len(lines) != 3 || len(lines[0]) == 0 || len(lines[1]) == 0 || len(lines[2]) != 0 {
			return fail()
		}
		assignment, e := decodeCanonicalTypedV2(lines[0], CanonicalSizeReviewAssignmentV2)
		if e != nil {
			return nil, e
		}
		terminal, e := decodeCanonicalTypedV2(lines[1], CanonicalSizeReviewTerminalV2)
		if e != nil {
			return nil, e
		}
		if assignment.SchemaVersion != "lsp-trace.program-c-representative-review-assignment.private.v2" || terminal.SchemaVersion != "lsp-trace.program-c-representative-review-terminal.private.v2" || assignment.ActorKind != kind || terminal.ActorKind != kind || assignment.HarnessActorID != actor.HarnessActorID || terminal.HarnessActorID != actor.HarnessActorID || assignment.ParentActorID != parent || terminal.ParentActorID != parent || assignment.Subject != subject || terminal.Subject != subject || terminal.Verdict != verdict || !equalMechanicsStrings(assignment.Scope, fixedReviewScopeV2()) || !equalMechanicsStrings(terminal.Scope, fixedReviewScopeV2()) {
			return fail()
		}
		recorded, e := source.RecordedReviewActorV2(ctx, kind, actor.HarnessActorID)
		if e != nil {
			return nil, e
		}
		if recorded.Kind != kind || recorded.ID != actor.HarnessActorID || recorded.ParentID != parent || recorded.IsolationKind != kind || uint64(len(recorded.Projection)) > MaxArtifactCanonicalBytesV2 || !bytes.Equal(recorded.Projection, projection) {
			return fail()
		}
	}
	if review.SeparationEvidence.SchemaVersion != "lsp-trace.program-c-representative-review-task-separation.private.v2" {
		return fail()
	}
	raw, err := read(review.SeparationEvidence)
	if err != nil {
		return nil, err
	}
	separation, err := ParseReviewTaskSeparationV2(raw)
	if err != nil || separation.Identity != review.SeparationEvidence.Identity || separation.WriterSessionID != review.Writer.HarnessActorID || separation.ReviewerTaskID != review.Reviewer.HarnessActorID || separation.ReviewerParentSessionID != review.Writer.HarnessActorID || !separation.ReviewerIsolated || separation.Subject != subject || !equalMechanicsStrings(separation.ReviewScope, fixedReviewScopeV2()) {
		return fail()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return objects, nil
}
