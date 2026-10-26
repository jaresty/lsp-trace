package graph

import (
	"encoding/json"
	"sort"
	"strings"
	"testing"
)

func TestNativeV5RepresentativeViewFiveMembershipKindsAndDefensiveAccessors(t *testing.T) {
	raw := nativeV5RepresentativeFixture(t)
	view, err := NewNativeV5RepresentativeView(raw)
	if err != nil || !view.Valid() || view.ExecutionBundleID() == "" {
		t.Fatalf("ASSERT_NATIVE_V5_REPRESENTATIVE_VIEW_VALID view=%+v err=%v", view, err)
	}
	kinds := map[string]bool{}
	memberships := view.SeedMemberships()
	for _, membership := range memberships {
		kinds[membership.EvidenceKind] = true
	}
	for _, kind := range []string{"PREPARED_TARGET", "REACHED_NODE", "CALL_RELATION", "SIBLING_CANDIDATE", "DISPATCH_ASSOCIATION"} {
		if !kinds[kind] {
			t.Fatalf("ASSERT_NATIVE_V5_REPRESENTATIVE_MEMBERSHIP_KIND kind=%q memberships=%+v", kind, memberships)
		}
	}
	edges := view.Edges()
	seeds := view.InvocationSeeds()
	nodes := view.NodeIDs()
	edges[0].CallSites[0].Start.Line = 999
	seeds[0].Label = "mutated"
	memberships[0].EndpointID = "mutated"
	nodes[0] = "mutated"
	if view.Edges()[0].CallSites[0].Start.Line == 999 || view.InvocationSeeds()[0].Label == "mutated" || view.SeedMemberships()[0].EndpointID == "mutated" || view.NodeIDs()[0] == "mutated" {
		t.Fatal("ASSERT_NATIVE_V5_REPRESENTATIVE_ACCESSORS_DEFENSIVE")
	}
	var zero NativeV5RepresentativeView
	if zero.Valid() || zero.ExecutionBundleID() != "" || zero.Edges() == nil || zero.InvocationSeeds() == nil || zero.SeedMemberships() == nil || zero.NodeIDs() == nil {
		t.Fatal("ASSERT_NATIVE_V5_REPRESENTATIVE_ZERO_INVALID")
	}
}

func TestNativeV5RepresentativeViewRejectsUnknownMembershipKind(t *testing.T) {
	raw := nativeV5RepresentativeFixture(t)
	var bundle bundleV3
	if err := json.Unmarshal(raw, &bundle); err != nil {
		t.Fatal(err)
	}
	bundle.SeedMemberships[0].EvidenceKind = "UNKNOWN_MEMBERSHIP"
	forged := resealNativeV5RepresentativeFixture(t, bundle)
	if _, err := NewNativeV5RepresentativeView(forged); err == nil {
		t.Fatal("ASSERT_NATIVE_V5_REPRESENTATIVE_UNKNOWN_MEMBERSHIP_REJECTED")
	}
}

func TestNativeV5RepresentativeViewRejectsNoncanonicalEdgeForms(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*bundleV3)
	}{
		{name: "permuted edges", mutate: func(bundle *bundleV3) {
			bundle.Edges[0], bundle.Edges[1] = bundle.Edges[1], bundle.Edges[0]
		}},
		{name: "split duplicate edge", mutate: func(bundle *bundleV3) {
			edge := bundle.Edges[0]
			first, second := edge, edge
			first.CallSites = append([]Range(nil), edge.CallSites[:1]...)
			second.CallSites = append([]Range(nil), edge.CallSites[1:]...)
			bundle.Edges = append([]Edge{first, second}, bundle.Edges[1:]...)
		}},
		{name: "permuted call sites", mutate: func(bundle *bundleV3) {
			bundle.Edges[0].CallSites[0], bundle.Edges[0].CallSites[1] = bundle.Edges[0].CallSites[1], bundle.Edges[0].CallSites[0]
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var bundle bundleV3
			if err := json.Unmarshal(nativeV5RepresentativeFixture(t), &bundle); err != nil {
				t.Fatal(err)
			}
			tc.mutate(&bundle)
			forged := resealNativeV5RepresentativeFixture(t, bundle)
			if err := ValidateSemanticBundle(forged); err != nil {
				t.Fatalf("fixture must remain semantically valid before representative checks: %v", err)
			}
			if _, err := NewNativeV5RepresentativeView(forged); err == nil {
				t.Fatal("ASSERT_NATIVE_V5_REPRESENTATIVE_NONCANONICAL_EDGE_REJECTED")
			}
		})
	}
}

func resealNativeV5RepresentativeFixture(t *testing.T, bundle bundleV3) []byte {
	t.Helper()
	bundle.EvidenceReceipt = (Result{
		SchemaVersion:         bundle.SchemaVersion,
		Edges:                 append([]Edge(nil), bundle.Edges...),
		SiblingCandidates:     append([]SiblingCandidate(nil), bundle.SiblingCandidates...),
		DispatchRelationships: append([]DispatchRelationship(nil), bundle.DispatchRelationships...),
	}).evidenceReceipt(bundle.Invocation.Provenance.SourceRevision)
	verified := Result{
		SchemaVersion: bundle.SchemaVersion, Targets: bundle.Targets, Nodes: bundle.Nodes, Edges: cloneRepresentativeEdges(bundle.Edges),
		Terminals: bundle.Terminals, Frontier: bundle.Frontier, Diagnostics: bundle.Diagnostics, Seeds: bundle.Seeds,
		SiblingCandidates: bundle.SiblingCandidates, DispatchRelationships: bundle.DispatchRelationships, Slice: bundle.Slice,
		Summary: Summary{Complete: true, Truncated: bundle.Summary.Truncated}, Capabilities: bundle.Capabilities, CapabilityQuality: bundle.CapabilityQuality,
	}
	verified.Canonicalize()
	bundle.Summary.NodeCount = verified.Summary.NodeCount
	bundle.Summary.TraversalComplete = verified.Summary.Complete
	bundle.Summary.EdgeCount = verified.Summary.EdgeCount
	bundle.Summary.TerminalCount = verified.Summary.TerminalCount
	bundle.Summary.CycleCount = verified.Summary.CycleCount
	bundle.CapabilityQuality = verified.CapabilityQuality
	canonical, err := json.Marshal(bundle.semanticV3)
	if err != nil {
		t.Fatal(err)
	}
	bundle.TraceReceipt.SemanticCommitmentDigest = domainDigest(SemanticDigestDomainV5, canonical)
	forged, err := json.Marshal(bundle)
	if err != nil {
		t.Fatal(err)
	}
	return forged
}

func nativeV5RepresentativeFixture(t *testing.T) []byte {
	t.Helper()
	node := func(name string, line uint32) Node {
		r := Range{Start: Position{Line: line}, End: Position{Line: line, Character: 2}}
		return NewNode(Item{Name: name, Kind: 12, URI: "file:///fixture/input.go", Range: r, SelectionRange: r})
	}
	caller, callee, candidate := node("caller", 0), node("callee", 2), node("candidate", 4)
	edges := []Edge{
		{CallerNodeID: caller.ID, CalleeNodeID: callee.ID, CallSites: []Range{
			{Start: Position{Line: 0, Character: 1}, End: Position{Line: 0, Character: 2}},
			{Start: Position{Line: 1, Character: 1}, End: Position{Line: 1, Character: 2}},
		}},
		{CallerNodeID: callee.ID, CalleeNodeID: candidate.ID, CallSites: []Range{{Start: Position{Line: 2, Character: 1}, End: Position{Line: 2, Character: 2}}}},
	}
	canonical := Result{SchemaVersion: SchemaVersionV5, Nodes: []Node{caller, callee, candidate}, Edges: edges}
	canonical.Canonicalize()
	edges = canonical.Edges
	seed := InvocationSeed{Label: "seed", At: "input.go:1:1", ResolvedURI: "file:///fixture/input.go", ContentSHA256: "sha256:" + strings.Repeat("a", 64), LanguageID: "go"}
	result := Result{
		SchemaVersion: SchemaVersionV5,
		Invocation:    Invocation{Server: ServerInvocation{Command: "fixture-ls"}, Seeds: []InvocationSeed{seed}, Provenance: InvocationProvenance{InvocationID: "fixture-session", SourceRevision: "fixture-revision", ServerVersion: "fixture-ls@1"}, Expansion: ExpansionConfig{TopmostSiblings: true, TopmostSiblingOutcome: TopmostSiblingExactRelationsFound, DispatchFamily: true}},
		Nodes:         []Node{candidate, callee, caller}, Edges: edges,
		Seeds:                 []SeedResult{{Label: seed.Label, PreparedTargetIDs: []string{caller.ID}, ReachedNodeIDs: []string{caller.ID, callee.ID, candidate.ID}, ReachedRelationIDs: []string{edges[0].RelationID, edges[1].RelationID}}},
		SiblingCandidates:     []SiblingCandidate{{SeedURI: seed.ResolvedURI, SeedLabel: seed.Label, Origin: caller, Candidate: candidate, Direction: "SIBLING", Kind: "TOPMOST_SIBLING", SeedIdentity: "fixture-session:seed:input.go:1:1", ProviderEvidence: []string{"command=fixture-ls;server_version=fixture-ls@1;invocation=fixture-session"}, LSPEvidence: []string{"textDocument/documentSymbol", "textDocument/prepareCallHierarchy"}, SourceDigests: []string{"candidate=" + seed.ContentSHA256, "origin=" + seed.ContentSHA256}, Custody: SourceCustodyEvidence{Class: SourceCustodyUnknown, ClaimCeiling: "NO_AUTHENTICATED_ANALYZED_SOURCE_IDENTITY"}}},
		DispatchRelationships: []DispatchRelationship{{SeedLabel: seed.Label, SeedLabels: []string{seed.Label}, Interface: caller, Implementation: callee}},
		Summary:               Summary{Complete: true}, Capabilities: Capabilities{CallHierarchyProvider: true},
	}
	raw, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	var bundle bundleV3
	if err := json.Unmarshal(raw, &bundle); err != nil {
		t.Fatal(err)
	}
	hasDispatchMembership := false
	for _, membership := range bundle.SeedMemberships {
		hasDispatchMembership = hasDispatchMembership || membership.EvidenceKind == "DISPATCH_ASSOCIATION"
	}
	if !hasDispatchMembership {
		bundle.SeedMemberships = append(bundle.SeedMemberships, newSeedMembership(bundle.ExecutionBundleID, bundle.Invocation.Seeds[0], "DISPATCH_ASSOCIATION", bundle.DispatchRelationships[0].RelationID))
		sort.Slice(bundle.SeedMemberships, func(i, j int) bool {
			return bundle.SeedMemberships[i].MembershipID < bundle.SeedMemberships[j].MembershipID
		})
		canonical, marshalErr := json.Marshal(bundle.semanticV3)
		if marshalErr != nil {
			t.Fatal(marshalErr)
		}
		bundle.TraceReceipt.SemanticCommitmentDigest = domainDigest(SemanticDigestDomainV5, canonical)
		raw, err = json.Marshal(bundle)
		if err != nil {
			t.Fatal(err)
		}
	}
	if err := ValidateSemanticBundle(raw); err != nil {
		t.Fatalf("build native V5 representative fixture: %v", err)
	}
	return raw
}
