//go:build darwin || linux

package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCensusStructuredDefaultDiagnostics(t *testing.T) {
	for _, flags := range [][]string{nil, {"--machine"}, {"--machine=false"}} {
		o, err := parseCensusCLIOptions(append([]string{"census", "folder"}, flags...))
		if err != nil || !o.Machine || !o.FolderInvocation {
			t.Fatalf("normal machine compatibility: %+v %v", o, err)
		}
	}
	legacy, err := parseCensusCLIOptions([]string{"census", "--workspace", "work", "--server", "fake", "--publication-root", "/private"})
	if err != nil || legacy.Machine || legacy.FolderInvocation {
		t.Fatalf("legacy machine behavior changed: %+v %v", legacy, err)
	}
	for _, args := range [][]string{{"--unknown"}, {"/does/not/exist"}, {"durable", "task"}} {
		var out, diagnostic bytes.Buffer
		if runCensus(args, &out, &diagnostic) == 0 || !json.Valid(diagnostic.Bytes()) {
			t.Fatalf("structured default diagnostic: %q %s", args, diagnostic.String())
		}
	}
	var out, diagnostic bytes.Buffer
	runCensus([]string{"--help"}, &out, &diagnostic)
	if strings.Contains(out.String(), "--durable-describe") {
		t.Fatal("legacy describe advertised")
	}
}

func TestCensusStorePreflightBeforeEffects(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "config"))
	t.Setenv(censusDurableStoreEnv, filepath.Join(home, "missing-store"))
	workspace := t.TempDir()
	if err := os.WriteFile(filepath.Join(workspace, "go.mod"), []byte("module fixture\n"), 0600); err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	root, _ = filepath.EvalSymlinks(root)
	if err := os.Chmod(root, 0700); err != nil {
		t.Fatal(err)
	}
	effects := 0
	deps := productionCensusRunnerDependencies()
	deps.lookPath = func(string) (string, error) { effects++; return "/unused", nil }
	var out, diagnostic bytes.Buffer
	if runCensusWithDependencies([]string{workspace}, &out, &diagnostic, deps) == 0 || effects != 0 {
		t.Fatalf("unsafe store before effects: %d %s", effects, diagnostic.String())
	}
	// Explicit selection wins over an unusable compatibility environment override.
	_, err := resolveCensusFolderInvocation(censusCLIOptions{Workspace: workspace, Sources: []string{"."}, DurableInventory: true, PublicationRoot: root})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "durable")); err != nil {
		t.Fatalf("selected store missing: %v", err)
	}
}
