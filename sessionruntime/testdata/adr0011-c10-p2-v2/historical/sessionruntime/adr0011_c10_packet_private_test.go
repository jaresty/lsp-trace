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

type c10PacketSource struct {
	Path       string `json:"path"`
	ByteLength int64  `json:"byte_length"`
	SHA256     string `json:"sha256"`
}

type c10PacketRevision struct {
	Commit string `json:"commit"`
	Tree   string `json:"tree"`
	Role   string `json:"role"`
}

type c10PacketExpected struct {
	ExitSuccess bool     `json:"exit_success"`
	PassedCount int      `json:"passed_count"`
	FailedCount int      `json:"failed_count"`
	PassedNames []string `json:"passed_names"`
	FailedNames []string `json:"failed_names"`
	Assertions  []string `json:"assertions"`
}

type c10PacketManifest struct {
	SchemaVersion  string            `json:"schema_version"`
	Authority      string            `json:"authority"`
	ClaimCeiling   []string          `json:"claim_ceiling"`
	Revision       c10PacketRevision `json:"revision"`
	Sources        []c10PacketSource `json:"sources"`
	Command        []string          `json:"command"`
	SelectedNames  []string          `json:"selected_names"`
	SelectedCount  int               `json:"selected_count"`
	ExpectedResult c10PacketExpected `json:"expected_result"`
}

var c10PacketSources = []string{
	"internal/lspwire/successor_ingress.go",
	"internal/lspwire/successor_ingress_p2_test.go",
	"sessionruntime/adr0011_c10_integrated_private_test.go",
	"sessionruntime/adr0011_c10_packet_private_test.go",
	"sessionruntime/adr0011_full_p1_private_test.go",
	"sessionruntime/b4_definition_private.go",
	"sessionruntime/sessionruntime.go",
}

func c10Identity(path string) (int64, string, error) {
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

func c10Canonical(raw []byte, out any) error {
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

func TestC10PortableReferencesPacket(t *testing.T) {
	const dir = "testdata/adr0011-c10-p2-v1"
	info, err := os.Lstat(dir)
	if err != nil {
		if os.IsNotExist(err) {
			t.Fatal("ASSERT_C10_PORTABLE_PACKET_MISSING")
		}
		t.Fatal(err)
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		t.Fatal("ASSERT_C10_PACKET_ROOT")
	}

	raw, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest c10PacketManifest
	if err := c10Canonical(raw, &manifest); err != nil {
		t.Fatalf("ASSERT_C10_PACKET_CANONICAL: %v", err)
	}

	wantCeiling := []string{"NO_BROADER_MATRIX_CLOSURE", "NO_C09_AUTHORITY_OR_RECERTIFICATION", "NO_C11_C12_AUTHORITY_OR_CLOSURE", "NO_C16_C18_RECERTIFICATION", "NO_LIVE_SERVER_OR_RUNTIME_PROOF", "NO_MANAGED_SESSION_IDENTITY", "NO_PRODUCTION_AUTHORITY"}
	wantCommand := []string{"go", "test", "./sessionruntime", "-run", "^(TestC10PrivateReferencesDecodedRemarshalBoundary|TestC10PortableReferencesPacket)$", "-count=1"}
	wantNames := []string{"TestC10PrivateReferencesDecodedRemarshalBoundary", "TestC10PortableReferencesPacket"}
	wantPassed := []string{"TestC10PrivateReferencesDecodedRemarshalBoundary", "TestC10PrivateReferencesDecodedRemarshalBoundary/size-1572863", "TestC10PrivateReferencesDecodedRemarshalBoundary/size-1572864", "TestC10PrivateReferencesDecodedRemarshalBoundary/size-1572865", "TestC10PortableReferencesPacket"}
	wantFailed := []string{}
	wantAssertions := []string{"ASSERT_C10_DECODED_REMARSHAL_FIXTURE", "ASSERT_C10_DECODED_REMARSHAL_EXACT_SIZE", "ASSERT_C10_DECODED_REMARSHAL_ACCEPT", "ASSERT_C10_ACCEPTED_EXACT_RESULT", "ASSERT_C10_ACCEPTED_EXACT_CLEANUP", "ASSERT_C10_DECODED_REMARSHAL_PLUS_ONE", "ASSERT_C10_PLUS_ONE_NO_ACCEPTED_OBSERVATION", "ASSERT_C10_PLUS_ONE_NO_DOWNSTREAM_PUBLICATION", "ASSERT_C10_PLUS_ONE_SYNCHRONOUS_CLEANUP", "ASSERT_C10_PORTABLE_PACKET_MISSING", "ASSERT_C10_PACKET_ROOT", "ASSERT_C10_PACKET_CANONICAL", "ASSERT_C10_PACKET_BINDING", "ASSERT_C10_PACKET_SOURCE_DENOMINATOR", "ASSERT_C10_PACKET_SOURCE_IDENTITY", "ASSERT_C10_PACKET_FILE", "ASSERT_C10_PACKET_FILE_SET"}
	if manifest.SchemaVersion != "lsp-trace.private.adr0011-c10-references-packet.v1" || manifest.Authority != "C10_HERMETIC_PRIVATE_REFERENCES_INGRESS_EVIDENCE" || manifest.Revision.Role != "PRE_C10_EVIDENCE_BASELINE" || manifest.Revision.Commit != "269152b802c5f291274026f554f8f3966e15f5b3" || manifest.Revision.Tree != "e261e110edbca4d28b8c39fe79138364f348f194" || !reflect.DeepEqual(manifest.ClaimCeiling, wantCeiling) || !reflect.DeepEqual(manifest.Command, wantCommand) || !reflect.DeepEqual(manifest.SelectedNames, wantNames) || manifest.SelectedCount != 2 || !manifest.ExpectedResult.ExitSuccess || manifest.ExpectedResult.PassedCount != 5 || manifest.ExpectedResult.FailedCount != 0 || !reflect.DeepEqual(manifest.ExpectedResult.PassedNames, wantPassed) || !reflect.DeepEqual(manifest.ExpectedResult.FailedNames, wantFailed) || !reflect.DeepEqual(manifest.ExpectedResult.Assertions, wantAssertions) {
		t.Fatal("ASSERT_C10_PACKET_BINDING")
	}
	if len(manifest.Sources) != len(c10PacketSources) {
		t.Fatal("ASSERT_C10_PACKET_SOURCE_DENOMINATOR")
	}

	root, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	for i, source := range manifest.Sources {
		if source.Path != c10PacketSources[i] || filepath.IsAbs(source.Path) || filepath.Clean(source.Path) != source.Path || strings.Contains(source.Path, "\\") || strings.HasPrefix(source.Path, "../") || source.Path == "sessionruntime/testdata/adr0011-c10-p2-v1/manifest.json" || source.Path == "sessionruntime/testdata/adr0011-c10-p2-v1/README.md" {
			t.Fatal("ASSERT_C10_PACKET_SOURCE_DENOMINATOR")
		}
		length, digest, identityErr := c10Identity(filepath.Join(root, filepath.FromSlash(source.Path)))
		if identityErr != nil || length != source.ByteLength || digest != source.SHA256 {
			t.Fatalf("ASSERT_C10_PACKET_SOURCE_IDENTITY path=%s", source.Path)
		}
	}
	for _, name := range []string{"manifest.json", "README.md"} {
		if _, _, err := c10Identity(filepath.Join(dir, name)); err != nil {
			t.Fatalf("ASSERT_C10_PACKET_FILE path=%s", name)
		}
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 2 {
		t.Fatal("ASSERT_C10_PACKET_FILE_SET")
	}
}
