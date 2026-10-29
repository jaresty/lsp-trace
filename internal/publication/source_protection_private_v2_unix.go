//go:build darwin || linux

package publication

import (
	"os"
	"syscall"
)

func sourceProtectionIdentityPrivateV2(info os.FileInfo) (uint64, uint64, error) {
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return 0, 0, ErrObservedRootUnsupported
	}
	return uint64(st.Dev), uint64(st.Ino), nil
}
