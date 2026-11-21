//go:build darwin || linux

package publication

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"time"
)

var ErrTimedCompletion = errors.New("timely verified completion not established")

type timedCompletion struct {
	Version              string
	Candidate, Digest    string
	Bytes                int
	VerifiedAt, Deadline int64
}

func receiptComplete(r *BoundFileReceipt) bool {
	return r != nil && r.NamespaceAtomic && r.VerificationStatus == "VERIFIED" && r.DirectorySyncStatus == DirectorySyncComplete && r.CloseStatus == CloseComplete
}

// prepareTimedCompletion commits a candidate, never an acceptance receipt. A
// crash or late return at any point here cannot make a reader accept the candidate.
func prepareTimedCompletion(ctx context.Context, root *Root, name string, binding []byte) error {
	r, e := CompareAndReplaceBoundFile(ctx, root, name, BoundFilePredecessor{Absent: true}, binding, func(got []byte) error {
		if !bytes.Equal(got, binding) {
			return ErrTimedCompletion
		}
		return ctx.Err()
	})
	if e != nil || r == nil || !r.Committed || r.Outcome != CASOutcomeCommitted || !receiptComplete(r.BoundFileReceipt) {
		return errors.Join(ErrTimedCompletion, e)
	}
	return nil
}

// sealTimedCompletion records a fact about already-finished verified work, not
// permission for later work. The timestamp is taken AFTER candidate readback,
// fsync and close have returned. Candidate-only records are always unaccepted.
func sealTimedCompletion(ctx context.Context, root *Root, name string, binding []byte, deadline time.Time) error {
	now := time.Now().UTC()
	if ctx.Err() != nil || !now.Before(deadline) {
		return ErrTimedCompletion
	}
	proof := timedCompletion{"lsp-trace.timed-completion.v1", name, DigestForTest(binding), len(binding), now.UnixNano(), deadline.UnixNano()}
	raw, e := json.Marshal(proof)
	if e != nil {
		return e
	}
	r, e := PublishBoundFile(root, name+".receipt", raw, func(got []byte) error {
		if !bytes.Equal(got, raw) {
			return ErrTimedCompletion
		}
		return nil
	})
	if e != nil || !receiptComplete(r) {
		return errors.Join(ErrTimedCompletion, e)
	}
	return VerifyTimedCompletion(root, name, binding, deadline)
}
func PublishTimedCompletion(ctx context.Context, root *Root, name string, binding []byte, deadline time.Time) error {
	if ctx == nil {
		return ErrTimedCompletion
	}
	if d, ok := ctx.Deadline(); ok && d.Before(deadline) {
		deadline = d
	}
	ctx, cancel := context.WithDeadline(ctx, deadline)
	defer cancel()
	if e := prepareTimedCompletion(ctx, root, name, binding); e != nil {
		return e
	}
	return sealTimedCompletion(ctx, root, name, binding, deadline)
}

// maxDeadline is the persisted task/run upper bound. A shorter caller deadline
// may have governed the accepted completion. Expired current time is irrelevant.
func VerifyTimedCompletion(root *Root, name string, binding []byte, maxDeadline time.Time) error {
	raw, e := ReadVerifiedBoundFileChecked(root, name+".receipt", 4096)
	if e != nil {
		return ErrTimedCompletion
	}
	var p timedCompletion
	if json.Unmarshal(raw, &p) != nil {
		return ErrTimedCompletion
	}
	canonical, e := json.Marshal(p)
	if e != nil || !bytes.Equal(raw, canonical) || p.Version != "lsp-trace.timed-completion.v1" || p.Candidate != name || p.Digest != DigestForTest(binding) || p.Bytes != len(binding) || p.VerifiedAt <= 0 || p.VerifiedAt >= p.Deadline || p.Deadline > maxDeadline.UnixNano() {
		return ErrTimedCompletion
	}
	got, e := ReadVerifiedBoundFileChecked(root, name, int64(len(binding)))
	if e != nil || !bytes.Equal(got, binding) {
		return ErrTimedCompletion
	}
	return nil
}
