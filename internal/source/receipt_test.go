package source

import (
	"bytes"
	"strings"
	"testing"
)

func readableFixture(content []byte) (DiscoveredItem, Acquisition, []byte) {
	return DiscoveredItem{ID: "source-1", Locator: "file:///workspace/main.go"}, Acquisition{
		Status:     Readable,
		Provenance: Provenance{Mechanism: "filesystem", Locator: "file:///workspace/main.go", Revision: "abc123"},
	}, content
}

func TestCanonicalizeReceiptReadableContentIdentity(t *testing.T) {
	item, acquisition, content := readableFixture([]byte("package main\n"))
	r, canonicalContent, canonicalReceipt, err := CanonicalizeReceipt(item, acquisition, content)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(canonicalContent, content) || r.Status != Readable || r.ContentIdentity == nil || r.ContentIdentity.Algorithm != "sha256" || r.ContentIdentity.Scope != "ACQUIRED_BYTES" || r.ContentIdentity.Digest != "sha256:df1d036cbbf3df46e2045071e082245ece204c7f53ecf0a4e022bff9bb228f47" {
		t.Fatalf("ASSERT_READABLE_CONTENT_ADDRESSED: receipt=%#v content=%q", r, canonicalContent)
	}
	if len(canonicalReceipt) == 0 {
		t.Fatal("ASSERT_READABLE_CANONICAL_RECEIPT_NONEMPTY")
	}
}

func TestCanonicalizeReceiptUnreadableEvidence(t *testing.T) {
	item := DiscoveredItem{ID: "source-2", Locator: "https://example.invalid/source"}
	acquisition := Acquisition{Status: Unreadable, Failure: &AcquisitionFailure{Reason: "permission denied"}, Provenance: Provenance{Mechanism: "http", Locator: item.Locator}}
	r, content, _, err := CanonicalizeReceipt(item, acquisition, nil)
	if err != nil {
		t.Fatal(err)
	}
	if r.Status != Unreadable || r.Failure == nil || r.Failure.Reason != "permission denied" || r.ContentIdentity != nil || content != nil {
		t.Fatalf("ASSERT_UNREADABLE_TYPED_NO_CONTENT: receipt=%#v content=%v", r, content)
	}
	acquisition.Failure = nil
	if _, _, _, err := CanonicalizeReceipt(item, acquisition, nil); err == nil || !strings.Contains(err.Error(), "failure reason") {
		t.Fatalf("ASSERT_UNREADABLE_REASON_REQUIRED: %v", err)
	}
	acquisition.Failure = &AcquisitionFailure{Reason: "permission denied"}
	if _, _, _, err := CanonicalizeReceipt(item, acquisition, []byte("invented")); err == nil || !strings.Contains(err.Error(), "must not include content") {
		t.Fatalf("ASSERT_UNREADABLE_CONTENT_REJECTED: %v", err)
	}
}

func TestCanonicalizeReceiptCanonicalAndDeterministic(t *testing.T) {
	item, acquisition, content := readableFixture([]byte("x\r\ny\n"))
	_, canonicalContent1, receipt1, err := CanonicalizeReceipt(item, acquisition, content)
	if err != nil {
		t.Fatal(err)
	}
	_, canonicalContent2, receipt2, err := CanonicalizeReceipt(item, acquisition, append([]byte(nil), content...))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(canonicalContent1, content) || !bytes.Equal(canonicalContent2, content) || !bytes.Equal(receipt1, receipt2) || receipt1[len(receipt1)-1] != '\n' {
		t.Fatalf("ASSERT_CANONICAL_DETERMINISTIC_BYTES: first=%q second=%q content=%q", receipt1, receipt2, canonicalContent1)
	}
}

func TestCanonicalizeReceiptDomainNeutralProvenance(t *testing.T) {
	item, acquisition, content := readableFixture([]byte("x"))
	r, _, encoded, err := CanonicalizeReceipt(item, acquisition, content)
	if err != nil {
		t.Fatal(err)
	}
	if r.Provenance.Mechanism != "filesystem" || r.Provenance.Locator != item.Locator || r.Provenance.Revision != "abc123" {
		t.Fatalf("ASSERT_TYPED_PROVENANCE: %#v", r.Provenance)
	}
	for _, forbidden := range [][]byte{[]byte("manifest"), []byte("trust"), []byte("authority")} {
		if bytes.Contains(bytes.ToLower(encoded), forbidden) {
			t.Fatalf("ASSERT_DOMAIN_NEUTRAL_PROVENANCE: %s", encoded)
		}
	}
	acquisition.Provenance.Mechanism = ""
	if _, _, _, err := CanonicalizeReceipt(item, acquisition, content); err == nil || !strings.Contains(err.Error(), "provenance mechanism") {
		t.Fatalf("ASSERT_PROVENANCE_MECHANISM_REQUIRED: %v", err)
	}
}

func TestCanonicalizeReceiptContentAndMetadataMutations(t *testing.T) {
	item, acquisition, content := readableFixture([]byte("before"))
	first, _, firstBytes, err := CanonicalizeReceipt(item, acquisition, content)
	if err != nil {
		t.Fatal(err)
	}
	changedContent, _, changedContentBytes, err := CanonicalizeReceipt(item, acquisition, []byte("after"))
	if err != nil {
		t.Fatal(err)
	}
	if first.ContentIdentity.Digest == changedContent.ContentIdentity.Digest || bytes.Equal(firstBytes, changedContentBytes) {
		t.Fatalf("ASSERT_CONTENT_MUTATION_CHANGES_IDENTITY: before=%s after=%s", firstBytes, changedContentBytes)
	}
	acquisition.Provenance.Revision = "def456"
	changedMetadata, _, changedMetadataBytes, err := CanonicalizeReceipt(item, acquisition, content)
	if err != nil {
		t.Fatal(err)
	}
	if first.ContentIdentity.Digest != changedMetadata.ContentIdentity.Digest || bytes.Equal(firstBytes, changedMetadataBytes) {
		t.Fatalf("ASSERT_METADATA_MUTATION_PRESERVES_CONTENT_IDENTITY: before=%s after=%s", firstBytes, changedMetadataBytes)
	}
}
