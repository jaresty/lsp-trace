package adr0011acquisition

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// Golden produced independently with Python:
// hashlib.sha256(b'ADR0011_SYNTHETIC_SOURCE_SET_V1\0' +
// (len(p)).to_bytes(8,'big') + p + (len(b)).to_bytes(8,'big') + hashlib.sha256(b).digest()).hexdigest()
// where p=b'internal/adr0011acquisition/a.go', b=b'package a\n'.
func syntheticSourceTestDigest(entries []SyntheticSourceEntry) [32]byte {
	h := sha256.New()
	_, _ = h.Write([]byte(syntheticSourceHeader))
	var word [8]byte
	for _, entry := range entries {
		binary.BigEndian.PutUint64(word[:], uint64(len(entry.Path)))
		_, _ = h.Write(word[:])
		_, _ = h.Write([]byte(entry.Path))
		binary.BigEndian.PutUint64(word[:], entry.ByteLength)
		_, _ = h.Write(word[:])
		_, _ = h.Write(entry.SHA256[:])
	}
	var digest [32]byte
	copy(digest[:], h.Sum(nil))
	return digest
}

func TestSyntheticSourcePin(t *testing.T) {
	root := t.TempDir()
	for dir := range syntheticSourceDirectories {
		if err := os.MkdirAll(filepath.Join(root, filepath.FromSlash(dir)), 0700); err != nil {
			t.Fatal(err)
		}
	}
	name := "internal/adr0011acquisition/a.go"
	filename := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(filename), 0700); err != nil {
		t.Fatal(err)
	}
	body := []byte("package a\n")
	if err := os.WriteFile(filename, body, 0600); err != nil {
		t.Fatal(err)
	}
	entry := SyntheticSourceEntry{Path: name, ByteLength: uint64(len(body)), SHA256: sha256.Sum256(body)}
	// Independent byte encoding (not the verifier's helper or digest routine).
	packet := append([]byte("ADR0011_SYNTHETIC_SOURCE_SET_V1\x00"), make([]byte, 8)...)
	packet[len(packet)-1] = byte(len(name))
	packet = append(packet, name...)
	packet = append(packet, make([]byte, 8)...)
	packet[len(packet)-1] = byte(len(body))
	packet = append(packet, entry.SHA256[:]...)
	golden := sha256.Sum256(packet)
	if err := verifySyntheticSourcePinAt(root, []SyntheticSourceEntry{entry}, golden); err != nil {
		t.Fatalf("golden: %v", err)
	}
	if got := hex.EncodeToString(golden[:]); got != "632b59cd0308c38701b6f51fa2f3fa6e51268a29e8baf68606339491128a0918" {
		t.Fatalf("independent golden mismatch: %s", got)
	}
	assertReject := func(label string, entries []SyntheticSourceEntry, digest [32]byte) {
		t.Helper()
		err := verifySyntheticSourcePinAt(root, entries, digest)
		if err == nil || err != errSyntheticSourcePin || strings.Contains(err.Error(), "package a") {
			t.Fatalf("%s: unsafe result %v", label, err)
		}
	}
	wrong := golden
	wrong[0] ^= 1
	assertReject("aggregate", []SyntheticSourceEntry{entry}, wrong)
	omitted := filepath.Join(root, "internal/adr0011acquisition/omitted.go")
	if err := os.WriteFile(omitted, body, 0600); err != nil {
		t.Fatal(err)
	}
	assertReject("omitted", []SyntheticSourceEntry{entry}, golden)
	if err := os.Remove(omitted); err != nil {
		t.Fatal(err)
	}
	altered := entry
	altered.SHA256[0] ^= 1
	assertReject("hash", []SyntheticSourceEntry{altered}, golden)
	altered = entry
	altered.ByteLength--
	assertReject("length", []SyntheticSourceEntry{altered}, golden)
	altered = entry
	altered.Path = "internal/adr0011acquisition/../a.go"
	assertReject("traversal", []SyntheticSourceEntry{altered}, golden)
	altered = entry
	altered.Path = "internal/adr0011acquisition/missing.go"
	assertReject("absent", []SyntheticSourceEntry{altered}, golden)
	assertReject("duplicate", []SyntheticSourceEntry{entry, entry}, golden)
	second := entry
	second.Path = "internal/adr0011acquisition/0.go"
	assertReject("unsorted", []SyntheticSourceEntry{entry, second}, golden)
	if err := os.WriteFile(filename, []byte("package b\n"), 0600); err != nil {
		t.Fatal(err)
	}
	assertReject("changed file", []SyntheticSourceEntry{entry}, golden)
	replacement := filepath.Join(root, "replacement.tmp")
	if err := os.WriteFile(replacement, []byte("package c\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(replacement, filename); err != nil {
		t.Fatal(err)
	}
	assertReject("swapped inode", []SyntheticSourceEntry{entry}, golden)
	if err := os.WriteFile(filename, body[:len(body)-1], 0600); err != nil {
		t.Fatal(err)
	}
	assertReject("truncated", []SyntheticSourceEntry{entry}, golden)
	if err := os.Remove(filename); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, "outside.go"), filename); err != nil {
		t.Fatal(err)
	}
	assertReject("symlink", []SyntheticSourceEntry{entry}, golden)
	if err := os.Remove(filename); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filename, bytes.Repeat([]byte("x"), syntheticSourceFileLimit+1), 0600); err != nil {
		t.Fatal(err)
	}
	assertReject("overlimit", []SyntheticSourceEntry{entry}, golden)
	tooLarge := entry
	tooLarge.ByteLength = syntheticSourceFileLimit + 1
	assertReject("manifest overlimit", []SyntheticSourceEntry{tooLarge}, golden)
}

func TestSyntheticSourcePinAdditionalProductionPaths(t *testing.T) {
	expectedAdditionalGoPaths := []string{
		"cmd/lsp-trace/census_assembly.go",
		"cmd/lsp-trace/census_core.go",
		"cmd/lsp-trace/census_runner.go",
		"cmd/lsp-trace/census_chunking.go",
		"internal/censuschunking/chunking.go",
		"cmd/lsp-trace-mcp/census_chunking.go",
		"cmd/lsp-trace-mcp/census_completion.go",
		"cmd/lsp-trace-mcp/census_mcp_binding.go",
		"cmd/lsp-trace-mcp/census_runtime.go",
		"cmd/lsp-trace-mcp/main.go",
		"cmd/lsp-trace/census_chunking_adapters_private.go",
		"internal/censuschunkingruntime/adapters.go",
		"internal/censuschunkingruntime/translate.go",
		"internal/mcpcontract/census_future.go",
		"internal/operation/types.go",
		"cmd/lsp-trace/acquisition_v2.go",
		"cmd/lsp-trace/census_discovery_adapter.go",
		"internal/censusacquisition/discovery.go",
		"internal/censusacquisition/request_budget.go",
		"cmd/lsp-trace/census_command.go",
		"internal/censuspreflight/result.go",
		"internal/censusrequest/receipt_v2.go",
		"internal/schema/schema.go",
		"internal/source/discovery.go",
	}
	expectedAdditionalSchemaPaths := []string{
		"internal/mcpcontract/testdata/schemas/envelope-census-chunk-terminal-result.v1.schema.json",
		"internal/schema/schemas/lsp-trace.census-enumeration-diagnostic.v1.schema.json",
		"internal/schema/schemas/lsp-trace.census-enumeration-preflight-result.v1.schema.json",
		"internal/schema/schemas/lsp-trace.census-request-receipt.v2.schema.json",
	}
	if strings.Join(syntheticSourceAdditionalGoPaths, "\n") != strings.Join(expectedAdditionalGoPaths, "\n") {
		t.Fatalf("additional Go paths differ:\n got: %q\nwant: %q", syntheticSourceAdditionalGoPaths, expectedAdditionalGoPaths)
	}
	if strings.Join(syntheticSourceAdditionalSchemaPaths, "\n") != strings.Join(expectedAdditionalSchemaPaths, "\n") {
		t.Fatalf("additional schema paths differ:\n got: %q\nwant: %q", syntheticSourceAdditionalSchemaPaths, expectedAdditionalSchemaPaths)
	}

	root := t.TempDir()
	for dir := range syntheticSourceDirectories {
		if err := os.MkdirAll(filepath.Join(root, filepath.FromSlash(dir)), 0700); err != nil {
			t.Fatal(err)
		}
	}
	legacyName := "internal/adr0011acquisition/a.go"
	legacyBody := []byte("package adr0011acquisition\n")
	legacyFile := filepath.Join(root, filepath.FromSlash(legacyName))
	if err := os.WriteFile(legacyFile, legacyBody, 0600); err != nil {
		t.Fatal(err)
	}
	legacyEntry := SyntheticSourceEntry{Path: legacyName, ByteLength: uint64(len(legacyBody)), SHA256: sha256.Sum256(legacyBody)}
	entries := []SyntheticSourceEntry{legacyEntry}
	allSuffixPaths := append(append([]string(nil), syntheticSourceAdditionalGoPaths...), syntheticSourceAdditionalSchemaPaths...)
	for i, name := range allSuffixPaths {
		body := []byte("package production\n// " + string(rune('a'+i)) + "\n")
		if strings.HasSuffix(name, ".json") {
			body = []byte("{}\n")
		}
		filename := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(filename), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filename, body, 0600); err != nil {
			t.Fatal(err)
		}
		entries = append(entries, SyntheticSourceEntry{Path: name, ByteLength: uint64(len(body)), SHA256: sha256.Sum256(body)})
	}
	oldFiveEnd := 1 + syntheticSourceHistoricalAdditionalGoCount
	if err := verifySyntheticSourcePinAt(root, entries, syntheticSourceTestDigest(entries)); err != nil {
		t.Fatalf("complete successor: %v", err)
	}
	if err := verifySyntheticSourcePinAt(root, entries[:1], syntheticSourceTestDigest(entries[:1])); err != nil {
		t.Fatalf("directory-only profile: %v", err)
	}
	if err := verifySyntheticSourcePinAt(root, entries[:oldFiveEnd], syntheticSourceTestDigest(entries[:oldFiveEnd])); err != nil {
		t.Fatalf("historical five-path profile: %v", err)
	}
	assertReject := func(label string, candidate []SyntheticSourceEntry) {
		t.Helper()
		if err := verifySyntheticSourcePinAt(root, candidate, syntheticSourceTestDigest(candidate)); err != errSyntheticSourcePin {
			t.Fatalf("%s: got %v", label, err)
		}
	}
	for omitted := oldFiveEnd; omitted < len(entries); omitted++ {
		candidate := append([]SyntheticSourceEntry(nil), entries[:omitted]...)
		candidate = append(candidate, entries[omitted+1:]...)
		assertReject("omitted successor "+entries[omitted].Path, candidate)
	}
	for omitted := 1; omitted < oldFiveEnd; omitted++ {
		candidate := append([]SyntheticSourceEntry(nil), entries[:omitted]...)
		candidate = append(candidate, entries[omitted+1:oldFiveEnd]...)
		assertReject("omitted historical "+entries[omitted].Path, candidate)
	}
	partial := append([]SyntheticSourceEntry(nil), entries[:oldFiveEnd+1]...)
	assertReject("partial successor", partial)
	mixed := append([]SyntheticSourceEntry(nil), entries[:oldFiveEnd]...)
	mixed = append(mixed, entries[oldFiveEnd+1:]...)
	assertReject("mixed successor", mixed)
	for i, name := range expectedAdditionalGoPaths {
		nearName := append([]SyntheticSourceEntry(nil), entries...)
		nearName[1+i].Path = name + ".renamed"
		assertReject("renamed go "+name, nearName)
	}
	for i, name := range expectedAdditionalSchemaPaths {
		nearName := append([]SyntheticSourceEntry(nil), entries...)
		nearName[1+len(expectedAdditionalGoPaths)+i].Path = strings.TrimSuffix(name, ".schema.json") + ".sibling.schema.json"
		assertReject("renamed schema "+name, nearName)
	}
	goNamespaceSubstitutions := map[string]string{
		"cmd/lsp-trace/census_command.go":      "cmd/lsp-trace/census_commands.go",
		"internal/censuspreflight/result.go":   "internal/censuspreflight/results.go",
		"internal/censusrequest/receipt_v2.go": "internal/censusrequest/receipt.go",
		"internal/schema/schema.go":            "internal/schema/schemas.go",
		"internal/source/discovery.go":         "internal/source/discover.go",
	}
	for name, substitute := range goNamespaceSubstitutions {
		candidate := append([]SyntheticSourceEntry(nil), entries...)
		for i := range candidate {
			if candidate[i].Path == name {
				candidate[i].Path = substitute
				break
			}
		}
		assertReject("namespace substitution "+name, candidate)
	}
	arbitraryJSON := append([]SyntheticSourceEntry(nil), entries...)
	arbitraryJSON[len(arbitraryJSON)-1].Path = "internal/mcpcontract/testdata/schemas/arbitrary.json"
	assertReject("arbitrary json", arbitraryJSON)
	testSource := append([]SyntheticSourceEntry(nil), entries...)
	testSource[oldFiveEnd].Path = "cmd/lsp-trace/acquisition_v2_test.go"
	assertReject("test source", testSource)
	docSource := append([]SyntheticSourceEntry(nil), entries...)
	docSource[oldFiveEnd].Path = "docs/qualification/source-pin.go"
	assertReject("docs source", docSource)
	siblingJSON := append([]SyntheticSourceEntry(nil), entries...)
	siblingJSON[len(siblingJSON)-1].Path = "internal/mcpcontract/testdata/schemas/envelope-census-chunk-result.v1.schema.json"
	assertReject("sibling json", siblingJSON)
	extensionSubstitution := append([]SyntheticSourceEntry(nil), entries...)
	extensionSubstitution[oldFiveEnd].Path = strings.TrimSuffix(extensionSubstitution[oldFiveEnd].Path, ".go") + ".json"
	assertReject("extension substitution", extensionSubstitution)
	traversal := append([]SyntheticSourceEntry(nil), entries...)
	traversal[len(traversal)-1].Path = "internal/mcpcontract/testdata/schemas/../schemas/envelope-census-chunk-terminal-result.v1.schema.json"
	assertReject("schema traversal", traversal)
	duplicate := append([]SyntheticSourceEntry(nil), entries...)
	duplicate[len(duplicate)-1] = duplicate[len(duplicate)-2]
	assertReject("successor duplicate", duplicate)
	outOfOrder := append([]SyntheticSourceEntry(nil), entries...)
	outOfOrder[oldFiveEnd], outOfOrder[oldFiveEnd+1] = outOfOrder[oldFiveEnd+1], outOfOrder[oldFiveEnd]
	assertReject("successor order", outOfOrder)

	symlinkPath := filepath.Join(root, filepath.FromSlash(syntheticSourceAdditionalSchemaPaths[0]))
	if err := os.Remove(symlinkPath); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, filepath.FromSlash(syntheticSourceAdditionalGoPaths[len(syntheticSourceAdditionalGoPaths)-1])), symlinkPath); err != nil {
		t.Fatal(err)
	}
	assertReject("schema symlink", entries)
}

func TestSyntheticSourcePinProducerCandidate(t *testing.T) {
	const manifestHash = "6054423fc7d6c991d7343b036ebec90b0d74f007801459ad58f96a0e666f68b6"
	const aggregate = "sha256:6f475e674272e18d6b5fee5a2a23803225788e999ae1de84c050fb12c491e339"
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("producer candidate source location unavailable")
	}
	manifestPath := filepath.Join(filepath.Dir(file), "..", "..", "docs", "qualification", "adr0011-synthetic-source-pin.6054423fc7d6c991.manifest.json")
	b, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	manifestSum := sha256.Sum256(b)
	if got := hex.EncodeToString(manifestSum[:]); got != manifestHash {
		t.Fatalf("producer manifest hash: %s", got)
	}
	var manifest struct {
		SchemaVersion string `json:"schema_version"`
		Digest        string `json:"digest"`
		Files         []struct {
			Path       string `json:"path"`
			ByteLength uint64 `json:"byte_length"`
			SHA256     string `json:"sha256"`
		} `json:"files"`
	}
	if err := json.Unmarshal(b, &manifest); err != nil || manifest.SchemaVersion != "ADR0011_SYNTHETIC_SOURCE_PIN_V1" || manifest.Digest != aggregate || len(manifest.Files) != 148 {
		t.Fatalf("producer manifest identity: files=%d digest=%q schema=%q err=%v", len(manifest.Files), manifest.Digest, manifest.SchemaVersion, err)
	}
	entries := make([]SyntheticSourceEntry, 0, len(manifest.Files))
	for _, f := range manifest.Files {
		raw, err := hex.DecodeString(strings.TrimPrefix(f.SHA256, "sha256:"))
		if err != nil || len(raw) != sha256.Size || !strings.HasPrefix(f.SHA256, "sha256:") {
			t.Fatalf("producer entry hash %q: %v", f.Path, err)
		}
		var sum [sha256.Size]byte
		copy(sum[:], raw)
		entries = append(entries, SyntheticSourceEntry{Path: f.Path, ByteLength: f.ByteLength, SHA256: sum})
	}
	rawAggregate, err := hex.DecodeString(strings.TrimPrefix(aggregate, "sha256:"))
	if err != nil || len(rawAggregate) != sha256.Size {
		t.Fatal("producer aggregate encoding")
	}
	var expected [sha256.Size]byte
	copy(expected[:], rawAggregate)
	if err := VerifySyntheticSourcePin(entries, expected); err != nil {
		t.Fatalf("producer candidate: %v", err)
	}
}

func TestSyntheticSourcePinCeilingsUnchanged(t *testing.T) {
	if syntheticSourceFileLimit != 2<<20 || syntheticSourceTotalLimit != 16<<20 || syntheticSourceCountLimit != 256 {
		t.Fatalf("source-pin ceilings changed: file=%d total=%d count=%d", syntheticSourceFileLimit, syntheticSourceTotalLimit, syntheticSourceCountLimit)
	}
}
