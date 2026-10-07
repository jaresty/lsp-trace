package censuscontinuation

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"reflect"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"lsp-trace/internal/graph"
	"lsp-trace/internal/programc"
	"lsp-trace/internal/retainedprojection"
	"lsp-trace/internal/sourceprojection"
	"lsp-trace/internal/v5sourcesnapshotv6"
)

const (
	CandidateGroupSchema         = "lsp-trace.census-continuation-candidate-group.v1"
	SourceUnavailable            = "SOURCE_UNAVAILABLE"
	InterpretationDescription    = "DESCRIPTION"
	InterpretationUnresolved     = "UNRESOLVED"
	candidateGroupIdentityDomain = "lsp-trace:census-continuation-candidate-group:v1"
	candidateGroupProfileDomain  = "lsp-trace:candidate-group-private-profile:v1"

	CandidateGroupPrivateV1ProfileID      = "candidate-group-private-v1"
	CandidateGroupPrivateV2ProfileID      = "candidate-group-private-v2"
	CandidateGroupPrivateV2ProfileDigest  = "sha256:94e015fa7eef8ce9f0683b530ea2de3d13ed350dd78b947d51001da31fa4c416"
	candidateGroupValidationPolicyID      = "candidate-group-boundary-artifact-validation"
	candidateGroupValidationPolicyVersion = "v1"
)

type CandidateGroupBounds struct {
	MaxCommunities, MaxMembersPerCommunity, MaxDistinctMembers, MaxCallOccurrences        int
	MaxSourceBytesPerMember, MaxTotalUniqueSourceBytes, MaxSourceRanges, MaxSourceObjects int
	MaxWorkUnits, MaxPageRankWork, MaxResponseBytes, MaxArtifactBytes                     int
	MaxCitations, MaxDescriptionUTF8Bytes, MaxDescriptions, MaxHostAttempts               int
}

func FrozenCandidateGroupBounds() CandidateGroupBounds {
	return CandidateGroupBounds{4, 4, 7, 11, 2048, 14336, 32, 7, 512, 1024, 65536, 131072, 16, 4096, 1, 1}
}

type CandidateGroupProfile struct {
	ProfileID               string               `json:"profile_id"`
	Bounds                  CandidateGroupBounds `json:"bounds"`
	ValidationPolicyID      string               `json:"validation_policy_id"`
	ValidationPolicyVersion string               `json:"validation_policy_version"`
}

func FrozenCandidateGroupProfile() CandidateGroupProfile {
	return CandidateGroupProfile{CandidateGroupPrivateV1ProfileID, FrozenCandidateGroupBounds(), candidateGroupValidationPolicyID, candidateGroupValidationPolicyVersion}
}

func CandidateGroupPrivateV2Profile() CandidateGroupProfile {
	bounds := FrozenCandidateGroupBounds()
	bounds.MaxPageRankWork = 1664
	return CandidateGroupProfile{CandidateGroupPrivateV2ProfileID, bounds, candidateGroupValidationPolicyID, candidateGroupValidationPolicyVersion}
}

func (p CandidateGroupProfile) Digest() string {
	raw, _ := json.Marshal(p)
	h := sha256.New()
	h.Write([]byte(candidateGroupProfileDomain))
	h.Write([]byte{0})
	h.Write(raw)
	return "sha256:" + hex.EncodeToString(h.Sum(nil))
}

func validCandidateGroupProfile(p CandidateGroupProfile) bool {
	return p == FrozenCandidateGroupProfile() || p == CandidateGroupPrivateV2Profile()
}

type Citation struct {
	SourceID             string                 `json:"source_id"`
	LogicalSourceID      string                 `json:"logical_source_id"`
	Range                sourceprojection.Range `json:"range"`
	Claim                string                 `json:"claim"`
	RelationID           string                 `json:"relation_id,omitempty"`
	OccurrenceID         string                 `json:"occurrence_id,omitempty"`
	CallerProgramCNodeID string                 `json:"caller_program_c_node_id,omitempty"`
	CalleeProgramCNodeID string                 `json:"callee_program_c_node_id,omitempty"`
}
type OriginalManagedMemberIdentity struct {
	GraphSubjectID, SymbolName, URI   string
	ItemRange, SelectionRange         sourceprojection.Range
	DocumentDigest, SessionID         string
	Generation                        uint64
	DocumentVersion, PositionEncoding string
	SourceResponseIdentity            string
}
type MemberCorrespondence struct {
	GraphSubjectID, V6EndpointGraphSubjectID, ProgramCNodeID, SymbolName, URI string
	ItemRange, SelectionRange                                                 sourceprojection.Range
	FixtureDigest, DocumentDigest                                             string
	SessionID                                                                 string
	Generation                                                                uint64
	DocumentVersion, PositionEncoding                                         string
	SourceResponseIdentity                                                    string
}
type HostInterpretation struct {
	Status, HostID, ModelID, ContextID, Description string
	Attempts                                        int
	MemberIDs                                       []string
	Citations                                       []Citation
	Claims                                          []string
}
type CandidateGroupInput struct {
	Outcome                programc.Outcome
	Boundary               programc.BoundaryArtifact
	CommunityMembers       []string
	RepresentativeID       string
	Bounds                 CandidateGroupBounds
	ResourceProfile        CandidateGroupProfile
	Interpretation         HostInterpretation
	SourceSnapshot         []byte
	SourceLookup           retainedprojection.Lookup
	OriginalManagedMembers map[string]OriginalManagedMemberIdentity
}
type MemberSourceOutcome struct {
	Status           string                                    `json:"status"`
	Reason           string                                    `json:"reason"`
	CustodyAvailable bool                                      `json:"custody_available"`
	SourceID         string                                    `json:"source_id,omitempty"`
	SourceByteLength int                                       `json:"source_byte_length,omitempty"`
	Body             string                                    `json:"body,omitempty"`
	UnitID           string                                    `json:"unit_id,omitempty"`
	CitationID       string                                    `json:"citation_id,omitempty"`
	EvidenceRange    sourceprojection.Range                    `json:"evidence_range,omitempty"`
	DisplayRange     sourceprojection.Range                    `json:"display_range,omitempty"`
	ItemRange        *sourceprojection.Range                   `json:"item_range,omitempty"`
	SelectionRange   *sourceprojection.Range                   `json:"selection_range,omitempty"`
	Custody          retainedprojection.RetainedCustodyBinding `json:"custody"`
	Correspondence   *MemberCorrespondence                     `json:"correspondence,omitempty"`
	Citations        []Citation                                `json:"citations"`
}
type CandidateGroupMember struct {
	ID     string              `json:"id"`
	Name   string              `json:"name"`
	Source MemberSourceOutcome `json:"source"`
}
type CandidateGroupCall struct {
	OccurrenceID, RelationID, SourceNodeID, TargetNodeID string
	CallSite                                             graph.Range
}
type CandidateGroupRepresentative struct {
	ID   string `json:"id"`
	Role string `json:"role"`
}
type CandidateGroupSourceAccounting struct {
	AttemptedMembers   int `json:"attempted_members"`
	TerminalMembers    int `json:"terminal_members"`
	UnavailableMembers int `json:"unavailable_members"`
	AvailableMembers   int `json:"available_members"`
	UniqueSourceBytes  int `json:"unique_source_bytes"`
	SourceRanges       int `json:"source_ranges"`
	SourceObjects      int `json:"source_objects"`
}
type candidateBoundary struct {
	Accounting                  programc.BoundaryAccounting  `json:"accounting"`
	PageRank                    programc.BoundaryPageRank    `json:"pagerank"`
	Community                   programc.BoundaryCommunity   `json:"selected_community"`
	HighCentralityCrossingNodes []programc.BoundaryNodeScore `json:"high_centrality_crossing_nodes"`
	HubCrossingNodes            []programc.BoundaryNodeScore `json:"hub_crossing_nodes"`
	CrossingWitnesses           []programc.CrossingWitness   `json:"crossing_witnesses"`
	Bridges                     []string                     `json:"bridges"`
	ArticulationPoints          []string                     `json:"articulation_points"`
}
type CandidateGroupInterpretation struct {
	Status      string     `json:"status"`
	HostID      string     `json:"host_id,omitempty"`
	ModelID     string     `json:"model_id,omitempty"`
	ContextID   string     `json:"context_id,omitempty"`
	Attempts    int        `json:"attempts"`
	Description string     `json:"description,omitempty"`
	Citations   []Citation `json:"citations"`
	Claims      []string   `json:"claims"`
}
type CandidateGroupArtifact struct {
	SchemaVersion         string                         `json:"schema_version"`
	ArtifactID            string                         `json:"artifact_id"`
	CommunityID           string                         `json:"community_id"`
	ProfileID             string                         `json:"profile_id"`
	ProfileDigest         string                         `json:"profile_digest"`
	ResourceProfileID     string                         `json:"resource_profile_id"`
	ResourceProfileDigest string                         `json:"resource_profile_digest"`
	Algorithm             string                         `json:"algorithm"`
	PartitionDigest       string                         `json:"partition_digest"`
	Seed                  uint64                         `json:"seed"`
	Bounds                CandidateGroupBounds           `json:"bounds"`
	Members               []CandidateGroupMember         `json:"members"`
	SourceAccounting      CandidateGroupSourceAccounting `json:"source_accounting"`
	InternalCalls         []CandidateGroupCall           `json:"internal_calls"`
	CrossingCalls         []CandidateGroupCall           `json:"crossing_calls"`
	BoundaryInputGZIP     []byte                         `json:"boundary_input_gzip"`
	Boundary              programc.BoundaryArtifact      `json:"boundary"`
	Representative        CandidateGroupRepresentative   `json:"representative"`
	Interpretation        CandidateGroupInterpretation   `json:"interpretation"`
	Authority             int                            `json:"authority"`
	Accepted              bool                           `json:"accepted"`
	Completeness          string                         `json:"completeness"`
}

func (a CandidateGroupArtifact) ID() string { return a.ArtifactID }
func (a CandidateGroupArtifact) MemberIDs() []string {
	out := make([]string, len(a.Members))
	for i := range a.Members {
		out[i] = a.Members[i].ID
	}
	return out
}
func (a CandidateGroupArtifact) Bytes() ([]byte, error) {
	if err := validateCandidateGroup(a); err != nil {
		return nil, err
	}
	raw, err := json.Marshal(a)
	if err != nil || len(raw) > a.Bounds.MaxArtifactBytes {
		return nil, errors.New("candidate group artifact bytes exceeded")
	}
	return raw, nil
}

func ParseCandidateGroup(raw []byte) (CandidateGroupArtifact, error) {
	var artifact CandidateGroupArtifact
	if err := strictCanonical(raw, &artifact); err != nil {
		return CandidateGroupArtifact{}, errors.New("invalid candidate group artifact")
	}
	canonical, err := artifact.Bytes()
	if err != nil || !bytes.Equal(raw, canonical) {
		return CandidateGroupArtifact{}, errors.New("invalid candidate group artifact")
	}
	return artifact, nil
}

func BuildCandidateGroup(in CandidateGroupInput) (CandidateGroupArtifact, error) {
	fail := func(err error) (CandidateGroupArtifact, error) { return CandidateGroupArtifact{}, err }
	profile := in.ResourceProfile
	if profile == (CandidateGroupProfile{}) {
		profile = FrozenCandidateGroupProfile()
	}
	if !validCandidateGroupProfile(profile) || in.Bounds != profile.Bounds {
		return fail(errors.New("candidate group bounds mismatch"))
	}
	if len(in.Outcome.Communities) > in.Bounds.MaxCommunities || len(in.Outcome.Projection.NodeIdentities) > in.Bounds.MaxDistinctMembers || len(in.Outcome.Projection.Occurrences) > in.Bounds.MaxCallOccurrences {
		return fail(errors.New("candidate group resource limit"))
	}
	if err := validatePageRankWork(in.Boundary.PageRank.Work, in.Bounds.MaxPageRankWork); err != nil {
		return fail(err)
	}
	if in.Bounds.MaxWorkUnits < len(in.Outcome.Projection.NodeIdentities)+len(in.Outcome.Projection.Occurrences) {
		return fail(errors.New("candidate group work limit"))
	}
	canonical, failure := programc.Compute(in.Outcome.Source.InputBytes(), in.Outcome.Seed)
	if failure != nil || !reflect.DeepEqual(canonical, in.Outcome) {
		return fail(errors.New("candidate group outcome mismatch"))
	}
	boundary, err := programc.ComputeBoundary(canonical, in.Boundary.Request)
	if err != nil || !reflect.DeepEqual(boundary, in.Boundary) {
		return fail(errors.New("candidate group boundary mismatch"))
	}
	members := append([]string(nil), in.CommunityMembers...)
	if len(members) == 0 || len(members) > in.Bounds.MaxMembersPerCommunity || !sort.StringsAreSorted(members) {
		return fail(errors.New("candidate group member order"))
	}
	var selected *programc.Community
	for i := range canonical.Communities {
		if reflect.DeepEqual(canonical.Communities[i].Members, members) {
			selected = &canonical.Communities[i]
			break
		}
	}
	if selected == nil {
		return fail(errors.New("candidate group community mismatch"))
	}
	memberSet := map[string]bool{}
	for _, id := range members {
		memberSet[id] = true
	}
	if !memberSet[in.RepresentativeID] {
		return fail(errors.New("candidate group representative mismatch"))
	}
	if len(in.Interpretation.MemberIDs) != 0 || in.Interpretation.Attempts < 0 || in.Interpretation.Attempts > in.Bounds.MaxHostAttempts || len(in.Interpretation.Citations) > in.Bounds.MaxCitations || !utf8.ValidString(in.Interpretation.Description) || len([]byte(in.Interpretation.Description)) > in.Bounds.MaxDescriptionUTF8Bytes {
		return fail(errors.New("candidate group interpretation limit"))
	}
	if in.Interpretation.Status != InterpretationUnresolved && in.Interpretation.Status != InterpretationDescription {
		return fail(errors.New("candidate group interpretation status"))
	}
	if in.Interpretation.Status == InterpretationUnresolved && (in.Interpretation.Description != "" || len(in.Interpretation.Citations) != 0 || len(in.Interpretation.Claims) != 0) {
		return fail(errors.New("unresolved interpretation must be empty"))
	}
	var native struct {
		Nodes []graph.Node `json:"nodes"`
	}
	if json.Unmarshal(canonical.Source.GraphV5Bytes(), &native) != nil {
		return fail(errors.New("candidate group graph join"))
	}
	names := map[string]string{}
	for _, n := range native.Nodes {
		if _, ok := names[n.ID]; ok {
			return fail(errors.New("duplicate graph node"))
		}
		names[n.ID] = n.Name
	}
	boundaryInput, err := gzipCanonicalInput(canonical.Source.InputBytes())
	if err != nil {
		return fail(err)
	}
	artifact := CandidateGroupArtifact{SchemaVersion: CandidateGroupSchema, ProfileID: canonical.ProfileID, ProfileDigest: canonical.ProfileDigest, ResourceProfileID: profile.ProfileID, ResourceProfileDigest: profile.Digest(), Algorithm: canonical.Algorithm, PartitionDigest: canonical.LogicalDigest, Seed: canonical.Seed, Bounds: in.Bounds, Members: make([]CandidateGroupMember, 0, len(members)), SourceAccounting: CandidateGroupSourceAccounting{AttemptedMembers: len(members), TerminalMembers: len(members)}, InternalCalls: []CandidateGroupCall{}, CrossingCalls: []CandidateGroupCall{}, BoundaryInputGZIP: boundaryInput, Representative: CandidateGroupRepresentative{in.RepresentativeID, "NAVIGATION_ONLY"}, Interpretation: CandidateGroupInterpretation{in.Interpretation.Status, in.Interpretation.HostID, in.Interpretation.ModelID, in.Interpretation.ContextID, in.Interpretation.Attempts, in.Interpretation.Description, append([]Citation(nil), in.Interpretation.Citations...), append([]string(nil), in.Interpretation.Claims...)}, Completeness: CompletenessUnknown}
	admitted, sourceErr := retainedprojection.Admit(in.SourceSnapshot)
	keys := map[string]retainedprojection.Key{}
	correspondences := map[string]MemberCorrespondence{}
	if sourceErr == nil {
		var retained v5sourcesnapshotv6.Artifact
		if json.Unmarshal(in.SourceSnapshot, &retained) == nil && retained.SchemaVersion == v5sourcesnapshotv6.Version {
			for _, binding := range retained.EndpointBindings {
				original := in.OriginalManagedMembers[binding.NodeID]
				if original.GraphSubjectID == "" || original.GraphSubjectID == binding.NodeID || original.SymbolName != names[binding.NodeID] || original.URI != binding.LogicalSourceID || original.ItemRange != projectionRange(binding.ItemRange) || original.SelectionRange != projectionRange(binding.SelectionRange) || original.DocumentDigest != binding.SourceDigest || original.SessionID != binding.SessionID || original.Generation != binding.Generation || original.DocumentVersion != binding.DocumentVersion || original.PositionEncoding != binding.PositionEncoding || original.SourceResponseIdentity == "" {
					return fail(errors.New("candidate group original managed identity mismatch"))
				}
				c := MemberCorrespondence{GraphSubjectID: original.GraphSubjectID, V6EndpointGraphSubjectID: binding.GraphSubjectID, ProgramCNodeID: binding.NodeID, SymbolName: original.SymbolName, URI: binding.LogicalSourceID, ItemRange: projectionRange(binding.ItemRange), SelectionRange: projectionRange(binding.SelectionRange), FixtureDigest: retained.GraphV5Digest, DocumentDigest: binding.SourceDigest, SessionID: binding.SessionID, Generation: binding.Generation, DocumentVersion: binding.DocumentVersion, PositionEncoding: binding.PositionEncoding, SourceResponseIdentity: original.SourceResponseIdentity}
				if prior, duplicate := correspondences[binding.NodeID]; duplicate && prior != c {
					return fail(errors.New("candidate group correspondence ambiguity"))
				}
				correspondences[binding.NodeID] = c
			}
		}
	}
	if sourceErr == nil {
		for _, key := range admitted.DisplayKeys() {
			if prior, duplicate := keys[key.GraphSubjectID]; duplicate && prior != key {
				return fail(errors.New("candidate group source binding ambiguity"))
			}
			keys[key.GraphSubjectID] = key
		}
	}
	sourceObjects := map[string]int{}
	for _, id := range members {
		name, ok := names[id]
		if !ok {
			return fail(errors.New("candidate group member graph join"))
		}
		source := MemberSourceOutcome{Status: SourceUnavailable, Reason: "exact retained V6 member source unavailable", Citations: []Citation{}}
		if sourceErr == nil && in.SourceLookup != nil {
			corr, ok := correspondences[id]
			if !ok {
				return fail(errors.New("candidate group correspondence missing"))
			}
			key, ok := keys[corr.V6EndpointGraphSubjectID]
			if !ok {
				return fail(errors.New("candidate group member source binding missing"))
			}
			plan, err := retainedprojection.Select(admitted, retainedprojection.Request{Target: key, Selections: []retainedprojection.Key{key}})
			if err != nil {
				return fail(err)
			}
			custody, err := admitted.CustodyBinding(plan)
			if err != nil {
				return fail(err)
			}
			resolved, err := retainedprojection.Resolve(plan, in.SourceLookup, retainedprojection.ResolveLimits{MaxDistinctObjects: uint64(in.Bounds.MaxSourceObjects), MaxUniqueSourceBytes: uint64(in.Bounds.MaxTotalUniqueSourceBytes), MaxLogicalSelections: uint64(in.Bounds.MaxSourceRanges)})
			if err != nil {
				return fail(err)
			}
			policy := sourceprojection.Policy{PolicyID: "candidate-group-retained-v6", BodyRequested: true, MaxBytes: in.Bounds.MaxSourceBytesPerMember, MaxRanges: in.Bounds.MaxSourceRanges, MaxObjects: in.Bounds.MaxSourceObjects, MaxWork: in.Bounds.MaxWorkUnits, EnforceLimits: true}
			wire, err := retainedprojection.AssembleV2Bounded(resolved, policy, custody, policy.PolicyID, in.Bounds.MaxResponseBytes)
			if err != nil || wire.Status != "COMPLETE" || len(wire.Units) != 1 || len(wire.Citations) != 1 {
				return fail(errors.New("candidate group retained source assembly"))
			}
			u, c := wire.Units[0], wire.Citations[0]
			if u.GraphSubjectID != corr.V6EndpointGraphSubjectID || u.BodyDisposition != "RETURNED" || u.Body == "" || c.UnitID != u.UnitID {
				return fail(errors.New("candidate group retained source identity"))
			}
			source = MemberSourceOutcome{Status: "CAPTURED", CustodyAvailable: true, SourceID: u.SourceDigest, SourceByteLength: u.SourceByteLength, Body: u.Body, UnitID: u.UnitID, CitationID: c.CitationID, EvidenceRange: u.EvidenceRange, DisplayRange: u.DisplayRange, ItemRange: u.ItemRange, SelectionRange: u.SelectionRange, Custody: custody, Correspondence: &corr, Citations: []Citation{{SourceID: c.CitationID, LogicalSourceID: corr.URI, Range: u.EvidenceRange}}}
			sourceObjects[u.SourceDigest] = u.SourceByteLength
			artifact.SourceAccounting.AvailableMembers++
			artifact.SourceAccounting.SourceRanges++
		} else {
			artifact.SourceAccounting.UnavailableMembers++
		}
		artifact.Members = append(artifact.Members, CandidateGroupMember{ID: id, Name: name, Source: source})
	}
	for _, n := range sourceObjects {
		artifact.SourceAccounting.SourceObjects++
		artifact.SourceAccounting.UniqueSourceBytes += n
	}
	for _, o := range canonical.Projection.Occurrences {
		c := CandidateGroupCall{o.Identity, o.RelationID, canonical.Projection.NodeIdentities[o.From], canonical.Projection.NodeIdentities[o.To], o.CallSite}
		if memberSet[c.SourceNodeID] && memberSet[c.TargetNodeID] {
			artifact.InternalCalls = append(artifact.InternalCalls, c)
		} else if memberSet[c.SourceNodeID] || memberSet[c.TargetNodeID] {
			artifact.CrossingCalls = append(artifact.CrossingCalls, c)
		}
	}
	for _, c := range boundary.Communities {
		if reflect.DeepEqual(c.Members, members) {
			artifact.CommunityID = c.CommunityID
			break
		}
	}
	if artifact.CommunityID == "" {
		return fail(errors.New("candidate group boundary community"))
	}
	artifact.Boundary = boundary
	artifact.ArtifactID = sealCandidateGroup(artifact)
	raw, err := artifact.Bytes()
	if err != nil {
		return fail(err)
	}
	if len(raw) > in.Bounds.MaxResponseBytes {
		return fail(errors.New("candidate group response limit"))
	}
	if bytes.Contains(raw, []byte("OPAQUE_ITEM_DATA_MARKER")) || bytes.Contains(raw, []byte("RETAINED_SOURCE_BODY_MARKER")) {
		return fail(errors.New("candidate group marker leakage"))
	}
	return artifact, nil
}

func validatePageRankWork(actual, maximum int) error {
	if actual < 0 || actual > maximum {
		return errors.New("candidate group pagerank work limit")
	}
	return nil
}

func validateCandidateGroup(a CandidateGroupArtifact) error {
	b := a.Bounds
	profile := CandidateGroupProfile{a.ResourceProfileID, b, candidateGroupValidationPolicyID, candidateGroupValidationPolicyVersion}
	if a.SchemaVersion != CandidateGroupSchema || !validCandidateGroupProfile(profile) || a.ResourceProfileDigest != profile.Digest() || a.ArtifactID == "" || a.CommunityID == "" || sealCandidateGroup(a) != a.ArtifactID || a.Authority != 0 || a.Accepted || a.Completeness != CompletenessUnknown {
		return errors.New("candidate group identity mismatch")
	}
	if len(a.Members) == 0 || len(a.Members) > b.MaxMembersPerCommunity || len(a.Boundary.Communities) > b.MaxCommunities || a.Boundary.Accounting.AdmittedNodes > b.MaxDistinctMembers || len(a.InternalCalls)+len(a.CrossingCalls) > b.MaxCallOccurrences || len(a.Interpretation.Citations) > b.MaxCitations || len(a.Interpretation.Claims) > b.MaxDescriptions || !utf8.ValidString(a.Interpretation.Description) || len([]byte(a.Interpretation.Description)) > b.MaxDescriptionUTF8Bytes || a.Interpretation.Attempts < 0 || a.Interpretation.Attempts > b.MaxHostAttempts {
		return errors.New("candidate group invariant mismatch")
	}
	if err := validatePageRankWork(a.Boundary.PageRank.Work, b.MaxPageRankWork); err != nil {
		return err
	}
	if !validBoundaryArtifact(a) {
		return errors.New("candidate group boundary validation mismatch")
	}
	memberSet, citationSet, sourceSet := map[string]bool{}, map[string]bool{}, map[string]int{}
	graphSubjectSet := map[string]bool{}
	available, unavailable, ranges := 0, 0, 0
	for i, member := range a.Members {
		if member.ID == "" || member.Name == "" || memberSet[member.ID] || (i > 0 && a.Members[i-1].ID >= member.ID) {
			return errors.New("candidate group member invariant")
		}
		memberSet[member.ID] = true
		s := member.Source
		switch s.Status {
		case "CAPTURED":
			if !s.CustodyAvailable || s.Reason != "" || s.SourceID == "" || s.SourceByteLength <= 0 || s.SourceByteLength > b.MaxSourceBytesPerMember || s.Body == "" || s.UnitID == "" || s.CitationID == "" || len(s.Citations) != 1 || s.ItemRange == nil || s.SelectionRange == nil || s.Correspondence == nil || s.Custody.Custody != retainedprojection.RetainedCustody || s.DisplayRange.End.Line-s.DisplayRange.Start.Line != uint32(strings.Count(s.Body, "\n")) || !projectionContains(s.DisplayRange, s.EvidenceRange) || !projectionContains(s.DisplayRange, *s.ItemRange) || !projectionContains(*s.ItemRange, *s.SelectionRange) || !validCorrespondence(member, graphSubjectSet) {
				return errors.New("candidate group captured source invariant")
			}
			if citationSet[s.CitationID] {
				return errors.New("candidate group citation duplicate")
			}
			citationSet[s.CitationID], sourceSet[s.SourceID] = true, s.SourceByteLength
			available++
			ranges++
		case SourceUnavailable:
			if s.CustodyAvailable || s.Reason == "" || s.Body != "" || len(s.Citations) != 0 {
				return errors.New("candidate group unavailable source invariant")
			}
			unavailable++
		default:
			return errors.New("candidate group terminal source invariant")
		}
	}
	uniqueBytes := 0
	for _, n := range sourceSet {
		uniqueBytes += n
	}
	if a.SourceAccounting != (CandidateGroupSourceAccounting{AttemptedMembers: len(a.Members), TerminalMembers: len(a.Members), UnavailableMembers: unavailable, AvailableMembers: available, UniqueSourceBytes: uniqueBytes, SourceRanges: ranges, SourceObjects: len(sourceSet)}) || uniqueBytes > b.MaxTotalUniqueSourceBytes || ranges > b.MaxSourceRanges || len(sourceSet) > b.MaxSourceObjects {
		return errors.New("candidate group source accounting mismatch")
	}
	occurrences := map[string]bool{}
	checkCalls := func(calls []CandidateGroupCall, internal bool) error {
		for _, c := range calls {
			if c.OccurrenceID == "" || c.RelationID == "" || c.SourceNodeID == "" || c.TargetNodeID == "" || occurrences[c.OccurrenceID] {
				return errors.New("candidate group call invariant")
			}
			occurrences[c.OccurrenceID] = true
			insideSource, insideTarget := memberSet[c.SourceNodeID], memberSet[c.TargetNodeID]
			if internal != (insideSource && insideTarget) || (!internal && insideSource == insideTarget) {
				return errors.New("candidate group call boundary invariant")
			}
		}
		return nil
	}
	if err := checkCalls(a.InternalCalls, true); err != nil {
		return err
	}
	if err := checkCalls(a.CrossingCalls, false); err != nil {
		return err
	}
	selectedBoundary := (*programc.BoundaryCommunity)(nil)
	for i := range a.Boundary.Communities {
		if a.Boundary.Communities[i].CommunityID == a.CommunityID {
			selectedBoundary = &a.Boundary.Communities[i]
			break
		}
	}
	if selectedBoundary == nil || !reflect.DeepEqual(selectedBoundary.Members, a.MemberIDs()) || float64(len(a.InternalCalls)) != selectedBoundary.IntraOccurrences.Numerator || float64(len(a.CrossingCalls)) != selectedBoundary.CrossingOccurrences.Numerator || a.Boundary.Accounting.AccountedOccurrences != a.Boundary.Accounting.AdmittedOccurrences {
		return errors.New("candidate group boundary accounting invariant")
	}
	if !memberSet[a.Representative.ID] || a.Representative.Role != "NAVIGATION_ONLY" {
		return errors.New("candidate group representative invariant")
	}
	validationWork := len(a.Members) + len(occurrences) + ranges + len(a.Interpretation.Citations)
	if validationWork > b.MaxWorkUnits || validationWork > b.MaxPageRankWork {
		return errors.New("candidate group validation work limit")
	}
	switch a.Interpretation.Status {
	case InterpretationDescription:
		if available != len(a.Members) || a.Interpretation.HostID != "pi-host" || a.Interpretation.ModelID != "gpt-5.6-sol" || a.Interpretation.Attempts != 1 || a.Interpretation.Description == "" || len(a.Interpretation.Claims) != 1 || a.Interpretation.Claims[0] != a.Interpretation.Description || len(a.Interpretation.Citations) == 0 {
			return errors.New("candidate group description requires complete custody")
		}
		seen := map[string]bool{}
		covered := map[string]bool{}
		callByOccurrence := map[string]CandidateGroupCall{}
		for _, call := range a.InternalCalls {
			callByOccurrence[call.OccurrenceID] = call
		}
		for _, c := range a.Interpretation.Citations {
			if seen[c.OccurrenceID] || !validDescriptionCitation(c, a.Interpretation.Description, a.Members, citationSet, callByOccurrence) {
				return errors.New("candidate group interpretation citation coverage")
			}
			seen[c.OccurrenceID], covered[c.Claim] = true, true
		}
		if !covered[a.Interpretation.Description] || len(seen) != len(a.InternalCalls) {
			return errors.New("candidate group interpretation claim coverage")
		}
	case InterpretationUnresolved:
		if a.Interpretation.Description != "" || len(a.Interpretation.Citations) != 0 || len(a.Interpretation.Claims) != 0 {
			return errors.New("unresolved interpretation must be empty")
		}
	default:
		return errors.New("candidate group interpretation status")
	}
	return nil
}

func validDescriptionCitation(c Citation, description string, members []CandidateGroupMember, citationSet map[string]bool, calls map[string]CandidateGroupCall) bool {
	if !citationSet[c.SourceID] || c.LogicalSourceID == "" || c.Claim != description || !projectionRangeValid(c.Range) {
		return false
	}
	call, ok := calls[c.OccurrenceID]
	if !ok || c.RelationID != call.RelationID || c.CallerProgramCNodeID != call.SourceNodeID || c.CalleeProgramCNodeID != call.TargetNodeID || c.Range != projectionRange(call.CallSite) {
		return false
	}
	for i := range members {
		member := &members[i]
		s := &member.Source
		if member.ID == c.CallerProgramCNodeID && s.CitationID == c.SourceID {
			return s.Correspondence != nil && c.LogicalSourceID == s.Correspondence.URI && projectionContains(s.DisplayRange, c.Range)
		}
	}
	return false
}

func projectionRange(r graph.Range) sourceprojection.Range {
	return sourceprojection.Range{Start: sourceprojection.Position{Line: r.Start.Line, Character: r.Start.Character}, End: sourceprojection.Position{Line: r.End.Line, Character: r.End.Character}}
}

func validCorrespondence(member CandidateGroupMember, graphSubjectSet map[string]bool) bool {
	c := member.Source.Correspondence
	if c == nil || c.GraphSubjectID == "" || c.GraphSubjectID == c.ProgramCNodeID || c.V6EndpointGraphSubjectID == "" || c.V6EndpointGraphSubjectID != c.ProgramCNodeID || c.ProgramCNodeID != member.ID || c.SymbolName != member.Name || c.URI == "" || c.FixtureDigest == "" || c.DocumentDigest != member.Source.SourceID || c.SessionID == "" || c.Generation == 0 || c.DocumentVersion == "" || c.PositionEncoding == "" || c.SourceResponseIdentity == "" || graphSubjectSet[c.GraphSubjectID] || c.ItemRange != *member.Source.ItemRange || c.SelectionRange != *member.Source.SelectionRange {
		return false
	}
	graphSubjectSet[c.GraphSubjectID] = true
	return true
}

func gzipCanonicalInput(raw []byte) ([]byte, error) {
	var out bytes.Buffer
	w, err := gzip.NewWriterLevel(&out, gzip.BestCompression)
	if err != nil {
		return nil, err
	}
	w.Header.ModTime = time.Time{}
	w.Header.OS = 255
	if _, err = w.Write(raw); err != nil {
		return nil, err
	}
	if err = w.Close(); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func validBoundaryArtifact(a CandidateGroupArtifact) bool {
	r, err := gzip.NewReader(bytes.NewReader(a.BoundaryInputGZIP))
	if err != nil {
		return false
	}
	input, err := io.ReadAll(io.LimitReader(r, programc.MaxWorkerInputBytes+1))
	if closeErr := r.Close(); err != nil || closeErr != nil || int64(len(input)) > programc.MaxWorkerInputBytes {
		return false
	}
	outcome, failure := programc.Compute(input, a.Seed)
	if failure != nil || outcome.ProfileID != a.ProfileID || outcome.ProfileDigest != a.ProfileDigest || outcome.Algorithm != a.Algorithm || outcome.LogicalDigest != a.PartitionDigest {
		return false
	}
	recomputed, err := programc.ComputeBoundary(outcome, a.Boundary.Request)
	if err != nil || !reflect.DeepEqual(recomputed, a.Boundary) {
		return false
	}
	boundary := a.Boundary
	claimed := boundary.Digest
	boundary.Digest = ""
	raw, err := json.Marshal(boundary)
	if err != nil {
		return false
	}
	var canonical any
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if decoder.Decode(&canonical) != nil {
		return false
	}
	raw, err = json.Marshal(canonical)
	if err != nil {
		return false
	}
	h := sha256.New()
	h.Write([]byte(programc.BoundaryVersion + ":result"))
	h.Write([]byte{0})
	h.Write(raw)
	wantDigest := "sha256:" + hex.EncodeToString(h.Sum(nil))
	return claimed == wantDigest &&
		a.Boundary.SchemaVersion == programc.BoundaryVersion &&
		a.Boundary.Policy.ID == programc.BoundaryPolicyID && reflect.DeepEqual(a.Boundary.Policy, programc.BoundaryPolicy) &&
		a.Boundary.Bindings.ProfileID == a.ProfileID && a.Boundary.Bindings.ProfileSHA256 == a.ProfileDigest &&
		a.Boundary.Bindings.Algorithm == a.Algorithm && a.Boundary.Bindings.PartitionSHA256 == a.PartitionDigest &&
		a.Boundary.Bindings.PolicySHA256 == programc.BoundaryPolicyDigest() && a.Boundary.Bindings.Seed == a.Seed &&
		a.Boundary.ClaimCeiling == programc.BoundaryClaimCeiling
}

func projectionRangeValid(r sourceprojection.Range) bool {
	return r.Start.Line < r.End.Line || (r.Start.Line == r.End.Line && r.Start.Character <= r.End.Character)
}

// Citation containment is inclusive at both ends. A citation may equal the
// admitted evidence range or select a deterministic subrange within it.
func projectionContains(outer, inner sourceprojection.Range) bool {
	if !projectionRangeValid(outer) || !projectionRangeValid(inner) {
		return false
	}
	before := func(a, b sourceprojection.Position) bool {
		return a.Line < b.Line || (a.Line == b.Line && a.Character <= b.Character)
	}
	return before(outer.Start, inner.Start) && before(inner.End, outer.End)
}

func sealCandidateGroup(a CandidateGroupArtifact) string {
	a.ArtifactID = ""
	raw, _ := json.Marshal(a)
	h := sha256.New()
	h.Write([]byte(candidateGroupIdentityDomain))
	h.Write([]byte{0})
	h.Write(raw)
	return "sha256:" + hex.EncodeToString(h.Sum(nil))
}
