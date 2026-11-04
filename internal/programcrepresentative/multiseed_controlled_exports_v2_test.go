package programcrepresentative

import (
	"context"
	"testing"
)

// TEST-ARCHIVE ONLY. None of these symbols exists in a production build.
// They supply controlled local implementation/review evidence for component
// fault injection, not real reviewed pins or connected production admission.
type ControlledCorpusForTestsV2 interface {
	ReconstructQualifiedMechanicsV2(context.Context, QualificationSubjectV2) ([]MechanicsV2, []CustodyIdentityDTO, error)
	ReconstructCheckpointConstituentsV2(context.Context, QualificationSubjectV2) ([]ConstituentV2, map[ArtifactBindingDTO][]byte, error)
}
type controlledReviewCorpusV2 struct {
	*reviewEvidenceFixtureV2
	corpus ControlledCorpusForTestsV2
}

func (s controlledReviewCorpusV2) ReconstructQualifiedMechanicsV2(ctx context.Context, subject QualificationSubjectV2) ([]MechanicsV2, []CustodyIdentityDTO, error) {
	return s.corpus.ReconstructQualifiedMechanicsV2(ctx, subject)
}
func (s controlledReviewCorpusV2) ReconstructCheckpointConstituentsV2(ctx context.Context, subject QualificationSubjectV2) ([]ConstituentV2, map[ArtifactBindingDTO][]byte, error) {
	return s.corpus.ReconstructCheckpointConstituentsV2(ctx, subject)
}
func ControlledSubjectContextForTestsV2(t *testing.T) QualificationSubjectExpectedContextV2 {
	return multiSeedSubjectExpectedFixtureV2(t)
}
func ControlledQualificationForTestsV2(t *testing.T, expected QualificationSubjectExpectedContextV2, corpus ControlledCorpusForTestsV2) CheckedQualificationCalculationV2 {
	t.Helper()
	subject, err := newQualificationSubjectV2FromExpected(expected)
	if err != nil {
		t.Fatal(err)
	}
	sb, review, source := multiSeedReviewFixtureForSubjectV2(t, subject)
	review, rb, raw, err := EncodeQualificationReviewV2(review)
	if err != nil {
		t.Fatal(err)
	}
	_ = review
	source.objects[rb] = raw
	_, pb, pr, err := BuildStage6PolicyArtifactV2()
	if err != nil {
		t.Fatal(err)
	}
	_, lb, lr, err := BuildStage6LimitsArtifactV2()
	if err != nil {
		t.Fatal(err)
	}
	_, fb, fr, err := BuildStage6FinalizationArtifactV2()
	if err != nil {
		t.Fatal(err)
	}
	source.objects[pb] = pr
	source.objects[lb] = lr
	source.objects[fb] = fr
	envelope := QualificationEnvelopeV2{SchemaVersion: QualificationEnvelopeSchemaVersionV2, Profile: ProfileCallsOnlyFrozenCensusV1, Subject: sb, IndependentReviewReceipt: rb, Stage6Policy: pb, Stage6Limits: lb, Stage6Finalization: fb, QualificationEpoch: expected.TerminalProgramCHead.Generation}
	_, _, raw, err = EncodeQualificationEnvelopeV2(envelope)
	if err != nil {
		t.Fatal(err)
	}
	q, err := checkEnvelopeForReconstructedSubjectV2(context.Background(), subject, raw, controlledReviewCorpusV2{source, corpus})
	if err != nil {
		t.Fatal(err)
	}
	return q
}
