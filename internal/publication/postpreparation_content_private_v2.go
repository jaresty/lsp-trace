package publication

import (
	"context"
	"errors"
	"os"
)

// This is an additive output family; neither V1 nor producer packet limits change.
const PostPreparationPrivateV2ObjectLimit int64 = 16 << 20

func (r *ContentRetentionRoot) ReadPostPreparationPrivateV2(ctx context.Context, id string, limit int64) ([]byte, error) {
	return r.readFixedContent(ctx, id, limit, PostPreparationPrivateV2ObjectLimit)
}
func (r *ContentRetentionRoot) PublishPostPreparationPrivateV2(ctx context.Context, id string, raw []byte) (*BoundFileReceipt, error) {
	return r.publishFixedContent(ctx, id, raw, PostPreparationPrivateV2ObjectLimit)
}

// CanonicalIdentityPrivateV2 observes only this validated pinned root's scalars.
// No descriptor, arbitrary filesystem access, or authenticity claim escapes.
func (r *ObservedRoot) CanonicalIdentityPrivateV2() (string, RetentionDirectoryIdentity, error) {
	id, err := r.DirectoryIdentity()
	if err != nil {
		return "", RetentionDirectoryIdentity{}, err
	}
	return r.root.path, id, nil
}

func CheckPostPreparationRetentionTopologyPrivateV2(ctx context.Context, outputs [2]*ContentRetentionRoot, protected []*ObservedRoot) error {
	if len(protected) < 2 || len(protected) > 16 {
		return ErrObservedRootAdmission
	}
	return checkPostPreparationRetentionTopologyV1Private(ctx, outputs, protected, defaultObservedRootOps())
}
func CheckPostPreparationObjectsDistinctPrivateV2(ctx context.Context, outputs [2]*ContentRetentionRoot, id string) error {
	_, err := observeContentPairCustodyPrivate(ctx, outputs, id, PostPreparationPrivateV2ObjectLimit)
	return err
}

// CheckPostPreparationInputObjectAliasPrivateV2 checks exact named CAS objects
// by metadata only. It does not scan a root or read input contents. The caller's
// versioned sealed owner supplies each original producer selector independently.
func CheckPostPreparationInputObjectAliasPrivateV2(ctx context.Context, outputs [2]*ContentRetentionRoot, outputID string, input *ObservedRoot, inputID string) (err error) {
	if ctx == nil || !observedContentID.MatchString(outputID) || !observedContentID.MatchString(inputID) {
		return ErrContentObservation
	}
	if err = input.ValidatePrivate(); err != nil {
		return err
	}
	var files []*os.File
	defer func() {
		for _, f := range files {
			err = errors.Join(err, f.Close())
		}
		err = errors.Join(err, ctx.Err())
	}()
	open := func(r *ObservedRoot, id string) (os.FileInfo, error) {
		if e := r.ValidatePrivate(); e != nil {
			return nil, e
		}
		if e := ctx.Err(); e != nil {
			return nil, e
		}
		f, e := r.root.handle.Open(id)
		if f != nil {
			files = append(files, f)
		}
		if e != nil {
			return nil, e
		}
		info, e := f.Stat()
		if e != nil {
			return nil, e
		}
		named, e := r.root.handle.Lstat(id)
		if e != nil {
			return nil, e
		}
		if !r.root.validObservedInfo(info) || !r.root.validObservedInfo(named) || !sameObservedInfo(info, named) {
			return nil, ErrContentObservation
		}
		return info, nil
	}
	in, e := open(input, inputID)
	if e != nil {
		return e
	}
	for _, r := range outputs {
		if e = r.ValidatePrivate(); e != nil {
			return e
		}
		info, e := open(r.observed, outputID)
		if e != nil {
			if pureMissingPostPrivateV2(e) {
				continue
			}
			return e
		}
		if os.SameFile(in, info) {
			return ErrContentObservation
		}
	}
	return nil
}

// Only a simple wrapped absence permits a not-yet-published output; joined
// close/cancel/corruption errors never do. Close errors are joined by the owner.
func pureMissingPostPrivateV2(err error) bool {
	for depth := 0; depth < 16; depth++ {
		if err == os.ErrNotExist {
			return true
		}
		if _, ok := err.(interface{ Unwrap() []error }); ok {
			return false
		}
		next := errors.Unwrap(err)
		if next == nil {
			return errors.Is(err, os.ErrNotExist)
		}
		err = next
	}
	return false
}
