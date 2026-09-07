//go:build !aix && !darwin && !dragonfly && !freebsd && !linux && !netbsd && !openbsd && !solaris && !windows

package source

import (
	"errors"
	"os"
)

// Fail closed where this adapter has no qualified race-safe nonblocking open.
func openInput(_ *os.Root, _ string) (*os.File, error) {
	return nil, errors.New("regular file acquisition unavailable on platform")
}
