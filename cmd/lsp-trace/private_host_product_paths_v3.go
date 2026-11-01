//go:build darwin || linux

package main

import (
	"path/filepath"
	"strings"
)

// Structural validation-only path check for the NEW setup contract. It confers
// no runtime permission: setup subsequently creates private roots and performs
// actual descriptor/identity/ancestry checks before any provider can start.
func hostProductLexicalRelatedV3(a, b string) bool {
	if !filepath.IsAbs(a) || !filepath.IsAbs(b) || filepath.Clean(a) != a || filepath.Clean(b) != b {
		return true
	}
	return a == b || strings.HasPrefix(a, b+string(filepath.Separator)) || strings.HasPrefix(b, a+string(filepath.Separator))
}
