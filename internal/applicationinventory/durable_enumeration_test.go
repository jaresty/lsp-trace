//go:build darwin || linux

package applicationinventory

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

	cat "lsp-trace/internal/provisionalfeaturecatalog"
	"lsp-trace/internal/scopejob"
)

func enumerationTestConfig(t *testing.T, n int) OwnedInventoryConfig {
	t.Helper()
	workspace, e := filepath.EvalSymlinks(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	for i := 0; i < n; i++ {
		if e := os.WriteFile(filepath.Join(workspace, fmt.Sprintf("%04d.go", i)), []byte("package example\n"), 0600); e != nil {
			t.Fatal(e)
		}
	}
	c, e := EnumerationConfig(scopejob.Config{Workspace: workspace, Roots: []string{"."}, Profile: "gopls", Language: "go", Budget: scopejob.DefaultBudget()})
	if e != nil {
		t.Fatal(e)
	}
	return c
}
func enumerationRun(t *testing.T, s DurableStore, c OwnedInventoryConfig) (InventoryJobStatus, InventoryJobStatus) {
	t.Helper()
	ctx := context.Background()
	created, e := s.Create(ctx, c)
	if e != nil {
		t.Fatal(e)
	}
	if created.Phase != "Enumerating" || created.Admissions != 0 || created.Enumeration == nil || created.Enumeration.Counts.Entries != 0 {
		t.Fatalf("creation did enumeration: %+v", created)
	}
	if _, e = s.ReserveAdmission(ctx, created.RunID, created.Head, InventoryAdmissionBinding{RequestID: "q", SourceUnitID: "u"}); e == nil {
		t.Fatal("admission before enumeration closure")
	}
	result, e := s.Enumerate(ctx, created.RunID, created.Head)
	if e != nil {
		t.Fatal(result, e)
	}
	return created, result
}
func TestInventoryEnumerationBudgetContinuity(t *testing.T) {
	for _, tc := range []struct {
		n       int
		phase   string
		batches uint64
	}{{0, "ReadyForAcquisition", 1}, {140, "ReadyForAcquisition", 2}, {300, "EnumerationPartial", 2}} {
		t.Run(fmt.Sprint(tc.n), func(t *testing.T) {
			s := ownedTestStore(t)
			c := enumerationTestConfig(t, tc.n)
			created, result := enumerationRun(t, s, c)
			if result.RunID != created.RunID || result.Phase != tc.phase || result.Used != created.Used+enumerationFixed+tc.batches*scopejob.BatchBytes || result.Packets != created.Packets+1+2*tc.batches || result.Admissions != 0 {
				t.Fatalf("ledger/phase: %+v", result)
			}
			if tc.phase == "ReadyForAcquisition" && (!result.Enumeration.Closed || result.Enumeration.Counts.Selected != uint64(tc.n) || !result.Enumeration.DenominatorComplete) {
				t.Fatal(result.Enumeration)
			}
			if tc.phase == "EnumerationPartial" && (!result.Enumeration.UnknownRemainder || result.Enumeration.Reason != "BUDGET_EXHAUSTED" || result.Enumeration.Closed) {
				t.Fatal(result.Enumeration)
			}
			h, old, e := s.head(result.RunID)
			if e != nil {
				t.Fatal(e)
			}
			h.Issued = time.Now().Add(-7 * 24 * time.Hour).UnixNano()
			if _, e = s.change(context.Background(), old, h); e != nil {
				t.Fatal(e)
			}
			reopened, e := s.JobStatus(context.Background(), result.RunID)
			if e != nil || reopened.Used != result.Used || reopened.Packets != result.Packets || reopened.JobExpiry != "NONE" || !reflect.DeepEqual(reopened.Enumeration, result.Enumeration) {
				t.Fatal(reopened, e)
			}
			if _, e = s.Enumerate(context.Background(), reopened.RunID, reopened.Head); e == nil {
				t.Fatal("enumeration replay reset budget")
			}
			if _, e = s.Seal(context.Background(), reopened.RunID, reopened.Head); e == nil {
				t.Fatal("metadata became semantic evidence")
			}
			if _, e = s.Task(context.Background(), reopened.RunID, ""); e == nil {
				t.Fatal("metadata issued a semantic task")
			}
			if reopened.Enumeration.SourceContentRead || reopened.Enumeration.AcquisitionImplemented || reopened.Enumeration.InventoryPublished {
				t.Fatal("metadata claimed native evidence")
			}
			if _, e = s.ReserveAdmission(context.Background(), reopened.RunID, reopened.Head, InventoryAdmissionBinding{RequestID: "q", SourceUnitID: "u"}); e == nil {
				t.Fatal("metadata-only config fabricated acquisition authority")
			}
		})
	}
}
func TestInventoryEnumerationFaultsRetainClaim(t *testing.T) {
	for _, stage := range []string{"after-claim", "before-segment", "after-segment", "disk"} {
		t.Run(stage, func(t *testing.T) {
			s := ownedTestStore(t)
			c := enumerationTestConfig(t, 1)
			ctx := context.Background()
			j, e := s.Create(ctx, c)
			if e != nil {
				t.Fatal(e)
			}
			got, e := s.enumerate(ctx, j.RunID, j.Head, func(at string) error {
				if stage == "disk" && at == "before-segment" {
					return os.Mkdir(filepath.Join(s.Directory, enumerationName(j.RunID, 0)), 0700)
				}
				if stage == at {
					return errors.New("injected interruption")
				}
				return nil
			})
			if e == nil || got.RunID != j.RunID || got.Status != "PARTIAL_UNCERTAIN" || got.Used != j.Used+enumerationFixed+scopejob.BatchBytes {
				t.Fatalf("fault lost charged owner: %+v %v", got, e)
			}
			again, e := s.JobStatus(ctx, j.RunID)
			if e != nil || again.Used != got.Used || again.Enumeration.Closed || !again.Enumeration.UnknownRemainder {
				t.Fatal(again, e)
			}
			if _, e = s.Enumerate(ctx, j.RunID, again.Head); e == nil {
				t.Fatal("resumed spent claim")
			}
			if _, e = s.Seal(ctx, j.RunID, again.Head); e == nil {
				t.Fatal("sealed uncertainty")
			}
		})
	}
}
func TestInventoryEnumerationCancelAndCollision(t *testing.T) {
	s := ownedTestStore(t)
	c := enumerationTestConfig(t, 1)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e := s.Create(ctx, c); e == nil {
		t.Fatal("canceled creation")
	}
	names, e := os.ReadDir(s.Directory)
	if e != nil || len(names) != 0 {
		t.Fatal(names, e)
	}
	j, e := s.Create(context.Background(), c)
	if e != nil {
		t.Fatal(e)
	}
	before, e := os.ReadFile(filepath.Join(s.Directory, inventoryHeadName(j.RunID)))
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.createOwned(context.Background(), c, j.RunID, nil); e == nil {
		t.Fatal("collision")
	}
	after, _ := os.ReadFile(filepath.Join(s.Directory, inventoryHeadName(j.RunID)))
	if string(before) != string(after) {
		t.Fatal("collision modified owner")
	}
	ctx, cancel = context.WithCancel(context.Background())
	result, e := s.enumerate(ctx, j.RunID, j.Head, func(stage string) error {
		if stage == "after-claim" {
			cancel()
		}
		return nil
	})
	if e == nil || result.Status != "PARTIAL_UNCERTAIN" || result.Used <= j.Used {
		t.Fatal(result, e)
	}
	if _, e = s.JobStatus(ctx, j.RunID); e == nil {
		t.Fatal("canceled status")
	}
}
func TestInventoryEnumerationPolicyAndDepth(t *testing.T) {
	for _, depth := range []int{1, 2} {
		t.Run(fmt.Sprint(depth), func(t *testing.T) {
			s := ownedTestStore(t)
			c := enumerationTestConfig(t, 0)
			c.Enumeration.Budget.Depth = depth
			if e := os.Mkdir(filepath.Join(c.Enumeration.Workspace, "a"), 0700); e != nil {
				t.Fatal(e)
			}
			_, got := enumerationRun(t, s, c)
			if depth == 1 && (got.Phase != "EnumerationPartial" || got.Enumeration.Reason != "TRAVERSAL_UNAVAILABLE") || depth == 2 && got.Phase != "ReadyForAcquisition" {
				t.Fatal(got)
			}
		})
	}
	s := ownedTestStore(t)
	c := enumerationTestConfig(t, 0)
	w := c.Enumeration.Workspace
	for _, name := range []string{"keep.go", "skip.go", "other.py", "notes.txt", ".gitignore"} {
		if e := os.WriteFile(filepath.Join(w, name), []byte("*"), 0600); e != nil {
			t.Fatal(e)
		}
	}
	if e := os.Mkdir(filepath.Join(w, "pruned"), 0700); e != nil {
		t.Fatal(e)
	}
	if e := os.Symlink("keep.go", filepath.Join(w, "link.go")); e != nil {
		t.Fatal(e)
	}
	c.Enumeration.Excludes = []string{"skip.go"}
	c.Enumeration.Prunes = []string{"pruned"}
	_, got := enumerationRun(t, s, c)
	n := got.Enumeration.Counts
	if n.Selected != 1 || n.UnsupportedLanguage != 1 || n.NotSource != 2 || n.UnsupportedKind != 1 || n.Excluded != 2 || n.Boundaries != 1 || got.Enumeration.DenominatorComplete || !got.Enumeration.UnknownRemainder {
		t.Fatal(n, got.Enumeration)
	}
}
func TestInventoryEnumerationStrictConfig(t *testing.T) {
	for _, change := range []func(*OwnedInventoryConfig){
		func(c *OwnedInventoryConfig) { c.Enumeration.Roots = []string{"bad\x00root"} },
		func(c *OwnedInventoryConfig) { c.Enumeration.Roots = []string{"a/b"}; c.Enumeration.Budget.Depth = 1 },
		func(c *OwnedInventoryConfig) { c.Enumeration.Roots = []string{"z", "a"} },
		func(c *OwnedInventoryConfig) { c.ResourcePolicy = "unknown" },
		func(c *OwnedInventoryConfig) { c.Enumeration.JobDeadline = new(time.Time) },
	} {
		s := ownedTestStore(t)
		c := enumerationTestConfig(t, 0)
		change(&c)
		if _, e := s.Create(context.Background(), c); e == nil {
			t.Fatal("invalid frozen enumeration accepted")
		}
		names, _ := os.ReadDir(s.Directory)
		if len(names) != 0 {
			t.Fatal("invalid config effects")
		}
	}
}
func TestInventoryEnumerationStoredRootProof(t *testing.T) {
	s := ownedTestStore(t)
	c := enumerationTestConfig(t, 0)
	_, j := enumerationRun(t, s, c)
	name := filepath.Join(s.Directory, enumerationName(j.RunID, 0))
	raw, e := os.ReadFile(name)
	if e != nil {
		t.Fatal(e)
	}
	var segment inventoryScopeSegment
	if e = json.Unmarshal(raw, &segment); e != nil {
		t.Fatal(e)
	}
	segment.Batch.Records = nil // closed frontier alone does not prove the explicit root
	raw, _ = json.Marshal(segment)
	if e = os.WriteFile(name, raw, 0600); e != nil {
		t.Fatal(e)
	}
	h, old, e := s.head(j.RunID)
	if e != nil {
		t.Fatal(e)
	}
	h.Enumeration.Last = cat.NativeDigestV3(raw)[7:]
	h.Enumeration.MetadataBytes = uint64(len(raw))
	if _, e = s.change(context.Background(), old, h); e != nil {
		t.Fatal(e)
	}
	if _, e = s.JobStatus(context.Background(), j.RunID); e == nil {
		t.Fatal("accepted invented closed root proof")
	}
}
func TestInventoryEnumerationExistingAdmissionReservation(t *testing.T) {
	s := ownedTestStore(t)
	metadata := enumerationTestConfig(t, 20)
	c := ownedTestConfig()
	c.Enumeration = metadata.Enumeration
	c.ResourcePolicy = metadata.ResourcePolicy
	c.BudgetBytes = metadata.BudgetBytes
	c.OperationWindow = metadata.OperationWindow
	created, ready := enumerationRun(t, s, c)
	op, err := s.ReserveAdmission(context.Background(), ready.RunID, ready.Head, InventoryAdmissionBinding{RequestID: "future-genuine-request", SourceUnitID: "future-genuine-unit"})
	if err != nil {
		t.Fatal(err)
	}
	reopened, err := s.JobStatus(context.Background(), ready.RunID)
	if err != nil || reopened.RunID != created.RunID || reopened.Phase != "Collecting" || reopened.Status != "PARTIAL_UNCERTAIN" || reopened.Used != ready.Used+c.AdmissionBytes || reopened.Packets != ready.Packets+1 || !reopened.Enumeration.Closed || op.state.head.RunID != created.RunID {
		t.Fatal(reopened, err)
	}
	// Reservation only: no fabricated native result is admitted by this test.
}
func TestInventoryEnumerationDirectRootsAndPaidSourceValidation(t *testing.T) {
	s := ownedTestStore(t)
	c := enumerationTestConfig(t, 0)
	for _, name := range []string{"a", "b"} {
		if e := os.Mkdir(filepath.Join(c.Enumeration.Workspace, name), 0700); e != nil {
			t.Fatal(e)
		}
	}
	c.Enumeration.Roots = []string{"a", "b", "missing"}
	c.Enumeration.Budget.Depth = 1
	_, done := enumerationRun(t, s, c)
	if !done.Enumeration.Closed || done.Enumeration.Counts.Directories != 2 || done.Enumeration.Counts.Unavailable != 1 || !done.Enumeration.UnknownRemainder {
		t.Fatal(done.Enumeration)
	}
	s = ownedTestStore(t)
	c = enumerationTestConfig(t, 0)
	c.Scope = filepath.Join(c.Scope, "absent")
	c.Enumeration.Workspace = c.Scope
	created, e := s.Create(context.Background(), c)
	if e != nil {
		t.Fatal("source validation preceded creation", e)
	}
	failed, e := s.Enumerate(context.Background(), created.RunID, created.Head)
	if e == nil || failed.Status != "PARTIAL_UNCERTAIN" || failed.Used != created.Used+enumerationFixed+scopejob.BatchBytes {
		t.Fatal(failed, e)
	}
}
func TestInventoryEnumerationStatusLeaseStartsAtEntry(t *testing.T) {
	s := ownedTestStore(t)
	_, j := enumerationRun(t, s, enumerationTestConfig(t, 1))
	h, _, e := s.head(j.RunID)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.inspectEnumerationAt(context.Background(), h, time.Now().Add(-time.Hour)); e == nil {
		t.Fatal("status renewed frozen operation window")
	}
}
func TestInventoryEnumerationStoredDepthAndFrontier(t *testing.T) {
	for _, kind := range []string{"depth", "frontier"} {
		t.Run(kind, func(t *testing.T) {
			s := ownedTestStore(t)
			c := enumerationTestConfig(t, 0)
			c.Enumeration.Budget.Depth = 1
			if kind == "frontier" {
				c.Enumeration.Budget.Depth = 2
			}
			j, e := s.Create(context.Background(), c)
			if e != nil {
				t.Fatal(e)
			}
			_, e = s.enumerate(context.Background(), j.RunID, j.Head, func(stage string) error {
				if stage == "after-claim" {
					return errors.New("stop")
				}
				return nil
			})
			if e == nil {
				t.Fatal("fault not reached")
			}
			h, old, e := s.head(j.RunID)
			if e != nil {
				t.Fatal(e)
			}
			directory := func(p string) scopejob.Record {
				return scopejob.Record{Path: p, Kind: "DIRECTORY", Role: "DIRECTORY", Mode: uint32(os.ModeDir), MetadataObserved: true}
			}
			batch := scopejob.EnumerationBatch{Records: []scopejob.Record{directory("."), directory("a")}, Done: true}
			h.Phase = "ReadyForAcquisition"
			h.Status = "READY"
			h.TaskID = ""
			h.Deadline = 0
			h.Enumeration.Closed = true
			if kind == "frontier" {
				batch = scopejob.EnumerationBatch{Records: []scopejob.Record{directory(".")}, Frontier: []string{".", "invented"}, Reason: "TRAVERSAL_UNAVAILABLE"}
				h.Phase = "EnumerationPartial"
				h.Status = "PARTIAL"
				h.Enumeration.Closed = false
				h.Enumeration.Reason = batch.Reason
			}
			raw, e := json.Marshal(inventoryScopeSegment{Schema: "lsp-trace.inventory-scope-segment.v1", RunID: j.RunID, Steps: 128, Batch: batch})
			if e != nil {
				t.Fatal(e)
			}
			if e = os.Mkdir(filepath.Join(s.Directory, enumerationDirectory(j.RunID)), 0700); e != nil {
				t.Fatal(e)
			}
			if e = os.WriteFile(filepath.Join(s.Directory, enumerationName(j.RunID, 0)), raw, 0600); e != nil {
				t.Fatal(e)
			}
			h.Enumeration.Batches = 1
			h.Enumeration.Last = cat.NativeDigestV3(raw)[7:]
			h.Enumeration.MetadataBytes = uint64(len(raw))
			if _, e = s.change(context.Background(), old, h); e != nil {
				t.Fatal(e)
			}
			if _, e = s.JobStatus(context.Background(), j.RunID); e == nil {
				t.Fatal("accepted hash-valid impossible", kind)
			}
		})
	}
}
func TestInventoryEnumerationFirstQuoteBeforeEffects(t *testing.T) {
	s := ownedTestStore(t)
	c := enumerationTestConfig(t, 0)
	c.BudgetBytes = 32 << 20
	c.Enumeration.Budget.LogicalBytes = c.BudgetBytes
	if _, e := s.Create(context.Background(), c); e == nil {
		t.Fatal("created an unfunded first enumeration")
	}
	names, e := os.ReadDir(s.Directory)
	if e != nil || len(names) != 0 {
		t.Fatal("underfunded creation had effects", names, e)
	}
}
func TestInventoryEnumerationFrozenCallerConfig(t *testing.T) {
	s := ownedTestStore(t)
	c := enumerationTestConfig(t, 1)
	j, e := s.Create(context.Background(), c)
	if e != nil {
		t.Fatal(e)
	}
	c.Enumeration.Roots[0] = "missing"
	c.Enumeration.Budget.OperationTimeout = time.Nanosecond
	c.ResourcePolicy = "changed"
	got, e := s.Enumerate(context.Background(), j.RunID, j.Head)
	if e != nil || got.Enumeration == nil || !got.Enumeration.Closed || got.Enumeration.Counts.Selected != 1 {
		t.Fatal("caller changed frozen scope", got, e)
	}
}
func TestInventoryEnumerationConcurrentClaim(t *testing.T) {
	s := ownedTestStore(t)
	c := enumerationTestConfig(t, 1)
	j, e := s.Create(context.Background(), c)
	if e != nil {
		t.Fatal(e)
	}
	claimed := make(chan struct{})
	release := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		_, e := s.enumerate(context.Background(), j.RunID, j.Head, func(stage string) error {
			if stage == "after-claim" {
				close(claimed)
				<-release
				return errors.New("interrupted")
			}
			return nil
		})
		done <- e
	}()
	<-claimed
	_, second := s.Enumerate(context.Background(), j.RunID, j.Head)
	close(release)
	first := <-done
	if first == nil || second == nil {
		t.Fatal("competing enumeration accepted")
	}
	got, e := s.JobStatus(context.Background(), j.RunID)
	if e != nil || got.Used != j.Used+enumerationFixed+scopejob.BatchBytes || got.Packets != j.Packets+3 || got.Status != "PARTIAL_UNCERTAIN" {
		t.Fatal(got, e)
	}
}
func TestInventoryEnumerationLegacyDeadlineBranch(t *testing.T) {
	now := time.Now().UnixNano()
	h := durableInventoryHead{Schema: durableHeadVersion, Issued: now, RunDeadline: now + int64(180*time.Second), Deadline: now + int64(30*time.Second), AcceptanceDeadline: now + int64(30*time.Second)}
	if !validInventoryDeadlines(h) || h.operationDeadline() != h.Deadline || h.budgetDeadline() != h.RunDeadline {
		t.Fatal("legacy deadline changed")
	}
	h.Deadline = h.RunDeadline + 1
	if validInventoryDeadlines(h) || h.operationDeadline() != h.RunDeadline {
		t.Fatal("legacy overall deadline bypassed")
	}
	h.Schema = ownedHeadVersion
	h.RunDeadline = 0
	if !validInventoryDeadlines(h) || h.operationDeadline() != h.Deadline || h.budgetDeadline() != h.Deadline {
		t.Fatal("new owner inherited overall expiry")
	}
}
func TestInventoryEnumerationNoLegacyArtifacts(t *testing.T) {
	s := ownedTestStore(t)
	_, _ = enumerationRun(t, s, enumerationTestConfig(t, 140))
	names, e := os.ReadDir(s.Directory)
	if e != nil {
		t.Fatal(e)
	}
	for _, n := range names {
		if strings.HasPrefix(n.Name(), "scope-") {
			t.Fatal("legacy controller artifact", n.Name())
		}
	}
}
