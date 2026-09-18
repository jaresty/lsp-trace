package retainedmanifest

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"lsp-trace/internal/graph"
	"lsp-trace/internal/graphprovenance"
	"lsp-trace/internal/manageddiagnostic"
	"lsp-trace/internal/schema"
	"lsp-trace/internal/sourceobject"
	"lsp-trace/internal/sourceprojection"
)

const (
	assertExactV5      = "ASSERT_R01_EXACT_V5_BINDING"
	assertAvailability = "ASSERT_R01_EXACT_AVAILABILITY_IDENTITY"
	assertCanonical    = "ASSERT_R01_CANONICAL_BYTES"
	assertCeiling      = "ASSERT_R01_CUSTODY_AVAILABILITY_ONLY"
	assertStrict       = "ASSERT_R01_STRICT_ADMISSION"
	assertPredecessor  = "ASSERT_R01_PREDECESSOR_BYTES_AND_READERS_PRESERVED"
)

func r01Fixture(t *testing.T) ([]byte, []byte, []EntryInput) {
	t.Helper()
	uri := "file:///w/a.go"
	origin := graph.NewNode(graph.Item{Name: "A", Kind: 6, URI: uri, Range: graph.Range{End: graph.Position{Character: 1}}, SelectionRange: graph.Range{End: graph.Position{Character: 1}}})
	candidate := graph.NewNode(graph.Item{Name: "B", Kind: 6, URI: uri, Range: graph.Range{Start: graph.Position{Line: 2}, End: graph.Position{Line: 2, Character: 1}}, SelectionRange: graph.Range{Start: graph.Position{Line: 2}, End: graph.Position{Line: 2, Character: 1}}})
	seed := graph.InvocationSeed{Label: "seed", At: "a.go:1:1", ResolvedURI: uri, ContentSHA256: "sha256:" + strings.Repeat("a", 64), LanguageID: "go"}
	g := graph.Result{SchemaVersion: graph.SchemaVersionV5, Invocation: graph.Invocation{Server: graph.ServerInvocation{Command: "fake-lsp"}, Seeds: []graph.InvocationSeed{seed}, Provenance: graph.InvocationProvenance{InvocationID: "session", SourceRevision: "commit", ServerVersion: "fake@1"}, Expansion: graph.ExpansionConfig{TopmostSiblings: true}}, Seeds: []graph.SeedResult{{Label: "seed"}}, Summary: graph.Summary{Complete: true}, Capabilities: graph.Capabilities{CallHierarchyProvider: true}}
	g.SiblingCandidates = []graph.SiblingCandidate{{SeedURI: uri, SeedLabel: "seed", SeedIdentity: "session:seed:a.go:1:1", Origin: origin, Declaration: &candidate, Candidate: candidate, Direction: "SIBLING", Kind: "TOPMOST_SIBLING", ProviderEvidence: []string{"command=fake-lsp;server_version=fake@1;invocation=session"}, LSPEvidence: []string{"textDocument/documentSymbol", "textDocument/prepareCallHierarchy"}, SourceDigests: []string{"candidate=" + seed.ContentSHA256, "origin=" + seed.ContentSHA256}, Custody: graph.SourceCustodyEvidence{Class: graph.SourceCustodyCallerAssertedLocal, SourceContentSHA256: seed.ContentSHA256, ClaimCeiling: "NO_AUTHENTICATED_ANALYZED_SOURCE_IDENTITY"}}}
	graphBytes, err := json.Marshal(g)
	if err != nil {
		t.Fatal(err)
	}
	source := graphprovenance.EvidenceV2{SchemaVersion: graphprovenance.VersionV2, Policy: graphprovenance.PolicyV2, WorkspaceURI: "file:///w", AnalyzedVersion: graphprovenance.Unverified, DependencyCompleteness: "UNKNOWN_INCOMPLETE", Supplies: []graphprovenance.SupplyReceiptV2{}, Captures: []graphprovenance.Receipt{}, Bindings: []graphprovenance.BindingV2{}}
	capture, err := graphprovenance.CaptureV5(graphBytes, "session", 1, manageddiagnostic.QueryResult{Status: manageddiagnostic.QueryUnavailable, Records: []manageddiagnostic.Record{}}, &source)
	if err != nil {
		t.Fatal(err)
	}
	entries := []EntryInput{
		{Source: sourceobject.Identity{Digest: "sha256:" + strings.Repeat("b", 64), ByteLength: 11}, StorageClass: "CONTENT_ADDRESS", Qualification: "QUALIFIED", PrivacyClassification: "PUBLIC", Availability: "AVAILABLE", Role: "ENDPOINT", GraphSubjectID: "node-a", LogicalSourceID: uri, Range: sourceprojection.Range{End: sourceprojection.Position{Character: 1}}, PositionEncoding: "utf-16", PolicyID: "policy-1", CustodyIdentity: "object:b"},
		{Source: sourceobject.Identity{Digest: "sha256:" + strings.Repeat("c", 64), ByteLength: 13}, StorageClass: "GIT_BLOB", Qualification: "QUALIFIED", PrivacyClassification: "PUBLIC", Availability: "AVAILABLE", Role: "RELATION", GraphSubjectID: "relation-a-b", OccurrenceID: "occurrence-1", LogicalSourceID: uri, Range: sourceprojection.Range{Start: sourceprojection.Position{Line: 2}, End: sourceprojection.Position{Line: 2, Character: 1}}, PositionEncoding: "utf-8", PolicyID: "policy-1", CustodyIdentity: "commit:tree:blob"},
	}
	return graphBytes, capture, entries
}

func TestR01ExactV5AndAvailabilityBinding(t *testing.T) {
	graphBytes, capture, entries := r01Fixture(t)
	raw, id, err := Build(graphBytes, capture, entries)
	if err != nil {
		t.Fatalf("%s: %v", assertExactV5, err)
	}
	manifest, admittedID, err := Admit(raw, graphBytes, capture)
	if err != nil || id != admittedID {
		t.Fatalf("%s: id=%s admitted=%s err=%v", assertExactV5, id, admittedID, err)
	}
	if manifest.Graph.SchemaID != graphprovenance.GraphV5SchemaID || manifest.Graph.Digest != digest(graphBytes) || manifest.Graph.ByteLength != len(graphBytes) || manifest.Graph.CaptureID != digest(capture) {
		t.Fatalf("%s: %+v", assertExactV5, manifest.Graph)
	}
	if len(manifest.Entries) != len(entries) {
		t.Fatalf("%s: %+v", assertAvailability, manifest.Entries)
	}
	for i, entry := range manifest.Entries {
		if entry.EntryID != entryID(entry.EntryInput) {
			t.Fatalf("%s[%d]: %+v", assertAvailability, i, entry)
		}
	}
}

func TestR01CanonicalBytesUnderPermutation(t *testing.T) {
	graphBytes, capture, entries := r01Fixture(t)
	raw, id, err := Build(graphBytes, capture, entries)
	if err != nil {
		t.Fatal(err)
	}
	permutedRaw, permutedID, err := Build(graphBytes, capture, []EntryInput{entries[1], entries[0]})
	if err != nil || !bytes.Equal(raw, permutedRaw) || id != permutedID {
		t.Fatalf("%s: id=%s permuted=%s err=%v", assertCanonical, id, permutedID, err)
	}
}

func TestR01CustodyAvailabilityOnlySemanticCeiling(t *testing.T) {
	graphBytes, capture, entries := r01Fixture(t)
	raw, _, err := Build(graphBytes, capture, entries)
	if err != nil {
		t.Fatal(err)
	}
	var manifest Manifest
	if err := json.Unmarshal(raw, &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.SemanticOwner != SemanticOwner || manifest.Authority != 0 || manifest.Accepted || manifest.SourceGraphComplete != "UNKNOWN" || manifest.GraphFactsAdded != 0 {
		t.Fatalf("%s: %+v", assertCeiling, manifest)
	}
}

func TestR01EveryAvailabilityIdentityFieldIsSensitive(t *testing.T) {
	graphBytes, capture, entries := r01Fixture(t)
	baseRaw, baseID, err := Build(graphBytes, capture, entries)
	if err != nil {
		t.Fatal(err)
	}
	mutations := []struct {
		name   string
		mutate func(*EntryInput)
	}{
		{"source-digest", func(e *EntryInput) { e.Source.Digest = "sha256:" + strings.Repeat("d", 64) }},
		{"source-length", func(e *EntryInput) { e.Source.ByteLength++ }},
		{"storage-class", func(e *EntryInput) { e.StorageClass = "EMBEDDED_IMMUTABLE" }},
		{"qualification", func(e *EntryInput) { e.Qualification = "UNQUALIFIED" }},
		{"privacy", func(e *EntryInput) { e.PrivacyClassification = "RESTRICTED" }},
		{"availability", func(e *EntryInput) { e.Availability = "WITHHELD" }},
		{"role", func(e *EntryInput) { e.Role = "ANCILLARY" }},
		{"graph-subject", func(e *EntryInput) { e.GraphSubjectID = "node-other" }},
		{"logical-source", func(e *EntryInput) { e.LogicalSourceID = "file:///w/other.go" }},
		{"range", func(e *EntryInput) { e.Range.End.Character++ }},
		{"encoding", func(e *EntryInput) { e.PositionEncoding = "utf-32" }},
		{"policy", func(e *EntryInput) { e.PolicyID = "policy-2" }},
		{"custody", func(e *EntryInput) { e.CustodyIdentity = "object:other" }},
	}
	for _, tc := range mutations {
		t.Run(tc.name, func(t *testing.T) {
			changed := append([]EntryInput(nil), entries...)
			tc.mutate(&changed[0])
			raw, id, err := Build(graphBytes, capture, changed)
			if err != nil || id == baseID || bytes.Equal(raw, baseRaw) {
				t.Fatalf("%s_%s: id=%s err=%v", assertAvailability, tc.name, id, err)
			}
		})
	}
}

func TestR01StrictAdmissionMutationCorpus(t *testing.T) {
	graphBytes, capture, entries := r01Fixture(t)
	raw, _, err := Build(graphBytes, capture, entries)
	if err != nil {
		t.Fatal(err)
	}
	var base map[string]any
	if err := json.Unmarshal(raw, &base); err != nil {
		t.Fatal(err)
	}
	mutations := []struct {
		name   string
		mutate func(map[string]any) []byte
	}{
		{"unknown", func(m map[string]any) []byte { m["unknown"] = true; b, _ := json.Marshal(m); return b }},
		{"missing", func(m map[string]any) []byte { delete(m, "semantic_owner"); b, _ := json.Marshal(m); return b }},
		{"semantic-owner", func(m map[string]any) []byte {
			m["semantic_owner"] = "PROJECTION_SEMANTICS"
			b, _ := json.Marshal(m)
			return b
		}},
		{"authority", func(m map[string]any) []byte { m["authority"] = 1; b, _ := json.Marshal(m); return b }},
		{"reordered", func(m map[string]any) []byte {
			a := m["entries"].([]any)
			a[0], a[1] = a[1], a[0]
			b, _ := json.Marshal(m)
			return b
		}},
		{"entry-identity", func(m map[string]any) []byte {
			m["entries"].([]any)[0].(map[string]any)["entry_id"] = "sha256:" + strings.Repeat("0", 64)
			b, _ := json.Marshal(m)
			return b
		}},
	}
	for _, tc := range mutations {
		t.Run(tc.name, func(t *testing.T) {
			cloneRaw, _ := json.Marshal(base)
			var clone map[string]any
			_ = json.Unmarshal(cloneRaw, &clone)
			if _, _, err := Admit(tc.mutate(clone), graphBytes, capture); err == nil {
				t.Fatalf("%s_%s", assertStrict, tc.name)
			}
		})
	}
	duplicate := bytes.Replace(raw, []byte(`"schema_version":"`), []byte(`"schema_version":"`+Version+`","schema_version":"`), 1)
	if _, _, err := Admit(duplicate, graphBytes, capture); err == nil {
		t.Fatalf("%s_DUPLICATE", assertStrict)
	}
	wrongGraph := append([]byte(nil), graphBytes...)
	wrongGraph[len(wrongGraph)-1] ^= 1
	if _, _, err := Admit(raw, wrongGraph, capture); err == nil {
		t.Fatalf("%s_GRAPH_SUBSTITUTION", assertStrict)
	}
	wrongCapture := append([]byte(nil), capture...)
	wrongCapture[len(wrongCapture)-1] ^= 1
	if _, _, err := Admit(raw, graphBytes, wrongCapture); err == nil {
		t.Fatalf("%s_CAPTURE_SUBSTITUTION", assertStrict)
	}
}

func TestR01PredecessorSchemaBytesAndReadersRemainIndependent(t *testing.T) {
	for _, tc := range []struct{ family, version, want string }{
		{schema.FamilyGraphProvenance, "v2", "cf36372444c7203d29ce836acfcdda78a8a884ea6733122c0aa3159aa9d0ea03"},
		{schema.FamilyGraphProvenance, "v3", "3f993f318248891d6fa6ee7dc79dda23d7c0d515df6532f8af1edb5448556bfc"},
		{schema.FamilyGraph, "v2", "9df7b845828aab53ee4a2aff8b33711c3547b0dca4e77b08fda9ef98a72bcefb"},
		{schema.FamilyGraph, "v3", "a0f35f8e1d637eee40447ee8e4bd2fe8bf56d12ed78242e86f8e7bf275cacc7c"},
	} {
		raw, err := schema.BytesFor(tc.family, tc.version)
		got := fmt.Sprintf("%x", sha256.Sum256(raw))
		if err != nil || got != tc.want {
			t.Fatalf("%s_%s_%s: got=%s err=%v", assertPredecessor, tc.family, tc.version, got, err)
		}
	}
	if reflect.ValueOf(graphprovenance.ValidateFor).Kind() != reflect.Func {
		t.Fatal(assertPredecessor)
	}
}
