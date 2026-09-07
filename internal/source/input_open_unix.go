//go:build aix || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris

package source

import (
	"os"
	"syscall"
)

// O_NONBLOCK applies during the scoped open itself. A concurrent replacement
// with a FIFO cannot block before we inspect the opened handle's file type.
func openInput(root *os.Root, name string) (*os.File, error) {
	return root.OpenFile(name, os.O_RDONLY|syscall.O_NONBLOCK, 0)
}
