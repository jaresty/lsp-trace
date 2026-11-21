//go:build darwin || linux

package main

import (
	"os"
	"syscall"
)

func censusConfigAncestorSafe(info os.FileInfo) bool {
	owner, ok := info.Sys().(*syscall.Stat_t)
	return info.IsDir() && info.Mode().Perm()&0022 == 0 && ok && owner.Uid == uint32(os.Geteuid())
}
