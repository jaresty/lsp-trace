//go:build darwin || linux

package applicationinventory

import (
	"context"
	dw "lsp-trace/internal/describeworker"
	sc "lsp-trace/internal/inventorysuccessor/sourcecontinuation"
	cat "lsp-trace/internal/provisionalfeaturecatalog"
	"lsp-trace/internal/publication"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func ownedTestStore(t *testing.T) DurableStore {
	t.Helper()
	d, e := filepath.EvalSymlinks(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	if e = os.Chmod(d, 0700); e != nil {
		t.Fatal(e)
	}
	r, e := publication.OpenRoot(d)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() {
		if e := r.Close(); e != nil {
			t.Error(e)
		}
	})
	return DurableStore{Root: r, Directory: d}
}
func ownedTestConfig() OwnedInventoryConfig {
	p := "sha256:" + strings.Repeat("1", 64)
	return OwnedInventoryConfig{Scope: "bounded controlled", Provenance: "not enumeration", Interpreter: "i", Policy: sc.DurableDescribePolicyV1{Reviewer: "r", ReviewMode: cat.ReviewModeSubagentsV3, RolePrompts: [2]string{"interpret", "review"}}, Pins: dw.HostPinsV2{WorkerSHA256: p, ModelSHA256: p, GrammarSHA256: p}, ResponseLimits: dw.SuppliedResponseLimits{MaxAdapterBytes: 4096, MaxSemanticBytes: 2048}, MaxAdmissions: 2, AdmissionBytes: 4 << 20, OperationWindow: time.Second, TaskWindow: 20 * time.Second}
}
func TestOwnedInventoryCreateInvalidPreeffect(t *testing.T) {
	for _, change := range []func(*OwnedInventoryConfig){func(c *OwnedInventoryConfig) { c.MaxAdmissions = 101 }, func(c *OwnedInventoryConfig) { c.BudgetBytes = 1 }, func(c *OwnedInventoryConfig) { c.Schema = "wrong" }, func(c *OwnedInventoryConfig) { c.OperationWindow = 0 }, func(c *OwnedInventoryConfig) { c.Policy.RolePrompts[0] = "" }} {
		s := ownedTestStore(t)
		c := ownedTestConfig()
		change(&c)
		if _, e := s.Create(context.Background(), c); e == nil {
			t.Fatal("invalid create")
		}
		xs, e := os.ReadDir(s.Directory)
		if e != nil || len(xs) != 0 {
			t.Fatal("pre-effect refusal", e, xs)
		}
	}
}
func TestOwnedInventoryNoJobExpiryAndFiniteLease(t *testing.T) {
	ctx := context.Background()
	s := ownedTestStore(t)
	j, e := s.Create(ctx, ownedTestConfig())
	if e != nil {
		t.Fatal(e)
	}
	h, old, e := s.head(j.RunID)
	if e != nil {
		t.Fatal(e)
	}
	h.Issued = time.Now().Add(-7 * 24 * time.Hour).UnixNano()
	if _, e = s.change(ctx, old, h); e != nil {
		t.Fatal(e)
	}
	j, e = s.JobStatus(ctx, j.RunID)
	if e != nil || j.JobExpiry != "NONE" || j.TaskID != "" {
		t.Fatal(j, e)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	binding := InventoryAdmissionBinding{Index: 0, RequestID: "request", SourceUnitID: "unit"}
	if _, e = s.ReserveAdmission(canceled, j.RunID, j.Head, binding); e == nil {
		t.Fatal("canceled reserve")
	}
	op, e := s.ReserveAdmission(ctx, j.RunID, j.Head, binding)
	if e != nil {
		t.Fatal("old job should remain active", e)
	}
	if op.state.head.Deadline <= time.Now().UnixNano() || op.state.head.Deadline > time.Now().Add(time.Second).UnixNano() {
		t.Fatal("operation lease")
	}
	// Model passage past only this operation's lease without sleeping or altering
	// the durable ledger. No renewal and no opaque capability is manufactured.
	op.state.head.Deadline = time.Now().Add(-time.Second).UnixNano()
	if _, e = s.Admit(ctx, op, sc.PrivateSelectedResultV2{}); e == nil {
		t.Fatal("expired operation")
	}
	after, e := s.JobStatus(ctx, j.RunID)
	if e != nil || after.Status != "PARTIAL_UNCERTAIN" || after.Used != j.Used+ownedTestConfig().AdmissionBytes {
		t.Fatal(after, e)
	}
	if _, e = s.Seal(ctx, j.RunID, after.Head); e == nil {
		t.Fatal("seal uncertain")
	}
	if _, e = s.Task(ctx, j.RunID, ""); e == nil {
		t.Fatal("pre-evidence task")
	}
	if _, e = s.Submit(ctx, j.RunID, op.state.head.TaskID, "", nil); e == nil {
		t.Fatal("pre-evidence submit")
	}
}
func TestOwnedInventoryHeadBoundsBeforeClaim(t *testing.T) {
	ctx := context.Background()
	s := ownedTestStore(t)
	j, e := s.Create(ctx, ownedTestConfig())
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.ReserveAdmission(ctx, j.RunID, j.Head, InventoryAdmissionBinding{Index: 0, Provenance: strings.Repeat("\"", 4096), RequestID: "request", SourceUnitID: "unit"}); e == nil {
		t.Fatal("escaped head exceeds read bound")
	}
	after, e := s.JobStatus(ctx, j.RunID)
	if e != nil || after != j {
		t.Fatal("oversized claim had effects", after, e)
	}
	bad := s
	bad.Directory = filepath.Join(s.Directory, "other")
	if _, e = bad.Create(ctx, ownedTestConfig()); e == nil {
		t.Fatal("foreign configured directory")
	}
}
func TestOwnedInventoryConcurrentExactPredecessor(t *testing.T) {
	ctx := context.Background()
	s := ownedTestStore(t)
	j, e := s.Create(ctx, ownedTestConfig())
	if e != nil {
		t.Fatal(e)
	}
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, e := s.ReserveAdmission(ctx, j.RunID, j.Head, InventoryAdmissionBinding{Index: 0, RequestID: "request", SourceUnitID: "unit"})
			results <- e
		}()
	}
	wg.Wait()
	close(results)
	success := 0
	for e := range results {
		if e == nil {
			success++
		}
	}
	if success != 1 {
		t.Fatal("CAS winners", success)
	}
	after, e := s.JobStatus(ctx, j.RunID)
	if e != nil || after.Used != j.Used+ownedTestConfig().AdmissionBytes || after.Admissions != 0 {
		t.Fatal(after, e)
	}
}
