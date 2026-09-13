//go:build darwin

package publication

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"os"

	"golang.org/x/sys/unix"
)

func publishExactFD(root *Root, selector string, raw []byte, verify func([]byte) error) (bool, string, string, error) {
	parentFD, name, err := boundParentFD(root, selector)
	if err != nil {
		return false, DirectorySyncNotAttemptedPostCommit, CloseNotAttempted, err
	}
	committed := false
	defer func() {
		if !committed {
			_ = closeBoundRootFD(parentFD)
		}
	}()
	sourceFD := -1
	for attempt := 0; attempt < 32; attempt++ {
		var token [16]byte
		if _, err := rand.Read(token[:]); err != nil {
			return false, DirectorySyncNotAttemptedPostCommit, CloseNotAttempted, err
		}
		tmp := ".lsp-trace-bundle-" + hex.EncodeToString(token[:])
		sourceFD, err = unix.Openat(parentFD, tmp, unix.O_RDWR|unix.O_CREAT|unix.O_EXCL|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0o600)
		if err == nil {
			if unlinkErr := unix.Unlinkat(parentFD, tmp, 0); unlinkErr != nil {
				unix.Close(sourceFD)
				return false, DirectorySyncNotAttemptedPostCommit, CloseNotAttempted, unlinkErr
			}
			break
		}
		if !errors.Is(err, unix.EEXIST) {
			return false, DirectorySyncNotAttemptedPostCommit, CloseNotAttempted, err
		}
	}
	if sourceFD < 0 {
		return false, DirectorySyncNotAttemptedPostCommit, CloseNotAttempted, errors.New("temporary source collision limit reached")
	}
	f := os.NewFile(uintptr(sourceFD), "unlinked-capture-bundle")
	if err := prepareSource(f, raw, verify); err != nil {
		return false, DirectorySyncNotAttemptedPostCommit, CloseNotAttempted, errors.Join(err, closeBoundSource(f))
	}
	if err := unix.Fclonefileat(sourceFD, parentFD, name, unix.CLONE_NOOWNERCOPY); err != nil {
		if errors.Is(err, unix.EEXIST) {
			return false, DirectorySyncNotAttemptedPostCommit, CloseNotAttempted, errors.Join(os.ErrExist, closeBoundSource(f))
		}
		if errors.Is(err, unix.ENOTSUP) || errors.Is(err, unix.EXDEV) || errors.Is(err, unix.ENOSYS) {
			return false, DirectorySyncNotAttemptedPostCommit, CloseNotAttempted, errors.Join(errExactFDUnsupported, closeBoundSource(f))
		}
		return false, DirectorySyncNotAttemptedPostCommit, CloseNotAttempted, errors.Join(err, closeBoundSource(f))
	}
	committed = true
	directoryStatus := postcommitDirectorySyncStatus(true, syncBoundDirectory(parentFD))
	sourceCloseErr := closeBoundSource(f)
	rootCloseErr := closeBoundRootFD(parentFD)
	closeStatus := postcommitCloseStatus(true, errors.Join(sourceCloseErr, rootCloseErr))
	return true, directoryStatus, closeStatus, nil
}
