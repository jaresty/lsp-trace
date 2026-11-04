package programcrepresentative

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
)

type reviewEvidenceFixtureV2 struct {
	objects map[ArtifactBindingDTO][]byte
	actors  map[string]RecordedReviewActorV2
}

func (s *reviewEvidenceFixtureV2) ResolveQualificationArtifactV2(_ context.Context, b ArtifactBindingDTO) ([]byte, error) {
	raw, ok := s.objects[b]
	if !ok {
		return nil, fmt.Errorf("test-only missing review object")
	}
	return append([]byte{}, raw...), nil
}
func (s *reviewEvidenceFixtureV2) RecordedReviewActorV2(_ context.Context, kind, id string) (RecordedReviewActorV2, error) {
	a, ok := s.actors[kind+":"+id]
	if !ok {
		return RecordedReviewActorV2{}, fmt.Errorf("test-only missing harness metadata")
	}
	return a, nil
}
func (s *reviewEvidenceFixtureV2) ReconstructQualifiedMechanicsV2(context.Context, QualificationSubjectV2) ([]MechanicsV2, []CustodyIdentityDTO, error) {
	return nil, nil, fmt.Errorf("test-only review fixture has no corpus authority")
}

func (s *reviewEvidenceFixtureV2) ReconstructCheckpointConstituentsV2(context.Context, QualificationSubjectV2) ([]ConstituentV2, map[ArtifactBindingDTO][]byte, error) {
	return nil, nil, fmt.Errorf("test-only review fixture has no constituent authority")
}

func multiSeedReviewFixtureV2(t *testing.T) (ArtifactBindingDTO, QualificationReviewV2, *reviewEvidenceFixtureV2) {
	t.Helper()
	s, err := newQualificationSubjectV2FromExpected(multiSeedSubjectExpectedFixtureV2(t))
	if err != nil {
		t.Fatal(err)
	}
	return multiSeedReviewFixtureForSubjectV2(t, s)
}
func multiSeedReviewFixtureForSubjectV2(t *testing.T, s ProgramCRepresentativeQualificationSubjectV2) (ArtifactBindingDTO, QualificationReviewV2, *reviewEvidenceFixtureV2) {
	t.Helper()
	source := &reviewEvidenceFixtureV2{objects: map[ArtifactBindingDTO][]byte{}, actors: map[string]RecordedReviewActorV2{}}
	raw, _ := s.CanonicalBytes()
	subject := multiSeedBindingV2(QualificationSubjectSchemaVersionV2, s.Identity(), raw)
	source.objects[subject] = raw
	var err error
	actors := []ReviewActorV2{}
	for i, kind := range []string{"PI_SESSION", "PI_ISOLATED_TASK"} {
		id := "test-only-writer"
		parent := ""
		verdict := "SUBJECT_PRODUCED"
		if i == 1 {
			id = "test-only-reviewer"
			parent = "test-only-writer"
			verdict = "ACCEPT"
		}
		assignment := ReviewAssignmentV2{SchemaVersion: "lsp-trace.program-c-representative-review-assignment.private.v2", ActorKind: kind, HarnessActorID: id, ParentActorID: parent, Subject: subject, Scope: fixedReviewScopeV2()}
		terminal := ReviewTerminalV2{SchemaVersion: "lsp-trace.program-c-representative-review-terminal.private.v2", ActorKind: kind, HarnessActorID: id, ParentActorID: parent, Subject: subject, Scope: fixedReviewScopeV2(), Verdict: verdict}
		a, _ := json.Marshal(assignment)
		z, _ := json.Marshal(terminal)
		projection := append(append(append(append([]byte{}, a...), '\n'), z...), '\n')
		rb := ArtifactBindingDTO{SchemaVersion: "lsp-trace.program-c-representative-review-transcript-jsonl.private.v2", SHA256: rawDigest(projection), ByteLength: uint64(len(projection))}
		source.objects[rb] = projection
		snapshot := ReviewTranscriptSnapshotV2{SchemaVersion: "lsp-trace.program-c-representative-review-transcript-snapshot.private.v2", ActorKind: kind, HarnessActorID: id, ParentActorID: parent, RawTranscriptSelector: rb.SHA256, RawTranscriptSHA256: rb.SHA256, RawTranscriptByteLength: rb.ByteLength, AssignmentSubject: subject, AssignmentScope: fixedReviewScopeV2(), TerminalSubject: subject, TerminalScope: fixedReviewScopeV2(), TerminalVerdict: verdict}
		_, binding, body, err := EncodeReviewTranscriptSnapshotV2(snapshot)
		if err != nil {
			t.Fatal(err)
		}
		source.objects[binding] = body
		actor := ReviewActorV2{IdentityKind: kind, HarnessActorID: id, AttributionEvidence: binding}
		n, err := CanonicalSizeReviewActorV2(actor)
		pre, err := marshalSizedV2(actor, n, err)
		if err != nil {
			t.Fatal(err)
		}
		actor.Identity = multiSeedIdentityV2("lsp-trace:program-c-representative-review-actor:v2", pre)
		actors = append(actors, actor)
		source.actors[kind+":"+id] = RecordedReviewActorV2{Kind: kind, ID: id, ParentID: parent, IsolationKind: kind, Projection: append([]byte{}, projection...)}
	}
	separation := ReviewTaskSeparationV2{SchemaVersion: "lsp-trace.program-c-representative-review-task-separation.private.v2", WriterSessionID: actors[0].HarnessActorID, ReviewerTaskID: actors[1].HarnessActorID, ReviewerParentSessionID: actors[0].HarnessActorID, ReviewerIsolated: true, Subject: subject, ReviewScope: fixedReviewScopeV2()}
	_, sb, sraw, err := EncodeReviewTaskSeparationV2(separation)
	if err != nil {
		t.Fatal(err)
	}
	source.objects[sb] = sraw
	review := QualificationReviewV2{SchemaVersion: QualificationReviewSchemaVersionV2, Profile: ProfileCallsOnlyFrozenCensusV1, Subject: subject, ReviewScope: fixedReviewScopeV2(), Verdict: "ACCEPT", Writer: actors[0], Reviewer: actors[1], SeparationMethod: "DISTINCT_HARNESS_ISOLATED_TASK", SeparationEvidence: sb}
	review, _, _, err = EncodeQualificationReviewV2(review)
	if err != nil {
		t.Fatal(err)
	}
	return subject, review, source
}
func TestMultiSeedV2IndependentReviewClosure(t *testing.T) {
	t.Log("procedure: independently recorded harness metadata and canonical two-line projections must close over exact subject/scope and distinct actors")
	subject, review, source := multiSeedReviewFixtureV2(t)
	objects, err := verifyQualificationReviewClosureV2(context.Background(), subject, review, source)
	if err != nil || len(objects) != 5 {
		t.Fatalf("INDEPENDENT_REVIEW_CLOSURE: objects=%d err=%v", len(objects), err)
	}
	for _, name := range []string{"self-review", "scope", "parent", "projection", "missing-snapshot", "foreign-subject"} {
		t.Run(name, func(t *testing.T) {
			sb, r, src := multiSeedReviewFixtureV2(t)
			switch name {
			case "self-review":
				r.Reviewer = r.Writer
			case "scope":
				r.ReviewScope = r.ReviewScope[:len(r.ReviewScope)-1]
			case "parent":
				key := "PI_ISOLATED_TASK:" + r.Reviewer.HarnessActorID
				a := src.actors[key]
				a.ParentID = "foreign-parent"
				src.actors[key] = a
			case "projection":
				key := "PI_ISOLATED_TASK:" + r.Reviewer.HarnessActorID
				a := src.actors[key]
				a.Projection = append(a.Projection, '\n')
				src.actors[key] = a
			case "missing-snapshot":
				delete(src.objects, r.Reviewer.AttributionEvidence)
			case "foreign-subject":
				r.Subject.SHA256 = rawDigest([]byte("foreign-subject"))
			}
			r, _, _, err = EncodeQualificationReviewV2(r)
			if err != nil {
				t.Fatal(err)
			}
			if out, err := verifyQualificationReviewClosureV2(context.Background(), sb, r, src); err == nil || out != nil {
				t.Fatal("RESEALED_REVIEW_CLOSURE_REFUSAL")
			}
		})
	}
}
