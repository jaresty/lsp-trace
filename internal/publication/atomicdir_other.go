//go:build !darwin && !linux

package publication

import (
	"errors"
	"os"
)

func renameNoReplace(_ int, _, _ string) error {
	return errors.New("atomic no-replace directory publication unsupported")
}
func nlink(os.FileInfo) uint64 { return 0 }
