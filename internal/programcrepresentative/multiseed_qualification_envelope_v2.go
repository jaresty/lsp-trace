package programcrepresentative

import (
	"bytes"
	"context"
	"fmt"
)

// CheckQualificationCalculationV2 checks canonical and mechanical consistency
// against the supplied evidence adapter. Its result is not production authority:
// callers can supply mutually consistent but false provenance or classifications.
func CheckQualificationCalculationV2(ctx context.Context, expected QualificationSubjectExpectedContextV2, envelopeRaw []byte, source QualificationEvidenceSourceV2) (CheckedQualificationCalculationV2, error) {
	subject, err := NewProgramCRepresentativeQualificationSubjectV2(expected)
	if err != nil {
		return CheckedQualificationCalculationV2{}, err
	}
	return checkEnvelopeForReconstructedSubjectV2(ctx, subject, envelopeRaw, source)
}

func checkEnvelopeForReconstructedSubjectV2(ctx context.Context, subject ProgramCRepresentativeQualificationSubjectV2, envelopeRaw []byte, source QualificationEvidenceSourceV2) (CheckedQualificationCalculationV2, error) {
	zero := CheckedQualificationCalculationV2{}
	if ctx == nil || source == nil || !subject.Trusted() {
		return zero, fmt.Errorf("v2 qualification requires reconstructed subject")
	}
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	envelope, err := ParseQualificationEnvelopeV2(envelopeRaw)
	if err != nil {
		return zero, err
	}
	subjectRaw, err := subject.CanonicalBytes()
	if err != nil {
		return zero, err
	}
	sb := multiSeedBindingV2(QualificationSubjectSchemaVersionV2, subject.Identity(), subjectRaw)
	s := subject.dto
	if err := verifyEnvelopeExpectedBindingsV2(envelope, s, sb); err != nil {
		return zero, err
	}
	artifacts := map[ArtifactBindingDTO][]byte{}
	read := func(b ArtifactBindingDTO) ([]byte, error) {
		if err := checkArtifactBindingV2(b); err != nil {
			return nil, err
		}
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		raw, err := source.ResolveQualificationArtifactV2(ctx, b)
		if err != nil {
			return nil, err
		}
		if err := VerifyRawArtifactBindingV2(b, raw); err != nil {
			return nil, err
		}
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		artifacts[b] = append([]byte{}, raw...)
		return raw, nil
	}
	retainedSubject, err := read(sb)
	if err != nil || !bytes.Equal(retainedSubject, subjectRaw) {
		return zero, fmt.Errorf("v2 retained subject mismatch")
	}
	reviewRaw, err := read(envelope.IndependentReviewReceipt)
	if err != nil {
		return zero, err
	}
	review, err := ParseQualificationReviewV2(reviewRaw)
	if err != nil || review.Identity != envelope.IndependentReviewReceipt.Identity {
		return zero, fmt.Errorf("v2 review binding mismatch")
	}
	closure, err := verifyQualificationReviewClosureV2(ctx, sb, review, source)
	if err != nil {
		return zero, err
	}
	for b, raw := range closure {
		artifacts[b] = raw
	}
	_, pb, policyRaw, err := BuildStage6PolicyArtifactV2()
	if err != nil {
		return zero, err
	}
	_, lb, limitsRaw, err := BuildStage6LimitsArtifactV2()
	if err != nil {
		return zero, err
	}
	_, fb, finalRaw, err := BuildStage6FinalizationArtifactV2()
	if err != nil {
		return zero, err
	}
	for _, item := range []struct {
		b   ArtifactBindingDTO
		raw []byte
	}{{pb, policyRaw}, {lb, limitsRaw}, {fb, finalRaw}} {
		got, err := read(item.b)
		if err != nil || !bytes.Equal(got, item.raw) {
			return zero, fmt.Errorf("v2 retained fixed artifact mismatch")
		}
	}
	mechanics, custodies, err := source.ReconstructQualifiedMechanicsV2(ctx, cloneQualificationSubjectV2(s))
	if err != nil {
		return zero, err
	}
	state := &checkedQualificationCalculationStateV2{subject: sb, subjectDTO: cloneQualificationSubjectV2(s), artifacts: artifacts, envelope: multiSeedBindingV2(envelope.SchemaVersion, envelope.Identity, envelopeRaw), terminalGeneration: envelope.QualificationEpoch, mechanics: map[string][]byte{}, mechanicsHandles: map[string]MechanicsV2{}, custody: map[string]CustodyIdentityDTO{}}
	for _, c := range custodies {
		if _, err := CanonicalSizeCustodyIdentityDTO(c); err != nil {
			return zero, err
		}
		copy := c
		copy.Identity = ""
		identity, err := hashIdentity(custodyDomain, copy)
		if err != nil || identity != c.Identity || c.SchemaVersion != CustodySchemaVersion || c.WorkflowKey != s.TerminalProgramCHead.WorkflowKey || c.TerminalProgramCHeadSelector != s.TerminalProgramCHead.Selector || c.ConstituentOrdinal < 0 || c.ConstituentOrdinal >= len(s.ReceiptInventory) {
			return zero, fmt.Errorf("v2 reconstructed custody mismatch")
		}
		receipt := s.ReceiptInventory[c.ConstituentOrdinal]
		if c.ReceiptSelector != receipt.ReceiptSelector || c.ReceiptOrdinal != receipt.ReceiptOrdinal {
			return zero, fmt.Errorf("v2 reconstructed receipt custody mismatch")
		}
		if previous, ok := state.custody[c.Identity]; ok && previous != c {
			return zero, fmt.Errorf("v2 duplicate custody mismatch")
		}
		state.custody[c.Identity] = c
	}
	for _, m := range mechanics {
		n, err := CanonicalSizeMechanicsIdentityDTOv2(m.dto)
		if err != nil {
			return zero, err
		}
		raw, err := marshalSizedV2(m.dto, n, err)
		if err != nil || !m.Trusted() || !bytes.Equal(raw, m.canonical) || state.custody[m.dto.CustodyIdentity].Identity == "" || state.mechanics[m.Identity()] != nil {
			return zero, fmt.Errorf("v2 reconstructed mechanics mismatch")
		}
		state.mechanics[m.Identity()] = append([]byte{}, raw...)
		state.mechanicsHandles[m.Identity()] = m
		b := multiSeedBindingV2(MechanicsSchemaVersionV2, m.Identity(), raw)
		state.artifacts[b] = append([]byte{}, raw...)
	}
	constituents, extra, err := source.ReconstructCheckpointConstituentsV2(ctx, cloneQualificationSubjectV2(s))
	if err != nil {
		return zero, err
	}
	for _, c := range constituents {
		n, err := CanonicalSizeConstituentV2(c)
		raw, err := marshalSizedV2(c, n, err)
		if err != nil {
			return zero, err
		}
		copy, err := decodeCanonicalTypedV2(raw, CanonicalSizeConstituentV2)
		if err != nil {
			return zero, err
		}
		state.constituents = append(state.constituents, copy)
	}
	for b, raw := range extra {
		if err := VerifyRawArtifactBindingV2(b, raw); err != nil {
			return zero, err
		}
		state.artifacts[b] = append([]byte{}, raw...)
	}
	if _, err := BuildDenominatorArtifactV2(CheckedQualificationCalculationV2{state: state}); err != nil {
		return zero, err
	}
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	state.artifacts[state.envelope] = append([]byte{}, envelopeRaw...)
	return CheckedQualificationCalculationV2{state: state}, nil
}

func verifyEnvelopeExpectedBindingsV2(envelope QualificationEnvelopeV2, s QualificationSubjectV2, sb ArtifactBindingDTO) error {
	if envelope.Subject != sb || envelope.QualificationEpoch == 0 || envelope.QualificationEpoch != s.TerminalProgramCHead.Generation || envelope.Stage6Policy != s.Stage6Policy || envelope.Stage6Limits != s.Stage6Limits || envelope.Stage6Finalization != s.Stage6Finalization || envelope.IndependentReviewReceipt.SchemaVersion != QualificationReviewSchemaVersionV2 {
		return fmt.Errorf("v2 qualification subject/artifact/epoch mismatch")
	}
	return nil
}

func (q CheckedQualificationCalculationV2) SubjectBindingV2() (ArtifactBindingDTO, error) {
	if q.state == nil {
		return ArtifactBindingDTO{}, fmt.Errorf("v2 qualification unavailable")
	}
	return q.state.subject, nil
}
func (q CheckedQualificationCalculationV2) EnvelopeBindingV2() (ArtifactBindingDTO, error) {
	if q.state == nil {
		return ArtifactBindingDTO{}, fmt.Errorf("v2 qualification unavailable")
	}
	return q.state.envelope, nil
}
func (q CheckedQualificationCalculationV2) RetainedQualificationArtifactsV2() (map[ArtifactBindingDTO][]byte, error) {
	if q.state == nil {
		return nil, fmt.Errorf("v2 qualification unavailable")
	}
	out := map[ArtifactBindingDTO][]byte{}
	for b, raw := range q.state.artifacts {
		if err := VerifyRawArtifactBindingV2(b, raw); err != nil {
			return nil, err
		}
		out[b] = append([]byte{}, raw...)
	}
	return out, nil
}
