//go:build darwin || linux

package adr0011lifecycle

import (
	"fmt"
	"os"
	"syscall"
)

const supportedLock = true

func directoryNumbers(info os.FileInfo) (uint64, uint64, error) {
	s, ok := info.Sys().(*syscall.Stat_t)
	if !ok || s.Dev == 0 || s.Ino == 0 {
		return 0, 0, fmt.Errorf("directory device/inode unavailable")
	}
	return uint64(s.Dev), uint64(s.Ino), nil
}
func directoryIdentity(info os.FileInfo) (string, error) {
	s, ok := info.Sys().(*syscall.Stat_t)
	if !ok || s.Dev == 0 || s.Ino == 0 {
		return "", fmt.Errorf("directory identity unavailable")
	}
	return fmt.Sprintf("%d:%d", s.Dev, s.Ino), nil
}

func openLockNoFollow(path string) (*os.File, error) {
	fd, err := syscall.Open(path, syscall.O_RDWR|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0)
	if err != nil {
		return nil, err
	}
	return os.NewFile(uintptr(fd), path), nil
}
func singleLink(info os.FileInfo) bool {
	s, ok := info.Sys().(*syscall.Stat_t)
	return ok && s.Nlink == 1
}
func tryExclusive(f *os.File) error {
	return syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
}
func unlockExclusive(f *os.File) error { return syscall.Flock(int(f.Fd()), syscall.LOCK_UN) }
func lockBusy(err error) bool {
	return err == syscall.EWOULDBLOCK || err == syscall.EAGAIN || err == syscall.EINTR
}
