//go:build darwin || linux

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	ai "lsp-trace/internal/applicationinventory"
	"lsp-trace/sessionruntime"
)

func TestScopeJobCandidateCLIHasNoProviderEffects(t *testing.T) {
	workspace, e := filepath.EvalSymlinks(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	store, e := filepath.EvalSymlinks(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	if e = os.Chmod(store, 0700); e != nil {
		t.Fatal(e)
	}
	for i := 0; i < 160; i++ {
		if e = os.WriteFile(filepath.Join(workspace, fmt.Sprintf("%03d.go", i)), []byte("package example\n"), 0600); e != nil {
			t.Fatal(e)
		}
	}
	calls := 0
	deps := censusRunnerDependencies{
		resolveFolder: func(o censusCLIOptions) (censusCLIOptions, error) {
			calls++
			t.Fatal("legacy folder resolution reached")
			return o, nil
		},
		lookPath:   func(string) (string, error) { calls++; t.Fatal("provider lookup reached"); return "", nil },
		newStarter: func() (sessionruntime.Starter, error) { calls++; t.Fatal("native starter reached"); return nil, nil },
		newManager: func(sessionruntime.Config) (*sessionruntime.Manager, error) {
			calls++
			t.Fatal("native manager reached")
			return nil, nil
		},
		runNativeCatalog: func(censusCLIOptions, censusCoreConfig, io.Writer, io.Writer) int {
			calls++
			t.Fatal("native acquisition reached")
			return 1
		},
	}
	var out, diag bytes.Buffer
	code := runCensusWithDependencies([]string{workspace, "--scope-job-candidate", "--profile", "gopls", "--publication-root", store}, &out, &diag, deps)
	if code != 0 {
		t.Fatalf("code=%d diag=%s", code, diag.String())
	}
	var r ai.InventoryJobStatus
	if e = json.Unmarshal(out.Bytes(), &r); e != nil {
		t.Fatal(e)
	}
	if calls != 0 || r.Phase != "ReadyForAcquisition" || r.Enumeration == nil || r.Enumeration.Counts.Selected != 160 || r.Enumeration.Counts.Regular != 160 || r.Enumeration.SourceContentRead || r.Enumeration.AcquisitionImplemented || r.Enumeration.InventoryPublished {
		t.Fatalf("%+v calls=%d", r, calls)
	}
	out.Reset()
	diag.Reset()
	code = runCensusWithDependencies([]string{"job-status", r.RunID, "--publication-root", store}, &out, &diag, deps)
	if code != 0 {
		t.Fatalf("status=%d %s", code, diag.String())
	}
	var reopened ai.InventoryJobStatus
	if e = json.Unmarshal(out.Bytes(), &reopened); e != nil || reopened.Used != r.Used || reopened.Packets != r.Packets || reopened.Enumeration == nil || *reopened.Enumeration != *r.Enumeration || reopened.Head != r.Head {
		t.Fatalf("%+v %v", reopened, e)
	}
}
func TestScopeJobCandidateSyntaxAndLegacyIsolation(t *testing.T) {
	normal, e := parseCensusCLIOptions([]string{"census", "."})
	if e != nil || normal.ScopeJobCandidate || !normal.FolderInvocation {
		t.Fatalf("%+v %v", normal, e)
	}
	// Eligibility is resolved from the configured declaration, not a syntax enum.
	if _, e := parseCensusCLIOptions([]string{"census", ".", "--scope-job-candidate", "--profile", "python"}); e != nil {
		t.Fatal(e)
	}
	candidate, e := parseCensusCLIOptions([]string{"census", ".", "--scope-job-candidate"})
	if e != nil || !candidate.ScopeJobCandidate || !candidate.FolderInvocation {
		t.Fatalf("%+v %v", candidate, e)
	}
	for _, args := range [][]string{
		{".", "--scope-job-budget", "/tmp/budget.json"},
		{".", "--scope-job-candidate", "--server", "gopls"},
		{".", "--scope-job-candidate", "--timeout", "24h"},
		{".", "--scope-job-candidate", "--scope-job-candidate"},
		{"--scope-job-candidate", "--workspace", "."},
		{".", "--scope-job-candidate", "--durable-describe"},
	} {
		if _, e := parseCensusCLIOptions(append([]string{"census"}, args...)); e == nil {
			t.Fatalf("accepted %v", args)
		}
	}
}
func TestScopeJobCandidateInvalidBudgetBeforeStoreEffects(t *testing.T) {
	workspace, e := filepath.EvalSymlinks(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	store, e := filepath.EvalSymlinks(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	budget := filepath.Join(t.TempDir(), "budget.json")
	if e = os.WriteFile(budget, []byte(`{"work":0}`), 0600); e != nil {
		t.Fatal(e)
	}
	var out, diag bytes.Buffer
	if runCensusWithDependencies([]string{workspace, "--scope-job-candidate", "--scope-job-budget", budget, "--profile", "gopls", "--publication-root", store}, &out, &diag, censusRunnerDependencies{}) == 0 {
		t.Fatal("invalid budget accepted")
	}
	entries, e := os.ReadDir(store)
	if e != nil || len(entries) != 0 {
		t.Fatalf("artifacts: %v %v", entries, e)
	}
}
