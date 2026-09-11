package hydratedinspection

import (
	"encoding/json"
	"testing"

	"lsp-trace/internal/graph"
	gp "lsp-trace/internal/graphprovenance"
	"lsp-trace/internal/manageddiagnostic"
)

func TestV2HydratesV5SiblingEndpointsFromRetainedSuppliesOffline(t *testing.T) {
	base := fixture(t)
	var source gp.EvidenceV2
	if err := json.Unmarshal([]byte(base.Input), &source); err != nil {
		t.Fatal(err)
	}
	g, err := graph.DecodeNativeV3(source.GraphBytes)
	if err != nil {
		t.Fatal(err)
	}
	if len(g.Nodes) < 2 {
		t.Fatal("fixture needs two nodes")
	}
	g.SchemaVersion = graph.SchemaVersionV5
	g.Invocation.Expansion.TopmostSiblings = true
	origin := g.Nodes[0]
	for _, n := range g.Nodes {
		if n.URI == g.Invocation.Seeds[0].ResolvedURI {
			origin = n
			break
		}
	}
	candidate := graph.NewNode(graph.Item{Name: "HydratedCandidate", Kind: origin.Kind, URI: origin.URI, Range: origin.Range, SelectionRange: origin.SelectionRange})
	relation := graph.SiblingCandidate{RelationID: "sibling:one", SeedURI: g.Invocation.Seeds[0].ResolvedURI, SeedLabel: g.Invocation.Seeds[0].Label, SeedIdentity: g.Invocation.Provenance.InvocationID + ":" + g.Invocation.Seeds[0].Label + ":" + g.Invocation.Seeds[0].At, Origin: origin, Candidate: candidate, Direction: "SIBLING", Kind: "TOPMOST_SIBLING", ProviderEvidence: []string{"command=" + g.Invocation.Server.Command + ";server_version=" + g.Invocation.Provenance.ServerVersion + ";invocation=" + g.Invocation.Provenance.InvocationID}, LSPEvidence: []string{"textDocument/documentSymbol", "textDocument/prepareCallHierarchy"}, SourceDigests: []string{"candidate=" + g.Invocation.Seeds[0].ContentSHA256, "origin=" + g.Invocation.Seeds[0].ContentSHA256}, Custody: graph.SourceCustodyEvidence{Class: graph.SourceCustodyCallerAssertedLocal, SourceContentSHA256: g.Invocation.Seeds[0].ContentSHA256, ClaimCeiling: "NO_AUTHENTICATED_ANALYZED_SOURCE_IDENTITY"}}
	g.SiblingCandidates = []graph.SiblingCandidate{relation}
	native, _ := json.Marshal(g)
	canonical, err := graph.DecodeNativeV3(native)
	if err != nil {
		t.Fatal(err)
	}
	requestedRelationID := canonical.SiblingCandidates[0].RelationID
	artifact, err := gp.CaptureV5(native, "session", 1, manageddiagnostic.QueryResult{Status: manageddiagnostic.QueryUnavailable, Records: []manageddiagnostic.Record{}}, &source)
	if err != nil {
		t.Fatal(err)
	}
	r := DefaultRequest()
	r.Input = string(artifact)
	r.SiblingRelationIDs = []string{requestedRelationID}
	r.IncludeBodies = true
	r.PositionEncoding = "utf-8"
	v, err := Inspect(r)
	if err != nil {
		t.Fatal(err)
	}
	if len(v.Manifest.Origins) != 1 || len(v.Manifest.Origins[0].Sites) != 6 || len(v.Bundle.Spans) == 0 {
		t.Fatalf("ASSERT_HYDRATED_V5_EXACT_SIBLING_SPANS: origins=%d sites=%d spans=%d", len(v.Manifest.Origins), len(v.Manifest.Origins[0].Sites), len(v.Bundle.Spans))
	}
	for _, span := range v.Bundle.Spans {
		if len(span.Content) == 0 {
			t.Fatal("ASSERT_HYDRATED_V5_BODY_OPT_IN")
		}
	}
	r.IncludeBodies = false
	without, err := Inspect(r)
	if err != nil {
		t.Fatal(err)
	}
	for _, span := range without.Bundle.Spans {
		if len(span.Content) != 0 {
			t.Fatal("ASSERT_HYDRATED_V5_BODY_DEFAULT_PRIVATE")
		}
	}

	absent := DefaultRequest()
	absent.Input = string(artifact)
	absent.NodeIDs = []string{"node:absent-despite-retained-source-bytes"}
	absent.IncludeBodies = true
	absent.PositionEncoding = "utf-8"
	missing, err := Inspect(absent)
	if err != nil {
		t.Fatal(err)
	}
	if len(missing.Manifest.Origins) != 1 || missing.Manifest.Origins[0].Status != "UNKNOWN_ID" || len(missing.Manifest.Origins[0].Sites) != 0 || len(missing.Bundle.Spans) != 0 {
		t.Fatalf("ASSERT_V5_RETAINED_BYTES_DO_NOT_CREATE_NODE_MEMBERSHIP: manifest=%#v spans=%#v", missing.Manifest, missing.Bundle.Spans)
	}
}
