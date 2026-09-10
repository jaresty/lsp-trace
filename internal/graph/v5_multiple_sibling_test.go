package graph

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestV5MultipleSiblingProducerAndReplayAreCanonical(t *testing.T) {
	r := validV5SiblingResult()
	second := r.SiblingCandidates[0]
	second.Declaration = nodePointer(NewNode(Item{Name: "peer2", Kind: 6, URI: second.Candidate.URI, Range: Range{Start: Position{Line: 8}, End: Position{Line: 10}}, SelectionRange: Range{Start: Position{Line: 9}, End: Position{Line: 9, Character: 5}}}))
	second.Candidate = NewNode(Item{Name: "peer2(int)", Kind: 6, URI: second.Candidate.URI, Range: Range{Start: Position{Line: 9}, End: Position{Line: 9, Character: 5}}, SelectionRange: second.Declaration.SelectionRange})
	second.RelationID = ""
	r.SiblingCandidates = append(r.SiblingCandidates, second)
	// Model managed upgrade input: candidates arrive in historical relation-ID
	// order, before V5 recomputes the identity coordinate.
	r.SiblingCandidates[0], r.SiblingCandidates[1] = r.SiblingCandidates[1], r.SiblingCandidates[0]

	raw, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateSemanticBundle(raw); err != nil {
		t.Fatalf("producer output failed canonical replay: %v", err)
	}

	var decoded bundleV3
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatal(err)
	}
	marker := []byte(`,"trace_receipt":`)
	cut := bytes.LastIndex(raw, marker)
	if cut < 0 {
		t.Fatal("trace receipt marker missing")
	}
	producerSemantic := append(append([]byte(nil), raw[:cut]...), '}')
	replayedSemantic, err := json.Marshal(decoded.semanticV3)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(producerSemantic, replayedSemantic) {
		t.Fatal("producer and replay semantic bytes differ")
	}

	verified := Result{SchemaVersion: decoded.SchemaVersion, SiblingCandidates: append([]SiblingCandidate(nil), decoded.SiblingCandidates...)}
	verified.Canonicalize()
	expected := verified.evidenceReceipt(decoded.Invocation.Provenance.SourceRevision)
	if !equalEvidenceReceipts(decoded.EvidenceReceipt, expected) {
		t.Fatalf("producer evidence receipt differs from canonical replay: producer=%#v replay=%#v", decoded.EvidenceReceipt, expected)
	}
}

func TestV5ExecutionBundleIdentityReordersProjectedSiblings(t *testing.T) {
	r := validV5SiblingResult()
	second := r.SiblingCandidates[0]
	second.Declaration = nodePointer(NewNode(Item{Name: "peer2", Kind: 6, URI: second.Candidate.URI, Range: Range{Start: Position{Line: 8}, End: Position{Line: 10}}, SelectionRange: Range{Start: Position{Line: 9}, End: Position{Line: 9, Character: 5}}}))
	second.Candidate = NewNode(Item{Name: "peer2(int)", Kind: 6, URI: second.Candidate.URI, Range: Range{Start: Position{Line: 9}, End: Position{Line: 9, Character: 5}}, SelectionRange: second.Declaration.SelectionRange})
	input := []SiblingCandidate{r.SiblingCandidates[0], second}
	_, projected, _ := projectExecutionBundleRelations(SchemaVersionV5, "sha256:managed-v5-bundle", nil, input, nil)
	if projected[0].RelationID < projected[1].RelationID {
		input[0], input[1] = input[1], input[0]
		_, projected, _ = projectExecutionBundleRelations(SchemaVersionV5, "sha256:managed-v5-bundle", nil, input, nil)
	}
	if projected[0].RelationID > projected[1].RelationID {
		t.Fatalf("ASSERT_V5_EXECUTION_BUNDLE_PREIMAGE_RECOMPUTATION_RESORTS_SIBLINGS: first=%q second=%q", projected[0].RelationID, projected[1].RelationID)
	}
}

func nodePointer(n Node) *Node { return &n }

func equalEvidenceReceipts(a, b *EvidenceReceipt) bool {
	if a == nil || b == nil {
		return a == b
	}
	aj, _ := json.Marshal(a)
	bj, _ := json.Marshal(b)
	return bytes.Equal(aj, bj)
}
