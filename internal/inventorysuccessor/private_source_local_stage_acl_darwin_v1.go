//go:build darwin

package inventorysuccessor

import (
	"encoding/binary"
	"os"
	"unsafe"

	"golang.org/x/sys/unix"
)

// Descriptor-bound Darwin extended security attribute, not a pathname ACL
// lookup or a new security authority. Unknown, oversized, or granting ACLs
// refuse. Deny-only entries (including the usual home-directory delete denial)
// cannot grant another UID namespace mutation permission.
func localDirectoryACLAdmissionV1(f *os.File, b *[4096]byte) error {
	if b == nil {
		return ErrPrivateSourceLocalV1
	}
	clear(b[:])
	attrs := struct {
		Count, Reserved                       uint16
		Common, Volume, Directory, File, Fork uint32
	}{Count: 5, Common: 0x00400000} // ATTR_CMN_EXTENDED_SECURITY
	_, _, errno := unix.Syscall6(unix.SYS_FGETATTRLIST, f.Fd(), uintptr(unsafe.Pointer(&attrs)), uintptr(unsafe.Pointer(&b[0])), uintptr(len(b)), 0, 0)
	if errno != 0 {
		return ErrPrivateSourceLocalV1
	}
	return admitLocalDarwinACLBytesV1(b[:])
}

func admitLocalDarwinACLBytesV1(b []byte) error {
	if len(b) < 12 {
		return ErrPrivateSourceLocalV1
	}
	u := binary.NativeEndian.Uint32
	size, off, n := uint64(u(b)), int64(int32(u(b[4:]))), uint64(u(b[8:]))
	if size < 12 || size > uint64(len(b)) {
		return ErrPrivateSourceLocalV1
	}
	if n == 0 {
		if size == 12 {
			return nil
		}
		return ErrPrivateSourceLocalV1
	}
	start := int64(4) + off // attrreference offset is relative to its own field
	if start < 12 || uint64(start) > size || n > size-uint64(start) || n < 44 {
		return ErrPrivateSourceLocalV1
	}
	sec := b[int(start):int(uint64(start)+n)]
	if u(sec) != 0x012cc16d {
		return ErrPrivateSourceLocalV1
	} // KAUTH_FILESEC_MAGIC
	count := u(sec[36:])
	if count == 0xffffffff {
		if n == 44 {
			return nil
		}
		return ErrPrivateSourceLocalV1
	} // KAUTH_FILESEC_NOACL
	if count > 128 || n != 44+24*uint64(count) {
		return ErrPrivateSourceLocalV1
	}
	for i := uint32(0); i < count; i++ {
		flags := u(sec[44+24*i+16:])
		if flags&0xf != 2 {
			return ErrPrivateSourceLocalV1
		} // KAUTH_ACE_DENY only
	}
	return nil
}
