//go:build darwin || linux

package privatehostproductv3

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

// Mirror the proposed data contract so the original decoder can execute RED
// rather than failing compilation on a not-yet-defined production type.
type freshDocumentFixtureV3 struct{ URI, SourceDigest string }
type freshIdentityFixtureV3 struct {
	Schema, SourceRoot, Kind, DeclaredRevision, GitHead, Claim, Completeness string
	Documents                                                                []freshDocumentFixtureV3
	Authority                                                                int
	Accepted                                                                 bool
}

func freshIdentityFixture(t *testing.T, kind string) (InventoryRequestV3, freshIdentityFixtureV3) {
	t.Helper()
	q, _, _, _ := inventoryProductSetupV3(t, "")
	i := freshIdentityFixtureV3{Schema: "inventory-source-identity.v1", SourceRoot: q.Configuration.SourceRoot, Kind: kind, DeclaredRevision: "current working source", Claim: "CALLER_ASSERTED_NOT_AUTHENTICATED", Completeness: "CALLER_DECLARED_COMPLETE_NOT_FILESYSTEM_VERIFIED", Documents: []freshDocumentFixtureV3{{URI: "file://" + q.Configuration.SourceRoot + "/query.go", SourceDigest: hash([]byte("approved document"))}}}
	if kind != "NON_GIT" {
		i.GitHead = strings.Repeat("a", 40)
	}
	return q, i
}
func freshIdentityRawV3(t *testing.T, q InventoryRequestV3, i freshIdentityFixtureV3) []byte {
	t.Helper()
	b, _ := json.Marshal(q.Inventory)
	var m map[string]json.RawMessage
	if json.Unmarshal(b, &m) != nil {
		t.Fatal("SETUP_MAP")
	}
	encoded, _ := json.Marshal(i)
	m["SourceIdentity"] = encoded
	revision := "inventory-source.v1/" + hash(encoded)
	m["SourceRevision"], _ = json.Marshal(revision)
	options, _ := json.Marshal(m)
	// Round-trip through the product type so the eventual production encoding is
	// canonical. Before implementation it drops the unknown identity, then refuses
	// the fresh revision at the existing admission predicate.
	if json.Unmarshal(options, &q.Inventory) != nil {
		t.Fatal("SETUP_IDENTITY")
	}
	q.Configuration.RevisionAssertion = revision
	q.Configuration.UnitPolicySHA256 = hash(mustJSON(q.Inventory))
	q.Configuration.ProducerConfigSHA256 = q.Configuration.UnitPolicySHA256
	q.Configuration.ApprovalSHA256 = ApplicationConfigDigest(q.Configuration)
	q.ApprovalSHA256 = InventoryRequestDigestV3(q)
	return mustJSON(q)
}
func TestInventoryFreshIdentityAdmissionV3(t *testing.T) {
	const a = "ASSERT_FRESH_IDENTITY_ADMISSION"
	for _, kind := range []string{"CLEAN_GIT", "DIRTY_GIT", "NON_GIT"} {
		q, i := freshIdentityFixture(t, kind)
		raw := freshIdentityRawV3(t, q, i)
		admitted, err := DecodeInventoryRequestV3(raw)
		if err != nil {
			t.Fatal(a, "FAIL valid declared identity", kind, err)
		}
		if admitted.Configuration.RevisionAssertion == InventorySourceRevisionV3 {
			t.Fatal(a, "FAIL historical substitution")
		}
		for _, bad := range []string{"claim", "complete", "kind", "head", "digest", "root", "authority", "accepted", "duplicate", "empty", "escape", "unordered", "too-many", "revision", "long-revision", "bad-head", "query", "host", "escape-path"} {
			v := i
			v.Documents = append([]freshDocumentFixtureV3(nil), i.Documents...)
			switch bad {
			case "claim":
				v.Claim = "AUTHENTICATED"
			case "complete":
				v.Completeness = "FILESYSTEM_VERIFIED"
			case "kind":
				v.Kind = "UNKNOWN"
			case "head":
				if kind == "NON_GIT" {
					v.GitHead = strings.Repeat("a", 40)
				} else {
					v.GitHead = ""
				}
			case "digest":
				v.Documents[0].SourceDigest = "sha256:bad"
			case "root":
				v.SourceRoot = "/different/root"
			case "authority":
				v.Authority = 1
			case "accepted":
				v.Accepted = true
			case "duplicate":
				v.Documents = append(v.Documents, v.Documents[0])
			case "empty":
				v.Documents = nil
			case "escape":
				v.Documents[0].URI = "file:///outside/query.go"
			case "unordered":
				v.Documents = append(v.Documents, freshDocumentFixtureV3{URI: "file://" + i.SourceRoot + "/a.go", SourceDigest: i.Documents[0].SourceDigest})
			case "too-many":
				v.Documents = nil
				for n := 0; n < 17; n++ {
					v.Documents = append(v.Documents, freshDocumentFixtureV3{URI: fmt.Sprintf("file://%s/d%02d.go", i.SourceRoot, n), SourceDigest: i.Documents[0].SourceDigest})
				}
			case "revision":
				v.DeclaredRevision = ""
			case "long-revision":
				v.DeclaredRevision = strings.Repeat("r", 4097)
			case "bad-head":
				v.GitHead = strings.Repeat("g", 40)
			case "query":
				v.Documents[0].URI += "?variant=1"
			case "host":
				v.Documents[0].URI = "file://untrusted" + i.SourceRoot + "/query.go"
			case "escape-path":
				v.Documents[0].URI = "file://" + i.SourceRoot + "/../query.go"
			}
			if _, err := DecodeInventoryRequestV3(freshIdentityRawV3(t, q, v)); err == nil {
				t.Fatal(a, "FAIL accepted malformed identity", kind, bad)
			}
		}
	}
	q, i := freshIdentityFixture(t, "NON_GIT")
	i.Documents = nil
	for n := 0; n < 16; n++ {
		i.Documents = append(i.Documents, freshDocumentFixtureV3{URI: fmt.Sprintf("file://%s/d%02d.go", i.SourceRoot, n), SourceDigest: hash([]byte("document"))})
	}
	if _, e := DecodeInventoryRequestV3(freshIdentityRawV3(t, q, i)); e != nil {
		t.Fatal(a, "FAIL valid boundary", e)
	}
	t.Log(a + ": PASS")
}
func TestInventoryFreshIdentityApprovalV3(t *testing.T) {
	const a = "ASSERT_FRESH_IDENTITY_APPROVAL"
	q, i := freshIdentityFixture(t, "DIRTY_GIT")
	raw := freshIdentityRawV3(t, q, i)
	var base InventoryRequestV3
	if json.Unmarshal(raw, &base) != nil {
		t.Fatal("SETUP_BASE")
	}
	detached := base.Inventory.Detached()
	original := InventoryRequestDigestV3(base)
	detached.SourceIdentity.Documents[0].SourceDigest = hash([]byte("mutated detached document"))
	if InventoryRequestDigestV3(base) != original {
		t.Fatal(a, "FAIL detached identity aliases original")
	}
	// Admission has its own guard: this assertion checks the executable digest
	// contract even before fresh admission exists.
	for _, field := range []string{"DeclaredRevision", "GitHead", "Documents", "Kind"} {
		var m map[string]json.RawMessage
		if json.Unmarshal(mustJSON(base.Inventory), &m) != nil {
			t.Fatal("SETUP_APPROVAL")
		}
		var identity map[string]any
		if json.Unmarshal(m["SourceIdentity"], &identity) != nil {
			t.Fatal(a, "FAIL identity absent from sealed request")
		}
		switch field {
		case "Documents":
			identity[field] = []freshDocumentFixtureV3{{URI: i.Documents[0].URI, SourceDigest: hash([]byte("changed content"))}}
		case "Kind":
			identity[field] = "CLEAN_GIT"
		default:
			identity[field] = "changed assertion"
		}
		m["SourceIdentity"], _ = json.Marshal(identity)
		body, _ := json.Marshal(m)
		changed := base
		if json.Unmarshal(body, &changed.Inventory) != nil {
			t.Fatal("SETUP_CHANGED")
		}
		if InventoryRequestDigestV3(changed) == base.ApprovalSHA256 {
			t.Fatal(a, "FAIL changed identity kept approval", field)
		}
		if _, err := DecodeInventoryRequestV3(mustJSON(changed)); err == nil {
			t.Fatal(a, "FAIL old approval reused", field)
		}
	}
	t.Log(a + ": PASS")
}
