// Package programcpresentation builds a transport-neutral, communities-first
// view of exact Program C evidence. It never reads source files.
package programcpresentation

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"sort"
	"strings"

	"lsp-trace/internal/graph"
	"lsp-trace/internal/programc"
	"lsp-trace/internal/schema"
)

const Version = "lsp-trace.community-presentation.v1"
const Authority = "NON_AUTHORITATIVE_DERIVED_VIEW"
const Disclaimer = "Structural communities do not establish feature identity, ownership, architecture, runtime execution, whole-source completeness, producer authentication, permission, or production authority."
const Stability = "Cross-seed stability is unassessed until A-08 exists."

type Location struct {
	URI            string `json:"uri"`
	StartLine      int    `json:"start_line"`
	StartCharacter int    `json:"start_character"`
	EndLine        int    `json:"end_line"`
	EndCharacter   int    `json:"end_character"`
}

func (l Location) MarshalJSON() ([]byte, error) {
	type x struct {
		URI            string `json:"uri"`
		StartLine      int    `json:"start_line"`
		StartCharacter int    `json:"start_character"`
		EndLine        int    `json:"end_line"`
		EndCharacter   int    `json:"end_character"`
	}
	return json.Marshal(x(l))
}

type Node struct {
	NodeID          string   `json:"node_id"`
	Name            string   `json:"name"`
	EnclosingDetail string   `json:"enclosing_detail,omitempty"`
	Location        Location `json:"location"`
}

func (n Node) MarshalJSON() ([]byte, error) {
	type x struct {
		NodeID          string   `json:"node_id"`
		Name            string   `json:"name"`
		EnclosingDetail string   `json:"enclosing_detail,omitempty"`
		Location        Location `json:"location"`
	}
	return json.Marshal(x(n))
}

type Community struct {
	CommunityID string                     `json:"community_id"`
	Members     []Node                     `json:"members"`
	Evidence    programc.BoundaryCommunity `json:"evidence"`
}
type Call struct {
	OccurrenceID string   `json:"occurrence_id"`
	Caller       Node     `json:"caller"`
	Callee       Node     `json:"callee"`
	CallSite     Location `json:"call_site"`
}

func (c Call) MarshalJSON() ([]byte, error) {
	type x struct {
		OccurrenceID string   `json:"occurrence_id"`
		Caller       Node     `json:"caller"`
		Callee       Node     `json:"callee"`
		CallSite     Location `json:"call_site"`
	}
	return json.Marshal(x{c.OccurrenceID, c.Caller, c.Callee, c.CallSite})
}

type Artifact struct {
	SchemaVersion               string                       `json:"schema_version"`
	Authority                   string                       `json:"authority"`
	Outcome                     string                       `json:"outcome"`
	Completeness                programc.SourceCompleteness  `json:"completeness"`
	Seed                        uint64                       `json:"seed"`
	ProfileID                   string                       `json:"profile_id"`
	ProfileSHA256               string                       `json:"profile_sha256"`
	Algorithm                   string                       `json:"algorithm"`
	PartitionSHA256             string                       `json:"partition_sha256"`
	ClaimCeiling                string                       `json:"claim_ceiling"`
	Disclaimer                  string                       `json:"disclaimer"`
	CrossSeedStability          string                       `json:"cross_seed_stability"`
	Policy                      programc.BoundaryPolicySpec  `json:"policy"`
	Request                     programc.BoundaryRequest     `json:"request"`
	Accounting                  programc.BoundaryAccounting  `json:"accounting"`
	PageRank                    programc.BoundaryPageRank    `json:"pagerank"`
	Communities                 []Community                  `json:"communities"`
	CrossCommunityCalls         []Call                       `json:"cross_community_calls"`
	HighCentralityCrossingNodes []programc.BoundaryNodeScore `json:"high_centrality_crossing_nodes"`
	HubCrossingNodes            []programc.BoundaryNodeScore `json:"hub_crossing_nodes"`
	CrossingWitnesses           []programc.CrossingWitness   `json:"crossing_witnesses"`
	Bridges                     []string                     `json:"bridges"`
	ArticulationPoints          []string                     `json:"articulation_points"`
}

func (a Artifact) MarshalJSON() ([]byte, error) {
	type wire struct {
		SchemaVersion       string                       `json:"schema_version"`
		Authority           string                       `json:"authority"`
		Outcome             string                       `json:"outcome"`
		Completeness        programc.SourceCompleteness  `json:"completeness"`
		Seed                uint64                       `json:"seed"`
		ProfileID           string                       `json:"profile_id"`
		ProfileSHA256       string                       `json:"profile_sha256"`
		Algorithm           string                       `json:"algorithm"`
		PartitionSHA256     string                       `json:"partition_sha256"`
		Policy              programc.BoundaryPolicySpec  `json:"policy"`
		Request             programc.BoundaryRequest     `json:"request"`
		Accounting          programc.BoundaryAccounting  `json:"accounting"`
		Communities         []Community                  `json:"communities"`
		CrossCommunityCalls []Call                       `json:"cross_community_calls"`
		PageRank            programc.BoundaryPageRank    `json:"pagerank"`
		High                []programc.BoundaryNodeScore `json:"high_centrality_crossing_nodes"`
		Hubs                []programc.BoundaryNodeScore `json:"hub_crossing_nodes"`
		Witnesses           []programc.CrossingWitness   `json:"crossing_witnesses"`
		Bridges             []string                     `json:"bridges"`
		Articulation        []string                     `json:"articulation_points"`
		ClaimCeiling        string                       `json:"claim_ceiling"`
		Disclaimer          string                       `json:"disclaimer"`
		Stability           string                       `json:"cross_seed_stability"`
	}
	return json.Marshal(wire{a.SchemaVersion, a.Authority, a.Outcome, a.Completeness, a.Seed, a.ProfileID, a.ProfileSHA256, a.Algorithm, a.PartitionSHA256, a.Policy, a.Request, a.Accounting, a.Communities, a.CrossCommunityCalls, a.PageRank, a.HighCentralityCrossingNodes, a.HubCrossingNodes, a.CrossingWitnesses, a.Bridges, a.ArticulationPoints, a.ClaimCeiling, a.Disclaimer, a.CrossSeedStability})
}

type native struct {
	Nodes []graph.Node `json:"nodes"`
}

func loc(uri string, r graph.Range) Location {
	return Location{uri, int(r.Start.Line) + 1, int(r.Start.Character) + 1, int(r.End.Line) + 1, int(r.End.Character) + 1}
}
func Build(o programc.Outcome, b programc.BoundaryArtifact) (Artifact, error) {
	canonicalOutcome, failure := programc.Compute(o.Source.InputBytes(), o.Seed)
	if failure != nil {
		return Artifact{}, fmt.Errorf("outcome source revalidation: %w", failure)
	}
	if !reflect.DeepEqual(o, canonicalOutcome) {
		return Artifact{}, fmt.Errorf("outcome semantic validation mismatch")
	}
	canonicalBoundary, err := programc.ComputeBoundary(canonicalOutcome, b.Request)
	if err != nil {
		return Artifact{}, fmt.Errorf("boundary recomputation: %w", err)
	}
	if !reflect.DeepEqual(b, canonicalBoundary) {
		return Artifact{}, fmt.Errorf("boundary semantic validation mismatch")
	}
	var d native
	dec := json.NewDecoder(bytes.NewReader(o.Source.GraphV5Bytes()))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&d); err != nil { // Graph V5 has other fields, so decode normally after proving one value.
		if err := json.Unmarshal(o.Source.GraphV5Bytes(), &d); err != nil {
			return Artifact{}, fmt.Errorf("embedded graph_v5: %w", err)
		}
	}
	nodes := map[string]graph.Node{}
	for _, n := range d.Nodes {
		if _, ok := nodes[n.ID]; ok {
			return Artifact{}, fmt.Errorf("duplicate graph node %q", n.ID)
		}
		nodes[n.ID] = n
	}
	if len(nodes) != len(o.Projection.NodeIdentities) {
		return Artifact{}, fmt.Errorf("graph/projection node count mismatch")
	}
	for _, id := range o.Projection.NodeIdentities {
		if _, ok := nodes[id]; !ok {
			return Artifact{}, fmt.Errorf("projection foreign node %q", id)
		}
	}
	bc := map[string]programc.BoundaryCommunity{}
	for _, c := range b.Communities {
		if _, ok := bc[c.CommunityID]; ok {
			return Artifact{}, fmt.Errorf("duplicate boundary community %q", c.CommunityID)
		}
		bc[c.CommunityID] = c
	}
	memberCommunity := map[string]string{}
	cs := make([]Community, 0, len(o.Communities))
	for _, c := range o.Communities {
		ids := append([]string(nil), c.Members...)
		if !sort.StringsAreSorted(ids) {
			return Artifact{}, fmt.Errorf("noncanonical member order")
		}
		var match *programc.BoundaryCommunity
		for _, x := range b.Communities {
			if equal(ids, x.Members) {
				y := x
				match = &y
				break
			}
		}
		if match == nil {
			return Artifact{}, fmt.Errorf("missing boundary community")
		}
		ms := make([]Node, 0, len(ids))
		for _, id := range ids {
			n, ok := nodes[id]
			if !ok {
				return Artifact{}, fmt.Errorf("foreign member %q", id)
			}
			if _, ok := memberCommunity[id]; ok {
				return Artifact{}, fmt.Errorf("duplicate member %q", id)
			}
			memberCommunity[id] = match.CommunityID
			ms = append(ms, Node{id, n.Name, n.Detail, loc(n.URI, n.Range)})
		}
		cs = append(cs, Community{match.CommunityID, ms, *match})
	}
	if len(cs) != len(b.Communities) || len(memberCommunity) != len(nodes) {
		return Artifact{}, fmt.Errorf("incomplete community join")
	}
	calls := []Call{}
	for _, e := range o.Projection.Occurrences {
		from, to := o.Projection.NodeIdentities[e.From], o.Projection.NodeIdentities[e.To]
		if memberCommunity[from] == memberCommunity[to] {
			continue
		}
		fn, tn := nodes[from], nodes[to]
		calls = append(calls, Call{e.Identity, Node{from, fn.Name, fn.Detail, loc(fn.URI, fn.Range)}, Node{to, tn.Name, tn.Detail, loc(tn.URI, tn.Range)}, loc(fn.URI, e.CallSite)})
	}
	a := Artifact{Version, Authority, b.Outcome, o.Source.Completeness, o.Seed, o.ProfileID, o.ProfileDigest, o.Algorithm, o.LogicalDigest, b.ClaimCeiling, Disclaimer, Stability, b.Policy, b.Request, b.Accounting, b.PageRank, cs, calls, append([]programc.BoundaryNodeScore(nil), b.HighCentralityCrossingNodes...), append([]programc.BoundaryNodeScore(nil), b.HubCrossingNodes...), append([]programc.CrossingWitness(nil), b.CrossingWitnesses...), append([]string(nil), b.Bridges...), append([]string(nil), b.ArticulationPoints...)}
	if err := Validate(a); err != nil {
		return Artifact{}, err
	}
	return a, nil
}
func compareIDs(a, b []string) int {
	for i := 0; i < len(a) && i < len(b); i++ {
		if a[i] < b[i] {
			return -1
		}
		if a[i] > b[i] {
			return 1
		}
	}
	if len(a) < len(b) {
		return -1
	}
	if len(a) > len(b) {
		return 1
	}
	return 0
}
func equal(a, b []string) bool {
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
func Validate(a Artifact) error {
	if a.SchemaVersion != Version || a.Authority != Authority || a.Disclaimer != Disclaimer || a.CrossSeedStability != Stability || a.ProfileID != programc.ProfileID || a.ProfileSHA256 != programc.ProfileDigest || a.ClaimCeiling != programc.BoundaryClaimCeiling {
		return fmt.Errorf("invalid presentation constants")
	}
	if a.Outcome != "COMPLETE" && a.Outcome != "EMPTY" && a.Outcome != "INCOMPLETE" && a.Outcome != "UNAVAILABLE" {
		return fmt.Errorf("invalid outcome")
	}
	if !reflect.DeepEqual(a.Policy, programc.BoundaryPolicy) || a.Request.PageRankTopK < 1 || a.Request.HubTopK < 1 || len(a.Communities) > programc.MaxNodes || len(a.CrossCommunityCalls) > programc.MaxOccurrences {
		return fmt.Errorf("invalid policy, request, or limit")
	}
	nodes := map[string]Node{}
	communities := map[string]string{}
	var previous []string
	for _, c := range a.Communities {
		if c.CommunityID == "" || c.Evidence.CommunityID != c.CommunityID {
			return fmt.Errorf("community evidence identity mismatch")
		}
		current := make([]string, len(c.Members))
		for i := range c.Members {
			current[i] = c.Members[i].NodeID
		}
		if !equal(current, c.Evidence.Members) {
			return fmt.Errorf("community evidence members mismatch")
		}
		if previous != nil && compareIDs(previous, current) >= 0 {
			return fmt.Errorf("noncanonical community order")
		}
		previous = current
		p := ""
		for _, n := range c.Members {
			if n.NodeID == "" || (p != "" && n.NodeID <= p) {
				return fmt.Errorf("noncanonical member order")
			}
			if _, ok := nodes[n.NodeID]; ok {
				return fmt.Errorf("duplicate member")
			}
			if err := validateLocation(n.Location); err != nil {
				return err
			}
			nodes[n.NodeID] = n
			communities[n.NodeID] = c.CommunityID
			p = n.NodeID
		}
	}
	occurrences := map[string]Call{}
	previousOccurrence := ""
	for _, c := range a.CrossCommunityCalls {
		if c.OccurrenceID == "" || (previousOccurrence != "" && c.OccurrenceID <= previousOccurrence) {
			return fmt.Errorf("noncanonical or duplicate call order")
		}
		caller, callerOK := nodes[c.Caller.NodeID]
		callee, calleeOK := nodes[c.Callee.NodeID]
		if !callerOK || !calleeOK || !reflect.DeepEqual(caller, c.Caller) || !reflect.DeepEqual(callee, c.Callee) || communities[c.Caller.NodeID] == communities[c.Callee.NodeID] {
			return fmt.Errorf("invalid cross-community call join")
		}
		if err := validateLocation(c.CallSite); err != nil {
			return err
		}
		occurrences[c.OccurrenceID] = c
		previousOccurrence = c.OccurrenceID
	}
	if err := validateScores(a.HighCentralityCrossingNodes, nodes); err != nil {
		return fmt.Errorf("pagerank scores: %w", err)
	}
	if err := validateScores(a.HubCrossingNodes, nodes); err != nil {
		return fmt.Errorf("hub scores: %w", err)
	}
	if err := validateReferences(a.Bridges, func(id string) bool { return id != "" }); err != nil {
		return fmt.Errorf("bridges: %w", err)
	}
	if err := validateReferences(a.ArticulationPoints, func(id string) bool { _, ok := nodes[id]; return ok }); err != nil {
		return fmt.Errorf("articulation points: %w", err)
	}
	seenWitness := map[string]bool{}
	for _, w := range a.CrossingWitnesses {
		call, ok := occurrences[w.OccurrenceID]
		sourceCommunity, targetCommunity := communities[w.SourceNodeID], communities[w.TargetNodeID]
		communityPairMatches := (sourceCommunity == w.CommunityA && targetCommunity == w.CommunityB) || (sourceCommunity == w.CommunityB && targetCommunity == w.CommunityA)
		if !ok || seenWitness[w.OccurrenceID] || call.Caller.NodeID != w.SourceNodeID || call.Callee.NodeID != w.TargetNodeID || !communityPairMatches || w.CommunityA == w.CommunityB {
			return fmt.Errorf("invalid crossing witness join")
		}
		seenWitness[w.OccurrenceID] = true
	}
	return nil
}

func validateLocation(l Location) error {
	if l.URI == "" || l.StartLine < 1 || l.StartCharacter < 1 || l.EndLine < 1 || l.EndCharacter < 1 || l.EndLine < l.StartLine || (l.EndLine == l.StartLine && l.EndCharacter < l.StartCharacter) {
		return fmt.Errorf("invalid one-based location")
	}
	return nil
}

func validateScores(scores []programc.BoundaryNodeScore, nodes map[string]Node) error {
	seen := map[string]bool{}
	for i, score := range scores {
		if _, ok := nodes[score.NodeID]; !ok || seen[score.NodeID] || (i > 0 && (scores[i-1].Score < score.Score || (scores[i-1].Score == score.Score && scores[i-1].NodeID >= score.NodeID))) {
			return fmt.Errorf("foreign, duplicate, or noncanonical node score")
		}
		seen[score.NodeID] = true
	}
	return nil
}

func validateReferences(ids []string, exists func(string) bool) error {
	for i, id := range ids {
		if !exists(id) || (i > 0 && ids[i-1] >= id) {
			return fmt.Errorf("foreign, duplicate, or noncanonical reference")
		}
	}
	return nil
}
func ValidateJSON(raw []byte) error {
	if _, err := schema.ValidateFor(raw, schema.FamilyCommunityPresentation, "v1"); err != nil {
		return err
	}
	var a Artifact
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&a); err != nil {
		return err
	}
	if dec.Decode(&struct{}{}) != io.EOF {
		return fmt.Errorf("presentation must contain exactly one JSON document")
	}
	return Validate(a)
}

func JSON(a Artifact) ([]byte, error) {
	if err := Validate(a); err != nil {
		return nil, err
	}
	b, err := json.Marshal(a)
	if err == nil {
		b = append(b, '\n')
	}
	return b, err
}
func Text(w io.Writer, a Artifact) error {
	if err := Validate(a); err != nil {
		return err
	}
	fmt.Fprintf(w, "%s\nauthority: %s\noutcome: %s\ncompleteness: traversal=%t source=%s scope=%s truncated=%t\nseed: %d\nprofile: %s %s\nalgorithm: %s\npolicy: %s\n%s\n%s\n\nCOMMUNITIES\n", a.SchemaVersion, a.Authority, a.Outcome, a.Completeness.TraversalComplete, a.Completeness.SourceGraphComplete, a.Completeness.CompletenessScope, a.Completeness.Truncated, a.Seed, a.ProfileID, a.ProfileSHA256, a.Algorithm, a.Policy.ID, a.Disclaimer, a.CrossSeedStability)
	for _, c := range a.Communities {
		fmt.Fprintf(w, "\ncommunity %s\n", c.CommunityID)
		for _, n := range c.Members {
			fmt.Fprintf(w, "  %s — %s", n.NodeID, n.Name)
			if n.EnclosingDetail != "" {
				fmt.Fprintf(w, " (enclosing/detail: %s)", n.EnclosingDetail)
			}
			fmt.Fprintf(w, " @ %s:%d:%d-%d:%d\n", n.Location.URI, n.Location.StartLine, n.Location.StartCharacter, n.Location.EndLine, n.Location.EndCharacter)
		}
	}
	fmt.Fprintln(w, "\nCROSS-COMMUNITY CALLS")
	for _, c := range a.CrossCommunityCalls {
		fmt.Fprintf(w, "  %s -> %s occurrence=%s call-site=%d:%d-%d:%d\n", c.Caller.Name, c.Callee.Name, c.OccurrenceID, c.CallSite.StartLine, c.CallSite.StartCharacter, c.CallSite.EndLine, c.CallSite.EndCharacter)
	}
	fmt.Fprintln(w, "\nCONDUCTANCE")
	for _, c := range a.Communities {
		fmt.Fprintf(w, "  %s outcome=%s value=%g numerator=%g denominator=%g\n", c.CommunityID, c.Evidence.Conductance.Outcome, c.Evidence.Conductance.Value, c.Evidence.Conductance.Numerator, c.Evidence.Conductance.Denominator)
	}
	fmt.Fprintf(w, "\nPAGERANK status=%s iterations=%d work=%d\n", a.PageRank.Status, a.PageRank.Iterations, a.PageRank.Work)
	writeScores(w, "PAGERANK CROSSING NODES", a.HighCentralityCrossingNodes)
	writeScores(w, "HUBS", a.HubCrossingNodes)
	fmt.Fprintln(w, "\nCANONICAL WITNESSES")
	for _, x := range a.CrossingWitnesses {
		fmt.Fprintf(w, "  %s -> %s occurrence=%s communities=%s,%s\n", x.SourceNodeID, x.TargetNodeID, x.OccurrenceID, x.CommunityA, x.CommunityB)
	}
	fmt.Fprintf(w, "\nBRIDGES\n  %s\n\nARTICULATION POINTS\n  %s\n", strings.Join(a.Bridges, "\n  "), strings.Join(a.ArticulationPoints, "\n  "))
	return nil
}
func writeScores(w io.Writer, label string, x []programc.BoundaryNodeScore) {
	fmt.Fprintf(w, "\n%s\n", label)
	for _, s := range x {
		fmt.Fprintf(w, "  %s score=%g\n", s.NodeID, s.Score)
	}
}
