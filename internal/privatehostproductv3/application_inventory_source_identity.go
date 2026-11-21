//go:build darwin || linux

package privatehostproductv3

import (
	"encoding/hex"
	"net/url"
	"path/filepath"
	"strings"
	"unicode/utf8"

	tp "lsp-trace/internal/targetpacket"
)

const inventorySourceIdentityPrefixV3 = "inventory-source.v1/"

// InventorySourceIdentityV3 is sealed caller metadata, not a source snapshot,
// Git inspection, permission or opaque native capability. Documents are expected
// digests corroborated only against selected documents from paid native capture.
// Unlisted/unselected source and Git cleanliness remain unverified.
type InventorySourceIdentityV3 struct {
	Schema, SourceRoot, Kind, DeclaredRevision, GitHead, Claim, Completeness string
	Documents                                                                []InventorySourceDocumentV3
	Authority                                                                int
	Accepted                                                                 bool
}
type InventorySourceDocumentV3 struct{ URI, SourceDigest string }

// InventorySourceRevisionValidV3 checks syntax only, never admission or freshness.
func InventorySourceRevisionValidV3(revision string) bool { return inventoryRevisionValidV3(revision) }

func inventoryRevisionValidV3(revision string) bool {
	if revision == "" || len(revision) > 4096 || !utf8.ValidString(revision) || strings.ContainsAny(revision, "\x00\r\n") {
		return false
	}
	return !strings.HasPrefix(revision, inventorySourceIdentityPrefixV3) || digestValue(strings.TrimPrefix(revision, inventorySourceIdentityPrefixV3))
}
func inventorySourceURIWithinV3(raw, root string) bool {
	u, e := url.Parse(raw)
	return e == nil && len(raw) <= 4096 && u.Scheme == "file" && u.Host == "" && u.User == nil && u.RawQuery == "" && !u.ForceQuery && u.Fragment == "" && u.RawPath == "" && u.String() == raw && filepath.IsAbs(u.Path) && filepath.Clean(u.Path) == u.Path && !strings.ContainsRune(u.Path, 0) && strings.HasPrefix(u.Path, root+string(filepath.Separator))
}
func inventorySourceIdentityValidV3(i InventorySourceIdentityV3) bool {
	if i.Schema != "inventory-source-identity.v1" || !applicationCanonicalPath(i.SourceRoot) || !inventoryRevisionValidV3(i.DeclaredRevision) || i.Claim != "CALLER_ASSERTED_NOT_AUTHENTICATED" || i.Completeness != "CALLER_DECLARED_COMPLETE_NOT_FILESYSTEM_VERIFIED" || i.Authority != 0 || i.Accepted || len(i.Documents) == 0 || len(i.Documents) > 16 {
		return false
	}
	switch i.Kind {
	case "CLEAN_GIT", "DIRTY_GIT":
		if len(i.GitHead) != 40 && len(i.GitHead) != 64 {
			return false
		}
		if _, e := hex.DecodeString(i.GitHead); e != nil || strings.ToLower(i.GitHead) != i.GitHead {
			return false
		}
	case "NON_GIT":
		if i.GitHead != "" {
			return false
		}
	default:
		return false
	}
	previous := ""
	for _, d := range i.Documents {
		if !inventorySourceURIWithinV3(d.URI, i.SourceRoot) || d.URI <= previous || !digestValue(d.SourceDigest) {
			return false
		}
		previous = d.URI
	}
	return len(mustJSON(i)) <= 64<<10
}

// InventorySourceIdentityRevisionV3 returns a content-addressed declaration ID,
// never a Git revision or proof that the declared files form a complete tree.
func InventorySourceIdentityRevisionV3(i InventorySourceIdentityV3) string {
	if !inventorySourceIdentityValidV3(i) {
		return ""
	}
	return inventorySourceIdentityPrefixV3 + hash(mustJSON(i))
}
func inventoryRequestSourceIdentityV3(q InventoryRequestV3) bool {
	o := q.Inventory
	if !inventoryRevisionValidV3(o.SourceRevision) || q.Configuration.RevisionAssertion != o.SourceRevision {
		return false
	}
	if o.SourceIdentity == nil {
		return !strings.HasPrefix(o.SourceRevision, inventorySourceIdentityPrefixV3)
	}
	return o.SourceIdentity.SourceRoot == q.Configuration.SourceRoot && InventorySourceIdentityRevisionV3(*o.SourceIdentity) == o.SourceRevision
}
func inventorySourceIdentityCopyV3(i *InventorySourceIdentityV3) *InventorySourceIdentityV3 {
	if i == nil {
		return nil
	}
	v := *i
	v.Documents = append([]InventorySourceDocumentV3(nil), i.Documents...)
	return &v
}
func inventorySourceUnitMatchesV3(i *InventorySourceIdentityV3, revision string, u tp.PrivateSourceUnit) bool {
	if i == nil {
		return !strings.HasPrefix(revision, inventorySourceIdentityPrefixV3)
	}
	if InventorySourceIdentityRevisionV3(*i) != revision || u.Revision != revision || u.Workspace != i.SourceRoot {
		return false
	}
	for _, d := range i.Documents {
		if d.URI == u.URI {
			return d.SourceDigest == u.SourceDigest
		}
	}
	return false
}
