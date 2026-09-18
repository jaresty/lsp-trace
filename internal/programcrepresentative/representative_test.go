package programcrepresentative

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"lsp-trace/internal/graph"
	"lsp-trace/internal/graphprovenance"
	"lsp-trace/internal/manageddiagnostic"
	"lsp-trace/internal/programc"
	"lsp-trace/internal/programcadmission"
	"lsp-trace/internal/programccompose"
)

func rnode(name string) graph.Node {
	return graph.NewNode(graph.Item{Name: name, Kind: 12, URI: "file:///w/a.go", Range: graph.Range{End: graph.Position{Character: uint32(len(name))}}, SelectionRange: graph.Range{End: graph.Position{Character: uint32(len(name))}}})
}
func redge(a, b graph.Node) graph.Edge {
	return graph.Edge{CallerNodeID: a.ID, CalleeNodeID: b.ID, CallSites: []graph.Range{{End: graph.Position{Character: 1}}}}
}
func rcapture(t *testing.T, id string, nodes []graph.Node, edges []graph.Edge, ms []graph.SeedMembership) programccompose.Input {
	t.Helper()
	r := graph.Result{SchemaVersion: graph.SchemaVersionV5, Invocation: graph.Invocation{WorkspaceURI: "file:///w", Server: graph.ServerInvocation{Command: "gopls"}, LanguageID: "go", Seeds: []graph.InvocationSeed{{Label: "seed", At: "a.go:1:1", ResolvedURI: "file:///w/a.go", ContentSHA256: "sha256:" + strings.Repeat("a", 64), LanguageID: "go"}}, Provenance: graph.InvocationProvenance{InvocationID: id, SourceRevision: "rev", ServerVersion: "v"}}, Nodes: nodes, Edges: edges, Seeds: []graph.SeedResult{{Label: "seed"}}, Summary: graph.Summary{Complete: true}, Capabilities: graph.Capabilities{CallHierarchyProvider: true}}
	r.Canonicalize()
	for _, m := range ms {
		switch m.EvidenceKind {
		case "PREPARED_TARGET":
			r.Seeds[0].PreparedTargetIDs = append(r.Seeds[0].PreparedTargetIDs, m.EndpointID)
		case "REACHED_NODE":
			r.Seeds[0].ReachedNodeIDs = append(r.Seeds[0].ReachedNodeIDs, m.EndpointID)
		case "CALL_RELATION":
			r.Seeds[0].ReachedRelationIDs = append(r.Seeds[0].ReachedRelationIDs, r.Edges[0].RelationID)
		}
	}
	raw, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	captured, err := graphprovenance.CaptureV5(raw, "session", 7, manageddiagnostic.QueryResult{Status: manageddiagnostic.QueryUnavailable})
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(captured)
	return programccompose.Input{Bytes: captured, Identity: id, SHA256: fmt.Sprintf("sha256:%x", sum), ByteLength: len(captured), ExactMetadata: programccompose.ExactMetadata{WorkspaceIdentity: "w", RevisionCustody: "CALLER_ASSERTED", PositionEncoding: "utf-16", AcquisitionSemantics: "managed", PrivacyPolicy: "private"}}
}
func admitted(t *testing.T, ms []graph.SeedMembership, nodes []graph.Node, edges []graph.Edge) programcadmission.CompositeProjectionAdmission {
	t.Helper()
	x := rcapture(t, "one", nodes, edges, ms)
	y := rcapture(t, "two", nodes, nil, nil)
	c, err := programccompose.Compose([]programccompose.Input{x, y})
	if err != nil {
		t.Fatal(err)
	}
	a, err := programcadmission.Admit(c.Bytes)
	if err != nil {
		t.Fatal(err)
	}
	return a.Admission
}
func memberships(n []graph.Node, edges []graph.Edge, label string) []graph.SeedMembership {
	m := []graph.SeedMembership{{MembershipID: label + "-p", ExecutionBundleID: "bundle", SeedLabel: label, SeedAt: "at", EvidenceKind: "PREPARED_TARGET", EndpointID: n[0].ID}, {MembershipID: label + "-r", ExecutionBundleID: "bundle", SeedLabel: label, SeedAt: "at", EvidenceKind: "REACHED_NODE", EndpointID: n[len(n)-1].ID}}
	if len(edges) > 0 {
		m = append(m, graph.SeedMembership{MembershipID: label + "-c", ExecutionBundleID: "bundle", SeedLabel: label, SeedAt: "at", EvidenceKind: "CALL_RELATION"})
	}
	return m
}

func TestSelectRealAdmissionsNearestSCCAndIsolation(t *testing.T) {
	a, b := rnode("a"), rnode("b")
	nodes := []graph.Node{a, b}
	edges := []graph.Edge{redge(a, b)}
	got, err := Select(Input{Admission: admitted(t, memberships(nodes, edges, "s"), nodes, edges), Communities: []programc.Community{{Members: []string{b.ID}}}})
	if err != nil {
		t.Fatal(err)
	}
	if got.State != StateSelected || len(got.Nominations) != 1 || got.Nominations[0].SelectedNode != b.ID || got.Nominations[0].Distance != 1 {
		t.Fatalf("nearest=%+v", got)
	}
}
func TestSelectRealAdmissionUnresolvedAndEmpty(t *testing.T) {
	a, b := rnode("a"), rnode("b")
	nodes := []graph.Node{a, b}
	ms := memberships(nodes, nil, "s")
	got, err := Select(Input{Admission: admitted(t, ms, nodes, nil), Communities: []programc.Community{{Members: []string{b.ID}}}})
	if err != nil || got.State != StateUnresolved || len(got.Unresolved) != 1 {
		t.Fatalf("unresolved=%+v err=%v", got, err)
	}
	got, err = Select(Input{Admission: admitted(t, nil, nodes, nil), Communities: []programc.Community{{Members: []string{a.ID}}}})
	if err != nil || got.State != StateUnresolved || len(got.Nominations) != 0 || len(got.Unresolved) != 0 {
		t.Fatalf("no seeds=%+v err=%v", got, err)
	}
	got, err = Select(Input{Admission: admitted(t, nil, nil, nil)})
	if err != nil || got.State != StateEmpty {
		t.Fatalf("empty=%+v err=%v", got, err)
	}
}
func TestSelectRejectsCommunityAndMembershipFailures(t *testing.T) {
	a := rnode("a")
	in := Input{Admission: admitted(t, nil, []graph.Node{a}, nil)}
	for _, cs := range [][]string{{}, {a.ID, a.ID}, {"unknown"}} {
		if _, err := Select(Input{Admission: in.Admission, Communities: []programc.Community{{Members: cs}}}); err == nil {
			t.Fatalf("community %v accepted", cs)
		}
	}
	bad, _ := json.Marshal(append(memberships([]graph.Node{a}, nil, "s"), memberships([]graph.Node{a}, nil, "s")[0]))
	if _, err := decode(programcadmission.ConstituentReference{SeedMemberships: bad}, map[string]bool{a.ID: true}, nil); err == nil {
		t.Fatal("duplicate membership accepted")
	}
}
func TestDecodeRejectsTrailingAndUnknown(t *testing.T) {
	a := rnode("a")
	adm := admitted(t, memberships([]graph.Node{a}, nil, "s"), []graph.Node{a}, nil)
	source := adm.SourceBinding()
	source.Constituents[0].SeedMemberships = append(source.Constituents[0].SeedMemberships, []byte(" []")...)
	if _, err := decode(source.Constituents[0], map[string]bool{a.ID: true}, nil); err == nil {
		t.Fatal("trailing accepted")
	}
}
func TestChooseSCCDistanceTieSelfLoopAndMultiplicity(t *testing.T) {
	nodes := []string{"a", "b", "p"}
	occ := func(id, relation string, from, to int64) programcadmission.Occurrence {
		return programcadmission.Occurrence{Identity: id, RelationID: relation, From: from, To: to, Weight: 1}
	}
	s := &seed{prepared: map[string]bool{"p": true}, reached: map[string]bool{"a": true, "b": true}, relations: map[string]bool{"pa": true, "ab": true, "ba": true}}
	relations := map[string][]programcadmission.Occurrence{
		"pa": {occ("1", "pa", 2, 0), occ("2", "pa", 2, 0)},
		"ab": {occ("3", "ab", 0, 1)},
		"ba": {occ("4", "ba", 1, 0)},
	}
	selected, distance, members, ok := choose(nodes, relations, s, []string{"b", "a"})
	if !ok || selected != "a" || distance != 1 || !reflect.DeepEqual(members, []string{"a", "b"}) {
		t.Fatalf("SCC choice=(%q,%d,%v,%t)", selected, distance, members, ok)
	}

	tie := &seed{prepared: map[string]bool{"p": true}, reached: map[string]bool{"a": true, "b": true}, relations: map[string]bool{"pa": true, "pb": true}}
	relations["pb"] = []programcadmission.Occurrence{occ("5", "pb", 2, 1)}
	selected, distance, _, ok = choose(nodes, relations, tie, []string{"b", "a"})
	if !ok || selected != "a" || distance != 1 {
		t.Fatalf("tie choice=(%q,%d,%t)", selected, distance, ok)
	}

	loop := &seed{prepared: map[string]bool{"p": true}, reached: map[string]bool{"p": true}, relations: map[string]bool{"pp": true}}
	selected, distance, members, ok = choose(nodes, map[string][]programcadmission.Occurrence{"pp": {occ("6", "pp", 2, 2)}}, loop, []string{"p"})
	if !ok || selected != "p" || distance != 0 || !reflect.DeepEqual(members, []string{"p"}) {
		t.Fatalf("self-loop choice=(%q,%d,%v,%t)", selected, distance, members, ok)
	}
}

func TestDecodeFailsClosedOnMembershipContradictions(t *testing.T) {
	base := graph.SeedMembership{MembershipID: "m", ExecutionBundleID: "bundle", SeedLabel: "seed", SeedAt: "at", EvidenceKind: "REACHED_NODE", EndpointID: "node"}
	encode := func(ms ...graph.SeedMembership) programcadmission.ConstituentReference {
		raw, err := json.Marshal(ms)
		if err != nil {
			t.Fatal(err)
		}
		return programcadmission.ConstituentReference{SeedMemberships: raw}
	}
	cases := map[string][]graph.SeedMembership{
		"unknown kind":     {{MembershipID: "m", ExecutionBundleID: "bundle", SeedLabel: "seed", SeedAt: "at", EvidenceKind: "OTHER", EndpointID: "node"}},
		"unknown node":     {base},
		"mixed bundle":     {base, {MembershipID: "m2", ExecutionBundleID: "other", SeedLabel: "seed", SeedAt: "at", EvidenceKind: "PREPARED_TARGET", EndpointID: "node"}},
		"unknown relation": {{MembershipID: "m", ExecutionBundleID: "bundle", SeedLabel: "seed", SeedAt: "at", EvidenceKind: "CALL_RELATION", EndpointID: "relation"}},
	}
	for name, memberships := range cases {
		t.Run(name, func(t *testing.T) {
			nodes := map[string]bool{"node": true}
			if name == "unknown node" {
				nodes = nil
			}
			if _, err := decode(encode(memberships...), nodes, nil); err == nil {
				t.Fatal("contradictory membership accepted")
			}
		})
	}
}

func TestOutputSlicesAreCloned(t *testing.T) {
	a := rnode("a")
	input := Input{Admission: admitted(t, memberships([]graph.Node{a}, nil, "s"), []graph.Node{a}, nil), Communities: []programc.Community{{Members: []string{a.ID}}}}
	got, err := Select(input)
	if err != nil {
		t.Fatal(err)
	}
	again, err := Select(input)
	if err != nil || !reflect.DeepEqual(got, again) {
		t.Fatalf("nondeterministic: err=%v", err)
	}
	got.Nominations[0].PreparedTargets[0] = "prepared-mutated"
	got.Nominations[0].CommunityMembers[0] = "community-mutated"
	got.Nominations[0].SCCMembers[0] = "scc-mutated"
	if again.Nominations[0].PreparedTargets[0] == "prepared-mutated" || again.Nominations[0].CommunityMembers[0] == "community-mutated" || again.Nominations[0].SCCMembers[0] == "scc-mutated" {
		t.Fatal("returned nomination slices alias another selection")
	}
	if input.Communities[0].Members[0] != a.ID {
		t.Fatal("returned nomination aliases caller community")
	}
}
