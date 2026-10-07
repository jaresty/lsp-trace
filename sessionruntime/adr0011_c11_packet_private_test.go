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

type c11PacketSource struct {
	Path       string `json:"path"`
	ByteLength int64  `json:"byte_length"`
	SHA256     string `json:"sha256"`
}

type c11PacketRevision struct {
	Commit string `json:"commit"`
	Tree   string `json:"tree"`
	Role   string `json:"role"`
}

type c11PacketExpected struct {
	ExitSuccess bool     `json:"exit_success"`
	PassedCount int      `json:"passed_count"`
	FailedCount int      `json:"failed_count"`
	PassedNames []string `json:"passed_names"`
	FailedNames []string `json:"failed_names"`
	Assertions  []string `json:"assertions"`
}

type c11PacketManifest struct {
	SchemaVersion  string            `json:"schema_version"`
	Authority      string            `json:"authority"`
	ClaimCeiling   []string          `json:"claim_ceiling"`
	Revision       c11PacketRevision `json:"revision"`
	Sources        []c11PacketSource `json:"sources"`
	Command        []string          `json:"command"`
	SelectedNames  []string          `json:"selected_names"`
	SelectedCount  int               `json:"selected_count"`
	ExpectedResult c11PacketExpected `json:"expected_result"`
}

var c11PacketSources = []string{
	"internal/lspwire/successor_ingress.go",
	"internal/lspwire/successor_ingress_p2_test.go",
	"sessionruntime/adr0011_c11_integrated_private_test.go",
	"sessionruntime/adr0011_c11_packet_private_test.go",
	"sessionruntime/adr0011_full_p1_private_test.go",
	"sessionruntime/b4_definition_private.go",
	"sessionruntime/sessionruntime.go",
}

func c11Identity(path string) (int64, string, error) {
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

func c11Canonical(raw []byte, out any) error {
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

func TestC11PortableDefinitionResultTokenPacket(t *testing.T) {
	requirePacketAuthority(t, packetAuthorityConfig{
		Row: "C11", SelectorPath: "testdata/adr0011-c11-p2-selector-v2.json",
		SelectorSchema:  "lsp-trace.private.adr0011-c11-definition-result-token-selector.v2",
		CurrentManifest: "adr0011-c11-p2-v2/manifest.json", CurrentManifestSchema: "lsp-trace.private.adr0011-c11-definition-result-token-packet.v2",
		HistoricalManifest: "adr0011-c11-p2-v1/manifest.json", HistoricalManifestHash: "sha256:2baa38ff1d7e69ecf5ac4fc0ffaf2555cd4ffae96d12f7393cdd99f5b7c9b775",
		ClaimCeiling:   []string{"NO_BROADER_MATRIX_CLOSURE", "NO_C09_C10_AUTHORITY_OR_RECERTIFICATION", "NO_C12_AUTHORITY_OR_CLOSURE", "NO_C16_C18_RECERTIFICATION", "NO_LIVE_SERVER_OR_RUNTIME_PROOF", "NO_MANAGED_SESSION_IDENTITY", "NO_PRODUCTION_AUTHORITY"},
		ActiveVerifier: "sessionruntime/adr0011_c11_packet_private_test.go",
	})
	return
	const dir = "testdata/adr0011-c11-p2-v1"
	info, err := os.Lstat(dir)
	if err != nil {
		if os.IsNotExist(err) {
			t.Fatal("ASSERT_C11_PORTABLE_PACKET_MISSING")
		}
		t.Fatal(err)
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		t.Fatal("ASSERT_C11_PACKET_ROOT")
	}

	raw, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest c11PacketManifest
	if err := c11Canonical(raw, &manifest); err != nil {
		t.Fatalf("ASSERT_C11_PACKET_CANONICAL: %v", err)
	}

	wantCeiling := []string{"NO_BROADER_MATRIX_CLOSURE", "NO_C09_C10_AUTHORITY_OR_RECERTIFICATION", "NO_C12_AUTHORITY_OR_CLOSURE", "NO_C16_C18_RECERTIFICATION", "NO_LIVE_SERVER_OR_RUNTIME_PROOF", "NO_MANAGED_SESSION_IDENTITY", "NO_PRODUCTION_AUTHORITY"}
	wantCommand := []string{"go", "test", "./sessionruntime", "-run", "^(TestC11PrivateDefinitionExactOriginalResultTokenBoundary|TestC11PortableDefinitionResultTokenPacket)$", "-count=1"}
	wantNames := []string{"TestC11PrivateDefinitionExactOriginalResultTokenBoundary", "TestC11PortableDefinitionResultTokenPacket"}
	wantPassed := []string{"TestC11PrivateDefinitionExactOriginalResultTokenBoundary", "TestC11PrivateDefinitionExactOriginalResultTokenBoundary/size-524287", "TestC11PrivateDefinitionExactOriginalResultTokenBoundary/size-524288", "TestC11PrivateDefinitionExactOriginalResultTokenBoundary/size-524289", "TestC11PortableDefinitionResultTokenPacket"}
	wantFailed := []string{}
	wantAssertions := []string{"ASSERT_C11_RESULT_TOKEN_EXACT_SIZE", "ASSERT_C11_RESULT_TOKEN_ACCEPT", "ASSERT_C11_ACCEPTED_EXACT_RAW_RESULT", "ASSERT_C11_ACCEPTED_SELECTED_RESPONSE", "ASSERT_C11_ACCEPTED_MANAGER_RELEASE", "ASSERT_C11_ACCEPTED_CALLER_RELEASE", "ASSERT_C11_ACCEPTED_EXACT_CLEANUP", "ASSERT_C11_RESULT_TOKEN_PLUS_ONE", "ASSERT_C11_PLUS_ONE_NO_DOWNSTREAM_PUBLICATION", "ASSERT_C11_PLUS_ONE_NO_SUCCESSFUL_RESPONSE_OBSERVATION", "ASSERT_C11_PLUS_ONE_SYNCHRONOUS_CLEANUP", "ASSERT_C11_PORTABLE_PACKET_MISSING", "ASSERT_C11_PACKET_ROOT", "ASSERT_C11_PACKET_CANONICAL", "ASSERT_C11_PACKET_BINDING", "ASSERT_C11_PACKET_SOURCE_DENOMINATOR", "ASSERT_C11_PACKET_SOURCE_IDENTITY", "ASSERT_C11_PACKET_FILE", "ASSERT_C11_PACKET_FILE_SET"}
	if manifest.SchemaVersion != "lsp-trace.private.adr0011-c11-definition-result-token-packet.v1" || manifest.Authority != "C11_HERMETIC_PRIVATE_DEFINITION_RESULT_TOKEN_EVIDENCE" || manifest.Revision.Role != "PRE_C11_EVIDENCE_BASELINE" || manifest.Revision.Commit != "573972154d7fe20fea63ccec8fd73120f8520602" || manifest.Revision.Tree != "11a8fd9f38afccdcfd0bffef7db03aa39aa39969" || !reflect.DeepEqual(manifest.ClaimCeiling, wantCeiling) || !reflect.DeepEqual(manifest.Command, wantCommand) || !reflect.DeepEqual(manifest.SelectedNames, wantNames) || manifest.SelectedCount != 2 || !manifest.ExpectedResult.ExitSuccess || manifest.ExpectedResult.PassedCount != 5 || manifest.ExpectedResult.FailedCount != 0 || !reflect.DeepEqual(manifest.ExpectedResult.PassedNames, wantPassed) || !reflect.DeepEqual(manifest.ExpectedResult.FailedNames, wantFailed) || !reflect.DeepEqual(manifest.ExpectedResult.Assertions, wantAssertions) {
		t.Fatal("ASSERT_C11_PACKET_BINDING")
	}
	if len(manifest.Sources) != len(c11PacketSources) {
		t.Fatal("ASSERT_C11_PACKET_SOURCE_DENOMINATOR")
	}

	root, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	for i, source := range manifest.Sources {
		if source.Path != c11PacketSources[i] || filepath.IsAbs(source.Path) || filepath.Clean(source.Path) != source.Path || strings.Contains(source.Path, "\\") || strings.HasPrefix(source.Path, "../") || source.Path == "sessionruntime/testdata/adr0011-c11-p2-v1/manifest.json" || source.Path == "sessionruntime/testdata/adr0011-c11-p2-v1/README.md" {
			t.Fatal("ASSERT_C11_PACKET_SOURCE_DENOMINATOR")
		}
		length, digest, identityErr := c11Identity(filepath.Join(root, filepath.FromSlash(source.Path)))
		if identityErr != nil || length != source.ByteLength || digest != source.SHA256 {
			t.Fatalf("ASSERT_C11_PACKET_SOURCE_IDENTITY path=%s", source.Path)
		}
	}
	for _, name := range []string{"manifest.json", "README.md"} {
		if _, _, err := c11Identity(filepath.Join(dir, name)); err != nil {
			t.Fatalf("ASSERT_C11_PACKET_FILE path=%s", name)
		}
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 2 {
		t.Fatal("ASSERT_C11_PACKET_FILE_SET")
	}
}
