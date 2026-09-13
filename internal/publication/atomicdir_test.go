package publication

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func generationFixture(root *Root, final string) GenerationRequest {
	return GenerationRequest{Root: root, FinalSelector: final, Files: []GenerationFile{{Name: "nested/value", Bytes: []byte("exact")}}}
}

func stageEntry(t *testing.T, dir string) string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	const prefix = ".lsp-trace-generation-"
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), prefix) && len(entry.Name()) == len(prefix)+32 {
			return filepath.Join(dir, entry.Name())
		}
	}
	t.Fatal("staging entry not found")
	return ""
}

func withGenerationHooks(t *testing.T) {
	t.Helper()
	oldRoot, oldStage, oldCommit := testHookGenerationRootValidated, testHookGenerationStageOpened, testHookGenerationBeforeCommit
	t.Cleanup(func() {
		testHookGenerationRootValidated = oldRoot
		testHookGenerationStageOpened = oldStage
		testHookGenerationBeforeCommit = oldCommit
	})
}

func TestPublishGenerationContinuesAgainstPinnedRootAfterPathReplacement(t *testing.T) {
	withGenerationHooks(t)
	base := t.TempDir()
	ancestor := filepath.Join(base, "ancestor")
	movedAncestor := filepath.Join(base, "ancestor-pinned")
	path := filepath.Join(ancestor, "root")
	moved := filepath.Join(movedAncestor, "root")
	if err := os.MkdirAll(path, 0o700); err != nil {
		t.Fatal(err)
	}
	root, err := OpenRoot(path)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	testHookGenerationRootValidated = func() {
		if err := os.Rename(ancestor, movedAncestor); err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(path, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := PublishGeneration(generationFixture(root, "final")); err != nil {
		t.Fatalf("pinned publication failed after pathname replacement: %v", err)
	}
	if got, err := os.ReadFile(filepath.Join(moved, "final", "nested", "value")); err != nil || string(got) != "exact" {
		t.Fatalf("original pinned root not published: %q %v", got, err)
	}
	if _, err := os.Stat(filepath.Join(path, "final")); !os.IsNotExist(err) {
		t.Fatalf("replacement root received publication: %v", err)
	}
}

func TestPublishGenerationRejectsStageEntryIdentityChanges(t *testing.T) {
	for _, tc := range []struct {
		name      string
		precommit bool
		mutate    func(t *testing.T, stage string)
		check     func(t *testing.T, stage string)
	}{
		{
			name: "renamed",
			mutate: func(t *testing.T, stage string) {
				if err := os.Rename(stage, stage+"-moved"); err != nil {
					t.Fatal(err)
				}
			},
			check: func(t *testing.T, stage string) {
				if _, err := os.Stat(stage + "-moved"); err != nil {
					t.Fatalf("renamed bound stage unexpectedly removed: %v", err)
				}
			},
		},
		{
			name: "same-owner-substitution",
			mutate: func(t *testing.T, stage string) {
				if err := os.Rename(stage, stage+"-moved"); err != nil {
					t.Fatal(err)
				}
				if err := os.Mkdir(stage, 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(stage, "competitor"), []byte("keep"), 0o600); err != nil {
					t.Fatal(err)
				}
			},
			check: func(t *testing.T, stage string) {
				got, err := os.ReadFile(filepath.Join(stage, "competitor"))
				if err != nil || string(got) != "keep" {
					t.Fatalf("same-owner competitor changed: %q %v", got, err)
				}
			},
		},
		{
			name: "removed",
			mutate: func(t *testing.T, stage string) {
				if err := os.Rename(stage, stage+"-removed"); err != nil {
					t.Fatal(err)
				}
			},
			check: func(t *testing.T, stage string) {
				if _, err := os.Stat(stage); !os.IsNotExist(err) {
					t.Fatalf("removed stage name reappeared: %v", err)
				}
			},
		},
		{
			name:      "mode-change",
			precommit: true,
			mutate: func(t *testing.T, stage string) {
				if err := os.Chmod(stage, 0o755); err != nil {
					t.Fatal(err)
				}
			},
			check: func(t *testing.T, stage string) {
				if info, err := os.Stat(stage); err != nil || info.Mode().Perm() != 0o755 {
					t.Fatalf("mode-mutated stage changed during failure: %v %v", info, err)
				}
			},
		},
		{
			name:      "link-count-change",
			precommit: true,
			mutate: func(t *testing.T, stage string) {
				before, err := os.Stat(stage)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.Mkdir(filepath.Join(stage, "extra-directory"), 0o700); err != nil {
					t.Fatal(err)
				}
				after, err := os.Stat(stage)
				if err != nil {
					t.Fatal(err)
				}
				if nlink(before) == nlink(after) {
					if err := os.Chmod(stage, 0o755); err != nil {
						t.Fatal(err)
					}
				}
			},
			check: func(t *testing.T, stage string) {
				if _, err := os.Stat(filepath.Join(stage, "extra-directory")); err != nil {
					t.Fatalf("metadata-mutated stage changed during failure: %v", err)
				}
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			withGenerationHooks(t)
			dir := t.TempDir()
			if err := os.Chmod(dir, 0o700); err != nil {
				t.Fatal(err)
			}
			root, err := OpenRoot(dir)
			if err != nil {
				t.Fatal(err)
			}
			defer root.Close()
			var stage string
			mutate := func() {
				stage = stageEntry(t, dir)
				tc.mutate(t, stage)
			}
			if tc.precommit {
				testHookGenerationBeforeCommit = mutate
			} else {
				testHookGenerationStageOpened = mutate
			}
			receipt, err := PublishGeneration(generationFixture(root, "final"))
			if err == nil || receipt != nil {
				t.Fatalf("stage identity change committed: receipt=%+v err=%v", receipt, err)
			}
			if _, statErr := os.Stat(filepath.Join(dir, "final")); !os.IsNotExist(statErr) {
				t.Fatalf("final selector visible after precommit failure: %v", statErr)
			}
			tc.check(t, stage)
		})
	}
}

func TestPublishGenerationPreservesFinalCompetitorAndCleansStage(t *testing.T) {
	withGenerationHooks(t)
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	root, err := OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	testHookGenerationBeforeCommit = func() {
		if err := os.Mkdir(filepath.Join(dir, "final"), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "final", "competitor"), []byte("keep"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	receipt, err := PublishGeneration(generationFixture(root, "final"))
	if !errors.Is(err, os.ErrExist) || receipt != nil {
		t.Fatalf("final competitor outcome: receipt=%+v err=%v", receipt, err)
	}
	if got, err := os.ReadFile(filepath.Join(dir, "final", "competitor")); err != nil || string(got) != "keep" {
		t.Fatalf("final competitor changed: %q %v", got, err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".lsp-trace-generation-") {
			t.Fatalf("owned staging entry not cleaned: %s", entry.Name())
		}
	}
}

func TestPublishGenerationPrecommitFailuresDoNotLeakFDs(t *testing.T) {
	if openFDCount() < 0 {
		t.Skip("descriptor accounting unavailable")
	}
	withGenerationHooks(t)
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	root, err := OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	before := openFDCount()
	testHookGenerationStageOpened = func() {
		stage := stageEntry(t, dir)
		if err := os.Rename(stage, stage+"-moved"); err != nil {
			t.Fatal(err)
		}
	}
	for i := 0; i < 20; i++ {
		if receipt, err := PublishGeneration(generationFixture(root, "final")); err == nil || receipt != nil {
			t.Fatalf("iteration %d committed", i)
		}
	}
	after := openFDCount()
	if after != before {
		t.Fatalf("descriptor count changed: before=%d after=%d", before, after)
	}
}
