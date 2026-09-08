package qualificationpolicy

import (
	"fmt"
	"sort"
)

// SubstrateStatus is an upstream evaluator's explicit verdict for one Program A
// substrate dimension. Admission composes these verdicts; it does not replace
// the independent evaluators that produce them.
type SubstrateStatus string

const (
	SubstrateAdmitted SubstrateStatus = "ADMITTED"
	SubstrateMissing  SubstrateStatus = "MISSING"
	SubstrateRejected SubstrateStatus = "REJECTED"
)

// SubstrateEvidence binds an upstream verdict to retained evidence. Sequence is
// used only for the normalization -> support accounting -> projection ordering
// constraint; non-pipeline dimensions use zero.
type SubstrateEvidence struct {
	Status     SubstrateStatus `json:"status"`
	EvidenceID string          `json:"evidence_id,omitempty"`
	Sequence   int             `json:"sequence,omitempty"`
}

type ProgramASubstrate struct {
	Custody                SubstrateEvidence `json:"custody"`
	EffectiveConfiguration SubstrateEvidence `json:"effective_configuration"`
	Identity               SubstrateEvidence `json:"identity"`
	RelationNormalization  SubstrateEvidence `json:"relation_normalization"`
	SupportAccounting      SubstrateEvidence `json:"support_accounting"`
	Projection             SubstrateEvidence `json:"projection"`
}

type ProgramAAdmission struct {
	Status  SubstrateStatus `json:"status"`
	Reasons []string        `json:"reasons"`
}

// AdmitProgramA composes independently evaluated substrate verdicts. It does
// not infer acceptance from the presence of an existing primitive.
func AdmitProgramA(input ProgramASubstrate) (ProgramAAdmission, error) {
	dimensions := []struct {
		name     string
		evidence SubstrateEvidence
	}{
		{"custody", input.Custody},
		{"effective_configuration", input.EffectiveConfiguration},
		{"identity", input.Identity},
		{"relation_normalization", input.RelationNormalization},
		{"support_accounting", input.SupportAccounting},
		{"projection", input.Projection},
	}

	reasons := make([]string, 0)
	for _, dimension := range dimensions {
		switch dimension.evidence.Status {
		case SubstrateAdmitted:
			if dimension.evidence.EvidenceID == "" {
				reasons = append(reasons, dimension.name+": ADMITTED without evidence")
			}
		case SubstrateMissing, SubstrateRejected:
			reasons = append(reasons, dimension.name+": "+string(dimension.evidence.Status))
		default:
			return ProgramAAdmission{Status: SubstrateRejected, Reasons: []string{}}, fmt.Errorf("%s has unknown status %q", dimension.name, dimension.evidence.Status)
		}
	}

	pipelineAdmitted := input.RelationNormalization.Status == SubstrateAdmitted && input.RelationNormalization.EvidenceID != "" &&
		input.SupportAccounting.Status == SubstrateAdmitted && input.SupportAccounting.EvidenceID != "" &&
		input.Projection.Status == SubstrateAdmitted && input.Projection.EvidenceID != ""
	if pipelineAdmitted {
		if input.RelationNormalization.Sequence >= input.SupportAccounting.Sequence {
			reasons = append(reasons, "pipeline: relation_normalization must precede support_accounting")
		}
		if input.SupportAccounting.Sequence >= input.Projection.Sequence {
			reasons = append(reasons, "pipeline: support_accounting must precede projection")
		}
	}

	sort.Strings(reasons)
	if len(reasons) > 0 {
		return ProgramAAdmission{Status: SubstrateRejected, Reasons: reasons}, nil
	}
	return ProgramAAdmission{Status: SubstrateAdmitted, Reasons: []string{}}, nil
}
