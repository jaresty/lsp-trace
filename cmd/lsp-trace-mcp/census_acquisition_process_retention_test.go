package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"lsp-trace/internal/captureset"
	"lsp-trace/internal/publication"
)

const censusCaptureRetentionEnv = "LSP_TRACE_TEST_CENSUS_CAPTURE_RETENTION_DIR"

type retainedCensusMetadata struct {
	Selector     string `json:"selector"`
	SourceSHA256 string `json:"source_sha256"`
	CopySHA256   string `json:"copy_sha256"`
	ByteLength   int    `json:"byte_length"`
	SourceMode   uint32 `json:"source_mode"`
	CopyMode     uint32 `json:"copy_mode"`
	Source       string `json:"source"`
}

func TestCensusCaptureRetentionDisabledHasNoSideEffect(t *testing.T) {
	parent := t.TempDir()
	if err := os.Chmod(parent, 0o700); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadDir(parent)
	if err != nil {
		t.Fatal(err)
	}
	if got, source := retainCommittedCensusForTest(t, "", filepath.Join(parent, "missing-publication"), ""); got != "" || source != nil {
		t.Fatalf("ASSERT_RETENTION_DISABLED_NO_RESULT: leaf=%q source=%v", got, source != nil)
	}
	after, err := os.ReadDir(parent)
	if err != nil || len(before) != 0 || len(after) != 0 {
		t.Fatalf("ASSERT_RETENTION_DISABLED_NO_FILES: before=%d after=%d err=%v", len(before), len(after), err)
	}
}

func retainCommittedCensusForTest(t *testing.T, optIn, publicationRoot, selector string) (string, []byte) {
	t.Helper()
	if optIn == "" {
		return "", nil
	}
	if err := captureset.ValidatePublicationSelector(selector); err != nil {
		t.Fatalf("ASSERT_RETENTION_SELECTOR_VALID: %v", err)
	}
	parentInfo, err := os.Lstat(optIn)
	if err != nil || !parentInfo.IsDir() || parentInfo.Mode()&os.ModeSymlink != 0 || parentInfo.Mode().Perm() != 0o700 {
		t.Fatalf("ASSERT_RETENTION_PARENT_PRIVATE_NOFOLLOW: mode/path invalid")
	}
	parent, err := filepath.Abs(optIn)
	if err != nil || filepath.Clean(parent) != parent {
		t.Fatalf("ASSERT_RETENTION_PARENT_ABSOLUTE: invalid path")
	}
	rootPath, err := filepath.Abs(publicationRoot)
	if err != nil || filepath.Clean(rootPath) != rootPath {
		t.Fatalf("ASSERT_RETENTION_PUBLICATION_ROOT_PATH: invalid path")
	}
	root, err := publication.OpenRoot(rootPath)
	if err != nil {
		t.Fatalf("ASSERT_RETENTION_PUBLICATION_ROOT_OPEN: %v", err)
	}
	defer root.Close()
	if err := root.ValidatePrivate(); err != nil {
		t.Fatalf("ASSERT_RETENTION_PUBLICATION_ROOT_PRIVATE: %v", err)
	}
	raw, err := publication.ReadVerifiedBoundFile(root, selector, captureset.MaxBundleBytes)
	if err != nil {
		t.Fatalf("ASSERT_RETENTION_COMMITTED_BUNDLE_READ: %v", err)
	}
	primaryPath := filepath.Join(rootPath, filepath.FromSlash(selector))
	primaryInfo, err := os.Lstat(primaryPath)
	if err != nil || primaryInfo.Mode()&os.ModeSymlink != 0 || !primaryInfo.Mode().IsRegular() || primaryInfo.Mode().Perm() != 0o600 || int64(len(raw)) != primaryInfo.Size() {
		t.Fatalf("ASSERT_RETENTION_PRIMARY_MODE_AND_SIZE: invalid committed primary")
	}
	primaryHash := sha256.Sum256(raw)
	primaryDigest := "sha256:" + hex.EncodeToString(primaryHash[:])
	leaf, err := os.MkdirTemp(parent, "census-capture-")
	if err != nil {
		t.Fatalf("ASSERT_RETENTION_LEAF_CREATE: %v", err)
	}
	if err := os.Chmod(leaf, 0o700); err != nil {
		t.Fatalf("ASSERT_RETENTION_LEAF_CHMOD: %v", err)
	}
	leafInfo, err := os.Lstat(leaf)
	if err != nil || !leafInfo.IsDir() || leafInfo.Mode()&os.ModeSymlink != 0 || leafInfo.Mode().Perm() != 0o700 {
		t.Fatalf("ASSERT_RETENTION_LEAF_PRIVATE_NOFOLLOW: invalid leaf")
	}
	bundleName := filepath.Base(selector)
	bundlePath := filepath.Join(leaf, bundleName)
	f, err := os.OpenFile(bundlePath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		t.Fatalf("ASSERT_RETENTION_COPY_EXCLUSIVE_CREATE: %v", err)
	}
	if err := f.Chmod(0o600); err != nil {
		f.Close()
		t.Fatalf("ASSERT_RETENTION_COPY_MODE: %v", err)
	}
	written, writeErr := f.Write(raw)
	syncErr := f.Sync()
	closeErr := f.Close()
	if writeErr != nil || written != len(raw) || syncErr != nil || closeErr != nil {
		t.Fatalf("ASSERT_RETENTION_COPY_WRITE: write=%v sync=%v close=%v", writeErr, syncErr, closeErr)
	}
	copied, err := os.ReadFile(bundlePath)
	if err != nil {
		t.Fatalf("ASSERT_RETENTION_COPY_READBACK: %v", err)
	}
	copyHash := sha256.Sum256(copied)
	copyDigest := "sha256:" + hex.EncodeToString(copyHash[:])
	if !equalRetentionBytes(raw, copied) || copyDigest != primaryDigest {
		t.Fatalf("ASSERT_RETENTION_COPY_INTEGRITY: source=%s copy=%s", primaryDigest, copyDigest)
	}
	copyInfo, err := os.Lstat(bundlePath)
	if err != nil || copyInfo.Mode()&os.ModeSymlink != 0 || !copyInfo.Mode().IsRegular() || copyInfo.Mode().Perm() != 0o600 {
		t.Fatalf("ASSERT_RETENTION_COPY_PRIVATE_NOFOLLOW: invalid copy mode")
	}
	metadata := retainedCensusMetadata{Selector: selector, SourceSHA256: primaryDigest, CopySHA256: copyDigest, ByteLength: len(raw), SourceMode: uint32(primaryInfo.Mode().Perm()), CopyMode: uint32(copyInfo.Mode().Perm()), Source: "committed publication path; producer authentication not asserted"}
	metadataBytes, err := json.Marshal(metadata)
	if err != nil {
		t.Fatalf("ASSERT_RETENTION_METADATA_ENCODE: %v", err)
	}
	meta, err := os.OpenFile(filepath.Join(leaf, "metadata.json"), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		t.Fatalf("ASSERT_RETENTION_METADATA_CREATE: %v", err)
	}
	if _, err := meta.Write(append(metadataBytes, '\n')); err != nil {
		meta.Close()
		t.Fatalf("ASSERT_RETENTION_METADATA_WRITE: %v", err)
	}
	if err := meta.Sync(); err != nil {
		meta.Close()
		t.Fatalf("ASSERT_RETENTION_METADATA_SYNC: %v", err)
	}
	if err := meta.Close(); err != nil {
		t.Fatalf("ASSERT_RETENTION_METADATA_CLOSE: %v", err)
	}
	t.Logf("RETENTION_COPY path=%q selector=%q sha256=%s bytes=%d dir_mode=0700 file_mode=0600", leaf, selector, primaryDigest, len(raw))
	return leaf, raw
}

func equalRetentionBytes(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	var difference byte
	for i := range a {
		difference |= a[i] ^ b[i]
	}
	return difference == 0
}

func retentionSelectorFromResponse(response map[string]any) (string, error) {
	result, ok := response["result"].(map[string]any)
	if !ok {
		return "", errors.New("JSON-RPC result absent or wrong type")
	}
	envelope, ok := result["structuredContent"].(map[string]any)
	if !ok {
		return "", errors.New("structuredContent envelope absent or wrong type")
	}
	diagnostic, ok := envelope["result"].(map[string]any)
	if !ok {
		return "", errors.New("continuation diagnostic result absent or wrong type")
	}
	selector, ok := diagnostic["preserved_census_selector"].(string)
	if !ok || selector == "" {
		return "", errors.New("preserved census selector absent or wrong type")
	}
	return selector, nil
}

func TestRetentionSelectorFromJSONRPCStructuredContent(t *testing.T) {
	const selector = "capture-sets/v1/sha256/ccabe8c772ef273a5a379805602e3fda8f2ee9aa918492a7e1ac28c26329a5ca.bundle"
	valid := map[string]any{"result": map[string]any{"structuredContent": map[string]any{"result": map[string]any{"preserved_census_selector": selector}}}}
	tests := []struct {
		name     string
		response map[string]any
		want     string
		wantErr  bool
	}{
		{name: "actual JSON-RPC nested shape", response: valid, want: selector},
		{name: "missing JSON-RPC result", response: map[string]any{}, wantErr: true},
		{name: "wrong JSON-RPC result type", response: map[string]any{"result": "wrong"}, wantErr: true},
		{name: "missing structuredContent", response: map[string]any{"result": map[string]any{}}, wantErr: true},
		{name: "wrong structuredContent type", response: map[string]any{"result": map[string]any{"structuredContent": "wrong"}}, wantErr: true},
		{name: "missing diagnostic result", response: map[string]any{"result": map[string]any{"structuredContent": map[string]any{}}}, wantErr: true},
		{name: "wrong diagnostic result type", response: map[string]any{"result": map[string]any{"structuredContent": map[string]any{"result": "wrong"}}}, wantErr: true},
		{name: "missing selector", response: map[string]any{"result": map[string]any{"structuredContent": map[string]any{"result": map[string]any{}}}}, wantErr: true},
		{name: "wrong selector type", response: map[string]any{"result": map[string]any{"structuredContent": map[string]any{"result": map[string]any{"preserved_census_selector": 7}}}}, wantErr: true},
		{name: "empty selector", response: map[string]any{"result": map[string]any{"structuredContent": map[string]any{"result": map[string]any{"preserved_census_selector": ""}}}}, wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := retentionSelectorFromResponse(tc.response)
			if (err != nil) != tc.wantErr || got != tc.want {
				t.Fatalf("ASSERT_RETENTION_SELECTOR_EXTRACTION_%s: got=%q err=%v want=%q wantErr=%t", tc.name, got, err, tc.want, tc.wantErr)
			}
		})
	}
}

func verifyRetentionCopy(t *testing.T, leaf, selector string, expected []byte) {
	t.Helper()
	path := filepath.Join(leaf, filepath.Base(selector))
	copied, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ASSERT_RETENTION_COPY_EXISTS: %v", err)
	}
	if !equalRetentionBytes(expected, copied) {
		t.Fatalf("ASSERT_RETENTION_COPY_INTEGRITY: copy does not match source")
	}
	corrupt := append([]byte(nil), copied...)
	if len(corrupt) == 0 {
		t.Fatal("ASSERT_RETENTION_COPY_NONEMPTY: empty bundle")
	}
	corrupt[0] ^= 1
	originalHash := sha256.Sum256(expected)
	corruptHash := sha256.Sum256(corrupt)
	if equalRetentionBytes(expected, corrupt) || originalHash == corruptHash {
		t.Fatal("ASSERT_RETENTION_CORRUPTION_WITNESS: changed byte was not detected")
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("ASSERT_RETENTION_COPY_MODE_0600: mode=%v err=%v", info, err)
	}
	leafInfo, err := os.Stat(leaf)
	if err != nil || leafInfo.Mode().Perm() != 0o700 {
		t.Fatalf("ASSERT_RETENTION_DIR_MODE_0700: mode=%v err=%v", leafInfo, err)
	}
}

func retentionChildEnvironment(environment []string) []string {
	filtered := make([]string, 0, len(environment))
	for _, entry := range environment {
		if !strings.HasPrefix(entry, censusCaptureRetentionEnv+"=") {
			filtered = append(filtered, entry)
		}
	}
	return filtered
}

func retentionParentFromEnv(t *testing.T) string {
	t.Helper()
	value := strings.TrimSpace(os.Getenv(censusCaptureRetentionEnv))
	if value == "" {
		return ""
	}
	if !filepath.IsAbs(value) || filepath.Clean(value) != value {
		t.Fatalf("ASSERT_RETENTION_OPT_IN_ABSOLUTE_CLEAN_PATH: invalid configured parent")
	}
	return value
}
