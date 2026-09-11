//go:build aix || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris

package artifactingress

import (
	"os"
	"syscall"
)

func ambiguousIdentity(rootInfo, fileInfo os.FileInfo) bool {
	rootStat, rootOK := rootInfo.Sys().(*syscall.Stat_t)
	fileStat, fileOK := fileInfo.Sys().(*syscall.Stat_t)
	return !rootOK || !fileOK || fileStat.Nlink != 1 || fileStat.Uid != rootStat.Uid
}
