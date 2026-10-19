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

const reviewedSyntheticSourceDigest = "sha256:ea0cf4c563dfccd00b55398a5a6830c2d6f28d5a2d8b5c84357972d1b695c50c"
const reviewedSyntheticManifestHash = "33e3f6654709c47e6b0c484590cbb897b9d04114ec1f550bb3625ec086ac10c9"

// Only tests select this independently reviewed local source-byte expectation.
// It is not a binary identity, production pin or public issuer.
func reviewedSyntheticSourcePin() (string, error) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return "", errSyntheticSourcePin
	}
	manifestPath := filepath.Join(filepath.Dir(file), "..", "..", "docs", "qualification", "adr0011-synthetic-source-pin.33e3f6654709c47e.manifest.json")
	b, err := os.ReadFile(manifestPath)
	if err != nil || len(b) > 1<<20 || len(b) == 0 {
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
	if json.Unmarshal(b, &manifest) != nil || manifest.SchemaVersion != "ADR0011_SYNTHETIC_SOURCE_PIN_V1" || manifest.Digest != reviewedSyntheticSourceDigest || len(manifest.Files) != 128 {
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
