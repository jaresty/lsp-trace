package retainedlifecycle

import (
	"os/exec"
	"strings"
	"testing"
	"time"
)

const assertR03Lifecycle = "ASSERT_R03_EXACT_GIT_LEASE_GC_LIFECYCLE"

func git(t *testing.T, root string, args ...string) string {
	t.Helper()
	out, err := exec.Command("git", append([]string{"-C", root}, args...)...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func unreachableCommit(t *testing.T, root string) string {
	t.Helper()
	git(t, root, "init", "-q")
	git(t, root, "config", "user.email", "r03@example.invalid")
	git(t, root, "config", "user.name", "R03")
	blob := git(t, root, "hash-object", "-w", "--stdin")
	treeInput := "100644 blob " + blob + "\ta.go\n"
	cmd := exec.Command("git", "-C", root, "mktree")
	cmd.Stdin = strings.NewReader(treeInput)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("mktree: %v: %s", err, out)
	}
	tree := strings.TrimSpace(string(out))
	cmd = exec.Command("git", "-C", root, "commit-tree", tree, "-m", "unreachable")
	out, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("commit-tree: %v: %s", err, out)
	}
	return strings.TrimSpace(string(out))
}

func exists(t *testing.T, root, oid string) bool {
	t.Helper()
	return exec.Command("git", "-C", root, "cat-file", "-e", oid+"^{commit}").Run() == nil
}

func forceGC(t *testing.T, root string) {
	t.Helper()
	git(t, root, "reflog", "expire", "--expire=now", "--all")
	git(t, root, "gc", "--prune=now")
}

func TestExactLeasePreservesThenReleaseAllowsGC(t *testing.T) {
	root := t.TempDir()
	commit := unreachableCommit(t, root)
	now := time.Unix(1_800_000_000, 0).UTC()
	manager, err := NewGitLeaseManager(root, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	lease, err := manager.Acquire("lease-a", commit, now.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	forceGC(t, root)
	if !exists(t, root, commit) {
		t.Fatalf("%s_ACTIVE_LEASE_COLLECTED", assertR03Lifecycle)
	}
	if err := manager.Release(lease); err != nil {
		t.Fatal(err)
	}
	forceGC(t, root)
	if exists(t, root, commit) {
		t.Fatalf("%s_RELEASED_OBJECT_RETAINED", assertR03Lifecycle)
	}
}

func TestExpiredLeaseSweepAllowsGC(t *testing.T) {
	root := t.TempDir()
	commit := unreachableCommit(t, root)
	now := time.Unix(1_800_000_000, 0).UTC()
	manager, err := NewGitLeaseManager(root, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Acquire("lease-expiring", commit, now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	now = now.Add(2 * time.Minute)
	if removed, err := manager.SweepExpired(); err != nil || removed != 1 {
		t.Fatalf("removed=%d err=%v", removed, err)
	}
	forceGC(t, root)
	if exists(t, root, commit) {
		t.Fatalf("%s_EXPIRED_OBJECT_RETAINED", assertR03Lifecycle)
	}
}

func TestLedgerBindsExactManifestAndEntry(t *testing.T) {
	ledger := NewLedger()
	key := Key{ManifestDigest: "sha256:" + strings.Repeat("a", 64), EntryID: "sha256:" + strings.Repeat("b", 64)}
	if err := ledger.Record(key, StateDeleted); err != nil {
		t.Fatal(err)
	}
	if got := ledger.State(key); got != StateDeleted {
		t.Fatalf("state=%s", got)
	}
	if got := ledger.State(Key{ManifestDigest: key.ManifestDigest, EntryID: "sha256:" + strings.Repeat("c", 64)}); got != StateUnknown {
		t.Fatalf("neighbor state=%s", got)
	}
	if err := ledger.Record(key, StateLimit); err == nil {
		t.Fatalf("%s_LIMIT_IS_NOT_LIFECYCLE", assertR03Lifecycle)
	}
}
