package source

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
)

// IdentityPolicyV1 is opt-in; it does not reinterpret any historical identity.
const IdentityPolicyV1 = "lsp-trace.operational-identity.v1"

// RevisionAttestation is a claimant-supplied annotation, not authenticated trust.
type RevisionAttestation struct {
	System   string `json:"system"`
	Revision string `json:"revision"`
}

type AcquisitionContext struct {
	Adapter      string `json:"adapter"`
	WorkspaceURI string `json:"workspace_uri"`
	InvocationID string `json:"invocation_id"`
}

// IdentityRequest requires Receipts' Digest fields to identify acquired content
// bytes, not provenance-bearing receipt JSON. IDs may reference canonical receipt
// hashes. The caller supplies actual-read evidence; this API validates structure.
type IdentityRequest struct {
	Receipts    []ManifestReceipt
	Decisions   []ManifestDecision
	Revision    *RevisionAttestation
	Acquisition AcquisitionContext
}

type IdentityResult struct {
	Policy           string
	SourceID         string
	SnapshotID       string
	CollectionID     string
	LegacyManifestID string
	Manifest         []byte
	Revision         *RevisionAttestation
	Acquisition      AcquisitionContext
}

// BuildIdentity separates included source content from custody and acquisition.
// Revision is optional descriptive metadata, never evidence of authentication.
// The returned manifest and revision are owned copies. IDs commit to their build
// time inputs; mutating a returned value requires building a new identity.
func BuildIdentity(request IdentityRequest) (IdentityResult, error) {
	if strings.TrimSpace(request.Acquisition.Adapter) == "" {
		return IdentityResult{}, fmt.Errorf("identity: acquisition adapter is required")
	}
	var revision *RevisionAttestation
	if request.Revision != nil {
		if strings.TrimSpace(request.Revision.System) == "" || strings.TrimSpace(request.Revision.Revision) == "" {
			return IdentityResult{}, fmt.Errorf("identity: revision annotation requires system and revision")
		}
		copy := *request.Revision
		revision = &copy
	}
	manifest, err := AssembleManifest(request.Receipts, request.Decisions)
	if err != nil {
		return IdentityResult{}, err
	}
	var decoded SourceManifest
	if err := json.Unmarshal(manifest, &decoded); err != nil {
		return IdentityResult{}, err
	}
	type contentEntry struct {
		Path   string `json:"path"`
		Digest string `json:"digest"`
	}
	// AssembleManifest sorts included receipts by path and rejects duplicate paths.
	content := make([]contentEntry, 0, len(decoded.Included))
	for _, receipt := range decoded.Included {
		content = append(content, contentEntry{receipt.Path, receipt.Digest})
	}
	canonicalContent, err := json.Marshal(content)
	if err != nil {
		return IdentityResult{}, err
	}
	context, err := json.Marshal(request.Acquisition)
	if err != nil {
		return IdentityResult{}, err
	}
	sourceID := policyIdentity("source", canonicalContent)
	snapshotID := policyIdentity("snapshot", []byte(sourceID), manifest)
	collectionID := policyIdentity("collection", []byte(snapshotID), context)
	legacy := sha256.Sum256(manifest)
	return IdentityResult{
		Policy: IdentityPolicyV1, SourceID: sourceID, SnapshotID: snapshotID, CollectionID: collectionID,
		LegacyManifestID: "sha256:" + hex.EncodeToString(legacy[:]), Manifest: manifest,
		Revision: revision, Acquisition: request.Acquisition,
	}, nil
}

// policyIdentity frames each component independently; domain and version are
// part of the hash preimage, not aliases attached to an unversioned digest.
func policyIdentity(kind string, components ...[]byte) string {
	h := sha256.New()
	_, _ = h.Write([]byte(IdentityPolicyV1 + ":" + kind + "\x00"))
	for _, component := range components {
		var size [8]byte
		binary.BigEndian.PutUint64(size[:], uint64(len(component)))
		_, _ = h.Write(size[:])
		_, _ = h.Write(component)
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil))
}
