package sessionruntime

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
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
	requirePacketAuthority(t, packetAuthorityConfig{
		Row: "C10", SelectorPath: "testdata/adr0011-c10-p2-selector-v2.json",
		SelectorSchema:  "lsp-trace.private.adr0011-c10-references-selector.v2",
		CurrentManifest: "adr0011-c10-p2-v2/manifest.json", CurrentManifestSchema: "lsp-trace.private.adr0011-c10-references-packet.v2",
		HistoricalManifest: "adr0011-c10-p2-v1/manifest.json", HistoricalManifestHash: "sha256:6bdbceb4c56e4fd6968a273bf3aea9485e4c37f10e378becd465d462834406c7",
		ClaimCeiling:   []string{"NO_BROADER_MATRIX_CLOSURE", "NO_C09_AUTHORITY_OR_RECERTIFICATION", "NO_C11_C12_AUTHORITY_OR_CLOSURE", "NO_C16_C18_RECERTIFICATION", "NO_LIVE_SERVER_OR_RUNTIME_PROOF", "NO_MANAGED_SESSION_IDENTITY", "NO_PRODUCTION_AUTHORITY"},
		ActiveVerifier: "sessionruntime/adr0011_c10_packet_private_test.go",
	})
}
