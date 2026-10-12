package sessionruntime

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
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
}
