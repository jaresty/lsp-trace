//go:build darwin

package publication

import "golang.org/x/sys/unix"

func openFDCount() int {
	count := 0
	for fd := 0; fd < unix.Getdtablesize(); fd++ {
		if _, err := unix.FcntlInt(uintptr(fd), unix.F_GETFD, 0); err == nil {
			count++
		}
	}
	return count
}
