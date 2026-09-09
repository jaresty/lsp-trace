package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"lsp-trace/internal/seedbinding"
)

func localValidationFiles(t *testing.T) (string, string, seedbinding.Manifest) {
	t.Helper()
	root := t.TempDir()
	source := []byte("package fixture\nfunc Seed() {}\n")
	sourcePath := filepath.Join(root, "seed.go")
	if err := os.WriteFile(sourcePath, source, 0600); err != nil {
		t.Fatal(err)
	}
	sourceSum := sha256.Sum256(source)
	prepared := seedbinding.PreparedModificationManifest{Version: 1, FinderSHA256: strings.Repeat("1", 64), SourceArchiveSHA256: strings.Repeat("2", 64), SourceTreeSHA256: strings.Repeat("3", 64), PreparedTreeSHA256: strings.Repeat("4", 64), AllowedChanges: []string{"other.go"}, TargetPath: "seed.go", TargetUnchangedSHA256: fmt.Sprintf("%x", sourceSum), SourceCommit: "fixture-revision", AdaptationIDs: []string{"fixture"}, PolicyID: "local", AssessmentID: "local", ContextID: "local"}
	preparedBytes, err := seedbinding.CanonicalPreparedManifest(prepared)
	if err != nil {
		t.Fatal(err)
	}
	preparedPath := filepath.Join(root, "prepared.json")
	if err := os.WriteFile(preparedPath, preparedBytes, 0600); err != nil {
		t.Fatal(err)
	}
	preparedSum := sha256.Sum256(preparedBytes)
	manifest := seedbinding.Manifest{SchemaVersion: seedbinding.VersionV3, CustodyMode: seedbinding.CallerAssertedLocal, PreparedManifestSHA256: fmt.Sprintf("%x", preparedSum), PreparedManifestPath: "prepared.json", ID: "fixture", Locator: seedbinding.Locator{URI: (&url.URL{Scheme: "file", Path: sourcePath}).String(), Line: 1, Character: 5, Encoding: "utf-8"}, ExpectedSymbol: "Seed", ExpectedDeclaringFile: "seed.go", ExpectedDeclarationRange: seedbinding.Range{StartLine: 1, StartCharacter: 0, EndLine: 1, EndCharacter: 14}, SourceRevision: "fixture-revision", SourceSHA256: fmt.Sprintf("%x", sourceSum), Validator: seedbinding.ValidatorIdentity{Language: "go", Authority: "MANAGED_LSP", Name: "never-started", Version: "1", Class: "managed-stdio", ExecutableSHA256: "sha256:" + strings.Repeat("5", 64), PayloadSHA256: "sha256:" + strings.Repeat("6", 64), ConfigSHA256: "sha256:" + strings.Repeat("7", 64)}}
	manifestBytes, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	manifestPath := filepath.Join(root, "seed-manifest.json")
	if err := os.WriteFile(manifestPath, manifestBytes, 0600); err != nil {
		t.Fatal(err)
	}
	return root, manifestPath, manifest
}

func TestSeedValidateLocalIsProcessFreeAndProjectsReceipt(t *testing.T) {
	root, manifestPath, _ := localValidationFiles(t)
	marker := filepath.Join(root, "provider-started")
	var stdout, stderr bytes.Buffer
	code := run([]string{"seed", "validate-local", "--workspace", root, "--seed-manifest", manifestPath}, strings.NewReader(""), &stdout, &stderr)
	if code != 0 || stderr.Len() != 0 || strings.Contains(stdout.String(), "VERIFIED_HOST") || !strings.Contains(stdout.String(), `"provenance":"CALLER_ASSERTED_LOCAL"`) || !strings.Contains(stdout.String(), `"authenticated":false`) {
		t.Fatalf("ASSERT_VALIDATE_LOCAL_RECEIPT: code=%d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("ASSERT_VALIDATE_LOCAL_PROCESS_NOT_STARTED: %v", err)
	}
}

func TestSeedValidateLocalCanonicalWrongDigestParity(t *testing.T) {
	root, manifestPath, manifest := localValidationFiles(t)
	manifest.PreparedManifestSHA256 = strings.Repeat("0", 64)
	raw, _ := json.Marshal(manifest)
	if err := os.WriteFile(manifestPath, raw, 0600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	code := run([]string{"seed", "validate-local", "--workspace", root, "--seed-manifest", manifestPath}, strings.NewReader(""), &stdout, &stderr)
	if code != 1 || !strings.Contains(stdout.String(), `"status":"MISMATCH"`) || !strings.Contains(stdout.String(), `"code":"SEED_BINDING_MISMATCH"`) || strings.Contains(stdout.String(), "VERIFIED_HOST") {
		t.Fatalf("ASSERT_VALIDATE_LOCAL_CANONICAL_WRONG_DIGEST: code=%d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
}
