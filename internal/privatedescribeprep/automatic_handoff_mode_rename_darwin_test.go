//go:build darwin

package privatedescribeprep

import "golang.org/x/sys/unix"

func renameHandoffModeNoReplace(old, next string) error {
	return unix.RenameatxNp(unix.AT_FDCWD, old, unix.AT_FDCWD, next, unix.RENAME_EXCL)
}
