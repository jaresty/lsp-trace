//go:build darwin

package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
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

// selectCensusLegacyAcquisition selects the explicitly declared compatibility
// route. These fixtures stop before semantic execution; the declaration is not
// evidence of a caller, model execution, or a completed inventory. Ordinary
// owner-start behavior is covered by TestCensusOrdinaryOwnerStartNoAcquisition.
func selectCensusLegacyAcquisition(t *testing.T) {
	t.Helper()
	t.Setenv(censusCallerFDEnv, "")
	t.Setenv(censusDurableCallerEnv, "")
	declaration, err := defaultCensusDeclaration()
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(declaration)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv(censusDurableCallerEnv, string(raw))
}

func TestCensusFolderAcquisitionPartitionBoundBeforeEffects(t *testing.T) {
	selectCensusLegacyAcquisition(t)
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
	// Operation diagnostics intentionally hide raw causes. Keep the original
	// assertions above, then observe the existing machine diagnostic on the same
	// explicit compatibility route. Only the diagnostic writer changes here;
	// resolution, preparation and every production acquisition guard remain live.
	nativeCatalog := deps.runNativeCatalog
	catalogCalls := 0
	deps.runNativeCatalog = func(o censusCLIOptions, cfg censusCoreConfig, stdout, stderr io.Writer) int {
		catalogCalls++
		// The earlier depth guard shares the bound sentinel; exclude that cause.
		if !o.FolderInvocation || o.DownDepth > 1 || o.UpDepth > 1 {
			t.Fatal("ASSERT_FOLDER_PARTITION_REASON: wrong route or depth bound")
		}
		folder, ok := stderr.(censusFolderDiagnosticWriter)
		if !ok {
			t.Fatal("ASSERT_FOLDER_PARTITION_REASON: missing folder diagnostic writer")
		}
		operation, ok := folder.Writer.(censusOperationDiagnosticWriter)
		if !ok {
			t.Fatal("ASSERT_FOLDER_PARTITION_REASON: missing operation diagnostic writer")
		}
		folder.Writer = operation.Writer
		o.Machine = true
		return nativeCatalog(o, cfg, stdout, folder)
	}
	isPartitionBound := func(raw []byte) bool {
		var diagnostic censusCLIDiagnostic
		if err := json.Unmarshal(raw, &diagnostic); err != nil {
			return false
		}
		return diagnostic.SchemaVersion == censusDiagnosticSchemaVersion &&
			diagnostic.Status == "FAILED" && diagnostic.Stage == censusStageDiscovery &&
			diagnostic.Code == censusCodeDiscoveryFailed &&
			strings.SplitN(diagnostic.Detail, "\n", 2)[0] == errNativeCatalogBoundReached.Error()
	}
	out.Reset()
	diagnostics.Reset()
	code = runCensusWithDependencies([]string{workspace, "--server", "controlled", "--publication-root", store}, &out, &diagnostics, deps)
	if code != 1 || starts != 0 || out.Len() != 0 || catalogCalls != 1 || !isPartitionBound(diagnostics.Bytes()) {
		t.Fatalf("ASSERT_FOLDER_PARTITION_REASON: code=%d starts=%d catalog=%d out=%s diagnostic=%s", code, starts, catalogCalls, out.String(), diagnostics.String())
	}
	t.Logf("ASSERT_FOLDER_PARTITION_REASON: %s", diagnostics.String())

	// An earlier configuration refusal must not masquerade as the partition
	// bound. This control cannot reach acquisition: resolution fails first.
	deps.resolveFolder = func(o censusCLIOptions) (censusCLIOptions, error) {
		return o, errors.New("controlled earlier configuration refusal")
	}
	catalogCalls = 0
	out.Reset()
	diagnostics.Reset()
	code = runCensusWithDependencies([]string{workspace, "--server", "controlled", "--publication-root", store}, &out, &diagnostics, deps)
	control := checkOperationBytes(t, diagnostics.Bytes())
	if code != 1 || starts != 0 || out.Len() != 0 || catalogCalls != 0 || control.Operation != "start" || string(control.Reason) != "CONFIG_UNCLASSIFIED" || isPartitionBound(diagnostics.Bytes()) {
		t.Fatalf("ASSERT_FOLDER_PARTITION_REASON_CONTROL: code=%d starts=%d catalog=%d out=%s diagnostic=%s", code, starts, catalogCalls, out.String(), diagnostics.String())
	}
	t.Log("ASSERT_FOLDER_PARTITION_REASON_CONTROL: PASS earlier configuration refusal rejected")
	t.Log("ASSERT_FOLDER_PARTITION_BOUND: PASS")
}
