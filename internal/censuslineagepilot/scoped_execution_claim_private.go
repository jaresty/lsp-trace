package censuslineagepilot

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"sync"
	"unicode/utf8"

	"lsp-trace/internal/censusacquisition"
	"lsp-trace/internal/publication"
)

const scopedExecutionClaimSchemaPrivate = "lsp-trace.scoped-execution-claim.private.v1"
const scopedExecutionClaimLimitPrivate = 16 << 10

// Store-agnostic qualification seam ONLY. No production adapter is installed.
// Load must return nil,nil only for affirmative absence. CompareAbsent must make
// one absent-predecessor CAS, never retry publication, and return the concrete
// publication receipt/error shape. A future adapter needs a separate narrow
// publication namespace, not the input-bundle head or unrestricted Root access.
// Load must preserve a present empty object as a nonnil slice and bound reads to
// scopedExecutionClaimLimitPrivate; read/close uncertainty is an error, not absence.
type scopedExecutionClaimStorePrivate interface {
	Load(context.Context, string) ([]byte, error)
	CompareAbsent(context.Context, string, []byte) (*publication.CompareAndReplaceReceipt, error)
}

type scopedExecutionPolicyPrivate struct {
	Name       string `json:"name"`
	DownDepth  int    `json:"down_depth"`
	UpDepth    int    `json:"up_depth"`
	MaxTargets int    `json:"max_targets"`
}

func scopedExecutionPolicyFixedPrivate() scopedExecutionPolicyPrivate {
	return scopedExecutionPolicyPrivate{"one-native-v5-private-v1", 1, 1, 63}
}

type scopedExecutionBindingPrivate struct {
	InputHead          scopeRestartBlob                          `json:"input_head"`
	Scope              scopeRestartBlob                          `json:"scope"`
	Metadata           scopeRestartBlob                          `json:"metadata"`
	ExecutableRevision string                                    `json:"executable_revision"`
	SessionID          string                                    `json:"session_id"`
	Generation         uint64                                    `json:"generation"`
	Workspace          string                                    `json:"workspace"`
	Policy             scopedExecutionPolicyPrivate              `json:"request_policy"`
	OutputRoots        [3]publication.RetentionDirectoryIdentity `json:"output_roots"`
}

type scopedExecutionHeadPrivate struct {
	Schema                string                        `json:"schema"`
	State                 string                        `json:"state"`
	Binding               scopedExecutionBindingPrivate `json:"binding"`
	Authority             uint8                         `json:"authority"`
	Accepted              bool                          `json:"accepted"`
	Completeness          string                        `json:"completeness"`
	FeatureIdentity       string                        `json:"feature_identity"`
	RevisionQualification string                        `json:"revision_qualification"`
}

// An owner admits at most one attempt, including failed loads/CAS. These inputs
// describe provenance only: no Manager readiness, lease or runtime admission.
// There is deliberately no finalize, reset, resume, callback, or second CAS.
type scopedExecutionClaimOwnerPrivate struct {
	mu        sync.Mutex
	attempted bool
	raw       []byte
	selector  string
	pending   *scopedExecutionPendingPrivate
}

// Only the exact pointer minted by this owner is live; neither head bytes nor a
// copied pending value can reconstruct it. Later semantic integration is pending.
type scopedExecutionPendingPrivate struct {
	owner   *scopedExecutionClaimOwnerPrivate
	head    []byte
	receipt *publication.CompareAndReplaceReceipt
}

type scopedExecutionClaimResultPrivate struct {
	outcome string
	// Exact observed head, or submitted candidate on reported commit. With no
	// pending capability this is evidence only, NOT verified durable success.
	head           []byte
	cas            *publication.CompareAndReplaceReceipt
	commitReported bool // preserves commit evidence even if receipt verification fails
	pending        *scopedExecutionPendingPrivate
}

func (r scopedExecutionClaimResultPrivate) HeadBytes() []byte { return bytes.Clone(r.head) }
func (r scopedExecutionClaimResultPrivate) CASReceipt() *publication.CompareAndReplaceReceipt {
	return copyScopedExecutionCASPrivate(r.cas)
}
func copyScopedExecutionCASPrivate(r *publication.CompareAndReplaceReceipt) *publication.CompareAndReplaceReceipt {
	if r == nil {
		return nil
	}
	out := *r
	if r.BoundFileReceipt != nil {
		b := *r.BoundFileReceipt
		out.BoundFileReceipt = &b
	}
	return &out
}
func (o *scopedExecutionClaimOwnerPrivate) HeadBytes() []byte {
	if o == nil {
		return nil
	}
	return bytes.Clone(o.raw)
}
func (o *scopedExecutionClaimOwnerPrivate) ownsPending(p *scopedExecutionPendingPrivate) bool {
	if o == nil || p == nil {
		return false
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.pending == p && p.owner == o && bytes.Equal(p.head, o.raw) && scopedExecutionGoodCASPrivate(p.receipt, o.selector, o.raw)
}

// Requires an opaque retained-reader result, revalidates its entire terminal
// receipt chain in memory, and derives every input pin rather than trusting DTOs.
// Root capabilities are borrowed; callers retain ownership and must close them.
// This checks admitted identities and identity aliases, not ancestor topology or
// an ongoing root lease. The real adapter and broader output preflight are pending.
func newScopedExecutionClaimPrivate(ctx context.Context, restart scopeRestartResult, executableRevision string, session censusacquisition.SessionIdentity, workspace string, roots [3]*publication.ContentRetentionRoot) (*scopedExecutionClaimOwnerPrivate, error) {
	if ctx == nil || restart.state == nil || !featureScopeRevision.MatchString(executableRevision) || session.Validate() != nil || len(session.SessionID) > 1024 || !utf8.ValidString(session.SessionID) || len(workspace) > 4096 || !utf8.ValidString(workspace) {
		return nil, ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	// Check size/invariants before copying any caller-reachable retained state.
	plan, err := deriveScopeRetentionPlan(restart.state.scope, scopeRetentionByteCeiling, scopeRetentionWorkCeiling)
	if err != nil {
		return nil, err
	}
	if err = validateScopeRetentionPlan(plan, scopeRetentionByteCeiling, scopeRetentionWorkCeiling); err != nil {
		return nil, err
	}
	headRaw := restart.state.head
	if len(headRaw) == 0 || len(headRaw) > scopeBundleWireLimit || len(restart.state.metadata) > int(scopeRetentionMetadataLimit) || len(restart.state.linked) > 10 {
		return nil, ErrInvalid
	}
	h, err := parseScopeBundleHead(headRaw)
	if err != nil || h.State != "RETAINED_INPUT_BUNDLE" || h.Index != len(plan.state.objects)+1 || h.Generation != uint64(2*h.Index) || h.Binding.Scope != Digest(restart.state.scope.ManifestBytes()) || h.Binding.Metadata != Digest(restart.state.metadata) || !bytes.Equal(plan.state.metadata, restart.state.metadata) || len(restart.state.linked) != h.Index {
		return nil, ErrInvalid
	}
	if err = validateScopeRestartForAdapter(ctx, restart, plan, h); err != nil {
		return nil, err
	}
	var manifest scopeManifest
	if scopeBundleStrict(restart.state.scope.ManifestBytes(), &manifest) != nil || workspace != manifest.Root {
		return nil, ErrInvalid
	}
	b := scopedExecutionBindingPrivate{
		InputHead:          scopeRestartBlob{Digest(headRaw), uint64(len(headRaw))},
		Scope:              scopeRestartBlob{h.Binding.Scope, uint64(len(restart.state.scope.ManifestBytes()))},
		Metadata:           scopeRestartBlob{h.Binding.Metadata, uint64(len(restart.state.metadata))},
		ExecutableRevision: executableRevision, SessionID: session.SessionID, Generation: session.Generation, Workspace: workspace, Policy: scopedExecutionPolicyFixedPrivate(),
	}
	for i, root := range roots {
		if root == nil {
			return nil, ErrInvalid
		}
		id, e := root.DirectoryIdentity()
		if e != nil {
			return nil, e
		}
		if id.Inode == 0 {
			return nil, ErrInvalid
		}
		for _, input := range h.Binding.Roots {
			if id == input {
				return nil, ErrInvalid
			}
		}
		for j := 0; j < i; j++ {
			if id == b.OutputRoots[j] {
				return nil, ErrInvalid
			}
		}
		b.OutputRoots[i] = id
	}
	raw, err := json.Marshal(scopedExecutionHeadPrivate{Schema: scopedExecutionClaimSchemaPrivate, State: "CLAIMED", Binding: b, Completeness: "UNKNOWN", FeatureIdentity: "UNRESOLVED", RevisionQualification: "CALLER_ASSERTED_NOT_AUTHENTICATED"})
	if err != nil || len(raw) > scopedExecutionClaimLimitPrivate {
		return nil, ErrInvalid
	}
	// One slot per admitted coordinator identity: changing session, executable,
	// input pins or primary/backup roles must not select a fresh absent head in
	// the same coordinator. A future store adapter must bind this coordinator.
	keyBytes, err := json.Marshal(b.OutputRoots[2])
	if err != nil {
		return nil, err
	}
	key := Digest(keyBytes)
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return &scopedExecutionClaimOwnerPrivate{raw: bytes.Clone(raw), selector: "feature-scope-execution-head-private-v1-" + key[7:] + ".json"}, nil
}

func scopedExecutionGoodCASPrivate(r *publication.CompareAndReplaceReceipt, selector string, raw []byte) bool {
	if r == nil || !r.Committed || r.Outcome != publication.CASOutcomeCommitted || r.BoundFileReceipt == nil || r.FinalSelector != selector {
		return false
	}
	copy := *r.BoundFileReceipt
	copy.FinalSelector = copy.Digest
	return scopeBundleGoodReceipt(&copy, Digest(raw), uint64(len(raw)))
}

func (o *scopedExecutionClaimOwnerPrivate) claim(ctx context.Context, store scopedExecutionClaimStorePrivate) (out scopedExecutionClaimResultPrivate, err error) {
	out.outcome = "INDETERMINATE"
	if o == nil {
		return out, ErrInvalid
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.attempted {
		return out, ErrIndeterminateExecution
	}
	o.attempted = true
	if ctx == nil || store == nil {
		return out, ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return out, errors.Join(ErrIndeterminateExecution, err)
	}
	// Defensive bounds before ANY store traffic, even for package-local corruption.
	if len(o.raw) == 0 || len(o.raw) > scopedExecutionClaimLimitPrivate || len(o.selector) != len("feature-scope-execution-head-private-v1-")+64+len(".json") {
		return out, ErrInvalid
	}
	current, err := store.Load(ctx, o.selector)
	if len(current) > 0 && len(current) <= scopedExecutionClaimLimitPrivate {
		out.head = bytes.Clone(current)
	}
	if err != nil || ctx.Err() != nil {
		return out, errors.Join(ErrIndeterminateExecution, err, ctx.Err())
	}
	// Present heads, including malformed/oversized heads, are observation-only.
	// No parser or reconstructed capability can turn a restart into authorization.
	if current != nil {
		return out, ErrIndeterminateExecution
	}
	receipt, err := store.CompareAbsent(ctx, o.selector, bytes.Clone(o.raw))
	out.cas = copyScopedExecutionCASPrivate(receipt)
	if receipt != nil && receipt.Committed {
		out.commitReported = true
		out.head = bytes.Clone(o.raw)
	}
	if ctx.Err() == nil && scopeBundlePredecessorConflict(receipt, err) {
		out.outcome = "CONFLICT"
		return out, ErrConflict
	}
	if err != nil || ctx.Err() != nil || !scopedExecutionGoodCASPrivate(receipt, o.selector, o.raw) {
		return out, errors.Join(ErrIndeterminateExecution, err, ctx.Err())
	}
	p := &scopedExecutionPendingPrivate{owner: o, head: bytes.Clone(o.raw), receipt: copyScopedExecutionCASPrivate(receipt)}
	o.pending = p
	out.outcome, out.head, out.pending = "CLAIMED", bytes.Clone(o.raw), p
	return out, nil
}
