package graph

import (
	"encoding/json"
	"strings"
	"testing"
)

func validV5SiblingResult() Result {
	uri := "file:///w/chart.go"
	origin := NewNode(Item{Name: "render", Kind: 6, URI: uri, Range: Range{End: Position{Line: 2}}, SelectionRange: Range{End: Position{Character: 6}}})
	candidate := NewNode(Item{Name: "index", Kind: 6, URI: uri, Range: Range{Start: Position{Line: 3}, End: Position{Line: 5}}, SelectionRange: Range{Start: Position{Line: 3}, End: Position{Line: 3, Character: 5}}})
	seed := InvocationSeed{Label: "chart", At: "chart.go:1:1", ResolvedURI: uri, ContentSHA256: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", LanguageID: "go"}
	invocation := Invocation{Server: ServerInvocation{Command: "fake-lsp"}, Seeds: []InvocationSeed{seed}, Provenance: InvocationProvenance{InvocationID: "session-1", SourceRevision: "commit-1", ServerVersion: "fake@1"}, Expansion: ExpansionConfig{TopmostSiblings: true}}
	return Result{SchemaVersion: SchemaVersionV5, Invocation: invocation, SiblingCandidates: []SiblingCandidate{{SeedURI: uri, SeedLabel: seed.Label, SeedIdentity: "session-1:chart:chart.go:1:1", Origin: origin, Candidate: candidate, Direction: "SIBLING", Kind: "TOPMOST_SIBLING", ProviderEvidence: []string{"command=fake-lsp;server_version=fake@1;invocation=session-1"}, LSPEvidence: []string{"textDocument/documentSymbol", "textDocument/prepareCallHierarchy"}, SourceDigests: []string{"candidate=" + seed.ContentSHA256, "origin=" + seed.ContentSHA256}, Custody: SourceCustodyEvidence{Class: SourceCustodyCallerAssertedLocal, SourceContentSHA256: seed.ContentSHA256, AuthenticatedAnalyzedSourceIdentity: false, ClaimCeiling: "NO_AUTHENTICATED_ANALYZED_SOURCE_IDENTITY"}}}, Seeds: []SeedResult{{Label: seed.Label}}, Summary: Summary{Complete: true}}
}

func TestV5SiblingEvidenceRoundTripAndFieldMutations(t *testing.T) {
	valid := validV5SiblingResult()
	encoded, err := json.Marshal(valid)
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateSemanticBundle(encoded); err != nil {
		t.Fatalf("ASSERT_V5_EXACT_SIBLING_EVIDENCE_ACCEPTED: %v", err)
	}
	mutations := []struct {
		name string
		edit func(*SiblingCandidate)
	}{
		{"seed uri", func(c *SiblingCandidate) { c.SeedURI = "file:///w/other.go" }},
		{"seed label", func(c *SiblingCandidate) { c.SeedLabel = "other" }},
		{"seed identity", func(c *SiblingCandidate) { c.SeedIdentity = "other" }},
		{"origin", func(c *SiblingCandidate) { c.Origin = c.Candidate }},
		{"candidate", func(c *SiblingCandidate) { c.Candidate = c.Origin }},
		{"direction", func(c *SiblingCandidate) { c.Direction = "OTHER" }},
		{"kind", func(c *SiblingCandidate) { c.Kind = "OTHER" }},
		{"provider evidence", func(c *SiblingCandidate) { c.ProviderEvidence = []string{"other"} }},
		{"lsp evidence", func(c *SiblingCandidate) { c.LSPEvidence = []string{"other"} }},
		{"source digests", func(c *SiblingCandidate) { c.SourceDigests = []string{"candidate=sha256:other", "origin=sha256:other"} }},
		{"custody", func(c *SiblingCandidate) { c.Custody.Class = "OTHER" }},
		{"forged verified host", func(c *SiblingCandidate) { c.Custody.Class = SourceCustodyVerifiedHost }},
		{"authenticated overclaim", func(c *SiblingCandidate) { c.Custody.AuthenticatedAnalyzedSourceIdentity = true }},
		{"digest inconsistency", func(c *SiblingCandidate) {
			c.Custody.SourceContentSHA256 = "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
		}},
	}
	for _, tc := range mutations {
		t.Run(tc.name, func(t *testing.T) {
			mutated := validV5SiblingResult()
			tc.edit(&mutated.SiblingCandidates[0])
			raw, marshalErr := json.Marshal(mutated)
			if marshalErr != nil {
				return // closed structural fields may reject during producer validation
			}
			if err := ValidateSemanticBundle(raw); err == nil || (!strings.Contains(err.Error(), "v5 sibling") && !strings.Contains(err.Error(), "embedded sibling")) {
				t.Fatalf("ASSERT_V5_SIBLING_FIELD_MUTATION_REJECTED_%s: %v", tc.name, err)
			}
		})
	}
}

func TestV5SiblingRejectsRehashedBundleRelationAndMembershipMutations(t *testing.T) {
	raw, err := json.Marshal(validV5SiblingResult())
	if err != nil {
		t.Fatal(err)
	}
	var base bundleV3
	if err := json.Unmarshal(raw, &base); err != nil {
		t.Fatal(err)
	}
	rehash := func(b bundleV3) []byte {
		canonical, _ := json.Marshal(b.semanticV3)
		_, domain, scope := semanticReceiptCoordinates(SchemaVersionV5)
		b.TraceReceipt = semanticReceiptV3{"lsp-trace.semantic-receipt.v5", domainDigest(domain, canonical), scope}
		out, _ := json.Marshal(b)
		return out
	}
	cases := []struct {
		name string
		edit func(*bundleV3)
	}{
		{"bundle", func(b *bundleV3) {
			b.ExecutionBundleID = "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
		}},
		{"relation id", func(b *bundleV3) {
			b.SiblingCandidates[0].RelationID = "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
		}},
		{"membership", func(b *bundleV3) {
			b.SeedMemberships[0].EndpointID = "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			b := base
			b.SiblingCandidates = append([]SiblingCandidate(nil), base.SiblingCandidates...)
			b.SeedMemberships = append([]SeedMembership(nil), base.SeedMemberships...)
			tc.edit(&b)
			if ValidateSemanticBundle(rehash(b)) == nil {
				t.Fatalf("ASSERT_V5_%s_MUTATION_REJECTED", tc.name)
			}
		})
	}
}
