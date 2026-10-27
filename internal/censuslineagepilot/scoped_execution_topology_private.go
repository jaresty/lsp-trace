package censuslineagepilot

import (
	"context"
	"errors"

	"lsp-trace/internal/publication"
)

// Opaque, invocation-local topology observation, never serialized or resumable.
// The whole future invocation owns the output roots; this proof borrows them.
// It owns only read-only reconstructions of the exact retained-reader paths.
// No sharing, concurrent use, or asynchronous Close is supported: finish ALL
// owner/store calls, close stores, close this proof, then close output roots.
// Topology proof does NOT change ContentRetentionRoot's borrowed-root API contract.
// It grants no execution/readiness authority or output-retention budget.
type scopedExecutionTopologyPrivate struct {
	state *scopedExecutionTopologyStatePrivate
}
type scopedExecutionTopologyStatePrivate struct {
	inputs                [3]*publication.ObservedRoot
	outputs               [3]*publication.ContentRetentionRoot
	inputIDs, outputIDs   [3]publication.RetentionDirectoryIdentity
	head, scope, metadata scopeRestartBlob
	close                 func(*publication.ObservedRoot) error
}

type scopedExecutionTopologyOpsPrivate struct {
	open  func(context.Context, string) (*publication.ObservedRoot, error)
	close func(*publication.ObservedRoot) error
}

// Reopen only the EXACT paths the retained reader admitted, using its read-only
// opening layer. Match its pinned identities before doing any topology work.
// No new bundle walk, URI rebasing, source checkout admission, or input writes.
func preflightScopedExecutionRootsPrivate(ctx context.Context, restart scopeRestartResult, paths [3]string, outputs [3]*publication.ContentRetentionRoot) (*scopedExecutionTopologyPrivate, error) {
	return preflightScopedExecutionRootsWithOpsPrivate(ctx, restart, paths, outputs, scopedExecutionTopologyOpsPrivate{
		open:  publication.OpenObservedRoot,
		close: func(r *publication.ObservedRoot) error { return r.Close() },
	})
}
func preflightScopedExecutionRootsWithOpsPrivate(ctx context.Context, restart scopeRestartResult, paths [3]string, outputs [3]*publication.ContentRetentionRoot, ops scopedExecutionTopologyOpsPrivate) (out *scopedExecutionTopologyPrivate, err error) {
	if ctx == nil || restart.state == nil || ops.open == nil || ops.close == nil {
		return nil, ErrInvalid
	}
	if err = ctx.Err(); err != nil {
		return nil, err
	}
	h, err := parseScopeBundleHead(restart.state.head)
	if err != nil || h.State != "RETAINED_INPUT_BUNDLE" || h.Binding.Roots != restart.state.roots || !validScopeBundleBinding(h.Binding) {
		return nil, ErrInvalid
	}
	s := &scopedExecutionTopologyStatePrivate{outputs: outputs, inputIDs: restart.state.roots, close: ops.close,
		head:     scopeRestartBlob{Digest(restart.state.head), uint64(len(restart.state.head))},
		scope:    scopeRestartBlob{Digest(restart.state.scope.ManifestBytes()), uint64(len(restart.state.scope.ManifestBytes()))},
		metadata: scopeRestartBlob{Digest(restart.state.metadata), uint64(len(restart.state.metadata))},
	}
	p := &scopedExecutionTopologyPrivate{state: s}
	defer func() {
		err = errors.Join(err, ctx.Err())
		if err != nil {
			err = errors.Join(err, p.Close())
			out = nil
		}
	}()
	for i, path := range paths {
		s.inputs[i], err = ops.open(ctx, path)
		if err != nil {
			return nil, err
		}
		id, e := s.inputs[i].DirectoryIdentity()
		if e != nil {
			return nil, e
		}
		if id != s.inputIDs[i] {
			return nil, ErrInvalid
		}
	}
	for i, r := range outputs {
		s.outputIDs[i], err = r.DirectoryIdentity()
		if err != nil {
			return nil, err
		}
	}
	if err = p.validate(ctx); err != nil {
		return nil, err
	}
	return p, nil
}
func (p *scopedExecutionTopologyPrivate) validate(ctx context.Context) error {
	if ctx == nil || p == nil || p.state == nil {
		return ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	s := p.state
	for i := range s.inputs {
		id, err := s.inputs[i].DirectoryIdentity()
		if err != nil {
			return err
		}
		if id != s.inputIDs[i] || id.Inode == 0 {
			return ErrInvalid
		}
		id, err = s.outputs[i].DirectoryIdentity()
		if err != nil {
			return err
		}
		if id != s.outputIDs[i] || id.Inode == 0 {
			return ErrInvalid
		}
	}
	return publication.CheckRetentionRootTopologyPrivate(ctx, s.outputs, s.inputs)
}
func (p *scopedExecutionTopologyPrivate) Close() (err error) {
	if p == nil || p.state == nil {
		return nil
	}
	s := p.state
	// Consume the shared state as well as this pointer; a copied opaque value
	// cannot resurrect the read-only capabilities after a close error.
	for i := 2; i >= 0; i-- {
		r := s.inputs[i]
		s.inputs[i] = nil
		if r != nil {
			err = errors.Join(err, s.close(r))
		}
	}
	p.state = nil
	return err
}
