package publication

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"
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
	base := root.Path()
	if info, err := os.Lstat(base); err != nil || !os.SameFile(root.info, info) {
		return nil, errors.New("root identity changed")
	}
	finalPath := filepath.Join(base, filepath.FromSlash(selector))
	tempPath := filepath.Join(base, filepath.FromSlash(temp))
	if err := os.Rename(tempPath, finalPath); err != nil {
		return nil, err
	}
	if got, err := root.ReadSelector(selector, int64(len(raw))+1); err != nil || !bytes.Equal(got, raw) {
		return &CompareAndReplaceReceipt{BoundFileReceipt: receipt, Committed: true}, errors.New("bound file CAS final verification failed")
	}
	receipt.FinalSelector = selector
	return &CompareAndReplaceReceipt{BoundFileReceipt: receipt, Committed: true}, nil
}

func (r *Root) casLock(ctx context.Context, selector string) (func(), error) {
	lock := selector + ".lock"
	base := r.Path()
	lockPath := filepath.Join(base, filepath.FromSlash(lock))
	if err := os.MkdirAll(filepath.Dir(lockPath), 0o700); err != nil {
		return nil, err
	}
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err == nil {
			_ = f.Close()
			return func() { _ = os.Remove(lockPath) }, nil
		}
		if !errors.Is(err, os.ErrExist) {
			return nil, err
		}
		time.Sleep(time.Millisecond)
	}
}

func maxLen(a, b uint64) uint64 {
	if a > b {
		return a
	}
	return b
}
