//go:build linux && (amd64 || arm64)

package managedprocess

import (
	"context"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
	"unsafe"
)

// Native Linux-hosted fidelity tests only. No process is started or terminated;
// the sole kill syscall is signal zero against this test's independently queried
// current process group. These tests confer no full-supervision qualification.
// Missing kernel primitives or procfs fail, rather than skip, the qualification.
func nativeLinuxFixture(t *testing.T) (nativeLinuxCensusAdapter, int32) {
	t.Helper()
	a := nativeLinuxCensusAdapter{}
	pid, code := a.SelfPID()
	if code != 0 || pid <= 0 || int(pid) != os.Getpid() {
		t.Fatalf("ASSERT_NATIVE_SELF_PID: pid=%d errno=%d", pid, code)
	}
	group, err := syscall.Getpgid(int(pid))
	if err != nil || group <= 1 {
		t.Fatalf("ASSERT_NATIVE_OWN_GROUP: group=%d err=%v", group, err)
	}
	a.ownedPGID = int32(group)
	return a, pid
}

func nativeLinuxClose(t *testing.T, a nativeLinuxCensusAdapter, fd int) {
	t.Helper()
	if code := a.Close(fd); code != 0 {
		t.Errorf("ASSERT_NATIVE_CLOSE: fd=%d errno=%d (not retried)", fd, code)
	}
}

func nativeLinuxRoot(t *testing.T, a nativeLinuxCensusAdapter) int {
	t.Helper()
	fd, code := a.OpenRoot()
	if code != 0 || fd < 0 {
		t.Fatalf("ASSERT_NATIVE_ROOT: fd=%d errno=%d", fd, code)
	}
	t.Cleanup(func() { nativeLinuxClose(t, a, fd) })
	magic, code := a.VerifyProcFS(fd)
	if code != 0 || magic != linuxProcMagic {
		t.Fatalf("ASSERT_NATIVE_PROC_MAGIC: magic=%x errno=%d", magic, code)
	}
	return fd
}

func nativeLinuxFlags(t *testing.T, fd int, directory bool) {
	t.Helper()
	flags, _, code := syscall.Syscall(syscall.SYS_FCNTL, uintptr(fd), syscall.F_GETFD, 0)
	if code != 0 || flags&syscall.FD_CLOEXEC == 0 {
		t.Fatalf("ASSERT_NATIVE_CLOEXEC: flags=%x errno=%d", flags, code)
	}
	flags, _, code = syscall.Syscall(syscall.SYS_FCNTL, uintptr(fd), syscall.F_GETFL, 0)
	if code != 0 || flags&syscall.O_ACCMODE != syscall.O_RDONLY || flags&syscall.O_NOFOLLOW == 0 || directory && flags&syscall.O_DIRECTORY == 0 {
		t.Fatalf("ASSERT_NATIVE_OPEN_FLAGS: flags=%x errno=%d directory=%v", flags, code, directory)
	}
}

func TestNativeLinuxCensusIdentityAndRecord(t *testing.T) {
	a, pid := nativeLinuxFixture(t)
	group, code := a.SelfPGID(pid)
	if code != 0 || group != a.ownedPGID {
		t.Fatalf("ASSERT_NATIVE_SELF_GROUP: group=%d errno=%d", group, code)
	}
	if code := a.ProbeGroup(a.ownedPGID); code != 0 {
		t.Fatalf("ASSERT_NATIVE_OWNED_NONABSENT: errno=%d", code)
	}
	root := nativeLinuxRoot(t, a)
	nativeLinuxFlags(t, root, true)
	var decimal [16]byte
	dir, code := a.OpenPID(root, linuxPIDBytes(pid, &decimal))
	if code != 0 || dir < 0 {
		t.Fatalf("ASSERT_NATIVE_NUMERIC_DIRECTORY: fd=%d errno=%d", dir, code)
	}
	defer nativeLinuxClose(t, a, dir)
	nativeLinuxFlags(t, dir, true)
	fd, code := a.OpenStat(dir)
	if code != 0 || fd < 0 {
		t.Fatalf("ASSERT_NATIVE_STAT_OPEN: fd=%d errno=%d", fd, code)
	}
	defer nativeLinuxClose(t, a, fd)
	nativeLinuxFlags(t, fd, false)
	var buf [linuxCensusBuffer]byte
	// A one-byte request intentionally splits a real stat record. The adapter
	// must not consume or hide the continuation after the short first request.
	n, code := a.Read(fd, buf[:1])
	if code != 0 || n != 1 {
		t.Fatalf("ASSERT_NATIVE_FIRST_BYTE: n=%d errno=%d", n, code)
	}
	n, code = a.Read(fd, buf[1:])
	if code != 0 || n <= 0 || n >= len(buf)-1 {
		t.Fatalf("ASSERT_NATIVE_CONTINUATION: n=%d errno=%d", n, code)
	}
	id, ok := parseLinuxStat(buf[:n+1], pid)
	if !ok || id.pid != pid || id.pgid != group {
		t.Fatalf("ASSERT_NATIVE_STAT_PARSE: identity=%+v ok=%v", id, ok)
	}
	var end [1]byte
	if n, code := a.Read(fd, end[:]); n != 0 || code != 0 {
		t.Fatalf("ASSERT_NATIVE_EXPLICIT_EOF: n=%d errno=%d", n, code)
	}
	t.Log("NATIVE_ADAPTER_FIDELITY_ONLY: numeric self, procfs identity, raw stat and explicit EOF")
}

func TestNativeLinuxCensusDirentABI(t *testing.T) {
	a, pid := nativeLinuxFixture(t)
	root := nativeLinuxRoot(t, a)
	var buf [linuxCensusBuffer]byte
	check := func(fd int) int {
		t.Helper()
		n, code := a.ReadDir(fd, buf[:])
		if code != 0 || n < 0 || n > len(buf) {
			t.Fatalf("ASSERT_NATIVE_GETDENTS: n=%d errno=%d", n, code)
		}
		for at := 0; at < n; {
			_, next, ok := parseLinuxDirent64(buf[:n], at)
			if !ok || next <= at {
				t.Fatalf("ASSERT_NATIVE_DIRENT_ABI: offset=%d bytes=%d", at, n)
			}
			at = next
		}
		return n
	}
	if check(root) == 0 {
		t.Fatal("ASSERT_NATIVE_ROOT_NONEMPTY")
	}
	// Own numeric directory has a bounded fixture-sized set of entries. Do not
	// assert that a live machine-wide census is stable or fits the engine budget.
	var decimal [16]byte
	dir, code := a.OpenPID(root, linuxPIDBytes(pid, &decimal))
	if code != 0 || dir < 0 {
		t.Fatalf("ASSERT_NATIVE_DIRENT_DIRECTORY: fd=%d errno=%d", dir, code)
	}
	defer nativeLinuxClose(t, a, dir)
	for i := 0; i < linuxCensusDirs; i++ {
		if check(dir) == 0 {
			t.Log("NATIVE_ADAPTER_FIDELITY_ONLY: getdents64 Linux64LE and explicit directory EOF")
			return
		}
	}
	t.Fatal("ASSERT_NATIVE_DIRECTORY_EOF: fixture exceeded bounded reads")
}

func TestNativeLinuxCensusRefusalsAndBadFD(t *testing.T) {
	a, _ := nativeLinuxFixture(t)
	for _, p := range []int32{-1, 0, 1} {
		if got := a.ProbeGroup(p); got != linuxCensusErrno(syscall.EINVAL) {
			t.Fatalf("ASSERT_NATIVE_PROBE_REFUSAL: pgid=%d errno=%d", p, got)
		}
	}
	// Submit only our independently known group for the authority-mismatch
	// case. No guessed positive PID/PGID or ESRCH assertion is used.
	if (nativeLinuxCensusAdapter{}).ProbeGroup(a.ownedPGID) != linuxCensusErrno(syscall.EINVAL) || (nativeLinuxCensusAdapter{ownedPGID: 1}).ProbeGroup(1) != linuxCensusErrno(syscall.EINVAL) {
		t.Fatal("ASSERT_NATIVE_PROBE_AUTHORITY")
	}
	for _, name := range []string{"", "0", "01", "-1", "1/2", "1\x00", "2147483648", "12345678901"} {
		if fd, code := a.OpenPID(-1, []byte(name)); fd != -1 || code != linuxCensusErrno(syscall.EINVAL) {
			t.Fatalf("ASSERT_NATIVE_PID_REFUSAL: fd=%d errno=%d", fd, code)
		}
	}
	if _, code := a.SelfPGID(0); code != linuxCensusErrno(syscall.EINVAL) {
		t.Fatal("ASSERT_NATIVE_NUMERIC_SELF_REQUIRED")
	}
	bad := linuxCensusErrno(syscall.EBADF)
	if fd, code := a.OpenPID(-100, []byte("1")); fd != -1 || code != bad {
		t.Fatal("ASSERT_NATIVE_PID_DESCRIPTOR_RELATIVE")
	}
	if fd, code := a.OpenStat(-100); fd != -1 || code != bad {
		t.Fatal("ASSERT_NATIVE_STAT_DESCRIPTOR_RELATIVE")
	}
	if a.Close(-1) != bad {
		t.Fatal("ASSERT_NATIVE_BAD_CLOSE")
	}
	if _, code := a.VerifyProcFS(-1); code != bad {
		t.Fatal("ASSERT_NATIVE_BAD_FSTATFS")
	}
	if fd, code := a.OpenPID(-1, []byte("1")); fd != -1 || code != bad {
		t.Fatal("ASSERT_NATIVE_BAD_PID_OPEN")
	}
	if fd, code := a.OpenStat(-1); fd != -1 || code != bad {
		t.Fatal("ASSERT_NATIVE_BAD_STAT_OPEN")
	}
	var buf [1]byte
	if n, code := a.Read(-1, buf[:]); n != -1 || code != bad {
		t.Fatal("ASSERT_NATIVE_BAD_READ")
	}
	if n, code := a.ReadDir(-1, buf[:]); n != -1 || code != bad {
		t.Fatal("ASSERT_NATIVE_BAD_GETDENTS")
	}
}

func TestNativeLinuxCensusNoFollow(t *testing.T) {
	a := nativeLinuxCensusAdapter{}
	path := t.TempDir()
	if err := os.Mkdir(filepath.Join(path, "actual"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("actual", filepath.Join(path, "123")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("actual", filepath.Join(path, "stat")); err != nil {
		t.Fatal(err)
	}
	root, err := syscall.Open(path, syscall.O_RDONLY|syscall.O_DIRECTORY|syscall.O_CLOEXEC, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer nativeLinuxClose(t, a, root)
	if fd, code := a.OpenPID(root, []byte("123")); fd >= 0 || code == 0 {
		if fd >= 0 {
			nativeLinuxClose(t, a, fd)
		}
		t.Fatal("ASSERT_NATIVE_PID_NOFOLLOW")
	}
	if fd, code := a.OpenStat(root); fd >= 0 || code != linuxCensusErrno(syscall.ELOOP) {
		if fd >= 0 {
			nativeLinuxClose(t, a, fd)
		}
		t.Fatal("ASSERT_NATIVE_STAT_NOFOLLOW")
	}
	if magic, code := a.VerifyProcFS(root); code != 0 || magic == linuxProcMagic {
		t.Fatalf("ASSERT_NATIVE_NONPROC_IDENTITY: magic=%x errno=%d", magic, code)
	}
}

func TestNativeLinuxCensusEngineRecord(t *testing.T) {
	a, pid := nativeLinuxFixture(t)
	m := &Manager{}
	b := &privateProviderWindow{ctx: context.Background(), manager: m, profile: m.PrivateProviderWindowProfile(), deadline: time.Now().Add(time.Second)}
	e, err := b.bindLinuxCensus(a.ownedPGID, a)
	if err != nil {
		t.Fatal(err)
	}
	// Exercise the unchanged production open/record/close/spend/parser path,
	// not a second census model. Deliberately not a full live-machine observe.
	r := linuxCensusRun{e: e, end: b.deadline}
	if !r.open(0, linuxStageRoot, false, 0) {
		r.close(0) // an acquired descriptor remains owned after a clock refusal
		t.Fatalf("ASSERT_NATIVE_ENGINE_ROOT: %+v", r.result)
	}
	id, ok := r.record(pid)
	r.close(0)
	if !ok || id.pid != pid || id.pgid != a.ownedPGID || r.result.Failure.bits != 0 {
		t.Fatalf("ASSERT_NATIVE_ENGINE_RECORD: id=%+v ok=%v result=%+v", id, ok, r.result)
	}
	q := fixedLinuxCensusQuote()
	if q.operations-e.remaining.operations != 8 || q.raw-e.remaining.raw != 4097 || q.records-e.remaining.records != 1 || q.reads-e.remaining.reads != 2 || e.fds[0].held || e.fds[1].held || e.fds[2].held {
		t.Fatalf("ASSERT_NATIVE_ENGINE_LEDGER: remaining=%+v fds=%+v", e.remaining, e.fds)
	}
	if unsafe.Sizeof(a)+unsafe.Sizeof(*e)+unsafe.Sizeof(r)+unsafe.Sizeof(*b) > linuxCensusStorage {
		t.Fatal("ASSERT_NATIVE_FIXED_STORAGE")
	}
	t.Log("NATIVE_ADAPTER_ENGINE_RECORD_ONLY: no launch, supervision, absence, or termination claim")
}
