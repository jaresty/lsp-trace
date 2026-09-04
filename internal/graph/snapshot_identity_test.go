package graph

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"os"
	"strings"
	"testing"
)

func TestSnapshotIdentityCanonicalCompleteAndNonrecursive(t *testing.T) {
	manifest := []byte(`{"artifacts":[{"path":"a.go","sha256":"abc"}],"version":"v1"}`)
	receipts := []byte(`[{"path":"a.go","size":3}]`)

	h := sha256.New()
	h.Write([]byte("lsp-trace:source-snapshot:v1"))
	h.Write([]byte{0})
	var size [8]byte
	binary.BigEndian.PutUint64(size[:], uint64(len(manifest)))
	h.Write(size[:])
	h.Write(manifest)
	binary.BigEndian.PutUint64(size[:], uint64(len(receipts)))
	h.Write(size[:])
	h.Write(receipts)
	want := "sha256:" + hex.EncodeToString(h.Sum(nil))

	if got := SnapshotIdentity(manifest, receipts); got != want {
		t.Fatalf("ASSERT_SNAPSHOT_IDENTITY_EXACT_CANONICAL_DOMAIN_AND_FRAMING: got %q want %q", got, want)
	}
	if got := SnapshotIdentity(append([]byte(nil), manifest...), append([]byte(nil), receipts...)); got != want {
		t.Fatalf("ASSERT_SNAPSHOT_IDENTITY_DETERMINISTIC: got %q want %q", got, want)
	}
	if SnapshotIdentity([]byte(`{"version":"v2"}`), receipts) == want || SnapshotIdentity(manifest, []byte(`[]`)) == want {
		t.Fatal("ASSERT_SNAPSHOT_IDENTITY_COVERS_MANIFEST_AND_RECEIPTS: changed component retained identity")
	}
	if SnapshotIdentity([]byte("ab"), []byte("c")) == SnapshotIdentity([]byte("a"), []byte("bc")) {
		t.Fatal("ASSERT_SNAPSHOT_IDENTITY_LENGTH_FRAMING: component boundary collision")
	}

	source, err := os.ReadFile("snapshot_identity.go")
	if err != nil {
		t.Fatal(err)
	}
	body := string(source)
	if strings.Count(body, "func SnapshotIdentity(") != 1 || strings.Count(body, "SnapshotIdentity(") != 1 {
		t.Fatalf("ASSERT_SNAPSHOT_IDENTITY_NONRECURSIVE: source contains recursive or duplicate identity call: %q", body)
	}

	// The primitive is additive: it must not be referenced by historical V2/V3 serialization.
	v2, err := (Result{SchemaVersion: SchemaVersionV2, Summary: Summary{Complete: true}}).MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	v3, err := (Result{SchemaVersion: SchemaVersionV3, Summary: Summary{Complete: true}}).MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(v2, []byte("snapshot_identity")) || bytes.Contains(v3, []byte("snapshot_identity")) {
		t.Fatal("ASSERT_SNAPSHOT_IDENTITY_V2_V3_ADDITIVE_ONLY: historical bytes contain snapshot identity")
	}
}

func TestSnapshotBindingIdentityCanonicalCompleteAndNonrecursive(t *testing.T) {
	snapshot := "sha256:legacy-snapshot"
	manifest := []byte(`{"sources":[{"path":"a.go","sha256":"abc"}],"version":"v1"}`)
	members := []byte(`[{"path":"a.go","sha256":"abc"}]`)

	h := sha256.New()
	h.Write([]byte("lsp-trace:source-snapshot-binding:v1"))
	h.Write([]byte{0})
	var size [8]byte
	for _, component := range [][]byte{[]byte(snapshot), manifest, members} {
		binary.BigEndian.PutUint64(size[:], uint64(len(component)))
		h.Write(size[:])
		h.Write(component)
	}
	want := "sha256:" + hex.EncodeToString(h.Sum(nil))

	t.Run("exact domain framing and order", func(t *testing.T) {
		if got := SnapshotBindingIdentity(snapshot, manifest, members); got != want {
			t.Fatalf("ASSERT_SNAPSHOT_BINDING_EXACT_DOMAIN_FRAMING_ORDER: got %q want %q", got, want)
		}
	})
	got := SnapshotBindingIdentity(snapshot, manifest, members)
	t.Run("commits existing snapshot identity", func(t *testing.T) {
		if SnapshotBindingIdentity(snapshot+"-changed", manifest, members) == got {
			t.Fatal("ASSERT_SNAPSHOT_BINDING_COMMITS_SNAPSHOT_IDENTITY: changed snapshot retained binding")
		}
	})
	t.Run("commits canonical manifest", func(t *testing.T) {
		if SnapshotBindingIdentity(snapshot, append(append([]byte(nil), manifest...), ' '), members) == got {
			t.Fatal("ASSERT_SNAPSHOT_BINDING_COMMITS_CANONICAL_MANIFEST: changed manifest retained binding")
		}
	})
	t.Run("commits canonical members", func(t *testing.T) {
		if SnapshotBindingIdentity(snapshot, manifest, append(append([]byte(nil), members...), ' ')) == got {
			t.Fatal("ASSERT_SNAPSHOT_BINDING_COMMITS_CANONICAL_MEMBERS: changed members retained binding")
		}
	})
	t.Run("component boundaries cannot collide", func(t *testing.T) {
		if SnapshotBindingIdentity("ab", []byte("c"), []byte("d")) == SnapshotBindingIdentity("a", []byte("bc"), []byte("d")) {
			t.Fatal("ASSERT_SNAPSHOT_BINDING_LENGTH_FRAMING: component boundary collision")
		}
	})

	source, err := os.ReadFile("snapshot_identity.go")
	if err != nil {
		t.Fatal(err)
	}
	body := string(source)
	if strings.Count(body, "func SnapshotBindingIdentity(") != 1 || strings.Count(body, "SnapshotBindingIdentity(") != 1 {
		t.Fatalf("ASSERT_SNAPSHOT_BINDING_NONRECURSIVE: source contains recursive or duplicate binding call: %q", body)
	}
	if strings.Count(body, "func SnapshotIdentity(") != 1 || strings.Count(body, "SnapshotIdentity(") != 1 {
		t.Fatalf("ASSERT_SNAPSHOT_BINDING_PRESERVES_PRIMITIVE: legacy primitive was redefined or called: %q", body)
	}
}
