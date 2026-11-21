//go:build !darwin && !linux

package main

import "os"

// Unsupported ownership checks fail closed; this is not Windows ACL validation.
func censusConfigAncestorSafe(os.FileInfo) bool { return false }
