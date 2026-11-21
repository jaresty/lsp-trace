//go:build darwin || linux

package privatehostproductv3

import (
	dr "lsp-trace/internal/describerequest"
	cat "lsp-trace/internal/provisionalfeaturecatalog"
	"testing"
)

func TestInventoryFreshIdentityReadbackV3(t *testing.T) {
	const a = "ASSERT_FRESH_IDENTITY_READBACK"
	q, i := freshIdentityFixture(t, "NON_GIT")
	e := inventoryRevisionEvidenceV3(t)
	identity := InventorySourceIdentityV3{Schema: i.Schema, SourceRoot: i.SourceRoot, Kind: i.Kind, DeclaredRevision: i.DeclaredRevision, GitHead: i.GitHead, Claim: i.Claim, Completeness: i.Completeness, Documents: []InventorySourceDocumentV3{{URI: i.Documents[0].URI, SourceDigest: e.Unit.SourceDigest}}}
	rev := InventorySourceIdentityRevisionV3(identity)
	e.Unit.Revision = rev
	e.Unit.Workspace = q.Configuration.SourceRoot
	e.Unit.URI = identity.Documents[0].URI
	format := func(v cat.SelectedEvidenceV3) cat.SelectedEvidenceV3 {
		r, err := dr.FormatPrivateSourceRequestCandidate(v.Unit, v.Request.Envelope.DeadlineMS, 1<<20)
		if err != nil {
			t.Fatal("SETUP_READBACK", err)
		}
		v.Request = r
		return v
	}
	e = format(e)
	if !inventoryEvidenceRevisionV3(e, rev, &identity) {
		t.Fatal(a, "FAIL valid fresh readback")
	}
	// Reproduce internally consistent requests from wrong roots, unlisted documents
	// and changed digests. Internal request integrity alone must not accept them.
	for _, kind := range []string{"workspace", "unlisted", "digest"} {
		v := e
		switch kind {
		case "workspace":
			v.Unit.Workspace = "/another/root"
		case "unlisted":
			v.Unit.URI = "file://" + q.Configuration.SourceRoot + "/not-approved.go"
		case "digest":
			v.Unit.SourceDigest = hash([]byte("changed captured document"))
		}
		v = format(v)
		if inventoryEvidenceRevisionV3(v, rev, &identity) {
			t.Fatal(a, "FAIL accepted out-of-identity evidence", kind)
		}
	}
	t.Log(a + ": PASS")
}
