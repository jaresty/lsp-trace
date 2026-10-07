package censuscontinuation

import (
	"errors"
	"net/url"
	"path"
	"reflect"
	"sort"

	"lsp-trace/internal/censusprogramc"
	"lsp-trace/internal/graph"
	"lsp-trace/internal/programc"
	"lsp-trace/internal/programcpresentation"
	"lsp-trace/internal/transientstructural"
	"lsp-trace/internal/v5sourcesnapshotv3"
)

// admittedCommunityCheckpoint is deliberately private. Its selected membership
// is derived from, and revalidated against, an admitted Program C result.
type admittedCommunityCheckpoint struct {
	communityID, graphDigest, partitionDigest, profileID, profileDigest string
	seed                                                                uint64
	members                                                             []string
	capture                                                             CaptureResult
	authority                                                           int
	completeness                                                        string
}

func (c admittedCommunityCheckpoint) memberIDs() []string { return append([]string(nil), c.members...) }

type managedMemberStructural struct {
	NodeID  string
	URI     string
	Request transientstructural.Request
}

func managedMemberStructuralRequest(sessionID string, generation uint64, target programcpresentation.MachineTarget) (managedMemberStructural, error) {
	parsed, err := url.Parse(target.URI)
	if err != nil || parsed.Scheme != "file" || parsed.Host != "" || parsed.RawQuery != "" || parsed.Fragment != "" || parsed.Path == "" || path.Clean(parsed.Path) != parsed.Path || parsed.String() != target.URI || target.Line < 0 || target.Character < 0 || target.PositionEncoding != "utf-16" || target.CoordinateBase != 0 || target.RangeRole != "SELECTION_RANGE" || target.NodeID == "" || target.NodeKind < 1 || target.NodeKind > 26 {
		return managedMemberStructural{}, errors.New("private community machine target invalid")
	}
	line, character := uint32(target.Line), uint32(target.Character)
	request := transientstructural.Request{
		SessionID: sessionID, Generation: generation, Target: transientstructural.Target{URI: target.URI, Line: &line, Character: &character},
		UpDepth: 1, DownDepth: 1, MaxNodes: 100, TimeoutMS: 30000, RequestTimeoutMS: 15000, MaxMessages: 64, MaxBytes: 4 << 20,
		Analysis: transientstructural.AnalysisRequest{Kind: transientstructural.AnalysisNeighborhood},
	}
	if target.NodeKind != 6 && target.NodeKind != 9 && target.NodeKind != 12 {
		request.UpDepth, request.DownDepth, request.SourceOnlyTarget, request.CaptureSupply = 0, 0, true, true
	}
	return managedMemberStructural{NodeID: target.NodeID, URI: target.URI, Request: request}, nil
}

func captureAdmittedCommunity(h CommittedHandoff, admitted censusprogramc.Result, boundary programc.BoundaryArtifact, communityID, workspace, encoding string, limits v5sourcesnapshotv3.Limits, deps FreshCaptureDependencies) (admittedCommunityCheckpoint, error) {
	recomputed, err := ReconstructProgramC(h, admitted.Outcome.Seed)
	if err != nil || !reflect.DeepEqual(recomputed, admitted) {
		return admittedCommunityCheckpoint{}, errors.New("private community checkpoint admission mismatch")
	}
	recomputedBoundary, err := programc.ComputeBoundary(admitted.Outcome, boundary.Request)
	if err != nil || !reflect.DeepEqual(recomputedBoundary, boundary) {
		return admittedCommunityCheckpoint{}, errors.New("private community checkpoint boundary mismatch")
	}
	var selected *programc.BoundaryCommunity
	for i := range boundary.Communities {
		if boundary.Communities[i].CommunityID == communityID {
			selected = &boundary.Communities[i]
			break
		}
	}
	if selected == nil || len(selected.Members) == 0 {
		return admittedCommunityCheckpoint{}, errors.New("private community checkpoint foreign community")
	}

	projection, err := h.Projection()
	if err != nil {
		return admittedCommunityCheckpoint{}, err
	}
	type located struct {
		ordinal   int
		node      graph.Node
		relations map[string]bool
	}
	locations := map[string][]located{}
	for ordinal, constituent := range projection.Constituents {
		native, _, decodeErr := decodeNative(constituent.Raw)
		if decodeErr != nil {
			return admittedCommunityCheckpoint{}, decodeErr
		}
		relations := make(map[string]bool, len(native.Edges))
		for _, edge := range native.Edges {
			relations[edge.RelationID] = true
		}
		for _, node := range native.Nodes {
			locations[node.ID] = append(locations[node.ID], located{ordinal: ordinal, node: node, relations: relations})
		}
	}

	nominations := make([]censusprogramc.Representative, 0, len(selected.Members))
	for _, member := range selected.Members {
		where := locations[member]
		if len(where) == 0 {
			return admittedCommunityCheckpoint{}, errors.New("private community checkpoint member missing")
		}
		incoming := []censusprogramc.RepresentativePredecessor{}
		for _, occurrence := range admitted.Outcome.Projection.Occurrences {
			if occurrence.To < 0 || int(occurrence.To) >= len(admitted.Outcome.Projection.NodeIdentities) || occurrence.From < 0 || int(occurrence.From) >= len(admitted.Outcome.Projection.NodeIdentities) {
				return admittedCommunityCheckpoint{}, errors.New("private community checkpoint occurrence index")
			}
			if admitted.Outcome.Projection.NodeIdentities[occurrence.To] == member {
				incoming = append(incoming, censusprogramc.RepresentativePredecessor{OccurrenceID: occurrence.Identity, RelationID: occurrence.RelationID, CallerID: admitted.Outcome.Projection.NodeIdentities[occurrence.From], TargetID: member, CallSite: occurrence.CallSite})
			}
		}
		sort.Slice(incoming, func(i, j int) bool { return incoming[i].OccurrenceID < incoming[j].OccurrenceID })
		owner := -1
		for _, candidate := range where {
			ownsAll := true
			for _, predecessor := range incoming {
				if !candidate.relations[predecessor.RelationID] {
					ownsAll = false
					break
				}
			}
			if ownsAll {
				owner = candidate.ordinal
				break
			}
		}
		if owner < 0 {
			return admittedCommunityCheckpoint{}, errors.New("private community checkpoint member occurrence ownership ambiguous")
		}
		constituent := admitted.Admission.Artifact.Constituents[owner]
		nominations = append(nominations, censusprogramc.Representative{Status: censusprogramc.CandidateStatus, ClaimCeiling: admitted.Outcome.ClaimCeiling, SelectionState: "PRIVATE_ADMITTED_COMMUNITY_MEMBER", Members: append([]string(nil), selected.Members...), CensusID: admitted.CensusID, CommunityIdentity: communityID, ConstituentIdentity: constituent.Identity, ConstituentOrdinal: owner, SelectedNode: member, Authority: 0, SourceGraphComplete: CompletenessUnknown, IncomingPredecessors: incoming})
	}
	derived := admitted
	derived.Representatives = censusprogramc.RepresentativeSelection{State: "PRIVATE_ADMITTED_COMMUNITY", Nominations: nominations}
	capture, err := CaptureSnapshots(h, derived, workspace, encoding, limits, deps)
	if err != nil {
		return admittedCommunityCheckpoint{}, err
	}
	return admittedCommunityCheckpoint{communityID: communityID, graphDigest: admitted.Outcome.Source.GraphV5SHA256, partitionDigest: admitted.Outcome.LogicalDigest, profileID: admitted.Outcome.ProfileID, profileDigest: admitted.Outcome.ProfileDigest, seed: admitted.Outcome.Seed, members: append([]string(nil), selected.Members...), capture: capture, authority: 0, completeness: CompletenessUnknown}, nil
}
