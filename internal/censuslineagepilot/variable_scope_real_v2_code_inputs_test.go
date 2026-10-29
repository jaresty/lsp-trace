//go:build darwin || linux

package censuslineagepilot_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Only fixture SETUP may write/commit. Verification never calls go list/build,
// creates cache files, invokes a shell, starts a provider or trusts index bits.
func variableOwnedGitSetup(t *testing.T, root string, args ...string) string {
	t.Helper()
	cmd := exec.Command("/usr/bin/git", append([]string{"-c", "core.fsmonitor=false", "--no-optional-locks", "-C", root}, args...)...)
	cmd.Env = []string{"PATH=/usr/bin:/bin", "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null", "GIT_AUTHOR_NAME=Owned", "GIT_AUTHOR_EMAIL=owned@example.invalid", "GIT_COMMITTER_NAME=Owned", "GIT_COMMITTER_EMAIL=owned@example.invalid"}
	b, e := cmd.CombinedOutput()
	if e != nil {
		t.Fatalf("owned git setup %v: %s %v", args, b, e)
	}
	return string(b)
}
func variableOwnedGitRepo(t *testing.T) string {
	t.Helper()
	root, e := filepath.EvalSymlinks(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	variableOwnedGitSetup(t, root, "init", "-q")
	return root
}
func TestVariableCallerV2GitIgnoresAmbientPATH(t *testing.T) {
	root := variableOwnedGitRepo(t)
	if e := os.WriteFile(filepath.Join(root, "owned.txt"), []byte("owned\n"), 0600); e != nil {
		t.Fatal(e)
	}
	variableOwnedGitSetup(t, root, "add", "owned.txt")
	variableOwnedGitSetup(t, root, "commit", "-qm", "owned setup")
	head := strings.TrimSpace(variableOwnedGitSetup(t, root, "rev-parse", "HEAD"))
	trap := t.TempDir()
	receipt := filepath.Join(trap, "ambient-git-invoked")
	// Deliberately only an owned fake executable uses a shell in this negative
	// fixture; the caller must never select or execute it.
	body := "#!/bin/sh\nprintf invoked > '" + receipt + "'\nprintf FAKE_AMBIENT_GIT\\n\n"
	if e := os.WriteFile(filepath.Join(trap, "git"), []byte(body), 0700); e != nil {
		t.Fatal(e)
	}
	t.Setenv("PATH", trap+":/usr/bin:/bin")
	got, e := variableGit(context.Background(), root, "rev-parse", "HEAD")
	if _, se := os.Stat(receipt); !os.IsNotExist(se) || e != nil || string(got) != head+"\n" {
		t.Fatalf("ASSERT_GIT_ABSOLUTE_PATH: ambient receipt=%v output=%q err=%v", se, got, e)
	}
	t.Log("ASSERT_GIT_ABSOLUTE_PATH: PASS; ambient git never invoked")
}
