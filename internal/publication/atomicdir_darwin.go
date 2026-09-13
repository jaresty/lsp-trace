//go:build darwin

package publication

import (
	"os"
	"syscall"

	"golang.org/x/sys/unix"
)

func renameNoReplace(dirfd int, oldname, newname string) error {
	return unix.RenameatxNp(dirfd, oldname, dirfd, newname, unix.RENAME_EXCL|unix.RENAME_NOFOLLOW_ANY)
}

func nlink(info os.FileInfo) uint64 {
	if st, ok := info.Sys().(*syscall.Stat_t); ok {
		return uint64(st.Nlink)
	}
	return 0
}
