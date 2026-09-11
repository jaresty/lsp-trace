//go:build !aix && !darwin && !dragonfly && !freebsd && !linux && !netbsd && !openbsd && !solaris && !windows

package artifactingress

import "errors"

func makeFIFO(string) error { return errors.New("FIFO unavailable on platform") }
