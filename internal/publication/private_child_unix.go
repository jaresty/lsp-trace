//go:build linux || darwin

package publication

import (
	"golang.org/x/sys/unix"
	"os"
)

func removeEmptyChildDirectory(parent *os.File, name string) error {
	// AT_REMOVEDIR cannot unlink a substituted regular file or symlink.
	return unix.Unlinkat(int(parent.Fd()), name, unix.AT_REMOVEDIR)
}
