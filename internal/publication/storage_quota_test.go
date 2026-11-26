//go:build darwin || linux

package publication

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"golang.org/x/sys/unix"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

func quotaRoot(t *testing.T, limit uint64) *Root {
	t.Helper()
	dir := t.TempDir()
	if e := os.Chmod(dir, 0700); e != nil {
		t.Fatal(e)
	}
	r, e := OpenRoot(dir)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { r.Close() })
	if e = ConfigureStorage(context.Background(), r, limit); e != nil {
		t.Fatal(e)
	}
	return r
}
func quotaStatus(t *testing.T, r *Root) *StorageStatus {
	t.Helper()
	v, e := ReadStorageStatus(context.Background(), r)
	if e != nil || v == nil {
		t.Fatal(e)
	}
	b, _ := json.Marshal(v)
	t.Log("STORAGE_MEASUREMENT " + string(b))
	return v
}
func TestStorageQuotaBoundaryAndReplacement(t *testing.T) {
	r := quotaRoot(t, 128<<10)
	before := quotaStatus(t, r)
	if _, e := PublishBoundFile(r, "too-big", bytes.Repeat([]byte("x"), 30721), func([]byte) error { return nil }); !errors.Is(e, ErrStorageCapacity) {
		t.Fatal("EXPECTED_EARLY_CAPACITY", e)
	}
	if _, e := os.Stat(filepath.Join(r.Path(), "too-big")); !os.IsNotExist(e) {
		t.Fatal("CAPACITY_WROTE_BEFORE_RESERVE")
	}
	if got := quotaStatus(t, r); *got != *before {
		t.Fatal("refusal mutated storage accounting")
	}
	raw := bytes.Repeat([]byte("a"), 30720)
	if _, e := PublishBoundFile(r, "prefix", raw, func([]byte) error { return nil }); e != nil {
		t.Fatal(e)
	}
	v := quotaStatus(t, r)
	if v.FileBytes != 4096+30720 || v.UncertainReservedBytes != 0 || v.PeakReservedBytes != 65536 || runtime.GOOS == "darwin" && v.SampledPeakFileBytes != 65536 {
		t.Fatal("COPY_OVERLAP_ACCOUNTING", v)
	}
	if _, e := CompareAndReplaceBoundFile(context.Background(), r, "prefix", BoundFilePredecessor{Selector: "prefix", Digest: DigestForTest(raw), ByteLength: uint64(len(raw))}, raw, func([]byte) error { return nil }); !errors.Is(e, ErrStorageCapacity) {
		t.Fatal(e)
	}
	got, e := os.ReadFile(filepath.Join(r.Path(), "prefix"))
	if e != nil || !bytes.Equal(got, raw) {
		t.Fatal("accepted prefix changed", e)
	}
}
func TestStorageQuotaGrowthOrphansAndPolicy(t *testing.T) {
	r := quotaRoot(t, 2<<20)
	raw := bytes.Repeat([]byte("a"), 4096)
	if _, e := PublishBoundFile(r, "head", raw, func([]byte) error { return nil }); e != nil {
		t.Fatal(e)
	}
	next := bytes.Repeat([]byte("b"), 8192)
	if _, e := CompareAndReplaceBoundFile(context.Background(), r, "head", BoundFilePredecessor{Selector: "head", Digest: DigestForTest(raw), ByteLength: uint64(len(raw))}, next, func([]byte) error { return nil }); e != nil {
		t.Fatal(e)
	}
	v := quotaStatus(t, r)
	if v.FileBytes != 4096+8192 || runtime.GOOS == "darwin" && v.SampledPeakFileBytes < 4096+4096+2*8192 {
		t.Fatal("REPLACEMENT_PEAK", v)
	}
	// Controlled crash/orphan simulation: outside writer is not enforceable, but
	// every such regular file is charged on reopen, never silently deleted.
	if e := os.WriteFile(filepath.Join(r.Path(), "orphan.tmp"), bytes.Repeat([]byte("o"), 65536), 0600); e != nil {
		t.Fatal(e)
	}
	reopened, e := OpenRoot(r.Path())
	if e != nil {
		t.Fatal(e)
	}
	defer reopened.Close()
	if quotaStatus(t, reopened).FileBytes != v.FileBytes+65536 {
		t.Fatal("ORPHAN_NOT_CHARGED")
	}
	if e = ConfigureStorage(context.Background(), reopened, 4<<20); e == nil {
		t.Fatal("POLICY_EXTENDED")
	}
	if e = ConfigureStorage(context.Background(), reopened, 2<<20); e != nil {
		t.Fatal(e)
	}
	if e = os.Symlink("head", filepath.Join(r.Path(), "alias")); e != nil {
		t.Fatal(e)
	}
	if _, e = ReadStorageStatus(context.Background(), reopened); e == nil {
		t.Fatal("SYMLINK_ADMITTED")
	}
}
func TestStorageQuotaUncertaintyAndCancellation(t *testing.T) {
	r := quotaRoot(t, 2<<20)
	if _, e := PublishBoundFile(r, "failed", bytes.Repeat([]byte("x"), 4096), func([]byte) error { return errors.New("controlled verification refusal") }); e == nil {
		t.Fatal("expected failure")
	}
	if quotaStatus(t, r).UncertainReservedBytes != 8192 {
		t.Fatal("UNSAFE_REFUND")
	}
	reservation, e := ReserveStorage(context.Background(), r, 4096)
	if e != nil {
		t.Fatal(e)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	start := time.Now()
	if _, e = ReserveStorage(ctx, r, 1); e == nil {
		t.Fatal("lock admitted")
	}
	elapsed := time.Since(start)
	t.Logf("QUOTA_CANCEL_NS %d", elapsed.Nanoseconds())
	if elapsed > time.Second {
		t.Fatal("unresponsive lock")
	}
	if e = reservation.Complete(true); e != nil {
		t.Fatal(e)
	}
	if quotaStatus(t, r).UncertainReservedBytes != 8192 {
		t.Fatal("prior debt refunded")
	}
	f, e := os.OpenFile(filepath.Join(r.Path(), storageLedgerName), os.O_WRONLY, 0)
	if e != nil {
		t.Fatal(e)
	}
	_, e = f.WriteAt([]byte("torn"), 4)
	e = errors.Join(e, f.Close())
	if e != nil {
		t.Fatal(e)
	}
	if _, e = ReserveStorage(context.Background(), r, 1); e == nil {
		t.Fatal("TORN_LEDGER_RECOVERED_AS_FREE")
	}
}
func TestStorageQuotaNamespaceAndLegacy(t *testing.T) {
	r := quotaRoot(t, 2<<20)
	before, e := os.ReadFile(filepath.Join(r.Path(), storageLedgerName))
	if e != nil {
		t.Fatal(e)
	}
	if runtime.GOOS == "darwin" && !StorageReservedSelector(strings.ToUpper(storageLedgerName)+".tmp") {
		t.Fatal("CASE_FOLDED_LEDGER_BYPASS")
	}
	if e = RefuseUnaccountedStorage(r.Path()); e == nil {
		t.Fatal("UNACCOUNTED_LOCAL_STAGING_ADMITTED")
	}
	if _, e = r.RemovePrivateFile(storageLedgerName); e == nil {
		t.Fatal("LEDGER_REMOVED")
	}
	if _, e = PublishBoundFile(r, storageLedgerName+".forged.tmp", []byte("x"), func([]byte) error { return nil }); e == nil {
		t.Fatal("LEDGER_TEMP_ADMITTED")
	}
	child, e := r.OpenPrivateChild("nested", true)
	if e != nil {
		t.Fatal(e)
	}
	defer child.Close()
	if e = ConfigureStorage(context.Background(), child, 2<<20); e == nil {
		t.Fatal("NESTED_QUOTA_BYPASS")
	}
	after, e := os.ReadFile(filepath.Join(r.Path(), storageLedgerName))
	if e != nil || !bytes.Equal(before, after) {
		t.Fatal("POLICY_MUTATED")
	}
	legacy := t.TempDir()
	if e = os.Chmod(legacy, 0700); e != nil {
		t.Fatal(e)
	}
	lr, e := OpenRoot(legacy)
	if e != nil {
		t.Fatal(e)
	}
	defer lr.Close()
	if _, e = PublishBoundFile(lr, "historical", []byte("kept"), func([]byte) error { return nil }); e != nil {
		t.Fatal(e)
	}
	if v, e := ReadStorageStatus(context.Background(), lr); e != nil || v != nil {
		t.Fatal("LEGACY_SILENT_UPGRADE")
	}
	if e = ConfigureStorage(context.Background(), lr, 2<<20); e == nil {
		t.Fatal("POPULATED_STORE_UPGRADED")
	}
}
func TestStorageQuotaResponsivenessBaseline(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		t.Run(fmt.Sprint(enabled), func(t *testing.T) {
			path := t.TempDir()
			if e := os.Chmod(path, 0700); e != nil {
				t.Fatal(e)
			}
			r, e := OpenRoot(path)
			if e != nil {
				t.Fatal(e)
			}
			defer r.Close()
			if enabled {
				if e = ConfigureStorage(context.Background(), r, 4<<20); e != nil {
					t.Fatal(e)
				}
			}
			start := time.Now()
			for i := 0; i < 24; i++ {
				if _, e = PublishBoundFile(r, fmt.Sprintf("sample-%02d", i), bytes.Repeat([]byte("b"), 4096), func([]byte) error { return nil }); e != nil {
					t.Fatal(e)
				}
			}
			t.Logf("STORAGE_BASELINE enabled=%t writes=24 payload_bytes=98304 elapsed_ns=%d syscall_io_not_measured=true rss_not_measured=true", enabled, time.Since(start).Nanoseconds())
		})
	}
}
func TestStorageQuotaRetiredReaders(t *testing.T) {
	r := quotaRoot(t, 128<<10)
	old := bytes.Repeat([]byte("o"), 8192)
	if _, e := PublishBoundFile(r, "head", old, func([]byte) error { return nil }); e != nil {
		t.Fatal(e)
	}
	reader, e := os.Open(filepath.Join(r.Path(), "head"))
	if e != nil {
		t.Fatal(e)
	}
	defer reader.Close()
	if _, e = CompareAndReplaceBoundFile(context.Background(), r, "head", BoundFilePredecessor{Selector: "head", Digest: DigestForTest(old), ByteLength: uint64(len(old))}, bytes.Repeat([]byte("n"), 8192), func([]byte) error { return nil }); e != nil {
		t.Fatal(e)
	}
	b := make([]byte, len(old))
	if _, e = reader.ReadAt(b, 0); e != nil || !bytes.Equal(b, old) {
		t.Fatal("old reader custody lost", e)
	}
	v := quotaStatus(t, r)
	if v.RetirementReservedBytes != 8192 || v.ReservedBytes != 8192 || v.UncertainReservedBytes != 0 {
		t.Fatal("LIVE_RETIRED_INODE_NOT_CHARGED", v)
	}
	if _, e = ReserveStorage(context.Background(), r, v.AvailableBytes+1); !errors.Is(e, ErrStorageCapacity) {
		t.Fatal("RETIRED_BYTES_REUSED", e)
	}
	if e = reader.Close(); e != nil {
		t.Fatal(e)
	}
	if quotaStatus(t, r).RetirementReservedBytes != 8192 {
		t.Fatal("UNPROVED_CROSS_PROCESS_REFUND")
	}
	if _, e = r.RemovePrivateFile("head"); e != nil {
		t.Fatal(e)
	}
	v = quotaStatus(t, r)
	if v.RetirementReservedBytes != 16384 || v.FileBytes != 4096 {
		t.Fatal("REMOVAL_RELEASED_UNKNOWN_READERS", v)
	}
}
func TestStorageQuotaUnsafeReopen(t *testing.T) {
	for _, kind := range []string{"fifo", "hardlink", "nonprivate"} {
		t.Run(kind, func(t *testing.T) {
			r := quotaRoot(t, 2<<20)
			name := filepath.Join(r.Path(), "unsafe")
			var e error
			switch kind {
			case "fifo":
				e = unix.Mkfifo(name, 0600)
			case "hardlink":
				e = os.Link(filepath.Join(r.Path(), storageLedgerName), name)
			case "nonprivate":
				e = os.WriteFile(name, []byte("x"), 0644)
			}
			if e != nil {
				t.Fatal(e)
			}
			reopened, e := OpenRoot(r.Path())
			if e != nil {
				t.Fatal(e)
			}
			defer reopened.Close()
			if _, e = ReadStorageStatus(context.Background(), reopened); e == nil {
				t.Fatal("UNSAFE_REOPEN_ADMITTED", kind)
			}
		})
	}
}
func TestStorageQuotaAllocatedBlocksAreNotTheLimit(t *testing.T) {
	r := quotaRoot(t, 128<<10)
	for i := 0; i < 40; i++ {
		if _, e := PublishBoundFile(r, fmt.Sprintf("small-%02d", i), []byte("x"), func([]byte) error { return nil }); e != nil {
			t.Fatal(e)
		}
	}
	v := quotaStatus(t, r)
	if v.FileBytes != 4136 || v.FileBytes+v.ReservedBytes+v.ControlReserve > v.Limit {
		t.Fatal("content accounting", v)
	}
	t.Logf("ALLOCATION_SCOPE_DEMONSTRATION limit=%d file_bytes=%d allocated_block_sum=%d allocated_exceeds_content_limit=%t filesystem_metadata_excluded=true", v.Limit, v.FileBytes, v.AllocatedBytes, v.AllocatedBytes > v.Limit)
}
func TestStorageQuotaChild(t *testing.T) {
	at := -1
	for i, a := range os.Args {
		if a == "quota-process-v1" {
			at = i
			break
		}
	}
	if at < 0 {
		t.Skip("controlled process only")
	}
	p, mode, name := os.Args[at+1], os.Args[at+2], os.Args[at+3]
	n, _ := strconv.Atoi(os.Args[at+4])
	r, e := OpenRoot(p)
	if e != nil {
		os.Exit(10)
	}
	q, e := ReserveStorage(context.Background(), r, uint64(n))
	if errors.Is(e, ErrStorageCapacity) {
		os.Exit(3)
	}
	if e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(11)
	}
	if e = os.WriteFile(filepath.Join(p, name), bytes.Repeat([]byte("c"), n), 0600); e != nil {
		os.Exit(12)
	}
	if mode == "interrupt" {
		os.Exit(0)
	}
	if e = q.Complete(true); e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(13)
	}
	if e = r.Close(); e != nil {
		os.Exit(14)
	}
	os.Exit(0)
}
func TestStorageQuotaCrossProcessAndRestart(t *testing.T) {
	r := quotaRoot(t, 128<<10)
	exe, e := os.Executable()
	if e != nil {
		t.Fatal(e)
	}
	command := func(mode, name string, n int) *exec.Cmd {
		cmd := exec.Command(exe, "-test.run=^TestStorageQuotaChild$", "--", "quota-process-v1", r.Path(), mode, name, strconv.Itoa(n))
		cmd.Env = []string{"HOME=" + os.Getenv("HOME"), "PATH=" + os.Getenv("PATH"), "TMPDIR=" + os.Getenv("TMPDIR"), "GOPROXY=off", "GOSUMDB=off", "GOTOOLCHAIN=local", "GOTELEMETRY=off", "GOFLAGS=-buildvcs=false"}
		return cmd
	}
	a, b := command("complete", "a", 40000), command("complete", "b", 40000)
	if e = a.Start(); e != nil {
		t.Fatal(e)
	}
	if e = b.Start(); e != nil {
		t.Fatal(e)
	}
	ea, eb := a.Wait(), b.Wait()
	codes := []int{a.ProcessState.ExitCode(), b.ProcessState.ExitCode()}
	if !((codes[0] == 0 && codes[1] == 3) || (codes[0] == 3 && codes[1] == 0)) {
		t.Fatal("CROSS_PROCESS_OVERCOMMIT", codes, ea, eb)
	}
	if quotaStatus(t, r).FileBytes != 44096 {
		t.Fatal("unexpected sum")
	}
	if out, e := command("interrupt", "interrupted.tmp", 8192).CombinedOutput(); e != nil {
		t.Fatal(e, string(out))
	}
	v := quotaStatus(t, r)
	if v.UncertainReservedBytes != 8192 || v.FileBytes != 52288 {
		t.Fatal("RESTART_REFUND", v)
	}
	if out, e := command("complete", "cannot-fit", 8192).CombinedOutput(); e == nil || !strings.Contains(e.Error(), "exit status 3") {
		t.Fatal("restart overcommit", e, string(out))
	}
}
