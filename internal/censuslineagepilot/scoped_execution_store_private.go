package censuslineagepilot

import (
	"context"
	"encoding/json"

	"lsp-trace/internal/publication"
)

// Minimal private adapter, NOT live-qualified. Roots and topology are borrowed
// and checked before construction and every operation. The whole future
// invocation owns roots and closes them only AFTER all calls and stores finish.
// No sharing/concurrent use/asynchronous Close: topology does not alter the
// borrowed-root API contract. No Manager/readiness or output-budget authority.
type scopedExecutionStorePrivate struct {
	journal    *publication.ScopedExecutionJournalPrivate
	selector   string
	roots      [3]*publication.ContentRetentionRoot
	identities [3]publication.RetentionDirectoryIdentity
	topology   *scopedExecutionTopologyPrivate
	closed     bool
}

var _ scopedExecutionClaimStorePrivate = (*scopedExecutionStorePrivate)(nil)

func newScopedExecutionStorePrivate(ctx context.Context, owner *scopedExecutionClaimOwnerPrivate, roots [3]*publication.ContentRetentionRoot, topology *scopedExecutionTopologyPrivate) (*scopedExecutionStorePrivate, error) {
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
	if topology == nil || topology.state == nil {
		return nil, ErrInvalid
	}
	proof := topology.state
	if roots != proof.outputs || head.Binding.OutputRoots != proof.outputIDs || head.Binding.InputHead != proof.head || head.Binding.Scope != proof.scope || head.Binding.Metadata != proof.metadata {
		return nil, ErrInvalid
	}
	s := &scopedExecutionStorePrivate{selector: owner.selector, roots: roots, identities: head.Binding.OutputRoots, topology: topology}
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
	if ctx == nil || s == nil || s.closed || selector != s.selector {
		return ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := s.topology.validate(ctx); err != nil {
		return err
	}
	if s.topology.state.outputs != s.roots || s.topology.state.outputIDs != s.identities {
		return ErrInvalid
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
	s.closed = true
	j := s.journal
	s.journal = nil
	if j != nil {
		return j.Close()
	}
	return nil
}
