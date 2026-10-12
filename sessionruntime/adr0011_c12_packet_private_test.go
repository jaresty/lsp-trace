package sessionruntime

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"lsp-trace/internal/strictjson"
)

type c12PacketSource struct {
	Path       string `json:"path"`
	ByteLength int64  `json:"byte_length"`
	SHA256     string `json:"sha256"`
}

type c12PacketRevision struct {
	Commit string `json:"commit"`
	Tree   string `json:"tree"`
	Role   string `json:"role"`
}

type c12PacketExpected struct {
	ExitSuccess bool     `json:"exit_success"`
	PassedCount int      `json:"passed_count"`
	FailedCount int      `json:"failed_count"`
	PassedNames []string `json:"passed_names"`
	FailedNames []string `json:"failed_names"`
	Assertions  []string `json:"assertions"`
}

type c12PacketManifest struct {
	SchemaVersion  string            `json:"schema_version"`
	Authority      string            `json:"authority"`
	ClaimCeiling   []string          `json:"claim_ceiling"`
	Revision       c12PacketRevision `json:"revision"`
	Sources        []c12PacketSource `json:"sources"`
	Command        []string          `json:"command"`
	SelectedNames  []string          `json:"selected_names"`
	SelectedCount  int               `json:"selected_count"`
	ExpectedResult c12PacketExpected `json:"expected_result"`
}

var c12PacketSources = []string{
	"internal/lspwire/successor_ingress.go",
	"internal/lspwire/successor_ingress_p2_test.go",
	"sessionruntime/adr0011_c12_integrated_private_test.go",
	"sessionruntime/adr0011_c12_packet_private_test.go",
	"sessionruntime/adr0011_full_p1_private_test.go",
	"sessionruntime/b4_definition_private.go",
	"sessionruntime/sessionruntime.go",
}

func c12Identity(path string) (int64, string, error) {
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

func c12Canonical(raw []byte, out any) error {
	if len(raw) == 0 || raw[len(raw)-1] != '\n' || bytes.Count(raw, []byte{'\n'}) != 1 {
		return fmt.Errorf("newline")
	}
	if err := strictjson.RejectDuplicates(raw); err != nil {
		return err
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(out); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return fmt.Errorf("trailing json")
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

func TestC12PortableReferencesResultTokenPacket(t *testing.T) {
	requirePacketAuthority(t, packetAuthorityConfig{
		Row: "C12", SelectorPath: "testdata/adr0011-c12-p2-selector-v2.json",
		SelectorSchema:  "lsp-trace.private.adr0011-c12-references-result-token-selector.v2",
		CurrentManifest: "adr0011-c12-p2-v2/manifest.json", CurrentManifestSchema: "lsp-trace.private.adr0011-c12-references-result-token-packet.v2",
		HistoricalManifest: "adr0011-c12-p2-v1/manifest.json", HistoricalManifestHash: "sha256:5c6c9b7887afe7d007b4253317724a9ebebe2bcd2699a78f784ac06dc571de10",
		ClaimCeiling:   []string{"NO_BROADER_MATRIX_CLOSURE", "NO_C09_C10_C11_AUTHORITY_OR_RECERTIFICATION", "NO_C16_C18_RECERTIFICATION", "NO_LIVE_SERVER_OR_RUNTIME_PROOF", "NO_MANAGED_SESSION_IDENTITY", "NO_PRODUCTION_AUTHORITY"},
		ActiveVerifier: "sessionruntime/adr0011_c12_packet_private_test.go",
	})
	return
	const dir = "testdata/adr0011-c12-p2-v1"
	info, err := os.Lstat(dir)
	if err != nil {
		if os.IsNotExist(err) {
			t.Fatal("ASSERT_C12_PORTABLE_PACKET_MISSING")
		}
		t.Fatal(err)
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		t.Fatal("ASSERT_C12_PACKET_ROOT")
	}

	raw, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest c12PacketManifest
	if err := c12Canonical(raw, &manifest); err != nil {
		t.Fatalf("ASSERT_C12_PACKET_CANONICAL: %v", err)
	}

	wantCeiling := []string{"NO_BROADER_MATRIX_CLOSURE", "NO_C09_C10_C11_AUTHORITY_OR_RECERTIFICATION", "NO_C16_C18_RECERTIFICATION", "NO_LIVE_SERVER_OR_RUNTIME_PROOF", "NO_MANAGED_SESSION_IDENTITY", "NO_PRODUCTION_AUTHORITY"}
	wantCommand := []string{"go", "test", "./sessionruntime", "-run", "^(TestC12PrivateReferencesExactOriginalResultTokenBoundary|TestC12PortableReferencesResultTokenPacket)$", "-count=1"}
	wantNames := []string{"TestC12PrivateReferencesExactOriginalResultTokenBoundary", "TestC12PortableReferencesResultTokenPacket"}
	wantPassed := []string{"TestC12PrivateReferencesExactOriginalResultTokenBoundary", "TestC12PrivateReferencesExactOriginalResultTokenBoundary/size-524287", "TestC12PrivateReferencesExactOriginalResultTokenBoundary/size-524288", "TestC12PrivateReferencesExactOriginalResultTokenBoundary/size-524289", "TestC12PortableReferencesResultTokenPacket"}
	wantFailed := []string{}
	wantAssertions := []string{"ASSERT_C12_RESULT_TOKEN_EXACT_SIZE", "ASSERT_C12_RESULT_TOKEN_ACCEPT", "ASSERT_C12_ACCEPTED_EXACT_RAW_RESULT", "ASSERT_C12_ACCEPTED_SELECTED_RESPONSE", "ASSERT_C12_ACCEPTED_ONE_SUCCESSFUL_RESPONSE_OBSERVATION", "ASSERT_C12_ACCEPTED_NO_PRIVATE_LEASES", "ASSERT_C12_ACCEPTED_EXACT_CLEANUP", "ASSERT_C12_RESULT_TOKEN_PLUS_ONE", "ASSERT_C12_PLUS_ONE_NO_DOWNSTREAM_PUBLICATION", "ASSERT_C12_PLUS_ONE_NO_PRIVATE_LEASES", "ASSERT_C12_PLUS_ONE_NO_SUCCESSFUL_RESPONSE_OBSERVATION", "ASSERT_C12_PLUS_ONE_SYNCHRONOUS_CLEANUP", "ASSERT_C12_PORTABLE_PACKET_MISSING", "ASSERT_C12_PACKET_ROOT", "ASSERT_C12_PACKET_CANONICAL", "ASSERT_C12_PACKET_BINDING", "ASSERT_C12_PACKET_SOURCE_DENOMINATOR", "ASSERT_C12_PACKET_SOURCE_IDENTITY", "ASSERT_C12_PACKET_FILE", "ASSERT_C12_PACKET_FILE_SET"}
	if manifest.SchemaVersion != "lsp-trace.private.adr0011-c12-references-result-token-packet.v1" || manifest.Authority != "C12_HERMETIC_PRIVATE_REFERENCES_RESULT_TOKEN_EVIDENCE" || manifest.Revision.Role != "PRE_C12_EVIDENCE_BASELINE" || manifest.Revision.Commit != "92c252a05b275e16b482ad08b414ef46f8e7001d" || manifest.Revision.Tree != "7a8960d558176801f408203a350609b42132fd76" || !reflect.DeepEqual(manifest.ClaimCeiling, wantCeiling) || !reflect.DeepEqual(manifest.Command, wantCommand) || !reflect.DeepEqual(manifest.SelectedNames, wantNames) || manifest.SelectedCount != 2 || !manifest.ExpectedResult.ExitSuccess || manifest.ExpectedResult.PassedCount != 5 || manifest.ExpectedResult.FailedCount != 0 || !reflect.DeepEqual(manifest.ExpectedResult.PassedNames, wantPassed) || !reflect.DeepEqual(manifest.ExpectedResult.FailedNames, wantFailed) || !reflect.DeepEqual(manifest.ExpectedResult.Assertions, wantAssertions) {
		t.Fatal("ASSERT_C12_PACKET_BINDING")
	}
	if len(manifest.Sources) != len(c12PacketSources) {
		t.Fatal("ASSERT_C12_PACKET_SOURCE_DENOMINATOR")
	}

	root, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	for i, source := range manifest.Sources {
		if source.Path != c12PacketSources[i] || filepath.IsAbs(source.Path) || filepath.Clean(source.Path) != source.Path || strings.Contains(source.Path, "\\") || strings.HasPrefix(source.Path, "../") || source.Path == "sessionruntime/testdata/adr0011-c12-p2-v1/manifest.json" || source.Path == "sessionruntime/testdata/adr0011-c12-p2-v1/README.md" {
			t.Fatal("ASSERT_C12_PACKET_SOURCE_DENOMINATOR")
		}
		length, digest, identityErr := c12Identity(filepath.Join(root, filepath.FromSlash(source.Path)))
		if identityErr != nil || length != source.ByteLength || digest != source.SHA256 {
			t.Fatalf("ASSERT_C12_PACKET_SOURCE_IDENTITY path=%s", source.Path)
		}
	}
	for _, name := range []string{"manifest.json", "README.md"} {
		if _, _, err := c12Identity(filepath.Join(dir, name)); err != nil {
			t.Fatalf("ASSERT_C12_PACKET_FILE path=%s", name)
		}
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 2 {
		t.Fatal("ASSERT_C12_PACKET_FILE_SET")
	}
}
