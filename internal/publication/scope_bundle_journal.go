package publication

import (
	"bytes"
	"context"
	"errors"
	"reflect"
)

// RetentionDirectoryIdentity exposes scalars only, never an FD or root capability.
type RetentionDirectoryIdentity struct {
	Device uint64 `json:"device"`
	Inode  uint64 `json:"inode"`
}

func (r *ContentRetentionRoot) DirectoryIdentity() (RetentionDirectoryIdentity, error) {
	if err := r.ValidatePrivate(); err != nil {
		return RetentionDirectoryIdentity{}, err
	}
	v := reflect.ValueOf(r.observed.root.info.Sys())
	if v.Kind() == reflect.Pointer {
		v = v.Elem()
	}
	if !v.IsValid() || v.Kind() != reflect.Struct {
		return RetentionDirectoryIdentity{}, ErrObservedRootUnsupported
	}
	scalar := func(name string) (uint64, bool) {
		f := v.FieldByName(name)
		if !f.IsValid() {
			return 0, false
		}
		if f.CanUint() {
			return f.Uint(), true
		}
		if f.CanInt() && f.Int() >= 0 {
			return uint64(f.Int()), true
		}
		return 0, false
	}
	dev, ok := scalar("Dev")
	ino, ok2 := scalar("Ino")
	if !ok || !ok2 || ino == 0 {
		return RetentionDirectoryIdentity{}, ErrObservedRootUnsupported
	}
	return RetentionDirectoryIdentity{dev, ino}, nil
}

// The only mutable selector family admitted here is the finite private bundle
// head. Its immutable attempts have a separate, fixed root-level namespace.
// This is not a general selector, callback workflow, or raw-root escape hatch.
func scopeBundleHeadSelector(key string) (string, error) {
	if !observedContentID.MatchString(key) {
		return "", ErrContentObservation
	}
	return "feature-scope-bundle-head-private-v1-" + key[7:] + ".json", nil
}
func scopeBundleAttemptSelector(id string) (string, error) {
	if !observedContentID.MatchString(id) {
		return "", ErrContentObservation
	}
	return "feature-scope-bundle-attempt-private-v1-" + id[7:] + ".json", nil
}
func (r *ContentRetentionRoot) ReadScopeBundleReceipt(ctx context.Context, id string) ([]byte, error) {
	if err := r.ValidatePrivate(); err != nil {
		return nil, err
	}
	selector, err := scopeBundleAttemptSelector(id)
	if err != nil {
		return nil, err
	}
	raw, err := r.observed.root.readObservedFile(ctx, selector, 64<<10, contentReadOps{lstat: statObservedContent, open: openObservedContent}, false)
	if err != nil {
		return nil, err
	}
	if DigestForTest(raw) != id {
		return nil, ErrContentObservation
	}
	return raw, nil
}
func (r *ContentRetentionRoot) PublishScopeBundleReceipt(ctx context.Context, id string, raw []byte) (*BoundFileReceipt, error) {
	if ctx == nil || len(raw) == 0 || len(raw) > 64<<10 {
		return nil, ErrContentObservation
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := r.ValidatePrivate(); err != nil {
		return nil, err
	}
	selector, err := scopeBundleAttemptSelector(id)
	if err != nil || DigestForTest(raw) != id {
		return nil, ErrContentObservation
	}
	expected := bytes.Clone(raw)
	receipt, err := PublishBoundFile(r.observed.root, selector, expected, func(got []byte) error {
		if !bytes.Equal(got, expected) {
			return ErrContentObservation
		}
		return nil
	})
	return receipt, errors.Join(err, ctx.Err())
}
func (r *ContentRetentionRoot) ReadScopeBundleHead(ctx context.Context, key string) ([]byte, error) {
	if err := r.ValidatePrivate(); err != nil {
		return nil, err
	}
	selector, err := scopeBundleHeadSelector(key)
	if err != nil {
		return nil, err
	}
	return r.observed.root.readObservedFile(ctx, selector, 64<<10, contentReadOps{lstat: statObservedContent, open: openObservedContent}, false)
}

// CAS compares exact predecessor bytes. The returned receipt is authoritative
// about known namespace commit even alongside cancellation or cleanup failure.
func (r *ContentRetentionRoot) CompareScopeBundleHead(ctx context.Context, key string, previous, replacement []byte, verify func([]byte) error) (*CompareAndReplaceReceipt, error) {
	if ctx == nil || verify == nil || len(replacement) == 0 || len(replacement) > 64<<10 || len(previous) > 64<<10 {
		return nil, ErrContentObservation
	}
	if err := r.ValidatePrivate(); err != nil {
		return nil, err
	}
	selector, err := scopeBundleHeadSelector(key)
	if err != nil {
		return nil, err
	}
	predecessor := BoundFilePredecessor{Absent: previous == nil}
	if previous != nil {
		predecessor.Selector = selector
		predecessor.Digest = DigestForTest(previous)
		predecessor.ByteLength = uint64(len(previous))
	}
	expected := bytes.Clone(replacement)
	return CompareAndReplaceBoundFile(ctx, r.observed.root, selector, predecessor, expected, func(got []byte) error {
		if !bytes.Equal(got, expected) {
			return ErrContentObservation
		}
		return verify(got)
	})
}
