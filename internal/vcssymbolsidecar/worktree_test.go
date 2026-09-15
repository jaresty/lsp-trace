package vcssymbolsidecar

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestGitWorktreeProviderMaterializesExactDetachedRevision(t *testing.T) {
	root := t.TempDir()
	run := func(args ...string) string {
		c := exec.Command("git", append([]string{"-C", root}, args...)...)
		c.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.com", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.com")
		out, err := c.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	run("init", "-q")
	os.WriteFile(filepath.Join(root, "a.go"), []byte("package a\n"), 0600)
	run("add", "a.go")
	run("commit", "-qm", "one")
	want := run("rev-parse", "HEAD")
	w, err := (GitWorktreeProvider{Timeout: 5 * time.Second}).Open(context.Background(), root, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	path := w.Root
	if resolved, err := filepath.EvalSymlinks(path); err != nil || resolved != path {
		t.Fatalf("ASSERT_HISTORICAL_WORKTREE_RETURNS_CANONICAL_ROOT: resolved=%q root=%q err=%v", resolved, path, err)
	}
	if w.Revision != want {
		t.Fatalf("ASSERT_HISTORICAL_WORKTREE_EXACT_REVISION: got=%s want=%s", w.Revision, want)
	}
	if got := strings.TrimSpace(runAt(t, path, "rev-parse", "HEAD")); got != want {
		t.Fatalf("ASSERT_HISTORICAL_WORKTREE_CHECKED_OUT_REVISION: %s", got)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("ASSERT_HISTORICAL_WORKTREE_REMOVED: %v", err)
	}
}
func runAt(t *testing.T, root string, args ...string) string {
	t.Helper()
	out, err := exec.Command("git", append([]string{"-C", root}, args...)...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v %s", args, err, out)
	}
	return string(out)
}
