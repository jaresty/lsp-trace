//go:build aix || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris

package artifactingress

import "syscall"

func makeFIFO(path string) error { return syscall.Mkfifo(path, 0600) }
