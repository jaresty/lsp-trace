package source

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveTarget(t *testing.T) {
	workspace := t.TempDir()
	file := filepath.Join(workspace, "src", "main.go")
	if err := os.MkdirAll(filepath.Dir(file), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, []byte("package main\n"), 0600); err != nil {
		t.Fatal(err)
	}

	workspaceURI, targetURI, resolved, err := ResolveTarget(workspace, filepath.Join("src", "main.go"))
	if err != nil {
		t.Fatal(err)
	}
	if resolved != file || !strings.HasPrefix(workspaceURI, "file://") || !strings.HasSuffix(targetURI, "/src/main.go") {
		t.Fatalf("ResolveTarget = %q, %q, %q", workspaceURI, targetURI, resolved)
	}

	_, _, absolute, err := ResolveTarget(workspace, file)
	if err != nil || absolute != file {
		t.Fatalf("absolute ResolveTarget = %q, %v", absolute, err)
	}
}

func TestResolveTargetRejectsInvalidBoundaries(t *testing.T) {
	workspace := t.TempDir()
	outside := filepath.Join(t.TempDir(), "outside.go")
	if err := os.WriteFile(outside, []byte("package outside\n"), 0600); err != nil {
		t.Fatal(err)
	}
	dirTarget := filepath.Join(workspace, "directory")
	if err := os.Mkdir(dirTarget, 0700); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name      string
		workspace string
		target    string
		want      string
	}{
		{"workspace file", outside, outside, "workspace is not a directory"},
		{"outside", workspace, outside, "outside workspace"},
		{"missing", workspace, "missing.go", "no such file"},
		{"directory target", workspace, "directory", "target is not a regular file"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, _, err := ResolveTarget(tt.workspace, tt.target)
			if err == nil || !strings.Contains(strings.ToLower(err.Error()), tt.want) {
				t.Fatalf("ResolveTarget error = %v, want %q", err, tt.want)
			}
		})
	}
}
