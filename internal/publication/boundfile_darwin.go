//go:build darwin

package publication

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"os"

	"golang.org/x/sys/unix"
)

func publishExactFD(root *Root, selector string, raw []byte, verify func([]byte) error) (bool, string, error) {
	parentFD, name, err := boundParentFD(root, selector)
	if err != nil {
		return false, "", err
	}
	defer unix.Close(parentFD)
	sourceFD := -1
	for attempt := 0; attempt < 32; attempt++ {
		var token [16]byte
		if _, err := rand.Read(token[:]); err != nil {
			return false, "", err
		}
		tmp := ".lsp-trace-bundle-" + hex.EncodeToString(token[:])
		sourceFD, err = unix.Openat(parentFD, tmp, unix.O_RDWR|unix.O_CREAT|unix.O_EXCL|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0o600)
		if err == nil {
			if unlinkErr := unix.Unlinkat(parentFD, tmp, 0); unlinkErr != nil {
				unix.Close(sourceFD)
				return false, "", unlinkErr
			}
			break
		}
		if !errors.Is(err, unix.EEXIST) {
			return false, "", err
		}
	}
	if sourceFD < 0 {
		return false, "", errors.New("temporary source collision limit reached")
	}
	f := os.NewFile(uintptr(sourceFD), "unlinked-capture-bundle")
	defer f.Close()
	if err := prepareSource(f, raw, verify); err != nil {
		return false, "", err
	}
	if err := unix.Fclonefileat(sourceFD, parentFD, name, unix.CLONE_NOOWNERCOPY); err != nil {
		if errors.Is(err, unix.EEXIST) {
			return false, "", os.ErrExist
		}
		if errors.Is(err, unix.ENOTSUP) || errors.Is(err, unix.EXDEV) || errors.Is(err, unix.ENOSYS) {
			return false, "", errExactFDUnsupported
		}
		return false, "", err
	}
	durability := "NOT_CHECKED_POST_COMMIT"
	if unix.Fsync(parentFD) == nil {
		durability = "FINAL_DIRECTORY_SYNCED_NO_CRASH_GUARANTEE"
	}
	return true, durability, nil
}
