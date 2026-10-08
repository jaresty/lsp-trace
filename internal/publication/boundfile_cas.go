package publication

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"runtime"
	"time"

	"golang.org/x/sys/unix"
)

var ErrStalePredecessor = errors.New("bound file stale predecessor")

type BoundFilePredecessor struct {
	Absent     bool
	Selector   string
	Digest     string
	ByteLength uint64
}

type CompareAndReplaceReceipt struct {
	*BoundFileReceipt
	Committed bool
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
		return &CompareAndReplaceReceipt{BoundFileReceipt: receipt, Committed: false}, err
	}
	if err := renameBoundFileSibling(root, temp, selector); err != nil {
		return nil, err
	}
	verifyErr := error(nil)
	if got, err := root.ReadSelector(selector, int64(len(raw))+1); err != nil || !bytes.Equal(got, raw) {
		verifyErr = errors.New("bound file CAS final verification failed")
	} else if err := verify(got); err != nil {
		verifyErr = err
	}
	receipt.FinalSelector = selector
	if verifyErr != nil {
		receipt.VerificationStatus = "COMMITTED_VERIFICATION_FAILED"
		return &CompareAndReplaceReceipt{BoundFileReceipt: receipt, Committed: true}, nil
	}
	return &CompareAndReplaceReceipt{BoundFileReceipt: receipt, Committed: true}, nil
}

func (r *Root) casLock(ctx context.Context, selector string) (func(), error) {
	parentFD, _, err := boundParentFD(r, selector)
	if err != nil {
		return nil, err
	}
	lockFD, err := unix.Openat(parentFD, ".lsp-trace-candidate-publication.lock", unix.O_RDWR|unix.O_CREAT|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0o600)
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
		if err := unix.Flock(lockFD, unix.LOCK_EX|unix.LOCK_NB); err == nil {
			return func() { _ = unix.Flock(lockFD, unix.LOCK_UN); _ = unix.Close(lockFD); _ = closeBoundRootFD(parentFD) }, nil
		} else if !errors.Is(err, unix.EWOULDBLOCK) && !errors.Is(err, unix.EAGAIN) {
			_ = unix.Close(lockFD)
			_ = closeBoundRootFD(parentFD)
			return nil, err
		}
		time.Sleep(time.Millisecond)
	}
}

func renameBoundFileSibling(root *Root, tempSelector, finalSelector string) error {
	tempFD, tempName, err := boundParentFD(root, tempSelector)
	if err != nil {
		return err
	}
	defer closeBoundRootFD(tempFD)
	finalFD, finalName, err := boundParentFD(root, finalSelector)
	if err != nil {
		return err
	}
	defer closeBoundRootFD(finalFD)
	if tempFD != finalFD {
		// Descriptor numbers differ even for the same directory; require stable same parent by fstat.
		var a, b unix.Stat_t
		if unix.Fstat(tempFD, &a) != nil || unix.Fstat(finalFD, &b) != nil || a.Dev != b.Dev || a.Ino != b.Ino {
			return errors.New("bound file CAS rename requires same pinned parent")
		}
	}
	if err := unix.Renameat(tempFD, tempName, finalFD, finalName); err != nil {
		return err
	}
	if err := syncBoundDirectory(finalFD); err != nil {
		return errors.Join(errors.New("bound file CAS committed directory sync failed"), err)
	}
	return nil
}

func maxLen(a, b uint64) uint64 {
	if a > b {
		return a
	}
	return b
}
