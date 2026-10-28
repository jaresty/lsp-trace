package graph

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

// Fixed synthetic metadata, not environment/cwd observations of this verifier.
func retainedV3ProjectionFixturePrivate(t *testing.T, fileBacked bool) []byte {
	t.Helper()
	item := func(name string, line uint32) Node {
		span := Range{Start: Position{Line: line}, End: Position{Line: line, Character: 8}}
		return NewNode(Item{Name: name, Kind: 12, URI: "file:///fixture/retained.go", Range: span, SelectionRange: span, Data: json.RawMessage(`{"fixture":"raw-item-data"}`)})
	}
	a, b, c := item("caller", 0), item("callee", 3), item("empty", 7)
	first := Range{Start: Position{Line: 0, Character: 2}, End: Position{Line: 0, Character: 5}}
	later := Range{Start: Position{Line: 1, Character: 2}, End: Position{Line: 1, Character: 5}}
	seed := InvocationSeed{Label: "root", At: "retained.go:1:1", ResolvedURI: a.URI, ContentSHA256: "sha256:" + strings.Repeat("a", 64), LanguageID: "go"}
	inv := Invocation{WorkspaceURI: "file:///fixture", Server: ServerInvocation{Command: "fixture-server", Environment: map[string]string{"FIXTURE_TOKEN": "not-a-real-secret"}}, EffectiveEnvironment: []string{"FIXTURE_TOKEN=not-a-real-secret", "PATH=/fixture/frozen-bin"}, WorkingDirectory: "/fixture/original-cwd", Seeds: []InvocationSeed{seed}, Expansion: ExpansionConfig{TopmostSiblingOutcome: TopmostSiblingNotRequested}, Provenance: InvocationProvenance{InvocationID: "retained-fixture", SourceRevision: "caller-asserted-fixture-revision"}}
	if fileBacked {
		inv.Trace = TraceConfig{Enabled: true, Path: "/fixture/not-opened/trace.json", ContentSHA256: "sha256:" + strings.Repeat("b", 64)}
	}
	g := Result{SchemaVersion: SchemaVersionV3, Invocation: inv, Nodes: []Node{c, b, a}, Targets: []string{a.ID}, Edges: []Edge{{CallerNodeID: a.ID, CalleeNodeID: b.ID, CallSites: []Range{later, first, later}}}, Frontier: []Boundary{{NodeID: b.ID, Reason: MaxDepth, Message: "retained depth boundary", Provenance: "retained-provider"}}, Terminals: []Boundary{{NodeID: c.ID, Reason: NoIncomingCalls, Message: "retained successful empty", Provenance: "retained-provider"}}, Summary: Summary{Complete: false}, Capabilities: Capabilities{CallHierarchyProvider: true}}
	g.Canonicalize()
	g.Seeds = []SeedResult{{Label: seed.Label, PreparedTargetIDs: []string{a.ID}, ReachedNodeIDs: []string{a.ID, b.ID, c.ID}, ReachedRelationIDs: []string{g.Edges[0].RelationID}}}
	raw, err := json.Marshal(g)
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateSemanticBundle(raw); err != nil {
		t.Fatal("fixture semantic validity", err)
	}
	return raw
}

func TestRetainedV5PrivatePreservesNativeMetadata(t *testing.T) {
	for _, fileBacked := range []bool{false, true} {
		name := "identified-not-embedded"
		if fileBacked {
			name = "file-backed-metadata-no-read"
		}
		t.Run(name, func(t *testing.T) {
			original := retainedV3ProjectionFixturePrivate(t, fileBacked)
			var source bundleV3
			if err := json.Unmarshal(original, &source); err != nil {
				t.Fatal(err)
			}
			got, err := ProjectRetainedV3ToV5Private(original, source.Invocation.WorkspaceURI, source.Invocation.Server.Command)
			if err != nil {
				t.Fatal(err)
			}
			var derived bundleV3
			if err = json.Unmarshal(got, &derived); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(source.ProcessContext, derived.ProcessContext) {
				t.Errorf("ASSERT_RETAINED_PROCESS_CONTEXT_EXACT: original=%+v derived=%+v", source.ProcessContext, derived.ProcessContext)
			}
			if source.ProcessContext.EffectiveEnvironmentVariableCount != 2 || len(source.ProcessContext.Environment) != 1 {
				t.Fatal("non-default context witness collapsed")
			}
			if len(source.Edges) != 1 || len(source.Edges[0].CallSites) != 2 || source.Edges[0].CallSites[0].Start.Line != 0 || source.Edges[0].CallSites[1].Start.Line != 1 || len(source.Frontier) != 1 || len(source.Terminals) != 1 || len(source.SeedMemberships) == 0 {
				t.Fatal("ASSERT_CANONICAL_OCCURRENCE_AND_BOUNDARY_WITNESS_NONTRIVIAL")
			}
			if ValidateSemanticBundle(got) != nil {
				t.Fatal("ASSERT_DERIVED_NATIVE_SEMANTIC_VALID")
			}
			// The source already uses the enriched workspace/server/expansion. Only V5
			// schema/receipt coordinates are different; no metadata is normalized away.
			projected := source.semanticV3
			projected.SchemaVersion = SchemaVersionV5
			if !reflect.DeepEqual(projected, derived.semanticV3) {
				t.Error("ASSERT_RETAINED_NATIVE_FULL_FIELD_PROJECTION")
			}
			again, err := ProjectRetainedV3ToV5Private(original, source.Invocation.WorkspaceURI, source.Invocation.Server.Command)
			if err != nil || !bytes.Equal(got, again) {
				t.Fatal("ASSERT_RETAINED_NATIVE_DETERMINISTIC", err)
			}
			if !bytes.Equal(original, retainedV3ProjectionFixturePrivate(t, fileBacked)) {
				t.Fatal("ASSERT_RETAINED_SOURCE_UNCHANGED")
			}
		})
	}
}

func TestRetainedV5PrivateRefusesUnavailableContext(t *testing.T) {
	source := retainedV3ProjectionFixturePrivate(t, false)
	var b bundleV3
	if json.Unmarshal(source, &b) != nil {
		t.Fatal("fixture decode")
	}
	for _, variant := range []string{"unavailable", "unknown-state", "bad-environment-metadata"} {
		t.Run(variant, func(t *testing.T) {
			var original bundleV3
			if json.Unmarshal(source, &original) != nil {
				t.Fatal("fixture decode")
			}
			switch variant {
			case "unavailable":
				original.ProcessContext = ProcessContext{}
			case "unknown-state":
				original.ProcessContext.AmbientEnvironmentState = "UNKNOWN"
			case "bad-environment-metadata":
				original.ProcessContext.Environment[0].EnvironmentNameProcessContextDigest = "sha256:" + strings.Repeat("0", 64)
			}
			canonical, _ := json.Marshal(original.semanticV3)
			original.TraceReceipt.SemanticCommitmentDigest = domainDigest(SemanticDigestDomain, canonical)
			unavailable, _ := json.Marshal(original)
			if _, err := ProjectRetainedV3ToV5Private(unavailable, "file:///fixture", "fixture-server"); err == nil {
				t.Fatal("ASSERT_UNAVAILABLE_CONTEXT_NO_FALLBACK", variant)
			}
		})
	}
	if _, err := ProjectRetainedV3ToV5Private(make([]byte, (4<<20)+1), "file:///fixture", "fixture-server"); err == nil {
		t.Fatal("ASSERT_RETAINED_NATIVE_FINITE_BOUND")
	}
}
