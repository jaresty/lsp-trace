package sessionruntime

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
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
type c07PacketSelector struct {
	SchemaVersion     string   `json:"schema_version"`
	Row               string   `json:"row"`
	Authority         string   `json:"authority"`
	ClaimCeiling      []string `json:"claim_ceiling"`
	HistoricalPackets []string `json:"historical_packets"`
	CurrentPacket     string   `json:"current_packet"`
}

func c07FileIdentity(path string) (int64, string, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return 0, "", err
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return 0, "", fmt.Errorf("not regular non-symlink")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return 0, "", err
	}
	sum := sha256.Sum256(raw)
	return int64(len(raw)), "sha256:" + hex.EncodeToString(sum[:]), nil
}
func c07CanonicalJSON(raw []byte, out any) error {
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
func c07LowerHexDigest(value string) bool {
	if len(value) != 71 || !strings.HasPrefix(value, "sha256:") || value == "sha256:"+strings.Repeat("0", 64) {
		return false
	}
	for _, r := range strings.TrimPrefix(value, "sha256:") {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f')) {
			return false
		}
	}
	return true
}

var c07V2SourcePaths = []string{
	"sessionruntime/adr0011_c07_integrated_private_test.go",
	"sessionruntime/adr0011_c07_packet_private_test.go",
	"sessionruntime/adr0011_p2_deadline.go",
	"sessionruntime/sessionruntime.go",
	"sessionruntime/testdata/adr0011-c07-p2-selector-v2.json",
}

func c07ValidateSelectedSourceDenominator(sources []c07PacketSource) error {
	if len(sources) != len(c07V2SourcePaths) {
		return fmt.Errorf("count=%d", len(sources))
	}
	for i := range sources {
		if sources[i].Path != c07V2SourcePaths[i] {
			return fmt.Errorf("source[%d]=%q", i, sources[i].Path)
		}
	}
	return nil
}

func c07RequirePacketDirectory(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("packet root is not a local non-symlink directory")
	}
	return nil
}

func TestC07SelectedPacketRootRejectsSymlink(t *testing.T) {
	parent := t.TempDir()
	target := filepath.Join(parent, "external")
	if err := os.Mkdir(target, 0o700); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(parent, "selected")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	if c07RequirePacketDirectory(link) == nil {
		t.Fatal("ASSERT_C07_SELECTED_PACKET_ROOT_SYMLINK")
	}
}

func TestC07SelectedPacketSourceDenominatorRejectsDrift(t *testing.T) {
	valid := make([]c07PacketSource, len(c07V2SourcePaths))
	for i, path := range c07V2SourcePaths {
		valid[i].Path = path
	}
	cases := map[string][]c07PacketSource{
		"empty":       {},
		"missing":     append([]c07PacketSource(nil), valid[:len(valid)-1]...),
		"extra":       append(append([]c07PacketSource(nil), valid...), c07PacketSource{Path: "sessionruntime/unexpected.go"}),
		"substituted": append([]c07PacketSource(nil), valid...),
		"reordered":   append([]c07PacketSource(nil), valid...),
		"duplicate":   append([]c07PacketSource(nil), valid...),
	}
	cases["substituted"][0].Path = "sessionruntime/substitute.go"
	cases["reordered"][0], cases["reordered"][1] = cases["reordered"][1], cases["reordered"][0]
	cases["duplicate"][1].Path = cases["duplicate"][0].Path
	if err := c07ValidateSelectedSourceDenominator(valid); err != nil {
		t.Fatalf("ASSERT_C07_SELECTED_SOURCE_DENOMINATOR_VALID: %v", err)
	}
	for name, sources := range cases {
		t.Run(name, func(t *testing.T) {
			if c07ValidateSelectedSourceDenominator(sources) == nil {
				t.Fatal("ASSERT_C07_SELECTED_SOURCE_DENOMINATOR_DRIFT")
			}
		})
	}
}

func TestC07PortableP2Packet(t *testing.T) {
	requirePacketAuthority(t, packetAuthorityConfig{
		Row: "C07", SelectorPath: "testdata/adr0011-c07-p2-selector-v3.json",
		SelectorSchema:  "lsp-trace.private.adr0011-c07-p2-selector.v3",
		CurrentManifest: "adr0011-c07-p2-v3/manifest.json", CurrentManifestSchema: "lsp-trace.private.adr0011-c07-p2-packet.v3",
		HistoricalManifest: "adr0011-c07-p2-v2/manifest.json", HistoricalManifestHash: "sha256:4dafe6d2b2e6df6106a7adecb9b043ffbe109503117e267389afbe35a838ff83",
		ClaimCeiling:   []string{"NO_BROADER_MATRIX_CLOSURE", "NO_C08_AUTHORITY", "NO_LIVE_SERVER_OR_RUNTIME_PROOF", "NO_MANAGED_SESSION_IDENTITY", "NO_PRODUCTION_AUTHORITY"},
		ActiveVerifier: "sessionruntime/adr0011_c07_packet_private_test.go",
	})
}
