//go:build !linux && !darwin

package publication

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
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

func CompareAndReplaceBoundFile(context.Context, *Root, string, BoundFilePredecessor, []byte, func([]byte) error) (*CompareAndReplaceReceipt, error) {
	return nil, errors.New("bound file CAS unsupported platform")
}
