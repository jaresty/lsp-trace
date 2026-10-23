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

const reviewedSyntheticSourceDigest = "sha256:12436ecd7d4a9bd7b079c7c8fe1a7e59309b82fd6f1a989b7258f5ec7f3461b5"
const reviewedSyntheticManifestHash = "fc825861052f8f4fc2bed9faf8fecfa3d9bcf9293ed6dd125c20b6b2c1cdc652"
const reviewedSyntheticReviewHash = "952dbeac3ef32ffe9f5eb3e93bf7528d99f97629d9cfb6024acb215237539487"

// Only tests select this independently reviewed local source-byte expectation.
// It is not a binary identity, production pin or public issuer.
func reviewedSyntheticSourcePin() (string, error) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return "", errSyntheticSourcePin
	}
	qualificationDir := filepath.Join(filepath.Dir(file), "..", "..", "docs", "qualification")
	manifestPath := filepath.Join(qualificationDir, "adr0011-synthetic-source-pin.fc825861052f8f4f.manifest.json")
	b, err := os.ReadFile(manifestPath)
	if err != nil || len(b) > 1<<20 || len(b) == 0 {
		return "", errSyntheticSourcePin
	}
	review, err := os.ReadFile(filepath.Join(qualificationDir, "adr0011-synthetic-source-pin.fc825861052f8f4f.review.md"))
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
	if json.Unmarshal(b, &manifest) != nil || manifest.SchemaVersion != "ADR0011_SYNTHETIC_SOURCE_PIN_V1" || manifest.Digest != reviewedSyntheticSourceDigest || len(manifest.Files) != 166 {
		return "", errSyntheticSourcePin
	}
	entries := make([]SyntheticSourceEntry, 0, len(manifest.Files))
	for _, f := range manifest.Files {
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
