package sessionruntime

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"lsp-trace/internal/strictjson"
)

type packetAuthoritySource struct {
	Path       string `json:"path"`
	ByteLength int64  `json:"byte_length"`
	SHA256     string `json:"sha256"`
}

type packetAuthorityManifest struct {
	SchemaVersion string                  `json:"schema_version"`
	Authority     string                  `json:"authority"`
	ClaimCeiling  []string                `json:"claim_ceiling"`
	Sources       []packetAuthoritySource `json:"sources"`
}

type packetAuthoritySelector struct {
	SchemaVersion     string   `json:"schema_version"`
	Row               string   `json:"row"`
	Authority         string   `json:"authority"`
	ClaimCeiling      []string `json:"claim_ceiling"`
	HistoricalPackets []string `json:"historical_packets"`
	CurrentPacket     string   `json:"current_packet"`
}

type packetAuthorityConfig struct {
	Row                    string
	SelectorPath           string
	SelectorSchema         string
	CurrentManifest        string
	CurrentManifestSchema  string
	HistoricalManifest     string
	HistoricalManifestHash string
	ClaimCeiling           []string
	ActiveVerifier         string
}

func packetAuthorityRegular(path string, wantDir bool) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || (wantDir && !info.IsDir()) || (!wantDir && !info.Mode().IsRegular()) {
		return fmt.Errorf("not regular non-symlink path: %s", path)
	}
	return nil
}

func packetAuthoritySafeRelative(path string) bool {
	return path != "" && path == filepath.ToSlash(filepath.Clean(path)) && !filepath.IsAbs(path) && !strings.Contains(path, "\\") && path != "." && path != ".." && !strings.HasPrefix(path, "../") && !strings.Contains(path, "/../")
}

func packetAuthorityCanonical(path string, out any) ([]byte, error) {
	if err := packetAuthorityRegular(path, false); err != nil {
		return nil, err
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if len(raw) == 0 || raw[len(raw)-1] != '\n' || bytes.Count(raw, []byte{'\n'}) != 1 {
		return nil, fmt.Errorf("not canonical one-LF JSON")
	}
	if err := strictjson.RejectDuplicates(raw); err != nil {
		return nil, err
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(out); err != nil {
		return nil, err
	}
	var generic any
	if err := json.Unmarshal(raw, &generic); err != nil {
		return nil, err
	}
	canonical, err := json.Marshal(generic)
	if err != nil || !bytes.Equal(raw, append(canonical, '\n')) {
		return nil, fmt.Errorf("not canonical JSON")
	}
	return raw, nil
}

func packetAuthorityHistoricalCanonical(path string, out *packetAuthorityManifest) ([]byte, error) {
	if err := packetAuthorityRegular(path, false); err != nil {
		return nil, err
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if len(raw) == 0 || raw[len(raw)-1] != '\n' || bytes.Count(raw, []byte{'\n'}) != 1 {
		return nil, fmt.Errorf("not canonical one-LF JSON")
	}
	if err := strictjson.RejectDuplicates(raw); err != nil {
		return nil, err
	}
	var generic any
	if err := json.Unmarshal(raw, &generic); err != nil {
		return nil, err
	}
	canonical, err := json.Marshal(generic)
	if err != nil || !bytes.Equal(raw, append(canonical, '\n')) {
		return nil, fmt.Errorf("not canonical JSON")
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return nil, err
	}
	return raw, nil
}

func packetAuthorityDigest(raw []byte) string {
	sum := sha256.Sum256(raw)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func packetAuthorityValidateDenominator(got, want []packetAuthoritySource, selectorPath, _ string) error {
	if len(got) != len(want) {
		return fmt.Errorf("source count=%d want=%d", len(got), len(want))
	}
	previous := ""
	seen := make(map[string]bool, len(got))
	for i := range got {
		source := got[i]
		if !packetAuthoritySafeRelative(source.Path) || source.Path <= previous || seen[source.Path] {
			return fmt.Errorf("source[%d] unsafe, unordered, or duplicate: %q", i, source.Path)
		}
		if source.Path == selectorPath || strings.HasSuffix(source.Path, "/manifest.json") || strings.HasSuffix(source.Path, "/README.md") {
			return fmt.Errorf("source[%d] self-referential: %q", i, source.Path)
		}
		if source != want[i] {
			return fmt.Errorf("source[%d]=%+v want=%+v", i, source, want[i])
		}
		seen[source.Path] = true
		previous = source.Path
	}
	return nil
}

func packetAuthorityExactTree(root string, logical []packetAuthoritySource) error {
	if err := packetAuthorityRegular(root, true); err != nil {
		return err
	}
	want := make(map[string]packetAuthoritySource, len(logical))
	for _, source := range logical {
		want[source.Path] = source
	}
	seen := map[string]bool{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == root {
			return nil
		}
		info, err := os.Lstat(path)
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("historical symlink: %s", path)
		}
		if entry.IsDir() {
			if !info.IsDir() {
				return fmt.Errorf("historical non-directory: %s", path)
			}
			return nil
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("historical non-regular file: %s", path)
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		expected, ok := want[rel]
		if !ok {
			return fmt.Errorf("extra historical file: %s", rel)
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if int64(len(raw)) != expected.ByteLength || packetAuthorityDigest(raw) != expected.SHA256 {
			return fmt.Errorf("historical identity: %s", rel)
		}
		seen[rel] = true
		return nil
	})
	if err != nil {
		return err
	}
	if len(seen) != len(want) {
		return fmt.Errorf("historical file count=%d want=%d", len(seen), len(want))
	}
	return nil
}

func requirePacketAuthority(t *testing.T, cfg packetAuthorityConfig) {
	t.Helper()
	if err := packetAuthorityRegular(cfg.SelectorPath, false); err != nil {
		if os.IsNotExist(err) {
			t.Fatalf("ASSERT_%s_SELECTED_PACKET_MISSING selector=%s", cfg.Row, cfg.SelectorPath)
		}
		t.Fatalf("ASSERT_%s_SELECTOR_REGULAR: %v", cfg.Row, err)
	}
	var selector packetAuthoritySelector
	if _, err := packetAuthorityCanonical(cfg.SelectorPath, &selector); err != nil {
		t.Fatalf("ASSERT_%s_SELECTOR_CANONICAL: %v", cfg.Row, err)
	}
	if selector.SchemaVersion != cfg.SelectorSchema || selector.Row != cfg.Row || selector.CurrentPacket != cfg.CurrentManifest || !reflect.DeepEqual(selector.HistoricalPackets, []string{cfg.HistoricalManifest}) || !reflect.DeepEqual(selector.ClaimCeiling, cfg.ClaimCeiling) || !packetAuthoritySafeRelative(selector.CurrentPacket) || !packetAuthoritySafeRelative(selector.HistoricalPackets[0]) {
		t.Fatalf("ASSERT_%s_SELECTOR_SEMANTIC_POINTER", cfg.Row)
	}
	currentPath := filepath.Join(filepath.Dir(cfg.SelectorPath), filepath.FromSlash(selector.CurrentPacket))
	currentDir := filepath.Dir(currentPath)
	if err := packetAuthorityRegular(currentDir, true); err != nil {
		if os.IsNotExist(err) {
			t.Fatalf("ASSERT_%s_SELECTED_PACKET_MISSING packet=%s", cfg.Row, selector.CurrentPacket)
		}
		t.Fatalf("ASSERT_%s_SELECTED_PACKET_ROOT: %v", cfg.Row, err)
	}
	var current packetAuthorityManifest
	if _, err := packetAuthorityCanonical(currentPath, &current); err != nil {
		t.Fatalf("ASSERT_%s_SELECTED_MANIFEST_CANONICAL: %v", cfg.Row, err)
	}
	if current.SchemaVersion != cfg.CurrentManifestSchema || !reflect.DeepEqual(current.ClaimCeiling, cfg.ClaimCeiling) {
		t.Fatalf("ASSERT_%s_SELECTED_MANIFEST_BINDING", cfg.Row)
	}
	historicalManifestPath := filepath.Join(filepath.Dir(cfg.SelectorPath), filepath.FromSlash(cfg.HistoricalManifest))
	var historical packetAuthorityManifest
	historicalRaw, err := packetAuthorityHistoricalCanonical(historicalManifestPath, &historical)
	if err != nil || packetAuthorityDigest(historicalRaw) != cfg.HistoricalManifestHash {
		t.Fatalf("ASSERT_%s_PREDECESSOR_ARTIFACT_DIGEST", cfg.Row)
	}
	if err := packetAuthorityValidateDenominator(current.Sources, historical.Sources, filepath.ToSlash(strings.TrimPrefix(cfg.SelectorPath, "testdata/")), cfg.ActiveVerifier); err != nil {
		t.Fatalf("ASSERT_%s_HISTORICAL_LOGICAL_DENOMINATOR: %v", cfg.Row, err)
	}
	if err := packetAuthorityExactTree(filepath.Join(currentDir, "historical"), historical.Sources); err != nil {
		t.Fatalf("ASSERT_%s_HISTORICAL_SNAPSHOT: %v", cfg.Row, err)
	}
	entries, err := os.ReadDir(currentDir)
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	sort.Strings(names)
	if !reflect.DeepEqual(names, []string{"README.md", "historical", "manifest.json"}) {
		t.Fatalf("ASSERT_%s_SELECTED_PACKET_FILE_SET: %v", cfg.Row, names)
	}
}

func TestPacketAuthorityHelperRejectsAdversarialStates(t *testing.T) {
	valid := []packetAuthoritySource{{Path: "a.go", ByteLength: 1, SHA256: "sha256:a"}, {Path: "b.go", ByteLength: 1, SHA256: "sha256:b"}}
	for name, mutate := range map[string]func([]packetAuthoritySource) []packetAuthoritySource{
		"substitution": func(v []packetAuthoritySource) []packetAuthoritySource { v[0].Path = "x.go"; return v },
		"order":        func(v []packetAuthoritySource) []packetAuthoritySource { v[0], v[1] = v[1], v[0]; return v },
		"extra": func(v []packetAuthoritySource) []packetAuthoritySource {
			return append(v, packetAuthoritySource{Path: "z.go"})
		},
		"self": func(v []packetAuthoritySource) []packetAuthoritySource {
			v[0].Path = "sessionruntime/adr0011_c07_packet_private_test.go"
			return v
		},
	} {
		t.Run(name, func(t *testing.T) {
			got := mutate(append([]packetAuthoritySource(nil), valid...))
			if packetAuthorityValidateDenominator(got, valid, "selector.json", "sessionruntime/adr0011_c07_packet_private_test.go") == nil {
				t.Fatalf("ASSERT_PACKET_AUTHORITY_HELPER_%s", strings.ToUpper(name))
			}
		})
	}
	t.Run("symlink", func(t *testing.T) {
		root := t.TempDir()
		target := filepath.Join(root, "target")
		if err := os.WriteFile(target, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(target, filepath.Join(root, "a.go")); err != nil {
			t.Fatal(err)
		}
		if packetAuthorityExactTree(root, []packetAuthoritySource{{Path: "a.go", ByteLength: 1, SHA256: packetAuthorityDigest([]byte("x"))}}) == nil {
			t.Fatal("ASSERT_PACKET_AUTHORITY_HELPER_SYMLINK")
		}
	})
	t.Run("frozen-not-live", func(t *testing.T) {
		base := t.TempDir()
		historical := filepath.Join(base, "packet", "historical")
		live := filepath.Join(base, "live", "source.go")
		frozen := filepath.Join(historical, "source.go")
		if err := os.MkdirAll(filepath.Dir(live), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Dir(frozen), 0o700); err != nil {
			t.Fatal(err)
		}
		original := []byte("frozen\n")
		if err := os.WriteFile(live, original, 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(frozen, original, 0o600); err != nil {
			t.Fatal(err)
		}
		logical := []packetAuthoritySource{{Path: "source.go", ByteLength: int64(len(original)), SHA256: packetAuthorityDigest(original)}}
		if err := os.WriteFile(live, []byte("mutated live\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := packetAuthorityExactTree(historical, logical); err != nil {
			t.Fatalf("ASSERT_PACKET_AUTHORITY_HELPER_LIVE_MUTATION_IRRELEVANT: %v", err)
		}
		if err := os.WriteFile(frozen, []byte("mutated frozen\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		if packetAuthorityExactTree(historical, logical) == nil {
			t.Fatal("ASSERT_PACKET_AUTHORITY_HELPER_FROZEN_MUTATION")
		}
	})
}
