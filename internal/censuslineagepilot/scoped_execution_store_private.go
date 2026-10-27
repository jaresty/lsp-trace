package censuslineagepilot

import (
	"context"
	"encoding/json"

	"lsp-trace/internal/publication"
)

// Minimal private adapter, NOT live-qualified. Roots are borrowed and checked
// before each operation. It binds an already-created owner's provenance to the
// coordinator journal, but grants no Manager/readiness authority. Ancestor
// topology preflight is a separate outstanding unit: the retained-reader result
// keeps identities, not live input capabilities/paths. Do not wire this to a
// live entrypoint until that preflight is available.
type scopedExecutionStorePrivate struct {
	journal    *publication.ScopedExecutionJournalPrivate
	selector   string
	roots      [3]*publication.ContentRetentionRoot
	identities [3]publication.RetentionDirectoryIdentity
}

var _ scopedExecutionClaimStorePrivate = (*scopedExecutionStorePrivate)(nil)

func newScopedExecutionStorePrivate(ctx context.Context, owner *scopedExecutionClaimOwnerPrivate, roots [3]*publication.ContentRetentionRoot) (*scopedExecutionStorePrivate, error) {
	if ctx == nil || owner == nil || len(owner.raw) == 0 || len(owner.raw) > scopedExecutionClaimLimitPrivate {
		return nil, ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	var head scopedExecutionHeadPrivate
	if err := json.Unmarshal(owner.raw, &head); err != nil {
		return nil, err
	}
	// A different canonical selector must not open a second absent slot in the
	// same coordinator, even for an accidentally corrupted package-local owner.
	keyBytes, err := json.Marshal(head.Binding.OutputRoots[2])
	if err != nil {
		return nil, err
	}
	key := Digest(keyBytes)
	if owner.selector != "feature-scope-execution-head-private-v1-"+key[7:]+".json" {
		return nil, ErrInvalid
	}
	s := &scopedExecutionStorePrivate{selector: owner.selector, roots: roots, identities: head.Binding.OutputRoots}
	if err := s.validate(ctx, owner.selector); err != nil {
		return nil, err
	}
	j, err := roots[2].OpenScopedExecutionJournalPrivate(owner.selector)
	if err != nil {
		return nil, err
	}
	s.journal = j
	return s, nil
}
func (s *scopedExecutionStorePrivate) validate(ctx context.Context, selector string) error {
	if ctx == nil || s == nil || selector != s.selector {
		return ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	for i, root := range s.roots {
		id, err := root.DirectoryIdentity()
		if err != nil {
			return err
		}
		if id != s.identities[i] || id.Inode == 0 {
			return ErrInvalid
		}
		for k := 0; k < i; k++ {
			if id == s.identities[k] {
				return ErrInvalid
			}
		}
	}
	return ctx.Err()
}
func (s *scopedExecutionStorePrivate) Load(ctx context.Context, selector string) ([]byte, error) {
	if err := s.validate(ctx, selector); err != nil {
		return nil, err
	}
	return s.journal.Load(ctx)
}
func (s *scopedExecutionStorePrivate) CompareAbsent(ctx context.Context, selector string, raw []byte) (*publication.CompareAndReplaceReceipt, error) {
	if err := s.validate(ctx, selector); err != nil {
		return nil, err
	}
	return s.journal.CompareAbsent(ctx, raw)
}
func (s *scopedExecutionStorePrivate) Close() error {
	if s == nil {
		return nil
	}
	j := s.journal
	s.journal = nil
	if j != nil {
		return j.Close()
	}
	return nil
}
