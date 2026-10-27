//go:build linux

package publication

import (
	"golang.org/x/sys/unix"
	"os"
)

func newScopedExecutionSourcePrivate(parent int) (*os.File, error) {
	fd, err := unix.Openat(parent, ".", unix.O_RDWR|unix.O_TMPFILE|unix.O_CLOEXEC, 0600)
	if err != nil {
		return nil, err
	}
	return os.NewFile(uintptr(fd), "private-execution-source"), nil
}
func installScopedExecutionPrivate(source, parent int, selector string) error {
	return unix.Linkat(source, "", parent, selector, unix.AT_EMPTY_PATH)
}
