package censuscontinuation

import (
	"bytes"
	"compress/gzip"
	"context"
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

const (
	privateGroupingSubmissionSchema   = "lsp-trace.census-grouping-hypothesis-submission.private.v1"
	privateGroupingArtifactSchema     = "lsp-trace.census-grouping-hypothesis.private.v1"
	privateGroupingProfileID          = "census-grouping-hypothesis-private-v1"
	privateGroupingProfileDigestPin   = "sha256:d390f61a23d62fdc533b6ee32e78511f9c583699811a8f3b9a17c237a3c99841"
	privateGroupingProfileDomain      = "lsp-trace:census-grouping-hypothesis-profile:private:v1"
	privateGroupingArtifactDomain     = "lsp-trace:census-grouping-hypothesis-artifact:private:v1"
	privateGroupingGroupDomain        = "lsp-trace:census-grouping-hypothesis-group:private:v1"
	privateGroupingAlternativeDomain  = "lsp-trace:census-grouping-hypothesis-alternative:private:v1"
	privateGroupingSemantic           = "SEMANTIC_HYPOTHESIS"
	privateGroupingSingleton          = "MECHANICAL_SINGLETON"
	privateGroupingMaxBytes           = 1 << 20
	privateGroupingMaxGroups          = 1024
	privateGroupingMaxCandidates      = 4096
	privateGroupingMaxMembersPerGroup = 4096
	privateGroupingMaxMemberships     = 4096
	privateGroupingMaxAlternatives    = 4096
	privateGroupingMaxClientKeyBytes  = 256
	privateGroupingMaxSemanticBytes   = 4096
	privateGroupingMaxConditions      = 256
)

type privateGroupingSubmissionWire struct {
	SchemaVersion string                                 `json:"schema_version"`
	Groups        []privateGroupingSubmissionGroup       `json:"groups"`
	Alternatives  []privateGroupingSubmissionAlternative `json:"alternatives"`
}
type privateGroupingSubmissionGroup struct {
	ClientGroupKey      string   `json:"client_group_key"`
	Kind                string   `json:"kind"`
	CandidateIDs        []string `json:"candidate_ids"`
	Label               string   `json:"label"`
	Claim               string   `json:"claim"`
	Basis               string   `json:"basis"`
	RejectionConditions []string `json:"rejection_conditions"`
}
type privateGroupingSubmissionAlternative struct {
	LeftClientGroupKey  string `json:"left_client_group_key"`
	RightClientGroupKey string `json:"right_client_group_key"`
}
type privateGroupingBounds struct {
	MaxGroups                      int `json:"max_groups"`
	MaxCandidates                  int `json:"max_candidates"`
	MaxMembersPerGroup             int `json:"max_members_per_group"`
	MaxMemberships                 int `json:"max_memberships"`
	MaxAlternatives                int `json:"max_alternatives"`
	MaxClientGroupKeyUTF8Bytes     int `json:"max_client_group_key_utf8_bytes"`
	MaxSemanticUTF8Bytes           int `json:"max_semantic_utf8_bytes"`
	MaxRejectionConditionsPerGroup int `json:"max_rejection_conditions_per_group"`
	MaxArtifactBytes               int `json:"max_artifact_bytes"`
}
type privateGroupingSnapshot struct {
	Ordinal           int    `json:"ordinal"`
	CandidateID       string `json:"candidate_id"`
	DescriptionID     string `json:"description_id"`
	OutcomeDigest     string `json:"outcome_digest"`
	OutcomeByteLength uint64 `json:"outcome_byte_length"`
}
type privateGroupingGroup struct {
	GroupID             string   `json:"group_id"`
	ClientGroupKey      string   `json:"client_group_key"`
	Kind                string   `json:"kind"`
	CandidateIDs        []string `json:"candidate_ids"`
	Label               string   `json:"label"`
	Claim               string   `json:"claim"`
	Basis               string   `json:"basis"`
	RejectionConditions []string `json:"rejection_conditions"`
}
type privateGroupingAlternative struct {
	AlternativeID       string `json:"alternative_id"`
	LeftClientGroupKey  string `json:"left_client_group_key"`
	LeftGroupID         string `json:"left_group_id"`
	RightClientGroupKey string `json:"right_client_group_key"`
	RightGroupID        string `json:"right_group_id"`
}
type privateGroupingAccounting struct {
	CandidateCount                int `json:"candidate_count"`
	GroupCount                    int `json:"group_count"`
	SemanticGroupCount            int `json:"semantic_group_count"`
	MechanicalSingletonCount      int `json:"mechanical_singleton_count"`
	MembershipCount               int `json:"membership_count"`
	DistinctGroupedCandidateCount int `json:"distinct_grouped_candidate_count"`
	UngroupedCandidateCount       int `json:"ungrouped_candidate_count"`
	AlternativeCount              int `json:"alternative_count"`
}
type privateGroupingArtifactWire struct {
	SchemaVersion               string                       `json:"schema_version"`
	ArtifactID                  string                       `json:"artifact_id"`
	Status                      string                       `json:"status"`
	Authority                   int                          `json:"authority"`
	Accepted                    bool                         `json:"accepted"`
	Completeness                string                       `json:"completeness"`
	FeatureIdentity             string                       `json:"feature_identity"`
	SemanticQualification       string                       `json:"semantic_qualification"`
	AutomaticProgressionAllowed bool                         `json:"automatic_progression_allowed"`
	ReviewStatus                string                       `json:"review_status"`
	ChallengeStatus             string                       `json:"challenge_status"`
	AdjudicationStatus          string                       `json:"adjudication_status"`
	CandidateSelector           string                       `json:"candidate_selector"`
	CandidateDigest             string                       `json:"candidate_digest"`
	CandidateByteLength         uint64                       `json:"candidate_byte_length"`
	CandidateArtifactID         string                       `json:"candidate_artifact_id"`
	ProfileID                   string                       `json:"profile_id"`
	ProfileDigest               string                       `json:"profile_digest"`
	Bounds                      privateGroupingBounds        `json:"bounds"`
	CandidateSnapshots          []privateGroupingSnapshot    `json:"candidate_snapshots"`
	Groups                      []privateGroupingGroup       `json:"groups"`
	Alternatives                []privateGroupingAlternative `json:"alternatives"`
	UngroupedCandidateIDs       []string                     `json:"ungrouped_candidate_ids"`
	Accounting                  privateGroupingAccounting    `json:"accounting"`
}
type privateGroupingArtifact struct{ wire privateGroupingArtifactWire }

func (a privateGroupingArtifact) ID() string { return a.wire.ArtifactID }

func privateGroupingFrozenBounds() privateGroupingBounds {
	return privateGroupingBounds{
		MaxGroups:                      privateGroupingMaxGroups,
		MaxCandidates:                  privateGroupingMaxCandidates,
		MaxMembersPerGroup:             privateGroupingMaxMembersPerGroup,
		MaxMemberships:                 privateGroupingMaxMemberships,
		MaxAlternatives:                privateGroupingMaxAlternatives,
		MaxClientGroupKeyUTF8Bytes:     privateGroupingMaxClientKeyBytes,
		MaxSemanticUTF8Bytes:           privateGroupingMaxSemanticBytes,
		MaxRejectionConditionsPerGroup: privateGroupingMaxConditions,
		MaxArtifactBytes:               privateGroupingMaxBytes,
	}
}
func privateGroupingProfileDigest() string { return privateGroupingProfileDigestPin }
func privateGroupingComputedProfileDigest() string {
	return identityJSON(privateGroupingProfileDomain, privateGroupingFrozenBounds(), func(*privateGroupingBounds) {})
}

func validatePrivateGroupingGroup(clientKey, kind string, candidateIDs []string, label, claim, basis string, conditions []string, bounds privateGroupingBounds) error {
	if clientKey == "" || !utf8.ValidString(clientKey) || len([]byte(clientKey)) > bounds.MaxClientGroupKeyUTF8Bytes || candidateIDs == nil || len(candidateIDs) == 0 || len(candidateIDs) > bounds.MaxMembersPerGroup || conditions == nil {
		return errors.New("private grouping group shape")
	}
	for i, id := range candidateIDs {
		if !validDigest(id) || (i > 0 && candidateIDs[i-1] >= id) {
			return errors.New("private grouping candidate canonicality")
		}
	}
	for _, text := range []string{label, claim, basis} {
		if !utf8.ValidString(text) || len([]byte(text)) > bounds.MaxSemanticUTF8Bytes {
			return errors.New("private grouping semantic limit")
		}
	}
	switch kind {
	case privateGroupingSemantic:
		if label == "" || claim == "" || basis == "" || len(conditions) == 0 || len(conditions) > bounds.MaxRejectionConditionsPerGroup {
			return errors.New("private grouping semantic group invalid")
		}
		for i, condition := range conditions {
			if condition == "" || !utf8.ValidString(condition) || len([]byte(condition)) > bounds.MaxSemanticUTF8Bytes || (i > 0 && conditions[i-1] >= condition) {
				return errors.New("private grouping rejection canonicality")
			}
		}
	case privateGroupingSingleton:
		if len(candidateIDs) != 1 || label != "" || claim != "" || basis != "" || len(conditions) != 0 {
			return errors.New("private grouping singleton invalid")
		}
	default:
		return errors.New("private grouping kind invalid")
	}
	return nil
}

func privateGroupingPairOutOfOrder(previousLeft, previousRight, left, right string) bool {
	return previousLeft > left || (previousLeft == left && previousRight >= right)
}

func parsePrivateGroupingSubmission(raw []byte) (privateGroupingSubmissionWire, error) {
	bounds := privateGroupingFrozenBounds()
	if len(raw) == 0 || len(raw) > bounds.MaxArtifactBytes {
		return privateGroupingSubmissionWire{}, errors.New("private grouping submission byte limit")
	}
	var w privateGroupingSubmissionWire
	if err := strictCanonical(raw, &w); err != nil {
		return w, err
	}
	if w.SchemaVersion != privateGroupingSubmissionSchema || w.Groups == nil || w.Alternatives == nil || len(w.Groups) == 0 || len(w.Groups) > bounds.MaxGroups || len(w.Alternatives) > bounds.MaxAlternatives {
		return w, errors.New("private grouping submission invalid")
	}
	keys, semantic := map[string]bool{}, map[string]bool{}
	memberships := 0
	for i, g := range w.Groups {
		if keys[g.ClientGroupKey] || (i > 0 && w.Groups[i-1].ClientGroupKey >= g.ClientGroupKey) || validatePrivateGroupingGroup(g.ClientGroupKey, g.Kind, g.CandidateIDs, g.Label, g.Claim, g.Basis, g.RejectionConditions, bounds) != nil {
			return w, errors.New("private grouping group canonicality")
		}
		keys[g.ClientGroupKey] = true
		memberships += len(g.CandidateIDs)
		if memberships > bounds.MaxMemberships {
			return w, errors.New("private grouping membership limit")
		}
		semantic[g.ClientGroupKey] = g.Kind == privateGroupingSemantic
	}
	for i, alternative := range w.Alternatives {
		if alternative.LeftClientGroupKey >= alternative.RightClientGroupKey || !semantic[alternative.LeftClientGroupKey] || !semantic[alternative.RightClientGroupKey] || (i > 0 && privateGroupingPairOutOfOrder(w.Alternatives[i-1].LeftClientGroupKey, w.Alternatives[i-1].RightClientGroupKey, alternative.LeftClientGroupKey, alternative.RightClientGroupKey)) {
			return w, errors.New("private grouping alternative canonicality")
		}
	}
	return w, nil
}

func (a privateGroupingArtifact) Bytes() ([]byte, error) {
	if err := a.validate(); err != nil {
		return nil, err
	}
	raw, err := json.Marshal(a.wire)
	if err != nil || len(raw) > privateGroupingMaxBytes {
		return nil, errors.New("private grouping artifact byte limit")
	}
	return raw, nil
}
func parsePrivateGroupingArtifact(raw []byte) (privateGroupingArtifact, error) {
	if len(raw) > privateGroupingMaxBytes {
		return privateGroupingArtifact{}, errors.New("private grouping artifact byte limit")
	}
	var w privateGroupingArtifactWire
	if err := strictCanonical(raw, &w); err != nil {
		return privateGroupingArtifact{}, err
	}
	a := privateGroupingArtifact{w}
	return a, a.validate()
}
func (a privateGroupingArtifact) validate() error {
	w := a.wire
	if w.SchemaVersion != privateGroupingArtifactSchema || privateGroupingComputedProfileDigest() != privateGroupingProfileDigestPin || !validDigest(w.ArtifactID) || w.Status != "GROUPING_HYPOTHESIS_RETAINED" || w.Authority != 0 || w.Accepted || w.Completeness != CompletenessUnknown || w.FeatureIdentity != "UNRESOLVED" || w.SemanticQualification != "NOT_PERFORMED" || w.AutomaticProgressionAllowed || w.ReviewStatus != "NOT_PERFORMED" || w.ChallengeStatus != "NOT_PERFORMED" || w.AdjudicationStatus != "NOT_PERFORMED" || !validDigest(w.CandidateSelector) || w.CandidateDigest != w.CandidateSelector || w.CandidateByteLength == 0 || !validDigest(w.CandidateArtifactID) || w.ProfileID != privateGroupingProfileID || w.ProfileDigest != privateGroupingProfileDigest() || w.Bounds != privateGroupingFrozenBounds() || w.CandidateSnapshots == nil || w.Groups == nil || w.Alternatives == nil || w.UngroupedCandidateIDs == nil {
		return errors.New("private grouping artifact integrity")
	}
	if identityJSON(privateGroupingArtifactDomain, w, func(v *privateGroupingArtifactWire) { v.ArtifactID = "" }) != w.ArtifactID {
		return errors.New("private grouping artifact identity")
	}
	bounds := w.Bounds
	canonical, err := json.Marshal(w)
	if err != nil || len(canonical) > bounds.MaxArtifactBytes {
		return errors.New("private grouping artifact byte limit")
	}
	if len(w.CandidateSnapshots) == 0 || len(w.CandidateSnapshots) > bounds.MaxCandidates || len(w.Groups) == 0 || len(w.Groups) > bounds.MaxGroups || len(w.Alternatives) > bounds.MaxAlternatives {
		return errors.New("private grouping retained count limit")
	}
	candidateSet, descriptionSet, grouped := map[string]bool{}, map[string]bool{}, map[string]bool{}
	for i, snapshot := range w.CandidateSnapshots {
		if snapshot.Ordinal != i || !validDigest(snapshot.CandidateID) || !validDigest(snapshot.DescriptionID) || !validDigest(snapshot.OutcomeDigest) || snapshot.OutcomeByteLength == 0 || candidateSet[snapshot.CandidateID] || descriptionSet[snapshot.DescriptionID] {
			return errors.New("private grouping snapshot invalid")
		}
		candidateSet[snapshot.CandidateID] = true
		descriptionSet[snapshot.DescriptionID] = true
	}
	groupIDs, semantic := map[string]string{}, map[string]bool{}
	memberships := 0
	semanticCount, singletonCount := 0, 0
	for i, group := range w.Groups {
		if !validDigest(group.GroupID) || groupIDs[group.ClientGroupKey] != "" || (i > 0 && w.Groups[i-1].ClientGroupKey >= group.ClientGroupKey) || validatePrivateGroupingGroup(group.ClientGroupKey, group.Kind, group.CandidateIDs, group.Label, group.Claim, group.Basis, group.RejectionConditions, bounds) != nil {
			return errors.New("private grouping retained group invalid")
		}
		if identityJSON(privateGroupingGroupDomain, group, func(v *privateGroupingGroup) { v.GroupID = "" }) != group.GroupID {
			return errors.New("private grouping retained group identity invalid")
		}
		groupIDs[group.ClientGroupKey] = group.GroupID
		memberships += len(group.CandidateIDs)
		if memberships > bounds.MaxMemberships {
			return errors.New("private grouping retained membership limit")
		}
		for _, id := range group.CandidateIDs {
			if !candidateSet[id] {
				return errors.New("private grouping foreign candidate")
			}
			grouped[id] = true
		}
		semantic[group.ClientGroupKey] = group.Kind == privateGroupingSemantic
		if group.Kind == privateGroupingSemantic {
			semanticCount++
		} else {
			singletonCount++
		}
	}
	alternativeIDs := map[string]bool{}
	for i, alternative := range w.Alternatives {
		if !validDigest(alternative.AlternativeID) || alternativeIDs[alternative.AlternativeID] || alternative.LeftClientGroupKey >= alternative.RightClientGroupKey || !semantic[alternative.LeftClientGroupKey] || !semantic[alternative.RightClientGroupKey] || alternative.LeftGroupID != groupIDs[alternative.LeftClientGroupKey] || alternative.RightGroupID != groupIDs[alternative.RightClientGroupKey] || (i > 0 && privateGroupingPairOutOfOrder(w.Alternatives[i-1].LeftClientGroupKey, w.Alternatives[i-1].RightClientGroupKey, alternative.LeftClientGroupKey, alternative.RightClientGroupKey)) || identityJSON(privateGroupingAlternativeDomain, alternative, func(v *privateGroupingAlternative) { v.AlternativeID = "" }) != alternative.AlternativeID {
			return errors.New("private grouping retained alternative invalid")
		}
		alternativeIDs[alternative.AlternativeID] = true
	}
	ungrouped := make([]string, 0)
	for _, s := range w.CandidateSnapshots {
		if !grouped[s.CandidateID] {
			ungrouped = append(ungrouped, s.CandidateID)
		}
	}
	if !reflect.DeepEqual(ungrouped, w.UngroupedCandidateIDs) || w.Accounting != (privateGroupingAccounting{len(w.CandidateSnapshots), len(w.Groups), semanticCount, singletonCount, memberships, len(grouped), len(ungrouped), len(w.Alternatives)}) {
		return errors.New("private grouping accounting mismatch")
	}
	return nil
}

// RetainPrivateGroupingHypothesisSelector retains a caller-authored, authority-zero
// grouping hypothesis over the exact provisional candidate predecessor.
func RetainPrivateGroupingHypothesisSelector(ctx context.Context, store Store, candidateSelector string, submission []byte) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if store == nil || !validDigest(candidateSelector) {
		return "", errors.New("private grouping input invalid")
	}
	in, err := parsePrivateGroupingSubmission(submission)
	if err != nil {
		return "", err
	}
	candidateRaw, err := store.Get(ctx, candidateSelector)
	if err != nil || digestOf(candidateRaw) != candidateSelector {
		return "", errors.New("private grouping candidate selector mismatch")
	}
	predecessor, err := parsePrivateProvisionalCandidate(candidateRaw)
	if err != nil {
		return "", err
	}
	descriptionByID := map[string]privateProvisionalDescription{}
	for _, d := range predecessor.wire.Descriptions {
		descriptionByID[d.DescriptionID] = d
	}
	candidateByID := map[string]privateProvisionalCandidate{}
	for _, c := range predecessor.wire.Candidates {
		candidateByID[c.CandidateID] = c
	}
	snapshots := make([]privateGroupingSnapshot, 0, len(candidateByID))
	for ordinal, c := range predecessor.wire.Candidates {
		d := descriptionByID[c.DescriptionID]
		snapshots = append(snapshots, privateGroupingSnapshot{Ordinal: ordinal, CandidateID: c.CandidateID, DescriptionID: c.DescriptionID, OutcomeDigest: d.OutcomeDigest, OutcomeByteLength: d.OutcomeLength})
	}
	groups := make([]privateGroupingGroup, len(in.Groups))
	grouped := map[string]bool{}
	groupIDs := map[string]string{}
	for i, g := range in.Groups {
		for _, id := range g.CandidateIDs {
			if _, ok := candidateByID[id]; !ok {
				return "", errors.New("private grouping unknown candidate")
			}
			grouped[id] = true
		}
		row := privateGroupingGroup{"", g.ClientGroupKey, g.Kind, append([]string{}, g.CandidateIDs...), g.Label, g.Claim, g.Basis, append([]string{}, g.RejectionConditions...)}
		row.GroupID = identityJSON(privateGroupingGroupDomain, row, func(v *privateGroupingGroup) { v.GroupID = "" })
		groups[i] = row
		groupIDs[g.ClientGroupKey] = row.GroupID
	}
	alternatives := make([]privateGroupingAlternative, len(in.Alternatives))
	for i, p := range in.Alternatives {
		row := privateGroupingAlternative{"", p.LeftClientGroupKey, groupIDs[p.LeftClientGroupKey], p.RightClientGroupKey, groupIDs[p.RightClientGroupKey]}
		row.AlternativeID = identityJSON(privateGroupingAlternativeDomain, row, func(v *privateGroupingAlternative) { v.AlternativeID = "" })
		alternatives[i] = row
	}
	ungrouped := make([]string, 0)
	for _, s := range snapshots {
		if !grouped[s.CandidateID] {
			ungrouped = append(ungrouped, s.CandidateID)
		}
	}
	semantic, singletons, memberships := 0, 0, 0
	for _, g := range groups {
		memberships += len(g.CandidateIDs)
		if g.Kind == privateGroupingSemantic {
			semantic++
		} else {
			singletons++
		}
	}
	w := privateGroupingArtifactWire{SchemaVersion: privateGroupingArtifactSchema, Status: "GROUPING_HYPOTHESIS_RETAINED", Completeness: CompletenessUnknown, FeatureIdentity: "UNRESOLVED", SemanticQualification: "NOT_PERFORMED", ReviewStatus: "NOT_PERFORMED", ChallengeStatus: "NOT_PERFORMED", AdjudicationStatus: "NOT_PERFORMED", CandidateSelector: candidateSelector, CandidateDigest: candidateSelector, CandidateByteLength: uint64(len(candidateRaw)), CandidateArtifactID: predecessor.ID(), ProfileID: privateGroupingProfileID, ProfileDigest: privateGroupingProfileDigest(), Bounds: privateGroupingFrozenBounds(), CandidateSnapshots: snapshots, Groups: groups, Alternatives: alternatives, UngroupedCandidateIDs: ungrouped, Accounting: privateGroupingAccounting{len(snapshots), len(groups), semantic, singletons, memberships, len(grouped), len(ungrouped), len(alternatives)}}
	w.ArtifactID = identityJSON(privateGroupingArtifactDomain, w, func(v *privateGroupingArtifactWire) { v.ArtifactID = "" })
	artifact := privateGroupingArtifact{w}
	raw, err := artifact.Bytes()
	if err != nil {
		return "", err
	}
	parsed, err := parsePrivateGroupingArtifact(raw)
	if err != nil || parsed.ID() != artifact.ID() {
		return "", errors.New("private grouping self validation")
	}
	again, getErr := store.Get(ctx, candidateSelector)
	if getErr != nil || !bytes.Equal(again, candidateRaw) {
		return "", errors.New("private grouping predecessor changed")
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	selector, err := store.Put(ctx, raw)
	if err != nil {
		return "", err
	}
	if selector != digestOf(raw) {
		return "", errors.New("private grouping store selector mismatch")
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	readback, err := store.Get(ctx, selector)
	if err != nil {
		return "", err
	}
	readArtifact, err := parsePrivateGroupingArtifact(readback)
	if err != nil || !bytes.Equal(readback, raw) || digestOf(readback) != selector || readArtifact.ID() != artifact.ID() {
		return "", errors.New("private grouping readback mismatch")
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	return selector, nil
}
