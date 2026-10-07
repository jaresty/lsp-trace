package sessionruntime

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func copyB4FixtureTree(t *testing.T) string {
	t.Helper()
	source, err := b4ID1FixtureRoot()
	if err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(t.TempDir(), b4ID1FixtureDir)
	if err := filepath.WalkDir(source, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		destination := filepath.Join(target, rel)
		if entry.IsDir() {
			return os.MkdirAll(destination, 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(destination, data, 0o644)
	}); err != nil {
		t.Fatal(err)
	}
	return target
}

func TestADR0011FixtureCWDIndependent(t *testing.T) {
	before, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(before) })
	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	root, err := b4ID1FixtureRoot()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := b4ID1FixtureLoadAt(root, b4ID1Manifest); err != nil {
		t.Fatalf("source-relative fixture failed after cwd change: %v", err)
	}
}

func TestADR0011FixtureInventoryAndPins(t *testing.T) {
	root, err := b4ID1FixtureRoot()
	if err != nil {
		t.Fatal(err)
	}
	raw, err := b4ID1ReadFile(root, "manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Assets       []b4ID1Pin `json:"assets"`
		Predecessors []struct {
			Path string `json:"path"`
			SHA  string `json:"sha256"`
		} `json:"predecessors"`
	}
	if err := json.Unmarshal(raw, &manifest); err != nil {
		t.Fatal(err)
	}
	if len(manifest.Assets) != 57 {
		t.Fatalf("asset count=%d want=57", len(manifest.Assets))
	}
	authorPinned := false
	for _, asset := range manifest.Assets {
		if asset.Path == "author_id1.py" {
			authorPinned = asset.SHA == "84da41b4f44ead249cbb9386b94f9d71901f819d75aa0ef935f5754ede435d76"
		}
	}
	if !authorPinned {
		t.Fatal("frozen author asset pin missing")
	}
	wantPredecessors := map[string]string{
		"DERIVATION.md":  "6180b736af5ec7e70e366425e8e658ec48ba64e3aae40ccdb19ac6487888d069",
		"A/source.bytes": "33fe4fbf4871969c874b738e0912e1199bee4781213bf50ed99bb62a19777374",
		"oracle.json":    "d82cf78ae7a8c024d22a5eac3b31e1a3c8e5a8f6dfc9c26630f97f5296432b59",
		"A/target-a.go":  "2b61fbe6058fe0c78aeb9ba3a52a92efc4ccb9fc3e9280275b44c85f9bb15a3c",
		"A/target-b.go":  "318cb48346e30c507f390c86e5dceaaac490a02f8df0deff7f205db558266176",
		"repository:docs/qualification/originals/adr0011-generic-envelope-v4.schema.json": "df4908187030e3d3110bd3745290eb25d8a3934ffb2a85c5d47d8d22dc28f6f3",
		"repository:docs/qualification/originals/generic-lsp-definition-exact-v5.json":    "da62f5a161532fcb7c79dd93b0dca940a460c6e0c5fab83e8987de21c91b4dd3",
		"repository:docs/qualification/originals/generic-lsp-exact-transport-v4.json":     "f3e25fab98cc4b8b96b76978cf56220ccc81f4ef41455ab0401041ea281d0bc8",
	}
	if len(manifest.Predecessors) != len(wantPredecessors) {
		t.Fatalf("predecessor count=%d want=%d", len(manifest.Predecessors), len(wantPredecessors))
	}
	for _, predecessor := range manifest.Predecessors {
		if wantPredecessors[predecessor.Path] != predecessor.SHA {
			t.Fatalf("predecessor pin mismatch: %q", predecessor.Path)
		}
	}
	for path, sha := range map[string]string{
		"SPEC.md":       "8fa536964c6542b151b7a5e5bd10170ea63ee46374250087a7923c2cc8d2ebae",
		"DESIGN.md":     "9eb8dfa992d4db3f4ea9b1a8eef64c18f7fec7c8e3103904d0a3dfd85cbb96dc",
		"DERIVATION.md": "6180b736af5ec7e70e366425e8e658ec48ba64e3aae40ccdb19ac6487888d069",
		"oracle.json":   "d82cf78ae7a8c024d22a5eac3b31e1a3c8e5a8f6dfc9c26630f97f5296432b59",
	} {
		data, err := b4ID1ReadFile(root, path)
		if err != nil || b4ID1Hash(data) != sha {
			t.Fatalf("provenance pin mismatch %s: %v", path, err)
		}
	}
}

func TestADR0011FixtureMissingOrCorruptFailsWithoutFallback(t *testing.T) {
	for _, test := range []struct {
		name   string
		mutate func(string) error
	}{
		{"missing", func(root string) error { return os.Remove(filepath.Join(root, "A", "source.bytes")) }},
		{"corrupt", func(root string) error {
			return os.WriteFile(filepath.Join(root, "A", "source.bytes"), []byte("corrupt"), 0o644)
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := copyB4FixtureTree(t)
			if err := test.mutate(root); err != nil {
				t.Fatal(err)
			}
			if _, err := b4ID1FixtureLoadAt(root, b4ID1Manifest); err == nil {
				t.Fatal("mutated fixture unexpectedly loaded")
			}
		})
	}
	root := copyB4FixtureTree(t)
	if err := os.WriteFile(filepath.Join(root, "author_id1.py"), []byte("corrupt"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := b4ID1FixtureLoadAt(root, b4ID1Manifest); err == nil {
		t.Fatal("corrupt inert author asset unexpectedly loaded")
	}
}

func TestADR0011FixtureRejectsUnsafePaths(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "regular"), []byte("ok"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, "directory"), 0o755); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), "outside")
	if err := os.WriteFile(outside, []byte("outside"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "link")); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{outside, "../outside", "sub/../regular", `sub\escape`, "link", "directory"} {
		t.Run(strings.NewReplacer("/", "_", `\`, "_").Replace(path), func(t *testing.T) {
			if _, err := b4ID1ReadFile(root, path); err == nil {
				t.Fatalf("unsafe path accepted: %q", path)
			}
		})
	}
	if got, err := b4ID1ReadFile(root, "regular"); err != nil || string(got) != "ok" {
		t.Fatalf("regular confined file rejected: %q %v", got, err)
	}
}

func assertADR0011FixtureAuthorIsNeverExecutedOrImported(t *testing.T) {
	_, source, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("test source location unavailable")
	}
	packageDir := filepath.Dir(source)
	if err := filepath.WalkDir(packageDir, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if path != packageDir {
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) != ".go" || path == source {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if strings.Contains(string(data), "author_id1.py") {
			t.Fatalf("Go source references inert author tool: %s", path)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestADR0011FixtureHasNoExternalVocabulary(t *testing.T) {
	assertADR0011FixtureAuthorIsNeverExecutedOrImported(t)
	root, err := b4ID1FixtureRoot()
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := b4ID1ReadFile(root, "manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	_, source, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("test source location unavailable")
	}
	for _, path := range []string{
		filepath.Join(filepath.Dir(source), "b4_definition_private_fixture_test.go"),
		filepath.Join(filepath.Dir(source), "b4_private_selected_read_lease_v10_test.go"),
	} {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		manifest = append(manifest, data...)
	}
	for _, forbidden := range []string{
		strings.Join([]string{".pi", "evidence"}, "/"),
		strings.Join([]string{"", "Users"}, "/"),
		strings.Join([]string{"..", ".pi", "evidence"}, "/"),
		strings.Join([]string{"H", "OME"}, ""),
	} {
		if strings.Contains(string(manifest), forbidden) {
			t.Fatalf("fixture resolution metadata contains external vocabulary %q", forbidden)
		}
	}
}
