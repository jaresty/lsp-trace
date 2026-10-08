package locationqualificationv5

import (
	"os"
	"path/filepath"
	"testing"
)

func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		next := filepath.Dir(dir)
		if next == dir {
			t.Fatal("go.mod not found")
		}
		dir = next
	}
}

func TestVerifyFreeze(t *testing.T) {
	root := filepath.Join(repoRoot(t), "docs/pilot/adr0007/experiment/location-intersection-prospective-v5")
	got, err := VerifyFreeze(root)
	if err != nil {
		t.Fatal(err)
	}
	if got != AuthorizedRootIdentity {
		t.Fatalf("identity %s", got)
	}
}

func TestVerifyExecutionAbsentBeforeRun(t *testing.T) {
	root := repoRoot(t)
	execRoot := filepath.Join(root, "docs/pilot/adr0007/experiment/location-intersection-v5-qualification-2026-10-07")
	frozenRoot := filepath.Join(root, "docs/pilot/adr0007/experiment/location-intersection-prospective-v5")
	if err := VerifyExecution(execRoot, frozenRoot); err == nil {
		t.Fatal("expected absent pre-execution manifest to fail")
	}
}
