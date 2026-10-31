package publication

import (
	"context"
	"errors"
	"os"
)

// Separate fixed accounting family. Never a Native/producer qualification.
const TerminalDiscoveryPrivateV2ObjectLimit int64 = 1 << 20

func (r *ContentRetentionRoot) ReadTerminalDiscoveryPrivateV2(ctx context.Context, id string, limit int64) ([]byte, error) {
	return r.readFixedContent(ctx, id, limit, TerminalDiscoveryPrivateV2ObjectLimit)
}
func (r *ContentRetentionRoot) PublishTerminalDiscoveryPrivateV2(ctx context.Context, id string, raw []byte) (*BoundFileReceipt, error) {
	return r.publishFixedContent(ctx, id, raw, TerminalDiscoveryPrivateV2ObjectLimit)
}

// Terminal-only final custody. The descriptor-relative platform opener uses
// O_RDONLY|O_NOFOLLOW|O_NONBLOCK|O_CLOEXEC on supported local filesystems. This
// rejects substituted FIFOs before any blocking IO and never borrows the 16MiB
// producer family. No namespace lease or arbitrary-filesystem latency is claimed.
func CheckTerminalDiscoveryObjectsDistinctPrivateV2(ctx context.Context, roots [2]*ContentRetentionRoot, id string) error {
	return observeTerminalPairCustodyPrivateV2(ctx, roots, id)
}
func observeTerminalPairCustodyPrivateV2(ctx context.Context, roots [2]*ContentRetentionRoot, id string) (err error) {
	if ctx == nil || !observedContentID.MatchString(id) {
		return ErrContentObservation
	}
	var opened [2]observedFile
	var infos [2]os.FileInfo
	defer func() {
		for _, f := range opened {
			if f != nil {
				err = errors.Join(err, f.Close())
			}
		}
		err = errors.Join(err, ctx.Err())
	}()
	for i, r := range roots {
		if err = r.ValidatePrivate(); err != nil {
			return err
		}
		if err = ctx.Err(); err != nil {
			return err
		}
		opened[i], err = openObservedContent(r.observed.root, id)
		if err != nil {
			return err
		}
		infos[i], err = opened[i].Stat()
		if err != nil {
			return err
		}
		if !r.observed.root.validObservedInfo(infos[i]) || infos[i].Size() < 1 || infos[i].Size() > TerminalDiscoveryPrivateV2ObjectLimit {
			return ErrContentObservation
		}
		named, e := statObservedContent(r.observed.root, id)
		if e != nil {
			return e
		}
		if !r.observed.root.validObservedInfo(named) || !sameObservedInfo(named, infos[i]) {
			return ErrContentObservation
		}
		if err = r.ValidatePrivate(); err != nil {
			return err
		}
	}
	if os.SameFile(infos[0], infos[1]) {
		return ErrContentObservation
	}
	return ctx.Err()
}
