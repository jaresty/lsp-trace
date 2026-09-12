//go:build unix

package publication

import (
	"errors"
	"os"
	"syscall"
)

func validateRootOwner(info os.FileInfo) error {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || stat.Uid != uint32(os.Geteuid()) {
		return errors.New("publication root must be owned by the current effective user")
	}
	return nil
}
