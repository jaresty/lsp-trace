//go:build darwin || linux

package privatehostproductv3

import (
	"encoding/json"
	dr "lsp-trace/internal/describerequest"
	cat "lsp-trace/internal/provisionalfeaturecatalog"
	tp "lsp-trace/internal/targetpacket"
	"testing"
)

// Pure exported-wire fixture only: formatting/validation cannot mint source
// admission or opaque native custody. The tagged CLI test supplies that path.
func inventoryRevisionEvidenceV3(t *testing.T) cat.SelectedEvidenceV3 {
	t.Helper()
	u := tp.PrivateSourceUnit{Version: tp.PrivateSourceVersion, ID: hash([]byte("unit")), SourceDigest: hash([]byte("controlled document")), Revision: InventorySourceRevisionV3, URI: "file:///fixture/query.go", Workspace: "/fixture", Body: "func Query() {}", ClaimCeiling: tp.PrivateSourceCeiling}
	r, err := dr.FormatPrivateSourceRequestCandidate(u, 1000, 1<<20)
	if err != nil {
		t.Fatal("SETUP_NATIVE_RECORD", err)
	}
	return cat.SelectedEvidenceV3{Unit: u, Request: r}
}

func TestInventoryNativeDescribeRevisionV3(t *testing.T) {
	const a = "ASSERT_NATIVE_DESCRIBE_REVISION"
	e := inventoryRevisionEvidenceV3(t)
	raw, _ := json.Marshal(e.Request)
	prompt, err := inventoryPayloadV3("Describe", raw)
	if err != nil || prompt != "sha256:"+e.Request.Envelope.InputSHA256 {
		t.Fatal(a, "FAIL valid content identity", err)
	}
	for _, field := range []string{"revision", "prompt", "record", "lineage", "authority"} {
		r := e.Request
		switch field {
		case "revision":
			r.Envelope.SourceRevision = InventorySourceRevisionV3
		case "prompt":
			r.Envelope.Prompt += "tampered"
		case "record":
			r.RecordID = hash([]byte("other record"))
		case "lineage":
			r.Lineage.LineageIdentity = hash([]byte("other lineage"))
		case "authority":
			r.Lineage.Authority = 1
		}
		raw, _ := json.Marshal(r)
		if _, err := inventoryPayloadV3("Describe", raw); err == nil {
			t.Fatal(a, "FAIL accepted tamper", field)
		}
	}
	// This record is internally sealed, but its claimed content digest is not SHA256.
	u := e.Unit
	u.SourceDigest = "sha256:invalid"
	r, err := dr.FormatPrivateSourceRequestCandidate(u, 1000, 1<<20)
	if err != nil {
		t.Fatal("SETUP_INVALID_DIGEST", err)
	}
	raw, _ = json.Marshal(r)
	if _, err := inventoryPayloadV3("Describe", raw); err == nil {
		t.Fatal(a, "FAIL malformed digest")
	}
	t.Log(a + ": PASS")
}

func TestInventoryNativeEvidenceRevisionV3(t *testing.T) {
	const a = "ASSERT_NATIVE_EVIDENCE_REVISION"
	e := inventoryRevisionEvidenceV3(t)
	if !inventoryEvidenceRevisionV3(e, InventorySourceRevisionV3) {
		t.Fatal(a, "FAIL valid distinct identity domains")
	}
	for _, field := range []string{"unit-revision", "source-digest", "body", "uri", "request-revision", "request-id", "downgrade"} {
		v := e
		switch field {
		case "unit-revision":
			v.Unit.Revision = "another caller revision"
		case "source-digest":
			v.Unit.SourceDigest = hash([]byte("another document"))
		case "body":
			v.Unit.Body += "changed"
		case "uri":
			v.Unit.URI = "file:///elsewhere/query.go"
		case "request-revision":
			v.Request.Envelope.SourceRevision = InventorySourceRevisionV3
		case "request-id":
			v.Request.RecordID = hash([]byte("another record"))
		case "downgrade":
			v.Request.Lineage.ConsumerResolution = "legacy"
			v.Request.Envelope.SourceRevision = InventorySourceRevisionV3
		}
		if inventoryEvidenceRevisionV3(v, InventorySourceRevisionV3) {
			t.Fatal(a, "FAIL accepted mismatch", field)
		}
	}
	t.Log(a + ": PASS")
}

func TestInventoryLegacyDescribeRevisionV3(t *testing.T) {
	const a = "ASSERT_LEGACY_DESCRIBE_REVISION"
	raw := inventoryProductDescribeV3()
	if _, err := inventoryPayloadV3("Describe", raw); err != nil {
		t.Fatal(a, "FAIL legacy admission", err)
	}
	var r dr.Record
	if json.Unmarshal(raw, &r) != nil {
		t.Fatal("SETUP_LEGACY")
	}
	e := cat.SelectedEvidenceV3{Request: r}
	if !inventoryEvidenceRevisionV3(e, InventorySourceRevisionV3) {
		t.Fatal(a, "FAIL legacy readback")
	}
	r.Envelope.SourceRevision = "other"
	raw, _ = json.Marshal(r)
	if _, err := inventoryPayloadV3("Describe", raw); err == nil {
		t.Fatal(a, "FAIL legacy foreign revision")
	}
	e.Request = r
	if inventoryEvidenceRevisionV3(e, InventorySourceRevisionV3) {
		t.Fatal(a, "FAIL legacy foreign readback")
	}
	t.Log(a + ": PASS")
}
