//go:build unix

package publication

import (
	"errors"
	"os"
	"syscall"
)

func validateRootOwner(info os.FileInfo) error {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return errors.New("publication root ownership unavailable")
	}
	if stat.Uid != uint32(os.Geteuid()) {
		return ErrRootOwner
	}
	return nil
}
