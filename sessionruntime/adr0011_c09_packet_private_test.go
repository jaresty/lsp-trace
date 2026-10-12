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

type c09PacketSource struct {
	Path       string `json:"path"`
	ByteLength int64  `json:"byte_length"`
	SHA256     string `json:"sha256"`
}
type c09PacketRevision struct {
	Commit string `json:"commit"`
	Tree   string `json:"tree"`
	Role   string `json:"role"`
}
type c09PacketExpected struct {
	ExitSuccess bool     `json:"exit_success"`
	PassedCount int      `json:"passed_count"`
	FailedCount int      `json:"failed_count"`
	PassedNames []string `json:"passed_names"`
	FailedNames []string `json:"failed_names"`
	Assertions  []string `json:"assertions"`
}
type c09PacketManifest struct {
	SchemaVersion  string            `json:"schema_version"`
	Authority      string            `json:"authority"`
	ClaimCeiling   []string          `json:"claim_ceiling"`
	Revision       c09PacketRevision `json:"revision"`
	Sources        []c09PacketSource `json:"sources"`
	Command        []string          `json:"command"`
	SelectedNames  []string          `json:"selected_names"`
	SelectedCount  int               `json:"selected_count"`
	ExpectedResult c09PacketExpected `json:"expected_result"`
}

var c09PacketSources = []string{
	"internal/lspwire/successor_ingress.go",
	"internal/lspwire/successor_ingress_p2_test.go",
	"sessionruntime/adr0011_c09_integrated_private_test.go",
	"sessionruntime/adr0011_c09_packet_private_test.go",
	"sessionruntime/adr0011_full_p1_private_test.go",
	"sessionruntime/b4_definition_private.go",
	"sessionruntime/sessionruntime.go",
}

func c09Identity(path string) (int64, string, error) {
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

func c09Canonical(raw []byte, out any) error {
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

func TestC09PortableDefinitionPacket(t *testing.T) {
	requirePacketAuthority(t, packetAuthorityConfig{
		Row: "C09", SelectorPath: "testdata/adr0011-c09-p2-selector-v2.json",
		SelectorSchema:  "lsp-trace.private.adr0011-c09-definition-selector.v2",
		CurrentManifest: "adr0011-c09-p2-v2/manifest.json", CurrentManifestSchema: "lsp-trace.private.adr0011-c09-definition-packet.v2",
		HistoricalManifest: "adr0011-c09-p2-v1/manifest.json", HistoricalManifestHash: "sha256:4a7de084c9379118c7868ce31424a003259492f88339d8012b19e48b76076790",
		ClaimCeiling:   []string{"NO_BROADER_MATRIX_CLOSURE", "NO_C10_AUTHORITY_OR_CLOSURE", "NO_C11_C12_RECERTIFICATION", "NO_C16_C18_RECERTIFICATION", "NO_LIVE_SERVER_OR_RUNTIME_PROOF", "NO_MANAGED_SESSION_IDENTITY", "NO_PRODUCTION_AUTHORITY"},
		ActiveVerifier: "sessionruntime/adr0011_c09_packet_private_test.go",
	})
	return
	const dir = "testdata/adr0011-c09-p2-v1"
	info, err := os.Lstat(dir)
	if err != nil {
		if os.IsNotExist(err) {
			t.Fatal("ASSERT_C09_PORTABLE_PACKET_MISSING")
		}
		t.Fatal(err)
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		t.Fatal("ASSERT_C09_PACKET_ROOT")
	}
	raw, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest c09PacketManifest
	if err := c09Canonical(raw, &manifest); err != nil {
		t.Fatalf("ASSERT_C09_PACKET_CANONICAL: %v", err)
	}
	wantCeiling := []string{"NO_BROADER_MATRIX_CLOSURE", "NO_C10_AUTHORITY_OR_CLOSURE", "NO_C11_C12_RECERTIFICATION", "NO_C16_C18_RECERTIFICATION", "NO_LIVE_SERVER_OR_RUNTIME_PROOF", "NO_MANAGED_SESSION_IDENTITY", "NO_PRODUCTION_AUTHORITY"}
	wantCommand := []string{"go", "test", "./sessionruntime", "-run", "^(TestC09PrivateDefinitionDecodedRemarshalBoundary|TestC09PortableDefinitionPacket)$", "-count=1"}
	wantNames := []string{"TestC09PrivateDefinitionDecodedRemarshalBoundary", "TestC09PortableDefinitionPacket"}
	wantPassed := []string{"TestC09PrivateDefinitionDecodedRemarshalBoundary", "TestC09PrivateDefinitionDecodedRemarshalBoundary/size-1048575", "TestC09PrivateDefinitionDecodedRemarshalBoundary/size-1048576", "TestC09PrivateDefinitionDecodedRemarshalBoundary/size-1048577", "TestC09PortableDefinitionPacket"}
	wantFailed := []string{}
	wantAssertions := []string{"ASSERT_C09_DECODED_REMARSHAL_EXACT_SIZE", "ASSERT_C09_DECODED_REMARSHAL_ACCEPT", "ASSERT_C09_DECODED_REMARSHAL_PLUS_ONE", "ASSERT_C09_ACCEPTED_EXACT_CLEANUP", "ASSERT_C09_PLUS_ONE_NO_ACCEPTED_OBSERVATION", "ASSERT_C09_PLUS_ONE_NO_DOWNSTREAM_PUBLICATION", "ASSERT_C09_PLUS_ONE_SYNCHRONOUS_CLEANUP", "ASSERT_C09_PACKET_SOURCE_IDENTITY", "ASSERT_C09_PORTABLE_PACKET_MISSING"}
	if manifest.SchemaVersion != "lsp-trace.private.adr0011-c09-definition-packet.v1" || manifest.Authority != "C09_HERMETIC_PRIVATE_DEFINITION_INGRESS_EVIDENCE" || manifest.Revision.Role != "PRE_C09_EVIDENCE_BASELINE" || manifest.Revision.Commit != "2074d1f3165d076ef50a68c08a7f9281880474d7" || manifest.Revision.Tree != "119eda1d6c9a4e6c366fc06e8a18ad2131230a0c" || !reflect.DeepEqual(manifest.ClaimCeiling, wantCeiling) || !reflect.DeepEqual(manifest.Command, wantCommand) || !reflect.DeepEqual(manifest.SelectedNames, wantNames) || manifest.SelectedCount != 2 || !manifest.ExpectedResult.ExitSuccess || manifest.ExpectedResult.PassedCount != 5 || manifest.ExpectedResult.FailedCount != 0 || !reflect.DeepEqual(manifest.ExpectedResult.PassedNames, wantPassed) || !reflect.DeepEqual(manifest.ExpectedResult.FailedNames, wantFailed) || !reflect.DeepEqual(manifest.ExpectedResult.Assertions, wantAssertions) {
		t.Fatal("ASSERT_C09_PACKET_BINDING")
	}
	if len(manifest.Sources) != len(c09PacketSources) {
		t.Fatal("ASSERT_C09_PACKET_SOURCE_DENOMINATOR")
	}
	root, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	for i, source := range manifest.Sources {
		if source.Path != c09PacketSources[i] || filepath.IsAbs(source.Path) || filepath.Clean(source.Path) != source.Path || strings.Contains(source.Path, "\\") || strings.HasPrefix(source.Path, "../") || source.Path == "sessionruntime/testdata/adr0011-c09-p2-v1/manifest.json" {
			t.Fatal("ASSERT_C09_PACKET_SOURCE_DENOMINATOR")
		}
		length, digest, identityErr := c09Identity(filepath.Join(root, filepath.FromSlash(source.Path)))
		if identityErr != nil || length != source.ByteLength || digest != source.SHA256 {
			t.Fatalf("ASSERT_C09_PACKET_SOURCE_IDENTITY path=%s", source.Path)
		}
	}
	for _, name := range []string{"manifest.json", "README.md"} {
		if _, _, err := c09Identity(filepath.Join(dir, name)); err != nil {
			t.Fatalf("ASSERT_C09_PACKET_FILE path=%s", name)
		}
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 2 {
		t.Fatal("ASSERT_C09_PACKET_FILE_SET")
	}
}
