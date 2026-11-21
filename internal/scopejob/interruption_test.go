package scopejob

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestScopeJobCancellationRetainsEarlierSegmentAndNewDebit(t *testing.T) {
	c, root, _ := fixture(t)
	for i := 0; i < 300; i++ {
		file(t, c.Workspace, fmt.Sprintf("%d.go", i))
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	r, e := Start(ctx, root, c, Hooks{AfterClaim: func(seq uint64) error {
		if seq == 1 {
			cancel()
		}
		return nil
	}})
	if e != nil || r.State != "PARTIAL" || r.Reason != "INTERRUPTED" || r.Counts.Entries != 128 || r.Spent.Steps != 256 || len(r.Frontier) == 0 || r.DenominatorComplete {
		t.Fatalf("%+v %v", r, e)
	}
	again, e := Inspect(root, r.JobID, time.Now().Add(48*time.Hour))
	if e != nil || again.Spent != r.Spent || again.Counts != r.Counts || again.Expired {
		t.Fatalf("%+v %v", again, e)
	}
}
func TestScopeJobCallerMutationCannotRenewDeadlineOrChangeRoots(t *testing.T) {
	c, root, _ := fixture(t)
	file(t, c.Workspace, "a.go")
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	original := now.Add(time.Hour)
	deadline := original
	c.JobDeadline = &deadline
	r, e := Start(context.Background(), root, c, Hooks{Now: func() time.Time { return now }, AfterClaim: func(uint64) error {
		deadline = now.Add(365 * 24 * time.Hour)
		c.Roots[0] = "elsewhere"
		c.Budget.Work = 1
		now = now.Add(2 * time.Hour)
		return nil
	}})
	if e != nil || r.Reason != "EXPIRED" || r.Head.Expires == nil || !r.Head.Expires.Equal(original) || r.Head.Config.Roots[0] != "." || r.Head.Config.Budget.Work == 1 {
		t.Fatalf("%+v %v", r, e)
	}
}
func TestScopeJobWorkspaceAliasAndFileRejectedBeforeArtifacts(t *testing.T) {
	for _, alias := range []bool{true, false} {
		c, root, store := fixture(t)
		target := filepath.Join(t.TempDir(), "input")
		var e error
		if alias {
			e = os.Symlink(c.Workspace, target)
		} else {
			e = os.WriteFile(target, []byte("not a directory"), 0600)
		}
		if e != nil {
			t.Fatal(e)
		}
		c.Workspace = target
		if _, e = Start(context.Background(), root, c, Hooks{}); e == nil {
			t.Fatal("accepted invalid workspace")
		}
		entries, e := os.ReadDir(store)
		if e != nil || len(entries) != 0 {
			t.Fatalf("%v %v", entries, e)
		}
	}
}
func TestScopeJobUnavailableRootAndUnreadSourceAreNotReadabilityProof(t *testing.T) {
	c, root, _ := fixture(t)
	file(t, c.Workspace, "unread.go")
	if e := os.Chmod(filepath.Join(c.Workspace, "unread.go"), 0000); e != nil {
		t.Fatal(e)
	}
	c.Roots = []string{"unread.go", "missing"}
	r, e := Start(context.Background(), root, c, Hooks{})
	if e != nil || r.Counts.Selected != 1 || r.Counts.Unavailable != 1 || r.Counts.Boundaries != 1 || r.SourceContentRead || r.DenominatorComplete || !r.UnknownRemainder {
		t.Fatalf("%+v %v", r, e)
	}
}
