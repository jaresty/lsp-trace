//go:build linux

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// This witness calls only the funded metadata preparer, never a runner,
// provider, acquisition lane, or ordinary owner. Linux runtime qualification
// belongs on a Linux host, not to a cross-compiled test binary.
func TestCensusFolderLinuxMetadataPreparation(t *testing.T) {
	privateDir := func(t *testing.T) string {
		t.Helper()
		dir, err := filepath.EvalSymlinks(t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(dir, 0700); err != nil {
			t.Fatal(err)
		}
		return dir
	}
	workspace := privateDir(t)
	for _, name := range []string{"selected.go", "unselected.go"} {
		if err := os.WriteFile(filepath.Join(workspace, name), []byte("package fixture\n"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	t.Run("bound-roots-no-provider", func(t *testing.T) {
		runRoot := privateDir(t)
		got, err := prepareCensusFolder(censusCLIOptions{
			Workspace: workspace, PublicationRoot: runRoot,
			Sources: []string{"selected.go", "selected.go"},
		}, time.Now().Add(10*time.Second))
		if err != nil {
			t.Fatal(err)
		}
		if got == nil || got.workspace != workspace || got.runRoot != runRoot || len(got.records) != 1 || got.records[0].Name != "selected.go" {
			t.Fatalf("ASSERT_LINUX_METADATA_BOUND_ROOTS: %+v", got)
		}
	})
	for _, fault := range []string{"expired", "deadline-limit", "unsafe-identity", "depth-limit"} {
		t.Run(fault, func(t *testing.T) {
			o := censusCLIOptions{Workspace: workspace, PublicationRoot: privateDir(t), Sources: []string{"selected.go"}}
			deadline := time.Now().Add(10 * time.Second)
			switch fault {
			case "expired":
				deadline = time.Now().Add(-time.Second)
			case "deadline-limit":
				deadline = time.Now().Add(time.Minute)
			case "unsafe-identity":
				o.Sources = []string{"../escape.go"}
			case "depth-limit":
				o.Sources = []string{strings.Repeat("x/", censusMaxSourceDepth+1) + "leaf.go"}
			}
			if got, err := prepareCensusFolder(o, deadline); err == nil || got != nil {
				t.Fatalf("ASSERT_LINUX_METADATA_REFUSAL_%s: got=%+v err=%v", fault, got, err)
			}
		})
	}
}
