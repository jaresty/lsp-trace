package inventorysuccessor

import (
	"bytes"

	"lsp-trace/internal/programcrepresentative"
)

func encodeMultiSeedGapLedgerV2(v programcrepresentative.GapLedgerV2) ([]byte, error) {
	v.Selector = ""
	n, err := programcrepresentative.CanonicalSizeGapLedgerV2(v)
	raw, err := marshalMultiSeedSizedV2(v, n, err)
	if err != nil {
		return nil, err
	}
	v.Selector = digest(raw)
	n, err = programcrepresentative.CanonicalSizeGapLedgerV2(v)
	return marshalMultiSeedSizedV2(v, n, err)
}

// ParseMultiSeedGapLedgerV2 verifies wire structure/self-identity only. Trusted
// reopening still requires independently verified denominator/lineage and the
// exact immutable v1 predecessor snapshot through the checkpoint operation.
func ParseMultiSeedGapLedgerV2(raw []byte) (programcrepresentative.GapLedgerV2, error) {
	var v programcrepresentative.GapLedgerV2
	if uint64(len(raw)) > programcrepresentative.MaxArtifactCanonicalBytesV2 || strict(raw, &v) != nil || v.SchemaVersion != multiSeedGapLedgerVersionV2 || !sha256Pattern.MatchString(v.Selector) || !sha256Pattern.MatchString(v.SuccessorHeadSelector) || !sha256Pattern.MatchString(v.ProgramCPredecessorHead) || !validIdentity(WorkflowIdentity(v.WorkflowIdentity)) || v.Outcome != string(OutcomeWaitingEvidence) || v.CurrentStage != string(StageDescribePreparation) || v.LastCompletedStage != string(StageProgramC) || len(v.Requirements) != 5 || v.RepresentativeNomination.Status != "COMPUTED" || v.RepresentativeNomination.Count == nil || v.CandidatePreparation.Status != "NOT_COMPUTED" || v.CandidatePreparation.Count != nil || v.Authority != 0 || v.Accepted || v.FeatureIdentity != "UNRESOLVED" || v.Completeness != "UNKNOWN" || v.Effects != "NONE" || v.ImplementationDecision != "UNISSUED" || v.ProductionAdmission != "REFUSED_MISSING_EVIDENCE" || v.SourceGraphComplete != "UNKNOWN" || v.AbsenceInference != "PROHIBITED" {
		return programcrepresentative.GapLedgerV2{}, ErrInvalid
	}
	for i, r := range v.Requirements {
		if r.ID != string(describePreparationGaps[i]) {
			return programcrepresentative.GapLedgerV2{}, ErrInvalid
		}
		if i < 2 {
			if r.Status != "RETAINED" || r.EvidenceSelector == nil || !sha256Pattern.MatchString(*r.EvidenceSelector) || r.EvidenceByteLength == nil || *r.EvidenceByteLength == 0 || *r.EvidenceByteLength > programcrepresentative.MaxArtifactCanonicalBytesV2 {
				return programcrepresentative.GapLedgerV2{}, ErrInvalid
			}
		} else if r.Status != "NOT_RETAINED" || r.EvidenceSelector != nil || r.EvidenceByteLength != nil {
			return programcrepresentative.GapLedgerV2{}, ErrInvalid
		}
	}
	canonical, err := encodeMultiSeedGapLedgerV2(v)
	if err != nil || !bytes.Equal(raw, canonical) {
		return programcrepresentative.GapLedgerV2{}, ErrInvalid
	}
	return v, nil
}
