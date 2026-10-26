package programcrepresentative

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
)

const (
	QualificationSubjectSchemaVersion = "lsp-trace.program-c-representative-qualification-subject.private.v1"
	Stage6PolicySchemaVersion         = "lsp-trace.program-c-representative-stage6-policy.private.v1"
	Stage6LimitsSchemaVersion         = "lsp-trace.program-c-representative-stage6-limits.private.v1"
	Stage6FinalizationSchemaVersion   = "lsp-trace.program-c-representative-stage6-finalization.private.v1"

	AcceptedADR0014SHA256                            = "sha256:85daf073ecb8836d3b5762bd44d6a6ba911d14e23a81847dd9741a274084ebda"
	AcceptedReceiptDiscoveryAmendmentSHA256          = "sha256:df8d12f62d3eff9cd7553cbd65db0e6b9b6561d326c5d220f4ae8b387a374d57"
	AcceptedPrequalificationMechanicsAmendmentSHA256 = "sha256:b0af4add81ab71ca34d91873d8faa728c58166f48ff9b5d46c7ae9c4eb385f47"
	AcceptedQualificationSubjectAmendmentSHA256      = "sha256:235facbbba34f479df4771be845f34b3a39c83988a339dca69eec4f2b6bad9a6"

	qualificationSubjectDomain = "lsp-trace:program-c-representative-qualification-subject:v1"
	stage6ImplementationDomain = "lsp-trace:program-c-representative-stage6-implementation:v1"
	stage6PolicyDomain         = "lsp-trace:program-c-representative-stage6-policy:v1"
	stage6LimitsDomain         = "lsp-trace:program-c-representative-stage6-limits:v1"
	stage6FinalizationDomain   = "lsp-trace:program-c-representative-stage6-finalization:v1"

	stage6ImplementationCommit = "b33cf8cdbda56eefd54c049832129eb67a056727"
	stage6ImplementationTree   = "f87ac81bfd429d196044b0283574d3de370dc1c3"
	programCHeadSchemaVersion  = "lsp-trace.census-program-c-transition-head.private.v2"
	programCReceiptSchemaV3    = "lsp-trace.census-program-c-transition-receipt.private.v3"
	programCReceiptSchemaV4    = "lsp-trace.census-program-c-transition-receipt.private.v4"
)

type QualificationSubjectDTO struct {
	SchemaVersion                                    string                           `json:"schema_version"`
	Profile                                          string                           `json:"profile"`
	AcceptedADRSHA256                                string                           `json:"accepted_adr_sha256"`
	AcceptedReceiptDiscoveryAmendmentSHA256          string                           `json:"accepted_receipt_discovery_amendment_sha256"`
	AcceptedPrequalificationMechanicsAmendmentSHA256 string                           `json:"accepted_prequalification_mechanics_amendment_sha256"`
	AcceptedQualificationSubjectAmendmentSHA256      string                           `json:"accepted_qualification_subject_amendment_sha256"`
	AdapterContract                                  AdapterContractDTO               `json:"adapter_contract"`
	MechanicsSchemaVersion                           string                           `json:"mechanics_schema_version"`
	MechanicsProfile                                 string                           `json:"mechanics_profile"`
	Stage6Implementation                             Stage6ImplementationDTO          `json:"stage6_implementation"`
	Stage6Policy                                     ArtifactBindingDTO               `json:"stage6_policy"`
	Stage6Limits                                     ArtifactBindingDTO               `json:"stage6_limits"`
	Stage6Finalization                               ArtifactBindingDTO               `json:"stage6_finalization"`
	CorpusIdentity                                   CorpusIdentityDTO                `json:"corpus_identity"`
	WorkflowIdentity                                 QualificationWorkflowIdentityDTO `json:"workflow_identity"`
	SourcePinSHA256                                  string                           `json:"source_pin_sha256"`
	ReceiptInventory                                 []ReceiptInventoryEntryDTO       `json:"receipt_inventory"`
	TerminalProgramCHead                             TerminalProgramCHeadDTO          `json:"terminal_program_c_head"`
	ClaimCeiling                                     QualificationClaimCeilingDTO     `json:"claim_ceiling"`
	Authority                                        uint8                            `json:"authority"`
	Accepted                                         bool                             `json:"accepted"`
	Identity                                         string                           `json:"identity"`
}

type Stage6ImplementationDTO struct {
	RepositoryCommit string                  `json:"repository_commit"`
	RepositoryTree   string                  `json:"repository_tree"`
	Files            []ImplementationFileDTO `json:"files"`
	Identity         string                  `json:"identity"`
}

type ImplementationFileDTO struct {
	Path        string `json:"path"`
	GitBlobSHA1 string `json:"git_blob_sha1"`
	SHA256      string `json:"sha256"`
	ByteLength  uint64 `json:"byte_length"`
}

type ArtifactBindingDTO struct {
	SchemaVersion string `json:"schema_version"`
	Identity      string `json:"identity"`
	SHA256        string `json:"sha256"`
	ByteLength    uint64 `json:"byte_length"`
}

type CorpusIdentityDTO struct {
	FrozenCorpusSHA256       string `json:"frozen_corpus_sha256"`
	CensusReceiptSHA256      string `json:"census_receipt_sha256"`
	AvailabilityLedgerSHA256 string `json:"availability_ledger_sha256"`
	ProgramCPlanSHA256       string `json:"program_c_plan_sha256"`
	ProgramCBindingsSHA256   string `json:"program_c_bindings_sha256"`
}

type QualificationWorkflowIdentityDTO struct {
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
	Provenance                string `json:"provenance"`
}

type ReceiptInventoryEntryDTO struct {
	PlanOrdinal          int    `json:"plan_ordinal"`
	ReceiptOrdinal       int    `json:"receipt_ordinal"`
	ReceiptSchemaVersion string `json:"receipt_schema_version"`
	ReceiptSelector      string `json:"receipt_selector"`
	ReceiptSHA256        string `json:"receipt_sha256"`
	ReceiptByteLength    uint64 `json:"receipt_byte_length"`
}

type TerminalProgramCHeadDTO struct {
	SchemaVersion    string `json:"schema_version"`
	WorkflowKey      string `json:"workflow_key"`
	HeadKey          string `json:"head_key"`
	Generation       uint64 `json:"generation"`
	Selector         string `json:"selector"`
	HeadBytesSHA256  string `json:"head_bytes_sha256"`
	ByteLength       uint64 `json:"byte_length"`
	CommittedPrefix  int    `json:"committed_prefix"`
	ConstituentCount int    `json:"constituent_count"`
}

type QualificationClaimCeilingDTO struct {
	Authority              uint8  `json:"authority"`
	Accepted               bool   `json:"accepted"`
	FeatureIdentity        string `json:"feature_identity"`
	Completeness           string `json:"completeness"`
	Effects                string `json:"effects"`
	Reviews                string `json:"reviews"`
	ImplementationDecision string `json:"implementation_decision"`
	ProductionAdmission    string `json:"production_admission"`
	SourceGraphComplete    string `json:"source_graph_complete"`
	AbsenceInference       string `json:"absence_inference"`
}

type Stage6PolicyDTO struct {
	SchemaVersion               string   `json:"schema_version"`
	Profile                     string   `json:"profile"`
	AdmittedKind                string   `json:"admitted_kind"`
	AdmittedDisposition         string   `json:"admitted_disposition"`
	SelfLoopDisposition         string   `json:"self_loop_disposition"`
	ContextNodeScope            string   `json:"context_node_scope"`
	RelationOccurrenceClosure   string   `json:"relation_occurrence_closure"`
	GraphDirection              string   `json:"graph_direction"`
	StronglyConnectedComponents string   `json:"strongly_connected_components"`
	PreparedRootDistance        string   `json:"prepared_root_distance"`
	CandidateScope              string   `json:"candidate_scope"`
	PreparedRootScope           string   `json:"prepared_root_scope"`
	SelectedSCCMembersScope     string   `json:"selected_scc_members_scope"`
	PredecessorScope            string   `json:"predecessor_scope"`
	NoCallsOutcome              string   `json:"no_calls_outcome"`
	SelectionFailureOutcome     string   `json:"selection_failure_outcome"`
	SelectedOutcome             string   `json:"selected_outcome"`
	SelectionOrder              []string `json:"selection_order"`
	PredecessorOrder            []string `json:"predecessor_order"`
	SingletonResultRequired     bool     `json:"singleton_result_required"`
	Authority                   uint8    `json:"authority"`
	Accepted                    bool     `json:"accepted"`
	Identity                    string   `json:"identity"`
}

type Stage6LimitsDTO struct {
	SchemaVersion     string `json:"schema_version"`
	Profile           string `json:"profile"`
	MaxCommunities    int    `json:"max_communities"`
	MaxMembers        int    `json:"max_members"`
	MaxOccurrences    int    `json:"max_occurrences"`
	MaxCandidates     int    `json:"max_candidates"`
	MaxCanonicalBytes int    `json:"max_canonical_bytes"`
	MaxStringBytes    int    `json:"max_string_bytes"`
	MaxWork           int    `json:"max_work"`
	Authority         uint8  `json:"authority"`
	Accepted          bool   `json:"accepted"`
	Identity          string `json:"identity"`
}

type Stage6FinalizationDTO struct {
	SchemaVersion           string   `json:"schema_version"`
	Profile                 string   `json:"profile"`
	CompositionFamily       string   `json:"composition_family"`
	CompositionVersion      string   `json:"composition_version"`
	GroupingFamily          string   `json:"grouping_family"`
	GroupingVersion         string   `json:"grouping_version"`
	GroupingAlgorithm       string   `json:"grouping_algorithm"`
	GroupingProfile         string   `json:"grouping_profile"`
	GroupingNumericContract string   `json:"grouping_numeric_contract"`
	QualificationFamily     string   `json:"qualification_family"`
	QualificationTrust      string   `json:"qualification_trust"`
	CompositionMappings     []string `json:"composition_mappings"`
	EvidenceMappings        []string `json:"evidence_mappings"`
	GroupingMappings        []string `json:"grouping_mappings"`
	ContextMappings         []string `json:"context_mappings"`
	QualificationMappings   []string `json:"qualification_mappings"`
	FinalizationOrder       []string `json:"finalization_order"`
	Authority               uint8    `json:"authority"`
	Accepted                bool     `json:"accepted"`
	Identity                string   `json:"identity"`
}

// QualificationSubjectExpectedContext is non-wire trusted input reconstructed by
// the private census mapper. MaxImplementationSourceBytes is an explicit
// verification bound because ADR 0014 freezes no numeric implementation-source
// ceiling; it is never serialized into the subject.
type QualificationSubjectExpectedContext struct {
	AdapterContract              AdapterContractDTO
	Stage6Implementation         Stage6ImplementationDTO
	Stage6Policy                 Stage6PolicyDTO
	Stage6PolicyBinding          ArtifactBindingDTO
	Stage6Limits                 Stage6LimitsDTO
	Stage6LimitsBinding          ArtifactBindingDTO
	Stage6Finalization           Stage6FinalizationDTO
	Stage6FinalizationBinding    ArtifactBindingDTO
	CorpusIdentity               CorpusIdentityDTO
	WorkflowIdentity             QualificationWorkflowIdentityDTO
	SourcePinSHA256              string
	ReceiptInventory             []ReceiptInventoryEntryDTO
	TerminalProgramCHead         TerminalProgramCHeadDTO
	MaxImplementationSourceBytes uint64
}

// ProgramCRepresentativeQualificationSubject is immutable. Parsing alone never
// establishes trust; trusted values are constructed or reconstructed from exact
// independently verified expected context.
type ProgramCRepresentativeQualificationSubject struct {
	dto       QualificationSubjectDTO
	canonical []byte
	trusted   bool
}

func (s ProgramCRepresentativeQualificationSubject) Valid() bool {
	return s.trusted && validateQualificationSubjectDTO(s.dto) == nil && len(s.canonical) > 0
}
func (s ProgramCRepresentativeQualificationSubject) Trusted() bool    { return s.Valid() }
func (s ProgramCRepresentativeQualificationSubject) Identity() string { return s.dto.Identity }
func (s ProgramCRepresentativeQualificationSubject) DTO() QualificationSubjectDTO {
	return cloneQualificationSubjectDTO(s.dto)
}
func (s ProgramCRepresentativeQualificationSubject) CanonicalBytes() []byte {
	return append([]byte(nil), s.canonical...)
}

func NewProgramCRepresentativeQualificationSubject(in QualificationSubjectExpectedContext) (ProgramCRepresentativeQualificationSubject, error) {
	if err := validateExpectedQualificationContext(in); err != nil {
		return ProgramCRepresentativeQualificationSubject{}, err
	}
	dto := QualificationSubjectDTO{
		SchemaVersion:                           QualificationSubjectSchemaVersion,
		Profile:                                 ProfileCallsOnlyFrozenCensusV1,
		AcceptedADRSHA256:                       AcceptedADR0014SHA256,
		AcceptedReceiptDiscoveryAmendmentSHA256: AcceptedReceiptDiscoveryAmendmentSHA256,
		AcceptedPrequalificationMechanicsAmendmentSHA256: AcceptedPrequalificationMechanicsAmendmentSHA256,
		AcceptedQualificationSubjectAmendmentSHA256:      AcceptedQualificationSubjectAmendmentSHA256,
		AdapterContract:        cloneAdapterContractDTO(in.AdapterContract),
		MechanicsSchemaVersion: MechanicsSchemaVersion,
		MechanicsProfile:       ProfileCallsOnlyFrozenCensusV1,
		Stage6Implementation:   cloneStage6ImplementationDTO(in.Stage6Implementation),
		Stage6Policy:           in.Stage6PolicyBinding,
		Stage6Limits:           in.Stage6LimitsBinding,
		Stage6Finalization:     in.Stage6FinalizationBinding,
		CorpusIdentity:         in.CorpusIdentity,
		WorkflowIdentity:       in.WorkflowIdentity,
		SourcePinSHA256:        in.SourcePinSHA256,
		ReceiptInventory:       append([]ReceiptInventoryEntryDTO(nil), in.ReceiptInventory...),
		TerminalProgramCHead:   in.TerminalProgramCHead,
		ClaimCeiling:           fixedQualificationClaimCeiling(),
		Authority:              0,
		Accepted:               false,
	}
	var err error
	dto.Identity, err = qualificationIdentity(dto)
	if err != nil {
		return ProgramCRepresentativeQualificationSubject{}, err
	}
	if err := validateQualificationSubjectDTO(dto); err != nil {
		return ProgramCRepresentativeQualificationSubject{}, err
	}
	canonical, err := json.Marshal(dto)
	if err != nil {
		return ProgramCRepresentativeQualificationSubject{}, err
	}
	return ProgramCRepresentativeQualificationSubject{dto: cloneQualificationSubjectDTO(dto), canonical: canonical, trusted: true}, nil
}

// ParseProgramCRepresentativeQualificationSubject performs strict canonical and
// self-identity validation, but intentionally returns an untrusted value.
func ParseProgramCRepresentativeQualificationSubject(raw []byte) (ProgramCRepresentativeQualificationSubject, error) {
	if err := rejectDuplicateJSONKeys(raw); err != nil {
		return ProgramCRepresentativeQualificationSubject{}, err
	}
	var dto QualificationSubjectDTO
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&dto); err != nil {
		return ProgramCRepresentativeQualificationSubject{}, err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return ProgramCRepresentativeQualificationSubject{}, fmt.Errorf("trailing qualification subject bytes")
	}
	if err := validateQualificationSubjectDTO(dto); err != nil {
		return ProgramCRepresentativeQualificationSubject{}, err
	}
	canonical, err := json.Marshal(dto)
	if err != nil || !bytes.Equal(canonical, raw) {
		return ProgramCRepresentativeQualificationSubject{}, fmt.Errorf("noncanonical qualification subject bytes")
	}
	return ProgramCRepresentativeQualificationSubject{dto: cloneQualificationSubjectDTO(dto), canonical: append([]byte(nil), raw...)}, nil
}

// ValidateProgramCRepresentativeQualificationSubject establishes trust only by
// rebuilding the complete expected subject and requiring exact populated bytes.
func ValidateProgramCRepresentativeQualificationSubject(raw []byte, expected QualificationSubjectExpectedContext) (ProgramCRepresentativeQualificationSubject, error) {
	parsed, err := ParseProgramCRepresentativeQualificationSubject(raw)
	if err != nil {
		return ProgramCRepresentativeQualificationSubject{}, err
	}
	constructed, err := NewProgramCRepresentativeQualificationSubject(expected)
	if err != nil {
		return ProgramCRepresentativeQualificationSubject{}, err
	}
	if !bytes.Equal(parsed.canonical, constructed.canonical) {
		return ProgramCRepresentativeQualificationSubject{}, fmt.Errorf("qualification subject does not match reconstructed context")
	}
	return constructed, nil
}

func BuildStage6PolicyArtifact() (Stage6PolicyDTO, ArtifactBindingDTO, []byte, error) {
	dto := Stage6PolicyDTO{
		SchemaVersion:               Stage6PolicySchemaVersion,
		Profile:                     ProfileCallsOnlyFrozenCensusV1,
		AdmittedKind:                "CALLS",
		AdmittedDisposition:         "INCLUDED",
		SelfLoopDisposition:         "RETAINED_INCLUDED_DERIVED_INELIGIBLE",
		ContextNodeScope:            "UNION_PREPARED_TARGET_AND_REACHED_NODE",
		RelationOccurrenceClosure:   "EXACT_ALL_CALLS_OCCURRENCES_PER_CONTEXT_RELATION",
		GraphDirection:              "CALLER_TO_CALLEE",
		StronglyConnectedComponents: "DETERMINISTIC_TARJAN_CANONICAL_NODE_ORDER",
		PreparedRootDistance:        "SHORTEST_DIRECTED_CONDENSATION_EDGE_COUNT",
		CandidateScope:              "REACHED_COMMUNITY_MEMBER_REACHABLE_FROM_PREPARED_ROOT_SCC",
		PreparedRootScope:           "ALL_SCCS_CONTAINING_PREPARED_TARGETS",
		SelectedSCCMembersScope:     "ALL_ACTIVE_CONTEXT_NODES_IN_SELECTED_SCC",
		PredecessorScope:            "INCLUDED_NON_SELF_CALLS_DIRECTLY_INCOMING_TO_SELECTED_NODE",
		NoCallsOutcome:              "NO_CALLS_REPRESENTATIVE",
		SelectionFailureOutcome:     "CALLS_EVIDENCE_INELIGIBLE",
		SelectedOutcome:             "SELECTED",
		SelectionOrder:              []string{"MIN_DISTANCE", "LEXICOGRAPHIC_NODE_ID"},
		PredecessorOrder:            []string{"RELATION_ID", "OCCURRENCE_ID"},
		SingletonResultRequired:     true,
	}
	identity, err := hashIdentity(stage6PolicyDomain, dto)
	if err != nil {
		return Stage6PolicyDTO{}, ArtifactBindingDTO{}, nil, err
	}
	dto.Identity = identity
	binding, canonical, err := artifactBinding(Stage6PolicySchemaVersion, dto.Identity, dto)
	return cloneStage6PolicyDTO(dto), binding, canonical, err
}

func BuildStage6LimitsArtifact() (Stage6LimitsDTO, ArtifactBindingDTO, []byte, error) {
	dto := Stage6LimitsDTO{
		SchemaVersion:     Stage6LimitsSchemaVersion,
		Profile:           ProfileCallsOnlyFrozenCensusV1,
		MaxCommunities:    10000,
		MaxMembers:        10000,
		MaxOccurrences:    100000,
		MaxCandidates:     10000,
		MaxCanonicalBytes: 67108864,
		MaxStringBytes:    1048576,
		MaxWork:           110000,
	}
	identity, err := hashIdentity(stage6LimitsDomain, dto)
	if err != nil {
		return Stage6LimitsDTO{}, ArtifactBindingDTO{}, nil, err
	}
	dto.Identity = identity
	binding, canonical, err := artifactBinding(Stage6LimitsSchemaVersion, dto.Identity, dto)
	return dto, binding, canonical, err
}

func BuildStage6FinalizationArtifact() (Stage6FinalizationDTO, ArtifactBindingDTO, []byte, error) {
	dto := Stage6FinalizationDTO{
		SchemaVersion:           Stage6FinalizationSchemaVersion,
		Profile:                 ProfileCallsOnlyFrozenCensusV1,
		CompositionFamily:       "PROGRAM_C_REPRESENTATIVE_COMPOSITION",
		CompositionVersion:      "private.v1",
		GroupingFamily:          "PROGRAM_C_REPRESENTATIVE_GROUPING",
		GroupingVersion:         "private.v1",
		GroupingAlgorithm:       "gonum.org/v1/gonum/graph/community.Leiden",
		GroupingProfile:         ProfileCallsOnlyFrozenCensusV1,
		GroupingNumericContract: "EXACT_INTEGER_DIRECTED_GRAPH_V1",
		QualificationFamily:     "PROGRAM_C_REPRESENTATIVE_QUALIFICATION",
		QualificationTrust:      "PRIVATE_INDEPENDENT_REVIEW_ENVELOPE_V1",
		CompositionMappings:     []string{"INPUT_FROM_CUSTODY", "POLICY_FROM_VERIFIED_POLICY", "COMPOSITION_ID_FROM_MECHANICS", "QUALIFICATION_BINDING_FROM_VERIFIED_ENVELOPE", "OCCURRENCES_FROM_EXACT_EVIDENCE_MAPPING", "ACCOUNTING_ALL_INCLUDED_NO_TRUNCATION", "MODEL_FIELDS_EMPTY"},
		EvidenceMappings:        []string{"IDENTITY_EXACT", "RELATION_ID_EXACT", "KIND_CALLS", "FROM_NODE_ID_TO_FROM", "TO_NODE_ID_TO_TO", "DISPOSITION_INCLUDED", "CONSTITUENT_SELECTOR_TO_SOURCE_PREDECESSOR", "RECOMPUTED_CUSTODY_IDENTITY_TO_CUSTODY", "CANONICAL_ARRAY_INDEX_TO_ORDINAL", "CALL_SITE_EXACT"},
		GroupingMappings:        []string{"LIMITS_FROM_VERIFIED_LIMITS", "COMPOSITION_DIGEST_AND_LENGTH_EXACT", "SEED_FROM_MECHANICS", "SINGLETON_PARTITION_FROM_COMMUNITY", "SINGLETON_CONTEXT_FROM_EXACT_CONTEXT_MAPPING"},
		ContextMappings:         []string{"IDENTITY_EXACT", "CONSTITUENT_SELECTOR_TO_CONSTITUENT_IDENTITY", "EXECUTION_BUNDLE_EXACT", "SINGLE_MEMBERSHIP_GROUP_SEED_LABEL_AND_AT", "RECOMPUTED_CUSTODY_ORDINAL", "COMMUNITY_AND_CLOSURE_ARRAYS_EXACT"},
		QualificationMappings:   []string{"COMPOSITION_DIGEST_EXACT", "GROUPING_DIGEST_TO_EVIDENCE_DIGEST", "EPOCH_TO_VALID_FROM_AND_VALID_THROUGH", "VERIFIER_RECONSTRUCTS_ALL_BINDINGS"},
		FinalizationOrder:       []string{"POPULATE_AND_HASH_COMPOSITION", "POPULATE_AND_HASH_GROUPING", "DERIVE_QUALIFICATION"},
	}
	identity, err := hashIdentity(stage6FinalizationDomain, dto)
	if err != nil {
		return Stage6FinalizationDTO{}, ArtifactBindingDTO{}, nil, err
	}
	dto.Identity = identity
	binding, canonical, err := artifactBinding(Stage6FinalizationSchemaVersion, dto.Identity, dto)
	return cloneStage6FinalizationDTO(dto), binding, canonical, err
}

func artifactBinding(schema, identity string, dto any) (ArtifactBindingDTO, []byte, error) {
	canonical, err := json.Marshal(dto)
	if err != nil {
		return ArtifactBindingDTO{}, nil, err
	}
	return ArtifactBindingDTO{SchemaVersion: schema, Identity: identity, SHA256: rawDigest(canonical), ByteLength: uint64(len(canonical))}, canonical, nil
}

func validateExpectedQualificationContext(in QualificationSubjectExpectedContext) error {
	if in.MaxImplementationSourceBytes == 0 {
		return fmt.Errorf("missing implementation source byte ceiling")
	}
	if err := validateAdapterContract(in.AdapterContract); err != nil {
		return err
	}
	if err := validateStage6Implementation(in.Stage6Implementation, in.MaxImplementationSourceBytes); err != nil {
		return err
	}
	policy, policyBinding, _, err := BuildStage6PolicyArtifact()
	if err != nil || !equalJSON(policy, in.Stage6Policy) || policyBinding != in.Stage6PolicyBinding {
		return fmt.Errorf("invalid Stage 6 policy binding")
	}
	limits, limitsBinding, _, err := BuildStage6LimitsArtifact()
	if err != nil || limits != in.Stage6Limits || limitsBinding != in.Stage6LimitsBinding {
		return fmt.Errorf("invalid Stage 6 limits binding")
	}
	finalization, finalizationBinding, _, err := BuildStage6FinalizationArtifact()
	if err != nil || !equalJSON(finalization, in.Stage6Finalization) || finalizationBinding != in.Stage6FinalizationBinding {
		return fmt.Errorf("invalid Stage 6 finalization binding")
	}
	if err := validateCorpusWorkflow(in.CorpusIdentity, in.WorkflowIdentity, in.SourcePinSHA256); err != nil {
		return err
	}
	if err := validateReceiptAndHead(in.ReceiptInventory, in.TerminalProgramCHead); err != nil {
		return err
	}
	return nil
}

func validateQualificationSubjectDTO(dto QualificationSubjectDTO) error {
	if dto.SchemaVersion != QualificationSubjectSchemaVersion || dto.Profile != ProfileCallsOnlyFrozenCensusV1 ||
		dto.AcceptedADRSHA256 != AcceptedADR0014SHA256 ||
		dto.AcceptedReceiptDiscoveryAmendmentSHA256 != AcceptedReceiptDiscoveryAmendmentSHA256 ||
		dto.AcceptedPrequalificationMechanicsAmendmentSHA256 != AcceptedPrequalificationMechanicsAmendmentSHA256 ||
		dto.AcceptedQualificationSubjectAmendmentSHA256 != AcceptedQualificationSubjectAmendmentSHA256 ||
		dto.MechanicsSchemaVersion != MechanicsSchemaVersion || dto.MechanicsProfile != ProfileCallsOnlyFrozenCensusV1 ||
		dto.Authority != 0 || dto.Accepted || !validDigest(dto.Identity) {
		return fmt.Errorf("invalid qualification subject envelope")
	}
	if err := validateAdapterContract(dto.AdapterContract); err != nil {
		return err
	}
	if err := validateStage6Implementation(dto.Stage6Implementation, ^uint64(0)); err != nil {
		return err
	}
	_, policy, _, err := BuildStage6PolicyArtifact()
	if err != nil || dto.Stage6Policy != policy {
		return fmt.Errorf("invalid qualification subject policy")
	}
	_, limits, _, err := BuildStage6LimitsArtifact()
	if err != nil || dto.Stage6Limits != limits {
		return fmt.Errorf("invalid qualification subject limits")
	}
	_, finalization, _, err := BuildStage6FinalizationArtifact()
	if err != nil || dto.Stage6Finalization != finalization {
		return fmt.Errorf("invalid qualification subject finalization")
	}
	if err := validateCorpusWorkflow(dto.CorpusIdentity, dto.WorkflowIdentity, dto.SourcePinSHA256); err != nil {
		return err
	}
	if err := validateReceiptAndHead(dto.ReceiptInventory, dto.TerminalProgramCHead); err != nil {
		return err
	}
	if dto.ClaimCeiling != fixedQualificationClaimCeiling() {
		return fmt.Errorf("invalid qualification claim ceiling")
	}
	identity, err := qualificationIdentity(dto)
	if err != nil || identity != dto.Identity {
		return fmt.Errorf("invalid qualification subject identity")
	}
	return nil
}

func validateAdapterContract(dto AdapterContractDTO) error {
	if dto.ProgramCAlgorithmVersion == "" {
		return fmt.Errorf("invalid qualification adapter contract")
	}
	expected, err := newAdapterContract(dto.ProgramCAlgorithmVersion)
	if err != nil || !equalJSON(expected, dto) {
		return fmt.Errorf("invalid qualification adapter contract")
	}
	return nil
}

func validateStage6Implementation(dto Stage6ImplementationDTO, maxBytes uint64) error {
	expected := fixedImplementationFiles()
	if dto.RepositoryCommit != stage6ImplementationCommit || dto.RepositoryTree != stage6ImplementationTree || dto.Files == nil || len(dto.Files) != len(expected) || !validDigest(dto.Identity) {
		return fmt.Errorf("invalid Stage 6 implementation manifest")
	}
	var total uint64
	for i := range expected {
		file := dto.Files[i]
		if file != expected[i] || !validGitSHA1(file.GitBlobSHA1) || !validDigest(file.SHA256) || file.ByteLength == 0 || total > ^uint64(0)-file.ByteLength {
			return fmt.Errorf("invalid Stage 6 implementation file")
		}
		total += file.ByteLength
		if total > maxBytes {
			return fmt.Errorf("Stage 6 implementation source byte ceiling exceeded")
		}
	}
	copy := cloneStage6ImplementationDTO(dto)
	copy.Identity = ""
	identity, err := hashIdentity(stage6ImplementationDomain, copy)
	if err != nil || identity != dto.Identity {
		return fmt.Errorf("invalid Stage 6 implementation identity")
	}
	return nil
}

func fixedImplementationFiles() []ImplementationFileDTO {
	return []ImplementationFileDTO{
		{Path: "internal/censuscontinuation/programc_stage6_adapter_private.go", GitBlobSHA1: "4144ff5a3980de6c56305d94eeb844a4b7f74cd9", SHA256: "sha256:eaba97734c84b0c06da51a6b0d0e5d94c41dee81feded7634b7acb91c9439dfa", ByteLength: 8699},
		{Path: "internal/censuscontinuation/programc_transition.go", GitBlobSHA1: "eb308697b4e8c968478180a8b775a314a43d43e4", SHA256: "sha256:c67659e1df91a821a2152d6cb70973238a646fbca3d5c2d3fc0d5421f147d29e", ByteLength: 38854},
		{Path: "internal/graph/native_v5_representative.go", GitBlobSHA1: "f80851433cb22cfbf52cc5464c609a4801c7714f", SHA256: "sha256:a743139187ecc7eb9091537f7baa0c45de36fbe0bbca9d99bbc04bea3b9ed0c2", ByteLength: 3636},
		{Path: "internal/programc/core.go", GitBlobSHA1: "e88fac874a84ecde11296fc7db96210d50bfe2ea", SHA256: "sha256:51d0abcb92482e9192086d6a60ea1606db129de459190dade62300b0b0054094", ByteLength: 20636},
		{Path: "internal/programcrepresentative/mechanics.go", GitBlobSHA1: "8fe46ec75f2eaec186579ed19ccbe6357f4ae72a", SHA256: "sha256:b1198396497bac5c2cfb96f9f116149fff9a9357e37574a3a5b72481e7722da4", ByteLength: 25846},
		{Path: "internal/programcrepresentative/representative.go", GitBlobSHA1: "101c9ca01df288db8aec2344d693143edbce8082", SHA256: "sha256:0b4faac4e05e4e85d48fe6e05507b053d2111770d08af13ef9e5d8acbeee5d2e", ByteLength: 15636},
		{Path: "internal/programcrepresentative/stage6_result.go", GitBlobSHA1: "2ca877428e66a60652947cd7f4c54fb16e835774", SHA256: "sha256:83e052984fbe64221ff3484da4430a4cc5f617d4a6ad68be757a30994430ed8d", ByteLength: 21974},
	}
}

func validateCorpusWorkflow(c CorpusIdentityDTO, w QualificationWorkflowIdentityDTO, sourcePin string) error {
	for _, digest := range []string{c.FrozenCorpusSHA256, c.CensusReceiptSHA256, c.AvailabilityLedgerSHA256, c.ProgramCPlanSHA256, c.ProgramCBindingsSHA256, w.CensusReceiptSHA256, w.AvailabilityLedgerSHA256, w.SourcePinSHA256, w.ProgramCPlanSHA256, w.ProgramCBindingsSHA256, w.PrivacyPolicySHA256, w.RetentionPolicySHA256, w.ResourcePolicySHA256, w.ReplayPolicySHA256, w.QualificationPolicySHA256, sourcePin} {
		if !validDigest(digest) {
			return fmt.Errorf("invalid corpus or workflow digest")
		}
	}
	if w.Profile != ProfileCallsOnlyFrozenCensusV1 || w.Contract == "" || w.Provenance != "PREDECESSOR_RETAINED_CALLER_ASSERTED" ||
		w.CensusReceiptSHA256 != c.CensusReceiptSHA256 || w.AvailabilityLedgerSHA256 != c.AvailabilityLedgerSHA256 ||
		w.ProgramCPlanSHA256 != c.ProgramCPlanSHA256 || w.ProgramCBindingsSHA256 != c.ProgramCBindingsSHA256 || sourcePin != w.SourcePinSHA256 {
		return fmt.Errorf("invalid qualification workflow binding")
	}
	return nil
}

func validateReceiptAndHead(receipts []ReceiptInventoryEntryDTO, head TerminalProgramCHeadDTO) error {
	if receipts == nil || len(receipts) == 0 || head.SchemaVersion != programCHeadSchemaVersion || head.WorkflowKey == "" || head.HeadKey == "" || head.Generation == 0 || !validDigest(head.Selector) || !validDigest(head.HeadBytesSHA256) || head.ByteLength == 0 || head.CommittedPrefix < 0 || head.ConstituentCount < 0 || len(receipts) != head.CommittedPrefix || len(receipts) != head.ConstituentCount {
		return fmt.Errorf("invalid receipt inventory terminal head")
	}
	seenOrdinals := make(map[int]struct{}, len(receipts))
	receiptLengths := make(map[string]uint64, len(receipts))
	for i, receipt := range receipts {
		if receipt.PlanOrdinal != i || receipt.ReceiptOrdinal <= 0 || (receipt.ReceiptSchemaVersion != programCReceiptSchemaV3 && receipt.ReceiptSchemaVersion != programCReceiptSchemaV4) || !validDigest(receipt.ReceiptSelector) || receipt.ReceiptSelector != receipt.ReceiptSHA256 || receipt.ReceiptByteLength == 0 {
			return fmt.Errorf("invalid receipt inventory entry")
		}
		if _, duplicate := seenOrdinals[receipt.ReceiptOrdinal]; duplicate {
			return fmt.Errorf("duplicate receipt ordinal")
		}
		receiptLength, duplicateReceipt := receiptLengths[receipt.ReceiptSelector]
		if duplicateReceipt && receiptLength != receipt.ReceiptByteLength {
			return fmt.Errorf("conflicting receipt selector or digest byte length")
		}
		if duplicateReceipt {
			return fmt.Errorf("duplicate receipt selector or digest")
		}
		seenOrdinals[receipt.ReceiptOrdinal] = struct{}{}
		receiptLengths[receipt.ReceiptSelector] = receipt.ReceiptByteLength
	}
	return nil
}

func fixedQualificationClaimCeiling() QualificationClaimCeilingDTO {
	return QualificationClaimCeilingDTO{
		Authority:              0,
		Accepted:               false,
		FeatureIdentity:        "UNRESOLVED",
		Completeness:           "UNKNOWN",
		Effects:                "NONE",
		Reviews:                "NOT_REQUESTED",
		ImplementationDecision: "UNISSUED",
		ProductionAdmission:    "REFUSED_MISSING_EVIDENCE",
		SourceGraphComplete:    "UNKNOWN",
		AbsenceInference:       "PROHIBITED",
	}
}

func qualificationIdentity(dto QualificationSubjectDTO) (string, error) {
	copy := cloneQualificationSubjectDTO(dto)
	copy.Identity = ""
	return hashIdentity(qualificationSubjectDomain, copy)
}

func validGitSHA1(value string) bool {
	if len(value) != 40 || value != string(bytes.ToLower([]byte(value))) {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func equalJSON(a, b any) bool {
	x, errA := json.Marshal(a)
	y, errB := json.Marshal(b)
	return errA == nil && errB == nil && bytes.Equal(x, y)
}

func rejectDuplicateJSONKeys(raw []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	var walk func() error
	walk = func() error {
		token, err := decoder.Token()
		if err != nil {
			return err
		}
		delim, ok := token.(json.Delim)
		if !ok {
			return nil
		}
		switch delim {
		case '{':
			seen := map[string]bool{}
			for decoder.More() {
				keyToken, err := decoder.Token()
				if err != nil {
					return err
				}
				key, ok := keyToken.(string)
				if !ok || seen[key] {
					return fmt.Errorf("duplicate or invalid qualification subject key")
				}
				seen[key] = true
				if err := walk(); err != nil {
					return err
				}
			}
			end, err := decoder.Token()
			if err != nil || end != json.Delim('}') {
				return fmt.Errorf("invalid qualification subject object")
			}
		case '[':
			for decoder.More() {
				if err := walk(); err != nil {
					return err
				}
			}
			end, err := decoder.Token()
			if err != nil || end != json.Delim(']') {
				return fmt.Errorf("invalid qualification subject array")
			}
		default:
			return fmt.Errorf("invalid qualification subject JSON")
		}
		return nil
	}
	if err := walk(); err != nil {
		return err
	}
	if _, err := decoder.Token(); err != io.EOF {
		return fmt.Errorf("trailing qualification subject JSON")
	}
	return nil
}

func cloneAdapterContractDTO(in AdapterContractDTO) AdapterContractDTO {
	out := in
	out.Domains = append([]string(nil), in.Domains...)
	out.RunSeeds = append([]uint64(nil), in.RunSeeds...)
	out.MembershipKinds = append([]string(nil), in.MembershipKinds...)
	out.MappedMembershipKinds = append([]string(nil), in.MappedMembershipKinds...)
	return out
}

func cloneStage6ImplementationDTO(in Stage6ImplementationDTO) Stage6ImplementationDTO {
	out := in
	out.Files = append([]ImplementationFileDTO(nil), in.Files...)
	return out
}

func cloneStage6PolicyDTO(in Stage6PolicyDTO) Stage6PolicyDTO {
	out := in
	out.SelectionOrder = append([]string(nil), in.SelectionOrder...)
	out.PredecessorOrder = append([]string(nil), in.PredecessorOrder...)
	return out
}

func cloneStage6FinalizationDTO(in Stage6FinalizationDTO) Stage6FinalizationDTO {
	out := in
	out.CompositionMappings = append([]string(nil), in.CompositionMappings...)
	out.EvidenceMappings = append([]string(nil), in.EvidenceMappings...)
	out.GroupingMappings = append([]string(nil), in.GroupingMappings...)
	out.ContextMappings = append([]string(nil), in.ContextMappings...)
	out.QualificationMappings = append([]string(nil), in.QualificationMappings...)
	out.FinalizationOrder = append([]string(nil), in.FinalizationOrder...)
	return out
}

func cloneQualificationSubjectDTO(in QualificationSubjectDTO) QualificationSubjectDTO {
	out := in
	out.AdapterContract = cloneAdapterContractDTO(in.AdapterContract)
	out.Stage6Implementation = cloneStage6ImplementationDTO(in.Stage6Implementation)
	out.ReceiptInventory = append([]ReceiptInventoryEntryDTO(nil), in.ReceiptInventory...)
	return out
}
