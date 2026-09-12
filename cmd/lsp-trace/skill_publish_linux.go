//go:build linux

package main

import (
	"os"

	"golang.org/x/sys/unix"
)

func renameDirectoryNoReplace(parent, _ *os.File, oldName, newName string) error {
	fd := int(parent.Fd())
	return unix.Renameat2(fd, oldName, fd, newName, unix.RENAME_NOREPLACE)
}
