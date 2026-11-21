//go:build darwin || linux

package publication

import (
	"bytes"
	"context"
	"errors"
	"sync/atomic"
)

// BoundFileCommit is process-local evidence of this invocation's successful CAS,
// not a caller receipt DTO. There is deliberately no reopen/JSON constructor.
// Copies share retirement. It does not by itself authorize any domain operation.
type BoundFileCommit struct{ state *boundFileCommit }
type boundFileCommit struct {
	used          atomic.Bool
	root          *Root
	selector      string
	before, after []byte
}

var ErrCommitCapability = errors.New("committed replacement unavailable or spent")

// CommitBoundFile creates the capability only after exact-predecessor CAS and
// verified durable readback. Even committed errors produce no capability.
func CommitBoundFile(ctx context.Context, root *Root, selector string, before, after []byte) (BoundFileCommit, error) {
	if ctx == nil || ctx.Err() != nil || root == nil || len(before) == 0 || len(before) > 65536 || len(after) == 0 || len(after) > 65536 {
		return BoundFileCommit{}, ErrCommitCapability
	}
	before, after = bytes.Clone(before), bytes.Clone(after)
	pred := BoundFilePredecessor{Selector: selector, Digest: DigestForTest(before), ByteLength: uint64(len(before))}
	r, e := CompareAndReplaceBoundFile(ctx, root, selector, pred, after, func(got []byte) error {
		if !bytes.Equal(got, after) {
			return ErrCommitCapability
		}
		return nil
	})
	if e != nil || r == nil || !r.Committed || r.Outcome != CASOutcomeCommitted || r.VerificationStatus != "VERIFIED" || r.DirectorySyncStatus != DirectorySyncComplete || r.CloseStatus != CloseComplete || ctx.Err() != nil {
		return BoundFileCommit{}, errors.Join(ErrCommitCapability, e)
	}
	return BoundFileCommit{&boundFileCommit{root: root, selector: selector, before: before, after: after}}, nil
}

// Take retires first, then rereads the actual selected bytes. A stale or failed
// read cannot be retried with this capability. Persisted bytes cannot remint it.
func (c BoundFileCommit) Take(ctx context.Context) (*Root, string, []byte, []byte, error) {
	if c.state == nil || !c.state.used.CompareAndSwap(false, true) {
		return nil, "", nil, nil, ErrCommitCapability
	}
	s := c.state
	if ctx == nil || ctx.Err() != nil {
		return nil, "", nil, nil, ErrCommitCapability
	}
	got, e := ReadVerifiedBoundFileCheckedContext(ctx, s.root, s.selector, int64(len(s.after)))
	if e != nil || !bytes.Equal(got, s.after) {
		return nil, "", nil, nil, ErrCommitCapability
	}
	return s.root, s.selector, bytes.Clone(s.before), bytes.Clone(s.after), nil
}
