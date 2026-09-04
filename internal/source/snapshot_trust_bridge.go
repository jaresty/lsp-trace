package source

import (
	"crypto/sha256"
	"encoding/hex"

	"lsp-trace/internal/schema"
)

// SnapshotTrustRequest composes canonical manifest inputs with optional,
// verifier-provisioned trust material.
type SnapshotTrustRequest struct {
	Receipts  []ManifestReceipt
	Decisions []ManifestDecision
	Trust     schema.TrustAdmissionRequest
}

// SnapshotTrustResult binds one canonical manifest to its trust outcome.
type SnapshotTrustResult struct {
	Manifest         []byte
	SourceSnapshotID string
	TrustAdmission   schema.TrustAdmissionResult
}

// BindSnapshotTrust assembles canonical manifest bytes, binds their identity,
// and applies optional verifier-provisioned trust without publishing.
func BindSnapshotTrust(request SnapshotTrustRequest) (SnapshotTrustResult, error) {
	manifest, err := AssembleManifest(request.Receipts, request.Decisions)
	if err != nil {
		return SnapshotTrustResult{}, err
	}
	sum := sha256.Sum256(manifest)
	snapshotID := "sha256:" + hex.EncodeToString(sum[:])

	trust := request.Trust
	trust.ClaimedSourceSnapshotIdentity = snapshotID
	admission, admissionErr := schema.AdmitTrust(trust)
	return SnapshotTrustResult{
		Manifest:         manifest,
		SourceSnapshotID: snapshotID,
		TrustAdmission:   admission,
	}, admissionErr
}
