//go:build linux && (amd64 || arm64)

package managedprocess

import (
	"syscall"
	"time"
	"unsafe"
)

// nativeLinuxCensusAdapter is only the kernel-facing census adapter. Nothing
// selects it for process startup, funding, teardown, or reconciliation. The
// owner must supply an already-owned PGID; /proc never supplies that authority.
// Its build constraint deliberately excludes unqualified Linux ABIs: the engine
// consumes the 64-bit little-endian getdents64 layout, not portable readdir data.
//
// Admitted methods issue exactly one syscall, without EINTR retries, fallback,
// hidden probes, path resolution helpers, or growing storage. Invalid arguments
// refuse before effect. Now is the interface's explicit clock exception. Calls
// never access the owner's mutex or wait for owner work. Deadlines bound engine
// admission, not kernel return time; this adapter cannot interrupt a syscall.
// Go runtime scheduling is not an additional census operation or a time bound.
type nativeLinuxCensusAdapter struct {
	ownedPGID int32
}

var _ linuxCensusAdapter = nativeLinuxCensusAdapter{}

func (a nativeLinuxCensusAdapter) Now() time.Time { return time.Now() }

func (a nativeLinuxCensusAdapter) ProbeGroup(pgid int32) linuxCensusErrno {
	// kill(-1, 0) means all permitted processes, NOT process group 1.
	// Reject it as well as zero, negative, and non-owner-supplied targets.
	if pgid <= 1 || pgid != a.ownedPGID {
		return linuxCensusErrno(syscall.EINVAL)
	}
	_, _, errno := syscall.Syscall(syscall.SYS_KILL, uintptr(-int64(pgid)), 0, 0)
	return linuxCensusErrno(errno)
}

func (nativeLinuxCensusAdapter) SelfPID() (int32, linuxCensusErrno) {
	pid, _, errno := syscall.Syscall(syscall.SYS_GETPID, 0, 0, 0)
	return int32(pid), linuxCensusErrno(errno)
}

func (nativeLinuxCensusAdapter) SelfPGID(pid int32) (int32, linuxCensusErrno) {
	if pid <= 0 {
		return 0, linuxCensusErrno(syscall.EINVAL)
	}
	pgid, _, errno := syscall.Syscall(syscall.SYS_GETPGID, uintptr(pid), 0, 0)
	return int32(pgid), linuxCensusErrno(errno)
}

func (nativeLinuxCensusAdapter) OpenRoot() (int, linuxCensusErrno) {
	path := [...]byte{'/', 'p', 'r', 'o', 'c', 0}
	// AT_FDCWD is signed; converting a variable preserves its syscall bit pattern.
	dir := -100
	fd, _, errno := syscall.Syscall6(syscall.SYS_OPENAT, uintptr(dir), uintptr(unsafe.Pointer(&path[0])),
		syscall.O_RDONLY|syscall.O_DIRECTORY|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0, 0, 0)
	return int(fd), linuxCensusErrno(errno)
}

func (nativeLinuxCensusAdapter) VerifyProcFS(fd int) (uint64, linuxCensusErrno) {
	var stat syscall.Statfs_t
	_, _, errno := syscall.Syscall(syscall.SYS_FSTATFS, uintptr(fd), uintptr(unsafe.Pointer(&stat)), 0)
	return uint64(stat.Type), linuxCensusErrno(errno)
}

func (nativeLinuxCensusAdapter) OpenPID(root int, decimal []byte) (int, linuxCensusErrno) {
	if root == -100 { // AT_FDCWD is not an owned directory descriptor.
		return -1, linuxCensusErrno(syscall.EBADF)
	}
	// Positive int32 decimal occupies at most ten bytes, plus one NUL. Validation
	// also rejects slash, NUL, leading zero, and overflow without allocating.
	if len(decimal) > 10 {
		return -1, linuxCensusErrno(syscall.EINVAL)
	}
	if _, ok := linuxPIDName(decimal); !ok {
		return -1, linuxCensusErrno(syscall.EINVAL)
	}
	var path [11]byte
	copy(path[:], decimal)
	fd, _, errno := syscall.Syscall6(syscall.SYS_OPENAT, uintptr(root), uintptr(unsafe.Pointer(&path[0])),
		syscall.O_RDONLY|syscall.O_DIRECTORY|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0, 0, 0)
	return int(fd), linuxCensusErrno(errno)
}

func (nativeLinuxCensusAdapter) OpenStat(pidFD int) (int, linuxCensusErrno) {
	if pidFD == -100 { // Never fall back to the working directory.
		return -1, linuxCensusErrno(syscall.EBADF)
	}
	path := [...]byte{'s', 't', 'a', 't', 0}
	fd, _, errno := syscall.Syscall6(syscall.SYS_OPENAT, uintptr(pidFD), uintptr(unsafe.Pointer(&path[0])),
		syscall.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0, 0, 0)
	return int(fd), linuxCensusErrno(errno)
}

func (nativeLinuxCensusAdapter) Read(fd int, dst []byte) (int, linuxCensusErrno) {
	// SliceData also permits a zero-length request without indexing or a helper
	// read. Preserve the kernel count: a positive short read is not EOF.
	n, _, errno := syscall.Syscall(syscall.SYS_READ, uintptr(fd), uintptr(unsafe.Pointer(unsafe.SliceData(dst))), uintptr(len(dst)))
	return int(n), linuxCensusErrno(errno)
}

func (nativeLinuxCensusAdapter) ReadDir(root int, dst []byte) (int, linuxCensusErrno) {
	n, _, errno := syscall.Syscall(syscall.SYS_GETDENTS64, uintptr(root), uintptr(unsafe.Pointer(unsafe.SliceData(dst))), uintptr(len(dst)))
	return int(n), linuxCensusErrno(errno)
}

func (nativeLinuxCensusAdapter) Close(fd int) linuxCensusErrno {
	// One attempt, including EINTR. Only the engine owns persistent close
	// uncertainty; neither a retry nor a later historical absence discharges it.
	_, _, errno := syscall.Syscall(syscall.SYS_CLOSE, uintptr(fd), 0, 0)
	return linuxCensusErrno(errno)
}
