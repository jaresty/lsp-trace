//go:build !windows

package main

import "os"

func openSkillPayloadDirectory(_ *os.File, _, path string) (*os.File, error) {
	return os.Open(path)
}
