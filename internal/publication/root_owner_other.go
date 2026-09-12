//go:build !unix

package publication

import "os"

// Portable ownership metadata is unavailable here; OpenRoot's descriptor
// identity and private mode checks remain authoritative on this platform.
func validateRootOwner(os.FileInfo) error { return nil }
