package programcrepresentative

// These are additive wire projections of the reserved successor/gap v2 shapes.
// The frozen v1 types and parsers remain unchanged in inventorysuccessor.
type SuccessorWorkflowIdentityV2 struct {
	Profile                   string `json:"profile"`
	Contract                  string `json:"contract"`
	CensusReceiptSHA256       string `json:"census_receipt_sha256"`
	AvailabilityLedgerSHA256  string `json:"availability_ledger_sha256"`
	SourcePinSHA256           string `json:"source_pin_sha256"`
	ProgramCPlanSHA256        string `json:"program_c_plan_sha256"`
	ProgramCBindingsSHA256    string `json:"program_c_bindings_sha256"`
	PrivacyPolicySHA256       string `json:"privacy_policy_sha256"`
	RetentionPolicySHA256     string `json:"retention_policy_sha256"`
	ResourcePolicySHA256      string `json:"resource_policy_sha256"`
	ReplayPolicySHA256        string `json:"replay_policy_sha256"`
	QualificationPolicySHA256 string `json:"qualification_policy_sha256"`
}
type SuccessorHeadV2 struct {
	SchemaVersion          string                      `json:"schema_version"`
	Selector               string                      `json:"selector"`
	WorkflowKey            string                      `json:"workflow_key"`
	Identity               SuccessorWorkflowIdentityV2 `json:"identity"`
	Authority              uint8                       `json:"authority"`
	Generation             uint64                      `json:"generation"`
	PredecessorHead        string                      `json:"predecessor_head"`
	CurrentStage           string                      `json:"current_stage"`
	LastCompletedStage     string                      `json:"last_completed_stage"`
	Outcome                string                      `json:"outcome"`
	MissingEvidence        []string                    `json:"missing_evidence"`
	Accepted               bool                        `json:"accepted"`
	FeatureIdentity        string                      `json:"feature_identity"`
	Completeness           string                      `json:"completeness"`
	Effects                string                      `json:"effects"`
	ImplementationDecision string                      `json:"implementation_decision"`
	ProductionAdmission    string                      `json:"production_admission"`
	SourceGraphComplete    string                      `json:"source_graph_complete"`
	AbsenceInference       string                      `json:"absence_inference"`
	PriorSuccessor         ArtifactBindingDTO          `json:"prior_successor"`
	Denominator            ArtifactBindingDTO          `json:"denominator"`
	Lineage                ArtifactBindingDTO          `json:"lineage"`
}
type GapRequirementV2 struct {
	ID                 string  `json:"id"`
	Status             string  `json:"status"`
	EvidenceSelector   *string `json:"evidence_selector"`
	EvidenceByteLength *uint64 `json:"evidence_byte_length"`
}
type ComputationStateV2 struct {
	Status string  `json:"status"`
	Count  *uint64 `json:"count"`
}
type GapLedgerV2 struct {
	SchemaVersion            string                      `json:"schema_version"`
	Selector                 string                      `json:"selector"`
	SuccessorHeadSelector    string                      `json:"successor_head_selector"`
	ProgramCPredecessorHead  string                      `json:"program_c_predecessor_head"`
	WorkflowIdentity         SuccessorWorkflowIdentityV2 `json:"workflow_identity"`
	Outcome                  string                      `json:"outcome"`
	CurrentStage             string                      `json:"current_stage"`
	LastCompletedStage       string                      `json:"last_completed_stage"`
	Requirements             []GapRequirementV2          `json:"requirements"`
	RepresentativeNomination ComputationStateV2          `json:"representative_nomination"`
	CandidatePreparation     ComputationStateV2          `json:"candidate_preparation"`
	Authority                uint8                       `json:"authority"`
	Accepted                 bool                        `json:"accepted"`
	FeatureIdentity          string                      `json:"feature_identity"`
	Completeness             string                      `json:"completeness"`
	Effects                  string                      `json:"effects"`
	ImplementationDecision   string                      `json:"implementation_decision"`
	ProductionAdmission      string                      `json:"production_admission"`
	SourceGraphComplete      string                      `json:"source_graph_complete"`
	AbsenceInference         string                      `json:"absence_inference"`
}
