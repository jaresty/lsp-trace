package retainedrelations

import (
	"encoding/json"
	"strings"
	"testing"

	"lsp-trace/internal/graph"
	"lsp-trace/internal/graphprovenance"
	"lsp-trace/internal/manageddiagnostic"
)

func fixture(t *testing.T) []byte {
	t.Helper()
	uri := "file:///w/a.go"
	a := graph.NewNode(graph.Item{Name: "A", Kind: 6, URI: uri, Range: graph.Range{End: graph.Position{Line: 1}}, SelectionRange: graph.Range{End: graph.Position{Character: 1}}})
	declaration := graph.NewNode(graph.Item{Name: "DeclaredB", Kind: 6, URI: uri, Range: graph.Range{Start: graph.Position{Line: 2}, End: graph.Position{Line: 4}}, SelectionRange: graph.Range{Start: graph.Position{Line: 2}, End: graph.Position{Line: 2, Character: 1}}})
	prepared := graph.NewNode(graph.Item{Name: "PreparedB", Kind: 6, URI: uri, Range: graph.Range{Start: graph.Position{Line: 2}, End: graph.Position{Line: 3}}, SelectionRange: declaration.SelectionRange})
	seed := graph.InvocationSeed{Label: "one", At: "a.go:1:1", ResolvedURI: uri, ContentSHA256: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", LanguageID: "go"}
	r := graph.Result{SchemaVersion: graph.SchemaVersionV5, Invocation: graph.Invocation{Server: graph.ServerInvocation{Command: "fake-lsp"}, Seeds: []graph.InvocationSeed{seed}, Provenance: graph.InvocationProvenance{InvocationID: "session", SourceRevision: "commit", ServerVersion: "fake@1"}, Expansion: graph.ExpansionConfig{TopmostSiblings: true}}, Seeds: []graph.SeedResult{{Label: "one"}}, Summary: graph.Summary{Complete: true}}
	r.SiblingCandidates = []graph.SiblingCandidate{{SeedURI: uri, SeedLabel: "one", SeedIdentity: "session:one:a.go:1:1", Origin: a, Declaration: &declaration, Candidate: prepared, Direction: "SIBLING", Kind: "TOPMOST_SIBLING", ProviderEvidence: []string{"command=fake-lsp;server_version=fake@1;invocation=session"}, LSPEvidence: []string{"textDocument/documentSymbol", "textDocument/prepareCallHierarchy"}, SourceDigests: []string{"candidate=" + seed.ContentSHA256, "origin=" + seed.ContentSHA256}, Custody: graph.SourceCustodyEvidence{Class: graph.SourceCustodyCallerAssertedLocal, SourceContentSHA256: seed.ContentSHA256, ClaimCeiling: "NO_AUTHENTICATED_ANALYZED_SOURCE_IDENTITY"}}}
	native, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	parent, err := graphprovenance.CaptureV5(native, "session", 1, manageddiagnostic.QueryResult{Status: manageddiagnostic.QueryUnavailable, Records: []manageddiagnostic.Record{}})
	if err != nil {
		t.Fatal(err)
	}
	return parent
}
func TestExportExactV5SiblingAndMutations(t *testing.T) {
	raw, err := Export(fixture(t))
	if err != nil {
		t.Fatal("ASSERT_RETAINED_RELATIONS_V1_VALID_PASS", err)
	}
	var a Artifact
	if json.Unmarshal(raw, &a) != nil {
		t.Fatal("ASSERT_RETAINED_RELATIONS_V1_DECODE")
	}
	if len(a.Tables.Calls) != 0 || len(a.Tables.SiblingCandidates) != 1 || a.Tables.SiblingCandidates[0].Support != 0 || a.Tables.SiblingCandidates[0].EvidenceRole != "DISCOVERY_ONLY" {
		t.Fatal("ASSERT_SIBLING_SEPARATE_ZERO_CALLS_PASS")
	}
	sibling := a.Tables.SiblingCandidates[0]
	if sibling.Origin.ID == sibling.Declaration.ID || sibling.Declaration.ID == sibling.PreparedCandidate.ID || sibling.Declaration.Name != "DeclaredB" || sibling.PreparedCandidate.Name != "PreparedB" || sibling.Declaration.Range == sibling.PreparedCandidate.Range {
		t.Fatalf("ASSERT_SIBLING_EXACT_DISTINCT_IDENTITIES: %+v", sibling)
	}
	if _, err = Validate(raw); err != nil {
		t.Fatal("ASSERT_RETAINED_RELATIONS_V1_REPLAY_PASS", err)
	}
	cases := map[string]func(*Artifact){
		"parent_digest":   func(x *Artifact) { x.ParentDigest = "sha256:" + strings.Repeat("0", 64) },
		"sibling_support": func(x *Artifact) { x.Tables.SiblingCandidates[0].Support = 1 },
		"declaration_identity": func(x *Artifact) {
			x.Tables.SiblingCandidates[0].Declaration = x.Tables.SiblingCandidates[0].PreparedCandidate
		},
		"calls_inflation": func(x *Artifact) {
			x.Tables.Calls = append(x.Tables.Calls, Calls{RelationID: "forged", EvidenceRole: "CALL_SUPPORT", Support: 1})
		},
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			x := a
			x.Tables.Calls = append([]Calls{}, a.Tables.Calls...)
			x.Tables.SiblingCandidates = append([]Sibling{}, a.Tables.SiblingCandidates...)
			mutate(&x)
			bad, _ := json.Marshal(x)
			if _, err := Validate(bad); err == nil {
				t.Fatal("ASSERT_RETAINED_RELATIONS_V1_MUTATION_REJECTED", name)
			}
		})
	}
}
