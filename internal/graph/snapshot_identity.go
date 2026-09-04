package graph

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
)

const (
	snapshotIdentityDomain        = "lsp-trace:source-snapshot:v1"
	snapshotBindingIdentityDomain = "lsp-trace:source-snapshot-binding:v1"
)

// SnapshotIdentity returns the domain-separated identity of a canonical source
// manifest and its canonical admitted artifact receipts. Length framing makes
// both complete components unambiguous. The identity is deliberately computed
// directly from those bytes, never from a structure containing the identity.
func SnapshotIdentity(canonicalManifest, canonicalArtifactReceipts []byte) string {
	h := sha256.New()
	_, _ = h.Write([]byte(snapshotIdentityDomain))
	_, _ = h.Write([]byte{0})
	writeSnapshotIdentityComponent(h, canonicalManifest)
	writeSnapshotIdentityComponent(h, canonicalArtifactReceipts)
	return "sha256:" + hex.EncodeToString(h.Sum(nil))
}

// SnapshotBindingIdentity binds an existing snapshot identity to its validated
// canonical source manifest and canonical member commitments. All inputs must
// be finalized before this nonrecursive identity is computed.
func SnapshotBindingIdentity(snapshotIdentity string, canonicalManifest, canonicalMemberCommitments []byte) string {
	h := sha256.New()
	_, _ = h.Write([]byte(snapshotBindingIdentityDomain))
	_, _ = h.Write([]byte{0})
	writeSnapshotIdentityComponent(h, []byte(snapshotIdentity))
	writeSnapshotIdentityComponent(h, canonicalManifest)
	writeSnapshotIdentityComponent(h, canonicalMemberCommitments)
	return "sha256:" + hex.EncodeToString(h.Sum(nil))
}

func writeSnapshotIdentityComponent(h interface{ Write([]byte) (int, error) }, component []byte) {
	var size [8]byte
	binary.BigEndian.PutUint64(size[:], uint64(len(component)))
	_, _ = h.Write(size[:])
	_, _ = h.Write(component)
}
