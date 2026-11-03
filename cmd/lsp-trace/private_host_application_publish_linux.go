//go:build linux

package main

import (
	"golang.org/x/sys/unix"
	"os"
)

func applicationPublishNoReplace(dir *os.File, stage, final string) error {
	e := unix.Renameat2(int(dir.Fd()), stage, int(dir.Fd()), final, unix.RENAME_NOREPLACE)
	if e != nil {
		return &os.LinkError{Op: "publish", Old: stage, New: final, Err: e}
	}
	return nil
}
