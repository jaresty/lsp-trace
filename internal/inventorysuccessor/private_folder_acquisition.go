package inventorysuccessor

import (
	"context"
	"time"

	"lsp-trace/internal/publication"
	"lsp-trace/sessionruntime"
)

// OpenLocalFolderDiscoveryV1 implements invocation consent for exactly one fresh
// bounded discovery lifetime. It does not spend enumeration funding, renew a
// spent owner, admit a native target, or authenticate the caller. The normal CLI
// owns the fresh root/attempt and supplies the verified preparation/plan binding.
// All execution uses the existing reserve/execute/complete/readback machinery.
func OpenLocalFolderDiscoveryV1(ctx context.Context, root *publication.Root, b PrivateDiscoveryBindingV2, attempt string) (PrivateDiscoveryOperationV2, error) {
	// This is the existing CLI partition enclosure, not a new larger grant.
	ceiling := sessionruntime.PrivateDiscoveryLimits{SourceBytes: 16 * (1<<20 + 1), RequestBytes: 32 << 20, ResponseBytes: 16 << 20, WireBytes: 48 << 20, RetainedBytes: 512 << 20, Work: 1 << 32, Messages: 256, Requests: 128, FrameBytes: 1 << 20, HeaderBytes: 8192}
	if ctx == nil || ctx.Err() != nil || root == nil || !hashV2(attempt) || b.Ordinal != 0 || b.Limits != ceiling || b.PolicyDigest != "" {
		return PrivateDiscoveryOperationV2{}, ErrPrivateDiscoveryOperationV2
	}
	domain := "local-folder-acquisition.v1/"
	j := jobV2{Schema: jobSchemaV2, StableID: hashLabelV2(domain + attempt), Application: hashLabelV2(domain + "census"), Scope: PrivateDiscoveryScopeDigestV2(), SourceAuthorization: b.DenominatorDigest, RevisionPolicy: hashLabelV2(domain + b.Revision), EvidencePolicies: []string{hashLabelV2(domain + "structural-observation-not-native-authority")}, ProviderPolicies: []string{hashLabelV2(domain + b.RuntimeFingerprint)}, SemanticPolicies: []string{hashLabelV2(domain + "no-semantic-continuation")}, Privacy: hashLabelV2(domain + "private"), Retention: hashLabelV2(domain + "no-replay"), ResourceRules: PrivateDiscoveryResourceRulesV2(b), Origin: "NEW_EMPTY"}
	job, err := sealV2(&j)
	if err != nil {
		return PrivateDiscoveryOperationV2{}, err
	}
	b.PolicyDigest = PrivateDiscoveryPolicyDigestV2(job)
	now := time.Now()
	charge, err := privateDiscoveryChargeV2(b, now)
	if err != nil {
		return PrivateDiscoveryOperationV2{}, err
	}
	decision := hashLabelV2(domain + "invocation-consent/" + attempt)
	a := authorizationV2{Schema: authorizationSchemaV2, Job: j.Identity, Revision: 1, Previous: noneV2, BoundHead: noneV2, Ceiling: charge, MaxTranche: charge, MaxWindows: 1, MaxWindowNS: charge.ActiveNS, Expires: b.Deadline.UnixNano(), Decision: decision, Assurance: assuranceV2}
	auth, err := sealV2(&a)
	if err != nil {
		return PrivateDiscoveryOperationV2{}, err
	}
	v := PrivateManagedInputVerificationV2{Records: 4096, Bytes: 64 << 20, Work: 16384}
	ready, err := InitializePrivateReadyV2(ctx, root, job, auth, PrivateReadyDecisionAssertionV2{Job: j.Identity, Authorization: a.Identity, Reference: decision}, v)
	if err != nil {
		return PrivateDiscoveryOperationV2{}, err
	}
	owner, err := OpenPrivateDiscoveryOwnerV2(ctx, root, job, ready.ConfirmedHead(), v)
	if err != nil {
		return PrivateDiscoveryOperationV2{}, err
	}
	op, err := owner.ReservePartition(ctx, b, attempt)
	if err != nil {
		return PrivateDiscoveryOperationV2{}, err
	}
	// Explicit reserve readback precedes any runtime construction. Uncertain
	// custody stays spent; no retry, refund, or new attempt is synthesized here.
	head, err := owner.k.reopen(ctx, owner.job, owner.limits)
	if err != nil || head.Identity != owner.head || head.Disposition != "PENDING" || head.Attempt != attempt {
		owner.uncertain = true
		return PrivateDiscoveryOperationV2{}, ErrPrivateDiscoveryOperationV2
	}
	return op, nil
}
