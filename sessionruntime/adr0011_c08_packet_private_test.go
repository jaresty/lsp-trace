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
	requirePacketAuthority(t, packetAuthorityConfig{
		Row: "C08", SelectorPath: "testdata/adr0011-c08-p2-selector-v2.json",
		SelectorSchema:  "lsp-trace.private.adr0011-c08-p2-selector.v2",
		CurrentManifest: "adr0011-c08-p2-v2/manifest.json", CurrentManifestSchema: "lsp-trace.private.adr0011-c08-p2-packet.v2",
		HistoricalManifest: "adr0011-c08-p2-v1/manifest.json", HistoricalManifestHash: "sha256:d8a1458e601501873697ae39255c45eb79203546540cbac9bdb04dafc8cd5d2f",
		ClaimCeiling:   []string{"NO_BROADER_MATRIX_CLOSURE", "NO_C07_AUTHORITY", "NO_C07_RECERTIFICATION", "NO_LIVE_SERVER_OR_RUNTIME_PROOF", "NO_MANAGED_SESSION_IDENTITY", "NO_PRODUCTION_AUTHORITY"},
		ActiveVerifier: "sessionruntime/adr0011_c08_packet_private_test.go",
	})
}
