//go:build darwin || linux

package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	pilot "lsp-trace/internal/censuslineagepilot"
	"lsp-trace/internal/publication"
)

// Filesystem admission only: no runner, manager, pipe provider or saved evidence.
func TestInventoryFixturePrivateFreshAdmission(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	home, err = filepath.EvalSymlinks(home)
	if err != nil {
		t.Fatal(err)
	}
	owned, err := os.MkdirTemp(home, ".census-fixture-admission-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(owned); err != nil {
			t.Error(err)
		}
	})
	t.Setenv("HOME", owned)
	t.Setenv("LSP_TRACE_RETAIN_CENSUS_TEST_EVIDENCE", "")
	base, source, config, _ := inventoryFixturePrivate(t)
	if filepath.Dir(base) != owned {
		t.Fatalf("ASSERT_FIXTURE_OWNED_PARENT: %s", base)
	}
	for _, path := range []string{base, source, filepath.Join(base, "publication")} {
		if err := pilot.ValidateDurableRoot(path); err != nil {
			t.Fatal("ASSERT_FIXTURE_DURABLE_ADMISSION", err)
		}
		r, err := publication.OpenObservedRoot(context.Background(), path)
		if err != nil {
			t.Fatal("ASSERT_FIXTURE_NOFOLLOW_ADMISSION", err)
		}
		valid, closed := r.ValidatePrivate(), r.Close()
		if valid != nil || closed != nil {
			t.Fatalf("ASSERT_FIXTURE_PRIVATE_ROOT: %v %v", valid, closed)
		}
	}
	if _, err := os.Stat(config); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(filepath.Join(owned, ".local")); !os.IsNotExist(err) {
		t.Fatal("ASSERT_NO_PERSONAL_STATE_CREATED", err)
	}
	t.Log("ASSERT_FIXTURE_FRESH_ADMISSION PASS; no execution")
}
