//go:build windows

package artifactingress

import "os"

// Windows file IDs are checked by os.SameFile before and after the bounded read.
func ambiguousIdentity(os.FileInfo, os.FileInfo) bool { return false }
