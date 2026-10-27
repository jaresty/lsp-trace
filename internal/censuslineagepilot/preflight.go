package censuslineagepilot

import (
	"bytes"
	"context"
	"errors"
	"io"

	"lsp-trace/internal/publication"
)

// PreflightRequest admits only comparison references and workflow/lineage labels,
// not manifests, trusted plans, expected head revisions, or execution policy.
// Inputs must name exactly the two fixed input kinds at root-level content IDs.
type PreflightRequest struct {
	RootPath, WorkflowKey, LineageID string
	Inputs                           []Artifact
}

const MonitoringNotQualified = "MONITORING_NOT_QUALIFIED"

// PreflightResult has no public trust fields. Even a populated result is only a
// read-only observation: not monitoring qualification, sealing, or runtime admission.
type PreflightResult struct{ state *preflightState }
type preflightState struct {
	inputs                 ObservedInputs
	workflowKey, lineageID string
}

func (r PreflightResult) Authority() uint8 { return 0 }
func (r PreflightResult) Status() string   { return MonitoringNotQualified }
func (r PreflightResult) Inputs() ObservedInputs {
	if r.state == nil {
		return ObservedInputs{}
	}
	return r.state.inputs
}
func (r PreflightResult) WorkflowKey() string {
	if r.state == nil {
		return ""
	}
	return r.state.workflowKey
}
func (r PreflightResult) LineageID() string {
	if r.state == nil {
		return ""
	}
	return r.state.lineageID
}

// The acquired capability deliberately has no write, Put, CAS, or Coordinate API.
type preflightRoot interface {
	ValidatePrivate() error
	ReadContentObserved(context.Context, string, int64) ([]byte, error)
	Close() error
}

// Preflight validates the durable-root policy, then independently enforces it
// during descriptor-traversed no-follow acquisition. The initial pathname check
// alone grants no authority. It retains the admitted capability through
// observation and closes it before returning. No ambient checkout lookup,
// root creation, cleanup, Census execution, or persistent mutation occurs.
// Context is required and checked at safe read boundaries. Byte-bounded regular
// file I/O is not arbitrary-I/O interruption or an elapsed-time deadline guarantee.
func Preflight(ctx context.Context, req PreflightRequest) (PreflightResult, error) {
	return preflightWithOpen(ctx, req, func(path string) (preflightRoot, error) { return openPreflightRoot(ctx, path) })
}
func openPreflightRoot(ctx context.Context, path string) (preflightRoot, error) {
	return publication.OpenObservedRoot(ctx, path)
}

func preflightWithOpen(ctx context.Context, req PreflightRequest, open func(string) (preflightRoot, error)) (out PreflightResult, err error) {
	if ctx == nil {
		return out, ErrInvalid
	}
	if err = ctx.Err(); err != nil {
		return out, err
	}
	if req.WorkflowKey == "" || req.LineageID == "" || len(req.Inputs) != 2 {
		return out, ErrInvalid
	}
	refs := append([]Artifact(nil), req.Inputs...)
	byKind := make(map[string]Artifact, 2)
	seen := make(map[string]bool, 2)
	for _, a := range refs {
		limit := inputKindLimit(a.Kind)
		if limit == 0 || byKind[a.Kind].Kind != "" || seen[a.Selector] || !validDigest(a.Selector) || a.Selector != a.SHA256 || a.ByteLength == 0 || a.ByteLength > uint64(limit) {
			return out, ErrInvalid
		}
		byKind[a.Kind] = a
		seen[a.Selector] = true
	}
	if err = ctx.Err(); err != nil {
		return out, err
	}
	if err = ValidateDurableRoot(req.RootPath); err != nil {
		return out, err
	}
	if err = ctx.Err(); err != nil {
		return out, err
	}
	root, err := open(req.RootPath)
	// An acquired handle, including one returned alongside an error by a test
	// transport, is always closed; root close failure invalidates all output state.
	if root != nil {
		defer func() {
			err = errors.Join(err, root.Close(), ctx.Err())
			if err != nil {
				out = PreflightResult{}
			}
		}()
	}
	if err != nil {
		return out, err
	}
	if root == nil {
		return out, ErrInvalid
	}
	if err = ctx.Err(); err != nil {
		return out, err
	}
	if err = root.ValidatePrivate(); err != nil {
		return out, err
	}
	source := &filesystemInputSource{ctx: ctx, root: root, refs: byKind}
	observed, err := ObserveInputs(source, DefaultInputLimits(), refs)
	if err != nil {
		return out, err
	}
	if err = ctx.Err(); err != nil {
		return out, err
	}
	if err = root.ValidatePrivate(); err != nil {
		return out, err
	}
	return PreflightResult{state: &preflightState{observed, req.WorkflowKey, req.LineageID}}, nil
}
func inputKindLimit(kind string) int64 {
	switch kind {
	case SourceSelectionKind:
		return MaxSelectionInputBytes
	case PartitionPlanKind:
		return MaxPlanInputBytes
	}
	return 0
}

type filesystemInputSource struct {
	ctx  context.Context
	root preflightRoot
	refs map[string]Artifact
}

func (s *filesystemInputSource) OpenInput(kind string) (io.ReadCloser, error) {
	if err := s.ctx.Err(); err != nil {
		return nil, err
	}
	a, ok := s.refs[kind]
	limit := inputKindLimit(kind)
	if !ok || limit == 0 {
		return nil, ErrInvalid
	}
	raw, err := s.root.ReadContentObserved(s.ctx, a.Selector, limit)
	if err != nil {
		return nil, err
	}
	if err = s.ctx.Err(); err != nil {
		return nil, err
	}
	if uint64(len(raw)) != a.ByteLength || Digest(raw) != a.SHA256 {
		return nil, ErrInvalid
	}
	return io.NopCloser(preflightContextReader{s.ctx, bytes.NewReader(raw)}), nil
}

type preflightContextReader struct {
	ctx context.Context
	r   io.Reader
}

func (r preflightContextReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	n, err := r.r.Read(p)
	if canceled := r.ctx.Err(); canceled != nil {
		return n, errors.Join(err, canceled)
	}
	return n, err
}
