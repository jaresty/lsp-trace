package featureattribution

import (
	"bytes"
	"strings"
	"testing"
)

func testRange() Range {
	return Range{Start: Position{Line: 1, Character: 2}, End: Position{Line: 1, Character: 4}}
}
func testSource() SourceIdentity {
	return SourceIdentity{LogicalURI: "file:///a.go", Digest: "sha256:" + strings.Repeat("a", 64), ByteLength: 10, PositionEncoding: "utf-16", DocumentVersion: 3, Revision: "abc", RevisionCustody: "PROVIDER_VERIFIED"}
}
func graphAnchor(id string) Anchor {
	return Anchor{Kind: AnchorGraphSubject, GraphSubject: &GraphSubjectAnchor{GraphSubjectID: id, EvidenceRange: testRange(), ItemRange: testRange(), SelectionRange: testRange(), DisplayRange: testRange(), Source: testSource()}}
}

func TestAnchorUnionRejectsUnknownAndMultiplePayloads(t *testing.T) {
	for _, a := range []Anchor{{Kind: "NAME"}, {Kind: AnchorGraphSubject, GraphSubject: graphAnchor("g").GraphSubject, SourceRange: &SourceRangeAnchor{Source: testSource(), Range: testRange()}}} {
		if err := a.Validate(); err == nil {
			t.Fatalf("anchor union accepted %#v", a)
		}
	}
}

func TestReceiptRequiresTerminalAccountingAndProvenance(t *testing.T) {
	r := LocatorResolutionReceipt{Schema: "receipt/v1", RequestID: "sha256:" + strings.Repeat("b", 64), Entries: []ResolutionEntry{{LocatorID: "l1", Disposition: DispositionResolved, Anchor: ptrAnchor(graphAnchor("g")), CandidateCount: 1}}, Accounting: ReceiptAccounting{Submitted: 1}}
	if err := r.Validate(); err == nil {
		t.Fatal("receipt accepted missing provenance and unequal accounting")
	}
	r.Provenance = ResolutionProvenance{SessionID: "s", Generation: 1, WorkspaceRevision: "abc", RevisionCustody: "PROVIDER_VERIFIED", Provider: ProviderIdentity{Name: "gopls", Version: "v", InstanceID: "i"}}
	r.Accounting = ReceiptAccounting{Submitted: 1, Resolved: 1, Candidates: 1, Work: 1, Bytes: 1}
	if err := r.Validate(); err != nil {
		t.Fatalf("valid receipt rejected: %v", err)
	}
}

func TestAttributionRequestClosedInputsAndCeilings(t *testing.T) {
	q := validRequest()
	q.Subjects[0].Inputs[0] = AttributionInput{}
	if err := q.Validate(); err == nil {
		t.Fatal("empty attribution input accepted")
	}
	q = validRequest()
	q.Authority = 1
	if err := q.Validate(); err == nil {
		t.Fatal("authority ceiling accepted")
	}
}

func TestStrictDecodeRejectsUnknownDuplicateAndTrailing(t *testing.T) {
	cases := []string{
		`{"kind":"GRAPH_SUBJECT","graph_subject":{},"unknown":1}`,
		`{"kind":"GRAPH_SUBJECT","kind":"SOURCE_RANGE","graph_subject":{}}`,
		`{"kind":"GRAPH_SUBJECT","graph_subject":{}} {}`,
	}
	for _, input := range cases {
		var a Anchor
		if err := DecodeStrict([]byte(input), &a); err == nil {
			t.Fatalf("strict decoder accepted %s", input)
		}
	}
}

func TestIdentityBindsContentAndRejectsSubstitution(t *testing.T) {
	q := validRequest()
	id, err := Identity(q)
	if err != nil {
		t.Fatal(err)
	}
	q.ID = id
	b, err := CanonicalBytes(q)
	if err != nil {
		t.Fatal(err)
	}
	var decoded AttributionRequest
	if err := DecodeStrict(b, &decoded); err != nil {
		t.Fatalf("self identity rejected: %v", err)
	}
	b = bytes.Replace(b, []byte(`"subject_id":"s"`), []byte(`"subject_id":"tampered"`), 1)
	if err := DecodeStrict(b, &decoded); err == nil {
		t.Fatal("tampered identity accepted")
	}
}

func TestCanonicalOrderingIsPermutationInvariant(t *testing.T) {
	a := validRequest()
	a.Subjects = append(a.Subjects, Subject{SubjectID: "a", Inputs: []AttributionInput{{Anchor: ptrAnchor(graphAnchor("a"))}}})
	b := a
	b.Subjects = []Subject{a.Subjects[1], a.Subjects[0]}
	ab, _ := CanonicalBytes(a)
	bb, _ := CanonicalBytes(b)
	if !bytes.Equal(ab, bb) {
		t.Fatalf("canonical bytes differ by input order\n%s\n%s", ab, bb)
	}
}

func TestAttributeIsExplicitlyNotReady(t *testing.T) {
	got := Attribute(validRequest())
	if got.Outcome != OutcomeNotReady || got.Authority != 0 || got.Accepted || got.Completeness != CompletenessUnknown {
		t.Fatalf("unsafe blocked result: %#v", got)
	}
}

func validRequest() AttributionRequest {
	return AttributionRequest{Schema: "attribution-request/v1", Inventory: ArtifactSelector{ID: "sha256:" + strings.Repeat("c", 64), Schema: "inventory/v1", ByteLength: 1}, Membership: ArtifactSelector{ID: "sha256:" + strings.Repeat("d", 64), Schema: "membership/v1", ByteLength: 1}, Subjects: []Subject{{SubjectID: "s", Inputs: []AttributionInput{{Anchor: ptrAnchor(graphAnchor("g"))}}}}, Completeness: CompletenessUnknown}
}
func ptrAnchor(a Anchor) *Anchor { return &a }
