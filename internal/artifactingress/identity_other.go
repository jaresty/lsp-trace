//go:build !aix && !darwin && !dragonfly && !freebsd && !linux && !netbsd && !openbsd && !solaris && !windows

package artifactingress

import "os"

// Unknown platforms fail closed because link-count custody is unavailable.
func ambiguousIdentity(os.FileInfo, os.FileInfo) bool { return true }
