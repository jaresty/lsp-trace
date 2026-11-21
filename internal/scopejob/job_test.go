package scopejob

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"lsp-trace/internal/publication"
)

func fixture(t *testing.T) (Config, *publication.Root, string) {
	t.Helper()
	workspace, e := filepath.EvalSymlinks(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	store, e := filepath.EvalSymlinks(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	if e = os.Chmod(store, 0700); e != nil {
		t.Fatal(e)
	}
	root, e := publication.OpenRoot(store)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { _ = root.Close() })
	return Config{Workspace: workspace, Roots: []string{"."}, Profile: "gopls", Language: "go", Budget: DefaultBudget()}, root, store
}
func file(t *testing.T, base, name string) {
	t.Helper()
	path := filepath.Join(base, name)
	if e := os.MkdirAll(filepath.Dir(path), 0700); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(path, []byte("package fixture\n"), 0600); e != nil {
		t.Fatal(e)
	}
}
func TestScopeJobLargeClosedScopeAndReadback(t *testing.T) {
	c, root, store := fixture(t)
	for i := 0; i < 300; i++ {
		file(t, c.Workspace, fmt.Sprintf("f%03d.go", i))
	}
	for _, p := range []string{"vendor/v.go", "generated/gen.go", "tests/a_test.go", "web/a.ts", "notes.txt", "exclude.go"} {
		file(t, c.Workspace, p)
	}
	c.Roots = []string{"vendor", ".", "vendor", "tests"}
	c.Excludes = []string{"exclude.go"}
	r, e := Start(context.Background(), root, c, Hooks{})
	if e != nil {
		t.Fatal(e)
	}
	if r.State != "SCOPE_READY" || r.UnknownRemainder || !r.DenominatorComplete || r.Counts.Regular != 306 || r.Counts.Directories != 5 || r.Counts.Selected != 303 || r.Counts.Excluded != 1 || r.Counts.UnsupportedLanguage != 1 || r.Counts.NotSource != 1 || r.Counts.Entries != 311 {
		t.Fatalf("%+v", r)
	}
	if r.Spent.Steps <= 128 || r.Spent.Packets <= 4 || r.SourceContentRead || r.InventoryPublished || r.AcquisitionImplemented || r.Head.Expires != nil || r.Head.Config.JobDeadline != nil {
		t.Fatalf("false contract: %+v", r)
	}
	if !reflect.DeepEqual(r.Head.Config.Roots, []string{"."}) {
		t.Fatal(r.Head.Config.Roots)
	}
	later, e := Inspect(root, r.JobID, time.Now().Add(30*24*time.Hour))
	if e != nil || later.Expired || later.Spent != r.Spent || later.RetainedBytes != r.RetainedBytes {
		t.Fatalf("readback: %+v %v", later, e)
	}
	entries, e := os.ReadDir(filepath.Join(store, "scope-"+r.JobID))
	if e != nil {
		t.Fatal(e)
	}
	var bytes uint64
	for _, entry := range entries {
		info, e := entry.Info()
		if e != nil {
			t.Fatal(e)
		}
		bytes += uint64(info.Size())
		if !strings.HasPrefix(entry.Name(), "packet-") || strings.Contains(entry.Name(), "exclude") {
			t.Fatal("source name leaked into artifact filename")
		}
	}
	if bytes != r.RetainedBytes {
		t.Fatalf("retained=%d actual=%d", r.RetainedBytes, bytes)
	}
}
func TestScopeJobStepExhaustionRetainsFrontier(t *testing.T) {
	c, root, _ := fixture(t)
	for i := 0; i < 200; i++ {
		file(t, c.Workspace, fmt.Sprintf("%d.go", i))
	}
	c.Budget.Steps = 40
	r, e := Start(context.Background(), root, c, Hooks{})
	if e != nil {
		t.Fatal(e)
	}
	if r.State != "PARTIAL" || r.Reason != "STEP_LIMIT" || r.Counts.Selected != 39 || !r.UnknownRemainder || len(r.Frontier) == 0 || r.Spent.Steps != 40 {
		t.Fatalf("%+v", r)
	}
	r2, e := Inspect(root, r.JobID, time.Now())
	if e != nil || r2.Spent != r.Spent || r2.Counts != r.Counts {
		t.Fatalf("%+v %v", r2, e)
	}
}
func TestScopeJobPendingClaimNeverRefundedOrRenewed(t *testing.T) {
	c, root, _ := fixture(t)
	file(t, c.Workspace, "a.go")
	crash := errors.New("simulated interruption after reservation")
	r, e := Start(context.Background(), root, c, Hooks{AfterClaim: func(uint64) error { return crash }})
	if !errors.Is(e, crash) {
		t.Fatal(e)
	}
	first, e := Inspect(root, r.JobID, time.Now())
	if e != nil {
		t.Fatal(e)
	}
	if !first.PendingClaim || first.PendingDiskEscrow != SegmentBytes || first.Spent.Steps != BatchSteps || first.Counts.Entries != 0 || !first.UnknownRemainder || len(first.Frontier) == 0 {
		t.Fatalf("%+v", first)
	}
	c.Budget.Work = 1
	c.Roots[0] = "other"
	second, e := Inspect(root, r.JobID, time.Now().Add(48*time.Hour))
	if e != nil || second.Spent != first.Spent || second.Expired || !reflect.DeepEqual(second.Head, first.Head) {
		t.Fatalf("%+v %v", second, e)
	}
}
func TestScopeJobDiskEscrowBoundary(t *testing.T) {
	c, root, _ := fixture(t)
	file(t, c.Workspace, "a.go")
	c.Budget.RetainedBytes = HeadBytes + StopBytes
	r, e := Start(context.Background(), root, c, Hooks{})
	if e != nil {
		t.Fatal(e)
	}
	if r.Reason != "RETAINED_STORAGE_LIMIT" || r.Counts.Entries != 0 || r.Spent.Steps != 0 || r.RetainedBytes > c.Budget.RetainedBytes {
		t.Fatalf("%+v", r)
	}
	c2, root2, _ := fixture(t)
	file(t, c2.Workspace, "a.go")
	c2.Budget.RetainedBytes = HeadBytes + ClaimBytes + SegmentBytes + StopBytes
	r, e = Start(context.Background(), root2, c2, Hooks{})
	if e != nil || r.State != "SCOPE_READY" {
		t.Fatalf("%+v %v", r, e)
	}
}
func TestScopeJobInvalidConfigHasNoArtifacts(t *testing.T) {
	for _, change := range []func(*Config){func(c *Config) { c.Budget.Work = 0 }, func(c *Config) { c.Budget.RetainedBytes = 1 }, func(c *Config) { c.Budget.PathBytes = 2000 }, func(c *Config) { c.Roots = []string{"../escape"} }, func(c *Config) { c.Excludes = []string{"["} }, func(c *Config) { c.Profile = "other" }} {
		c, root, store := fixture(t)
		change(&c)
		if _, e := Start(context.Background(), root, c, Hooks{}); e == nil {
			t.Fatal("accepted invalid config")
		}
		entries, _ := os.ReadDir(store)
		if len(entries) != 0 {
			t.Fatal(entries)
		}
	}
}
func TestScopeJobOptionalDeadlineAndOperationTimeout(t *testing.T) {
	t.Run("optional deadline fixed", func(t *testing.T) {
		c, root, _ := fixture(t)
		now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
		deadline := now.Add(time.Hour)
		c.JobDeadline = &deadline
		r, e := Start(context.Background(), root, c, Hooks{Now: func() time.Time { return now }, AfterClaim: func(uint64) error { now = now.Add(2 * time.Hour); return nil }})
		if e != nil || r.Reason != "EXPIRED" || !r.Expired || r.Spent.Steps != BatchSteps {
			t.Fatalf("%+v %v", r, e)
		}
		again, e := Inspect(root, r.JobID, now.Add(24*time.Hour))
		if e != nil || !again.Head.Expires.Equal(deadline) || again.Spent != r.Spent {
			t.Fatalf("%+v %v", again, e)
		}
	})
	t.Run("operation timeout no global expiry", func(t *testing.T) {
		c, root, _ := fixture(t)
		file(t, c.Workspace, "a.go")
		now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
		calls := 0
		c.Budget.OperationTimeout = time.Second
		r, e := Start(context.Background(), root, c, Hooks{Now: func() time.Time {
			calls++
			if calls > 3 {
				now = now.Add(2 * time.Second)
			}
			return now
		}})
		if e != nil || r.Reason != "OPERATION_TIMEOUT" || r.Expired || r.Head.Expires != nil {
			t.Fatalf("%+v %v", r, e)
		}
	})
}
func TestScopeJobPruneBoundaryAndSymlinks(t *testing.T) {
	c, root, _ := fixture(t)
	file(t, c.Workspace, "vendor/a.go")
	file(t, c.Workspace, "test.go")
	file(t, c.Workspace, "x.ts")
	c.Prunes = []string{"vendor"}
	c.Excludes = []string{"**"}
	if e := os.Symlink("test.go", filepath.Join(c.Workspace, "link.go")); e != nil {
		t.Fatal(e)
	}
	r, e := Start(context.Background(), root, c, Hooks{})
	if e != nil {
		t.Fatal(e)
	}
	if r.State != "SCOPE_READY" || r.Counts.Boundaries != 1 || !r.UnknownRemainder || r.Counts.Selected != 0 || r.Counts.UnsupportedLanguage != 1 || r.Counts.UnsupportedKind != 1 || r.Counts.Excluded != 2 {
		t.Fatalf("%+v", r)
	}
	c2, r2, _ := fixture(t)
	outside := t.TempDir()
	file(t, outside, "secret.go")
	if e = os.Symlink(outside, filepath.Join(c2.Workspace, "link")); e != nil {
		t.Fatal(e)
	}
	c2.Roots = []string{"link"}
	got, e := Start(context.Background(), r2, c2, Hooks{})
	if e != nil || got.State != "PARTIAL" || got.Counts.Selected != 0 || !got.UnknownRemainder {
		t.Fatalf("%+v %v", got, e)
	}
}
func TestScopeJobCorruptionAndOldSchemaRefused(t *testing.T) {
	c, root, store := fixture(t)
	file(t, c.Workspace, "a.go")
	r, e := Start(context.Background(), root, c, Hooks{})
	if e != nil {
		t.Fatal(e)
	}
	name := filepath.Join(store, filename(r.JobID, "segment", 0))
	if e = os.WriteFile(name, []byte(`{"digest":"bad","payload":{}}`), 0600); e != nil {
		t.Fatal(e)
	}
	if _, e = Inspect(root, r.JobID, time.Now()); e == nil {
		t.Fatal("accepted corrupt segment")
	}
	if _, e = Inspect(root, "../legacy", time.Now()); e == nil {
		t.Fatal("accepted unsafe ID")
	}
	c2, root2, store2 := fixture(t)
	r, e = Start(context.Background(), root2, c2, Hooks{})
	if e != nil {
		t.Fatal(e)
	}
	name = filepath.Join(store2, prefix(r.JobID)+"head.json")
	raw, _ := os.ReadFile(name)
	var h Head
	if e = unseal(raw, &h); e != nil {
		t.Fatal(e)
	}
	h.Schema = "lsp-trace.scope-job.v0"
	raw, _ = seal(h)
	if e = os.WriteFile(name, raw, 0600); e != nil {
		t.Fatal(e)
	}
	if _, e = Inspect(root2, r.JobID, time.Now()); e == nil {
		t.Fatal("silently upgraded old head")
	}
}
func TestScopeJobConfiguredLimitsAndCanonicalBudgetJSON(t *testing.T) {
	b := DefaultBudget()
	raw, e := json.Marshal(b)
	if e != nil {
		t.Fatal(e)
	}
	var round Budget
	if e = json.Unmarshal(raw, &round); e != nil || round != b {
		t.Fatal(e)
	}
	for _, kind := range []string{"work", "bytes", "packets", "entries", "directories"} {
		t.Run(kind, func(t *testing.T) {
			c, root, _ := fixture(t)
			file(t, c.Workspace, "a/b.go")
			switch kind {
			case "work":
				c.Budget.Work = NamespaceWork
			case "bytes":
				c.Budget.LogicalBytes = baseSpent().LogicalBytes
			case "packets":
				c.Budget.Packets = 2
			case "entries":
				c.Budget.Entries = 1
				c.Budget.Directories = 1
			case "directories":
				c.Budget.Directories = 1
			}
			r, e := Start(context.Background(), root, c, Hooks{})
			if e != nil || r.State != "PARTIAL" || !r.UnknownRemainder {
				t.Fatalf("%+v %v", r, e)
			}
		})
	}
}
