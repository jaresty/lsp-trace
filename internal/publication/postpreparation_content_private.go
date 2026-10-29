package publication

import "context"

// PostPreparationV1PrivateObjectLimit is a fixed prospective output profile.
// It changes neither legacy input admission nor the partial-inspection profile.
const PostPreparationV1PrivateObjectLimit int64 = 16 << 20

func (r *ContentRetentionRoot) ReadPostPreparationV1Private(ctx context.Context, id string, limit int64) ([]byte, error) {
	return r.readFixedContent(ctx, id, limit, PostPreparationV1PrivateObjectLimit)
}

func (r *ContentRetentionRoot) PublishPostPreparationV1Private(ctx context.Context, id string, raw []byte) (*BoundFileReceipt, error) {
	return r.publishFixedContent(ctx, id, raw, PostPreparationV1PrivateObjectLimit)
}
