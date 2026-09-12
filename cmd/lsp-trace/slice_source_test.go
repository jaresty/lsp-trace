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
