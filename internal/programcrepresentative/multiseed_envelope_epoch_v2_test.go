package programcrepresentative

import "testing"

func TestMultiSeedV2EnvelopeExactSubjectEpoch(t *testing.T) {
	t.Log("procedure: envelope binds exact reconstructed subject/fixed artifacts and positive terminal generation; resealed stale/new/zero epoch and foreign bindings refuse")
	subject, review, _ := multiSeedReviewFixtureV2(t)
	s, err := newQualificationSubjectV2FromExpected(multiSeedSubjectExpectedFixtureV2(t))
	if err != nil {
		t.Fatal(err)
	}
	_, rb, _, err := EncodeQualificationReviewV2(review)
	if err != nil {
		t.Fatal(err)
	}
	e := QualificationEnvelopeV2{SchemaVersion: QualificationEnvelopeSchemaVersionV2, Profile: ProfileCallsOnlyFrozenCensusV1, Subject: subject, IndependentReviewReceipt: rb, Stage6Policy: s.dto.Stage6Policy, Stage6Limits: s.dto.Stage6Limits, Stage6Finalization: s.dto.Stage6Finalization, QualificationEpoch: s.dto.TerminalProgramCHead.Generation}
	if err := verifyEnvelopeExpectedBindingsV2(e, s.dto, subject); err != nil {
		t.Fatalf("EXACT_SUBJECT_TERMINAL_EPOCH: %v", err)
	}
	for _, name := range []string{"zero", "stale", "future", "subject", "policy", "limits", "finalization", "v1-review"} {
		t.Run(name, func(t *testing.T) {
			bad := e
			switch name {
			case "zero":
				bad.QualificationEpoch = 0
			case "stale":
				bad.QualificationEpoch--
			case "future":
				bad.QualificationEpoch++
			case "subject":
				bad.Subject.SHA256 = rawDigest([]byte("foreign"))
			case "policy":
				bad.Stage6Policy.Identity = rawDigest([]byte("foreign"))
			case "limits":
				bad.Stage6Limits.SHA256 = rawDigest([]byte("foreign"))
			case "finalization":
				bad.Stage6Finalization.ByteLength++
			case "v1-review":
				bad.IndependentReviewReceipt.SchemaVersion = "lsp-trace.program-c-representative-qualification-review.private.v1"
			}
			bad, _, raw, err := EncodeQualificationEnvelopeV2(bad)
			if err != nil {
				t.Fatal(err)
			}
			parsed, err := ParseQualificationEnvelopeV2(raw)
			if err != nil {
				t.Fatal(err)
			}
			if parsed.Identity != bad.Identity || verifyEnvelopeExpectedBindingsV2(parsed, s.dto, subject) == nil {
				t.Fatal("RESEALED_ENVELOPE_CONTEXT_REFUSAL")
			}
		})
	}
}
