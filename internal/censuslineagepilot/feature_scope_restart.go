package censuslineagepilot

import (
	"bytes"
	"context"
	"errors"

	"lsp-trace/internal/publication"
)

// Lookup/comparison data only. In particular, a locator is neither a retained
// scope nor permission to advance an owner. All digests here are RAW byte IDs.
// No mutable selector, stage, ordinal, source path, or raw plan is accepted.
type scopeRestartBlob struct {
	ID         string `json:"id"`
	ByteLength uint64 `json:"byte_length"`
}
type scopeRestartLocator struct {
	HeadKey  string           `json:"head_key"`
	Head     scopeRestartBlob `json:"head"`
	Scope    scopeRestartBlob `json:"scope"`
	Metadata scopeRestartBlob `json:"metadata"`
}

func (b scopeRestartBlob) valid(limit uint64) bool {
	return validScopeBundleDigest(b.ID) && b.ByteLength > 0 && b.ByteLength <= limit
}
func (l scopeRestartLocator) valid() bool {
	return validScopeBundleDigest(l.HeadKey) && l.Head.valid(scopeBundleWireLimit) && l.Scope.valid(scopeRetentionMetadataLimit) && l.Metadata.valid(scopeRetentionMetadataLimit) && l.HeadKey == scopeBundleKey(scopeBundleBinding{Scope: l.Scope.ID, Metadata: l.Metadata.ID, Policy: scopeBundlePolicy})
}

type scopeRestartMetadata struct {
	Schema     string                    `json:"schema"`
	Authority  uint8                     `json:"authority"`
	References []scopeRetentionReference `json:"active_references"`
}

// No write operation is representable in these fixture seams. Concrete root
// capabilities are kept local and never returned. verifyPrefix receives only
// the two read closures; advance, CAS, pair and put are never installed/called.
type scopeRestartOps struct {
	open    func(context.Context, string) (*publication.ContentRetentionRoot, error)
	close   func(*publication.ContentRetentionRoot) error
	head    func(context.Context, *publication.ContentRetentionRoot, string) ([]byte, error)
	get     func(context.Context, *publication.ContentRetentionRoot, string, int64) ([]byte, error)
	receipt func(context.Context, *publication.ContentRetentionRoot, string) ([]byte, error)
}

func defaultScopeRestartOps() scopeRestartOps {
	return scopeRestartOps{
		open:  publication.OpenContentRetentionRoot,
		close: func(r *publication.ContentRetentionRoot) error { return r.Close() },
		head: func(c context.Context, r *publication.ContentRetentionRoot, k string) ([]byte, error) {
			return r.ReadScopeBundleHead(c, k)
		},
		get: func(c context.Context, r *publication.ContentRetentionRoot, k string, n int64) ([]byte, error) {
			return r.Read(c, k, n)
		},
		receipt: func(c context.Context, r *publication.ContentRetentionRoot, k string) ([]byte, error) {
			return r.ReadScopeBundleReceipt(c, k)
		},
	}
}

type scopeRestartResult struct{ state *scopeRestartState }
type scopeRestartState struct {
	scope          featureScope
	head, metadata []byte
	linked         [][]byte
	// Exact identities observed by this reader, not caller-supplied root DTOs.
	roots [3]publication.RetentionDirectoryIdentity
}

func (r scopeRestartResult) Scope() featureScope {
	if r.state == nil {
		return featureScope{}
	}
	s := r.state.scope
	return featureScope{&featureScopeState{files: s.Files(), manifest: s.ManifestBytes(), selection: s.SelectionBytes(), plan: s.PlanBytes()}}
}
func (r scopeRestartResult) HeadBytes() []byte {
	if r.state == nil {
		return nil
	}
	return bytes.Clone(r.state.head)
}
func (r scopeRestartResult) MetadataBytes() []byte {
	if r.state == nil {
		return nil
	}
	return bytes.Clone(r.state.metadata)
}
func (r scopeRestartResult) LinkedReceiptBytes() [][]byte {
	if r.state == nil {
		return nil
	}
	out := make([][]byte, len(r.state.linked))
	for i, b := range r.state.linked {
		out[i] = bytes.Clone(b)
	}
	return out
}

// Reconstruct only the fixed retained feature scope. The source root and its
// file identities are historical observations embedded in retained bytes: they
// are NOT reopened, compared with this checkout, or authenticated. Successful
// observation preserves authority=0, accepted=false, UNKNOWN completeness,
// UNRESOLVED identity and CALLER_ASSERTED_NOT_AUTHENTICATED revision.
func readRetainedFeatureScope(ctx context.Context, roots [3]string, locator scopeRestartLocator) (scopeRestartResult, error) {
	return readRetainedFeatureScopeWithOps(ctx, roots, locator, defaultScopeRestartOps())
}
func readRetainedFeatureScopeWithOps(ctx context.Context, paths [3]string, locator scopeRestartLocator, ops scopeRestartOps) (out scopeRestartResult, err error) {
	if ctx == nil || !locator.valid() || ops.open == nil || ops.close == nil || ops.head == nil || ops.get == nil || ops.receipt == nil {
		return out, ErrInvalid
	}
	if err = ctx.Err(); err != nil {
		return out, err
	}
	ctx, cancel := context.WithTimeout(ctx, scopeBundleIODeadline)
	defer cancel()
	var roots [3]*publication.ContentRetentionRoot
	defer func() {
		for i := 2; i >= 0; i-- {
			if roots[i] != nil {
				err = errors.Join(err, ops.close(roots[i]))
			}
		}
		err = errors.Join(err, ctx.Err())
		if err != nil {
			out = scopeRestartResult{}
		}
	}()
	binding := scopeBundleBinding{Scope: locator.Scope.ID, Metadata: locator.Metadata.ID, Policy: scopeBundlePolicy}
	for i, path := range paths {
		roots[i], err = ops.open(ctx, path)
		if err != nil {
			return out, err
		}
		binding.Roots[i], err = roots[i].DirectoryIdentity()
		if err != nil {
			return out, err
		}
	}
	if !validScopeBundleBinding(binding) {
		return out, ErrInvalid
	}
	head, err := ops.head(ctx, roots[2], locator.HeadKey)
	if err != nil {
		return out, err
	}
	if uint64(len(head)) != locator.Head.ByteLength || Digest(head) != locator.Head.ID {
		return out, ErrInvalid
	}
	h, err := parseScopeBundleHead(head)
	if err != nil {
		return out, err
	}
	// Refuse nonterminal states BEFORE object reads or receipt lookup. Never scan.
	if h.State != "RETAINED_INPUT_BUNDLE" || h.Binding != binding || h.Key != locator.HeadKey {
		return out, ErrInvalid
	}
	// Repository-owned finite accounting, independent of locator lengths and object
	// ordinals. Includes all reference reads (even dedup aliases), prefix rereads,
	// bootstrap, canonical rederivation and at most ten journal receipts.
	var work uint64
	charge := func(n uint64) error {
		next, e := retentionAdd(work, n)
		if e != nil || next > scopeRetentionWorkCeiling {
			return ErrInvalid
		}
		work = next
		return nil
	}
	get := func(c context.Context, r *publication.ContentRetentionRoot, id string, n int64) ([]byte, error) {
		if n < 1 || n > publication.ContentRetentionObjectLimit {
			return nil, ErrInvalid
		}
		// Read/hash/clone/equality allowance; n is already bounded above.
		if e := charge(4 * uint64(n)); e != nil {
			return nil, e
		}
		if e := c.Err(); e != nil {
			return nil, e
		}
		raw, e := ops.get(c, r, id, n)
		if e != nil {
			return nil, e
		}
		if int64(len(raw)) != n || Digest(raw) != id {
			return nil, ErrInvalid
		}
		return bytes.Clone(raw), c.Err()
	}
	pair := func(pin scopeRestartBlob) ([]byte, error) {
		a, e := get(ctx, roots[0], pin.ID, int64(pin.ByteLength))
		if e != nil {
			return nil, e
		}
		b, e := get(ctx, roots[1], pin.ID, int64(pin.ByteLength))
		if e != nil {
			return nil, e
		}
		if !bytes.Equal(a, b) {
			return nil, ErrInvalid
		}
		return a, nil
	}
	manifest, err := pair(locator.Scope)
	if err != nil {
		return out, err
	}
	metadata, err := pair(locator.Metadata)
	if err != nil {
		return out, err
	}
	var m scopeManifest
	var md scopeRestartMetadata
	if scopeBundleStrict(manifest, &m) != nil || len(m.Files) != 6 || scopeBundleStrict(metadata, &md) != nil || md.Schema != "lsp-trace.feature-scope-retention.private.v1" || md.Authority != 0 || len(md.References) != 9 {
		return out, ErrInvalid
	}
	// Bound every reference before observing any role payload. Metadata may name
	// aliases but cannot raise limits or redefine the fixed ordered role policy.
	var input uint64
	for i, r := range md.References {
		limit := uint64(scopeRetentionMetadataLimit)
		role, path := "", ""
		if i < 6 {
			limit = featureScopeFileLimit
			path = featureScopePaths()[i]
			role = "NOMINATION"
			if i >= 4 {
				role = "CONTEXT_ONLY"
			}
		} else {
			role = []string{"SOURCE_SELECTION", "PARTITION_PLAN", "SCOPE_MANIFEST"}[i-6]
		}
		if r.Role != role || r.Path != path || r.Object < 0 || r.Object > 8 || !(scopeRestartBlob{r.SHA256, r.ByteLength}).valid(limit) {
			return out, ErrInvalid
		}
		input, err = retentionAdd(input, r.ByteLength)
		if err != nil || input > featureScopeTotalLimit+3*scopeRetentionMetadataLimit {
			return out, ErrInvalid
		}
	}
	if md.References[8].SHA256 != locator.Scope.ID || md.References[8].ByteLength != locator.Scope.ByteLength {
		return out, ErrInvalid
	}
	// Reserve the existing conservative rederivation allowance before parsing
	// inputs, reconstructing through Build, hashing, and validating the plan.
	reconstruction, e := retentionMultiply(input, 32)
	if e != nil {
		return out, e
	}
	reconstruction, e = retentionAdd(reconstruction, 2*scopeRetentionMetadataLimit)
	if e != nil {
		return out, e
	}
	if err = charge(reconstruction); err != nil {
		return out, err
	}
	var payload [9][]byte
	for i, r := range md.References {
		payload[i], err = pair(scopeRestartBlob{r.SHA256, r.ByteLength})
		if err != nil {
			return out, err
		}
	}
	if !bytes.Equal(payload[8], manifest) {
		return out, ErrInvalid
	}
	selection, err := ParseSourceSelection(payload[6])
	if err != nil {
		return out, err
	}
	if _, err = ParsePartitionPlan(payload[7], selection); err != nil {
		return out, err
	}
	state := &featureScopeState{manifest: bytes.Clone(manifest), selection: payload[6], plan: payload[7], files: make([]scopeObservedFile, 6)}
	for i := range state.files {
		state.files[i] = scopeObservedFile{m.Files[i], payload[i]}
	}
	scope := featureScope{state}
	// This reconstructs selection and plan through the existing Build APIs and
	// requires byte-identical canonical manifest, including the authority tuple,
	// fixed paths, roles, exclusions, observed identities and revision qualifier.
	plan, err := deriveScopeRetentionPlan(scope, scopeRetentionByteCeiling, scopeRetentionWorkCeiling)
	if err != nil {
		return out, err
	}
	if err = validateScopeRetentionPlan(plan, scopeRetentionByteCeiling, scopeRetentionWorkCeiling); err != nil {
		return out, err
	}
	if !bytes.Equal(plan.metadataBytes(), metadata) || h.Index != len(plan.state.objects)+1 || h.Generation != uint64(2*h.Index) {
		return out, ErrInvalid
	}
	receipt := func(c context.Context, r *publication.ContentRetentionRoot, id string) ([]byte, error) {
		if e := charge(4 * scopeBundleWireLimit); e != nil {
			return nil, e
		}
		if e := c.Err(); e != nil {
			return nil, e
		}
		raw, e := ops.receipt(c, r, id)
		if e != nil {
			return nil, e
		}
		if len(raw) > scopeBundleWireLimit {
			return nil, ErrInvalid
		}
		return raw, nil
	}
	verifier := &scopeBundleOwner{plan: plan, ops: scopeBundleOps{get: get, getReceipt: receipt}}
	var linked [][]byte
	if _, err = verifier.verifyPrefix(ctx, roots[2], roots[0], roots[1], binding, h, head, &linked); err != nil {
		return out, err
	}
	// Recheck the exact head and root admission after the observation. This is not
	// a snapshot claim, but a changed head cannot pass unnoticed at completion.
	finalHead, err := ops.head(ctx, roots[2], locator.HeadKey)
	if err != nil {
		return out, err
	}
	if !bytes.Equal(finalHead, head) {
		return out, ErrInvalid
	}
	for _, r := range roots {
		if err = r.ValidatePrivate(); err != nil {
			return out, err
		}
	}
	if err = ctx.Err(); err != nil {
		return out, err
	}
	out = scopeRestartResult{&scopeRestartState{scope: scope, head: bytes.Clone(head), metadata: bytes.Clone(metadata), linked: linked, roots: binding.Roots}}
	return out, nil
}
