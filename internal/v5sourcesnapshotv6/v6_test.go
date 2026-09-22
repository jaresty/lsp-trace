package v5sourcesnapshotv6

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"lsp-trace/internal/graph"
	"lsp-trace/internal/graphprovenance"
	"lsp-trace/internal/manageddiagnostic"
)

func TestV6FullDefinitionPreparedCustody(t *testing.T) {
	raw := []byte("package p\nfunc Target() {\n\tprintln(\"full\")\n}\n")
	selection := graph.Range{Start: graph.Position{Line: 1, Character: 5}, End: graph.Position{Line: 1, Character: 11}}
	item := graph.Range{Start: graph.Position{Line: 1}, End: graph.Position{Line: 1, Character: 13}}
	display := graph.Range{Start: graph.Position{Line: 1}, End: graph.Position{Line: 3, Character: 1}}
	carrier, node := v6Fixture(t, item, selection)
	result, err := CaptureSelected(context.Background(), CaptureInput{
		GraphV5Bytes: carrier, PositionEncoding: "utf-16", SessionID: "session", Generation: 1,
		Nominations: []Nomination{{ID: "n1", TargetNodeID: node.ID}},
		Documents:   []PreparedDocument{{URI: node.URI, Bytes: raw, Digest: digest(raw), ByteLength: uint64(len(raw)), Version: "7", SessionID: "session", Generation: 1, PositionEncoding: "utf-16"}},
		Resolver: ResolverFunc(func(_ context.Context, in ResolveRequest) (ResolveResult, error) {
			return ResolveResult{DisplayRange: display, ItemRange: item, SelectionRange: selection, ProvenanceKind: ProvenanceKind, Method: ProvenanceMethod, DocumentDigest: digest(raw), DocumentByteLength: uint64(len(raw)), DocumentVersion: "7"}, nil
		}), Limits: testLimits(),
	})
	if err != nil {
		t.Fatalf("ASSERT_V6_FULL_DEFINITION_BODY: %v", err)
	}
	var artifact Artifact
	if err := json.Unmarshal(result.Raw, &artifact); err != nil {
		t.Fatal(err)
	}
	if len(artifact.EndpointBindings) != 1 {
		t.Fatalf("ASSERT_V6_FULL_DEFINITION_BODY bindings=%d", len(artifact.EndpointBindings))
	}
	b := artifact.EndpointBindings[0]
	if b.EvidenceRange != item || b.ItemRange != item || b.SelectionRange != selection || b.DisplayRange != display {
		t.Fatalf("ASSERT_V6_PRESERVES_EXACT_RANGES: %#v", b)
	}
	if !bytes.Contains(artifact.Receipts[0].Content, []byte("println")) {
		t.Fatal("ASSERT_V6_FULL_DEFINITION_BODY")
	}
	if _, _, err := Replay(result.Raw, testLimits()); err != nil {
		t.Fatalf("ASSERT_V6_REPLAY_IMMUTABLE: %v", err)
	}
}

func TestV6RejectsPreparedDigestMismatch(t *testing.T) {
	raw := []byte("package p\nfunc Target() {}\n")
	r := graph.Range{Start: graph.Position{Line: 1}, End: graph.Position{Line: 1, Character: 16}}
	carrier, node := v6Fixture(t, r, r)
	_, err := CaptureSelected(context.Background(), CaptureInput{GraphV5Bytes: carrier, PositionEncoding: "utf-16", SessionID: "session", Generation: 1, Nominations: []Nomination{{ID: "n1", TargetNodeID: node.ID}}, Documents: []PreparedDocument{{URI: node.URI, Bytes: raw, Digest: digest([]byte("wrong")), ByteLength: uint64(len(raw)), Version: "1", SessionID: "session", Generation: 1, PositionEncoding: "utf-16"}}, Resolver: ResolverFunc(func(context.Context, ResolveRequest) (ResolveResult, error) {
		return ResolveResult{DisplayRange: r, ItemRange: r, SelectionRange: r, ProvenanceKind: ProvenanceKind, Method: ProvenanceMethod, DocumentDigest: digest(raw), DocumentByteLength: uint64(len(raw)), DocumentVersion: "1"}, nil
	}), Limits: testLimits()})
	if err == nil {
		t.Fatal("ASSERT_V6_REJECTS_DIGEST_MISMATCH")
	}
}

func TestV6OneLayerMultipleCallersFullBodiesAndWitnesses(t *testing.T) {
	targetBody := []byte("package p\nfunc Target() {\n println(\"target-full\")\n}\n")
	caller1Body := []byte("package p\nfunc CallerOne() {\n Target() // caller-one-full\n}\n")
	caller2Body := []byte("package p\nfunc CallerTwo() {\n Target() // caller-two-full\n}\n")
	mk := func(name, uri string, end uint32) graph.Node {
		return graph.NewNode(graph.Item{Name: name, Kind: 12, URI: uri, Range: graph.Range{Start: graph.Position{Line: 1}, End: graph.Position{Line: 1, Character: end}}, SelectionRange: graph.Range{Start: graph.Position{Line: 1, Character: 5}, End: graph.Position{Line: 1, Character: end - 3}}})
	}
	target, c1, c2 := mk("Target", "file:///target.go", 13), mk("CallerOne", "file:///caller1.go", 16), mk("CallerTwo", "file:///caller2.go", 16)
	call1 := graph.Range{Start: graph.Position{Line: 2, Character: 1}, End: graph.Position{Line: 2, Character: 7}}
	call2 := graph.Range{Start: graph.Position{Line: 2, Character: 1}, End: graph.Position{Line: 2, Character: 7}}
	edges := []graph.Edge{{RelationID: "r1", CallerNodeID: c1.ID, CalleeNodeID: target.ID, CallSites: []graph.Range{call1}}, {RelationID: "r2", CallerNodeID: c2.ID, CalleeNodeID: target.ID, CallSites: []graph.Range{call2}}}
	native, _ := json.Marshal(graph.Result{SchemaVersion: graph.SchemaVersionV5, Invocation: graph.Invocation{Server: graph.ServerInvocation{Command: "fake"}, Provenance: graph.InvocationProvenance{InvocationID: "session", SourceRevision: "commit", ServerVersion: "fake@1"}}, Nodes: []graph.Node{target, c1, c2}, Edges: edges, Summary: graph.Summary{Complete: true}})
	source := graphprovenance.EvidenceV2{SchemaVersion: graphprovenance.VersionV2, Policy: graphprovenance.PolicyV2, WorkspaceURI: "file:///workspace", AnalyzedVersion: graphprovenance.Unverified, DependencyCompleteness: "UNKNOWN_INCOMPLETE"}
	carrier, err := graphprovenance.CaptureV5(native, "session", 1, manageddiagnostic.QueryResult{Status: manageddiagnostic.QueryUnavailable}, &source)
	if err != nil {
		t.Fatal(err)
	}
	var retained graphprovenance.EvidenceV5
	_ = json.Unmarshal(carrier, &retained)
	retainedNative, _ := base64.StdEncoding.DecodeString(retained.GraphV5)
	var retainedGraph graph.Result
	_ = json.Unmarshal(retainedNative, &retainedGraph)
	retainedRelations := map[string]string{}
	for _, edge := range retainedGraph.Edges {
		retainedRelations[edge.CallerNodeID] = edge.RelationID
	}
	docs := []PreparedDocument{{URI: target.URI, Bytes: targetBody}, {URI: c1.URI, Bytes: caller1Body}, {URI: c2.URI, Bytes: caller2Body}}
	for i := range docs {
		docs[i].Digest = digest(docs[i].Bytes)
		docs[i].ByteLength = uint64(len(docs[i].Bytes))
		docs[i].Version = "9"
		docs[i].SessionID = "session"
		docs[i].Generation = 1
		docs[i].PositionEncoding = "utf-16"
	}
	displays := map[string]graph.Range{target.ID: {Start: graph.Position{Line: 1}, End: graph.Position{Line: 3, Character: 1}}, c1.ID: {Start: graph.Position{Line: 1}, End: graph.Position{Line: 3, Character: 1}}, c2.ID: {Start: graph.Position{Line: 1}, End: graph.Position{Line: 3, Character: 1}}}
	result, err := CaptureSelected(context.Background(), CaptureInput{GraphV5Bytes: carrier, PositionEncoding: "utf-16", SessionID: "session", Generation: 1, Documents: docs, Nominations: []Nomination{{ID: "n", TargetNodeID: target.ID, Incoming: []Occurrence{{RelationID: retainedRelations[c1.ID], OccurrenceID: "o1", CallerNodeID: c1.ID, CalleeNodeID: target.ID, Range: call1}, {RelationID: retainedRelations[c2.ID], OccurrenceID: "o2", CallerNodeID: c2.ID, CalleeNodeID: target.ID, Range: call2}}}}, Resolver: ResolverFunc(func(_ context.Context, in ResolveRequest) (ResolveResult, error) {
		return ResolveResult{DisplayRange: displays[in.GraphSubjectID], ItemRange: in.ItemRange, SelectionRange: in.SelectionRange, ProvenanceKind: ProvenanceKind, Method: ProvenanceMethod, DocumentDigest: in.DocumentDigest, DocumentByteLength: uint64(len(in.Bytes)), DocumentVersion: in.DocumentVersion}, nil
	}), Limits: testLimits()})
	if err != nil {
		t.Fatalf("ASSERT_V6_ONE_LAYER_CAPTURE: %v", err)
	}
	var a Artifact
	_ = json.Unmarshal(result.Raw, &a)
	if len(a.EndpointBindings) != 3 || len(a.RelationBindings) != 2 {
		t.Fatalf("ASSERT_V6_ONE_LAYER_SELECTED_ENDPOINTS endpoints=%d relations=%d", len(a.EndpointBindings), len(a.RelationBindings))
	}
	if a.RelationBindings[0].Range != call1 || a.RelationBindings[1].Range != call2 {
		t.Fatalf("ASSERT_V6_EXACT_CALL_SITE_WITNESSES: %#v", a.RelationBindings)
	}
	joined := append(append(append([]byte{}, a.Receipts[0].Content...), a.Receipts[1].Content...), a.Receipts[2].Content...)
	for _, marker := range [][]byte{[]byte("target-full"), []byte("caller-one-full"), []byte("caller-two-full")} {
		if !bytes.Contains(joined, marker) {
			t.Fatalf("ASSERT_V6_ALL_FULL_BODIES missing=%s", marker)
		}
	}
}

func TestV6CaptureResultJSONDoesNotDuplicateSourceObjects(t *testing.T) {
	raw := []byte("package p\nfunc Target() {}\n")
	r := graph.Range{Start: graph.Position{Line: 1}, End: graph.Position{Line: 1, Character: 16}}
	carrier, node := v6Fixture(t, r, r)
	result, err := CaptureSelected(context.Background(), CaptureInput{GraphV5Bytes: carrier, PositionEncoding: "utf-16", SessionID: "session", Generation: 1, Nominations: []Nomination{{ID: "n1", TargetNodeID: node.ID}}, Documents: []PreparedDocument{{URI: node.URI, Bytes: raw, Digest: digest(raw), ByteLength: uint64(len(raw)), Version: "1", SessionID: "session", Generation: 1, PositionEncoding: "utf-16"}}, Resolver: testResolver(r), Limits: testLimits()})
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Count(encoded, raw) != 0 || bytes.Contains(encoded, []byte(`"Objects"`)) {
		t.Fatalf("ASSERT_V6_CAPTURE_RESULT_NO_DUPLICATE_SOURCE_OBJECTS encoded=%d raw=%d", len(encoded), len(result.Raw))
	}
}

func TestV6RepeatedSourceChargesUniqueBytesOnce(t *testing.T) {
	raw := []byte("package p\nfunc Target() {}\n")
	r := graph.Range{Start: graph.Position{Line: 1}, End: graph.Position{Line: 1, Character: 16}}
	carrier, node := v6Fixture(t, r, r)
	doc := func(uri string) PreparedDocument {
		return PreparedDocument{URI: uri, Bytes: raw, Digest: digest(raw), ByteLength: uint64(len(raw)), Version: "1", SessionID: "session", Generation: 1, PositionEncoding: "utf-16"}
	}
	limits := testLimits()
	limits.MaxTotalSourceBytes = len(raw)
	_, err := CaptureSelected(context.Background(), CaptureInput{GraphV5Bytes: carrier, PositionEncoding: "utf-16", SessionID: "session", Generation: 1, Nominations: []Nomination{{ID: "n1", TargetNodeID: node.ID}}, Documents: []PreparedDocument{doc(node.URI), doc("file:///duplicate.go")}, Resolver: testResolver(r), Limits: limits})
	if err != nil {
		t.Fatalf("ASSERT_V6_REPEATED_SOURCE_CHARGED_ONCE: %v", err)
	}
}

func TestV6ExactObservedProgramCSizeReturnsTypedInputLimit(t *testing.T) {
	const observed = 25_639_706
	limits := testLimits()
	limits.MaxGraphBytes = 8 << 20
	_, err := CaptureSelected(context.Background(), CaptureInput{GraphV5Bytes: make([]byte, observed), Limits: limits})
	var limitErr *LimitError
	if !errors.As(err, &limitErr) || limitErr.Category != LimitCategoryInput || limitErr.Observed != observed || limitErr.Limit != 8<<20 {
		t.Fatalf("ASSERT_EXACT_PROGRAM_C_TYPED_CAPTURE_LIMIT: %#v err=%v", limitErr, err)
	}
}

func TestV6NominationCapacityBoundaryIsTypedBeforeResolution(t *testing.T) {
	r := graph.Range{Start: graph.Position{Line: 1}, End: graph.Position{Line: 1, Character: 16}}
	carrier, node := v6Fixture(t, r, r)
	makeNominations := func(n int) []Nomination {
		out := make([]Nomination, n)
		for i := range out {
			out[i] = Nomination{ID: fmt.Sprintf("n%04d", i), TargetNodeID: node.ID}
		}
		return out
	}
	limits := testLimits()
	limits.MaxBindings, limits.MaxOutcomes, limits.MaxReceipts, limits.MaxWork = 1000, 1000, 1000, 10000
	calls := 0
	resolver := ResolverFunc(func(context.Context, ResolveRequest) (ResolveResult, error) {
		calls++
		return ResolveResult{}, errors.New("unexpected resolution")
	})
	if _, err := CaptureSelected(context.Background(), CaptureInput{GraphV5Bytes: carrier, PositionEncoding: "utf-16", SessionID: "session", Generation: 1, Nominations: makeNominations(135), Resolver: resolver, Limits: limits}); err != nil {
		t.Fatalf("ASSERT_RETAINED_PROGRAM_C_135_ADMITTED: %v", err)
	}
	limits.MaxBindings, limits.MaxOutcomes = 135, 135
	_, err := CaptureSelected(context.Background(), CaptureInput{GraphV5Bytes: carrier, PositionEncoding: "utf-16", SessionID: "session", Generation: 1, Nominations: makeNominations(136), Resolver: resolver, Limits: limits})
	var limitErr *LimitError
	if !errors.As(err, &limitErr) || limitErr.Field != LimitFieldBindings || limitErr.Observed != 136 || limitErr.Limit != 135 || calls != 0 {
		t.Fatalf("ASSERT_V6_CAP_PLUS_ONE_PRE_RESOLUTION: calls=%d limit=%#v err=%v", calls, limitErr, err)
	}
}

func TestV6GraphInputLimitOwnedBySerializer(t *testing.T) {
	r := graph.Range{Start: graph.Position{Line: 1}, End: graph.Position{Line: 1, Character: 16}}
	carrier, node := v6Fixture(t, r, r)
	limits := testLimits()
	limits.MaxGraphBytes = len(carrier)
	if _, err := CaptureSelected(context.Background(), CaptureInput{GraphV5Bytes: carrier, PositionEncoding: "utf-16", SessionID: "session", Generation: 1, Nominations: []Nomination{{ID: "n1", TargetNodeID: node.ID}}, Resolver: testResolver(r), Limits: limits}); err != nil {
		t.Fatalf("ASSERT_V6_GRAPH_INPUT_LIMIT_EXACT_SUCCEEDS: %v", err)
	}
	limits.MaxGraphBytes = len(carrier) - 1
	_, err := CaptureSelected(context.Background(), CaptureInput{GraphV5Bytes: carrier, PositionEncoding: "utf-16", SessionID: "session", Generation: 1, Nominations: []Nomination{{ID: "n1", TargetNodeID: node.ID}}, Resolver: testResolver(r), Limits: limits})
	var limitErr *LimitError
	if !errors.As(err, &limitErr) || limitErr.Component != "v5sourcesnapshotv6.CaptureSelected" || limitErr.Limit != limits.MaxGraphBytes || limitErr.Observed != len(carrier) || limitErr.Category != LimitCategoryInput {
		t.Fatalf("ASSERT_V6_GRAPH_INPUT_LIMIT_OWNER: %#v", limitErr)
	}
}

func testResolver(r graph.Range) ResolverFunc {
	return func(_ context.Context, in ResolveRequest) (ResolveResult, error) {
		return ResolveResult{DisplayRange: r, ItemRange: r, SelectionRange: r, ProvenanceKind: ProvenanceKind, Method: ProvenanceMethod, DocumentDigest: in.DocumentDigest, DocumentByteLength: uint64(len(in.Bytes)), DocumentVersion: in.DocumentVersion}, nil
	}
}

func v6Fixture(t *testing.T, item, selection graph.Range) ([]byte, graph.Node) {
	t.Helper()
	node := graph.NewNode(graph.Item{Name: "Target", Kind: 12, URI: "file:///not/read/target.go", Range: item, SelectionRange: selection})
	native, _ := json.Marshal(graph.Result{SchemaVersion: graph.SchemaVersionV5, Invocation: graph.Invocation{Server: graph.ServerInvocation{Command: "fake"}, Provenance: graph.InvocationProvenance{InvocationID: "session", SourceRevision: "commit", ServerVersion: "fake@1"}}, Nodes: []graph.Node{node}, Summary: graph.Summary{Complete: true}})
	source := graphprovenance.EvidenceV2{SchemaVersion: graphprovenance.VersionV2, Policy: graphprovenance.PolicyV2, WorkspaceURI: "file:///workspace", AnalyzedVersion: graphprovenance.Unverified, DependencyCompleteness: "UNKNOWN_INCOMPLETE"}
	carrier, err := graphprovenance.CaptureV5(native, "session", 1, manageddiagnostic.QueryResult{Status: manageddiagnostic.QueryUnavailable}, &source)
	if err != nil {
		t.Fatal(err)
	}
	var retained graphprovenance.EvidenceV5
	_ = json.Unmarshal(carrier, &retained)
	decoded, _ := base64.StdEncoding.DecodeString(retained.GraphV5)
	var g graph.Result
	_ = json.Unmarshal(decoded, &g)
	return carrier, g.Nodes[0]
}
func testLimits() Limits {
	return Limits{MaxArtifactBytes: 16 << 20, MaxGraphBytes: 8 << 20, MaxReceipts: 100, MaxSourceBytes: 1 << 20, MaxTotalSourceBytes: 4 << 20, MaxBindings: 100, MaxOutcomes: 100, MaxWork: 1000}
}
