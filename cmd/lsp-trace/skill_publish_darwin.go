//go:build darwin

package main

import (
	"os"

	"golang.org/x/sys/unix"
)

func renameDirectoryNoReplace(parent, _ *os.File, oldName, newName string) error {
	fd := int(parent.Fd())
	return unix.RenameatxNp(fd, oldName, fd, newName, unix.RENAME_EXCL)
}
