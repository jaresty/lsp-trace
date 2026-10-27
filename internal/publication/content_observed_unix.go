//go:build linux || darwin

package publication

import (
	"golang.org/x/sys/unix"
	"os"
)

func statObservedContent(r *Root, id string) (os.FileInfo, error) {
	// os.Root.Lstat is descriptor-bound. Check that both root capabilities still
	// identify the same pinned directory before using it for namespace observation.
	info, err := r.handle.Stat(".")
	if err != nil {
		return nil, err
	}
	if !os.SameFile(r.info, info) {
		return nil, ErrContentObservation
	}
	return r.handle.Lstat(id)
}
func openObservedContent(r *Root, id string) (observedFile, error) {
	// O_NOFOLLOW rejects replacement by a symlink at the open itself. NONBLOCK
	// avoids waiting on a FIFO substituted after lstat; fstat then rejects it.
	fd, err := unix.Openat(int(r.file.Fd()), id, unix.O_RDONLY|unix.O_NOFOLLOW|unix.O_NONBLOCK|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, err
	}
	return os.NewFile(uintptr(fd), id), nil
}
