package sessionruntime

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"lsp-trace/internal/strictjson"
)

type c07PacketSource struct {
	Path       string `json:"path"`
	ByteLength int64  `json:"byte_length"`
	SHA256     string `json:"sha256"`
}

type c07PacketRevision struct {
	Commit string `json:"commit"`
	Tree   string `json:"tree"`
	Role   string `json:"role"`
}

type c07PacketExpected struct {
	ExitSuccess bool     `json:"exit_success"`
	PassedCount int      `json:"passed_count"`
	FailedCount int      `json:"failed_count"`
	PassedNames []string `json:"passed_names"`
	Assertions  []string `json:"assertions"`
}

type c07PacketManifest struct {
	SchemaVersion  string            `json:"schema_version"`
	Authority      string            `json:"authority"`
	ClaimCeiling   []string          `json:"claim_ceiling"`
	Revision       c07PacketRevision `json:"revision"`
	Sources        []c07PacketSource `json:"sources"`
	Command        []string          `json:"command"`
	SelectedNames  []string          `json:"selected_names"`
	SelectedCount  int               `json:"selected_count"`
	ExpectedResult c07PacketExpected `json:"expected_result"`
}

func c07FileIdentity(path string) (int64, string, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return 0, "", err
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return 0, "", fmt.Errorf("not a regular non-symlink file")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return 0, "", err
	}
	sum := sha256.Sum256(raw)
	return int64(len(raw)), "sha256:" + hex.EncodeToString(sum[:]), nil
}

func c07LowerHex40(value string) bool {
	if len(value) != 40 || value == strings.Repeat("0", 40) {
		return false
	}
	for _, r := range value {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f')) {
			return false
		}
	}
	return true
}

func TestC07PortableP2Packet(t *testing.T) {
	const packetDir = "testdata/adr0011-c07-p2-v1"
	const manifestName = "manifest.json"
	if info, err := os.Lstat(packetDir); err != nil {
		if os.IsNotExist(err) {
			t.Fatal("ASSERT_C07_PORTABLE_PACKET_MISSING")
		}
		t.Fatalf("ASSERT_C07_PORTABLE_PACKET_ROOT: %v", err)
	} else if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		t.Fatal("ASSERT_C07_PORTABLE_PACKET_ROOT")
	}
	manifestPath := filepath.Join(packetDir, manifestName)
	raw, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("ASSERT_C07_PORTABLE_PACKET_MANIFEST: %v", err)
	}
	if len(raw) == 0 || raw[len(raw)-1] != '\n' || bytes.Count(raw, []byte{'\n'}) != 1 {
		t.Fatal("ASSERT_C07_PORTABLE_PACKET_CANONICAL_NEWLINE")
	}
	if err := strictjson.RejectDuplicates(raw); err != nil {
		t.Fatalf("ASSERT_C07_PORTABLE_PACKET_DUPLICATE_KEY: %v", err)
	}
	var manifest c07PacketManifest
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&manifest); err != nil {
		t.Fatalf("ASSERT_C07_PORTABLE_PACKET_SCHEMA: %v", err)
	}
	var generic any
	if err := json.Unmarshal(raw, &generic); err != nil {
		t.Fatalf("ASSERT_C07_PORTABLE_PACKET_CANONICAL_JSON: %v", err)
	}
	canonical, err := json.Marshal(generic)
	if err != nil || !bytes.Equal(raw, append(canonical, '\n')) {
		t.Fatal("ASSERT_C07_PORTABLE_PACKET_CANONICAL_JSON")
	}
	if manifest.SchemaVersion != "lsp-trace.private.adr0011-c07-p2-packet.v1" || manifest.Authority != "HERMETIC_GO_SOURCE_AND_TEST_EVIDENCE_ONLY" || manifest.Revision.Role != "PRE_TEST_PRODUCTION_BASELINE" {
		t.Fatal("ASSERT_C07_PORTABLE_PACKET_IDENTITY")
	}
	const wantBaselineCommit = "2b3358f560073ffbaf203d573a5938c98a983d2e"
	const wantBaselineTree = "6bf02d8067361e640bc0a715d16004eb21030235"
	if manifest.Revision.Commit != wantBaselineCommit || manifest.Revision.Tree != wantBaselineTree || !c07LowerHex40(manifest.Revision.Commit) || !c07LowerHex40(manifest.Revision.Tree) {
		t.Fatal("ASSERT_C07_PORTABLE_PACKET_REVISION")
	}
	wantCeiling := []string{"NO_BROADER_MATRIX_CLOSURE", "NO_C08_CONTINUITY", "NO_LIVE_SERVER_OR_RUNTIME_PROOF", "NO_MANAGED_SESSION_IDENTITY", "NO_PRODUCTION_AUTHORITY"}
	if !reflect.DeepEqual(manifest.ClaimCeiling, wantCeiling) {
		t.Fatal("ASSERT_C07_PORTABLE_PACKET_CLAIM_CEILING")
	}
	wantCommand := []string{"go", "test", "./sessionruntime", "-run", "^(TestC07PrivateP2IntegratedPreBlockBoundary|TestC07PortableP2Packet)$", "-count=1"}
	wantNames := []string{"TestC07PrivateP2IntegratedPreBlockBoundary", "TestC07PortableP2Packet"}
	wantPassedNames := []string{"TestC07PrivateP2IntegratedPreBlockBoundary", "TestC07PrivateP2IntegratedPreBlockBoundary/before", "TestC07PrivateP2IntegratedPreBlockBoundary/equality", "TestC07PrivateP2IntegratedPreBlockBoundary/plus-one", "TestC07PortableP2Packet"}
	wantAssertions := []string{"ASSERT_C07_CLOCK_CALL_COUNT", "ASSERT_C07_INTEGRATED_BOUNDARY", "ASSERT_C07_PREBLOCK_ZERO_MUTATION", "ASSERT_C07_PORTABLE_PACKET_CANONICAL_JSON", "ASSERT_C07_PORTABLE_PACKET_SOURCE_IDENTITY"}
	if !reflect.DeepEqual(manifest.Command, wantCommand) || !reflect.DeepEqual(manifest.SelectedNames, wantNames) || manifest.SelectedCount != len(wantNames) || !manifest.ExpectedResult.ExitSuccess || manifest.ExpectedResult.PassedCount != 5 || manifest.ExpectedResult.FailedCount != 0 || !reflect.DeepEqual(manifest.ExpectedResult.PassedNames, wantPassedNames) || !reflect.DeepEqual(manifest.ExpectedResult.Assertions, wantAssertions) {
		t.Fatal("ASSERT_C07_PORTABLE_PACKET_EXECUTION_BINDING")
	}
	wantPaths := []string{"sessionruntime/adr0011_c07_integrated_private_test.go", "sessionruntime/adr0011_c07_packet_private_test.go", "sessionruntime/adr0011_p2_deadline.go", "sessionruntime/sessionruntime.go"}
	if len(manifest.Sources) != len(wantPaths) {
		t.Fatal("ASSERT_C07_PORTABLE_PACKET_SOURCE_SET")
	}
	root, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	gotPaths := make([]string, 0, len(manifest.Sources))
	previous := ""
	for _, source := range manifest.Sources {
		if source.Path == "" || strings.Contains(source.Path, "\\") || filepath.IsAbs(source.Path) || filepath.Clean(source.Path) != source.Path || strings.HasPrefix(source.Path, "../") || source.Path <= previous {
			t.Fatalf("ASSERT_C07_PORTABLE_PACKET_SOURCE_PATH path=%q", source.Path)
		}
		previous = source.Path
		full := filepath.Join(root, filepath.FromSlash(source.Path))
		if rel, err := filepath.Rel(root, full); err != nil || strings.HasPrefix(rel, "..") {
			t.Fatalf("ASSERT_C07_PORTABLE_PACKET_SOURCE_PATH path=%q", source.Path)
		}
		length, digest, err := c07FileIdentity(full)
		if err != nil || length != source.ByteLength || digest != source.SHA256 {
			t.Fatalf("ASSERT_C07_PORTABLE_PACKET_SOURCE_IDENTITY path=%q length=%d digest=%s err=%v", source.Path, length, digest, err)
		}
		gotPaths = append(gotPaths, source.Path)
	}
	if !sort.StringsAreSorted(gotPaths) || !reflect.DeepEqual(gotPaths, wantPaths) {
		t.Fatal("ASSERT_C07_PORTABLE_PACKET_SOURCE_SET")
	}
	for _, name := range []string{manifestName, "README.md"} {
		if _, _, err := c07FileIdentity(filepath.Join(packetDir, name)); err != nil {
			t.Fatalf("ASSERT_C07_PORTABLE_PACKET_CONTAINMENT path=%q err=%v", name, err)
		}
	}
	entries, err := os.ReadDir(packetDir)
	if err != nil || len(entries) != 2 {
		t.Fatal("ASSERT_C07_PORTABLE_PACKET_CONTAINMENT")
	}
}
