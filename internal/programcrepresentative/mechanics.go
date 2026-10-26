package programcrepresentative

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"

	"lsp-trace/internal/graph"
	"lsp-trace/internal/programc"
)

const (
	AdapterContractSchemaVersion   = "lsp-trace.program-c-stage6-adapter-contract.private.v1"
	EvidenceSchemaVersion          = "lsp-trace.program-c-stage6-evidence.private.v1"
	CustodySchemaVersion           = "lsp-trace.program-c-stage6-custody.private.v1"
	SelectionContextSchemaVersion  = "lsp-trace.program-c-stage6-context.private.v1"
	MechanicsSchemaVersion         = "lsp-trace.program-c-stage6-mechanics.private.v1"
	ProfileCallsOnlyFrozenCensusV1 = "CALLS_ONLY_FROZEN_CENSUS_V1"

	adapterContractDomain = "lsp-trace:program-c-stage6-adapter-contract:v1"
	evidenceDomain        = "lsp-trace:program-c-stage6-evidence:v1"
	contextDomain         = "lsp-trace:program-c-stage6-context:v1"
	custodyDomain         = "lsp-trace:program-c-stage6-custody:v1"
	mechanicsDomain       = "lsp-trace:program-c-stage6-mechanics:v1"
)

var (
	adapterDomains        = []string{adapterContractDomain, evidenceDomain, contextDomain, custodyDomain, mechanicsDomain}
	membershipKinds       = []string{"PREPARED_TARGET", "REACHED_NODE", "CALL_RELATION", "SIBLING_CANDIDATE", "DISPATCH_ASSOCIATION"}
	mappedMembershipKinds = []string{"PREPARED_TARGET", "REACHED_NODE", "CALL_RELATION"}
)

type PositionDTO struct {
	Line      uint32 `json:"line"`
	Character uint32 `json:"character"`
}

type RangeDTO struct {
	Start PositionDTO `json:"start"`
	End   PositionDTO `json:"end"`
}

type AdapterContractDTO struct {
	SchemaVersion            string   `json:"schema_version"`
	Profile                  string   `json:"profile"`
	Domains                  []string `json:"domains"`
	NativeSchemaVersion      string   `json:"native_schema_version"`
	ProgramCProfileID        string   `json:"program_c_profile_id"`
	ProgramCProfileSHA256    string   `json:"program_c_profile_sha256"`
	ProgramCAlgorithm        string   `json:"program_c_algorithm"`
	ProgramCAlgorithmVersion string   `json:"program_c_algorithm_version"`
	RunSeeds                 []uint64 `json:"run_seeds"`
	ReplaysPerSeed           uint8    `json:"replays_per_seed"`
	RunIDGrammar             string   `json:"run_id_grammar"`
	OccurrenceOrdinalRule    string   `json:"occurrence_ordinal_rule"`
	MembershipKinds          []string `json:"membership_kinds"`
	MappedMembershipKinds    []string `json:"mapped_membership_kinds"`
	CommunityOrderRule       string   `json:"community_order_rule"`
	Authority                uint8    `json:"authority"`
	Accepted                 bool     `json:"accepted"`
	Identity                 string   `json:"identity"`
}

type EvidenceIdentityDTO struct {
	SchemaVersion       string   `json:"schema_version"`
	ConstituentSelector string   `json:"constituent_selector"`
	ExecutionBundleID   string   `json:"execution_bundle_id"`
	RelationID          string   `json:"relation_id"`
	CallSiteOrdinal     int      `json:"call_site_ordinal"`
	FromNodeID          string   `json:"from_node_id"`
	ToNodeID            string   `json:"to_node_id"`
	CallSite            RangeDTO `json:"call_site"`
	ReceiptSelector     string   `json:"receipt_selector"`
	Identity            string   `json:"identity"`
}

type CustodyIdentityDTO struct {
	SchemaVersion                string `json:"schema_version"`
	WorkflowKey                  string `json:"workflow_key"`
	TerminalProgramCHeadSelector string `json:"terminal_program_c_head_selector"`
	ReceiptSelector              string `json:"receipt_selector"`
	ReceiptOrdinal               int    `json:"receipt_ordinal"`
	ConstituentOrdinal           int    `json:"constituent_ordinal"`
	PublicationSelector          string `json:"publication_selector"`
	ConstituentSelector          string `json:"constituent_selector"`
	NativeV5SHA256               string `json:"native_v5_sha256"`
	NativeV5ByteLength           uint64 `json:"native_v5_byte_length"`
	ExecutionBundleID            string `json:"execution_bundle_id"`
	Identity                     string `json:"identity"`
}

type MembershipIdentityDTO struct {
	MembershipID      string `json:"membership_id"`
	ExecutionBundleID string `json:"execution_bundle_id"`
	SeedLabel         string `json:"seed_label"`
	SeedAt            string `json:"seed_at"`
	EvidenceKind      string `json:"evidence_kind"`
	EndpointID        string `json:"endpoint_id"`
}

type SelectionContextIdentityDTO struct {
	SchemaVersion           string                  `json:"schema_version"`
	AdapterContractSHA256   string                  `json:"adapter_contract_sha256"`
	CustodyIdentity         string                  `json:"custody_identity"`
	ConstituentSelector     string                  `json:"constituent_selector"`
	ExecutionBundleID       string                  `json:"execution_bundle_id"`
	RunID                   string                  `json:"run_id"`
	Seed                    uint64                  `json:"seed"`
	ReplayOrdinal           uint8                   `json:"replay_ordinal"`
	CommunityOrdinal        int                     `json:"community_ordinal"`
	CommunityArtifactSHA256 string                  `json:"community_artifact_sha256"`
	CommunityMembers        []string                `json:"community_members"`
	Memberships             []MembershipIdentityDTO `json:"memberships"`
	PreparedTargets         []string                `json:"prepared_targets"`
	ReachedNodes            []string                `json:"reached_nodes"`
	RelationIDs             []string                `json:"relation_ids"`
	OccurrenceIDs           []string                `json:"occurrence_ids"`
	Identity                string                  `json:"identity"`
}

type MechanicsIdentityDTO struct {
	SchemaVersion           string                      `json:"schema_version"`
	Profile                 string                      `json:"profile"`
	AdapterContractSHA256   string                      `json:"adapter_contract_sha256"`
	CustodyIdentity         string                      `json:"custody_identity"`
	ReceiptSelector         string                      `json:"receipt_selector"`
	RunID                   string                      `json:"run_id"`
	Seed                    uint64                      `json:"seed"`
	ReplayOrdinal           uint8                       `json:"replay_ordinal"`
	CommunityOrdinal        int                         `json:"community_ordinal"`
	CommunityArtifactSHA256 string                      `json:"community_artifact_sha256"`
	Evidence                []EvidenceIdentityDTO       `json:"evidence"`
	CommunityMembers        []string                    `json:"community_members"`
	SelectionContext        SelectionContextIdentityDTO `json:"selection_context"`
	Authority               uint8                       `json:"authority"`
	Accepted                bool                        `json:"accepted"`
	Identity                string                      `json:"identity"`
}

type SingletonMechanicsInput struct {
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

// Mechanics is an immutable prequalification carrier. Its zero value is invalid.
type Mechanics struct {
	dto       MechanicsIdentityDTO
	canonical []byte
	trusted   bool
}

func NewSingletonMechanics(in SingletonMechanicsInput) (Mechanics, error) {
	sourceBytes := in.Source.InputBytes()
	graphV5Bytes := in.Source.GraphV5Bytes()
	if in.WorkflowKey == "" || !validDigest(in.TerminalProgramCHeadSelector) || !validDigest(in.ReceiptSelector) || in.ReceiptOrdinal <= 0 || in.ConstituentOrdinal < 0 || in.PublicationSelector == "" || in.ConstituentSelector == "" || !validProgramCSourceBinding(in.Source, sourceBytes, graphV5Bytes) || in.ProgramCAlgorithmVersion == "" || in.CommunityOrdinal < 0 || !validDigest(in.CommunityArtifactSHA256) || !strictStrings(in.CommunityMembers) {
		return Mechanics{}, fmt.Errorf("invalid singleton mechanics input")
	}
	if in.ReplayOrdinal < 1 || in.ReplayOrdinal > 3 || (in.Seed != 1 && in.Seed != 2) || in.RunID != fmt.Sprintf("seed-%020d-replay-%d", in.Seed, in.ReplayOrdinal) {
		return Mechanics{}, fmt.Errorf("invalid singleton mechanics run identity")
	}
	view, err := graph.NewNativeV5RepresentativeView(graphV5Bytes)
	if err != nil || view.ExecutionBundleID() != in.Source.ExecutionBundleID {
		return Mechanics{}, fmt.Errorf("invalid singleton mechanics source binding")
	}
	contract, err := newAdapterContract(in.ProgramCAlgorithmVersion)
	if err != nil {
		return Mechanics{}, err
	}
	bundleID := view.ExecutionBundleID()
	nativeDigest := rawDigest(sourceBytes)
	custody := CustodyIdentityDTO{
		SchemaVersion: CustodySchemaVersion, WorkflowKey: in.WorkflowKey,
		TerminalProgramCHeadSelector: in.TerminalProgramCHeadSelector,
		ReceiptSelector:              in.ReceiptSelector, ReceiptOrdinal: in.ReceiptOrdinal,
		ConstituentOrdinal: in.ConstituentOrdinal, PublicationSelector: in.PublicationSelector,
		ConstituentSelector: in.ConstituentSelector, NativeV5SHA256: nativeDigest,
		NativeV5ByteLength: uint64(len(sourceBytes)), ExecutionBundleID: bundleID,
	}
	custody.Identity, err = hashIdentity(custodyDomain, custody)
	if err != nil {
		return Mechanics{}, err
	}
	evidence, err := buildEvidence(view, in.ConstituentSelector, in.ReceiptSelector)
	if err != nil {
		return Mechanics{}, err
	}
	memberships, prepared, reached, relations, err := buildMembershipClosure(view)
	if err != nil {
		return Mechanics{}, err
	}
	occurrences := make([]string, 0)
	relationSet := stringSet(relations)
	for _, item := range evidence {
		if relationSet[item.RelationID] {
			occurrences = append(occurrences, item.Identity)
		}
	}
	sort.Strings(occurrences)
	if !strictStrings(occurrences) {
		return Mechanics{}, fmt.Errorf("invalid occurrence closure")
	}
	context := SelectionContextIdentityDTO{
		SchemaVersion: SelectionContextSchemaVersion, AdapterContractSHA256: contract.Identity,
		CustodyIdentity: custody.Identity, ConstituentSelector: in.ConstituentSelector,
		ExecutionBundleID: bundleID, RunID: in.RunID, Seed: in.Seed,
		ReplayOrdinal: in.ReplayOrdinal, CommunityOrdinal: in.CommunityOrdinal,
		CommunityArtifactSHA256: in.CommunityArtifactSHA256,
		CommunityMembers:        append([]string(nil), in.CommunityMembers...), Memberships: memberships,
		PreparedTargets: prepared, ReachedNodes: reached, RelationIDs: relations, OccurrenceIDs: occurrences,
	}
	context.Identity, err = hashIdentity(contextDomain, context)
	if err != nil {
		return Mechanics{}, err
	}
	dto := MechanicsIdentityDTO{
		SchemaVersion: MechanicsSchemaVersion, Profile: ProfileCallsOnlyFrozenCensusV1,
		AdapterContractSHA256: contract.Identity, CustodyIdentity: custody.Identity,
		ReceiptSelector: in.ReceiptSelector, RunID: in.RunID, Seed: in.Seed,
		ReplayOrdinal: in.ReplayOrdinal, CommunityOrdinal: in.CommunityOrdinal,
		CommunityArtifactSHA256: in.CommunityArtifactSHA256, Evidence: evidence,
		CommunityMembers: append([]string(nil), in.CommunityMembers...), SelectionContext: context,
		Authority: 0, Accepted: false,
	}
	dto.Identity, err = hashIdentity(mechanicsDomain, dto)
	if err != nil {
		return Mechanics{}, err
	}
	if err := validateMechanicsEnvelope(dto); err != nil {
		return Mechanics{}, err
	}
	canonical, err := json.Marshal(dto)
	if err != nil {
		return Mechanics{}, err
	}
	return Mechanics{dto: cloneMechanicsDTO(dto), canonical: canonical, trusted: true}, nil
}

func (m Mechanics) Valid() bool {
	return m.trusted && validateMechanicsEnvelope(m.dto) == nil && len(m.canonical) > 0
}
func (m Mechanics) Trusted() bool             { return m.Valid() }
func (m Mechanics) Identity() string          { return m.dto.Identity }
func (m Mechanics) DTO() MechanicsIdentityDTO { return cloneMechanicsDTO(m.dto) }
func (m Mechanics) CanonicalBytes() []byte    { return append([]byte(nil), m.canonical...) }
func (m Mechanics) Evidence() []EvidenceIdentityDTO {
	return append([]EvidenceIdentityDTO(nil), m.dto.Evidence...)
}
func (m Mechanics) CommunityMembers() []string {
	return append([]string(nil), m.dto.CommunityMembers...)
}
func (m Mechanics) SelectionContext() SelectionContextIdentityDTO {
	return cloneSelectionContext(m.dto.SelectionContext)
}

func ParseMechanics(raw []byte) (Mechanics, error) {
	var dto MechanicsIdentityDTO
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&dto); err != nil {
		return Mechanics{}, err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return Mechanics{}, fmt.Errorf("trailing mechanics bytes")
	}
	if err := validateMechanicsEnvelope(dto); err != nil {
		return Mechanics{}, err
	}
	canonical, err := json.Marshal(dto)
	if err != nil || !bytes.Equal(canonical, raw) {
		return Mechanics{}, fmt.Errorf("noncanonical mechanics bytes")
	}
	return Mechanics{dto: cloneMechanicsDTO(dto), canonical: append([]byte(nil), raw...)}, nil
}

// ValidateMechanicsDTO establishes trust only by reconstructing the exact
// expected mechanics value from independently supplied context.
func ValidateMechanicsDTO(dto MechanicsIdentityDTO, expected SingletonMechanicsInput) error {
	constructed, err := NewSingletonMechanics(expected)
	if err != nil {
		return err
	}
	canonical, err := json.Marshal(dto)
	if err != nil || !bytes.Equal(canonical, constructed.canonical) {
		return fmt.Errorf("mechanics DTO does not match reconstructed context")
	}
	return nil
}

func validateMechanicsEnvelope(dto MechanicsIdentityDTO) error {
	if dto.SchemaVersion != MechanicsSchemaVersion || dto.Profile != ProfileCallsOnlyFrozenCensusV1 || dto.Authority != 0 || dto.Accepted || !validDigest(dto.AdapterContractSHA256) || !validDigest(dto.CustodyIdentity) || !validDigest(dto.ReceiptSelector) || dto.RunID == "" || dto.ReplayOrdinal < 1 || dto.ReplayOrdinal > 3 || dto.CommunityOrdinal < 0 || !validDigest(dto.CommunityArtifactSHA256) || dto.Evidence == nil || !strictStrings(dto.CommunityMembers) || !validDigest(dto.Identity) {
		return fmt.Errorf("invalid mechanics DTO")
	}
	seenEvidence := map[string]bool{}
	for _, evidence := range dto.Evidence {
		if evidence.SchemaVersion != EvidenceSchemaVersion || evidence.ConstituentSelector == "" || evidence.ExecutionBundleID == "" || evidence.RelationID == "" || evidence.CallSiteOrdinal < 0 || evidence.FromNodeID == "" || evidence.ToNodeID == "" || !validDigest(evidence.ReceiptSelector) || !validDigest(evidence.Identity) || seenEvidence[evidence.Identity] {
			return fmt.Errorf("invalid mechanics evidence")
		}
		if graph.ValidateRange(graph.Range{Start: graph.Position{Line: evidence.CallSite.Start.Line, Character: evidence.CallSite.Start.Character}, End: graph.Position{Line: evidence.CallSite.End.Line, Character: evidence.CallSite.End.Character}}) != nil {
			return fmt.Errorf("invalid mechanics call site")
		}
		copy := evidence
		copy.Identity = ""
		identity, err := hashIdentity(evidenceDomain, copy)
		if err != nil || identity != evidence.Identity {
			return fmt.Errorf("invalid evidence identity")
		}
		seenEvidence[evidence.Identity] = true
	}
	context := dto.SelectionContext
	if context.SchemaVersion != SelectionContextSchemaVersion || context.AdapterContractSHA256 != dto.AdapterContractSHA256 || context.CustodyIdentity != dto.CustodyIdentity || context.RunID != dto.RunID || context.Seed != dto.Seed || context.ReplayOrdinal != dto.ReplayOrdinal || context.CommunityOrdinal != dto.CommunityOrdinal || context.CommunityArtifactSHA256 != dto.CommunityArtifactSHA256 || !equalMechanicsStrings(context.CommunityMembers, dto.CommunityMembers) || context.Memberships == nil || context.PreparedTargets == nil || context.ReachedNodes == nil || context.RelationIDs == nil || context.OccurrenceIDs == nil || !strictMemberships(context.Memberships) || !strictOptionalStrings(context.PreparedTargets) || !strictOptionalStrings(context.ReachedNodes) || !strictOptionalStrings(context.RelationIDs) || !strictOptionalStrings(context.OccurrenceIDs) || !validDigest(context.Identity) {
		return fmt.Errorf("invalid selection context")
	}
	contextCopy := cloneSelectionContext(context)
	contextCopy.Identity = ""
	contextIdentity, err := hashIdentity(contextDomain, contextCopy)
	if err != nil || contextIdentity != context.Identity {
		return fmt.Errorf("invalid selection context identity")
	}
	copy := cloneMechanicsDTO(dto)
	copy.Identity = ""
	identity, err := hashIdentity(mechanicsDomain, copy)
	if err != nil || identity != dto.Identity {
		return fmt.Errorf("invalid mechanics identity")
	}
	return nil
}

func newAdapterContract(algorithmVersion string) (AdapterContractDTO, error) {
	if algorithmVersion == "" {
		return AdapterContractDTO{}, fmt.Errorf("missing Program C algorithm version")
	}
	dto := AdapterContractDTO{
		SchemaVersion: AdapterContractSchemaVersion, Profile: ProfileCallsOnlyFrozenCensusV1,
		Domains: append([]string(nil), adapterDomains...), NativeSchemaVersion: graph.SchemaVersionV5,
		ProgramCProfileID: "calls-v1", ProgramCProfileSHA256: "sha256:f4fb309c6e849b5a8e6057f355b1db6ee3c53414c3c430b67be17f68fe9e97f9",
		ProgramCAlgorithm: "gonum.org/v1/gonum/graph/community.Leiden", ProgramCAlgorithmVersion: algorithmVersion,
		RunSeeds: []uint64{1, 2}, ReplaysPerSeed: 3, RunIDGrammar: "seed-%020d-replay-%d",
		OccurrenceOrdinalRule: "ZERO_BASED_WITHIN_CANONICAL_EDGE_CALL_SITES",
		MembershipKinds:       append([]string(nil), membershipKinds...), MappedMembershipKinds: append([]string(nil), mappedMembershipKinds...),
		CommunityOrderRule: "PROGRAM_C_VALID_CANONICAL_COMMUNITIES", Authority: 0, Accepted: false,
	}
	identity, err := hashIdentity(adapterContractDomain, dto)
	if err != nil {
		return AdapterContractDTO{}, err
	}
	dto.Identity = identity
	return dto, nil
}

func buildEvidence(view graph.NativeV5RepresentativeView, constituent, receipt string) ([]EvidenceIdentityDTO, error) {
	edges := view.Edges()
	if edges == nil {
		return nil, fmt.Errorf("missing canonical edges")
	}
	out := make([]EvidenceIdentityDTO, 0)
	seen := map[string]bool{}
	for _, edge := range edges {
		if edge.RelationID == "" || edge.ExecutionBundleID != view.ExecutionBundleID() || edge.CallerNodeID == "" || edge.CalleeNodeID == "" || len(edge.CallSites) == 0 {
			return nil, fmt.Errorf("invalid canonical edge")
		}
		var previous graph.Range
		for ordinal, site := range edge.CallSites {
			if graph.ValidateRange(site) != nil || (ordinal > 0 && !lessRange(previous, site)) {
				return nil, fmt.Errorf("noncanonical call sites")
			}
			dto := EvidenceIdentityDTO{
				SchemaVersion: EvidenceSchemaVersion, ConstituentSelector: constituent,
				ExecutionBundleID: edge.ExecutionBundleID, RelationID: edge.RelationID,
				CallSiteOrdinal: ordinal, FromNodeID: edge.CallerNodeID, ToNodeID: edge.CalleeNodeID,
				CallSite:        RangeDTO{Start: PositionDTO{Line: site.Start.Line, Character: site.Start.Character}, End: PositionDTO{Line: site.End.Line, Character: site.End.Character}},
				ReceiptSelector: receipt,
			}
			identity, err := hashIdentity(evidenceDomain, dto)
			if err != nil || seen[identity] {
				return nil, fmt.Errorf("duplicate evidence identity")
			}
			dto.Identity = identity
			seen[identity] = true
			out = append(out, dto)
			previous = site
		}
	}
	return out, nil
}

func buildMembershipClosure(view graph.NativeV5RepresentativeView) ([]MembershipIdentityDTO, []string, []string, []string, error) {
	seeds := view.InvocationSeeds()
	memberships := view.SeedMemberships()
	if len(seeds) != 1 || len(memberships) == 0 || seeds[0].Label == "" || seeds[0].At == "" {
		return nil, nil, nil, nil, fmt.Errorf("membership group multiplicity")
	}
	nodes := stringSet(view.NodeIDs())
	relationsAvailable := map[string]bool{}
	for _, edge := range view.Edges() {
		relationsAvailable[edge.RelationID] = true
	}
	out := make([]MembershipIdentityDTO, len(memberships))
	prepared, reached, relations := []string{}, []string{}, []string{}
	for i, membership := range memberships {
		if membership.MembershipID == "" || (i > 0 && memberships[i-1].MembershipID >= membership.MembershipID) || membership.ExecutionBundleID != view.ExecutionBundleID() || membership.SeedLabel != seeds[0].Label || membership.SeedAt != seeds[0].At || membership.EndpointID == "" || !contains(membershipKinds, membership.EvidenceKind) {
			return nil, nil, nil, nil, fmt.Errorf("invalid seed membership")
		}
		switch membership.EvidenceKind {
		case "PREPARED_TARGET":
			if !nodes[membership.EndpointID] {
				return nil, nil, nil, nil, fmt.Errorf("unknown prepared target")
			}
			prepared = append(prepared, membership.EndpointID)
		case "REACHED_NODE":
			if !nodes[membership.EndpointID] {
				return nil, nil, nil, nil, fmt.Errorf("unknown reached node")
			}
			reached = append(reached, membership.EndpointID)
		case "CALL_RELATION":
			if !relationsAvailable[membership.EndpointID] {
				return nil, nil, nil, nil, fmt.Errorf("unknown call relation")
			}
			relations = append(relations, membership.EndpointID)
		case "SIBLING_CANDIDATE", "DISPATCH_ASSOCIATION":
			// Strict Graph V5 validation already endpoint-checks these relation kinds.
		}
		out[i] = MembershipIdentityDTO{MembershipID: membership.MembershipID, ExecutionBundleID: membership.ExecutionBundleID, SeedLabel: membership.SeedLabel, SeedAt: membership.SeedAt, EvidenceKind: membership.EvidenceKind, EndpointID: membership.EndpointID}
	}
	prepared, reached, relations = sortedUnique(prepared), sortedUnique(reached), sortedUnique(relations)
	if len(prepared) == 0 || len(reached) == 0 || len(relations) == 0 {
		return nil, nil, nil, nil, fmt.Errorf("incomplete mapped membership closure")
	}
	return out, prepared, reached, relations, nil
}

func hashIdentity(domain string, dto any) (string, error) {
	body, err := json.Marshal(dto)
	if err != nil {
		return "", err
	}
	preimage := make([]byte, 0, len(domain)+1+len(body))
	preimage = append(preimage, domain...)
	preimage = append(preimage, 0)
	preimage = append(preimage, body...)
	sum := sha256.Sum256(preimage)
	return "sha256:" + hex.EncodeToString(sum[:]), nil
}

func validProgramCSourceBinding(source programc.SourceBinding, inputBytes, graphV5Bytes []byte) bool {
	return len(inputBytes) > 0 && source.InputLength == len(inputBytes) && source.InputSHA256 == rawDigest(inputBytes) &&
		len(graphV5Bytes) > 0 && source.GraphV5Length == len(graphV5Bytes) && source.GraphV5SHA256 == rawDigest(graphV5Bytes) &&
		source.ExecutionBundleID != ""
}

func rawDigest(raw []byte) string {
	sum := sha256.Sum256(raw)
	return "sha256:" + hex.EncodeToString(sum[:])
}
func validDigest(value string) bool {
	if len(value) != 71 || !strings.HasPrefix(value, "sha256:") {
		return false
	}
	_, err := hex.DecodeString(value[7:])
	return err == nil && value == strings.ToLower(value)
}
func contains(values []string, value string) bool {
	for _, candidate := range values {
		if candidate == value {
			return true
		}
	}
	return false
}
func stringSet(values []string) map[string]bool {
	out := make(map[string]bool, len(values))
	for _, value := range values {
		out[value] = true
	}
	return out
}
func sortedUnique(values []string) []string {
	out := append([]string(nil), values...)
	sort.Strings(out)
	if len(out) == 0 {
		return []string{}
	}
	result := out[:1]
	for _, value := range out[1:] {
		if value != result[len(result)-1] {
			result = append(result, value)
		}
	}
	return result
}
func strictStrings(values []string) bool { return len(values) > 0 && strictOptionalStrings(values) }
func strictOptionalStrings(values []string) bool {
	if values == nil {
		return false
	}
	for i, value := range values {
		if value == "" || (i > 0 && values[i-1] >= value) {
			return false
		}
	}
	return true
}
func strictMemberships(values []MembershipIdentityDTO) bool {
	if values == nil {
		return false
	}
	for i, value := range values {
		if value.MembershipID == "" || value.ExecutionBundleID == "" || value.SeedLabel == "" || value.SeedAt == "" || value.EndpointID == "" || !contains(membershipKinds, value.EvidenceKind) || (i > 0 && values[i-1].MembershipID >= value.MembershipID) {
			return false
		}
	}
	return true
}
func equalMechanicsStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
func lessRange(a, b graph.Range) bool {
	if a.Start.Line != b.Start.Line {
		return a.Start.Line < b.Start.Line
	}
	if a.Start.Character != b.Start.Character {
		return a.Start.Character < b.Start.Character
	}
	if a.End.Line != b.End.Line {
		return a.End.Line < b.End.Line
	}
	return a.End.Character < b.End.Character
}

func cloneSelectionContext(in SelectionContextIdentityDTO) SelectionContextIdentityDTO {
	out := in
	out.CommunityMembers = append([]string(nil), in.CommunityMembers...)
	out.Memberships = append([]MembershipIdentityDTO(nil), in.Memberships...)
	out.PreparedTargets = append([]string(nil), in.PreparedTargets...)
	out.ReachedNodes = append([]string(nil), in.ReachedNodes...)
	out.RelationIDs = append([]string(nil), in.RelationIDs...)
	out.OccurrenceIDs = append([]string(nil), in.OccurrenceIDs...)
	return out
}
func cloneMechanicsDTO(in MechanicsIdentityDTO) MechanicsIdentityDTO {
	out := in
	out.Evidence = append([]EvidenceIdentityDTO(nil), in.Evidence...)
	out.CommunityMembers = append([]string(nil), in.CommunityMembers...)
	out.SelectionContext = cloneSelectionContext(in.SelectionContext)
	return out
}
