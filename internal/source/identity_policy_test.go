package source

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"reflect"
	"strings"
	"testing"

	"lsp-trace/internal/graph"
)

func identityFixture() IdentityRequest {
	return IdentityRequest{
		Receipts:    []ManifestReceipt{{ID: "a", Path: "a.go", Digest: "sha256:" + strings.Repeat("a", 64)}, {ID: "b", Path: "b.go", Digest: "sha256:" + strings.Repeat("b", 64)}},
		Decisions:   []ManifestDecision{{ReceiptID: "a", State: ManifestInclude}, {ReceiptID: "b", State: ManifestInclude}},
		Acquisition: AcquisitionContext{Adapter: "lsp", WorkspaceURI: "file:///workspace", InvocationID: "run-1"},
	}
}

func mustIdentity(t *testing.T, r IdentityRequest) IdentityResult {
	t.Helper()
	got, err := BuildIdentity(r)
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func TestIdentityPolicyContract(t *testing.T) {
	t.Run("fixed-vectors", func(t *testing.T) {
		// Independently encoded from the documented v1 preimage with Python
		// json.dumps(separators=(',', ':')), struct.pack('>Q'), hashlib.sha256.
		got := mustIdentity(t, identityFixture())
		if got.SourceID != "sha256:5be13416494e40c866e2beeb8c0996f35a025b0138ba8d846b64a1d31162f38c" || got.SnapshotID != "sha256:925658fe840498a20e2056c3e9ccc20b54b3c2cc089ea520903ce1eeda2d7dea" || got.CollectionID != "sha256:21a9046d1b532bc51f66374e49dc070fc51f4212afaa384d90b7ba36204b0b09" {
			t.Fatal("ASSERT_IDENTITY_FIXED_VECTORS: v1 byte contract changed")
		}
	})
	t.Run("component-framing", func(t *testing.T) {
		if policyIdentity("source", []byte("ab"), []byte("c")) == policyIdentity("source", []byte("a"), []byte("bc")) {
			t.Fatal("ASSERT_IDENTITY_FRAMING: ambiguous concatenation")
		}
	})
	t.Run("revision-changes", func(t *testing.T) {
		r := identityFixture()
		r.Revision = &RevisionAttestation{System: "git", Revision: "first"}
		a := mustIdentity(t, r)
		r.Revision.Revision = "second"
		b := mustIdentity(t, r)
		if a.SourceID != b.SourceID || a.SnapshotID != b.SnapshotID || a.CollectionID != b.CollectionID || !bytes.Equal(a.Manifest, b.Manifest) || a.Revision.Revision == b.Revision.Revision {
			t.Fatal("ASSERT_IDENTITY_REVISION_SEPARATION: mutable attestation contaminated identities")
		}
	})
	t.Run("exclusion-reason", func(t *testing.T) {
		r := identityFixture()
		r.Decisions[1] = ManifestDecision{ReceiptID: "b", State: ManifestExclude, Reason: "not consumed"}
		a := mustIdentity(t, r)
		r.Decisions[1].Reason = "not requested"
		b := mustIdentity(t, r)
		if a.SourceID != b.SourceID || a.SnapshotID == b.SnapshotID || a.CollectionID == b.CollectionID {
			t.Fatal("ASSERT_IDENTITY_EXCLUSION_REASON: custody decision must bind snapshot, not source")
		}
	})
	t.Run("parent", func(t *testing.T) {
		r := identityFixture()
		a := mustIdentity(t, r)
		r.Receipts[1].ParentID = "a"
		b := mustIdentity(t, r)
		if a.SourceID != b.SourceID || a.SnapshotID == b.SnapshotID || a.CollectionID == b.CollectionID {
			t.Fatal("ASSERT_IDENTITY_PARENT: lineage must bind snapshot, not source")
		}
	})
	t.Run("cross-adapter-receipts", func(t *testing.T) {
		build := func(adapter string) IdentityResult {
			receipt, _, canonical, err := CanonicalizeReceipt(DiscoveredItem{ID: "a", Locator: "a.go"}, Acquisition{Status: Readable, Provenance: Provenance{Mechanism: adapter, Locator: "a.go"}}, []byte("package a"))
			if err != nil {
				t.Fatal(err)
			}
			sum := sha256.Sum256(canonical)
			id := "sha256:" + hex.EncodeToString(sum[:])
			return mustIdentity(t, IdentityRequest{Receipts: []ManifestReceipt{{ID: id, Path: "a.go", Digest: receipt.ContentIdentity.Digest}}, Decisions: []ManifestDecision{{ReceiptID: id, State: ManifestInclude}}, Acquisition: AcquisitionContext{Adapter: adapter}})
		}
		a, b := build("lsp"), build("offline")
		if a.SourceID != b.SourceID || a.SnapshotID == b.SnapshotID || a.CollectionID == b.CollectionID {
			t.Fatal("ASSERT_IDENTITY_CROSS_ADAPTER: receipt provenance is not source bytes")
		}
	})
	t.Run("domain-and-legacy", func(t *testing.T) {
		r := identityFixture()
		got := mustIdentity(t, r)
		manifest, err := AssembleManifest(r.Receipts, r.Decisions)
		if err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(manifest)
		if got.Policy != IdentityPolicyV1 || !bytes.Equal(got.Manifest, manifest) || got.LegacyManifestID != "sha256:"+hex.EncodeToString(sum[:]) {
			t.Fatal("ASSERT_IDENTITY_LEGACY_MANIFEST: canonical bytes and legacy formula must be retained")
		}
		ids := []string{got.SourceID, got.SnapshotID, got.CollectionID, got.LegacyManifestID, graph.SnapshotIdentity(manifest, []byte("[]"))}
		for i, id := range ids {
			if !manifestDigest.MatchString(id) {
				t.Fatal("ASSERT_IDENTITY_DOMAIN: malformed identity")
			}
			for j := 0; j < i; j++ {
				if id == ids[j] {
					t.Fatal("ASSERT_IDENTITY_DOMAIN: policies must not alias")
				}
			}
		}
	})
	t.Run("canonical-order", func(t *testing.T) {
		r := identityFixture()
		base := mustIdentity(t, r)
		r.Receipts[0], r.Receipts[1] = r.Receipts[1], r.Receipts[0]
		r.Decisions[0], r.Decisions[1] = r.Decisions[1], r.Decisions[0]
		if !reflect.DeepEqual(base, mustIdentity(t, r)) || base.SourceID == "" {
			t.Fatal("ASSERT_IDENTITY_ORDER: input order must not affect identity")
		}
	})
	for _, tc := range []struct {
		name                         string
		mutate                       func(*IdentityRequest)
		source, snapshot, collection bool
	}{
		{"bytes", func(r *IdentityRequest) { r.Receipts[0].Digest = "sha256:" + strings.Repeat("c", 64) }, true, true, true},
		{"path", func(r *IdentityRequest) { r.Receipts[0].Path = "c.go" }, true, true, true},
		{"receipt", func(r *IdentityRequest) { r.Receipts[0].ID = "new"; r.Decisions[0].ReceiptID = "new" }, false, true, true},
		{"decision", func(r *IdentityRequest) {
			r.Decisions[0].State = ManifestExclude
			r.Decisions[0].Reason = "outside scope"
		}, true, true, true},
		{"adapter", func(r *IdentityRequest) { r.Acquisition.Adapter = "offline" }, false, false, true},
		{"workspace", func(r *IdentityRequest) { r.Acquisition.WorkspaceURI = "file:///other" }, false, false, true},
		{"invocation", func(r *IdentityRequest) { r.Acquisition.InvocationID = "run-2" }, false, false, true},
		{"git", func(r *IdentityRequest) {
			r.Revision = &RevisionAttestation{System: "git", Revision: strings.Repeat("d", 40)}
		}, false, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := identityFixture()
			base := mustIdentity(t, r)
			tc.mutate(&r)
			got := mustIdentity(t, r)
			if base.SourceID == "" || (got.SourceID != base.SourceID) != tc.source || (got.SnapshotID != base.SnapshotID) != tc.snapshot || (got.CollectionID != base.CollectionID) != tc.collection {
				t.Fatalf("ASSERT_IDENTITY_%s: independent dimensions violated", strings.ToUpper(tc.name))
			}
		})
	}
	t.Run("excluded-bytes", func(t *testing.T) {
		r := identityFixture()
		r.Decisions[1] = ManifestDecision{ReceiptID: "b", State: ManifestExclude, Reason: "not consumed"}
		base := mustIdentity(t, r)
		r.Receipts[1].Digest = "sha256:" + strings.Repeat("c", 64)
		got := mustIdentity(t, r)
		if base.SourceID == "" || !reflect.DeepEqual(base, got) {
			t.Fatal("ASSERT_IDENTITY_EXCLUDED: excluded bytes are not committed by historical custody manifest")
		}
	})
	t.Run("immutable", func(t *testing.T) {
		r := identityFixture()
		r.Revision = &RevisionAttestation{System: "git", Revision: "abc"}
		got := mustIdentity(t, r)
		if got.Revision == nil || len(got.Manifest) == 0 {
			t.Fatal("ASSERT_IDENTITY_IMMUTABLE: missing owned values")
		}
		r.Revision.Revision = "changed"
		r.Receipts[0].Path = "changed.go"
		if got.Revision.Revision != "abc" {
			t.Fatal("ASSERT_IDENTITY_IMMUTABLE: aliased revision")
		}
		original := append([]byte(nil), got.Manifest...)
		got.Manifest[0] = '!'
		fresh := identityFixture()
		fresh.Revision = &RevisionAttestation{System: "git", Revision: "abc"}
		if !bytes.Equal(mustIdentity(t, fresh).Manifest, original) {
			t.Fatal("ASSERT_IDENTITY_IMMUTABLE: result mutates later builds")
		}
	})
	t.Run("invalid-manifest", func(t *testing.T) {
		r := identityFixture()
		r.Decisions = r.Decisions[:1]
		if _, err := BuildIdentity(r); err == nil {
			t.Fatal("ASSERT_IDENTITY_ACCOUNTING: incomplete manifest accepted")
		}
	})
	t.Run("invalid-revision", func(t *testing.T) {
		r := identityFixture()
		r.Revision = &RevisionAttestation{System: "git"}
		if _, err := BuildIdentity(r); err == nil {
			t.Fatal("ASSERT_IDENTITY_REVISION_VALIDATION: partial annotation accepted")
		}
	})
	t.Run("invalid-context", func(t *testing.T) {
		r := identityFixture()
		r.Acquisition.Adapter = ""
		if _, err := BuildIdentity(r); err == nil {
			t.Fatal("ASSERT_IDENTITY_CONTEXT_VALIDATION: ambiguous collection accepted")
		}
	})
}
