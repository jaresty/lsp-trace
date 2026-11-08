package inventorysuccessor

import (
	"bytes"
	"context"
	"errors"
	"time"

	"lsp-trace/internal/publication"
)

var ErrPrivateReadyProvisioningV2 = errors.New("private READY provisioning refused; partial publication may remain, do not retry or reset")

// PrivateReadyDecisionAssertionV2 is an explicit trusted-parent assertion. Its
// three hashes are bindings, NOT authentication, a signature, or approval proof.
// No decision is derived from the supplied job or authorization.
type PrivateReadyDecisionAssertionV2 struct{ Job, Authorization, Reference string }

// PrivateReadyInitializedV2 is immutable confirmation of the initial accounting
// head, not source custody, an execution permission, or additional credit. The
// zero value is invalid. The caller retains ownership of the borrowed Root.
type PrivateReadyInitializedV2 struct {
	job     []byte
	head    string
	expires int64
}

func (r *PrivateReadyInitializedV2) Valid() bool {
	return r != nil && len(r.job) != 0 && hashV2(r.head) && r.expires > 0
}
func (r *PrivateReadyInitializedV2) CanonicalJob() []byte {
	if !r.Valid() {
		return nil
	}
	return bytes.Clone(r.job)
}
func (r *PrivateReadyInitializedV2) ConfirmedHead() string {
	if !r.Valid() {
		return ""
	}
	return r.head
}

// CheckUsable checks the ORIGINAL authorization expiry, without renewing it. In
// particular the CLI calls this after its final owned Root.Close and before
// writing success. Valid alone describes immutable proof shape, not freshness.
func (r *PrivateReadyInitializedV2) CheckUsable(ctx context.Context) error {
	if !r.Valid() || ctx == nil || ctx.Err() != nil || time.Now().UnixNano() >= r.expires {
		return ErrPrivateReadyProvisioningV2
	}
	return nil
}

// InitializePrivateReadyV2 provisions only an explicitly supplied NEW_EMPTY job
// using the existing kernel. Verification limits bound verification; they are
// not authorization, funding, or a resource grant. All partial publications are
// retained on failure. No import, rollback, retry, reset, or refund is offered.
func InitializePrivateReadyV2(ctx context.Context, root *publication.Root, canonicalJob, canonicalAuthorization []byte, decision PrivateReadyDecisionAssertionV2, limits PrivateManagedInputVerificationV2) (*PrivateReadyInitializedV2, error) {
	return initializePrivateReadyV2(ctx, &kernelV2{root: root}, canonicalJob, canonicalAuthorization, decision, limits)
}

// Fixed production continuation, also permitting existing kernel instance-local
// CAS/publication fault seams in tests. No mutable global or executor hook.
func initializePrivateReadyV2(ctx context.Context, k *kernelV2, canonicalJob, canonicalAuthorization []byte, decision PrivateReadyDecisionAssertionV2, limits PrivateManagedInputVerificationV2) (*PrivateReadyInitializedV2, error) {
	fail := func(err error) (*PrivateReadyInitializedV2, error) {
		return nil, errors.Join(ErrPrivateReadyProvisioningV2, err)
	}
	if ctx == nil || k == nil || k.root == nil || len(canonicalJob) == 0 || len(canonicalJob) > recordLimitV2 || len(canonicalAuthorization) == 0 || len(canonicalAuthorization) > recordLimitV2 {
		return fail(errRecordV2)
	}
	if err := ctx.Err(); err != nil {
		return fail(err)
	}
	l := verificationLimitsV2{limits.Records, limits.Bytes, limits.Work}
	if _, err := budgetV2(l); err != nil {
		return fail(err)
	}
	// initialize needs two absence probes; initial reopen needs three records.
	// Preserve the kernel's byte ceiling, not a fresh resource allowance.
	if l.Records < 3 || l.Work < 3 || l.Bytes < 2*headLimitV2 {
		return fail(errVerificationStopV2)
	}
	if len(decision.Job) != 71 || len(decision.Authorization) != 71 || len(decision.Reference) != 71 || !hashV2(decision.Job) || !hashV2(decision.Authorization) || !hashV2(decision.Reference) {
		return fail(errRecordV2)
	}
	// Own bounded input snapshots before parsing or invoking the kernel. The
	// confirmed result must retain these exact bytes, not later caller mutations.
	jobBytes := bytes.Clone(canonicalJob)
	authorizationBytes := bytes.Clone(canonicalAuthorization)
	var j jobV2
	var a authorizationV2
	if parseV2(jobBytes, &j) != nil || parseV2(authorizationBytes, &a) != nil {
		return fail(errRecordV2)
	}
	d := decisionV2{Job: decision.Job, Authorization: decision.Authorization, Reference: decision.Reference}
	if a.Job != j.Identity || a.Revision != 1 || !approvedV2(j, a, d) {
		return fail(errRecordV2)
	}
	h, err := k.initialize(ctx, j, a, d, time.Now().UnixNano(), l)
	if err != nil {
		return fail(err)
	}
	observed, err := k.reopen(ctx, j, l)
	if err != nil {
		return fail(err)
	}
	if observed.Identity != h.Identity || observed.Job.Identity != j.Identity || observed.Authorization != a.Identity || observed.Event != "INITIALIZE" || observed.Sequence != 1 || observed.Disposition != "READY" || observed.Charged != (countersV2{}) || observed.Windows != 0 || observed.Previous != noneV2 || observed.Attempt != noneV2 || observed.ReservationSelector != noneV2 {
		return fail(errRecordV2)
	}
	if err := k.root.ValidatePrivate(); err != nil {
		return fail(err)
	}
	r := &PrivateReadyInitializedV2{job: jobBytes, head: observed.Identity, expires: a.Expires}
	if err := r.CheckUsable(ctx); err != nil {
		return fail(err)
	}
	return r, nil
}
