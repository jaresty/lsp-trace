package inventorysuccessor

import (
	"bytes"
	"context"
	"encoding/json"
	"io"

	"lsp-trace/internal/censussourcecontinuation"
)

const DescribePreparationGapLedgerVersion = "lsp-trace.describe-preparation-gap-ledger.private.v1"

type GapStatus string

const (
	GapStatusNotRetained GapStatus = "NOT_RETAINED"
	GapStatusRetained    GapStatus = "RETAINED"
)

type ComputationStatus string

const ComputationNotComputed ComputationStatus = "NOT_COMPUTED"

type GapRequirement struct {
	ID                 EvidenceGap `json:"id"`
	Status             GapStatus   `json:"status"`
	EvidenceSelector   *string     `json:"evidence_selector"`
	EvidenceByteLength *uint64     `json:"evidence_byte_length"`
}

type ComputationState struct {
	Status ComputationStatus `json:"status"`
	Count  *uint64           `json:"count"`
}

type DescribePreparationGapLedger struct {
	SchemaVersion            string           `json:"schema_version"`
	Selector                 string           `json:"selector"`
	SuccessorHeadSelector    string           `json:"successor_head_selector"`
	ProgramCPredecessorHead  string           `json:"program_c_predecessor_head"`
	WorkflowIdentity         WorkflowIdentity `json:"workflow_identity"`
	Outcome                  Outcome          `json:"outcome"`
	CurrentStage             Stage            `json:"current_stage"`
	LastCompletedStage       Stage            `json:"last_completed_stage"`
	Requirements             []GapRequirement `json:"requirements"`
	RepresentativeNomination ComputationState `json:"representative_nomination"`
	CandidatePreparation     ComputationState `json:"candidate_preparation"`
	Authority                uint8            `json:"authority"`
	Accepted                 bool             `json:"accepted"`
	FeatureIdentity          string           `json:"feature_identity"`
	Completeness             string           `json:"completeness"`
	Effects                  string           `json:"effects"`
	ImplementationDecision   string           `json:"implementation_decision"`
	ProductionAdmission      string           `json:"production_admission"`
	SourceGraphComplete      string           `json:"source_graph_complete"`
	AbsenceInference         string           `json:"absence_inference"`
}

func encodeGapLedger(value *DescribePreparationGapLedger) ([]byte, error) {
	value.Selector = ""
	unsigned, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	value.Selector = digest(unsigned)
	return json.Marshal(value)
}

func strictGapLedger(raw []byte, value *DescribePreparationGapLedger) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return ErrInvalid
	}
	canonical, err := json.Marshal(value)
	if err != nil || !bytes.Equal(canonical, raw) {
		return ErrInvalid
	}
	return nil
}

func expectedGapRequirements() []GapRequirement {
	out := make([]GapRequirement, len(describePreparationGaps))
	for index, gap := range describePreparationGaps {
		out[index] = GapRequirement{ID: gap, Status: GapStatusNotRetained}
	}
	return out
}

type GapLedgerRequest struct {
	WorkflowKey string
	HeadKey     string
	Identity    WorkflowIdentity
	Heads       censussourcecontinuation.HeadStore
}

// BuildDescribePreparationGapLedger derives a read-only refusal ledger from a
// strictly verified head loaded through retained successor custody.
func BuildDescribePreparationGapLedger(ctx context.Context, request GapLedgerRequest) ([]byte, error) {
	if ctx == nil || request.WorkflowKey == "" || request.HeadKey == "" || !validIdentity(request.Identity) || request.Heads == nil {
		return nil, ErrInvalid
	}
	successorHeadRaw, err := request.Heads.Load(ctx, request.HeadKey)
	if err != nil {
		return nil, err
	}
	var successor head
	if strict(successorHeadRaw, &successor) != nil || validateHead(successor) != nil || successor.WorkflowKey != request.WorkflowKey || successor.Identity != request.Identity {
		return nil, ErrInvalid
	}
	copyHead := successor
	canonicalHead, err := encodeHead(&copyHead)
	if err != nil || !bytes.Equal(canonicalHead, successorHeadRaw) {
		return nil, ErrInvalid
	}
	ledger := DescribePreparationGapLedger{
		SchemaVersion: DescribePreparationGapLedgerVersion, SuccessorHeadSelector: successor.Selector,
		ProgramCPredecessorHead: successor.PredecessorHead, WorkflowIdentity: successor.Identity,
		Outcome: OutcomeWaitingEvidence, CurrentStage: StageDescribePreparation, LastCompletedStage: StageProgramC,
		Requirements:             expectedGapRequirements(),
		RepresentativeNomination: ComputationState{Status: ComputationNotComputed},
		CandidatePreparation:     ComputationState{Status: ComputationNotComputed},
		Authority:                0, Accepted: false, FeatureIdentity: "UNRESOLVED", Completeness: "UNKNOWN", Effects: "NONE",
		ImplementationDecision: "UNISSUED", ProductionAdmission: "REFUSED_MISSING_EVIDENCE",
		SourceGraphComplete: "UNKNOWN", AbsenceInference: "PROHIBITED",
	}
	raw, err := encodeGapLedger(&ledger)
	if err != nil || validateGapLedger(ledger) != nil {
		return nil, ErrInvalid
	}
	return raw, nil
}

func ValidateDescribePreparationGapLedger(raw []byte) (DescribePreparationGapLedger, error) {
	var ledger DescribePreparationGapLedger
	if strictGapLedger(raw, &ledger) != nil || validateGapLedger(ledger) != nil {
		return DescribePreparationGapLedger{}, ErrInvalid
	}
	copyLedger := ledger
	canonical, err := encodeGapLedger(&copyLedger)
	if err != nil || !bytes.Equal(canonical, raw) {
		return DescribePreparationGapLedger{}, ErrInvalid
	}
	return ledger, nil
}

func validateGapLedger(value DescribePreparationGapLedger) error {
	if value.SchemaVersion != DescribePreparationGapLedgerVersion || !sha256Pattern.MatchString(value.Selector) ||
		!sha256Pattern.MatchString(value.SuccessorHeadSelector) || !sha256Pattern.MatchString(value.ProgramCPredecessorHead) ||
		!validIdentity(value.WorkflowIdentity) || value.Outcome != OutcomeWaitingEvidence || value.CurrentStage != StageDescribePreparation ||
		value.LastCompletedStage != StageProgramC || len(value.Requirements) != len(describePreparationGaps) ||
		value.RepresentativeNomination.Status != ComputationNotComputed || value.RepresentativeNomination.Count != nil ||
		value.CandidatePreparation.Status != ComputationNotComputed || value.CandidatePreparation.Count != nil ||
		value.Authority != 0 || value.Accepted || value.FeatureIdentity != "UNRESOLVED" || value.Completeness != "UNKNOWN" ||
		value.Effects != "NONE" || value.ImplementationDecision != "UNISSUED" || value.ProductionAdmission != "REFUSED_MISSING_EVIDENCE" ||
		value.SourceGraphComplete != "UNKNOWN" || value.AbsenceInference != "PROHIBITED" {
		return ErrInvalid
	}
	for index, requirement := range value.Requirements {
		if requirement.ID != describePreparationGaps[index] || requirement.Status != GapStatusNotRetained || requirement.EvidenceSelector != nil || requirement.EvidenceByteLength != nil {
			return ErrInvalid
		}
	}
	return nil
}
