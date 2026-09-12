//go:build linux

package main

import (
	"os"

	"golang.org/x/sys/unix"
)

func renameDirectoryNoReplace(parent, container, _ *os.File, _, payload, newName string) error {
	return unix.Renameat2(int(container.Fd()), payload, int(parent.Fd()), newName, unix.RENAME_NOREPLACE)
}
