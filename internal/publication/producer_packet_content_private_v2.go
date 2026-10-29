package publication

import (
	"context"
	"errors"
	"os"
)

// ProducerPacketPrivateV2ObjectLimit is a new fixed family, not a widened legacy
// input or post-preparation profile. Serialized bytes include base64 overhead.
const ProducerPacketPrivateV2ObjectLimit int64 = 16 << 20

func (r *ContentRetentionRoot) ReadProducerPacketPrivateV2(ctx context.Context, id string, limit int64) ([]byte, error) {
	return r.readFixedContent(ctx, id, limit, ProducerPacketPrivateV2ObjectLimit)
}
func (r *ContentRetentionRoot) PublishProducerPacketPrivateV2(ctx context.Context, id string, raw []byte) (*BoundFileReceipt, error) {
	return r.publishFixedContent(ctx, id, raw, ProducerPacketPrivateV2ObjectLimit)
}

// Detached metadata observation only: no constructor or seal consumer accepts it
// as custody authority. The caps remain privately pinned and NONCONCURRENT.
type ProducerPacketCopyObservationPrivateV2 struct {
	RootPath         string `json:"root_path"`
	RootDevice       uint64 `json:"root_device"`
	RootInode        uint64 `json:"root_inode"`
	ObjectDevice     uint64 `json:"object_device"`
	ObjectInode      uint64 `json:"object_inode"`
	ObjectByteLength uint64 `json:"object_byte_length"`
	Selector         string `json:"selector"`
}

// This observes the named objects through both pinned roots, not their ambient
// directory paths. It is a custody boundary, not a content read, snapshot, or
// guarantee against later same-user mutation. Close/cancellation errors refuse.
func ObserveProducerPacketPairCustodyPrivateV2(ctx context.Context, roots [2]*ContentRetentionRoot, id string) (out [2]ProducerPacketCopyObservationPrivateV2, err error) {
	return observeContentPairCustodyPrivate(ctx, roots, id, ProducerPacketPrivateV2ObjectLimit)
}

// Shared metadata-only primitive; wrappers select fixed object families.
func observeContentPairCustodyPrivate(ctx context.Context, roots [2]*ContentRetentionRoot, id string, ceiling int64) (out [2]ProducerPacketCopyObservationPrivateV2, err error) {
	if ctx == nil || !observedContentID.MatchString(id) {
		return out, ErrContentObservation
	}
	var files [2]*os.File
	var infos [2]os.FileInfo
	defer func() {
		for _, f := range files {
			if f != nil {
				err = errors.Join(err, f.Close())
			}
		}
		err = errors.Join(err, ctx.Err())
		if err != nil {
			out = [2]ProducerPacketCopyObservationPrivateV2{}
		}
	}()
	for i, r := range roots {
		if err = r.ValidatePrivate(); err != nil {
			return out, err
		}
		if err = ctx.Err(); err != nil {
			return out, err
		}
		files[i], err = r.observed.root.handle.Open(id)
		if err != nil {
			return out, err
		}
		infos[i], err = files[i].Stat()
		if err != nil {
			return out, err
		}
		if !r.observed.root.validObservedInfo(infos[i]) || infos[i].Size() < 1 || infos[i].Size() > ceiling {
			return out, ErrContentObservation
		}
		named, e := r.observed.root.handle.Lstat(id)
		if e != nil {
			return out, e
		}
		if !sameObservedInfo(named, infos[i]) || !r.observed.root.validObservedInfo(named) {
			return out, ErrContentObservation
		}
		dev, ino, e := sourceProtectionIdentityPrivateV2(r.observed.root.info)
		if e != nil {
			return out, e
		}
		objectDev, objectIno, e := sourceProtectionIdentityPrivateV2(infos[i])
		if e != nil {
			return out, e
		}
		out[i] = ProducerPacketCopyObservationPrivateV2{r.observed.root.path, dev, ino, objectDev, objectIno, uint64(infos[i].Size()), id}
	}
	if os.SameFile(infos[0], infos[1]) {
		return out, ErrContentObservation
	}
	return out, nil
}
func CheckProducerPacketObjectsDistinctPrivateV2(ctx context.Context, roots [2]*ContentRetentionRoot, id string) error {
	_, err := ObserveProducerPacketPairCustodyPrivateV2(ctx, roots, id)
	return err
}
