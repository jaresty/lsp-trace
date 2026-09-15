package vcssymbolsidecar

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestGitDiffCollectResolvesExactRevisions(t *testing.T) {
	root := t.TempDir()
	run := func(args ...string) {
		c := exec.Command("git", append([]string{"-C", root}, args...)...)
		c.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.com", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.com")
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
	}
	run("init", "-q")
	path := filepath.Join(root, "a.go")
	os.WriteFile(path, []byte("package a\nfunc A() {}\n"), 0600)
	run("add", "a.go")
	run("commit", "-qm", "one")
	fromBytes, _ := exec.Command("git", "-C", root, "rev-parse", "HEAD").Output()
	from := string(fromBytes[:len(fromBytes)-1])
	os.WriteFile(path, []byte("package a\nfunc A() { println(1) }\n"), 0600)
	run("commit", "-qam", "two")
	got, err := (GitDiff{Timeout: 5 * time.Second}).Collect(root, []string{"a.go"}, from, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if got.FromRevision != from || len(got.ToRevision) != 40 || len(got.Lines) != 2 {
		t.Fatalf("ASSERT_SYMBOL_CHURN_GIT_EXACT_REVISION_AND_LINES: %+v", got)
	}
}

func TestGitDiffCollectRejectsSubdirectoryWorkspace(t *testing.T) {
	root := t.TempDir()
	os.Mkdir(filepath.Join(root, "sub"), 0700)
	if _, err := (GitDiff{}).Collect(filepath.Join(root, "sub"), []string{"a.go"}, "HEAD~1", "HEAD"); err == nil {
		t.Fatal("ASSERT_SYMBOL_CHURN_WORKSPACE_EQUALS_GIT_TOP_LEVEL")
	}
}
