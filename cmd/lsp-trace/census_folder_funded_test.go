//go:build darwin

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestCensusFolderFundedRecursiveBatches(t *testing.T) {
	workspace := t.TempDir()
	for _, nestedEntries := range []int{2, 9} {
		t.Run(fmt.Sprint(nestedEntries), func(t *testing.T) {
			assertCensusFolderFundedBatches(t, workspace, nestedEntries)
		})
	}
}

func assertCensusFolderFundedBatches(t *testing.T, workspace string, nestedEntries int) {
	t.Helper()
	store, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(store, 0700); err != nil {
		t.Fatal(err)
	}
	nested := filepath.Join(workspace, "nested")
	if err := os.MkdirAll(nested, 0700); err != nil {
		t.Fatal(err)
	}
	paths := []string{"root.go"}
	// Only the nested contents change between invocations; crossing the
	// production read count of eight must cause an extra verified batch.
	for i := 0; i < nestedEntries; i++ {
		paths = append(paths, fmt.Sprintf("nested/%d.go", i))
	}
	for _, path := range paths {
		if err := os.WriteFile(filepath.Join(workspace, path), []byte("package fixture\n"), 0600); err != nil {
			t.Fatal(err)
		}
	}

	var out, diagnostics bytes.Buffer
	// Use the folder route and a command that cannot perform acquisition.
	code := runCensus([]string{"--server", "/usr/bin/false", "--publication-root", store, workspace}, &out, &diagnostics)
	runs, err := filepath.Glob(filepath.Join(store, "run-*"))
	if err != nil || len(runs) != 1 {
		t.Fatalf("folder route setup: runs=%v err=%v code=%d stderr=%q", runs, err, code, diagnostics.String())
	}
	heads, err := filepath.Glob(filepath.Join(runs[0], "inventory-successor", "heads", "private", "v2", "*.json"))
	if err != nil {
		t.Fatal(err)
	}
	var observed int
	for _, path := range heads {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var head struct {
			Event       string `json:"event"`
			Disposition string `json:"disposition"`
		}
		if err := json.Unmarshal(raw, &head); err != nil {
			t.Fatal(err)
		}
		if head.Event == "OBSERVE" && head.Disposition == "OBSERVED" {
			observed++
		}
	}
	if observed < 2 {
		t.Fatalf("ASSERT_FOLDER_FUNDED_COMPLETIONS: got %d OBSERVED heads, want at least two", observed)
	}
	t.Log("ASSERT_FOLDER_FUNDED_COMPLETIONS: PASS")
	want := 2
	if nestedEntries > 8 {
		want++
	}
	if observed != want {
		t.Fatalf("ASSERT_FOLDER_NESTED_SCHEDULE: got %d completions, want %d for %d nested entries", observed, want, nestedEntries)
	}
	// Invocation consent now admits a separate acquisition lane after these
	// exact preparation completions. /usr/bin/false still cannot supply evidence
	// or produce inventory; preparation credit is not execution credit.
	if code != 1 || out.Len() != 0 || diagnostics.Len() == 0 {
		t.Fatalf("ASSERT_FOLDER_FAILED_PROVIDER_NO_INVENTORY: code=%d stdout=%q stderr=%q", code, out.String(), diagnostics.String())
	}
	t.Log("ASSERT_FOLDER_NESTED_SCHEDULE: PASS; ASSERT_FOLDER_FAILED_PROVIDER_NO_INVENTORY: PASS")
}
