package adr0011lifecycle

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"lsp-trace/internal/publication"
)

const testInstallationID = "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

func testV2Binding(t *testing.T) (*v2Binding, []byte, []byte) {
	t.Helper()
	l, schema, policy := partialFixture(t)
	c, err := newV2Codec(schema, policy)
	if err != nil {
		t.Fatal(err)
	}
	b, err := bindV2Synthetic(l, c, testInstallationID)
	if err != nil {
		t.Fatal(err)
	}
	return b, schema, policy
}
func testV2Verification() map[string]any {
	return map[string]any{"no_replace": "VERIFIED", "readback": "EXACT", "directory_sync": "SUCCEEDED"}
}
func testV2Record(t *testing.T, b *v2Binding, role string) map[string]any {
	t.Helper()
	digest := "sha256:" + strings.Repeat("a", 64)
	rootRef, err := b.ref("fenceIntent", "adr0011-fence-intent-v2-"+strings.TrimPrefix(v2Digest([]byte("sample")), "sha256:")+".json", []byte("sample"))
	if err != nil {
		t.Fatal(err)
	}
	hostRef, err := b.ref("hostHeadPrepare", "adr0011-host-head-prepare-v2-"+strings.TrimPrefix(v2Digest([]byte("sample")), "sha256:")+".json", []byte("sample"))
	if err != nil {
		t.Fatal(err)
	}
	r := map[string]any{"role": v2Roles[role].domain, "version": int64(2), "root_identity": b.rootIdentity, "epoch": int64(1)}
	switch role {
	case "trustedHostRoot":
		r["genesis_head"] = digest
		r["anchor_uri"] = v2FileURI(b.l.anchor.Path())
		r["anchor_identity"] = b.anchorIdentity
		r["lock_selector"] = "adr0011-root.lock"
		r["provisioning_verification"] = testV2Verification()
	case "hostHeadPrepare":
		r["epoch"] = int64(2)
		r["anchor_identity"] = b.anchorIdentity
		r["previous_head"] = digest
		r["next_head"] = rootRef
		r["verification"] = testV2Verification()
	case "hostHeadCommit":
		r["epoch"] = int64(2)
		r["anchor_identity"] = b.anchorIdentity
		r["previous_head"] = digest
		r["next_head"] = rootRef
		r["prepare_ref"] = hostRef
		r["transition_ref"] = rootRef
		r["verification"] = testV2Verification()
	case "headTransition":
		r["epoch"] = int64(2)
		r["previous_head"] = digest
		r["next_head"] = rootRef
		r["successors"] = []any{}
		r["enumeration"] = map[string]any{"objects": int64(0), "bytes": int64(0), "depth": int64(0), "work": int64(0), "complete": true}
		r["prepare_ref"] = hostRef
		r["publication_verification"] = testV2Verification()
	case "fenceIntent":
		r["previous_head"] = digest
		r["decision_id"] = digest
		r["action"] = "PRIVACY_REVOKE"
		r["closure_ref"] = rootRef
		r["policy_digest"] = "sha256:" + partialPolicyHash
		r["verification"] = testV2Verification()
	case "closureSnapshot":
		r["anchored_head"] = digest
		r["decision_id"] = digest
		r["selected_dependencies"] = []any{rootRef}
		r["dependents"] = []any{}
		r["pins"] = []any{}
		r["enumeration"] = map[string]any{"objects": int64(1), "bytes": int64(1), "depth": int64(1), "work": int64(1), "complete": true}
		r["completeness_witness"] = rootRef
		r["verification"] = testV2Verification()
	}
	return r
}
func TestV2SixSelectedRoleCodec(t *testing.T) {
	b, _, _ := testV2Binding(t)
	for role := range v2Roles {
		t.Run(role, func(t *testing.T) {
			fields := testV2Record(t, b, role)
			selector, raw, err := b.codec.encode(role, fields)
			if err != nil {
				t.Fatal(err)
			}
			decoded, err := b.codec.decode(role, selector, raw)
			if err != nil {
				t.Fatal(err)
			}
			if decoded["record_id"] == nil || !bytes.HasPrefix(raw, []byte("{")) {
				t.Fatal("missing canonical record ID")
			}
			if role != "fenceIntent" {
				if _, err := b.codec.decode("fenceIntent", selector, raw); err == nil {
					t.Fatal("cross-role substitution admitted")
				}
			}
		})
	}
}
func TestV2ReadHeldRejectsRolePathAndOriginalBytes(t *testing.T) {
	b, _, _ := testV2Binding(t)
	ref, err := b.publishRecord("trustedHostRoot", testV2Record(t, b, "trustedHostRoot"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := b.readHeld("trustedHostRoot", ref); err != nil {
		t.Fatal(err)
	}
	if _, err := b.publishRecord("trustedHostRoot", testV2Record(t, b, "trustedHostRoot")); err == nil {
		t.Fatal("no-replace publication admitted duplicate")
	}
	changed := make(map[string]any, len(ref))
	for k, v := range ref {
		changed[k] = v
	}
	changed["path"] = v2FileURI(filepath.Join(b.l.root.Path(), ref["selector"].(string)))
	if _, err := b.readHeld("trustedHostRoot", changed); err == nil {
		t.Fatal("schema-valid wrong host path admitted")
	}
	changed["path"] = ref["path"]
	changed["role_uri"] = v2RoleURI("hostHeadPrepare")
	if _, err := b.readHeld("trustedHostRoot", changed); err == nil {
		t.Fatal("schema-valid wrong role URI admitted")
	}
	if _, err := b.readHeld("hostHeadPrepare", ref); err == nil {
		t.Fatal("host role read private/other-role slot")
	}
	path := filepath.Join(b.l.anchor.Path(), ref["selector"].(string))
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	raw[0] = 'x'
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := b.readHeld("trustedHostRoot", ref); err == nil {
		t.Fatal("stale original byte pin admitted")
	}
}
func TestV2CoherentRehashStillDeniedByHeldRef(t *testing.T) {
	b, _, _ := testV2Binding(t)
	original := testV2Record(t, b, "trustedHostRoot")
	held, err := b.publishRecord("trustedHostRoot", original)
	if err != nil {
		t.Fatal(err)
	}
	replacement := testV2Record(t, b, "trustedHostRoot")
	replacement["genesis_head"] = "sha256:" + strings.Repeat("b", 64)
	changed, err := b.publishRecord("trustedHostRoot", replacement)
	if err != nil {
		t.Fatal(err)
	}
	if err := v2MatchHeldRef(held, changed); err == nil {
		t.Fatal("coherent self-rehash replaced held expected record")
	}
	if _, err := b.readHeld("trustedHostRoot", changed); err != nil {
		t.Fatalf("replacement itself should be shape-valid but not adopted: %v", err)
	}
}
func TestV2UnverifiedDirectorySyncReturnsNoRef(t *testing.T) {
	b, _, _ := testV2Binding(t)
	b.syncForTest = func(*publication.Root) (bool, error) { return false, nil }
	ref, err := b.publishRecord("hostHeadCommit", testV2Record(t, b, "hostHeadCommit"))
	if err == nil || ref != nil {
		t.Fatalf("committed-unverified host slot promoted: ref=%v err=%v", ref, err)
	}
	names, err := os.ReadDir(b.l.anchor.Path())
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, n := range names {
		if strings.HasPrefix(n.Name(), "adr0011-host-head-commit-v2-") {
			found = true
		}
	}
	if !found {
		t.Fatal("test did not reach uncertain post-write boundary")
	}
}
func alternateTrustedRoot(t *testing.T, b *v2Binding) map[string]any {
	t.Helper()
	alternate := filepath.Join(filepath.Dir(b.l.anchor.Path()), "alternate-anchor")
	if err := os.Mkdir(alternate, 0700); err != nil {
		t.Fatal(err)
	}
	fields := testV2Record(t, b, "trustedHostRoot")
	fields["anchor_uri"] = v2FileURI(alternate)
	selector, raw, err := b.codec.encode("trustedHostRoot", fields)
	if err != nil {
		t.Fatalf("counterexample must be V2 schema-valid: %v", err)
	}
	if _, err := b.codec.decode("trustedHostRoot", selector, raw); err != nil {
		t.Fatalf("counterexample must have coherent ID/selector: %v", err)
	}
	return fields
}
func TestV2TrustedRootAlternateAnchorURIPublicationDenied(t *testing.T) {
	b, _, _ := testV2Binding(t)
	valid := testV2Record(t, b, "trustedHostRoot")
	if valid["anchor_uri"] != v2FileURI(b.l.anchor.Path()) {
		t.Fatal("valid fixture not bound to selected anchor")
	}
	fields := alternateTrustedRoot(t, b)
	if _, err := b.publishRecord("trustedHostRoot", fields); err == nil {
		t.Fatal("ASSERT_ALTERNATE_ANCHOR_URI_PUBLICATION_DENIED: coherent alternate URI published")
	}
	held, err := b.publishRecord("trustedHostRoot", valid)
	if err != nil {
		t.Fatalf("selected anchor URI publication rejected: %v", err)
	}
	if _, err := b.readHeld("trustedHostRoot", held); err != nil {
		t.Fatalf("selected anchor URI read rejected: %v", err)
	}
}
func TestV2TrustedRootAlternateAnchorURIReadDenied(t *testing.T) {
	b, _, _ := testV2Binding(t)
	fields := alternateTrustedRoot(t, b)
	selector, raw, err := b.codec.encode("trustedHostRoot", fields)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(b.l.anchor.Path(), selector)
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.Write(raw); err != nil {
		_ = f.Close()
		t.Fatal(err)
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	synced, err := b.l.anchor.SyncDirectory()
	if err != nil || !synced {
		t.Fatalf("test fixture directory sync: %v %v", synced, err)
	}
	held, err := b.ref("trustedHostRoot", selector, raw)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := b.readHeld("trustedHostRoot", held); err == nil {
		t.Fatal("ASSERT_ALTERNATE_ANCHOR_URI_READ_DENIED: coherent alternate URI read as selected")
	}
}
func TestV2SelectedIdentityAndNestedRefDenial(t *testing.T) {
	b, _, _ := testV2Binding(t)
	fields := testV2Record(t, b, "trustedHostRoot")
	other := map[string]any{}
	for k, v := range b.rootIdentity {
		other[k] = v
	}
	other["inode"] = other["inode"].(int64) + 1
	fields["root_identity"] = other
	if _, _, err := b.codec.encode("trustedHostRoot", fields); err != nil {
		t.Fatalf("counterexample must remain schema-valid: %v", err)
	}
	if _, err := b.publishRecord("trustedHostRoot", fields); err == nil {
		t.Fatal("schema-valid substituted root identity published")
	}
	fields = testV2Record(t, b, "hostHeadPrepare")
	next := fields["next_head"].(map[string]any)
	next["path"] = v2FileURI(filepath.Join(b.l.anchor.Path(), next["selector"].(string)))
	if _, _, err := b.codec.encode("hostHeadPrepare", fields); err != nil {
		t.Fatalf("counterexample must remain schema-valid: %v", err)
	}
	if _, err := b.publishRecord("hostHeadPrepare", fields); err == nil {
		t.Fatal("schema-valid private ref into host anchor published")
	}
}
func TestV2SelectedBytesCopied(t *testing.T) {
	b, schema, policy := testV2Binding(t)
	schema[0] = 'x'
	policy[0] = 'x'
	if partialHash(b.codec.selectedSchema) != partialSchemaHash || partialHash(b.codec.selectedPolicy) != partialPolicyHash {
		t.Fatal("selected byte copy changed after caller mutation")
	}
	if _, _, err := b.codec.encode("trustedHostRoot", testV2Record(t, b, "trustedHostRoot")); err != nil {
		t.Fatal(err)
	}
}
func TestV2StrictFailures(t *testing.T) {
	b, schema, policy := testV2Binding(t)
	if _, err := newV2Codec(append(schema, ' '), policy); err == nil {
		t.Fatal("changed selected schema accepted")
	}
	fields := testV2Record(t, b, "hostHeadCommit")
	fields["verification"].(map[string]any)["directory_sync"] = "FAILED"
	if _, _, err := b.codec.encode("hostHeadCommit", fields); err == nil {
		t.Fatal("committed-unverified shape accepted")
	}
	fields = testV2Record(t, b, "trustedHostRoot")
	fields["unknown"] = true
	if _, _, err := b.codec.encode("trustedHostRoot", fields); err == nil {
		t.Fatal("unknown field accepted")
	}
	delete(fields, "unknown")
	fields["genesis_head"] = "sha256:é"
	if _, _, err := b.codec.encode("trustedHostRoot", fields); err == nil {
		t.Fatal("unsupported Unicode accepted")
	}
	fields = testV2Record(t, b, "trustedHostRoot")
	selector, raw, err := b.codec.encode("trustedHostRoot", fields)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := b.codec.decode("trustedHostRoot", selector, append(raw, ' ')); err == nil {
		t.Fatal("trailing noncanonical bytes accepted")
	}
	if _, err := b.codec.decode("trustedHostRoot", selector, []byte(`{"role":"a","role":"b"}`)); err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("duplicate decoded key accepted: %v", err)
	}
	fields["epoch"] = 1.5
	if _, _, err := b.codec.encode("trustedHostRoot", fields); err == nil {
		t.Fatal("unsupported float admitted")
	}
}
