//go:build linux

package publication

import (
	"os"
	"syscall"

	"golang.org/x/sys/unix"
)

func renameNoReplace(dirfd int, oldname, newname string) error {
	return unix.Renameat2(dirfd, oldname, dirfd, newname, unix.RENAME_NOREPLACE)
}

func nlink(info os.FileInfo) uint64 {
	if st, ok := info.Sys().(*syscall.Stat_t); ok {
		return uint64(st.Nlink)
	}
	return 0
}
