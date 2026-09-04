package graph

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func validCustodyRequest() DocumentCustodyRequest {
	return DocumentCustodyRequest{
		WorkspaceRevision:     RevisionIdentity{Kind: "git", Value: "commit-a", Custody: CustodyCallerAsserted},
		FailOnUnknownRevision: true,
		Documents: []SourceDocumentRecord{{
			DocumentID: "doc-a", OriginalURI: "file:///workspace/a.go", ContentSHA256: strings.Repeat("a", 64),
			Revision: RevisionIdentity{Kind: "git", Value: "commit-a", Blob: strings.Repeat("b", 40), Custody: CustodyProviderProved},
		}},
	}
}

func TestDocumentCustodySeparatesRequestAndDocumentRevision(t *testing.T) {
	req := validCustodyRequest()
	req.Documents[0].Revision.Value = "commit-b"
	_, err := ValidateDocumentCustody(req)
	if err == nil || !strings.Contains(err.Error(), "ASSERT_CUSTODY_REQUEST_DOCUMENT_REVISION_CONFLICT") {
		t.Fatalf("ASSERT_CUSTODY_REQUEST_DOCUMENT_REVISION_CONFLICT: got %v", err)
	}
}

func TestDocumentCustodyRejectsAuthorityOutsideClosedVocabulary(t *testing.T) {
	req := validCustodyRequest()
	req.Documents[0].Revision.Custody = RevisionCustody("INFERRED")
	_, err := ValidateDocumentCustody(req)
	if err == nil || !strings.Contains(err.Error(), "ASSERT_CUSTODY_CLOSED_AUTHORITY_VOCABULARY") {
		t.Fatalf("ASSERT_CUSTODY_CLOSED_AUTHORITY_VOCABULARY: got %v", err)
	}
}

func TestDocumentCustodyRequiresImmutableSourceIdentity(t *testing.T) {
	for _, mutate := range []func(*SourceDocumentRecord){
		func(d *SourceDocumentRecord) { d.OriginalURI = "relative/a.go" },
		func(d *SourceDocumentRecord) { d.ContentSHA256 = "mutable-label" },
	} {
		req := validCustodyRequest()
		mutate(&req.Documents[0])
		_, err := ValidateDocumentCustody(req)
		if err == nil || !strings.Contains(err.Error(), "ASSERT_CUSTODY_IMMUTABLE_SOURCE_IDENTITY") {
			t.Fatalf("ASSERT_CUSTODY_IMMUTABLE_SOURCE_IDENTITY: got %v", err)
		}
	}
}

func TestDocumentCustodyRequiresGitCommitAndBlob(t *testing.T) {
	req := validCustodyRequest()
	req.Documents[0].Revision.Blob = ""
	_, err := ValidateDocumentCustody(req)
	if err == nil || !strings.Contains(err.Error(), "ASSERT_CUSTODY_GIT_COMMIT_BLOB") {
		t.Fatalf("ASSERT_CUSTODY_GIT_COMMIT_BLOB: got %v", err)
	}
}

func TestDocumentCustodyRequiresOriginalVirtualMapping(t *testing.T) {
	req := validCustodyRequest()
	req.Documents = append(req.Documents, SourceDocumentRecord{
		DocumentID: "virtual-a", OriginalURI: "file:///workspace/a.go", VirtualURI: "virtual:///a.gjs.ts",
		ContentSHA256: strings.Repeat("c", 64), Revision: req.Documents[0].Revision,
	})
	_, err := ValidateDocumentCustody(req)
	if err == nil || !strings.Contains(err.Error(), "ASSERT_CUSTODY_VIRTUAL_MAPPING_REQUIRED") {
		t.Fatalf("ASSERT_CUSTODY_VIRTUAL_MAPPING_REQUIRED: got %v", err)
	}
}

func TestDocumentCustodyRejectsCollapsedVirtualIdentity(t *testing.T) {
	req := validCustodyRequest()
	req.Documents[0].VirtualURI = req.Documents[0].OriginalURI
	req.Documents[0].Mapping = &VirtualDocumentMapping{MappingID: "map-a", OriginalDocumentID: "doc-a"}
	_, err := ValidateDocumentCustody(req)
	if err == nil || !strings.Contains(err.Error(), "ASSERT_CUSTODY_DISTINCT_VIRTUAL_IDENTITY") {
		t.Fatalf("ASSERT_CUSTODY_DISTINCT_VIRTUAL_IDENTITY: got %v", err)
	}
}

func TestDocumentCustodyRestrictsGeneratedCoordinates(t *testing.T) {
	req := validCustodyRequest()
	req.Documents[0].Coordinates = CoordinatesGenerated
	_, err := ValidateDocumentCustody(req)
	if err == nil || !strings.Contains(err.Error(), "ASSERT_CUSTODY_GENERATED_COORDINATES_REQUIRE_MAPPING") {
		t.Fatalf("ASSERT_CUSTODY_GENERATED_COORDINATES_REQUIRE_MAPPING: got %v", err)
	}
}

func TestDocumentCustodyRejectsGeneratedOriginalCoordinates(t *testing.T) {
	req := validCustodyRequest()
	req.Documents[0].Coordinates = CoordinatesGenerated
	req.Documents[0].VirtualURI = "virtual:///a.go"
	req.Documents[0].Mapping = &VirtualDocumentMapping{MappingID: "map-a", OriginalDocumentID: "doc-a"}
	_, err := ValidateDocumentCustody(req)
	if err == nil || !strings.Contains(err.Error(), "ASSERT_CUSTODY_ORIGINAL_COORDINATES_NOT_GENERATED") {
		t.Fatalf("ASSERT_CUSTODY_ORIGINAL_COORDINATES_NOT_GENERATED: got %v", err)
	}
}

func TestDocumentCustodyRejectsMixedRevisions(t *testing.T) {
	req := validCustodyRequest()
	req.WorkspaceRevision = RevisionIdentity{}
	second := req.Documents[0]
	second.DocumentID = "doc-b"
	second.OriginalURI = "file:///workspace/b.go"
	second.Revision.Value = "commit-b"
	req.Documents = append(req.Documents, second)
	_, err := ValidateDocumentCustody(req)
	if err == nil || !strings.Contains(err.Error(), "ASSERT_CUSTODY_MIXED_REVISIONS") {
		t.Fatalf("ASSERT_CUSTODY_MIXED_REVISIONS: got %v", err)
	}
}

func TestDocumentCustodyFailOnUnknownRevision(t *testing.T) {
	req := validCustodyRequest()
	req.WorkspaceRevision = RevisionIdentity{}
	req.Documents[0].Revision = RevisionIdentity{Custody: CustodyUnknown}
	_, err := ValidateDocumentCustody(req)
	if err == nil || !strings.Contains(err.Error(), "ASSERT_CUSTODY_UNKNOWN_REVISION") {
		t.Fatalf("ASSERT_CUSTODY_UNKNOWN_REVISION: got %v", err)
	}
}

func TestDocumentCustodyReceiptDeterministicAcrossOrder(t *testing.T) {
	req := validCustodyRequest()
	second := req.Documents[0]
	second.DocumentID = "doc-b"
	second.OriginalURI = "file:///workspace/b.go"
	req.Documents = append(req.Documents, second)
	first, err := ValidateDocumentCustody(req)
	if err != nil {
		t.Fatal(err)
	}
	req.Documents[0], req.Documents[1] = req.Documents[1], req.Documents[0]
	secondReceipt, err := ValidateDocumentCustody(req)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, secondReceipt) || first.LogicalDigest == "" || first.Documents[0].DocumentID != "doc-a" {
		t.Fatalf("ASSERT_CUSTODY_RECEIPT_ORDER_INDEPENDENT: %#v != %#v", first, secondReceipt)
	}
}

func TestDocumentCustodyReceiptContainsCustodyAndDigest(t *testing.T) {
	receipt, err := ValidateDocumentCustody(validCustodyRequest())
	if err != nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal(receipt)
	for _, field := range []string{"workspace_revision", "documents", "conflicts", "logical_digest"} {
		if !strings.Contains(string(data), `"`+field+`"`) {
			t.Fatalf("ASSERT_CUSTODY_RECEIPT_REQUIRED_FIELDS: missing %s in %s", field, data)
		}
	}
	if receipt.LogicalDigest == "" || receipt.Conflicts == nil {
		t.Fatalf("ASSERT_CUSTODY_RECEIPT_REQUIRED_FIELDS: empty custody values in %s", data)
	}
}
