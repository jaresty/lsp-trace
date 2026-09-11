// Package programcpresentation builds a transport-neutral, communities-first
// view of exact Program C evidence. It never reads source files.
package programcpresentation

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"

	"lsp-trace/internal/graph"
	"lsp-trace/internal/programc"
)

const Version = "lsp-trace.community-presentation.v1"
const Authority = "NON_AUTHORITATIVE_DERIVED_VIEW"
const Disclaimer = "Structural communities do not establish feature identity, ownership, architecture, runtime execution, whole-source completeness, producer authentication, permission, or production authority."
const Stability = "Cross-seed stability is unassessed until A-08 exists."

type Location struct {
	URI                                              string
	StartLine, StartCharacter, EndLine, EndCharacter int
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
	NodeID, Name, EnclosingDetail string
	Location                      Location
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
	OccurrenceID   string `json:"occurrence_id"`
	Caller, Callee Node
	CallSite       Location `json:"call_site"`
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
	SchemaVersion                                                                                      string                      `json:"schema_version"`
	Authority                                                                                          string                      `json:"authority"`
	Outcome                                                                                            string                      `json:"outcome"`
	Completeness                                                                                       programc.SourceCompleteness `json:"completeness"`
	Seed                                                                                               uint64                      `json:"seed"`
	ProfileID, ProfileSHA256, Algorithm, PartitionSHA256, ClaimCeiling, Disclaimer, CrossSeedStability string
	Policy                                                                                             programc.BoundaryPolicySpec  `json:"policy"`
	Request                                                                                            programc.BoundaryRequest     `json:"request"`
	Accounting                                                                                         programc.BoundaryAccounting  `json:"accounting"`
	PageRank                                                                                           programc.BoundaryPageRank    `json:"pagerank"`
	Communities                                                                                        []Community                  `json:"communities"`
	CrossCommunityCalls                                                                                []Call                       `json:"cross_community_calls"`
	HighCentralityCrossingNodes                                                                        []programc.BoundaryNodeScore `json:"high_centrality_crossing_nodes"`
	HubCrossingNodes                                                                                   []programc.BoundaryNodeScore `json:"hub_crossing_nodes"`
	CrossingWitnesses                                                                                  []programc.CrossingWitness   `json:"crossing_witnesses"`
	Bridges, ArticulationPoints                                                                        []string
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
	if b.SchemaVersion != programc.BoundaryVersion || b.Bindings.Seed != o.Seed || b.Bindings.PartitionSHA256 != o.LogicalDigest || b.Bindings.ProfileID != o.ProfileID || b.Bindings.ProfileSHA256 != o.ProfileDigest || b.Bindings.Algorithm != o.Algorithm {
		return Artifact{}, fmt.Errorf("boundary/outcome binding mismatch")
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
	if a.SchemaVersion != Version || a.Authority != Authority || a.Disclaimer != Disclaimer || a.CrossSeedStability != Stability {
		return fmt.Errorf("invalid presentation constants")
	}
	if len(a.Communities) > programc.MaxNodes {
		return fmt.Errorf("community limit")
	}
	seen := map[string]bool{}
	var previous []string
	for _, c := range a.Communities {
		current := make([]string, len(c.Members))
		for i := range c.Members {
			current[i] = c.Members[i].NodeID
		}
		if previous != nil && compareIDs(previous, current) >= 0 {
			return fmt.Errorf("noncanonical community order")
		}
		previous = current
		p := ""
		for _, n := range c.Members {
			if n.NodeID <= p && p != "" {
				return fmt.Errorf("noncanonical member order")
			}
			if seen[n.NodeID] {
				return fmt.Errorf("duplicate member")
			}
			seen[n.NodeID] = true
			p = n.NodeID
			if n.Location.StartLine < 1 || n.Location.StartCharacter < 1 || n.Location.EndLine < 1 || n.Location.EndCharacter < 1 {
				return fmt.Errorf("non-one-based location")
			}
		}
	}
	return nil
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
