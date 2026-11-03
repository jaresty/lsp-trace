//go:build darwin

package main

import (
	"golang.org/x/sys/unix"
	"os"
)

func applicationPublishNoReplace(dir *os.File, stage, final string) error {
	e := unix.RenameatxNp(int(dir.Fd()), stage, int(dir.Fd()), final, unix.RENAME_EXCL)
	if e != nil {
		return &os.LinkError{Op: "publish", Old: stage, New: final, Err: e}
	}
	return nil
}
