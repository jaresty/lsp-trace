package inventorysuccessor

import (
	"bytes"
	"encoding/json"

	"lsp-trace/internal/censussourceassembly"
	"lsp-trace/internal/censussourcecontinuation"
	"lsp-trace/internal/programcrepresentative"
	"lsp-trace/internal/publication"
)

const multiSeedSuccessorVersionV2 = "lsp-trace.inventory-successor-head.private.v2"
const multiSeedGapLedgerVersionV2 = "lsp-trace.describe-preparation-gap-ledger.private.v2"

func multiSeedEvidenceHeadVersionV2(kind string) string {
	return "lsp-trace.program-c-representative-" + kind + "-head.private.v2"
}

func marshalMultiSeedSizedV2(v any, n uint64, err error) ([]byte, error) {
	if err != nil {
		return nil, err
	}
	if n > programcrepresentative.MaxArtifactCanonicalBytesV2 {
		return nil, ErrInvalid
	}
	raw, err := json.Marshal(v)
	if err != nil || uint64(len(raw)) != n {
		return nil, ErrInvalid
	}
	return raw, nil
}
func validMultiSeedBindingV2(b programcrepresentative.ArtifactBindingDTO, schema string) bool {
	return b.SchemaVersion == schema && sha256Pattern.MatchString(b.Identity) && sha256Pattern.MatchString(b.SHA256) && b.ByteLength > 0 && b.ByteLength <= programcrepresentative.MaxArtifactCanonicalBytesV2
}

func encodeMultiSeedSuccessorV2(v programcrepresentative.SuccessorHeadV2) ([]byte, error) {
	v.Selector = ""
	n, err := programcrepresentative.CanonicalSizeSuccessorHeadV2(v)
	raw, err := marshalMultiSeedSizedV2(v, n, err)
	if err != nil {
		return nil, err
	}
	v.Selector = digest(raw)
	n, err = programcrepresentative.CanonicalSizeSuccessorHeadV2(v)
	return marshalMultiSeedSizedV2(v, n, err)
}
func parseMultiSeedSuccessorV2(raw []byte) (programcrepresentative.SuccessorHeadV2, error) {
	var v programcrepresentative.SuccessorHeadV2
	if uint64(len(raw)) > programcrepresentative.MaxArtifactCanonicalBytesV2 || strict(raw, &v) != nil || v.SchemaVersion != multiSeedSuccessorVersionV2 || v.Generation != 2 || len(v.MissingEvidence) != 3 {
		return programcrepresentative.SuccessorHeadV2{}, ErrInvalid
	}
	for i, gap := range describePreparationGaps[2:] {
		if v.MissingEvidence[i] != string(gap) {
			return programcrepresentative.SuccessorHeadV2{}, ErrInvalid
		}
	}
	base := head{SchemaVersion: headVersion, Selector: v.Selector, WorkflowKey: v.WorkflowKey, Identity: WorkflowIdentity(v.Identity), Authority: v.Authority, Generation: 1, PredecessorHead: v.PredecessorHead, CurrentStage: Stage(v.CurrentStage), LastCompletedStage: Stage(v.LastCompletedStage), Outcome: Outcome(v.Outcome), MissingEvidence: describePreparationGaps, Accepted: v.Accepted, FeatureIdentity: v.FeatureIdentity, Completeness: v.Completeness, Effects: v.Effects, ImplementationDecision: v.ImplementationDecision, ProductionAdmission: v.ProductionAdmission, SourceGraphComplete: v.SourceGraphComplete, AbsenceInference: v.AbsenceInference}
	if validateHead(base) != nil || !validMultiSeedBindingV2(v.PriorSuccessor, headVersion) || !validMultiSeedBindingV2(v.Denominator, programcrepresentative.DenominatorSchemaVersionV2) || !validMultiSeedBindingV2(v.Lineage, programcrepresentative.LineageSchemaVersionV2) {
		return programcrepresentative.SuccessorHeadV2{}, ErrInvalid
	}
	canonical, err := encodeMultiSeedSuccessorV2(v)
	if err != nil || !bytes.Equal(canonical, raw) {
		return programcrepresentative.SuccessorHeadV2{}, ErrInvalid
	}
	return v, nil
}
func ParseMultiSeedSuccessorHeadRevisionV2(raw []byte) (censussourcecontinuation.HeadRevision, error) {
	if len(raw) == 0 {
		return censussourcecontinuation.HeadRevision{}, nil
	}
	if uint64(len(raw)) > programcrepresentative.MaxArtifactCanonicalBytesV2 {
		return censussourcecontinuation.HeadRevision{}, ErrInvalid
	}
	var version struct {
		SchemaVersion string `json:"schema_version"`
	}
	if json.Unmarshal(raw, &version) != nil {
		return censussourcecontinuation.HeadRevision{}, ErrInvalid
	}
	if version.SchemaVersion == headVersion {
		return ParseHeadRevision(raw)
	}
	v, err := parseMultiSeedSuccessorV2(raw)
	if err != nil {
		return censussourcecontinuation.HeadRevision{}, err
	}
	return censussourcecontinuation.HeadRevision{Generation: v.Generation, Selector: v.Selector}, nil
}

// Same physical namespace and key as frozen v1; resource-job v2 is not used.
func NewMultiSeedSuccessorHeadsV2(root *publication.Root) (censussourcecontinuation.HeadStore, error) {
	return censussourceassembly.NewHeadsWithParser(root, headNamespace, ParseMultiSeedSuccessorHeadRevisionV2)
}

func encodeMultiSeedEvidenceHeadV2(v programcrepresentative.EvidenceHeadV2) ([]byte, error) {
	v.Selector = ""
	n, err := programcrepresentative.CanonicalSizeEvidenceHeadV2(v)
	raw, err := marshalMultiSeedSizedV2(v, n, err)
	if err != nil {
		return nil, err
	}
	v.Selector = digest(raw)
	n, err = programcrepresentative.CanonicalSizeEvidenceHeadV2(v)
	return marshalMultiSeedSizedV2(v, n, err)
}
func parseMultiSeedEvidenceHeadV2(raw []byte, kind string) (programcrepresentative.EvidenceHeadV2, error) {
	var v programcrepresentative.EvidenceHeadV2
	if kind != "denominator" && kind != "lineage" {
		return v, ErrInvalid
	}
	if uint64(len(raw)) > programcrepresentative.MaxArtifactCanonicalBytesV2 || strict(raw, &v) != nil || v.SchemaVersion != multiSeedEvidenceHeadVersionV2(kind) || v.WorkflowKey == "" || v.HeadKey == "" || v.Generation != 1 || v.PredecessorHead != "" || v.ClaimCeiling != programcrepresentative.FixedClaimCeilingV2() || !validMultiSeedBindingV2(v.Subject, programcrepresentative.QualificationSubjectSchemaVersionV2) || !validMultiSeedBindingV2(v.QualificationEnvelope, programcrepresentative.QualificationEnvelopeSchemaVersionV2) || !validMultiSeedBindingV2(v.Denominator, programcrepresentative.DenominatorSchemaVersionV2) {
		return programcrepresentative.EvidenceHeadV2{}, ErrInvalid
	}
	schema := programcrepresentative.DenominatorSchemaVersionV2
	if kind == "lineage" {
		schema = programcrepresentative.LineageSchemaVersionV2
	}
	if !validMultiSeedBindingV2(v.Artifact, schema) || kind == "denominator" && v.Artifact != v.Denominator {
		return programcrepresentative.EvidenceHeadV2{}, ErrInvalid
	}
	canonical, err := encodeMultiSeedEvidenceHeadV2(v)
	if err != nil || !bytes.Equal(canonical, raw) {
		return programcrepresentative.EvidenceHeadV2{}, ErrInvalid
	}
	return v, nil
}
func NewMultiSeedEvidenceHeadsV2(root *publication.Root, kind string) (censussourcecontinuation.HeadStore, error) {
	if kind != "denominator" && kind != "lineage" {
		return nil, ErrInvalid
	}
	parser := func(raw []byte) (censussourcecontinuation.HeadRevision, error) {
		if len(raw) == 0 {
			return censussourcecontinuation.HeadRevision{}, nil
		}
		v, err := parseMultiSeedEvidenceHeadV2(raw, kind)
		if err != nil {
			return censussourcecontinuation.HeadRevision{}, err
		}
		return censussourcecontinuation.HeadRevision{Generation: v.Generation, Selector: v.Selector}, nil
	}
	return censussourceassembly.NewHeadsWithParser(root, "program-c-representative/"+kind+"/heads/private/v2", parser)
}
func gapLedgerForMultiSeedSuccessorV2(v programcrepresentative.SuccessorHeadV2, d uint64) ([]byte, error) {
	requirements := make([]programcrepresentative.GapRequirementV2, 5)
	for i, gap := range describePreparationGaps {
		requirements[i] = programcrepresentative.GapRequirementV2{ID: string(gap), Status: "NOT_RETAINED"}
		if i < 2 {
			b := v.Denominator
			if i == 1 {
				b = v.Lineage
			}
			selector, length := b.SHA256, b.ByteLength
			requirements[i].Status = "RETAINED"
			requirements[i].EvidenceSelector = &selector
			requirements[i].EvidenceByteLength = &length
		}
	}
	value := programcrepresentative.GapLedgerV2{SchemaVersion: multiSeedGapLedgerVersionV2, SuccessorHeadSelector: v.Selector, ProgramCPredecessorHead: v.PredecessorHead, WorkflowIdentity: v.Identity, Outcome: v.Outcome, CurrentStage: v.CurrentStage, LastCompletedStage: v.LastCompletedStage, Requirements: requirements, RepresentativeNomination: programcrepresentative.ComputationStateV2{Status: "COMPUTED", Count: &d}, CandidatePreparation: programcrepresentative.ComputationStateV2{Status: "NOT_COMPUTED"}, Authority: v.Authority, Accepted: v.Accepted, FeatureIdentity: v.FeatureIdentity, Completeness: v.Completeness, Effects: v.Effects, ImplementationDecision: v.ImplementationDecision, ProductionAdmission: v.ProductionAdmission, SourceGraphComplete: v.SourceGraphComplete, AbsenceInference: v.AbsenceInference}
	n, err := programcrepresentative.CanonicalSizeGapLedgerV2(value)
	raw, err := marshalMultiSeedSizedV2(value, n, err)
	if err != nil {
		return nil, err
	}
	value.Selector = digest(raw)
	n, err = programcrepresentative.CanonicalSizeGapLedgerV2(value)
	return marshalMultiSeedSizedV2(value, n, err)
}
