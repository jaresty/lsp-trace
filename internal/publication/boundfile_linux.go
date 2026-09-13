//go:build linux

package publication

import (
	"errors"
	"os"

	"golang.org/x/sys/unix"
)

func publishExactFD(root *Root, selector string, raw []byte, verify func([]byte) error) (bool, string, error) {
	parentFD, name, err := boundParentFD(root, selector)
	if err != nil {
		return false, "", err
	}
	defer unix.Close(parentFD)
	fd, err := unix.Openat(parentFD, ".", unix.O_RDWR|unix.O_TMPFILE|unix.O_CLOEXEC, 0o600)
	if err != nil {
		if errors.Is(err, unix.EOPNOTSUPP) || errors.Is(err, unix.ENOSYS) || errors.Is(err, unix.EINVAL) {
			return false, "", errExactFDUnsupported
		}
		return false, "", err
	}
	f := os.NewFile(uintptr(fd), "unnamed-capture-bundle")
	defer f.Close()
	if err := prepareSource(f, raw, verify); err != nil {
		return false, "", err
	}
	if err := unix.Linkat(fd, "", parentFD, name, unix.AT_EMPTY_PATH); err != nil {
		if errors.Is(err, unix.EEXIST) {
			return false, "", os.ErrExist
		}
		if errors.Is(err, unix.EOPNOTSUPP) || errors.Is(err, unix.ENOSYS) || errors.Is(err, unix.EINVAL) || errors.Is(err, unix.EPERM) {
			return false, "", errExactFDUnsupported
		}
		return false, "", err
	}
	durability := "NOT_CHECKED_POST_COMMIT"
	if unix.Fsync(parentFD) == nil {
		durability = "FINAL_DIRECTORY_SYNCED_NO_CRASH_GUARANTEE"
	}
	return true, durability, nil
}
