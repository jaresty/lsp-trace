package censuslineagepilot

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"sync"
	"time"

	"lsp-trace/internal/publication"
)

const (
	scopeBundlePolicy               = "feature-scope-bundle-finite-private-v1"
	scopeBundleHeadSchema           = "lsp-trace.feature-scope-bundle-head.private.v1"
	scopeBundleReceiptSchema        = "lsp-trace.feature-scope-bundle-attempt.private.v1"
	scopeBundleWireLimit            = 64 << 10
	scopeBundleCalls                = 24 // <= 10 pairs, their finalizations, and bounded reopen observations
	scopeBundleByteLimit     uint64 = 64 << 20
	scopeBundleWorkLimit     uint64 = 16 << 30
	scopeBundleIODeadline           = 5 * time.Second
)

type scopeBundleBinding struct {
	Scope    string                                    `json:"scope"`
	Metadata string                                    `json:"metadata"`
	Policy   string                                    `json:"policy"`
	Roots    [3]publication.RetentionDirectoryIdentity `json:"roots"`
}
type scopeBundleHead struct {
	Schema       string             `json:"schema"`
	Key          string             `json:"key"`
	Binding      scopeBundleBinding `json:"binding"`
	Generation   uint64             `json:"generation"`
	Predecessor  string             `json:"predecessor"`
	State        string             `json:"state"`
	Index        int                `json:"index"`
	Receipt      string             `json:"receipt"`
	Authority    uint8              `json:"authority"`
	Accepted     bool               `json:"accepted"`
	Completeness string             `json:"completeness"`
	Identity     string             `json:"identity"`
}
type scopeBundleCopy struct {
	Outcome string                        `json:"outcome"`
	Receipt *publication.BoundFileReceipt `json:"receipt"`
}
type scopeBundleReceipt struct {
	Schema      string             `json:"schema"`
	Key         string             `json:"key"`
	Binding     scopeBundleBinding `json:"binding"`
	Index       int                `json:"index"`
	Claim       string             `json:"claim"`
	Previous    string             `json:"previous"`
	Object      string             `json:"object"`
	Length      uint64             `json:"length"`
	Phase       string             `json:"phase"`
	Primary     scopeBundleCopy    `json:"primary"`
	Backup      scopeBundleCopy    `json:"backup"`
	RootsClosed bool               `json:"roots_closed"`
	Success     bool               `json:"success"`
	Failure     string             `json:"failure"`
}

// Capability identity is checked by pointer AND owning instance. No wire or
// caller boolean can recreate custody. Consumed once even on finalize failure.
type scopeBundlePending struct {
	owner       *scopeBundleOwner
	claim       []byte
	receipt     []byte
	selector    string
	attempt     scopeRetentionAttempt
	publication *publication.BoundFileReceipt
}
type scopeBundleResult struct {
	outcome                string
	head                   []byte // exact known committed revision, including post-commit errors
	casCommitted           bool
	cas                    *publication.CompareAndReplaceReceipt
	attempt                scopeRetentionAttempt
	receipt                []byte
	linked                 [][]byte
	receiptSelector        string
	receiptPublication     *publication.BoundFileReceipt
	pending                *scopeBundlePending
	authority              uint8
	accepted               bool
	completeness, identity string
}

func (r scopeBundleResult) HeadBytes() []byte    { return bytes.Clone(r.head) }
func (r scopeBundleResult) ReceiptBytes() []byte { return bytes.Clone(r.receipt) }
func (r scopeBundleResult) LinkedReceiptBytes() [][]byte {
	out := make([][]byte, len(r.linked))
	for i, raw := range r.linked {
		out[i] = bytes.Clone(raw)
	}
	return out
}

type scopeBundleOps struct {
	open       func(context.Context, string) (*publication.ContentRetentionRoot, error)
	close      func(*publication.ContentRetentionRoot) error
	cas        func(context.Context, *publication.ContentRetentionRoot, string, []byte, []byte) (*publication.CompareAndReplaceReceipt, error)
	pair       func(context.Context, scopeRetentionPlan, int, *publication.ContentRetentionRoot, *publication.ContentRetentionRoot) (scopeRetentionAttempt, error)
	put        func(context.Context, *publication.ContentRetentionRoot, string, []byte) (*publication.BoundFileReceipt, error)
	get        func(context.Context, *publication.ContentRetentionRoot, string, int64) ([]byte, error)
	getReceipt func(context.Context, *publication.ContentRetentionRoot, string) ([]byte, error)
}

func defaultScopeBundleOps() scopeBundleOps {
	return scopeBundleOps{
		open:  publication.OpenContentRetentionRoot,
		close: func(r *publication.ContentRetentionRoot) error { return r.Close() },
		cas: func(ctx context.Context, r *publication.ContentRetentionRoot, key string, old, new []byte) (*publication.CompareAndReplaceReceipt, error) {
			return r.CompareScopeBundleHead(ctx, key, old, new, func(raw []byte) error { _, e := parseScopeBundleHead(raw); return e })
		},
		pair: func(ctx context.Context, p scopeRetentionPlan, i int, a, b *publication.ContentRetentionRoot) (scopeRetentionAttempt, error) {
			if i == len(p.state.objects) {
				return retainScopeMetadata(ctx, p, a, b)
			}
			return retainScopeObject(ctx, p, i, a, b, scopeRetentionByteCeiling, scopeRetentionWorkCeiling)
		},
		put: func(ctx context.Context, r *publication.ContentRetentionRoot, id string, raw []byte) (*publication.BoundFileReceipt, error) {
			return r.PublishScopeBundleReceipt(ctx, id, raw)
		},
		getReceipt: func(ctx context.Context, r *publication.ContentRetentionRoot, id string) ([]byte, error) {
			return r.ReadScopeBundleReceipt(ctx, id)
		},
		get: func(ctx context.Context, r *publication.ContentRetentionRoot, id string, n int64) ([]byte, error) {
			return r.Read(ctx, id, n)
		},
	}
}

type scopeBundleOwner struct {
	mu          sync.Mutex
	plan        scopeRetentionPlan
	scopeDigest string
	paths       [3]string
	binding     *scopeBundleBinding
	pending     *scopeBundlePending
	calls       uint64
	budget      scopeRetentionBudget
	charged     scopeRetentionBudget
	ops         scopeBundleOps
}

// No ordinal, expected head, plan, policy override or trust input is accepted.
func newScopeBundleOwner(scope featureScope, primary, backup, journal string) (*scopeBundleOwner, error) {
	plan, err := deriveScopeRetentionPlan(scope, scopeRetentionByteCeiling, scopeRetentionWorkCeiling)
	if err != nil {
		return nil, err
	}
	budget, err := scopeBundleBudget(plan)
	if err != nil {
		return nil, err
	}
	return &scopeBundleOwner{plan: plan, scopeDigest: Digest(scope.ManifestBytes()), paths: [3]string{primary, backup, journal}, budget: budget, ops: defaultScopeBundleOps()}, nil
}

// Reserve the WHOLE finite run before effects: stored two copies + two staging
// copies, ten bounded receipts, all 20 CAS staging heads and lock overhead.
// Each invocation reserves worst-case input validation/rehash, full receipt
// chain reconstruction and two-copy prefix verification, including read-only
// terminal/claimed calls. No counters are replenished by an unsuccessful call.
func scopeBundleBudget(p scopeRetentionPlan) (scopeRetentionBudget, error) {
	if p.state == nil || len(p.state.objects) < 1 || len(p.state.objects) > 9 {
		return scopeRetentionBudget{}, ErrInvalid
	}
	input := uint64(len(p.state.metadata))
	for _, r := range p.state.references {
		var err error
		input, err = retentionAdd(input, r.ByteLength)
		if err != nil {
			return scopeRetentionBudget{}, err
		}
	}
	storage, err := retentionAdd(p.state.budget.bytes, uint64(32*scopeBundleWireLimit))
	if err != nil {
		return scopeRetentionBudget{}, err
	}
	per, err := retentionMultiply(input, 64)
	if err != nil {
		return scopeRetentionBudget{}, err
	}
	per, err = retentionAdd(per, uint64(128*scopeBundleWireLimit))
	if err != nil {
		return scopeRetentionBudget{}, err
	}
	work, err := retentionMultiply(per, scopeBundleCalls)
	if err != nil {
		return scopeRetentionBudget{}, err
	}
	if storage > scopeBundleByteLimit || work > scopeBundleWorkLimit {
		return scopeRetentionBudget{}, ErrInvalid
	}
	return scopeRetentionBudget{storage, work}, nil
}
func scopeBundleReserve(used, delta, limit scopeRetentionBudget) (scopeRetentionBudget, error) {
	b, err := retentionAdd(used.bytes, delta.bytes)
	if err != nil {
		return used, err
	}
	w, err := retentionAdd(used.work, delta.work)
	if err != nil || b > limit.bytes || w > limit.work {
		return used, ErrInvalid
	}
	return scopeRetentionBudget{b, w}, nil
}
func scopeBundleStrict(raw []byte, v any) error {
	if len(raw) == 0 || len(raw) > scopeBundleWireLimit {
		return ErrInvalid
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if d.Decode(v) != nil || d.Decode(&struct{}{}) != io.EOF {
		return ErrInvalid
	}
	canon, err := json.Marshal(v)
	if err != nil || !bytes.Equal(canon, raw) {
		return ErrInvalid
	}
	return nil
}
func validScopeBundleDigest(s string) bool {
	if len(s) != 71 || s[:7] != "sha256:" {
		return false
	}
	for _, c := range s[7:] {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return false
		}
	}
	return true
}

// Lookup is stable within the journal for these frozen inputs. Root identities
// belong in the compared binding, NOT the lookup key: substituting a copy root
// must find and reject the old head rather than silently mint an absent key.
func scopeBundleKey(b scopeBundleBinding) string {
	raw, _ := json.Marshal(struct {
		Scope    string `json:"scope"`
		Metadata string `json:"metadata"`
		Policy   string `json:"policy"`
	}{b.Scope, b.Metadata, b.Policy})
	return Digest(raw)
}
func validScopeBundleBinding(b scopeBundleBinding) bool {
	if !validScopeBundleDigest(b.Scope) || !validScopeBundleDigest(b.Metadata) || b.Policy != scopeBundlePolicy {
		return false
	}
	for i, r := range b.Roots {
		if r.Inode == 0 {
			return false
		}
		for j := 0; j < i; j++ {
			if r == b.Roots[j] {
				return false
			}
		}
	}
	return true
}
func parseScopeBundleHead(raw []byte) (scopeBundleHead, error) {
	var h scopeBundleHead
	if scopeBundleStrict(raw, &h) != nil || h.Schema != scopeBundleHeadSchema || !validScopeBundleBinding(h.Binding) || h.Key != scopeBundleKey(h.Binding) || h.Authority != 0 || h.Accepted || h.Completeness != "UNKNOWN" || h.Identity != "UNRESOLVED" || h.Index < 0 || h.Index > 10 {
		return h, ErrInvalid
	}
	switch h.State {
	case "CLAIMED":
		if h.Index > 9 || h.Generation != uint64(2*h.Index+1) {
			return h, ErrInvalid
		}
	case "READY", "RETAINED_INPUT_BUNDLE":
		if h.Index < 1 || h.Generation != uint64(2*h.Index) {
			return h, ErrInvalid
		}
	case "FAILED":
		if h.Index > 9 || h.Generation != uint64(2*h.Index+2) {
			return h, ErrInvalid
		}
	default:
		return h, ErrInvalid
	}
	if h.Generation > 20 || h.Generation == 0 {
		return h, ErrInvalid
	}
	if h.Generation == 1 {
		if h.Predecessor != "" || h.Receipt != "" {
			return h, ErrInvalid
		}
	} else if !validScopeBundleDigest(h.Predecessor) || !validScopeBundleDigest(h.Receipt) {
		return h, ErrInvalid
	}
	return h, nil
}
func scopeBundleEncode(h scopeBundleHead) []byte { raw, _ := json.Marshal(h); return raw }
func scopeBundleClaim(b scopeBundleBinding, previous []byte, index int, receipt string) scopeBundleHead {
	pred := ""
	if previous != nil {
		pred = Digest(previous)
	}
	return scopeBundleHead{Schema: scopeBundleHeadSchema, Key: scopeBundleKey(b), Binding: b, Generation: uint64(2*index + 1), Predecessor: pred, State: "CLAIMED", Index: index, Receipt: receipt, Completeness: "UNKNOWN", Identity: "UNRESOLVED"}
}
func scopeBundleFinish(claim scopeBundleHead, claimRaw []byte, r scopeBundleReceipt, selector string, total int) scopeBundleHead {
	h := claim
	h.Generation++
	h.Predecessor = Digest(claimRaw)
	h.Receipt = selector
	h.State = "FAILED"
	if r.Success {
		h.Index++
		h.State = "READY"
		if h.Index == total {
			h.State = "RETAINED_INPUT_BUNDLE"
		}
	}
	return h
}
func (o *scopeBundleOwner) item(i int) scopeRetentionObject {
	if i == len(o.plan.state.objects) {
		return scopeRetentionObject{Digest(o.plan.state.metadata), o.plan.state.metadata}
	}
	return o.plan.state.objects[i]
}
func copyScopeBundleAttempt(a scopeRetentionAttempt) scopeRetentionAttempt {
	if a.primary.receipt != nil {
		r := *a.primary.receipt
		a.primary.receipt = &r
	}
	if a.backup.receipt != nil {
		r := *a.backup.receipt
		a.backup.receipt = &r
	}
	if a.verified.state != nil {
		v := *a.verified.state
		a.verified.state = &v
	}
	return a
}
func scopeBundleGoodReceipt(r *publication.BoundFileReceipt, id string, n uint64) bool {
	return r != nil && r.NamespaceAtomic && r.FinalSelector == id && r.Digest == id && r.ByteLength == n && r.Mechanism == publication.BoundFileMechanism && r.DirectorySyncStatus == publication.DirectorySyncComplete && r.CrashDurability == publication.DirectorySyncComplete && r.CloseStatus == publication.CloseComplete && r.VerificationStatus == "VERIFIED"
}
func scopeBundleReceiptSelector(id string) string {
	return "feature-scope-bundle-attempt-private-v1-" + id[7:] + ".json"
}
func scopeBundleGoodJournalReceipt(r *publication.BoundFileReceipt, id string, n uint64) bool {
	if r == nil || !validScopeBundleDigest(id) || r.FinalSelector != scopeBundleReceiptSelector(id) {
		return false
	}
	copied := *r
	copied.FinalSelector = copied.Digest
	return scopeBundleGoodReceipt(&copied, id, n)
}
func scopeBundleGoodCAS(r *publication.CompareAndReplaceReceipt, key string, raw []byte) bool {
	if r == nil || !r.Committed || r.Outcome != publication.CASOutcomeCommitted || r.BoundFileReceipt == nil || !validScopeBundleDigest(key) {
		return false
	}
	if r.FinalSelector != "feature-scope-bundle-head-private-v1-"+key[7:]+".json" {
		return false
	}
	copyReceipt := *r.BoundFileReceipt
	copyReceipt.FinalSelector = copyReceipt.Digest
	return scopeBundleGoodReceipt(&copyReceipt, Digest(raw), uint64(len(raw)))
}

// The concrete CAS reports predecessor mismatch as an empty, noncommitted
// receipt plus ErrStalePredecessor. NOT_COMMITTED instead accompanies a canceled
// post-stage publication; a nil-error NOT_COMMITTED conflict is not in that API.
// Require the exact mismatch shape and inspect every cause, not errors.Is alone:
// a matching sibling must never conceal cancellation or another failure.
func scopeBundlePredecessorConflict(receipt *publication.CompareAndReplaceReceipt, err error) bool {
	if receipt == nil || receipt.Committed || receipt.Outcome != "" || receipt.BoundFileReceipt != nil {
		return false
	}
	remaining := 64
	return scopeBundleOnlyStale(err, &remaining)
}

func scopeBundleOnlyStale(err error, remaining *int) bool {
	if err == nil || *remaining == 0 {
		return false
	}
	*remaining = *remaining - 1
	if err == publication.ErrStalePredecessor {
		return true
	}
	// Custom Is claims are not concrete predecessor evidence, including wrappers
	// that may claim cancellation while unwrapping to a stale sentinel.
	if _, opaque := err.(interface{ Is(error) bool }); opaque {
		return false
	}
	if joined, ok := err.(interface{ Unwrap() []error }); ok {
		children := joined.Unwrap()
		if len(children) == 0 || len(children) > *remaining {
			return false
		}
		for _, child := range children {
			if !scopeBundleOnlyStale(child, remaining) {
				return false
			}
		}
		return true
	}
	if wrapped := errors.Unwrap(err); wrapped != nil {
		return scopeBundleOnlyStale(wrapped, remaining)
	}
	return false
}

func (o *scopeBundleOwner) parseReceipt(raw []byte, b scopeBundleBinding) (scopeBundleReceipt, error) {
	var r scopeBundleReceipt
	if scopeBundleStrict(raw, &r) != nil || r.Schema != scopeBundleReceiptSchema || r.Key != scopeBundleKey(b) || r.Binding != b || r.Index < 0 || r.Index > len(o.plan.state.objects) || !validScopeBundleDigest(r.Claim) {
		return r, ErrInvalid
	}
	obj := o.item(r.Index)
	if r.Object != obj.digest || r.Length != uint64(len(obj.raw)) {
		return r, ErrInvalid
	}
	if r.Index == 0 {
		if r.Previous != "" {
			return r, ErrInvalid
		}
	} else if !validScopeBundleDigest(r.Previous) {
		return r, ErrInvalid
	}
	validCopy := func(c scopeBundleCopy) bool {
		switch c.Outcome {
		case "NOT_ATTEMPTED", "EXISTING_UNVERIFIED", "EXISTING_VERIFIED":
			return c.Receipt == nil
		case "NAMESPACE_COMMITTED":
			if c.Receipt == nil || !c.Receipt.NamespaceAtomic {
				return false
			}
		case "INDETERMINATE":
			if c.Receipt != nil && c.Receipt.NamespaceAtomic {
				return false
			}
		default:
			return false
		}
		return c.Receipt == nil || (c.Receipt.Digest == obj.digest && c.Receipt.FinalSelector == obj.digest && c.Receipt.ByteLength == uint64(len(obj.raw)))
	}
	if !validCopy(r.Primary) || !validCopy(r.Backup) || len(r.Failure) > 4096 {
		return r, ErrInvalid
	}
	good := func(c scopeBundleCopy) bool {
		return c.Outcome == "EXISTING_VERIFIED" && c.Receipt == nil || c.Outcome == "NAMESPACE_COMMITTED" && scopeBundleGoodReceipt(c.Receipt, obj.digest, uint64(len(obj.raw)))
	}
	na := func(c scopeBundleCopy) bool { return c.Outcome == "NOT_ATTEMPTED" }
	pre := func(c scopeBundleCopy) bool { return na(c) || c.Outcome == "EXISTING_UNVERIFIED" }
	publishing := func(c scopeBundleCopy) bool {
		return c.Outcome == "INDETERMINATE" || c.Outcome == "NAMESPACE_COMMITTED"
	}
	possible := false
	switch r.Phase {
	case "PREFLIGHT":
		possible = na(r.Primary) && na(r.Backup)
	case "PRIMARY_PRE_READ":
		possible = pre(r.Primary) && na(r.Backup)
	case "PRIMARY_PUBLICATION":
		possible = publishing(r.Primary) && na(r.Backup)
	case "BACKUP_PRE_READ":
		possible = good(r.Primary) && pre(r.Backup)
	case "BACKUP_PUBLICATION":
		possible = good(r.Primary) && publishing(r.Backup)
	case "PRIMARY_REOPEN", "BACKUP_REOPEN", "OBJECT_COPIES_VERIFIED":
		possible = good(r.Primary) && good(r.Backup)
	}
	if !possible {
		return r, ErrInvalid
	}
	if r.Success != (r.RootsClosed && r.Failure == "" && r.Phase == "OBJECT_COPIES_VERIFIED" && good(r.Primary) && good(r.Backup)) {
		return r, ErrInvalid
	}
	if !r.Success && r.Failure == "" {
		return r, ErrInvalid
	}
	return r, nil
}

// Walk only the exact head-linked prefix, never scan for orphan receipts. The
// deterministic reconstruction ties every receipt to its claim predecessor.
func (o *scopeBundleOwner) verifyPrefix(ctx context.Context, j, a, b *publication.ContentRetentionRoot, binding scopeBundleBinding, h scopeBundleHead, raw []byte, observed *[][]byte) ([]scopeBundleReceipt, error) {
	count := h.Index
	if h.State == "FAILED" {
		count++
	}
	selector := h.Receipt
	chain := make([]scopeBundleReceipt, count)
	ids := make([]string, count)
	for i := count - 1; i >= 0; i-- {
		rr, err := o.ops.getReceipt(ctx, j, selector)
		if err != nil {
			return nil, err
		}
		if Digest(rr) != selector {
			return nil, ErrInvalid
		}
		*observed = append(*observed, bytes.Clone(rr))
		r, err := o.parseReceipt(rr, binding)
		if err != nil || r.Index != i {
			return nil, ErrInvalid
		}
		chain[i] = r
		ids[i] = selector
		selector = r.Previous
	}
	if selector != "" {
		return nil, ErrInvalid
	}
	var prior []byte
	last := ""
	for i, r := range chain {
		claim := scopeBundleClaim(binding, prior, i, last)
		cr := scopeBundleEncode(claim)
		if r.Claim != Digest(cr) || r.Previous != last || (!r.Success && i != len(chain)-1) {
			return nil, ErrInvalid
		}
		final := scopeBundleFinish(claim, cr, r, ids[i], len(o.plan.state.objects)+1)
		prior = scopeBundleEncode(final)
		last = ids[i]
		if r.Success {
			obj := o.item(i)
			for _, root := range []*publication.ContentRetentionRoot{a, b} {
				got, err := o.ops.get(ctx, root, obj.digest, int64(len(obj.raw)))
				if err != nil {
					return nil, err
				}
				if !bytes.Equal(got, obj.raw) {
					return nil, ErrInvalid
				}
			}
		}
	}
	if h.State == "CLAIMED" {
		prior = scopeBundleEncode(scopeBundleClaim(binding, prior, h.Index, last))
	}
	if !bytes.Equal(prior, raw) {
		return nil, ErrInvalid
	}
	return chain, nil
}

// Advance performs zero or one CAS, and zero or one pair attempt. It is not a
// loop/retry engine. Roots are acquired and consumed within this invocation.
func (o *scopeBundleOwner) advance(ctx context.Context, pending *scopeBundlePending) (out scopeBundleResult, err error) {
	out.outcome = "REFUSED"
	out.completeness = "UNKNOWN"
	out.identity = "UNRESOLVED"
	if o == nil || ctx == nil {
		return out, ErrInvalid
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	livePending := o.pending
	if pending != nil && pending == livePending && pending.owner == o {
		out.outcome = "INDETERMINATE_EXECUTION"
		out.head = bytes.Clone(pending.claim)
		out.receipt = bytes.Clone(pending.receipt)
		out.receiptSelector = pending.selector
		out.attempt = copyScopeBundleAttempt(pending.attempt)
		if pending.publication != nil {
			r := *pending.publication
			out.receiptPublication = &r
		}
		o.pending = nil // consume even when reopening or admission fails
	}
	if o.calls >= scopeBundleCalls {
		return out, ErrInvalid
	}
	o.calls++
	delta := scopeRetentionBudget{work: o.budget.work / scopeBundleCalls}
	if o.calls == 1 {
		delta.bytes = o.budget.bytes
	}
	o.charged, err = scopeBundleReserve(o.charged, delta, o.budget)
	if err != nil {
		return out, err
	}
	if err = ctx.Err(); err != nil {
		return out, err
	}
	var roots [3]*publication.ContentRetentionRoot
	closed := [3]bool{}
	closeAt := func(i int) error {
		if roots[i] == nil || closed[i] {
			return nil
		}
		closed[i] = true
		return o.ops.close(roots[i])
	}
	defer func() {
		var closeErr error
		for i := 2; i >= 0; i-- {
			closeErr = errors.Join(closeErr, closeAt(i))
		}
		err = errors.Join(err, closeErr)
		if closeErr != nil {
			out.outcome = "INDETERMINATE_EXECUTION"
		}
	}()
	for i, path := range o.paths {
		roots[i], err = o.ops.open(ctx, path)
		if err != nil {
			return out, err
		}
	}
	binding := scopeBundleBinding{Scope: o.scopeDigest, Metadata: Digest(o.plan.state.metadata), Policy: scopeBundlePolicy}
	for i, r := range roots {
		binding.Roots[i], err = r.DirectoryIdentity()
		if err != nil {
			return out, err
		}
	}
	if !validScopeBundleBinding(binding) {
		return out, ErrInvalid
	}
	if o.binding != nil && *o.binding != binding {
		return out, ErrInvalid
	}
	if o.binding == nil {
		b := binding
		o.binding = &b
	}
	key := scopeBundleKey(binding)
	raw, loadErr := roots[2].ReadScopeBundleHead(ctx, key)
	if loadErr != nil && !retentionOnlyAbsent(loadErr, 0) {
		return out, loadErr
	}
	absent := retentionOnlyAbsent(loadErr, 0)
	var h scopeBundleHead
	if !absent {
		h, err = parseScopeBundleHead(raw)
		if err != nil || h.Binding != binding {
			return out, ErrInvalid
		}
		out.head = bytes.Clone(raw)
	}
	// No live capability: do not scan receipts, inspect objects, or recover work.
	if !absent && h.State == "CLAIMED" && (pending == nil || pending != livePending || pending.owner != o) {
		out.outcome = "INDETERMINATE_EXECUTION"
		return out, nil
	}
	if pending != nil && (pending != livePending || pending.owner != o || absent || h.State != "CLAIMED") {
		return out, ErrInvalid
	}
	if !absent && h.State != "CLAIMED" {
		if _, err = o.verifyPrefix(ctx, roots[2], roots[0], roots[1], binding, h, raw, &out.linked); err != nil {
			return out, err
		}
		if h.State == "RETAINED_INPUT_BUNDLE" || h.State == "FAILED" {
			out.outcome = h.State
			out.head = bytes.Clone(raw)
			return out, nil
		}
	}
	apply := func(previous, replacement []byte) bool {
		casCtx, cancelCAS := context.WithTimeout(ctx, scopeBundleIODeadline)
		defer cancelCAS()
		cas, e := o.ops.cas(casCtx, roots[2], key, previous, replacement)
		out.cas = cas
		err = e
		if cas != nil && cas.Committed {
			out.casCommitted = true
			out.head = bytes.Clone(replacement)
		}
		if scopeBundlePredecessorConflict(cas, e) {
			out.outcome = "CONFLICT"
			return false
		}
		if e != nil || !scopeBundleGoodCAS(cas, key, replacement) {
			out.outcome = "INDETERMINATE_EXECUTION"
			return false
		}
		return true
	}
	if pending != nil {
		o.pending = nil // consume before observation; replay never regains custody
		out.receipt = bytes.Clone(pending.receipt)
		out.receiptSelector = pending.selector
		if !bytes.Equal(raw, pending.claim) {
			return out, ErrInvalid
		}
		rr, e := o.ops.getReceipt(ctx, roots[2], pending.selector)
		if e != nil {
			return out, e
		}
		if !bytes.Equal(rr, pending.receipt) || Digest(rr) != pending.selector {
			return out, ErrInvalid
		}
		r, e := o.parseReceipt(rr, binding)
		if e != nil || r.Claim != Digest(raw) || r.Index != h.Index || r.Previous != h.Receipt {
			return out, ErrInvalid
		}
		// Verify the complete predecessor and current successful pair, not checkout.
		final := scopeBundleFinish(h, raw, r, pending.selector, len(o.plan.state.objects)+1)
		finalRaw := scopeBundleEncode(final)
		if _, err = o.verifyPrefix(ctx, roots[2], roots[0], roots[1], binding, final, finalRaw, &out.linked); err != nil {
			return out, err
		}
		if err = errors.Join(closeAt(0), closeAt(1)); err != nil {
			return out, err
		}
		if !apply(raw, finalRaw) {
			return out, err
		}
		out.outcome = final.State
		return out, nil
	}
	index := 0
	last := ""
	var previous []byte
	if !absent {
		index = h.Index
		last = h.Receipt
		previous = raw
	}
	if index >= len(o.plan.state.objects)+1 {
		return out, ErrInvalid
	}
	claim := scopeBundleClaim(binding, previous, index, last)
	claimRaw := scopeBundleEncode(claim)
	if !apply(previous, claimRaw) {
		return out, err
	}
	// A known claim with an ambiguous error MUST NOT execute the pair. No second
	// CAS can occur below, even if the pair, closes, or publication fails.
	attempt, pairErr := o.ops.pair(ctx, o.plan, index, roots[0], roots[1])
	objectCloseErr := errors.Join(closeAt(0), closeAt(1))
	pairErr = errors.Join(pairErr, objectCloseErr)
	out.attempt = copyScopeBundleAttempt(attempt)
	obj := o.item(index)
	r := scopeBundleReceipt{Schema: scopeBundleReceiptSchema, Key: key, Binding: binding, Index: index, Claim: Digest(claimRaw), Previous: last, Object: obj.digest, Length: uint64(len(obj.raw)), Phase: attempt.phase, Primary: scopeBundleCopy{attempt.primary.outcome, attempt.primary.receipt}, Backup: scopeBundleCopy{attempt.backup.outcome, attempt.backup.receipt}, RootsClosed: objectCloseErr == nil, Success: pairErr == nil && attempt.verified.state != nil}
	if !r.Success {
		if pairErr == nil {
			pairErr = ErrInvalid
		}
		r.Failure = pairErr.Error()
		if len(r.Failure) > 4096 {
			r.Failure = r.Failure[:4096]
		}
	}
	rr, encodeErr := json.Marshal(r)
	out.receipt = bytes.Clone(rr)
	out.receiptSelector = Digest(rr)
	if _, validationErr := o.parseReceipt(rr, binding); encodeErr != nil || validationErr != nil {
		out.outcome = "INDETERMINATE_EXECUTION"
		return out, errors.Join(pairErr, encodeErr, validationErr)
	}
	// Cancellation is recorded in the attempt, not allowed to erase it. This one
	// bounded cleanup publication is not a retry and confers no execution authority.
	receiptCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), scopeBundleIODeadline)
	defer cancel()
	receipt, putErr := o.ops.put(receiptCtx, roots[2], out.receiptSelector, rr)
	out.receiptPublication = receipt
	if putErr != nil || !scopeBundleGoodJournalReceipt(receipt, out.receiptSelector, uint64(len(rr))) {
		out.outcome = "INDETERMINATE_EXECUTION"
		return out, errors.Join(pairErr, putErr, ErrInvalid)
	}
	got, getErr := o.ops.getReceipt(receiptCtx, roots[2], out.receiptSelector)
	if getErr != nil || !bytes.Equal(got, rr) {
		out.outcome = "INDETERMINATE_EXECUTION"
		return out, errors.Join(pairErr, getErr, ErrInvalid)
	}
	if err = closeAt(2); err != nil {
		out.outcome = "INDETERMINATE_EXECUTION"
		return out, errors.Join(pairErr, err)
	}
	cap := &scopeBundlePending{owner: o, claim: bytes.Clone(claimRaw), receipt: bytes.Clone(rr), selector: out.receiptSelector, attempt: copyScopeBundleAttempt(attempt)}
	if receipt != nil {
		p := *receipt
		cap.publication = &p
	}
	o.pending = cap
	out.pending = cap
	out.outcome = "CLAIMED"
	return out, pairErr
}
