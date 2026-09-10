package graph

import (
	"bytes"
	"encoding/json"
	"reflect"
	"sort"
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

func TestV5CaptureAndVerifyOwnImmutableBytes(t *testing.T) {
	r := validV5SiblingResult()
	second := r.SiblingCandidates[0]
	second.Declaration = nodePointer(NewNode(Item{Name: "peer2", Kind: 6, URI: second.Candidate.URI, Range: Range{Start: Position{Line: 8}, End: Position{Line: 10}}, SelectionRange: Range{Start: Position{Line: 9}, End: Position{Line: 9, Character: 5}}}))
	second.Candidate = NewNode(Item{Name: "peer2(int)", Kind: 6, URI: second.Candidate.URI, Range: Range{Start: Position{Line: 9}, End: Position{Line: 9, Character: 5}}, SelectionRange: second.Declaration.SelectionRange})
	third := second
	third.Declaration = nodePointer(NewNode(Item{Name: "peer3", Kind: 6, URI: third.Candidate.URI, Range: Range{Start: Position{Line: 11}, End: Position{Line: 13}}, SelectionRange: Range{Start: Position{Line: 12}, End: Position{Line: 12, Character: 5}}}))
	third.Candidate = NewNode(Item{Name: "peer3(int)", Kind: 6, URI: third.Candidate.URI, Range: Range{Start: Position{Line: 12}, End: Position{Line: 12, Character: 5}}, SelectionRange: third.Declaration.SelectionRange})
	r.SiblingCandidates = append(r.SiblingCandidates, second, third)
	before, err := cloneResultV5(r)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(r, before) {
		t.Fatal("ASSERT_V5_CAPTURE_SOURCE_RESULT_UNCHANGED")
	}
	carrier := append([]byte(nil), raw...)
	if err := ValidateSemanticBundle(carrier); err != nil {
		t.Fatal(err)
	}
	if err := ValidateSemanticBundle(carrier); err != nil {
		t.Fatalf("ASSERT_V5_REPEATED_VERIFY_STABLE: %v", err)
	}
	if !bytes.Equal(carrier, raw) {
		t.Fatal("ASSERT_V5_DECODED_CARRIER_BYTES_UNCHANGED")
	}
}

func TestV5ImmutableCarrierRejectsTamperedBytesAndDigest(t *testing.T) {
	raw, err := json.Marshal(validV5SiblingResult())
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateSemanticBundle(raw); err != nil {
		t.Fatalf("ASSERT_V5_VALID_CARRIER_VERIFIES: %v", err)
	}

	var bundle bundleV3
	if err := json.Unmarshal(raw, &bundle); err != nil {
		t.Fatal(err)
	}
	bundle.SiblingCandidates[0].Candidate.Name += " tampered"
	tamperedBytes, err := json.Marshal(bundle)
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateSemanticBundle(tamperedBytes); err == nil {
		t.Fatal("ASSERT_V5_TAMPERED_SEMANTIC_BYTES_REJECTED")
	}

	if err := json.Unmarshal(raw, &bundle); err != nil {
		t.Fatal(err)
	}
	bundle.TraceReceipt.SemanticCommitmentDigest = "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	tamperedDigest, err := json.Marshal(bundle)
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateSemanticBundle(tamperedDigest); err == nil {
		t.Fatal("ASSERT_V5_TAMPERED_DIGEST_REJECTED")
	}
}

func TestHistoricalSiblingIDsFollowIdentityAcrossThreeSiblingReorder(t *testing.T) {
	makeSibling := func(name, relationID string, line uint32) SiblingCandidate {
		n := NewNode(Item{Name: name, Kind: 6, URI: "file:///w/a.go", Range: Range{Start: Position{Line: line}, End: Position{Line: line, Character: 1}}, SelectionRange: Range{Start: Position{Line: line}, End: Position{Line: line, Character: 1}}})
		return SiblingCandidate{RelationID: relationID, SeedLabel: "seed", SeedURI: n.URI, Origin: n, Declaration: nodePointer(n), Candidate: n, Direction: "SIBLING", Kind: "TOPMOST_SIBLING"}
	}
	siblings := []SiblingCandidate{makeSibling("z", "historical-z", 3), makeSibling("a", "historical-a", 1), makeSibling("m", "historical-m", 2)}
	ids := captureHistoricalSiblingIDs(siblings)
	sort.Slice(siblings, func(i, j int) bool { return siblings[i].Candidate.Name < siblings[j].Candidate.Name })
	for i := range siblings {
		siblings[i].RelationID = "recomputed"
	}
	if err := restoreHistoricalSiblingIDs(siblings, ids); err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"a": "historical-a", "m": "historical-m", "z": "historical-z"}
	for _, sibling := range siblings {
		if sibling.RelationID != want[sibling.Candidate.Name] {
			t.Fatalf("ASSERT_HISTORICAL_SIBLING_ID_FOLLOWS_IDENTITY: name=%q id=%q", sibling.Candidate.Name, sibling.RelationID)
		}
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
