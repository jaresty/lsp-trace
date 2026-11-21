//go:build (darwin || linux) && lsp_trace_controlled_describe_fixture

package applicationinventory

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"

	is "lsp-trace/internal/inventorysuccessor"
	"lsp-trace/internal/publication"
	"lsp-trace/internal/scopejob"
	"lsp-trace/sessionruntime"
)

type retentionPreflightReopen struct {
	PID    int
	Status InventoryJobStatus
	Batch  InventoryNativeBatchV1
}
type retentionPreflightFile struct {
	Path   string
	Bytes  int64
	SHA256 string
}

// Reads the entire OWNED disposable tree, not selected successful objects.
// This is a point-in-time serialized-length measurement, not disk allocation,
// physical IO, peak storage, a quota, or a real-repository capacity estimate.
func retentionPreflightFiles(t *testing.T, root string) []retentionPreflightFile {
	t.Helper()
	var result []retentionPreflightFile
	e := filepath.WalkDir(root, func(path string, d fs.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if d.IsDir() {
			return nil
		}
		if !d.Type().IsRegular() {
			t.Fatalf("nonregular owned measurement entry: %s", path)
		}
		raw, e := os.ReadFile(path)
		if e != nil {
			return e
		}
		rel, e := filepath.Rel(root, path)
		if e != nil {
			return e
		}
		digest := sha256.Sum256(raw)
		result = append(result, retentionPreflightFile{rel, int64(len(raw)), hex.EncodeToString(digest[:])})
		return nil
	})
	if e != nil {
		t.Fatal(e)
	}
	return result
}

// This helper performs ONLY product metadata reads. It never constructs a
// Starter, initializes native ownership, or consumes/passes a private capability.
// Exact entrypoint: test binary -test.run=^TestInventoryRetentionPreflightReader$
// -- retention-preflight-reader-v1 <private-temp-store-directory> <RunID>.
func TestInventoryRetentionPreflightReader(t *testing.T) {
	n := len(os.Args)
	if n < 4 || os.Args[n-3] != "retention-preflight-reader-v1" {
		t.Skip("explicit metadata reader helper only")
	}
	dir, run := os.Args[n-2], os.Args[n-1]
	tmp, e := filepath.EvalSymlinks(os.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	canonical, e := filepath.EvalSymlinks(dir)
	if e != nil || canonical != dir {
		t.Fatal("noncanonical helper root", e)
	}
	rel, e := filepath.Rel(tmp, dir)
	if e != nil || !filepath.IsLocal(rel) || rel == "." {
		t.Fatal("helper requires owned temporary child")
	}
	info, e := os.Stat(dir)
	if e != nil || !info.IsDir() || info.Mode().Perm() != 0700 {
		t.Fatal("helper requires private directory", e)
	}
	root, e := publication.OpenRoot(dir)
	if e != nil {
		t.Fatal(e)
	}
	defer func() {
		if e := root.Close(); e != nil {
			t.Error(e)
		}
	}()
	s := DurableStore{Root: root, Directory: dir}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	status, e := s.JobStatus(ctx, run)
	if e != nil {
		t.Fatal(e)
	}
	batch, e := s.NativeDiscoveryBatchV1(ctx, run, status.Head)
	if e != nil {
		t.Fatal(e)
	}
	raw, e := json.Marshal(retentionPreflightReopen{os.Getpid(), status, batch})
	if e != nil {
		t.Fatal(e)
	}
	t.Log("PREFLIGHT_REOPEN_JSON " + string(raw))
}

func TestInventoryRetentionPreflight(t *testing.T) {
	started := time.Now()
	ctx := context.Background()
	base, e := filepath.EvalSymlinks(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	source, dir := filepath.Join(base, "source"), filepath.Join(base, "store")
	for _, p := range []string{source, dir} {
		if e = os.Mkdir(p, 0700); e != nil {
			t.Fatal(e)
		}
	}
	fixture, e := sessionruntime.NewControlledInventoryPipeFixture(source)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() {
		if fixture != nil {
			c, cancel := context.WithTimeout(ctx, time.Second)
			defer cancel()
			if e := fixture.Close(c); e != nil {
				t.Error(e)
			}
		}
	})
	root, e := publication.OpenRoot(dir)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() {
		if root != nil {
			if e := root.Close(); e != nil {
				t.Error(e)
			}
		}
	})
	s := DurableStore{Root: root, Directory: dir}
	cfg, e := NativeBoundaryConfigV1(scopejob.Config{Workspace: source, Roots: []string{"."}, Excludes: []string{"*.excluded"}, Profile: "gopls", Language: "go", Budget: scopejob.DefaultBudget()})
	if e != nil {
		t.Fatal(e)
	}
	created, e := s.Create(ctx, cfg)
	if e != nil {
		t.Fatal(e)
	}
	// Caller edits cannot change the already committed exclusion policy.
	cfg.Enumeration.Excludes[0] = "*.go"
	ready, e := s.Enumerate(ctx, created.RunID, created.Head)
	if e != nil {
		t.Fatal(e)
	}
	batch, e := s.NativeDiscoveryBatchV1(ctx, ready.RunID, ready.Head)
	if e != nil || batch.Counts.Selected != 17 || batch.UnknownRemainder || batch.Ordinal != 0 || len(batch.Members) != 16 {
		t.Fatal("FROZEN_ENUMERATION", e, batch)
	}
	// A lower bound only: selected bootstrap/control, retention, admission and
	// additional checkpoint transactions have NOT been added or authorized.
	lower := ready.Used + 2*is.InventoryNativeControlBytesV1
	if lower <= ready.Limit {
		t.Fatal("OLD_PROFILE_NOT_TWO_BATCH_ENVELOPE")
	}
	before := retentionPreflightFiles(t, dir)
	if _, e = s.ReserveAdmission(ctx, ready.RunID, ready.Head, InventoryAdmissionBinding{RequestID: "data-only", SourceUnitID: "data-only"}); e == nil {
		t.Fatal("OLD_NATIVE_ADMISSION_MUST_REFUSE")
	}
	larger := cfg
	enum := *cfg.Enumeration
	larger.Enumeration = &enum
	larger.BudgetBytes = lower
	larger.Enumeration.Budget.LogicalBytes = lower
	if _, e = s.Create(ctx, larger); e == nil {
		t.Fatal("OLD_PROFILE_MUST_NOT_ENLARGE")
	}
	if !reflect.DeepEqual(before, retentionPreflightFiles(t, dir)) {
		t.Fatal("REFUSALS_MUST_NOT_MUTATE_STORE")
	}
	// No Construction/NewStarter/OpenInventoryFundedDiscovery call occurs here.
	// Remove constructor-owned source literals and close the parent directory
	// handle before launching the fresh reader. Only directory + RunID cross.
	closeCtx, cancel := context.WithTimeout(ctx, time.Second)
	e = fixture.Close(closeCtx)
	cancel()
	if e != nil {
		t.Fatal(e)
	}
	fixture = nil
	entries, e := os.ReadDir(source)
	if e != nil || len(entries) != 0 {
		t.Fatal("FIXTURE_CLEANUP", e)
	}
	if e = root.Close(); e != nil {
		t.Fatal(e)
	}
	root = nil
	s = DurableStore{}
	files := retentionPreflightFiles(t, base)
	binary, e := os.Executable()
	if e != nil {
		t.Fatal(e)
	}
	helperCtx, helperCancel := context.WithTimeout(ctx, 15*time.Second)
	defer helperCancel()
	cmd := exec.CommandContext(helperCtx, binary, "-test.run=^TestInventoryRetentionPreflightReader$", "-test.count=1", "-test.timeout=10s", "-test.v", "--", "retention-preflight-reader-v1", dir, ready.RunID)
	cmd.Env = []string{"HOME=" + os.Getenv("HOME"), "PATH=" + os.Getenv("PATH"), "TMPDIR=" + os.Getenv("TMPDIR"), "GOPROXY=off", "GOSUMDB=off", "GOTOOLCHAIN=local", "GOTELEMETRY=off"}
	invocation, e := json.Marshal(cmd.Args)
	if e != nil {
		t.Fatal(e)
	}
	t.Log("PREFLIGHT_HELPER_COMMAND_JSON " + string(invocation))
	helperStart := time.Now()
	out, e := cmd.CombinedOutput()
	elapsed := time.Since(helperStart)
	if e != nil {
		t.Fatalf("fresh metadata reader: %v\n%s", e, out)
	}
	// Retain the exact helper output as quoted text in the parent test log.
	// It contains disposable fixture metadata only, not an authority handoff.
	t.Logf("PREFLIGHT_READER_OUTPUT %q", out)
	var reopened retentionPreflightReopen
	found := 0
	for _, line := range strings.Split(string(out), "\n") {
		if _, suffix, ok := strings.Cut(line, "PREFLIGHT_REOPEN_JSON "); ok {
			if e = json.Unmarshal([]byte(suffix), &reopened); e != nil {
				t.Fatal(e)
			}
			found++
		}
	}
	if found != 1 || reopened.PID != cmd.ProcessState.Pid() || reopened.PID == os.Getpid() {
		t.Fatal("FRESH_PROCESS_REQUIRED")
	}
	if !reflect.DeepEqual(reopened.Status, ready) || !reflect.DeepEqual(reopened.Batch, batch) {
		t.Fatal("FRESH_REOPEN_METADATA_CHANGED")
	}
	if !reflect.DeepEqual(files, retentionPreflightFiles(t, base)) {
		t.Fatal("READ_ONLY_REOPEN_CHANGED_BYTES")
	}
	var total int64
	for _, f := range files {
		total += f.Bytes
		if strings.Contains(f.Path, "inventory-native-") || strings.Contains(f.Path, "inventory-evidence-") {
			t.Fatal("INCOMPLETE_ENVELOPE_CAUSED_ACQUISITION")
		}
	}
	rss := int64(-1)
	if usage, ok := cmd.ProcessState.SysUsage().(*syscall.Rusage); ok {
		rss = usage.Maxrss
		if runtime.GOOS == "linux" {
			rss *= 1024
		}
	}
	report := struct {
		Phase                                                 string
		ParentPID, ReaderPID                                  int
		HUsed, HLimit, TwoDiscoveryLowerBound                 uint64
		TotalRegularFileBytes                                 int64
		Files                                                 []retentionPreflightFile
		HelperElapsedNS, TestElapsedNS, HelperPeakRSSBytes    int64
		PeakStorageBytes, ApplicationReads, ApplicationWrites *uint64
	}{"ENUMERATION_PREFLIGHT_ONLY_NOT_RETAINED_CHECKPOINT", os.Getpid(), reopened.PID, ready.Used, ready.Limit, lower, total, files, int64(elapsed), int64(time.Since(started)), rss, nil, nil, nil}
	raw, e := json.Marshal(report)
	if e != nil {
		t.Fatal(e)
	}
	t.Log("PREFLIGHT_MEASUREMENT_JSON " + string(raw))
	t.Log("PREFLIGHT_BLOCKED: fresh metadata reopen preserves spending and frozen policy; no native allocation, no admission, no batch2; complete envelope and custody bridge absent")
}
