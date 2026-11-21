//go:build darwin || linux

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInventoryEnumerationCandidateOwnsLifecycle(t *testing.T) {
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
	for i := 0; i < 140; i++ {
		if err := os.WriteFile(filepath.Join(workspace, fmt.Sprintf("%03d.go", i)), []byte("package example\n"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	var out, diag bytes.Buffer
	if code := runCensusWithDependencies([]string{workspace, "--scope-job-candidate", "--profile", "gopls", "--publication-root", store}, &out, &diag, censusRunnerDependencies{}); code != 0 {
		t.Fatalf("code=%d diag=%s", code, &diag)
	}
	var got struct{ Schema, RunID, Phase, Status string }
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if len(got.RunID) != 64 || got.Phase != "ReadyForAcquisition" || got.Status != "READY" {
		t.Fatalf("candidate must return inventory ownership, got %s", &out)
	}
	entries, err := os.ReadDir(store)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "scope-") {
			t.Fatalf("independent scope lifecycle artifact %s", e.Name())
		}
	}
	out.Reset()
	diag.Reset()
	if code := runCensusScopeJobStatus([]string{got.RunID, "--publication-root", store}, &out, &diag); code != 0 {
		t.Fatalf("status code=%d %s", code, &diag)
	}
	var reopened struct{ RunID, Phase string }
	if err := json.Unmarshal(out.Bytes(), &reopened); err != nil {
		t.Fatal(err)
	}
	if reopened.RunID != got.RunID || reopened.Phase != got.Phase {
		t.Fatalf("different owner on reopen %s", &out)
	}
}
