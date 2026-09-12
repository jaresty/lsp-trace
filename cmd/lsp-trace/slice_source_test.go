package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestResolveSliceSourcesCanonicalizesMixedInputs(t *testing.T) {
	workspace := t.TempDir()
	mustWrite := func(path string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("package p\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	mustWrite(filepath.Join(workspace, "z.go"))
	mustWrite(filepath.Join(workspace, "nested", "a.go"))

	cfg := sliceConfig{workspace: workspace, fromFile: "nested", fromFiles: stringsFlag{"z.go", "nested", "nested/a.go", "z.go"}}
	_, _, sources, err := resolveSliceSources(cfg)
	if err != nil {
		t.Fatal(err)
	}
	got := make([]string, len(sources))
	for i, source := range sources {
		got[i] = source.path
	}
	want := []string{filepath.Join("nested", "a.go"), "z.go"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ASSERT_SLICE_SOURCE_CANONICAL_ORDER_AND_DEDUP: got=%q want=%q", got, want)
	}
}

func TestResolveSliceSourcesAppliesGitignoreFiltersWithExcludePrecedence(t *testing.T) {
	workspace := t.TempDir()
	for _, name := range []string{"src/a.go", "src/generated/a.go", "src/a_test.go", "docs/a.go", "src/z.ts"} {
		path := filepath.Join(workspace, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(name), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	cfg := sliceConfig{
		workspace: workspace, fromFile: ".", fromFiles: stringsFlag{"src", "src/a.go", "."},
		includes: stringsFlag{"src/**/*.go", "docs/*.go"},
		excludes: stringsFlag{"**/*_test.go", "src/generated/**", "docs/**"},
	}
	_, _, sources, accounting, err := resolveSliceSourcesWithAccounting(cfg)
	if err != nil {
		t.Fatal(err)
	}
	got := make([]string, len(sources))
	for i := range sources {
		got[i] = filepath.ToSlash(sources[i].path)
	}
	if want := []string{"src/a.go"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("ASSERT_DISCOVERY_FILTER_OVERLAP_ORDER_EXCLUDE_WINS: got=%q want=%q", got, want)
	}
	if accounting.FilesEnumerated != 5 || accounting.FilesSelected != 1 || accounting.FilesExcluded != 4 {
		t.Fatalf("ASSERT_DISCOVERY_FILE_ACCOUNTING_EXACT: %#v", accounting)
	}
}

func TestDiscoveryPatternsAreWorkspaceRelativeGitignoreStyle(t *testing.T) {
	cases := []struct {
		pattern, path string
		want          bool
	}{
		{"*.go", "main.go", true}, {"*.go", "src/main.go", true},
		{"/src/*.go", "src/main.go", true}, {"/src/*.go", "nested/src/main.go", false},
		{"src/**/main.go", "src/main.go", true}, {"src/**/main.go", "src/nested/main.go", true},
		{"src/?.go", "src/a.go", true}, {"src/?.go", "src/ab.go", false},
		{"src/[ab].go", "src/a.go", true}, {"src/[ab].go", "src/z.go", false},
	}
	for _, tc := range cases {
		if got := matchDiscoveryPattern(tc.pattern, tc.path); got != tc.want {
			t.Errorf("ASSERT_DISCOVERY_GITIGNORE_PATTERN[%s,%s]: got=%t want=%t", tc.pattern, tc.path, got, tc.want)
		}
	}
}

func TestAutomaticDiscoveryAccountingIsExactBoundedAndPathFree(t *testing.T) {
	got := formatDiscoveryAccounting(discoveryAccounting{
		FilesEnumerated: 5, FilesSelected: 2, FilesExcluded: 3,
		SymbolsEnumerated: 9, SymbolsSelected: 9, SymbolsUnsupported: 4,
		SymbolsPreparationFailed: 2, SymbolsPrepared: 3,
	})
	for _, want := range []string{"files_enumerated=5", "files_selected=2", "files_excluded=3", "symbols_enumerated=9", "symbols_selected=9", "symbols_excluded=0", "symbols_unsupported=4", "symbols_preparation_failed=2", "symbols_prepared=3", "does not claim endpoint or source completeness"} {
		if !strings.Contains(got, want) {
			t.Fatalf("ASSERT_DISCOVERY_ACCOUNTING_EXACT_BOUNDED: missing=%q got=%q", want, got)
		}
	}
	if strings.Contains(got, workspaceSeparatorForTest()) || len(got) > 512 {
		t.Fatalf("ASSERT_DISCOVERY_ACCOUNTING_PRIVACY_BOUNDED: %q", got)
	}
}

func workspaceSeparatorForTest() string { return string(filepath.Separator) + "private" }

func TestResolveSliceSourcesRejectsEscapeAndSkipsNestedSymlink(t *testing.T) {
	workspace := t.TempDir()
	outside := t.TempDir()
	outsideFile := filepath.Join(outside, "outside.go")
	if err := os.WriteFile(outsideFile, []byte("package outside\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := resolveSliceSources(sliceConfig{workspace: workspace, fromFile: "../outside.go", fromFiles: stringsFlag{"../outside.go"}}); err == nil || !strings.Contains(err.Error(), "remain within workspace") {
		t.Fatalf("ASSERT_SLICE_SOURCE_WORKSPACE_ESCAPE_REJECTED: err=%v", err)
	}

	inside := filepath.Join(workspace, "inside.go")
	if err := os.WriteFile(inside, []byte("package inside\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outsideFile, filepath.Join(workspace, "linked.go")); err != nil {
		t.Fatal(err)
	}
	_, _, sources, err := resolveSliceSources(sliceConfig{workspace: workspace, fromFile: ".", fromFiles: stringsFlag{"."}})
	if err != nil {
		t.Fatal(err)
	}
	if len(sources) != 1 || sources[0].path != "inside.go" {
		t.Fatalf("ASSERT_SLICE_SOURCE_NESTED_SYMLINK_EXCLUDED: sources=%v", sources)
	}
}
