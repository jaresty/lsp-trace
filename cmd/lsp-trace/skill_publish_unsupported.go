//go:build !darwin && !linux && !windows

package main

import (
	"errors"
	"os"
)

func renameDirectoryNoReplace(_, _, _ *os.File, _, _, _ string) error {
	return errors.New("skill directory export unsupported on this platform")
}
