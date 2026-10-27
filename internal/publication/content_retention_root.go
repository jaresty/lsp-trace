package publication

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
)

// ContentRetentionObjectLimit bounds one fixed-pilot source/input object.
const ContentRetentionObjectLimit int64 = 2 << 20

// ContentRetentionRoot owns an admitted capability without exposing Root, raw
// descriptors, arbitrary selectors, CAS, or directory creation. Not concurrent
// with Close or other use. Construction performs no writes.
type ContentRetentionRoot struct{ observed *ObservedRoot }

func OpenContentRetentionRoot(ctx context.Context, path string) (*ContentRetentionRoot, error) {
	root, err := OpenObservedRoot(ctx, path)
	if err != nil {
		return nil, err
	}
	return &ContentRetentionRoot{root}, nil
}
func (r *ContentRetentionRoot) ValidatePrivate() error {
	if r == nil || r.observed == nil {
		return ErrObservedRootAdmission
	}
	return r.observed.ValidatePrivate()
}
func (r *ContentRetentionRoot) SameDirectory(other *ContentRetentionRoot) (bool, error) {
	if err := r.ValidatePrivate(); err != nil {
		return false, err
	}
	if err := other.ValidatePrivate(); err != nil {
		return false, err
	}
	return os.SameFile(r.observed.root.info, other.observed.root.info), nil
}
func (r *ContentRetentionRoot) Read(ctx context.Context, id string, limit int64) ([]byte, error) {
	if err := r.ValidatePrivate(); err != nil {
		return nil, err
	}
	if limit < 1 || limit > ContentRetentionObjectLimit {
		return nil, ErrContentObservation
	}
	return r.observed.ReadContentObserved(ctx, id, limit)
}

// Publish is one independent no-replace namespace operation. A receipt must be
// inspected even when err is nonnil; cancellation cannot erase a known commit.
// DirectorySyncComplete reports its existing NO_CRASH_GUARANTEE qualification.
func (r *ContentRetentionRoot) Publish(ctx context.Context, id string, raw []byte) (*BoundFileReceipt, error) {
	if ctx == nil {
		return nil, ErrContentObservation
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := r.ValidatePrivate(); err != nil {
		return nil, err
	}
	if len(raw) == 0 || int64(len(raw)) > ContentRetentionObjectLimit || !observedContentID.MatchString(id) || fmt.Sprintf("sha256:%x", sha256.Sum256(raw)) != id {
		return nil, ErrContentObservation
	}
	// Own the expected bytes across verifier callbacks.
	expected := bytes.Clone(raw)
	receipt, err := PublishBoundFile(r.observed.root, id, expected, func(got []byte) error {
		if !bytes.Equal(got, expected) {
			return ErrContentObservation
		}
		return nil
	})
	return receipt, errors.Join(err, ctx.Err())
}
func (r *ContentRetentionRoot) Close() error {
	if r == nil || r.observed == nil {
		return nil
	}
	observed := r.observed
	r.observed = nil
	return observed.Close()
}
