package programcrepresentative

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"sort"

	"lsp-trace/internal/graph"
	"lsp-trace/internal/programc"
	"lsp-trace/internal/strictjson"
)

const (
	AdapterContractSchemaVersionV2  = "lsp-trace.program-c-stage6-adapter-contract.private.v2"
	SeedGroupSchemaVersionV2        = "lsp-trace.program-c-stage6-seed-group.private.v2"
	SelectionContextSchemaVersionV2 = "lsp-trace.program-c-stage6-context.private.v2"
	MechanicsSchemaVersionV2        = "lsp-trace.program-c-stage6-mechanics.private.v2"

	MaxSeedGroupsV2          = 64
	MaxMembershipsV2         = 2048
	MaxMembershipsPerGroupV2 = 256
	MaxGroupOccurrencesV2    = 256
	MaxSumGroupOccurrencesV2 = 2048
	MaxStringBytesV2         = 1048576
)

const (
	adapterContractDomainV2 = "lsp-trace:program-c-stage6-adapter-contract:v2"
	seedGroupDomainV2       = "lsp-trace:program-c-stage6-seed-group:v2"
	contextDomainV2         = "lsp-trace:program-c-stage6-context:v2"
	mechanicsDomainV2       = "lsp-trace:program-c-stage6-mechanics:v2"
)

var adapterDomainsV2 = []string{
	adapterContractDomainV2,
	evidenceDomain,
	seedGroupDomainV2,
	contextDomainV2,
	custodyDomain,
	mechanicsDomainV2,
}

type AdapterContractDTOv2 struct {
	SchemaVersion              string   `json:"schema_version"`
	Profile                    string   `json:"profile"`
	Domains                    []string `json:"domains"`
	NativeSchemaVersion        string   `json:"native_schema_version"`
	ProgramCProfileID          string   `json:"program_c_profile_id"`
	ProgramCProfileSHA256      string   `json:"program_c_profile_sha256"`
	ProgramCAlgorithm          string   `json:"program_c_algorithm"`
	ProgramCAlgorithmVersion   string   `json:"program_c_algorithm_version"`
	RunSeeds                   []uint64 `json:"run_seeds"`
	ReplaysPerSeed             uint8    `json:"replays_per_seed"`
	RunIDGrammar               string   `json:"run_id_grammar"`
	OccurrenceOrdinalRule      string   `json:"occurrence_ordinal_rule"`
	MembershipKinds            []string `json:"membership_kinds"`
	MappedMembershipKinds      []string `json:"mapped_membership_kinds"`
	CommunityOrderRule         string   `json:"community_order_rule"`
	SeedGroupOrderRule         string   `json:"seed_group_order_rule"`
	SeedGroupKeyRule           string   `json:"seed_group_key_rule"`
	MembershipPartitionRule    string   `json:"membership_partition_rule"`
	GroupOccurrenceClosureRule string   `json:"group_occurrence_closure_rule"`
	Authority                  uint8    `json:"authority"`
	Accepted                   bool     `json:"accepted"`
	Identity                   string   `json:"identity"`
}

type SeedGroupIdentityDTOv2 struct {
	SchemaVersion         string                  `json:"schema_version"`
	ExecutionBundleID     string                  `json:"execution_bundle_id"`
	InvocationSeedOrdinal int                     `json:"invocation_seed_ordinal"`
	SeedLabel             string                  `json:"seed_label"`
	SeedAt                string                  `json:"seed_at"`
	ResolvedURI           string                  `json:"resolved_uri"`
	ContentSHA256         string                  `json:"content_sha256"`
	LanguageID            string                  `json:"language_id"`
	Memberships           []MembershipIdentityDTO `json:"memberships"`
	PreparedTargets       []string                `json:"prepared_targets"`
	ReachedNodes          []string                `json:"reached_nodes"`
	RelationIDs           []string                `json:"relation_ids"`
	OccurrenceIDs         []string                `json:"occurrence_ids"`
	Identity              string                  `json:"identity"`
}

type SelectionContextIdentityDTOv2 struct {
	SchemaVersion           string                   `json:"schema_version"`
	AdapterContractSHA256   string                   `json:"adapter_contract_sha256"`
	CustodyIdentity         string                   `json:"custody_identity"`
	ConstituentSelector     string                   `json:"constituent_selector"`
	ExecutionBundleID       string                   `json:"execution_bundle_id"`
	RunID                   string                   `json:"run_id"`
	Seed                    uint64                   `json:"seed"`
	ReplayOrdinal           uint8                    `json:"replay_ordinal"`
	CommunityOrdinal        int                      `json:"community_ordinal"`
	CommunityArtifactSHA256 string                   `json:"community_artifact_sha256"`
	CommunityMembers        []string                 `json:"community_members"`
	SeedGroups              []SeedGroupIdentityDTOv2 `json:"seed_groups"`
	Identity                string                   `json:"identity"`
}

type MechanicsIdentityDTOv2 struct {
	SchemaVersion           string                        `json:"schema_version"`
	Profile                 string                        `json:"profile"`
	AdapterContractSHA256   string                        `json:"adapter_contract_sha256"`
	CustodyIdentity         string                        `json:"custody_identity"`
	ReceiptSelector         string                        `json:"receipt_selector"`
	RunID                   string                        `json:"run_id"`
	Seed                    uint64                        `json:"seed"`
	ReplayOrdinal           uint8                         `json:"replay_ordinal"`
	CommunityOrdinal        int                           `json:"community_ordinal"`
	CommunityArtifactSHA256 string                        `json:"community_artifact_sha256"`
	Evidence                []EvidenceIdentityDTO         `json:"evidence"`
	CommunityMembers        []string                      `json:"community_members"`
	SelectionContext        SelectionContextIdentityDTOv2 `json:"selection_context"`
	Authority               uint8                         `json:"authority"`
	Accepted                bool                          `json:"accepted"`
	Identity                string                        `json:"identity"`
}

type MechanicsInputV2 struct {
	WorkflowKey                  string
	TerminalProgramCHeadSelector string
	ReceiptSelector              string
	ReceiptOrdinal               int
	ConstituentOrdinal           int
	PublicationSelector          string
	ConstituentSelector          string
	Source                       programc.SourceBinding
	ProgramCAlgorithmVersion     string
	RunID                        string
	Seed                         uint64
	ReplayOrdinal                uint8
	CommunityOrdinal             int
	CommunityArtifactSHA256      string
	CommunityMembers             []string
}

type MechanicsCommonInputV2 struct {
	WorkflowKey                  string
	TerminalProgramCHeadSelector string
	ReceiptSelector              string
	ReceiptOrdinal               int
	ConstituentOrdinal           int
	PublicationSelector          string
	ConstituentSelector          string
	Source                       programc.SourceBinding
	ProgramCAlgorithmVersion     string
}

type MechanicsContextInputV2 struct {
	RunID                   string
	Seed                    uint64
	ReplayOrdinal           uint8
	CommunityOrdinal        int
	CommunityArtifactSHA256 string
	CommunityMembers        []string
}

type MechanicsSourceCountsV2 struct {
	SeedGroups       uint64
	Memberships      uint64
	GroupOccurrences uint64
}

type preparedMechanicsValidityV2 struct {
	marker              byte
	state               *preparedMechanicsStateV2
	contractIdentity    string
	custodyIdentity     string
	receiptSelector     string
	constituentSelector string
	sourceSHA256        string
	algorithmVersion    string
	executionBundleID   string
}

type preparedMechanicsStateV2 struct {
	validity            *preparedMechanicsValidityV2
	contract            AdapterContractDTOv2
	custody             CustodyIdentityDTO
	receiptSelector     string
	constituentSelector string
	bundleID            string
	evidence            []EvidenceIdentityDTO
	seedGroups          []SeedGroupIdentityDTOv2
	sourceCounts        MechanicsSourceCountsV2
}

// PreparedMechanicsV2 is an opaque constituent-scoped immutable mechanics input.
// Its zero value and values assembled without PrepareMechanicsV2 are invalid.
type PreparedMechanicsV2 struct {
	state    *preparedMechanicsStateV2
	validity *preparedMechanicsValidityV2
}

func (p PreparedMechanicsV2) SourceCounts() MechanicsSourceCountsV2 {
	if !validPreparedMechanicsV2(p) {
		return MechanicsSourceCountsV2{}
	}
	return p.state.sourceCounts
}

// MechanicsV2 is an immutable private prequalification carrier. Its zero value is invalid.
type MechanicsV2 struct {
	dto       MechanicsIdentityDTOv2
	canonical []byte
	trusted   bool
}

func NewMechanicsV2(in MechanicsInputV2) (MechanicsV2, error) {
	sourceBytes := in.Source.InputBytes()
	graphV5Bytes := in.Source.GraphV5Bytes()
	if in.WorkflowKey == "" || !validDigest(in.TerminalProgramCHeadSelector) || !validDigest(in.ReceiptSelector) || in.ReceiptOrdinal <= 0 || in.ConstituentOrdinal < 0 || in.PublicationSelector == "" || in.ConstituentSelector == "" || !validProgramCSourceBinding(in.Source, sourceBytes, graphV5Bytes) || in.ProgramCAlgorithmVersion == "" || in.CommunityOrdinal < 0 || !validDigest(in.CommunityArtifactSHA256) || !strictStrings(in.CommunityMembers) || !validateStringCapV2(in) {
		return MechanicsV2{}, fmt.Errorf("invalid v2 mechanics input")
	}
	if in.ReplayOrdinal < 1 || in.ReplayOrdinal > 3 || (in.Seed != 1 && in.Seed != 2) || in.RunID != fmt.Sprintf("seed-%020d-replay-%d", in.Seed, in.ReplayOrdinal) {
		return MechanicsV2{}, fmt.Errorf("invalid v2 mechanics run identity")
	}
	prepared, err := prepareMechanicsV2(mechanicsCommonInputV2(in), sourceBytes, graphV5Bytes)
	if err != nil {
		return MechanicsV2{}, err
	}
	return constructPreparedMechanicsV2(prepared, mechanicsContextInputV2(in))
}

func PrepareMechanicsV2(in MechanicsCommonInputV2) (PreparedMechanicsV2, error) {
	sourceBytes := in.Source.InputBytes()
	graphV5Bytes := in.Source.GraphV5Bytes()
	if !validMechanicsCommonInputV2(in, sourceBytes, graphV5Bytes) {
		return PreparedMechanicsV2{}, fmt.Errorf("invalid v2 mechanics input")
	}
	return prepareMechanicsV2(in, sourceBytes, graphV5Bytes)
}

func NewPreparedMechanicsV2(prepared PreparedMechanicsV2, in MechanicsContextInputV2) (MechanicsV2, error) {
	if !validPreparedMechanicsV2(prepared) {
		return MechanicsV2{}, fmt.Errorf("invalid prepared v2 mechanics")
	}
	if err := validateMechanicsContextInputV2(in); err != nil {
		return MechanicsV2{}, err
	}
	return constructPreparedMechanicsV2(prepared, in)
}

func NewPreparedMechanicsBatchV2(prepared PreparedMechanicsV2, inputs []MechanicsContextInputV2) ([]MechanicsV2, error) {
	if !validPreparedMechanicsV2(prepared) {
		return nil, fmt.Errorf("invalid prepared v2 mechanics")
	}
	for _, in := range inputs {
		if err := validateMechanicsContextInputV2(in); err != nil {
			return nil, err
		}
	}
	out := make([]MechanicsV2, len(inputs))
	for i, in := range inputs {
		item, err := constructPreparedMechanicsV2(prepared, in)
		if err != nil {
			return nil, err
		}
		out[i] = item
	}
	return out, nil
}

func validMechanicsCommonInputV2(in MechanicsCommonInputV2, sourceBytes, graphV5Bytes []byte) bool {
	return in.WorkflowKey != "" && validDigest(in.TerminalProgramCHeadSelector) && validDigest(in.ReceiptSelector) && in.ReceiptOrdinal > 0 && in.ConstituentOrdinal >= 0 && in.PublicationSelector != "" && in.ConstituentSelector != "" && validProgramCSourceBinding(in.Source, sourceBytes, graphV5Bytes) && in.ProgramCAlgorithmVersion != "" && validateStringCapV2(in)
}

func validateMechanicsContextInputV2(in MechanicsContextInputV2) error {
	if in.CommunityOrdinal < 0 || !validDigest(in.CommunityArtifactSHA256) || !strictStrings(in.CommunityMembers) || !validateStringCapV2(in) {
		return fmt.Errorf("invalid v2 mechanics input")
	}
	if in.ReplayOrdinal < 1 || in.ReplayOrdinal > 3 || (in.Seed != 1 && in.Seed != 2) || in.RunID != fmt.Sprintf("seed-%020d-replay-%d", in.Seed, in.ReplayOrdinal) {
		return fmt.Errorf("invalid v2 mechanics run identity")
	}
	return nil
}

func prepareMechanicsV2(in MechanicsCommonInputV2, sourceBytes, graphV5Bytes []byte) (PreparedMechanicsV2, error) {
	view, err := graph.NewNativeV5RepresentativeView(graphV5Bytes)
	if err != nil || view.ExecutionBundleID() != in.Source.ExecutionBundleID {
		return PreparedMechanicsV2{}, fmt.Errorf("invalid v2 mechanics source binding")
	}
	contract, err := newAdapterContractV2(in.ProgramCAlgorithmVersion)
	if err != nil {
		return PreparedMechanicsV2{}, err
	}
	bundleID := view.ExecutionBundleID()
	custody := CustodyIdentityDTO{
		SchemaVersion: CustodySchemaVersion, WorkflowKey: in.WorkflowKey,
		TerminalProgramCHeadSelector: in.TerminalProgramCHeadSelector,
		ReceiptSelector:              in.ReceiptSelector, ReceiptOrdinal: in.ReceiptOrdinal,
		ConstituentOrdinal: in.ConstituentOrdinal, PublicationSelector: in.PublicationSelector,
		ConstituentSelector: in.ConstituentSelector, NativeV5SHA256: rawDigest(sourceBytes),
		NativeV5ByteLength: uint64(len(sourceBytes)), ExecutionBundleID: bundleID,
	}
	custody.Identity, err = hashIdentity(custodyDomain, custody)
	if err != nil {
		return PreparedMechanicsV2{}, err
	}
	evidence, err := buildEvidence(view, in.ConstituentSelector, in.ReceiptSelector)
	if err != nil {
		return PreparedMechanicsV2{}, err
	}
	groups, err := buildSeedGroupsV2(view, evidence)
	if err != nil {
		return PreparedMechanicsV2{}, err
	}
	counts := MechanicsSourceCountsV2{SeedGroups: uint64(len(groups))}
	for _, group := range groups {
		counts.Memberships += uint64(len(group.Memberships))
		counts.GroupOccurrences += uint64(len(group.OccurrenceIDs))
	}
	validity := &preparedMechanicsValidityV2{
		marker: 1, contractIdentity: contract.Identity, custodyIdentity: custody.Identity,
		receiptSelector: in.ReceiptSelector, constituentSelector: in.ConstituentSelector,
		sourceSHA256: custody.NativeV5SHA256, algorithmVersion: contract.ProgramCAlgorithmVersion,
		executionBundleID: bundleID,
	}
	state := &preparedMechanicsStateV2{
		validity: validity, contract: contract, custody: custody,
		receiptSelector: in.ReceiptSelector, constituentSelector: in.ConstituentSelector,
		bundleID: bundleID, evidence: append([]EvidenceIdentityDTO{}, evidence...),
		seedGroups: cloneSeedGroupsV2(groups), sourceCounts: counts,
	}
	validity.state = state
	return PreparedMechanicsV2{state: state, validity: validity}, nil
}

func validPreparedMechanicsV2(prepared PreparedMechanicsV2) bool {
	state, validity := prepared.state, prepared.validity
	return state != nil && validity != nil && validity.marker == 1 && state.validity == validity && validity.state == state &&
		validity.contractIdentity == state.contract.Identity && validity.custodyIdentity == state.custody.Identity &&
		validity.receiptSelector == state.receiptSelector && validity.constituentSelector == state.constituentSelector &&
		validity.sourceSHA256 == state.custody.NativeV5SHA256 && validity.algorithmVersion == state.contract.ProgramCAlgorithmVersion &&
		validity.executionBundleID == state.bundleID && validDigest(state.contract.Identity) && validDigest(state.custody.Identity) &&
		validDigest(state.receiptSelector) && state.constituentSelector != "" && state.bundleID != "" && state.evidence != nil &&
		state.seedGroups != nil && len(state.seedGroups) > 0 && state.sourceCounts.SeedGroups == uint64(len(state.seedGroups))
}

func constructPreparedMechanicsV2(prepared PreparedMechanicsV2, in MechanicsContextInputV2) (MechanicsV2, error) {
	state := prepared.state
	context := SelectionContextIdentityDTOv2{
		SchemaVersion: SelectionContextSchemaVersionV2, AdapterContractSHA256: state.contract.Identity,
		CustodyIdentity: state.custody.Identity, ConstituentSelector: state.constituentSelector,
		ExecutionBundleID: state.bundleID, RunID: in.RunID, Seed: in.Seed,
		ReplayOrdinal: in.ReplayOrdinal, CommunityOrdinal: in.CommunityOrdinal,
		CommunityArtifactSHA256: in.CommunityArtifactSHA256,
		CommunityMembers:        append([]string{}, in.CommunityMembers...),
		SeedGroups:              cloneSeedGroupsV2(state.seedGroups),
	}
	var err error
	context.Identity, err = hashIdentity(contextDomainV2, context)
	if err != nil {
		return MechanicsV2{}, err
	}
	dto := MechanicsIdentityDTOv2{
		SchemaVersion: MechanicsSchemaVersionV2, Profile: ProfileCallsOnlyFrozenCensusV1,
		AdapterContractSHA256: state.contract.Identity, CustodyIdentity: state.custody.Identity,
		ReceiptSelector: state.receiptSelector, RunID: in.RunID, Seed: in.Seed,
		ReplayOrdinal: in.ReplayOrdinal, CommunityOrdinal: in.CommunityOrdinal,
		CommunityArtifactSHA256: in.CommunityArtifactSHA256,
		Evidence:                append([]EvidenceIdentityDTO{}, state.evidence...),
		CommunityMembers:        append([]string{}, in.CommunityMembers...),
		SelectionContext:        context, Authority: 0, Accepted: false,
	}
	dto.Identity, err = hashIdentity(mechanicsDomainV2, dto)
	if err != nil {
		return MechanicsV2{}, err
	}
	if err := validateMechanicsEnvelopeV2(dto); err != nil {
		return MechanicsV2{}, err
	}
	canonical, err := json.Marshal(dto)
	if err != nil {
		return MechanicsV2{}, err
	}
	return MechanicsV2{dto: cloneMechanicsDTOv2(dto), canonical: append([]byte{}, canonical...), trusted: true}, nil
}

func mechanicsCommonInputV2(in MechanicsInputV2) MechanicsCommonInputV2 {
	return MechanicsCommonInputV2{
		WorkflowKey: in.WorkflowKey, TerminalProgramCHeadSelector: in.TerminalProgramCHeadSelector,
		ReceiptSelector: in.ReceiptSelector, ReceiptOrdinal: in.ReceiptOrdinal,
		ConstituentOrdinal: in.ConstituentOrdinal, PublicationSelector: in.PublicationSelector,
		ConstituentSelector: in.ConstituentSelector, Source: in.Source,
		ProgramCAlgorithmVersion: in.ProgramCAlgorithmVersion,
	}
}

func mechanicsContextInputV2(in MechanicsInputV2) MechanicsContextInputV2 {
	return MechanicsContextInputV2{
		RunID: in.RunID, Seed: in.Seed, ReplayOrdinal: in.ReplayOrdinal,
		CommunityOrdinal: in.CommunityOrdinal, CommunityArtifactSHA256: in.CommunityArtifactSHA256,
		CommunityMembers: append([]string{}, in.CommunityMembers...),
	}
}

func (m MechanicsV2) Valid() bool {
	return m.trusted && validateMechanicsEnvelopeV2(m.dto) == nil && len(m.canonical) > 0
}
func (m MechanicsV2) Trusted() bool               { return m.Valid() }
func (m MechanicsV2) Identity() string            { return m.dto.Identity }
func (m MechanicsV2) DTO() MechanicsIdentityDTOv2 { return cloneMechanicsDTOv2(m.dto) }
func (m MechanicsV2) CanonicalBytes() []byte      { return append([]byte{}, m.canonical...) }
func (m MechanicsV2) Evidence() []EvidenceIdentityDTO {
	return append([]EvidenceIdentityDTO{}, m.dto.Evidence...)
}
func (m MechanicsV2) CommunityMembers() []string {
	return append([]string{}, m.dto.CommunityMembers...)
}
func (m MechanicsV2) SelectionContext() SelectionContextIdentityDTOv2 {
	return cloneSelectionContextV2(m.dto.SelectionContext)
}

func ParseMechanicsV2(raw []byte) (MechanicsV2, error) {
	if err := strictjson.RejectDuplicates(raw); err != nil {
		return MechanicsV2{}, err
	}
	var dto MechanicsIdentityDTOv2
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&dto); err != nil {
		return MechanicsV2{}, err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return MechanicsV2{}, fmt.Errorf("trailing v2 mechanics bytes")
	}
	if err := validateMechanicsEnvelopeV2(dto); err != nil {
		return MechanicsV2{}, err
	}
	canonical, err := json.Marshal(dto)
	if err != nil || !bytes.Equal(canonical, raw) {
		return MechanicsV2{}, fmt.Errorf("noncanonical v2 mechanics bytes")
	}
	return MechanicsV2{dto: cloneMechanicsDTOv2(dto), canonical: append([]byte{}, raw...)}, nil
}

// ValidateMechanicsDTOv2 establishes trust only by exact contextual reconstruction.
func ValidateMechanicsDTOv2(dto MechanicsIdentityDTOv2, expected MechanicsInputV2) error {
	constructed, err := NewMechanicsV2(expected)
	if err != nil {
		return err
	}
	canonical, err := json.Marshal(dto)
	if err != nil || !bytes.Equal(canonical, constructed.canonical) {
		return fmt.Errorf("v2 mechanics DTO does not match reconstructed context")
	}
	return nil
}

func newAdapterContractV2(algorithmVersion string) (AdapterContractDTOv2, error) {
	if algorithmVersion == "" || len(algorithmVersion) > MaxStringBytesV2 {
		return AdapterContractDTOv2{}, fmt.Errorf("missing Program C algorithm version")
	}
	dto := AdapterContractDTOv2{
		SchemaVersion: AdapterContractSchemaVersionV2, Profile: ProfileCallsOnlyFrozenCensusV1,
		Domains: append([]string{}, adapterDomainsV2...), NativeSchemaVersion: graph.SchemaVersionV5,
		ProgramCProfileID: "calls-v1", ProgramCProfileSHA256: "sha256:f4fb309c6e849b5a8e6057f355b1db6ee3c53414c3c430b67be17f68fe9e97f9",
		ProgramCAlgorithm: "gonum.org/v1/gonum/graph/community.Leiden", ProgramCAlgorithmVersion: algorithmVersion,
		RunSeeds: []uint64{1, 2}, ReplaysPerSeed: 3, RunIDGrammar: "seed-%020d-replay-%d",
		OccurrenceOrdinalRule: "ZERO_BASED_WITHIN_CANONICAL_EDGE_CALL_SITES",
		MembershipKinds:       append([]string{}, membershipKinds...), MappedMembershipKinds: append([]string{}, mappedMembershipKinds...),
		CommunityOrderRule:         "PROGRAM_C_VALID_CANONICAL_COMMUNITIES",
		SeedGroupOrderRule:         "NATIVE_V5_INVOCATION_SEEDS_ARRAY_ORDER",
		SeedGroupKeyRule:           "EXECUTION_BUNDLE_ID_SEED_LABEL_SEED_AT",
		MembershipPartitionRule:    "ORDERED_CARDINALITY_VALUE_PRESERVING_BIJECTION",
		GroupOccurrenceClosureRule: "EXACT_ALL_CALLS_OCCURRENCES_FOR_GROUP_RELATION_IDS",
		Authority:                  0, Accepted: false,
	}
	identity, err := hashIdentity(adapterContractDomainV2, dto)
	if err != nil {
		return AdapterContractDTOv2{}, err
	}
	dto.Identity = identity
	return dto, nil
}

type seedGroupKeyV2 struct {
	bundle string
	label  string
	at     string
}

func buildSeedGroupsV2(view graph.NativeV5RepresentativeView, evidence []EvidenceIdentityDTO) ([]SeedGroupIdentityDTOv2, error) {
	seeds := view.InvocationSeeds()
	memberships := view.SeedMemberships()
	if len(seeds) == 0 || len(memberships) == 0 || validateSourceLimitsV2(len(seeds), len(memberships), 0, 0, 0) != nil {
		return nil, fmt.Errorf("invalid v2 seed-group cardinality")
	}
	nodes := stringSet(view.NodeIDs())
	edges := map[string]graph.Edge{}
	for _, edge := range view.Edges() {
		if edge.RelationID == "" || edge.ExecutionBundleID != view.ExecutionBundleID() || edges[edge.RelationID].RelationID != "" {
			return nil, fmt.Errorf("invalid v2 CALLS edge")
		}
		edges[edge.RelationID] = edge
	}
	evidenceByRelation := map[string][]string{}
	for _, item := range evidence {
		evidenceByRelation[item.RelationID] = append(evidenceByRelation[item.RelationID], item.Identity)
	}
	groups := make([]SeedGroupIdentityDTOv2, len(seeds))
	indexes := make(map[seedGroupKeyV2]int, len(seeds))
	for i, seed := range seeds {
		key := seedGroupKeyV2{view.ExecutionBundleID(), seed.Label, seed.At}
		if seed.Label == "" || seed.At == "" || !validateStringCapV2(seed) {
			return nil, fmt.Errorf("invalid v2 invocation seed")
		}
		if _, duplicate := indexes[key]; duplicate {
			return nil, fmt.Errorf("duplicate v2 invocation seed")
		}
		indexes[key] = i
		groups[i] = SeedGroupIdentityDTOv2{
			SchemaVersion: SeedGroupSchemaVersionV2, ExecutionBundleID: view.ExecutionBundleID(),
			InvocationSeedOrdinal: i, SeedLabel: seed.Label, SeedAt: seed.At,
			ResolvedURI: seed.ResolvedURI, ContentSHA256: seed.ContentSHA256, LanguageID: seed.LanguageID,
			Memberships: []MembershipIdentityDTO{}, PreparedTargets: []string{}, ReachedNodes: []string{}, RelationIDs: []string{}, OccurrenceIDs: []string{},
		}
	}
	seenMembership := map[string]bool{}
	for i, membership := range memberships {
		key := seedGroupKeyV2{membership.ExecutionBundleID, membership.SeedLabel, membership.SeedAt}
		groupIndex, ok := indexes[key]
		if !ok || membership.MembershipID == "" || membership.EndpointID == "" || !contains(membershipKinds, membership.EvidenceKind) || seenMembership[membership.MembershipID] || (i > 0 && memberships[i-1].MembershipID >= membership.MembershipID) || !validateStringCapV2(membership) {
			return nil, fmt.Errorf("non-bijective v2 seed membership")
		}
		seenMembership[membership.MembershipID] = true
		group := &groups[groupIndex]
		group.Memberships = append(group.Memberships, MembershipIdentityDTO{MembershipID: membership.MembershipID, ExecutionBundleID: membership.ExecutionBundleID, SeedLabel: membership.SeedLabel, SeedAt: membership.SeedAt, EvidenceKind: membership.EvidenceKind, EndpointID: membership.EndpointID})
		switch membership.EvidenceKind {
		case "PREPARED_TARGET":
			if !nodes[membership.EndpointID] {
				return nil, fmt.Errorf("unknown v2 prepared target")
			}
			group.PreparedTargets = append(group.PreparedTargets, membership.EndpointID)
		case "REACHED_NODE":
			if !nodes[membership.EndpointID] {
				return nil, fmt.Errorf("unknown v2 reached node")
			}
			group.ReachedNodes = append(group.ReachedNodes, membership.EndpointID)
		case "CALL_RELATION":
			if edges[membership.EndpointID].RelationID == "" {
				return nil, fmt.Errorf("unknown v2 CALLS relation")
			}
			group.RelationIDs = append(group.RelationIDs, membership.EndpointID)
		case "SIBLING_CANDIDATE", "DISPATCH_ASSOCIATION":
		}
	}
	var maxMemberships, maxOccurrences, sumOccurrences int
	for i := range groups {
		group := &groups[i]
		if len(group.Memberships) == 0 {
			return nil, fmt.Errorf("invalid v2 group membership cardinality")
		}
		if len(group.Memberships) > maxMemberships {
			maxMemberships = len(group.Memberships)
		}
		var err error
		if group.PreparedTargets, err = strictSortedUniqueV2(group.PreparedTargets); err != nil {
			return nil, err
		}
		if group.ReachedNodes, err = strictSortedUniqueV2(group.ReachedNodes); err != nil {
			return nil, err
		}
		if group.RelationIDs, err = strictSortedUniqueV2(group.RelationIDs); err != nil {
			return nil, err
		}
		for _, relationID := range group.RelationIDs {
			ids := evidenceByRelation[relationID]
			if len(ids) != len(edges[relationID].CallSites) {
				return nil, fmt.Errorf("partial v2 group occurrence closure")
			}
			group.OccurrenceIDs = append(group.OccurrenceIDs, ids...)
		}
		if group.OccurrenceIDs, err = strictSortedUniqueV2(group.OccurrenceIDs); err != nil {
			return nil, err
		}
		if len(group.OccurrenceIDs) > maxOccurrences {
			maxOccurrences = len(group.OccurrenceIDs)
		}
		if sumOccurrences > MaxSumGroupOccurrencesV2-len(group.OccurrenceIDs) {
			return nil, fmt.Errorf("v2 group occurrence limit")
		}
		sumOccurrences += len(group.OccurrenceIDs)
		group.Identity, err = hashIdentity(seedGroupDomainV2, *group)
		if err != nil {
			return nil, err
		}
	}
	if err := validateSourceLimitsV2(len(groups), len(memberships), maxMemberships, maxOccurrences, sumOccurrences); err != nil {
		return nil, err
	}
	return groups, nil
}

func validateSourceLimitsV2(seedGroups, memberships, maxMembershipsPerGroup, maxGroupOccurrences, sumGroupOccurrences int) error {
	if seedGroups > MaxSeedGroupsV2 || memberships > MaxMembershipsV2 || maxMembershipsPerGroup > MaxMembershipsPerGroupV2 || maxGroupOccurrences > MaxGroupOccurrencesV2 || sumGroupOccurrences > MaxSumGroupOccurrencesV2 {
		return fmt.Errorf("v2 source limit exceeded")
	}
	return nil
}

func validateMechanicsEnvelopeV2(dto MechanicsIdentityDTOv2) error {
	if dto.SchemaVersion != MechanicsSchemaVersionV2 || dto.Profile != ProfileCallsOnlyFrozenCensusV1 || dto.Authority != 0 || dto.Accepted || !validDigest(dto.AdapterContractSHA256) || !validDigest(dto.CustodyIdentity) || !validDigest(dto.ReceiptSelector) || dto.RunID == "" || dto.ReplayOrdinal < 1 || dto.ReplayOrdinal > 3 || dto.CommunityOrdinal < 0 || !validDigest(dto.CommunityArtifactSHA256) || dto.Evidence == nil || !strictStrings(dto.CommunityMembers) || !validDigest(dto.Identity) || !validateStringCapV2(dto) {
		return fmt.Errorf("invalid v2 mechanics DTO")
	}
	seenEvidence := map[string]bool{}
	for _, evidence := range dto.Evidence {
		if evidence.SchemaVersion != EvidenceSchemaVersion || evidence.ConstituentSelector == "" || evidence.ExecutionBundleID == "" || evidence.RelationID == "" || evidence.CallSiteOrdinal < 0 || evidence.FromNodeID == "" || evidence.ToNodeID == "" || !validDigest(evidence.ReceiptSelector) || !validDigest(evidence.Identity) || seenEvidence[evidence.Identity] {
			return fmt.Errorf("invalid v2 mechanics evidence")
		}
		if graph.ValidateRange(graph.Range{Start: graph.Position{Line: evidence.CallSite.Start.Line, Character: evidence.CallSite.Start.Character}, End: graph.Position{Line: evidence.CallSite.End.Line, Character: evidence.CallSite.End.Character}}) != nil {
			return fmt.Errorf("invalid v2 mechanics call site")
		}
		copy := evidence
		copy.Identity = ""
		identity, err := hashIdentity(evidenceDomain, copy)
		if err != nil || identity != evidence.Identity {
			return fmt.Errorf("invalid v2 evidence identity")
		}
		seenEvidence[evidence.Identity] = true
	}
	context := dto.SelectionContext
	if context.SchemaVersion != SelectionContextSchemaVersionV2 || context.AdapterContractSHA256 != dto.AdapterContractSHA256 || context.CustodyIdentity != dto.CustodyIdentity || context.RunID != dto.RunID || context.Seed != dto.Seed || context.ReplayOrdinal != dto.ReplayOrdinal || context.CommunityOrdinal != dto.CommunityOrdinal || context.CommunityArtifactSHA256 != dto.CommunityArtifactSHA256 || !equalMechanicsStrings(context.CommunityMembers, dto.CommunityMembers) || context.SeedGroups == nil || len(context.SeedGroups) == 0 || len(context.SeedGroups) > MaxSeedGroupsV2 || !validDigest(context.Identity) {
		return fmt.Errorf("invalid v2 selection context")
	}
	seenKeys := map[seedGroupKeyV2]bool{}
	seenMemberships := map[string]bool{}
	totalMemberships, maxMemberships, maxOccurrences, totalOccurrences := 0, 0, 0, 0
	for i, group := range context.SeedGroups {
		key := seedGroupKeyV2{group.ExecutionBundleID, group.SeedLabel, group.SeedAt}
		if group.SchemaVersion != SeedGroupSchemaVersionV2 || group.ExecutionBundleID != context.ExecutionBundleID || group.InvocationSeedOrdinal != i || group.SeedLabel == "" || group.SeedAt == "" || seenKeys[key] || group.Memberships == nil || len(group.Memberships) == 0 || len(group.Memberships) > MaxMembershipsPerGroupV2 || group.PreparedTargets == nil || group.ReachedNodes == nil || group.RelationIDs == nil || group.OccurrenceIDs == nil || len(group.OccurrenceIDs) > MaxGroupOccurrencesV2 || !strictMemberships(group.Memberships) || !strictOptionalStrings(group.PreparedTargets) || !strictOptionalStrings(group.ReachedNodes) || !strictOptionalStrings(group.RelationIDs) || !strictOptionalStrings(group.OccurrenceIDs) || !validDigest(group.Identity) {
			return fmt.Errorf("invalid v2 seed group")
		}
		seenKeys[key] = true
		for _, membership := range group.Memberships {
			if membership.ExecutionBundleID != group.ExecutionBundleID || membership.SeedLabel != group.SeedLabel || membership.SeedAt != group.SeedAt || seenMemberships[membership.MembershipID] {
				return fmt.Errorf("invalid v2 seed-group membership")
			}
			seenMemberships[membership.MembershipID] = true
		}
		totalMemberships += len(group.Memberships)
		totalOccurrences += len(group.OccurrenceIDs)
		if len(group.Memberships) > maxMemberships {
			maxMemberships = len(group.Memberships)
		}
		if len(group.OccurrenceIDs) > maxOccurrences {
			maxOccurrences = len(group.OccurrenceIDs)
		}
		groupCopy := cloneSeedGroupDTOv2(group)
		groupCopy.Identity = ""
		identity, err := hashIdentity(seedGroupDomainV2, groupCopy)
		if err != nil || identity != group.Identity {
			return fmt.Errorf("invalid v2 seed-group identity")
		}
	}
	if err := validateSourceLimitsV2(len(context.SeedGroups), totalMemberships, maxMemberships, maxOccurrences, totalOccurrences); err != nil {
		return fmt.Errorf("invalid v2 source limits: %w", err)
	}
	contextCopy := cloneSelectionContextV2(context)
	contextCopy.Identity = ""
	contextIdentity, err := hashIdentity(contextDomainV2, contextCopy)
	if err != nil || contextIdentity != context.Identity {
		return fmt.Errorf("invalid v2 selection context identity")
	}
	copy := cloneMechanicsDTOv2(dto)
	copy.Identity = ""
	identity, err := hashIdentity(mechanicsDomainV2, copy)
	if err != nil || identity != dto.Identity {
		return fmt.Errorf("invalid v2 mechanics identity")
	}
	return nil
}

func strictSortedUniqueV2(in []string) ([]string, error) {
	out := append([]string{}, in...)
	sort.Strings(out)
	for i, value := range out {
		if value == "" || (i > 0 && out[i-1] == value) {
			return nil, fmt.Errorf("duplicate v2 group-local identity")
		}
	}
	return out, nil
}

func validateStringCapV2(value any) bool {
	return stringsWithinV2(reflect.ValueOf(value))
}

func stringsWithinV2(value reflect.Value) bool {
	if !value.IsValid() {
		return true
	}
	switch value.Kind() {
	case reflect.Interface, reflect.Pointer:
		if value.IsNil() {
			return true
		}
		return stringsWithinV2(value.Elem())
	case reflect.String:
		return value.Len() <= MaxStringBytesV2
	case reflect.Struct:
		for i := 0; i < value.NumField(); i++ {
			if !stringsWithinV2(value.Field(i)) {
				return false
			}
		}
	case reflect.Slice, reflect.Array:
		for i := 0; i < value.Len(); i++ {
			if !stringsWithinV2(value.Index(i)) {
				return false
			}
		}
	}
	return true
}

func cloneSeedGroupDTOv2(in SeedGroupIdentityDTOv2) SeedGroupIdentityDTOv2 {
	out := in
	out.Memberships = append([]MembershipIdentityDTO{}, in.Memberships...)
	out.PreparedTargets = append([]string{}, in.PreparedTargets...)
	out.ReachedNodes = append([]string{}, in.ReachedNodes...)
	out.RelationIDs = append([]string{}, in.RelationIDs...)
	out.OccurrenceIDs = append([]string{}, in.OccurrenceIDs...)
	return out
}
func cloneSeedGroupsV2(in []SeedGroupIdentityDTOv2) []SeedGroupIdentityDTOv2 {
	out := make([]SeedGroupIdentityDTOv2, len(in))
	for i := range in {
		out[i] = cloneSeedGroupDTOv2(in[i])
	}
	return out
}
func cloneSelectionContextV2(in SelectionContextIdentityDTOv2) SelectionContextIdentityDTOv2 {
	out := in
	out.CommunityMembers = append([]string{}, in.CommunityMembers...)
	out.SeedGroups = cloneSeedGroupsV2(in.SeedGroups)
	return out
}
func cloneMechanicsDTOv2(in MechanicsIdentityDTOv2) MechanicsIdentityDTOv2 {
	out := in
	out.Evidence = append([]EvidenceIdentityDTO{}, in.Evidence...)
	out.CommunityMembers = append([]string{}, in.CommunityMembers...)
	out.SelectionContext = cloneSelectionContextV2(in.SelectionContext)
	return out
}
