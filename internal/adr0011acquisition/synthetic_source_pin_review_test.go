package adr0011acquisition

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestReviewedSyntheticSourcePin(t *testing.T) {
	got, err := reviewedSyntheticSourcePin()
	if err != nil || got != reviewedSyntheticSourceDigest {
		t.Fatalf("reviewed source pin: %s %v", got, err)
	}
}

const reviewedSyntheticSourceDigest = "sha256:fa74cd343e0de7f067f000718592a2e09b3f25a1ad1e4f94c277a37425d16dd8"
const reviewedSyntheticManifestHash = "012e7bf737ced062e586acf6d8d69692c3e4d2f9163081235e1679fa9543b7cf"
const reviewedSyntheticReviewHash = "d17a4b3a63e884504c9aefcf1f4a4648823184eb3d763a3d4412b0cb71c25bf1"
const reviewedSyntheticSourceCount = 179
const reviewedSyntheticSourceBytes = 1795316

// Only tests select this independently reviewed local source-byte expectation.
// It is not a binary identity, production pin or public issuer.
func reviewedSyntheticSourcePin() (string, error) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return "", errSyntheticSourcePin
	}
	qualificationDir := filepath.Join(filepath.Dir(file), "..", "..", "docs", "qualification")
	manifestPath := filepath.Join(qualificationDir, "adr0011-synthetic-source-pin.012e7bf737ced062.manifest.json")
	b, err := os.ReadFile(manifestPath)
	if err != nil || len(b) > 1<<20 || len(b) == 0 {
		return "", errSyntheticSourcePin
	}
	review, err := os.ReadFile(filepath.Join(qualificationDir, "adr0011-synthetic-source-pin.012e7bf737ced062.review.md"))
	if err != nil || len(review) > 1<<20 || len(review) == 0 {
		return "", errSyntheticSourcePin
	}
	reviewDigest := sha256.Sum256(review)
	if hex.EncodeToString(reviewDigest[:]) != reviewedSyntheticReviewHash {
		return "", errSyntheticSourcePin
	}
	digest := sha256.Sum256(b)
	if hex.EncodeToString(digest[:]) != reviewedSyntheticManifestHash {
		return "", errSyntheticSourcePin
	}
	var manifest struct {
		SchemaVersion string `json:"schema_version"`
		Digest        string `json:"digest"`
		Files         []struct {
			Path       string `json:"path"`
			ByteLength uint64 `json:"byte_length"`
			SHA256     string `json:"sha256"`
		} `json:"files"`
	}
	if json.Unmarshal(b, &manifest) != nil || manifest.SchemaVersion != "ADR0011_SYNTHETIC_SOURCE_PIN_V1" || manifest.Digest != reviewedSyntheticSourceDigest || len(manifest.Files) != reviewedSyntheticSourceCount {
		return "", errSyntheticSourcePin
	}
	entries := make([]SyntheticSourceEntry, 0, len(manifest.Files))
	var totalBytes uint64
	for _, f := range manifest.Files {
		totalBytes += f.ByteLength
		if !strings.HasPrefix(f.SHA256, "sha256:") {
			return "", errSyntheticSourcePin
		}
		raw, err := hex.DecodeString(strings.TrimPrefix(f.SHA256, "sha256:"))
		if err != nil || len(raw) != 32 {
			return "", errSyntheticSourcePin
		}
		var hash [32]byte
		copy(hash[:], raw)
		entries = append(entries, SyntheticSourceEntry{Path: f.Path, ByteLength: f.ByteLength, SHA256: hash})
	}
	if totalBytes != reviewedSyntheticSourceBytes {
		return "", errSyntheticSourcePin
	}
	raw, err := hex.DecodeString(strings.TrimPrefix(reviewedSyntheticSourceDigest, "sha256:"))
	if err != nil || len(raw) != 32 {
		return "", errors.New("invalid test pin")
	}
	var expected [32]byte
	copy(expected[:], raw)
	if err := VerifySyntheticSourcePin(entries, expected); err != nil {
		return "", err
	}
	return reviewedSyntheticSourceDigest, nil
}
