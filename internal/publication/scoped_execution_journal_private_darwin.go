//go:build darwin

package publication

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"os"

	"golang.org/x/sys/unix"
)

func newScopedExecutionSourcePrivate(parent int) (*os.File, error) {
	var token [16]byte
	if _, err := rand.Read(token[:]); err != nil {
		return nil, err
	}
	name := ".lsp-trace-execution-private-" + hex.EncodeToString(token[:])
	fd, err := unix.Openat(parent, name, unix.O_RDWR|unix.O_CREAT|unix.O_EXCL|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0600)
	if err != nil {
		return nil, err
	}
	// The source is unlinked before any bytes are written. Fclonefileat publishes
	// from this exact FD, atomically and with EEXIST rather than overwrite.
	if err := unix.Unlinkat(parent, name, 0); err != nil {
		return nil, errors.Join(err, unix.Close(fd))
	}
	return os.NewFile(uintptr(fd), "private-execution-source"), nil
}

// Darwin sys/clonefile.h: these flags are not named in the pinned x/sys.
// Never remove them or retry without them: Fclonefileat without these flags
// follows a dangling destination symlink and can create a different selector.
const (
	scopedExecutionCloneNoFollowAnyPrivate    = 0x0008
	scopedExecutionCloneResolveBeneathPrivate = 0x0010
)

func installScopedExecutionPrivate(source, parent int, selector string) error {
	return unix.Fclonefileat(source, parent, selector, unix.CLONE_NOOWNERCOPY|scopedExecutionCloneNoFollowAnyPrivate|scopedExecutionCloneResolveBeneathPrivate)
}
