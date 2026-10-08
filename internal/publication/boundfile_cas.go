//go:build linux || darwin

package publication

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"runtime"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

var ErrStalePredecessor = errors.New("bound file stale predecessor")
var ErrLockSubstitution = errors.New("bound file CAS lock substitution")

const (
	CASOutcomeNotCommitted                = "NOT_COMMITTED"
	CASOutcomeCommitted                   = "COMMITTED"
	CASOutcomeCommittedVerificationFailed = "COMMITTED_VERIFICATION_FAILED"
)

type BoundFilePredecessor struct {
	Absent     bool
	Selector   string
	Digest     string
	ByteLength uint64
}

type CompareAndReplaceReceipt struct {
	*BoundFileReceipt
	Committed bool
	Outcome   string
}

func DigestForTest(raw []byte) string {
	sum := sha256.Sum256(raw)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func CompareAndReplaceBoundFile(ctx context.Context, root *Root, selector string, predecessor BoundFilePredecessor, raw []byte, verify func([]byte) error) (*CompareAndReplaceReceipt, error) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		return nil, errors.New("bound file CAS unsupported platform")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if root == nil || raw == nil || verify == nil || root.ValidatePrivate() != nil {
		return nil, errors.New("invalid bound file CAS request")
	}
	if _, err := capabilityTarget(root, selector); err != nil {
		return nil, err
	}
	unlock, err := root.casLock(ctx, selector)
	if err != nil {
		return nil, err
	}
	defer unlock()
	existing, err := root.ReadSelector(selector, 1<<20)
	if err != nil {
		if !predecessor.Absent {
			return &CompareAndReplaceReceipt{}, ErrStalePredecessor
		}
	} else {
		if predecessor.Absent || predecessor.Selector != selector || predecessor.Digest != DigestForTest(existing) || predecessor.ByteLength != uint64(len(existing)) {
			return &CompareAndReplaceReceipt{}, ErrStalePredecessor
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	temp := selector + fmt.Sprintf(".%d.tmp", time.Now().UnixNano())
	receipt, err := PublishBoundFile(root, temp, raw, verify)
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return &CompareAndReplaceReceipt{BoundFileReceipt: receipt, Committed: false, Outcome: CASOutcomeNotCommitted}, err
	}
	renameCommitted, directoryStatus, err := renameBoundFileSibling(root, temp, selector)
	if err != nil && !renameCommitted {
		return nil, err
	}
	if renameCommitted && directoryStatus != "" {
		receipt.DirectorySyncStatus = directoryStatus
		receipt.CrashDurability = directoryStatus
	}
	receipt.FinalSelector = selector
	verifyErr := error(nil)
	if got, err := root.ReadSelector(selector, int64(len(raw))+1); err != nil || !bytes.Equal(got, raw) {
		verifyErr = errors.New("bound file CAS final verification failed")
	} else if err := verify(got); err != nil {
		verifyErr = err
	}
	if cerr := ctx.Err(); cerr != nil {
		verifyErr = errors.Join(verifyErr, cerr)
	}
	if verifyErr != nil {
		receipt.VerificationStatus = CASOutcomeCommittedVerificationFailed
		return &CompareAndReplaceReceipt{BoundFileReceipt: receipt, Committed: true, Outcome: CASOutcomeCommittedVerificationFailed}, errors.Join(err, verifyErr)
	}
	return &CompareAndReplaceReceipt{BoundFileReceipt: receipt, Committed: true, Outcome: CASOutcomeCommitted}, err
}

func (r *Root) casLock(ctx context.Context, selector string) (func(), error) {
	parentFD, _, err := boundParentFD(r, selector)
	if err != nil {
		return nil, err
	}
	lockFD, err := openStableCASLock(parentFD, r.info)
	if err != nil {
		_ = closeBoundRootFD(parentFD)
		return nil, err
	}
	for {
		if err := ctx.Err(); err != nil {
			_ = unix.Close(lockFD)
			_ = closeBoundRootFD(parentFD)
			return nil, err
		}
		if substituted, err := casLockSubstituted(parentFD, lockFD, r.info); err != nil || substituted {
			_ = unix.Close(lockFD)
			_ = closeBoundRootFD(parentFD)
			if err == nil {
				err = ErrLockSubstitution
			}
			return nil, err
		}
		if err := unix.Flock(lockFD, unix.LOCK_EX|unix.LOCK_NB); err == nil {
			if substituted, err := casLockSubstituted(parentFD, lockFD, r.info); err != nil || substituted {
				_ = unix.Flock(lockFD, unix.LOCK_UN)
				_ = unix.Close(lockFD)
				_ = closeBoundRootFD(parentFD)
				if err == nil {
					err = ErrLockSubstitution
				}
				return nil, err
			}
			return func() {
				_, _ = casLockSubstituted(parentFD, lockFD, r.info)
				_ = unix.Flock(lockFD, unix.LOCK_UN)
				_ = unix.Close(lockFD)
				_ = closeBoundRootFD(parentFD)
			}, nil
		} else if !errors.Is(err, unix.EWOULDBLOCK) && !errors.Is(err, unix.EAGAIN) {
			_ = unix.Close(lockFD)
			_ = closeBoundRootFD(parentFD)
			return nil, err
		}
		time.Sleep(time.Millisecond)
	}
}

func renameBoundFileSibling(root *Root, tempSelector, finalSelector string) (bool, string, error) {
	tempFD, tempName, err := boundParentFD(root, tempSelector)
	if err != nil {
		return false, DirectorySyncNotAttemptedPostCommit, err
	}
	defer closeBoundRootFD(tempFD)
	finalFD, finalName, err := boundParentFD(root, finalSelector)
	if err != nil {
		return false, DirectorySyncNotAttemptedPostCommit, err
	}
	defer closeBoundRootFD(finalFD)
	if tempFD != finalFD {
		// Descriptor numbers differ even for the same directory; require stable same parent by fstat.
		var a, b unix.Stat_t
		if unix.Fstat(tempFD, &a) != nil || unix.Fstat(finalFD, &b) != nil || a.Dev != b.Dev || a.Ino != b.Ino {
			return false, DirectorySyncNotAttemptedPostCommit, errors.New("bound file CAS rename requires same pinned parent")
		}
	}
	if err := unix.Renameat(tempFD, tempName, finalFD, finalName); err != nil {
		return false, DirectorySyncNotAttemptedPostCommit, err
	}
	if err := syncBoundDirectory(finalFD); err != nil {
		return true, DirectorySyncFailed, errors.Join(errors.New("bound file CAS committed directory sync failed"), err)
	}
	return true, DirectorySyncComplete, nil
}

func openStableCASLock(parentFD int, rootInfo os.FileInfo) (int, error) {
	fd, err := unix.Openat(parentFD, ".lsp-trace-candidate-publication.lock", unix.O_RDWR|unix.O_CREAT|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0o600)
	if err != nil {
		return -1, err
	}
	if substituted, err := casLockSubstituted(parentFD, fd, rootInfo); err != nil || substituted {
		_ = unix.Close(fd)
		if err == nil {
			err = ErrLockSubstitution
		}
		return -1, err
	}
	return fd, nil
}

func casLockSubstituted(parentFD, lockFD int, rootInfo os.FileInfo) (bool, error) {
	var path, fd unix.Stat_t
	if err := unix.Fstatat(parentFD, ".lsp-trace-candidate-publication.lock", &path, unix.AT_SYMLINK_NOFOLLOW); err != nil {
		return false, errors.Join(ErrLockSubstitution, err)
	}
	if err := unix.Fstat(lockFD, &fd); err != nil {
		return false, errors.Join(ErrLockSubstitution, err)
	}
	if path.Dev != fd.Dev || path.Ino != fd.Ino {
		return true, nil
	}
	if !validCASLockMetadata(rootInfo, &fd) || !validCASLockMetadata(rootInfo, &path) {
		return false, errors.Join(ErrLockSubstitution, errors.New("unsafe lock metadata"))
	}
	return false, nil
}

func validCASLockMetadata(rootInfo os.FileInfo, st *unix.Stat_t) bool {
	rootStat, ok := rootInfo.Sys().(*syscall.Stat_t)
	if !ok || st == nil {
		return false
	}
	if st.Mode&unix.S_IFMT != unix.S_IFREG || st.Mode&0o022 != 0 || st.Nlink != 1 || st.Size != 0 {
		return false
	}
	return st.Uid == uint32(os.Geteuid()) || st.Uid == rootStat.Uid
}

func maxLen(a, b uint64) uint64 {
	if a > b {
		return a
	}
	return b
}
