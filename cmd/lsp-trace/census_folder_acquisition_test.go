//go:build darwin

package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"lsp-trace/sessionruntime"
)

func TestCensusFolderAcquisitionPreparationBinding(t *testing.T) {
	workspace, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	store, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(store, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(workspace, "a.go"), []byte("package fixture\n"), 0600); err != nil {
		t.Fatal(err)
	}
	o := censusCLIOptions{FolderInvocation: true, Workspace: workspace, PublicationRoot: store, Sources: []string{"."}, Timeout: 10 * time.Second}
	p, err := prepareCensusFolder(o, time.Now().Add(10*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, r := range p.records {
		if r.Name == "a.go" {
			found = true
		}
	}
	if !found {
		t.Fatal("ASSERT_FOLDER_PREPARATION_TRANSFER: file missing")
	}
	p.workspace = filepath.Join(workspace, "foreign")
	starts := 0
	cfg := censusCoreConfig{folderPreparation: p, folderStarter: func() (sessionruntime.Starter, error) { starts++; return nil, nil }}
	var diagnostics bytes.Buffer
	if code := runCensusFolderAcquisition(o, cfg, &diagnostics); code != 1 || starts != 0 {
		t.Fatal("ASSERT_FOLDER_PREPARATION_BINDING: foreign preparation executed", code, starts)
	}
	if _, err := os.Stat(filepath.Join(store, "folder-acquisition")); !os.IsNotExist(err) {
		t.Fatal("ASSERT_FOLDER_PREPARATION_BINDING: effects before binding", err)
	}
	t.Log("ASSERT_FOLDER_PREPARATION_TRANSFER: PASS; ASSERT_FOLDER_PREPARATION_BINDING: PASS")
}

func TestCensusFolderAcquisitionPartitionBoundBeforeEffects(t *testing.T) {
	workspace := t.TempDir()
	store, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(store, 0700); err != nil {
		t.Fatal(err)
	}
	for n := 0; n < 17; n++ {
		if err := os.WriteFile(filepath.Join(workspace, fmt.Sprintf("source-%02d.go", n)), []byte("package fixture\n"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	deps := productionCensusRunnerDependencies()
	deps.lookPath = func(string) (string, error) { return "/controlled/unstarted", nil }
	starts := 0
	deps.newStarter = func() (sessionruntime.Starter, error) { starts++; return nil, nil }
	var out, diagnostics bytes.Buffer
	code := runCensusWithDependencies([]string{workspace, "--server", "controlled", "--publication-root", store}, &out, &diagnostics, deps)
	if code != 1 || starts != 0 || out.Len() != 0 {
		t.Fatal("ASSERT_FOLDER_PARTITION_BOUND: effect or success", code, starts, out.String())
	}
	d := checkOperationBytes(t, diagnostics.Bytes())
	if d.Operation != "start" || d.Action != "STOP" || d.Retry || d.Recovery != nil {
		t.Fatal("ASSERT_FOLDER_PARTITION_BOUND: wrong refusal", diagnostics.String())
	}
	t.Log("ASSERT_FOLDER_PARTITION_BOUND: PASS")
}
