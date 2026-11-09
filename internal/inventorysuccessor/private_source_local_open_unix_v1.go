//go:build darwin || linux

package inventorysuccessor

import (
	"golang.org/x/sys/unix"
	"os"
	"path/filepath"
	"strings"
)

// Walk only the explicitly pinned path via descriptor-relative O_NOFOLLOW on
// every component. No ancestor symlink, directory browse or recursive scan.
func openLocalPinV1(path string) (*os.File, error) {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path || len(path) > 4096 {
		return nil, ErrPrivateSourceLocalV1
	}
	parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
	if len(parts) < 1 || len(parts) > 128 {
		return nil, ErrPrivateSourceLocalV1
	}
	dir, e := os.Open("/")
	if e != nil {
		return nil, e
	}
	for _, part := range parts[:len(parts)-1] {
		fd, e := unix.Openat(int(dir.Fd()), part, unix.O_RDONLY|unix.O_CLOEXEC|unix.O_NOFOLLOW|unix.O_DIRECTORY, 0)
		closeErr := dir.Close()
		if e != nil {
			return nil, e
		}
		dir = os.NewFile(uintptr(fd), part)
		if closeErr != nil {
			dir.Close()
			return nil, closeErr
		}
	}
	fd, e := unix.Openat(int(dir.Fd()), parts[len(parts)-1], unix.O_RDONLY|unix.O_CLOEXEC|unix.O_NOFOLLOW|unix.O_NONBLOCK, 0)
	closeErr := dir.Close()
	if e != nil {
		return nil, e
	}
	f := os.NewFile(uintptr(fd), path)
	if closeErr != nil {
		f.Close()
		return nil, closeErr
	}
	return f, nil
}
