package programcrepresentative

import (
	"bytes"
	"context"
	"fmt"
)

type CheckpointArtifactResolverV2 interface {
	Resolve(context.Context, ArtifactBindingDTO) ([]byte, error)
}

func (q CheckedQualificationCalculationV2) SubjectDTOv2() (QualificationSubjectV2, error) {
	if q.state == nil {
		return QualificationSubjectV2{}, fmt.Errorf("v2 qualification unavailable")
	}
	if _, err := CanonicalSizeQualificationSubjectV2(q.state.subjectDTO); err != nil {
		return QualificationSubjectV2{}, err
	}
	return cloneQualificationSubjectV2(q.state.subjectDTO), nil
}
func FixedClaimCeilingV2() QualificationClaimCeilingDTO { return fixedQualificationClaimCeiling() }
func VerifyLineageArtifactV2(ctx context.Context, raw []byte, d VerifiedDenominatorV2, q CheckedQualificationCalculationV2, resolver CheckpointArtifactResolverV2) (VerifiedLineageV2, error) {
	zero := VerifiedLineageV2{}
	if ctx == nil || resolver == nil || q.state == nil || d.qualification != q.state {
		return zero, fmt.Errorf("v2 lineage context unavailable")
	}
	dto, err := ParseLineageV2(raw)
	if err != nil {
		return zero, err
	}
	if dto.Denominator != d.binding || dto.QualificationEnvelope != q.state.envelope || dto.ClaimCeiling != fixedQualificationClaimCeiling() || len(dto.Records) != len(d.dto.Entries) {
		return zero, fmt.Errorf("v2 retained lineage context mismatch")
	}
	a := LineageAccountingV2{D: d.dto.Accounting.D, G: d.dto.Accounting.G}
	artifacts := map[ArtifactBindingDTO][]byte{}
	read := func(b ArtifactBindingDTO) ([]byte, error) {
		if err := checkArtifactBindingV2(b); err != nil {
			return nil, err
		}
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		body, err := resolver.Resolve(ctx, b)
		if err != nil {
			return nil, err
		}
		if err := VerifyRawArtifactBindingV2(b, body); err != nil {
			return nil, err
		}
		artifacts[b] = append([]byte{}, body...)
		return body, nil
	}
	for i, record := range dto.Records {
		entry := d.dto.Entries[i]
		sealed, _, _, err := EncodeLineageRecordV2(record)
		if err != nil || record.Identity != sealed.Identity || record.DenominatorEntryIdentity != entry.Identity || record.Mechanics != entry.Mechanics || record.QualificationEnvelope != q.state.envelope || record.Result.SchemaVersion != ResultSchemaVersionV2 {
			return zero, fmt.Errorf("v2 retained lineage record splice")
		}
		body, err := read(record.Result)
		if err != nil {
			return zero, err
		}
		result, err := ParseMultiSeedResultV2(body)
		if err != nil || result.Identity() != record.Result.Identity {
			return zero, fmt.Errorf("v2 retained result refusal")
		}
		r := result.dto
		m := r.Composition.Mechanics
		if m.Identity != entry.Mechanics.Identity || !bytes.Equal(q.state.mechanics[m.Identity], func() []byte {
			n, e := CanonicalSizeMechanicsIdentityDTOv2(m)
			b, _ := marshalSizedV2(m, n, e)
			return b
		}()) || r.Composition.Custody != entry.Custody || r.Composition.QualificationEnvelope != q.state.envelope || r.Qualification.ValidFrom != q.state.terminalGeneration || r.Qualification.ValidThrough != q.state.terminalGeneration {
			return zero, fmt.Errorf("v2 retained result foreign qualification/context")
		}
		for _, b := range []ArtifactBindingDTO{r.Grouping.Composition, r.Qualification.Grouping} {
			if _, err := read(b); err != nil {
				return zero, err
			}
		}
		a.TerminalOuterResults++
		a.SelectedGroups, err = checkedSumV2(a.SelectedGroups, r.GroupAccounting.Selected)
		if err != nil {
			return zero, err
		}
		a.NoCallsGroups, err = checkedSumV2(a.NoCallsGroups, r.GroupAccounting.NoCalls)
		if err != nil {
			return zero, err
		}
		a.CallsIneligibleGroups, err = checkedSumV2(a.CallsIneligibleGroups, r.GroupAccounting.CallsIneligible)
		if err != nil {
			return zero, err
		}
	}
	sum, err := checkedSumV2(a.SelectedGroups, a.NoCallsGroups, a.CallsIneligibleGroups)
	if err != nil || sum != a.G || a.TerminalOuterResults != a.D || dto.Accounting != a {
		return zero, fmt.Errorf("v2 retained lineage conservation mismatch")
	}
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	b := multiSeedBindingV2(LineageSchemaVersionV2, dto.Identity, raw)
	return VerifiedLineageV2{dto: dto, binding: b, canonical: append([]byte{}, raw...), denominator: d.binding, qualification: q.state, artifacts: artifacts}, nil
}

// CheckpointProjectionV2 is read-only output, not a token accepted for authority.
type CheckpointProjectionV2 struct {
	Subject, Envelope, Denominator, Lineage ArtifactBindingDTO
	TerminalProgramCHead                    TerminalProgramCHeadDTO
	WorkflowIdentity                        QualificationWorkflowIdentityDTO
	D                                       uint64
}

func ProjectVerifiedCheckpointV2(q CheckedQualificationCalculationV2, d VerifiedDenominatorV2, l VerifiedLineageV2) (CheckpointProjectionV2, error) {
	if q.state == nil || d.qualification != q.state || l.qualification != q.state || l.denominator != d.binding {
		return CheckpointProjectionV2{}, fmt.Errorf("v2 checkpoint requires common verified custody")
	}
	return CheckpointProjectionV2{Subject: q.state.subject, Envelope: q.state.envelope, Denominator: d.binding, Lineage: l.binding, TerminalProgramCHead: q.state.subjectDTO.TerminalProgramCHead, WorkflowIdentity: q.state.subjectDTO.WorkflowIdentity, D: d.dto.Accounting.D}, nil
}
