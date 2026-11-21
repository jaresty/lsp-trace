package inventorysuccessor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"lsp-trace/internal/publication"
	"lsp-trace/sessionruntime"
	"time"
)

// PrivateRetainedNativeCallsV1 is a completed conversion of runtime originals.
// Public artifact selectors are diagnostics, not a substitute issuance token.
type PrivateRetainedNativeCallsV1 struct{ state *privateRetainedNativeCallsV1 }
type privateRetainedNativeCallsV1 struct {
	acquisition *privateDiscoveryOperationV2
	index       int
	native      PrivateNativeCallsResultV2
	summary     PrivateRetainedNativeSummaryV1
}
type PrivateRetainedNativeSummaryV1 struct {
	URI                string    `json:"uri"`
	OriginalRequestIDs [3]uint64 `json:"original_request_ids"`
	CompositeSelector  string    `json:"composite_selector"`
	ProofSelector      string    `json:"proof_selector"`
}

func (r PrivateRetainedNativeCallsV1) Summary() (PrivateRetainedNativeSummaryV1, bool) {
	if r.state == nil {
		return PrivateRetainedNativeSummaryV1{}, false
	}
	return r.state.summary, true
}

// TransferLocalFolderNativeCallsV1 uses invocation consent for ONE fresh native
// conversion, not another provider execution or a reuse of discovery funding.
// Its quote is bounded by the existing mixed-input logical allowance. Provider
// coordinates are zero because the shared constructor body performs no RPC.
// Bootstrap uses the existing local initialize/reserve/readback idiom; the
// original completed acquisition and private one-shot slot are both required.
func TransferLocalFolderNativeCallsV1(ctx context.Context, root *publication.Root, acquisition PrivateDiscoveryOperationV2, index int) (result PrivateRetainedNativeCallsV1, err error) {
	if ctx == nil || ctx.Err() != nil || root == nil || acquisition.state == nil || index < 0 || index >= 128 {
		return result, ErrPrivateNativeConstructorV2
	}
	s := acquisition.state
	s.mu.Lock()
	if !s.completed || s.executionErr != nil || s.observation.Failed || !s.observation.CleanupJoined || s.nativeClaimed[index] || index >= s.observation.OwnedMethods.NativeCallCount() || root == s.owner.k.root {
		s.mu.Unlock()
		return result, ErrPrivateNativeConstructorV2
	}
	s.nativeClaimed[index] = true // failed/uncertain transfer cannot obtain a new grant
	owned, binding, originalAttempt := s.observation.OwnedMethods, s.binding, s.attempt
	s.mu.Unlock()
	execution, cancel := context.WithDeadline(ctx, binding.Deadline)
	defer cancel()
	now := time.Now()
	charge, err := privateMixedChargeV2(now, binding.Deadline, sessionruntime.PrivateProviderWindowBounds{WireBytes: 1})
	if err != nil {
		return result, err
	}
	charge.ProviderRequests, charge.ProviderMessages, charge.WireBytes = 0, 0, 0
	// Existing 128MiB native/proof, 8MiB source and bounded verification terms
	// cover this one constructor, two source views, proof and exact readbacks.
	domain := "local-folder-native-transfer.v1/"
	attempt := hashLabelV2(fmt.Sprintf("%s%s/%d", domain, originalAttempt, index))
	j := jobV2{Schema: jobSchemaV2, StableID: attempt, Application: hashLabelV2(domain + "census"), Scope: hashLabelV2(domain + "one-original-two-views"), SourceAuthorization: binding.DenominatorDigest, RevisionPolicy: hashLabelV2(domain + binding.Revision), EvidencePolicies: []string{hashLabelV2(domain + "actual-originals")}, ProviderPolicies: []string{hashLabelV2(domain + binding.RuntimeFingerprint)}, SemanticPolicies: []string{hashLabelV2(domain + "no-semantic-continuation")}, Privacy: hashLabelV2(domain + "private"), Retention: hashLabelV2(domain + "one-shot"), ResourceRules: hashLabelV2(domain + "existing-mixed-ceiling-no-provider"), Origin: "NEW_EMPTY"}
	job, err := sealV2(&j)
	if err != nil {
		return result, err
	}
	decision := hashLabelV2(domain + "invocation-consent/" + attempt)
	a := authorizationV2{Schema: authorizationSchemaV2, Job: j.Identity, Revision: 1, Previous: noneV2, BoundHead: noneV2, Ceiling: charge, MaxTranche: charge, MaxWindows: 1, MaxWindowNS: charge.ActiveNS, Expires: binding.Deadline.UnixNano(), Decision: decision, Assurance: assuranceV2}
	auth, err := sealV2(&a)
	if err != nil {
		return result, err
	}
	verification := PrivateManagedInputVerificationV2{Records: 4096, Bytes: 64 << 20, Work: 16384}
	ready, err := InitializePrivateReadyV2(execution, root, job, auth, PrivateReadyDecisionAssertionV2{Job: j.Identity, Authorization: a.Identity, Reference: decision}, verification)
	if err != nil {
		return result, err
	}
	k := &kernelV2{root: root}
	limits := verificationLimitsV2{verification.Records, verification.Bytes, verification.Work}
	pending, err := k.reserve(execution, j, ready.ConfirmedHead(), attempt, charge, binding.Deadline.UnixNano(), time.Now().UnixNano(), limits)
	if err != nil {
		return result, err
	}
	unfinished := true
	defer func() {
		if unfinished {
			_, failure := k.observe(execution, j, pending.Identity, attempt, "FAILED", time.Now().UnixNano(), limits)
			err = errors.Join(err, failure)
		}
		if err != nil {
			result = PrivateRetainedNativeCallsV1{}
		}
	}()
	checked, err := k.reopen(execution, j, limits)
	if err != nil || checked.Identity != pending.Identity || checked.Disposition != "PENDING" || checked.Attempt != attempt || checked.Charged != charge {
		return result, errors.Join(ErrPrivateNativeConstructorV2, err)
	}
	v, err := owned.TakeNativeCalls(execution, index)
	if err != nil {
		return result, err
	}
	native, err := constructNativeCallsFromOwnedViewPrivateV2(execution, PrivateNativeCallsRequestV2{Session: v.Session, Directional: v.Request, Revision: binding.Revision, Privacy: "PRIVATE"}, v, true)
	if err != nil {
		return result, err
	}
	summary := PrivateRetainedNativeSummaryV1{URI: v.Request.URI}
	for i, r := range v.Requests {
		summary.OriginalRequestIDs[i] = r.RequestID
	}
	summary.CompositeSelector, _, err = chainPublish(root, "folder-native-composite", native.Composite.Bytes)
	if err != nil {
		return result, err
	}
	summary.ProofSelector, _, err = chainPublish(root, "folder-native-proof", native.nativeEvidence)
	if err != nil {
		return result, err
	}
	raw, err := json.Marshal(summary)
	if err != nil {
		return result, err
	}
	if _, _, err = chainPublish(root, "folder-native-transfer", raw); err != nil {
		return result, err
	}
	completed, err := k.observe(execution, j, pending.Identity, attempt, "OBSERVED", time.Now().UnixNano(), limits)
	if err != nil {
		return result, err
	}
	unfinished = false
	readback, err := k.reopen(execution, j, limits)
	if err != nil || readback.Identity != completed.Identity || readback.Disposition != "OBSERVED" || readback.Attempt != attempt || readback.Charged != charge {
		return result, errors.Join(ErrPrivateNativeConstructorV2, err)
	}
	result = PrivateRetainedNativeCallsV1{state: &privateRetainedNativeCallsV1{acquisition: s, index: index, native: native, summary: summary}}
	return result, nil
}
