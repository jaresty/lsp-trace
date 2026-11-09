//go:build linux

package inventorysuccessor

import (
	"errors"
	"os"

	"golang.org/x/sys/unix"
)

// Linux is controlled-fixture only. Extended access ACLs are unsupported rather
// than interpreted as POSIX mode bits; an absent ACL uses the checked mode.
func localDirectoryACLAdmissionV1(f *os.File, b *[4096]byte) error {
	if b == nil {
		return ErrPrivateSourceLocalV1
	}
	clear(b[:])
	_, e := unix.Fgetxattr(int(f.Fd()), "system.posix_acl_access", b[:])
	if errors.Is(e, unix.ENODATA) || errors.Is(e, unix.ENOTSUP) {
		return nil
	}
	return ErrPrivateSourceLocalV1
}
