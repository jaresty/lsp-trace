//go:build linux

package main

import "os"

// No observed-entry-spelling capability is asserted on this platform. Qualified
// includes use the general native-name traversal, with its existing budget pause.
const applicationLiteralObservedPaths = false
const applicationLiteralOpenFlags = os.O_RDONLY

func applicationLiteralOpenAt(*os.File, string) (*os.File, uint64, error) {
	return nil, 0, os.ErrInvalid
}
func applicationLiteralFDPath(*os.File) (string, error) { return "", os.ErrInvalid }
