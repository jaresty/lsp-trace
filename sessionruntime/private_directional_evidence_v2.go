package sessionruntime

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"io"
	"time"

	"lsp-trace/internal/directionalraw"
	"lsp-trace/internal/managedprocess"
)

var ErrPrivateDirectionalEvidenceV2 = errors.New("private directional evidence refused")

const PrivateDirectionalEvidenceMaxBytesV2 = 65536

type privateDirectionalEvidenceKeyV2 struct{}

// PrivateDirectionalRequestObservationV2 preserves the actual request ordinal.
// View-local request IDs belong to the downstream coordinator, not this ledger.
type PrivateDirectionalRequestObservationV2 struct {
	Ordinal               int
	Method                string
	RequestID, Generation uint64
	Params, Result        json.RawMessage
	RequestFrame          []byte
	ResponseFrames        [][]byte
	Wire                  directionalraw.CompletionMetadataPrivate
	Observations          []directionalraw.Observation
}

type PrivateDirectionalEvidenceViewV2 struct {
	OperationID string
	StartedAt   time.Time
	Process     managedprocess.Spec
	Session     SessionLeaseRequest
	Request     DirectionalCallHierarchyRequestPrivate
	Metadata    SessionMetadata
	Supply      DocumentSupply
	Requests    []PrivateDirectionalRequestObservationV2
}

// The zero value and caller-created DTOs cannot mint this companion.
type PrivateDirectionalEvidenceV2 struct{ state *privateDirectionalEvidenceV2 }
type privateDirectionalEvidenceV2 struct {
	manager      *Manager
	token        *sessionLeaseToken
	view         PrivateDirectionalEvidenceViewV2
	used         bool // protected by manager.mu, shared by all handle copies
	valid        bool
	failure      error
	sourceDigest [32]byte
}

func (m *Manager) CaptureDirectionalEvidencePrivateV2(ctx context.Context, s SessionLeaseRequest, q DirectionalCallHierarchyRequestPrivate) (PrivateDirectionalEvidenceV2, error) {
	if m == nil || ctx == nil || m.CheckActiveSessionLeasePrivateV2(ctx, s) != "" || q.SessionID != s.SessionID || q.Generation != s.Generation || q.Symbol != "" || q.Line == nil || q.Character == nil || q.MaxBytes < 1 || q.MaxBytes > PrivateDirectionalEvidenceMaxBytesV2 || q.MaxMessages < 1 || q.MaxMessages > 32 || q.Deadline.IsZero() || !time.Now().Before(q.Deadline) || len(q.URI) > 4096 {
		return PrivateDirectionalEvidenceV2{}, ErrPrivateDirectionalEvidenceV2
	}
	meta, f := m.Metadata(s.SessionID, s.Generation)
	if f != "" || !meta.CallHierarchySupport || len(meta.PositionEncoding) > 32 || len(meta.ServerCommand) > 4096 || len(meta.ProviderName) > 4096 || len(meta.ProviderVersion) > 4096 || len(s.SessionID) > 4096 || len(s.Workspace) > 4096 || len(q.LanguageID) > 4096 {
		return PrivateDirectionalEvidenceV2{}, ErrPrivateDirectionalEvidenceV2
	}
	token, _ := ctx.Value(sessionLeaseContextKey{}).(*sessionLeaseToken)
	m.mu.Lock()
	r := m.sessions[s.SessionID]
	if r == nil || !m.validSessionLeaseLocked(ctx, r) {
		m.mu.Unlock()
		return PrivateDirectionalEvidenceV2{}, ErrPrivateDirectionalEvidenceV2
	}
	if !m.directionalContextPrivateV2(ctx, r, q.Deadline) {
		m.mu.Unlock()
		return PrivateDirectionalEvidenceV2{}, ErrPrivateDirectionalEvidenceV2
	}
	spec := r.spec
	if len(spec.Path) > 4096 || len(spec.Dir) > 4096 || len(spec.Args) > 128 || len(spec.Env) > 128 {
		m.mu.Unlock()
		return PrivateDirectionalEvidenceV2{}, ErrPrivateDirectionalEvidenceV2
	}
	for _, fields := range [][]string{spec.Args, spec.Env} {
		for _, field := range fields {
			if len(field) > 4096 {
				m.mu.Unlock()
				return PrivateDirectionalEvidenceV2{}, ErrPrivateDirectionalEvidenceV2
			}
		}
	}
	spec.Args = append([]string(nil), spec.Args...)
	spec.Env = append([]string(nil), spec.Env...)
	m.mu.Unlock()
	line, character := *q.Line, *q.Character
	q.Line, q.Character = &line, &character
	c := &privateDirectionalEvidenceV2{manager: m, token: token, view: PrivateDirectionalEvidenceViewV2{Session: s, Request: q, Metadata: meta, Process: spec, StartedAt: time.Now()}}
	_, err := m.ExecuteCallHierarchyDirectionsPrivate(context.WithValue(ctx, privateDirectionalEvidenceKeyV2{}, c), q)
	if err != nil || c.failure != nil || len(c.view.Requests) != 3 || m.CheckActiveSessionLeasePrivateV2(ctx, s) != "" {
		return PrivateDirectionalEvidenceV2{}, errors.Join(ErrPrivateDirectionalEvidenceV2, err, c.failure)
	}
	c.valid = true
	return PrivateDirectionalEvidenceV2{c}, nil
}

// detachDirectionalEvidencePrivateV2 is called while the immutable reader is
// live. Length admission precedes allocation; hashes are observations, not minting.
func detachDirectionalEvidencePrivateV2(ctx context.Context, method string, params []byte, lease directionalraw.Lease, observations []directionalraw.Observation) error {
	c, _ := ctx.Value(privateDirectionalEvidenceKeyV2{}).(*privateDirectionalEvidenceV2)
	if c == nil {
		return nil
	}
	meta, ok := lease.CompletedMetadataPrivate()
	if !ok || meta.ResultLengthPrivate == 0 || meta.ResultLengthPrivate > PrivateDirectionalEvidenceMaxBytesV2 || len(params) > PrivateDirectionalEvidenceMaxBytesV2 || len(c.view.Requests) >= 3 || len(observations) > 1024 {
		return ErrPrivateDirectionalEvidenceV2
	}
	requestFrame, responseFrames, ok := lease.CompletedFramesPrivate(meta.RequestIDPrivate, meta.GenerationPrivate, 2*PrivateDirectionalEvidenceMaxBytesV2, c.view.Request.MaxMessages)
	if !ok {
		return ErrPrivateDirectionalEvidenceV2
	}
	raw, err := io.ReadAll(io.LimitReader(lease.ReaderPrivate(), int64(meta.ResultLengthPrivate)+1))
	if err != nil || uint64(len(raw)) != meta.ResultLengthPrivate {
		return ErrPrivateDirectionalEvidenceV2
	}
	c.view.Requests = append(c.view.Requests, PrivateDirectionalRequestObservationV2{Ordinal: len(c.view.Requests), Method: method, RequestID: meta.RequestIDPrivate, Generation: meta.GenerationPrivate, Params: append(json.RawMessage(nil), params...), Result: raw, RequestFrame: requestFrame, ResponseFrames: responseFrames, Wire: meta, Observations: append([]directionalraw.Observation(nil), observations...)})
	return nil
}

// directionalContextPrivateV2 is evaluated under Manager.mu. Actual window
// sessions cannot escape through a stripped context or a replacement deadline.
// Ordinary sessions continue to require only their actual active lease.
func (m *Manager) directionalContextPrivateV2(ctx context.Context, r *runtimeSession, deadline time.Time) bool {
	w := privateProviderWindowFrom(ctx)
	if child, ok := r.process.(*privateProviderWindowChild); ok {
		d, hasDeadline := ctx.Deadline()
		if w != child.window || !hasDeadline || !d.Equal(w.deadline) || !deadline.Equal(w.deadline) || !privateProviderWindowInner(ctx) || !w.exactOwnerLocked(m, r, r.record.SessionID, r.record.Generation) {
			return false
		}
		w.mu.Lock()
		defer w.mu.Unlock()
		return w.liveLocked() && w.active == 1 && !w.released
	}
	return w == nil
}

func (m *Manager) directionalEvidenceValidPrivateV2(ctx context.Context, c *privateDirectionalEvidenceV2, taken bool) bool {
	token, _ := ctx.Value(sessionLeaseContextKey{}).(*sessionLeaseToken)
	r := m.sessions[c.view.Session.SessionID]
	if c.manager != m || !c.valid || c.used != taken || c.token == nil || token != c.token || r == nil || m.closed || r.record.Generation != c.view.Session.Generation || !m.validSessionLeaseLocked(ctx, r) || r.lifecycleOwned || ctx.Err() != nil || !time.Now().Before(c.view.Request.Deadline) || !m.directionalContextPrivateV2(ctx, r, c.view.Request.Deadline) {
		return false
	}
	if taken {
		if !token.directionalTaken || !r.protocolOwned {
			return false
		}
	} else if token.directionalTaken || r.protocolOwned {
		return false
	}
	doc, ok := r.documents[c.view.Supply.URI]
	digest := c.sourceDigest
	if !taken {
		digest = sha256.Sum256(c.view.Supply.Content)
	}
	return ok && doc.supply != nil && doc.version == c.view.Supply.DocumentVersion && digest == doc.digest && doc.supply.SessionID == c.view.Session.SessionID && doc.supply.Generation == c.view.Session.Generation
}

// Take transfers actual detached producer bytes exactly once. It executes no
// caller code. The existing active lease owns protocol exclusion until its
// callback actually returns; there is deliberately no caller completion API.
func (m *Manager) TakeDirectionalEvidencePrivateV2(ctx context.Context, h PrivateDirectionalEvidenceV2) (PrivateDirectionalEvidenceViewV2, error) {
	if m == nil || ctx == nil || h.state == nil {
		return PrivateDirectionalEvidenceViewV2{}, ErrPrivateDirectionalEvidenceV2
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	c := h.state
	if !m.directionalEvidenceValidPrivateV2(ctx, c, false) {
		return PrivateDirectionalEvidenceViewV2{}, ErrPrivateDirectionalEvidenceV2
	}
	c.used = true
	c.token.directionalTaken = true
	c.token.session.protocolOwned = true
	c.sourceDigest = sha256.Sum256(c.view.Supply.Content)
	v := c.view
	c.view.Requests = nil
	c.view.Supply.Content = nil
	c.view.Supply.Params = nil
	return v, nil
}

// CheckTakenDirectionalEvidencePrivateV2 performs bounded final validation only;
// it neither releases ownership nor admits caller-authored data.
func (m *Manager) CheckTakenDirectionalEvidencePrivateV2(ctx context.Context, h PrivateDirectionalEvidenceV2) error {
	if m == nil || ctx == nil || h.state == nil {
		return ErrPrivateDirectionalEvidenceV2
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.directionalEvidenceValidPrivateV2(ctx, h.state, true) {
		return ErrPrivateDirectionalEvidenceV2
	}
	return nil
}
