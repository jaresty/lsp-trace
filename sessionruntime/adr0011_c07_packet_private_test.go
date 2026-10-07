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
	return
	const v1 = "testdata/adr0011-c07-p2-v1"
	for path, want := range map[string]string{
		filepath.Join(v1, "manifest.json"): "sha256:800159a657e75a1f94d070867ca4bfed677ebd9b970b6dbc3eb45ad9ffefcf17",
		filepath.Join(v1, "README.md"):     "sha256:4ee322a3eb793de70345967591eb7459be3626ec9dbe3abc4d5f81ca0524556e",
	} {
		_, got, err := c07FileIdentity(path)
		if err != nil || got != want {
			t.Fatalf("ASSERT_C07_V1_HISTORICAL_ARTIFACT path=%s got=%s err=%v", path, got, err)
		}
	}
	v1raw, err := os.ReadFile(filepath.Join(v1, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var v1m c07PacketManifest
	if err := c07CanonicalJSON(v1raw, &v1m); err != nil {
		t.Fatalf("ASSERT_C07_V1_HISTORICAL_MANIFEST: %v", err)
	}
	for _, source := range v1m.Sources {
		p := filepath.Join(v1, "historical", filepath.FromSlash(source.Path))
		n, d, e := c07FileIdentity(p)
		if e != nil || n != source.ByteLength || d != source.SHA256 {
			t.Fatalf("ASSERT_C07_V1_HISTORICAL_SOURCE path=%s", source.Path)
		}
	}

	selectorPath := "testdata/adr0011-c07-p2-selector-v2.json"
	selectorRaw, err := os.ReadFile(selectorPath)
	if err != nil {
		t.Fatalf("ASSERT_C07_SELECTOR: %v", err)
	}
	var selector c07PacketSelector
	if err := c07CanonicalJSON(selectorRaw, &selector); err != nil {
		t.Fatalf("ASSERT_C07_SELECTOR: %v", err)
	}
	wantSelectorCeiling := []string{"NO_BROADER_MATRIX_CLOSURE", "NO_C08_AUTHORITY", "NO_LIVE_SERVER_OR_RUNTIME_PROOF", "NO_MANAGED_SESSION_IDENTITY", "NO_PRODUCTION_AUTHORITY"}
	if selector.SchemaVersion != "lsp-trace.private.adr0011-c07-p2-selector.v2" || selector.Row != "C07" || selector.Authority != "HERMETIC_GO_SOURCE_AND_TEST_EVIDENCE_ONLY" || !reflect.DeepEqual(selector.ClaimCeiling, wantSelectorCeiling) || !reflect.DeepEqual(selector.HistoricalPackets, []string{"adr0011-c07-p2-v1/manifest.json"}) || selector.CurrentPacket != "adr0011-c07-p2-v2/manifest.json" || filepath.Clean(selector.CurrentPacket) != selector.CurrentPacket || filepath.IsAbs(selector.CurrentPacket) || strings.Contains(selector.CurrentPacket, "\\") || strings.HasPrefix(selector.CurrentPacket, "../") {
		t.Fatal("ASSERT_C07_SELECTOR_IDENTITY")
	}
	entries, err := os.ReadDir("testdata")
	if err != nil {
		t.Fatal(err)
	}
	selectors := 0
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "adr0011-c07-p2-selector-") {
			selectors++
		}
	}
	if selectors != 1 {
		t.Fatal("ASSERT_C07_SELECTOR_UNIQUE")
	}
	currentPath := filepath.Join("testdata", filepath.FromSlash(selector.CurrentPacket))
	currentDir := filepath.Dir(currentPath)
	if err := c07RequirePacketDirectory(currentDir); err != nil {
		if os.IsNotExist(err) {
			t.Fatal("ASSERT_C07_SELECTED_PACKET_MISSING")
		}
		t.Fatalf("ASSERT_C07_SELECTED_PACKET_ROOT: %v", err)
	}
	if _, _, err := c07FileIdentity(currentPath); err != nil {
		t.Fatalf("ASSERT_C07_SELECTED_MANIFEST_REGULAR: %v", err)
	}
	currentRaw, err := os.ReadFile(currentPath)
	if err != nil {
		t.Fatal(err)
	}
	var current c07PacketManifest
	if err := c07CanonicalJSON(currentRaw, &current); err != nil {
		t.Fatalf("ASSERT_C07_SELECTED_MANIFEST_CANONICAL: %v", err)
	}
	wantCommand := []string{"go", "test", "./sessionruntime", "-run", "^(TestC07PrivateP2IntegratedPreBlockBoundary|TestC07PortableP2Packet)$", "-count=1"}
	wantNames := []string{"TestC07PrivateP2IntegratedPreBlockBoundary", "TestC07PortableP2Packet"}
	wantPassed := []string{"TestC07PrivateP2IntegratedPreBlockBoundary", "TestC07PrivateP2IntegratedPreBlockBoundary/before", "TestC07PrivateP2IntegratedPreBlockBoundary/equality", "TestC07PrivateP2IntegratedPreBlockBoundary/plus-one", "TestC07PortableP2Packet"}
	wantAssertions := []string{"ASSERT_C07_CLOCK_CALL_COUNT", "ASSERT_C07_INTEGRATED_BOUNDARY", "ASSERT_C07_PREBLOCK_ZERO_MUTATION", "ASSERT_C07_SELECTED_MANIFEST_IDENTITY", "ASSERT_C07_SELECTED_SOURCE_IDENTITY"}
	wantCeiling := []string{"NO_BROADER_MATRIX_CLOSURE", "NO_C08_AUTHORITY", "NO_LIVE_SERVER_OR_RUNTIME_PROOF", "NO_MANAGED_SESSION_IDENTITY", "NO_PRODUCTION_AUTHORITY"}
	if current.SchemaVersion != "lsp-trace.private.adr0011-c07-p2-packet.v2" || current.Authority != "C07_SELECTED_CURRENT_HERMETIC_GO_SOURCE_EVIDENCE" || current.Revision.Role != "PRE_C08_IMPLEMENTATION_BASELINE" || current.Revision.Commit != "a7b6d156f3e74437dbb57ef9e0c9f81a2807d997" || current.Revision.Tree != "f57db515bd5dd5b2463c24b34599b977fa9cdb52" || !reflect.DeepEqual(current.ClaimCeiling, wantCeiling) || !reflect.DeepEqual(current.Command, wantCommand) || !reflect.DeepEqual(current.SelectedNames, wantNames) || current.SelectedCount != 2 || !current.ExpectedResult.ExitSuccess || current.ExpectedResult.PassedCount != 5 || current.ExpectedResult.FailedCount != 0 || !reflect.DeepEqual(current.ExpectedResult.PassedNames, wantPassed) || !reflect.DeepEqual(current.ExpectedResult.Assertions, wantAssertions) {
		t.Fatal("ASSERT_C07_SELECTED_EXECUTION_BINDING")
	}
	if err := c07ValidateSelectedSourceDenominator(current.Sources); err != nil {
		t.Fatalf("ASSERT_C07_SELECTED_SOURCE_DENOMINATOR: %v", err)
	}
	root, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	paths := make([]string, 0, len(current.Sources))
	previous := ""
	for _, source := range current.Sources {
		if source.Path == "" || source.Path <= previous || filepath.IsAbs(source.Path) || filepath.Clean(source.Path) != source.Path || strings.Contains(source.Path, "\\") || strings.HasPrefix(source.Path, "../") {
			t.Fatal("ASSERT_C07_SELECTED_SOURCE_PATH")
		}
		previous = source.Path
		n, d, e := c07FileIdentity(filepath.Join(root, filepath.FromSlash(source.Path)))
		if e != nil || n != source.ByteLength || d != source.SHA256 {
			t.Fatalf("ASSERT_C07_SELECTED_SOURCE_IDENTITY path=%s", source.Path)
		}
		paths = append(paths, source.Path)
	}
	if !sort.StringsAreSorted(paths) {
		t.Fatal("ASSERT_C07_SELECTED_SOURCE_ORDER")
	}
	for _, name := range []string{"manifest.json", "README.md"} {
		if _, _, err := c07FileIdentity(filepath.Join(currentDir, name)); err != nil {
			t.Fatalf("ASSERT_C07_SELECTED_PACKET_FILE path=%s", name)
		}
	}
	packetEntries, err := os.ReadDir(currentDir)
	if err != nil || len(packetEntries) != 2 {
		t.Fatal("ASSERT_C07_SELECTED_PACKET_FILE_SET")
	}
}
