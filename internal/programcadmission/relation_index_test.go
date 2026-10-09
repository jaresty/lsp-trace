package programcadmission

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"testing"

	"lsp-trace/internal/graph"
	"lsp-trace/internal/graphprovenance"
	"lsp-trace/internal/manageddiagnostic"
	"lsp-trace/internal/programccompose"
)

func capture(t *testing.T, suffix string, nodes []graph.Node, edges []graph.Edge) programccompose.Input {
	t.Helper()
	seed := graph.InvocationSeed{Label: "seed" + suffix, At: "a.go:1:1", ResolvedURI: "file:///w/a.go", ContentSHA256: "sha256:" + strings.Repeat("a", 64), LanguageID: "go"}
	r := graph.Result{SchemaVersion: graph.SchemaVersionV5, Invocation: graph.Invocation{WorkspaceURI: "file:///w", Server: graph.ServerInvocation{Command: "gopls"}, LanguageID: "go", Seeds: []graph.InvocationSeed{seed}, Provenance: graph.InvocationProvenance{InvocationID: "invocation-" + suffix, SourceRevision: "commit", ServerVersion: "gopls@1"}}, Nodes: nodes, Edges: edges, Seeds: []graph.SeedResult{{Label: seed.Label}}, Summary: graph.Summary{Complete: false}, Capabilities: graph.Capabilities{CallHierarchyProvider: true}}
	native, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := graphprovenance.CaptureV5(native, "session", 7, manageddiagnostic.QueryResult{Status: manageddiagnostic.QueryUnavailable})
	if err != nil {
		t.Fatal(err)
	}
	s := sha256.Sum256(raw)
	return programccompose.Input{Bytes: raw, Identity: "capture-" + suffix, SHA256: fmt.Sprintf("sha256:%x", s), ByteLength: len(raw), ExactMetadata: programccompose.ExactMetadata{WorkspaceIdentity: "workspace-1", RevisionCustody: "CALLER_ASSERTED", PositionEncoding: "utf-16", AcquisitionSemantics: "managed-lsp-v1", PrivacyPolicy: "bundle-custodian-v1"}}
}

func node(name string) graph.Node {
	return graph.NewNode(graph.Item{Name: name, Kind: 12, URI: "file:///w/a.go", Range: graph.Range{End: graph.Position{Character: uint32(len(name))}}, SelectionRange: graph.Range{End: graph.Position{Character: uint32(len(name))}}})
}

func edge(a, b graph.Node) graph.Edge {
	return graph.Edge{CallerNodeID: a.ID, CalleeNodeID: b.ID, CallSites: []graph.Range{{End: graph.Position{Character: 1}}}}
}

func methodOccurrence(identity string, kind RelationKind, ordinal int, from, to string) MethodOccurrenceEvidence {
	return MethodOccurrenceEvidence{
		Identity: identity, Kind: kind, Ordinal: ordinal, From: from, To: to,
		SourceRole:    map[RelationKind]string{TypedReferencesSymbol: "REFERENCING_OCCURRENCE", TypedResolvesToDefinition: "QUERY_SYMBOL"}[kind],
		TargetRole:    map[RelationKind]string{TypedReferencesSymbol: "REFERENCED_SYMBOL", TypedResolvesToDefinition: "DEFINITION_TARGET"}[kind],
		Method:        map[RelationKind]string{TypedReferencesSymbol: "textDocument/references", TypedResolvesToDefinition: "textDocument/definition"}[kind],
		QuerySelector: "queries/" + identity + ".json", QueryDigest: "sha256:" + strings.Repeat("1", 64), QueryLength: 17,
		EvidenceSelector: "evidence/" + identity + ".json", EvidenceDigest: "sha256:" + strings.Repeat("2", 64), EvidenceLength: 29,
		Provider: "gopls", ProviderVersion: "v0.17.1", SessionID: "session-1", Generation: 7,
		Revision: "commit-1", Custody: "CALLER_ASSERTED", PositionEncoding: "utf-16",
	}
}

func validUntrustedMethodEvidenceEnvelope() UntrustedMethodEvidenceEnvelope {
	return UntrustedMethodEvidenceEnvelope{
		Family: MethodEvidenceFamily, Version: MethodEvidenceFamily, ClaimCeiling: MethodEvidenceClaimCeiling,
		Authority: 0, Accepted: false, Completeness: "UNKNOWN",
		Provider: "gopls", ProviderVersion: "v0.17.1", SessionID: "session-1", Generation: 7,
		Revision: "commit-1", Custody: "CALLER_ASSERTED", PositionEncoding: "utf-16",
		Occurrences: []MethodOccurrenceEvidence{
			methodOccurrence("reference-1", TypedReferencesSymbol, 1, "file:///w/use.go#1", "file:///w/decl.go#target"),
			methodOccurrence("definition-1", TypedResolvesToDefinition, 0, "file:///w/use.go#query", "file:///w/decl.go#definition"),
			methodOccurrence("reference-0", TypedReferencesSymbol, 0, "file:///w/use.go#1", "file:///w/decl.go#target"),
		},
	}
}

func TestRelationIndexRejectsUnsealedAndTamperedInputAtomically(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   TypedInput
	}{
		{name: "zero"},
		{name: "tampered", in: tamperedRelationIndexInput(t)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := buildRelationIndex(tc.in)
			var zero relationIndex
			if !errors.Is(err, errRelationIndexIntegrity) {
				t.Fatalf("ASSERT_RELATION_INDEX_INTEGRITY_ERROR err=%v", err)
			}
			if !reflect.DeepEqual(got, zero) {
				t.Fatalf("ASSERT_RELATION_INDEX_INTEGRITY_ATOMIC_ZERO got=%#v", got)
			}
			if got.allOccurrences() != nil || got.outgoingOccurrences("endpoint") != nil || got.incomingOccurrences("endpoint") != nil {
				t.Fatalf("ASSERT_RELATION_INDEX_ZERO_NIL_READS all=%#v outgoing=%#v incoming=%#v", got.allOccurrences(), got.outgoingOccurrences("endpoint"), got.incomingOccurrences("endpoint"))
			}
		})
	}
}

func TestRelationIndexPreservesMixedOccurrencesAndDirectionalIdentity(t *testing.T) {
	in := mixedRelationIndexInput(t, false)
	index, err := buildRelationIndex(in)
	if err != nil {
		t.Fatal(err)
	}

	wantAll := sortedTypedOccurrences(in.Occurrences)
	if got := index.allOccurrences(); !reflect.DeepEqual(got, wantAll) {
		t.Fatalf("ASSERT_RELATION_INDEX_ALL_FIELD_IDENTITY got=%+v want=%+v", got, wantAll)
	}
	for _, endpoint := range relationIndexEndpoints(wantAll) {
		wantOutgoing := relationIndexBucket(wantAll, endpoint, true)
		wantIncoming := relationIndexBucket(wantAll, endpoint, false)
		if got := index.outgoingOccurrences(endpoint); !reflect.DeepEqual(got, wantOutgoing) {
			t.Fatalf("ASSERT_RELATION_INDEX_OUTGOING_IDENTITY endpoint=%q got=%+v want=%+v", endpoint, got, wantOutgoing)
		}
		if got := index.incomingOccurrences(endpoint); !reflect.DeepEqual(got, wantIncoming) {
			t.Fatalf("ASSERT_RELATION_INDEX_INCOMING_ORIENTATION endpoint=%q got=%+v want=%+v", endpoint, got, wantIncoming)
		}
	}

	var self TypedOccurrence
	for _, occurrence := range wantAll {
		if occurrence.From == occurrence.To {
			self = occurrence
			break
		}
	}
	if self.Identity == "" {
		t.Fatal("fixture lacks a self-loop")
	}
	if countTypedOccurrence(index.allOccurrences(), self) != 1 || countTypedOccurrence(index.outgoingOccurrences(self.From), self) != 1 || countTypedOccurrence(index.incomingOccurrences(self.To), self) != 1 {
		t.Fatalf("ASSERT_RELATION_INDEX_SELF_LOOP_EXACTLY_ONCE self=%+v all=%+v outgoing=%+v incoming=%+v", self, index.allOccurrences(), index.outgoingOccurrences(self.From), index.incomingOccurrences(self.To))
	}
}

func TestRelationIndexDeterministicAcrossAdmittedSourcePermutations(t *testing.T) {
	left := mixedRelationIndexInput(t, false)
	right := mixedRelationIndexInput(t, true)
	if !reflect.DeepEqual(sortedTypedOccurrences(left.Occurrences), sortedTypedOccurrences(right.Occurrences)) {
		t.Fatalf("fixture permutations changed admitted occurrences: left=%+v right=%+v", left.Occurrences, right.Occurrences)
	}

	leftIndex, err := buildRelationIndex(left)
	if err != nil {
		t.Fatal(err)
	}
	rightIndex, err := buildRelationIndex(right)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(leftIndex.allOccurrences(), rightIndex.allOccurrences()) || !sort.SliceIsSorted(leftIndex.allOccurrences(), func(i, j int) bool {
		return leftIndex.allOccurrences()[i].Identity < leftIndex.allOccurrences()[j].Identity
	}) {
		t.Fatalf("ASSERT_RELATION_INDEX_ALL_DETERMINISTIC left=%+v right=%+v", leftIndex.allOccurrences(), rightIndex.allOccurrences())
	}
	for _, endpoint := range relationIndexEndpoints(leftIndex.allOccurrences()) {
		leftOutgoing, rightOutgoing := leftIndex.outgoingOccurrences(endpoint), rightIndex.outgoingOccurrences(endpoint)
		leftIncoming, rightIncoming := leftIndex.incomingOccurrences(endpoint), rightIndex.incomingOccurrences(endpoint)
		if !reflect.DeepEqual(leftOutgoing, rightOutgoing) || !typedOccurrencesIdentitySorted(leftOutgoing) {
			t.Fatalf("ASSERT_RELATION_INDEX_OUTGOING_DETERMINISTIC endpoint=%q left=%+v right=%+v", endpoint, leftOutgoing, rightOutgoing)
		}
		if !reflect.DeepEqual(leftIncoming, rightIncoming) || !typedOccurrencesIdentitySorted(leftIncoming) {
			t.Fatalf("ASSERT_RELATION_INDEX_INCOMING_DETERMINISTIC endpoint=%q left=%+v right=%+v", endpoint, leftIncoming, rightIncoming)
		}
	}
}

func TestRelationIndexAccessorsAreDefensiveAndZeroSafe(t *testing.T) {
	in := mixedRelationIndexInput(t, false)
	index, err := buildRelationIndex(in)
	if err != nil {
		t.Fatal(err)
	}
	wantAll := index.allOccurrences()
	endpoint := wantAll[0].From
	wantOutgoing, wantIncoming := index.outgoingOccurrences(endpoint), index.incomingOccurrences(endpoint)

	mutateTypedOccurrences(index.allOccurrences())
	mutateTypedOccurrences(index.outgoingOccurrences(endpoint))
	mutateTypedOccurrences(index.incomingOccurrences(endpoint))
	if !reflect.DeepEqual(index.allOccurrences(), wantAll) || !reflect.DeepEqual(index.outgoingOccurrences(endpoint), wantOutgoing) || !reflect.DeepEqual(index.incomingOccurrences(endpoint), wantIncoming) {
		t.Fatalf("ASSERT_RELATION_INDEX_DEFENSIVE_ACCESSORS all=%+v outgoing=%+v incoming=%+v", index.allOccurrences(), index.outgoingOccurrences(endpoint), index.incomingOccurrences(endpoint))
	}

	var zero relationIndex
	if zero.allOccurrences() != nil || zero.outgoingOccurrences(endpoint) != nil || zero.incomingOccurrences(endpoint) != nil {
		t.Fatalf("ASSERT_RELATION_INDEX_ZERO_SAFE all=%#v outgoing=%#v incoming=%#v", zero.allOccurrences(), zero.outgoingOccurrences(endpoint), zero.incomingOccurrences(endpoint))
	}
}

func TestRelationIndexCallsOnlyCompatibilityAndAdmissionBoundary(t *testing.T) {
	callsOnly := callsOnlyRelationIndexInput(t)
	beforeOccurrences := append([]TypedOccurrence(nil), callsOnly.Occurrences...)
	beforeWeights := make(map[TypedPair]int, len(callsOnly.PairWeights))
	for pair, weight := range callsOnly.PairWeights {
		beforeWeights[pair] = weight
	}
	beforeMetadata := typedInputMetadata(callsOnly)
	index, err := buildRelationIndex(callsOnly)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(index.allOccurrences(), callsOnly.Occurrences) {
		t.Fatalf("ASSERT_RELATION_INDEX_CALLS_ONLY_OCCURRENCES got=%+v want=%+v", index.allOccurrences(), callsOnly.Occurrences)
	}
	if !reflect.DeepEqual(callsOnly.Occurrences, beforeOccurrences) || !reflect.DeepEqual(callsOnly.PairWeights, beforeWeights) || typedInputMetadata(callsOnly) != beforeMetadata {
		t.Fatalf("ASSERT_RELATION_INDEX_CALLS_ONLY_INPUT_MUTATED got=%+v", callsOnly)
	}
	for _, occurrence := range index.allOccurrences() {
		if occurrence.Kind != TypedCalls {
			t.Fatalf("ASSERT_RELATION_INDEX_CALLS_ONLY_NO_DEFINITION_REFERENCE occurrence=%+v", occurrence)
		}
	}

	// PrepareTypedInput remains the sole owner of node and occurrence caps. The
	// index accepts its sealed output without introducing a second resource bound.
	if len(index.allOccurrences()) != len(callsOnly.Occurrences) {
		t.Fatalf("ASSERT_RELATION_INDEX_INHERITS_ADMISSION_BOUNDS got=%d want=%d", len(index.allOccurrences()), len(callsOnly.Occurrences))
	}
}

func tamperedRelationIndexInput(t *testing.T) TypedInput {
	t.Helper()
	in := callsOnlyRelationIndexInput(t)
	in.Occurrences[0].To += "-tampered"
	return in
}

func callsOnlyRelationIndexInput(t *testing.T) TypedInput {
	t.Helper()
	a, b := node("relation-index-calls-a"), node("relation-index-calls-b")
	e := edge(a, b)
	e.CallSites = []graph.Range{{End: graph.Position{Character: 2}}, {End: graph.Position{Character: 1}}}
	composite, err := programccompose.Compose([]programccompose.Input{
		capture(t, "relation-index-calls", []graph.Node{a, b}, []graph.Edge{e}),
		capture(t, "relation-index-calls-context", []graph.Node{a, b}, nil),
	})
	if err != nil {
		t.Fatal(err)
	}
	calls, err := Admit(composite.Bytes)
	if err != nil {
		t.Fatal(err)
	}
	in, err := PrepareTypedInput(TypedInputRequest{Calls: calls.Admission, Kinds: []RelationKind{TypedCalls}})
	if err != nil {
		t.Fatal(err)
	}
	return in
}

func mixedRelationIndexInput(t *testing.T, reverseMethodSources bool) TypedInput {
	t.Helper()
	a, b, c := node("relation-index-a"), node("relation-index-b"), node("relation-index-c")
	parallel := edge(a, b)
	parallel.CallSites = []graph.Range{{End: graph.Position{Character: 3}}, {End: graph.Position{Character: 1}}}
	reversed := edge(b, a)
	self := edge(c, c)
	composite, err := programccompose.Compose([]programccompose.Input{
		capture(t, "relation-index-mixed", []graph.Node{a, b, c}, []graph.Edge{parallel, reversed, self}),
		capture(t, "relation-index-mixed-context", []graph.Node{a, b, c}, nil),
	})
	if err != nil {
		t.Fatal(err)
	}
	calls, err := Admit(composite.Bytes)
	if err != nil {
		t.Fatal(err)
	}

	envelope := validUntrustedMethodEvidenceEnvelope()
	envelope.Occurrences = []MethodOccurrenceEvidence{
		methodOccurrence("index-reference-parallel", TypedReferencesSymbol, 1, a.ID, b.ID),
		methodOccurrence("index-definition-cross-kind", TypedResolvesToDefinition, 0, a.ID, b.ID),
		methodOccurrence("index-reference-reversed", TypedReferencesSymbol, 0, b.ID, a.ID),
		methodOccurrence("index-definition-self", TypedResolvesToDefinition, 0, c.ID, c.ID),
	}
	if reverseMethodSources {
		for left, right := 0, len(envelope.Occurrences)-1; left < right; left, right = left+1, right-1 {
			envelope.Occurrences[left], envelope.Occurrences[right] = envelope.Occurrences[right], envelope.Occurrences[left]
		}
	}
	methods, err := AdmitUntrustedMethodEvidenceEnvelope(envelope)
	if err != nil {
		t.Fatal(err)
	}
	in, err := PrepareTypedInput(TypedInputRequest{
		Calls: calls.Admission, Methods: methods, EnableMethods: true,
		Kinds: []RelationKind{TypedCalls, TypedReferencesSymbol, TypedResolvesToDefinition},
	})
	if err != nil {
		t.Fatal(err)
	}
	return in
}

type relationIndexTypedInputMetadata struct {
	version, compositeID, compositeOutputSHA256, claimCeiling string
	authority                                                 int
	sourceGraphComplete                                       string
}

func typedInputMetadata(in TypedInput) relationIndexTypedInputMetadata {
	return relationIndexTypedInputMetadata{in.Version, in.CompositeID, in.CompositeOutputSHA256, in.ClaimCeiling, in.Authority, in.SourceGraphComplete}
}

func sortedTypedOccurrences(in []TypedOccurrence) []TypedOccurrence {
	out := append([]TypedOccurrence(nil), in...)
	sort.Slice(out, func(i, j int) bool { return out[i].Identity < out[j].Identity })
	return out
}

func relationIndexBucket(all []TypedOccurrence, endpoint string, outgoing bool) []TypedOccurrence {
	var out []TypedOccurrence
	for _, occurrence := range all {
		selected := occurrence.To == endpoint
		if outgoing {
			selected = occurrence.From == endpoint
		}
		if selected {
			out = append(out, occurrence)
		}
	}
	return out
}

func relationIndexEndpoints(all []TypedOccurrence) []string {
	seen := make(map[string]bool)
	var out []string
	for _, occurrence := range all {
		for _, endpoint := range []string{occurrence.From, occurrence.To} {
			if !seen[endpoint] {
				seen[endpoint] = true
				out = append(out, endpoint)
			}
		}
	}
	sort.Strings(out)
	return out
}

func typedOccurrencesIdentitySorted(in []TypedOccurrence) bool {
	return sort.SliceIsSorted(in, func(i, j int) bool { return in[i].Identity < in[j].Identity })
}

func countTypedOccurrence(in []TypedOccurrence, want TypedOccurrence) int {
	count := 0
	for _, got := range in {
		if got == want {
			count++
		}
	}
	return count
}

func mutateTypedOccurrences(in []TypedOccurrence) {
	if len(in) != 0 {
		in[0] = TypedOccurrence{Identity: "mutated", RelationID: "mutated", Kind: "mutated", From: "mutated", To: "mutated", Ordinal: -1}
	}
}
