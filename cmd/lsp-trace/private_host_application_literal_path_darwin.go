//go:build darwin

package main

import (
	"os"
	"runtime"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/unix"
)

const applicationLiteralObservedPaths = true
const applicationLiteralOpenFlags = os.O_RDONLY | unix.O_NONBLOCK | unix.O_NOFOLLOW

// Open each component relative to the preceding held directory FD; never follow
// any component symlink, and never reopen by an absolute caller pathname.
func applicationLiteralOpenAt(root *os.File, relative string) (*os.File, uint64, error) {
	current := root
	var count uint64
	for _, part := range strings.Split(relative, string(os.PathSeparator)) {
		count++
		fd, e := unix.Openat(int(current.Fd()), part, applicationLiteralOpenFlags|unix.O_CLOEXEC, 0)
		if current != root {
			current.Close()
		}
		if e != nil {
			return nil, count, e
		}
		current = os.NewFile(uintptr(fd), part)
	}
	return current, count, nil
}

// F_GETPATH observes the kernel's pathname for this very descriptor, not a
// reopened caller pathname. The fixed kernel MAXPATHLEN buffer bounds the IO.
// Failure is fail-closed; it never substitutes the requested spelling.
func applicationLiteralFDPath(f *os.File) (string, error) {
	var buf [unix.PathMax]byte
	_, _, errno := syscall.Syscall(syscall.SYS_FCNTL, f.Fd(), uintptr(unix.F_GETPATH), uintptr(unsafe.Pointer(&buf[0])))
	runtime.KeepAlive(f)
	if errno != 0 {
		return "", errno
	}
	for n, b := range buf {
		if b == 0 {
			if n == 0 {
				return "", os.ErrInvalid
			}
			return string(buf[:n]), nil
		}
	}
	return "", os.ErrInvalid
}
