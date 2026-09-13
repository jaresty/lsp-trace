//go:build linux

package publication

import (
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
	fd, err := unix.Openat(parentFD, ".", unix.O_RDWR|unix.O_TMPFILE|unix.O_CLOEXEC, 0o600)
	if err != nil {
		if errors.Is(err, unix.EOPNOTSUPP) || errors.Is(err, unix.ENOSYS) || errors.Is(err, unix.EINVAL) {
			return false, DirectorySyncNotAttemptedPostCommit, CloseNotAttempted, errExactFDUnsupported
		}
		return false, DirectorySyncNotAttemptedPostCommit, CloseNotAttempted, err
	}
	f := os.NewFile(uintptr(fd), "unnamed-capture-bundle")
	if err := prepareSource(f, raw, verify); err != nil {
		return false, DirectorySyncNotAttemptedPostCommit, CloseNotAttempted, errors.Join(err, closeBoundSource(f))
	}
	if err := unix.Linkat(fd, "", parentFD, name, unix.AT_EMPTY_PATH); err != nil {
		closeErr := closeBoundSource(f)
		if errors.Is(err, unix.EEXIST) {
			return false, DirectorySyncNotAttemptedPostCommit, CloseNotAttempted, errors.Join(os.ErrExist, closeErr)
		}
		if errors.Is(err, unix.EOPNOTSUPP) || errors.Is(err, unix.ENOSYS) || errors.Is(err, unix.EINVAL) || errors.Is(err, unix.EPERM) {
			return false, DirectorySyncNotAttemptedPostCommit, CloseNotAttempted, errors.Join(errExactFDUnsupported, closeErr)
		}
		return false, DirectorySyncNotAttemptedPostCommit, CloseNotAttempted, errors.Join(err, closeErr)
	}
	committed = true
	directoryStatus := postcommitDirectorySyncStatus(true, syncBoundDirectory(parentFD))
	sourceCloseErr := closeBoundSource(f)
	rootCloseErr := closeBoundRootFD(parentFD)
	closeStatus := postcommitCloseStatus(true, errors.Join(sourceCloseErr, rootCloseErr))
	return true, directoryStatus, closeStatus, nil
}
