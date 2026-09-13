//go:build darwin || linux

package publication

import (
	"errors"
	"fmt"
	"os"
	"syscall"
)

func stableFileIdentity(info os.FileInfo) (string, error) {
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return "", errors.New("stable file identity unavailable")
	}
	return fmt.Sprintf("%d:%d", uint64(st.Dev), uint64(st.Ino)), nil
}
