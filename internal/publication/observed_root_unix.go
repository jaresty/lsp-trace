//go:build linux || darwin

package publication

import (
	"golang.org/x/sys/unix"
	"os"
)

func openObservedDirectory(parent *os.File, name string) (*os.File, error) {
	flags := unix.O_RDONLY | unix.O_DIRECTORY | unix.O_NOFOLLOW | unix.O_CLOEXEC
	var fd int
	var err error
	if parent == nil {
		if name != "/" {
			return nil, ErrObservedRootAdmission
		}
		fd, err = unix.Open("/", flags, 0)
	} else {
		fd, err = unix.Openat(int(parent.Fd()), name, flags, 0)
	}
	if err != nil {
		return nil, err
	}
	return os.NewFile(uintptr(fd), name), nil
}
