package privatedescribeprep

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"lsp-trace/internal/publication"
)

const postPublicationLimit = int(publication.PostPreparationV1PrivateObjectLimit)
const postPublicationOverhead = 4096
const postPublicationProfile = "POST_PREPARATION_V1_PRIVATE"

// Fixed, prospective accounting. Logical bytes include two copies and staging;
// read/hash allowance includes pre-reads, publication and four independent reads.
const postPublicationPolicy = "POST_PUBLICATION_POLICY_V1:logical=67108864;readhash=268435456;attempts=2;authority=0;accepted=false;absence=PROHIBITED"

// PostPublicationCopy is detached evidence, including receipts on failed calls.
// NAMESPACE_COMMITTED is not pair verification or a crash-durability guarantee.
type PostPublicationCopy struct {
	Outcome string
	Receipt *publication.BoundFileReceipt
}

type PostPublicationOutcome struct {
	PrivateCeilings
	Absence, Phase, Profile, PolicySHA256, PreparationSHA256, EnvelopeSHA256 string
	ByteLength                                                               uint64
	MaxLogicalBytes, MaxReadHashBytes                                        uint64
	MaxCopyAttempts                                                          int
	Copies                                                                   [2]PostPublicationCopy
	envelope                                                                 []byte
}

// EnvelopeBytes returns the exact bounded envelope even after a partial commit.
func (o PostPublicationOutcome) EnvelopeBytes() []byte { return bytes.Clone(o.envelope) }

// Only the sealed PostPreparation owns the producer bytes and derives the ledger.
// There is deliberately no StageLedger input, parser or DTO-to-seal admission.
type postPublicationEnvelope struct {
	Version string
	PrivateCeilings
	Absence, Profile, PolicySHA256, PreparationSHA256 string
	Preparation                                       json.RawMessage
	Ledger                                            StageLedger
}

func postPublicationDigest(raw []byte) string { return fmt.Sprintf("sha256:%x", sha256.Sum256(raw)) }

// encodePostPublicationPrivate assumes its ledger was already bounded by
// preflightCall; the production caller establishes that before Reconcile.
// Pure fixtures exercise encoding boundaries, not sealed producer qualification.
func encodePostPublicationPrivate(prepared []byte, ledger StageLedger, prepSHA string) ([]byte, error) {
	if len(prepared) == 0 || len(prepared) > postPublicationLimit || !json.Valid(prepared) {
		return nil, fmt.Errorf("private publication preparation bytes")
	}
	raw, err := json.Marshal(postPublicationEnvelope{
		Version: "post-preparation-publication.v1", PrivateCeilings: privateCeilings(), Absence: "PROHIBITED", Profile: postPublicationProfile,
		PolicySHA256: postPublicationDigest([]byte(postPublicationPolicy)), PreparationSHA256: prepSHA, Preparation: json.RawMessage(prepared), Ledger: ledger,
	})
	if err != nil {
		return nil, err
	}
	if len(raw) > postPublicationLimit {
		return nil, fmt.Errorf("private publication envelope bytes")
	}
	return raw, nil
}

// RetainPostPreparationPrivate is ONE object and two independent no-replace
// operations, never an atomic multi-object transaction or semantic admission.
// All roots are borrowed NONCONCURRENT; the owner must close every root after
// all calls and report close errors separately. No endpoint/source path is read.
func RetainPostPreparationPrivate(ctx context.Context, h PostPreparation, subs []CallerSubmission, reviews []CallerReview, outputs [2]*publication.ContentRetentionRoot, protectedInputs []*publication.ObservedRoot) (PostPublicationOutcome, error) {
	return retainPostPreparationPrivate(ctx, h, subs, reviews, outputs, protectedInputs, defaultPostPublicationOps())
}

type postPublicationOps struct {
	read    func(context.Context, *publication.ContentRetentionRoot, string, int64) ([]byte, error)
	publish func(context.Context, *publication.ContentRetentionRoot, string, []byte) (*publication.BoundFileReceipt, error)
}

func defaultPostPublicationOps() postPublicationOps {
	return postPublicationOps{
		read: func(ctx context.Context, r *publication.ContentRetentionRoot, id string, n int64) ([]byte, error) {
			return r.ReadPostPreparationV1Private(ctx, id, n)
		},
		publish: func(ctx context.Context, r *publication.ContentRetentionRoot, id string, b []byte) (*publication.BoundFileReceipt, error) {
			return r.PublishPostPreparationV1Private(ctx, id, b)
		},
	}
}
func retainPostPreparationPrivate(ctx context.Context, h PostPreparation, subs []CallerSubmission, reviews []CallerReview, outputs [2]*publication.ContentRetentionRoot, protected []*publication.ObservedRoot, ops postPublicationOps) (out PostPublicationOutcome, err error) {
	out = PostPublicationOutcome{PrivateCeilings: privateCeilings(), Absence: "PROHIBITED", Phase: "PREFLIGHT", Profile: postPublicationProfile, PolicySHA256: postPublicationDigest([]byte(postPublicationPolicy)), MaxLogicalBytes: 64 << 20, MaxReadHashBytes: 256 << 20, MaxCopyAttempts: 2}
	for i := range out.Copies {
		out.Copies[i].Outcome = "NOT_ATTEMPTED"
	}
	if err = postContext(ctx); err != nil {
		return out, err
	}
	if h.state == nil || ops.read == nil || ops.publish == nil {
		return out, fmt.Errorf("unsealed private publication handle or operations")
	}
	// Reserve before Reconcile's maps/clones/marshal. The ledger estimate includes
	// its own gaps and DTO duplication; preparation is independently embedded once.
	estimate, _, err := h.state.preflightCall(ctx, subs, reviews)
	if err != nil {
		return out, err
	}
	remaining := postPublicationLimit
	for _, n := range []int{postPublicationOverhead, len(h.state.prepared), estimate} {
		if n < 0 || n > remaining {
			return out, fmt.Errorf("private publication envelope estimate")
		}
		remaining -= n
	}
	for _, v := range subs {
		if !v.PrivateCeilings.valid() {
			return out, fmt.Errorf("private publication submission ceilings")
		}
	}
	for _, v := range reviews {
		if !v.PrivateCeilings.valid() {
			return out, fmt.Errorf("private publication review ceilings")
		}
	}
	if err = publication.CheckPostPreparationRetentionTopologyV1Private(ctx, outputs, protected); err != nil {
		return out, err
	}
	ledger, err := h.Reconcile(ctx, subs, reviews)
	if err != nil {
		return out, err
	}
	for _, row := range ledger.Targets {
		if row.SubmissionStatus == "REJECTED" || row.ReviewStatus == "REJECTED" {
			return out, fmt.Errorf("private publication rejected caller input: %s / %s", row.SubmissionReason, row.ReviewReason)
		}
	}
	raw, err := encodePostPublicationPrivate(h.state.prepared, ledger, h.state.sha)
	if err != nil {
		return out, err
	}
	// Fixed finite policy: subtraction (no unchecked aggregate multiplication).
	logical, readhash := out.MaxLogicalBytes, out.MaxReadHashBytes
	for i := 0; i < 4; i++ {
		if uint64(len(raw)) > logical {
			return out, fmt.Errorf("private logical byte ceiling")
		}
		logical -= uint64(len(raw))
	}
	for i := 0; i < 16; i++ {
		if uint64(len(raw)) > readhash {
			return out, fmt.Errorf("private read/hash ceiling")
		}
		readhash -= uint64(len(raw))
	}
	if err = ctx.Err(); err != nil {
		return out, err
	}
	out.envelope = bytes.Clone(raw)
	out.EnvelopeSHA256 = postPublicationDigest(raw)
	out.ByteLength = uint64(len(raw))
	out.PreparationSHA256 = h.state.sha
	return retainPostPublicationPair(ctx, out, outputs, ops)
}

// Finite error-tree traversal: joined cancellation/close errors never become
// absence admission. This is the concrete scope pair's classification boundary.
func postPublicationOnlyAbsent(err error, depth int) bool {
	if err == nil || depth > 16 {
		return false
	}
	if err == os.ErrNotExist {
		return true
	}
	if joined, ok := err.(interface{ Unwrap() []error }); ok {
		children := joined.Unwrap()
		if len(children) == 0 {
			return false
		}
		for _, child := range children {
			if !postPublicationOnlyAbsent(child, depth+1) {
				return false
			}
		}
		return true
	}
	if wrapped := errors.Unwrap(err); wrapped != nil {
		return postPublicationOnlyAbsent(wrapped, depth+1)
	}
	return errors.Is(err, os.ErrNotExist)
}
func retainPostPublicationPair(ctx context.Context, out PostPublicationOutcome, roots [2]*publication.ContentRetentionRoot, ops postPublicationOps) (PostPublicationOutcome, error) {
	raw, id := out.envelope, out.EnvelopeSHA256
	verify := func(r *publication.ContentRetentionRoot) error {
		got, e := ops.read(ctx, r, id, int64(len(raw)))
		if e != nil {
			return e
		}
		if !bytes.Equal(got, raw) || postPublicationDigest(got) != id {
			return fmt.Errorf("private publication exact bytes mismatch")
		}
		return ctx.Err()
	}
	names := [2]string{"PRIMARY", "BACKUP"}
	for i, r := range roots {
		out.Phase = names[i] + "_PRE_READ"
		if e := ctx.Err(); e != nil {
			return out, e
		}
		got, e := ops.read(ctx, r, id, int64(len(raw)))
		if e == nil {
			out.Copies[i].Outcome = "EXISTING_UNVERIFIED"
			if !bytes.Equal(got, raw) || postPublicationDigest(got) != id {
				return out, fmt.Errorf("private publication corrupt existing object")
			}
			if e = ctx.Err(); e != nil {
				return out, e
			}
			out.Copies[i].Outcome = "EXISTING_VERIFIED"
			continue
		}
		if !postPublicationOnlyAbsent(e, 0) {
			out.Copies[i].Outcome = "EXISTING_UNVERIFIED"
			return out, e
		}
		if e = ctx.Err(); e != nil {
			return out, e
		}
		out.Phase = names[i] + "_PUBLICATION"
		out.Copies[i].Outcome = "INDETERMINATE"
		receipt, e := ops.publish(ctx, r, id, bytes.Clone(raw))
		if receipt != nil {
			copy := *receipt
			out.Copies[i].Receipt = &copy
			if receipt.NamespaceAtomic {
				out.Copies[i].Outcome = "NAMESPACE_COMMITTED"
			}
		}
		// Any error after attempt stops: no ErrExist success readback or retry.
		if e != nil {
			return out, e
		}
		if receipt == nil || !receipt.NamespaceAtomic || receipt.FinalSelector != id || receipt.Digest != id || receipt.ByteLength != uint64(len(raw)) || receipt.Mechanism != publication.BoundFileMechanism || receipt.DirectorySyncStatus != publication.DirectorySyncComplete || receipt.CrashDurability != publication.DirectorySyncComplete || receipt.CloseStatus != publication.CloseComplete || receipt.VerificationStatus != "VERIFIED" {
			return out, fmt.Errorf("private publication inconsistent receipt")
		}
		if e = ctx.Err(); e != nil {
			return out, e
		}
	}
	for pass := 0; pass < 2; pass++ {
		for i, r := range roots {
			out.Phase = names[i] + "_REOPEN"
			if e := verify(r); e != nil {
				return out, e
			}
		}
	}
	out.Phase = "PAIR_VERIFIED"
	return out, nil
}
