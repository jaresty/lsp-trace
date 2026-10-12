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
	"strings"
	"testing"

	"lsp-trace/internal/strictjson"
)

type c08PacketSource struct {
	Path       string `json:"path"`
	ByteLength int64  `json:"byte_length"`
	SHA256     string `json:"sha256"`
}
type c08PacketRevision struct {
	Commit string `json:"commit"`
	Tree   string `json:"tree"`
	Role   string `json:"role"`
}
type c08PacketExpected struct {
	ExitSuccess bool     `json:"exit_success"`
	PassedCount int      `json:"passed_count"`
	FailedCount int      `json:"failed_count"`
	PassedNames []string `json:"passed_names"`
	Assertions  []string `json:"assertions"`
}
type c08PacketManifest struct {
	SchemaVersion  string            `json:"schema_version"`
	Authority      string            `json:"authority"`
	ClaimCeiling   []string          `json:"claim_ceiling"`
	Revision       c08PacketRevision `json:"revision"`
	Sources        []c08PacketSource `json:"sources"`
	Command        []string          `json:"command"`
	SelectedNames  []string          `json:"selected_names"`
	SelectedCount  int               `json:"selected_count"`
	ExpectedResult c08PacketExpected `json:"expected_result"`
}

var c08PacketSources = []string{"sessionruntime/adr0011_c08_lifecycle_private_test.go", "sessionruntime/adr0011_c08_packet_private_test.go", "sessionruntime/adr0011_c08_preflight_red_private_test.go", "sessionruntime/sessionruntime.go", "sessionruntime/startup_attempt.go"}

func c08Identity(path string) (int64, string, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return 0, "", err
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return 0, "", fmt.Errorf("not regular")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return 0, "", err
	}
	sum := sha256.Sum256(raw)
	return int64(len(raw)), "sha256:" + hex.EncodeToString(sum[:]), nil
}
func c08Canonical(raw []byte, out any) error {
	if len(raw) == 0 || raw[len(raw)-1] != '\n' || bytes.Count(raw, []byte{'\n'}) != 1 {
		return fmt.Errorf("newline")
	}
	if err := strictjson.RejectDuplicates(raw); err != nil {
		return err
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(out); err != nil {
		return err
	}
	var generic any
	if err := json.Unmarshal(raw, &generic); err != nil {
		return err
	}
	canonical, err := json.Marshal(generic)
	if err != nil || !bytes.Equal(raw, append(canonical, '\n')) {
		return fmt.Errorf("canonical")
	}
	return nil
}

func TestC08PortableP2Packet(t *testing.T) {
	const dir = "testdata/adr0011-c08-p2-v1"
	info, err := os.Lstat(dir)
	if err != nil {
		if os.IsNotExist(err) {
			t.Fatal("ASSERT_C08_PORTABLE_PACKET_MISSING")
		}
		t.Fatal(err)
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		t.Fatal("ASSERT_C08_PACKET_ROOT")
	}
	raw, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var m c08PacketManifest
	if err := c08Canonical(raw, &m); err != nil {
		t.Fatalf("ASSERT_C08_PACKET_CANONICAL: %v", err)
	}
	wantCeiling := []string{"NO_BROADER_MATRIX_CLOSURE", "NO_C07_AUTHORITY", "NO_C07_RECERTIFICATION", "NO_LIVE_SERVER_OR_RUNTIME_PROOF", "NO_MANAGED_SESSION_IDENTITY", "NO_PRODUCTION_AUTHORITY"}
	wantCommand := []string{"go", "test", "./sessionruntime", "-run", "^(TestC08PrivateP2AttemptStartsAtStartupPreflight|TestC08PrivateP2StopRetiresAttemptEpoch|TestC08PrivateP2RestartCreatesDistinctEpoch|TestC08PrivateP2FailedRestartAdmitsNoEpoch|TestC08PortableP2Packet)$", "-count=1"}
	wantNames := []string{"TestC08PrivateP2AttemptStartsAtStartupPreflight", "TestC08PrivateP2StopRetiresAttemptEpoch", "TestC08PrivateP2RestartCreatesDistinctEpoch", "TestC08PrivateP2FailedRestartAdmitsNoEpoch", "TestC08PortableP2Packet"}
	wantPassed := []string{"TestC08PrivateP2AttemptStartsAtStartupPreflight", "TestC08PrivateP2AttemptStartsAtStartupPreflight/before", "TestC08PrivateP2AttemptStartsAtStartupPreflight/equality", "TestC08PrivateP2AttemptStartsAtStartupPreflight/plus-one", "TestC08PrivateP2StopRetiresAttemptEpoch", "TestC08PrivateP2RestartCreatesDistinctEpoch", "TestC08PrivateP2FailedRestartAdmitsNoEpoch", "TestC08PortableP2Packet"}
	wantAssertions := []string{"ASSERT_C08_PREFLIGHT_BOUNDARY", "ASSERT_C08_STOP_RETIRES_EPOCH", "ASSERT_C08_RESTART_DISTINCT_EPOCH", "ASSERT_C08_RESTART_STALE_GENERATION", "ASSERT_C08_FAILED_RESTART_NO_EPOCH", "ASSERT_C08_PACKET_SOURCE_IDENTITY"}
	if m.SchemaVersion != "lsp-trace.private.adr0011-c08-p2-packet.v1" || m.Authority != "C08_GENERATION_BOUND_STARTUP_ATTEMPT_EPOCH" || m.Revision.Role != "PRE_C08_IMPLEMENTATION_BASELINE" || m.Revision.Commit != "a7b6d156f3e74437dbb57ef9e0c9f81a2807d997" || m.Revision.Tree != "f57db515bd5dd5b2463c24b34599b977fa9cdb52" || !reflect.DeepEqual(m.ClaimCeiling, wantCeiling) || !reflect.DeepEqual(m.Command, wantCommand) || !reflect.DeepEqual(m.SelectedNames, wantNames) || m.SelectedCount != 5 || !m.ExpectedResult.ExitSuccess || m.ExpectedResult.PassedCount != 8 || m.ExpectedResult.FailedCount != 0 || !reflect.DeepEqual(m.ExpectedResult.PassedNames, wantPassed) || !reflect.DeepEqual(m.ExpectedResult.Assertions, wantAssertions) {
		t.Fatal("ASSERT_C08_PACKET_BINDING")
	}
	if len(m.Sources) != len(c08PacketSources) {
		t.Fatal("ASSERT_C08_PACKET_SOURCE_DENOMINATOR")
	}
	root, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	for i, source := range m.Sources {
		if source.Path != c08PacketSources[i] || filepath.IsAbs(source.Path) || filepath.Clean(source.Path) != source.Path || strings.Contains(source.Path, "\\") || strings.HasPrefix(source.Path, "../") {
			t.Fatal("ASSERT_C08_PACKET_SOURCE_DENOMINATOR")
		}
		n, d, e := c08Identity(filepath.Join(root, filepath.FromSlash(source.Path)))
		if e != nil || n != source.ByteLength || d != source.SHA256 {
			t.Fatalf("ASSERT_C08_PACKET_SOURCE_IDENTITY path=%s", source.Path)
		}
	}
	for _, name := range []string{"manifest.json", "README.md"} {
		if _, _, err := c08Identity(filepath.Join(dir, name)); err != nil {
			t.Fatalf("ASSERT_C08_PACKET_FILE path=%s", name)
		}
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 2 {
		t.Fatal("ASSERT_C08_PACKET_FILE_SET")
	}
}
