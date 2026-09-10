package graphprovenance

import (
	"encoding/base64"
	"encoding/json"
	"testing"

	"lsp-trace/internal/graph"
	"lsp-trace/internal/manageddiagnostic"
)

func v5Fixture(t *testing.T) ([]byte, []byte) {
	t.Helper()
	uri := "file:///w/a.go"
	a := graph.NewNode(graph.Item{Name: "A", Kind: 6, URI: uri, Range: graph.Range{End: graph.Position{Character: 1}}, SelectionRange: graph.Range{End: graph.Position{Character: 1}}})
	b := graph.NewNode(graph.Item{Name: "B", Kind: 6, URI: uri, Range: graph.Range{Start: graph.Position{Line: 2}, End: graph.Position{Line: 2, Character: 1}}, SelectionRange: graph.Range{Start: graph.Position{Line: 2}, End: graph.Position{Line: 2, Character: 1}}})
	seed := graph.InvocationSeed{Label: "seed", At: "a.go:1:1", ResolvedURI: uri, ContentSHA256: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", LanguageID: "go"}
	r := graph.Result{SchemaVersion: graph.SchemaVersionV5, Invocation: graph.Invocation{Server: graph.ServerInvocation{Command: "fake-lsp"}, Seeds: []graph.InvocationSeed{seed}, Provenance: graph.InvocationProvenance{InvocationID: "session", SourceRevision: "commit", ServerVersion: "fake@1"}, Expansion: graph.ExpansionConfig{TopmostSiblings: true}}, Seeds: []graph.SeedResult{{Label: "seed"}}, Summary: graph.Summary{Complete: true}, Capabilities: graph.Capabilities{CallHierarchyProvider: true}}
	r.SiblingCandidates = []graph.SiblingCandidate{{SeedURI: uri, SeedLabel: "seed", SeedIdentity: "session:seed:a.go:1:1", Origin: a, Declaration: &b, Candidate: b, Direction: "SIBLING", Kind: "TOPMOST_SIBLING", ProviderEvidence: []string{"command=fake-lsp;server_version=fake@1;invocation=session"}, LSPEvidence: []string{"textDocument/documentSymbol", "textDocument/prepareCallHierarchy"}, SourceDigests: []string{"candidate=" + seed.ContentSHA256, "origin=" + seed.ContentSHA256}, Custody: graph.SourceCustodyEvidence{Class: graph.SourceCustodyCallerAssertedLocal, SourceContentSHA256: seed.ContentSHA256, AuthenticatedAnalyzedSourceIdentity: false, ClaimCeiling: "NO_AUTHENTICATED_ANALYZED_SOURCE_IDENTITY"}}}
	native, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := CaptureV5(native, "session", 1, manageddiagnostic.QueryResult{Status: manageddiagnostic.QueryUnavailable, Records: []manageddiagnostic.Record{}})
	if err != nil {
		t.Fatal(err)
	}
	return raw, native
}

func TestV5ExactNativeCarrierAndMutations(t *testing.T) {
	raw, native := v5Fixture(t)
	if got, err := ValidateFor(raw, Family, "v5"); err != nil || got != VersionV5 {
		t.Fatalf("ASSERT_V5_CANONICAL_PASS: got=%q err=%v", got, err)
	}
	var e EvidenceV5
	if err := json.Unmarshal(raw, &e); err != nil {
		t.Fatal(err)
	}
	decoded, _ := base64.StdEncoding.DecodeString(e.GraphV5)
	if string(decoded) != string(native) {
		t.Fatal("ASSERT_V5_EXACT_EMBEDDED_BYTES")
	}
	mutations := map[string]func(*EvidenceV5){
		"digest": func(x *EvidenceV5) {
			x.GraphV5SHA256 = "sha256:0000000000000000000000000000000000000000000000000000000000000000"
		},
		"schema": func(x *EvidenceV5) { x.GraphV5SchemaID = "https://example.invalid/substitute" },
		"bytes":  func(x *EvidenceV5) { x.GraphV5 = base64.StdEncoding.EncodeToString(append(decoded, ' ')) },
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			x := e
			mutate(&x)
			bad, _ := json.Marshal(x)
			if _, err := ValidateFor(bad, Family, "v5"); err == nil {
				t.Fatal("ASSERT_V5_MUTATION_REJECTED")
			}
		})
	}
}

func TestV5RejectsSiblingEndpointEvidenceAndMembershipMutations(t *testing.T) {
	raw, _ := v5Fixture(t)
	var e EvidenceV5
	_ = json.Unmarshal(raw, &e)
	native, _ := base64.StdEncoding.DecodeString(e.GraphV5)
	var doc map[string]any
	_ = json.Unmarshal(native, &doc)
	for _, field := range []string{"sibling_candidates", "seed_memberships"} {
		t.Run(field, func(t *testing.T) {
			var x map[string]any
			_ = json.Unmarshal(native, &x)
			x[field] = []any{}
			changed, _ := json.Marshal(x)
			if _, err := CaptureV5(changed, "session", 1, manageddiagnostic.QueryResult{Status: manageddiagnostic.QueryUnavailable, Records: []manageddiagnostic.Record{}}); err == nil {
				t.Fatal("ASSERT_V5_RELATION_MUTATION_REJECTED")
			}
		})
	}
}

func TestV3DispatchUnchanged(t *testing.T) {
	if _, err := ValidateFor([]byte(`{}`), Family, "v3"); err == nil {
		t.Fatal("ASSERT_V3_STILL_STRICT")
	}
}
