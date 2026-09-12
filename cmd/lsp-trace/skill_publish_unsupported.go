//go:build !darwin && !linux && !windows

package main

import (
	"errors"
	"os"
)

func renameDirectoryNoReplace(_, _ *os.File, _, _ string) error {
	return errors.New("skill directory export unsupported on this platform")
}
