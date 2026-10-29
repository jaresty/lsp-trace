package privatedescribeprep

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"lsp-trace/internal/publication"
)

const PostPublicationProfilePrivateV2 = "POST_PREPARATION_V2_PRIVATE"
const PostPublicationPolicyPrivateV2 = "POST_PUBLICATION_POLICY_V2:object=16777216;logical=67108864;readhash=268435456;attempts=2;actual-producer-roots=2..16;qualified-packet-pairs<=8;authority=0;accepted=false;absence=PROHIBITED;namespace-not-atomic-pair;no-powerloss-guarantee"

type PostPublicationOutcomePrivateV2 struct {
	PostPublicationOutcome
	Status string
}
type postPublicationEnvelopePrivateV2 struct {
	SchemaVersion string
	PrivateCeilings
	Absence, Profile, PolicySHA256, PreparationSHA256 string
	Preparation                                       json.RawMessage
	Ledger                                            StageLedgerPrivateV2
}

func encodePostPublicationPrivateV2(prepared []byte, ledger StageLedgerPrivateV2, sha string) ([]byte, error) {
	if len(prepared) == 0 || len(prepared) > int(publication.PostPreparationPrivateV2ObjectLimit) || !json.Valid(prepared) || postPublicationDigest(prepared) != sha || ledger.RequestType != RequestTypePrivateV2 {
		return nil, fmt.Errorf("private V2 publication preparation pins")
	}
	raw, err := json.Marshal(postPublicationEnvelopePrivateV2{"post-preparation-publication.private.v2", privateCeilings(), "PROHIBITED", PostPublicationProfilePrivateV2, postPublicationDigest([]byte(PostPublicationPolicyPrivateV2)), sha, json.RawMessage(prepared), ledger})
	if err != nil {
		return nil, err
	}
	if len(raw) > int(publication.PostPreparationPrivateV2ObjectLimit) {
		return nil, fmt.Errorf("private V2 publication envelope bound")
	}
	return raw, nil
}
func defaultPostPublicationOpsPrivateV2() postPublicationOps {
	return postPublicationOps{
		read: func(ctx context.Context, r *publication.ContentRetentionRoot, id string, n int64) ([]byte, error) {
			return r.ReadPostPreparationPrivateV2(ctx, id, n)
		},
		publish: func(ctx context.Context, r *publication.ContentRetentionRoot, id string, b []byte) (*publication.BoundFileReceipt, error) {
			return r.PublishPostPreparationPrivateV2(ctx, id, b)
		},
	}
}

// RetainPostPreparationPrivateV2 takes only the sealed owner and prospective
// caller assertions. The ledger is computed internally, never imported. Borrowed
// roots are NONCONCURRENT and must remain open until the caller closes them.
func RetainPostPreparationPrivateV2(ctx context.Context, h PostPreparationPrivateV2, subs []CallerSubmission, reviews []CallerReview, outputs [2]*publication.ContentRetentionRoot, protected []*publication.ObservedRoot) (PostPublicationOutcomePrivateV2, error) {
	return retainPostPreparationPrivateV2(ctx, h, subs, reviews, outputs, protected, defaultPostPublicationOpsPrivateV2())
}
func retainPostPreparationPrivateV2(ctx context.Context, h PostPreparationPrivateV2, subs []CallerSubmission, reviews []CallerReview, outputs [2]*publication.ContentRetentionRoot, protected []*publication.ObservedRoot, ops postPublicationOps) (out PostPublicationOutcomePrivateV2, err error) {
	out.PostPublicationOutcome = PostPublicationOutcome{PrivateCeilings: privateCeilings(), Absence: "PROHIBITED", Phase: "PREFLIGHT", Profile: PostPublicationProfilePrivateV2, PolicySHA256: postPublicationDigest([]byte(PostPublicationPolicyPrivateV2)), MaxLogicalBytes: 64 << 20, MaxReadHashBytes: 256 << 20, MaxCopyAttempts: 2}
	out.Status = "NOT_VERIFIED"
	for i := range out.Copies {
		out.Copies[i].Outcome = "NOT_ATTEMPTED"
	}
	if err = postContext(ctx); err != nil {
		return out, err
	}
	if h.state == nil || ops.read == nil || ops.publish == nil {
		return out, fmt.Errorf("unsealed private V2 publication owner")
	}
	if err = h.checkProtectedRoots(ctx, protected); err != nil {
		return out, err
	}
	if err = publication.CheckPostPreparationRetentionTopologyPrivateV2(ctx, outputs, protected); err != nil {
		return out, err
	}
	estimate, _, err := h.state.preflightCall(ctx, subs, reviews)
	if err != nil {
		return out, err
	}
	remaining := int(publication.PostPreparationPrivateV2ObjectLimit)
	for _, n := range []int{postPublicationOverhead, len(h.state.prepared), estimate} {
		if n < 0 || n > remaining {
			return out, fmt.Errorf("private V2 publication envelope estimate")
		}
		remaining -= n
	}
	ledger, err := h.Reconcile(ctx, subs, reviews)
	if err != nil {
		return out, err
	}
	for _, row := range ledger.Targets {
		if row.SubmissionStatus == "REJECTED" || row.ReviewStatus == "REJECTED" {
			return out, fmt.Errorf("private V2 publication rejected caller input: %s / %s", row.SubmissionReason, row.ReviewReason)
		}
	}
	raw, err := encodePostPublicationPrivateV2(h.state.prepared, ledger, h.state.sha)
	if err != nil {
		return out, err
	}
	logical, readhash := out.MaxLogicalBytes, out.MaxReadHashBytes
	for i := 0; i < 4; i++ {
		if uint64(len(raw)) > logical {
			return out, fmt.Errorf("private V2 logical byte ceiling")
		}
		logical -= uint64(len(raw))
	}
	for i := 0; i < 16; i++ {
		if uint64(len(raw)) > readhash {
			return out, fmt.Errorf("private V2 read/hash byte ceiling")
		}
		readhash -= uint64(len(raw))
	}
	out.envelope = bytes.Clone(raw)
	out.EnvelopeSHA256 = postPublicationDigest(raw)
	out.ByteLength = uint64(len(raw))
	out.PreparationSHA256 = h.state.sha
	checkAliases := func() error {
		for _, object := range h.objects {
			for _, root := range protected {
				path, id, e := root.CanonicalIdentityPrivateV2()
				if e != nil {
					return e
				}
				if object.Root == (producerRootBindingPrivateV2{path, id.Device, id.Inode}) {
					if e = publication.CheckPostPreparationInputObjectAliasPrivateV2(ctx, outputs, out.EnvelopeSHA256, root, object.Selector); e != nil {
						return e
					}
				}
			}
		}
		return nil
	}
	if err = checkAliases(); err != nil {
		return out, err
	}
	// Only pure absence authorizes a new write; exact pre-existing copies may
	// be verified. One write attempt per root; late ErrExist is failure. Exact
	// readback twice per root and close-aware per-copy receipts remain required.
	out.PostPublicationOutcome, err = retainPostPublicationPair(ctx, out.PostPublicationOutcome, outputs, ops)
	if err != nil {
		return out, err
	}
	out.Phase = "FINAL_VALIDATION"
	if err = h.checkProtectedRoots(ctx, protected); err != nil {
		return out, err
	}
	if err = publication.CheckPostPreparationRetentionTopologyPrivateV2(ctx, outputs, protected); err != nil {
		return out, err
	}
	if err = publication.CheckPostPreparationObjectsDistinctPrivateV2(ctx, outputs, out.EnvelopeSHA256); err != nil {
		return out, err
	}
	if err = checkAliases(); err != nil {
		return out, err
	}
	if err = ctx.Err(); err != nil {
		return out, err
	}
	out.Phase = "PAIR_VERIFIED"
	out.Status = "VERIFIED_PAIR_PRIVATE_WITH_LIMITS"
	return out, nil
}
