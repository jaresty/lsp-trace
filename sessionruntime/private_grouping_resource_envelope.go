package sessionruntime

import (
	"context"
	"io"
	"sync"
	"time"

	"lsp-trace/internal/lspwire"
)

// PrivateGroupingResourceEnvelope is an opaque Manager/config snapshot, not a
// caller assertion or authority. Only this package can construct a valid value.
// These closed bounds apply ONLY to private grouping, never ordinary defaults.
type PrivateGroupingResourceEnvelope struct {
	manager      *Manager
	configured   lspwire.Limits
	body, header uint64
}

type PrivateGroupingResourceBounds struct {
	SourceBytes, SourceReadBytes, BodyBytes, HeaderBytes, FrameBytes uint64
	RequestBytes, ResponseBytes, WireBytes, Messages, NativeRequests uint64
}

// PrivateGroupingResourceEnvelope returns actual enforced private settings.
// Reader <=0 defaults normalize exactly like lspwire.Limits.normalized. Larger
// than ordinary defaults refuses rather than silently accepting unknown owners.
func (m *Manager) PrivateGroupingResourceEnvelope() (PrivateGroupingResourceEnvelope, bool) {
	if m == nil {
		return PrivateGroupingResourceEnvelope{}, false
	}
	m.mu.Lock()
	w := m.wire
	m.mu.Unlock()
	d := lspwire.DefaultLimits()
	n := w
	if n.MaxBodyBytes <= 0 {
		n.MaxBodyBytes = d.MaxBodyBytes
	}
	if n.MaxHeaderBytes <= 0 {
		n.MaxHeaderBytes = d.MaxHeaderBytes
	}
	if n.MaxBodyBytes > d.MaxBodyBytes || n.MaxHeaderBytes > d.MaxHeaderBytes {
		return PrivateGroupingResourceEnvelope{}, false
	}
	if n.MaxBodyBytes > 128<<10 {
		n.MaxBodyBytes = 128 << 10
	}
	if n.MaxHeaderBytes > 4<<10 {
		n.MaxHeaderBytes = 4 << 10
	}
	return PrivateGroupingResourceEnvelope{m, w, uint64(n.MaxBodyBytes), uint64(n.MaxHeaderBytes)}, true
}

func (e PrivateGroupingResourceEnvelope) Bounds() PrivateGroupingResourceBounds {
	if e.manager == nil || e.body == 0 || e.header == 0 {
		return PrivateGroupingResourceBounds{}
	}
	const source = 16 << 10
	// Worst-case JSON escaping of source, URI and language, envelope, framing.
	const supply = 6*(source+2*4096) + 8192
	// Each method admits <=32KiB params. Include JSON/ID/method/header and one
	// cancellation per method. These prepaid allowances are debited before send.
	const request = 3 * (6*(32<<10) + 4096 + 4096)
	frame := e.body + e.header
	response := 27 * (frame + 1) // rejecting header byte, not just successful frames
	return PrivateGroupingResourceBounds{source, source + 1, e.body, e.header, frame, supply + request, response, supply + request + response, 31, 7}
}
func (m *Manager) privateGroupingEnvelopeMatches(e PrivateGroupingResourceEnvelope) bool {
	current, ok := m.PrivateGroupingResourceEnvelope()
	return ok && current == e
}

// PrivateGroupingEnvelopeContext installs one enforcement residual for a paid
// operation. Existing contexts cannot reset their residual. This is enforcement,
// not a second grant/ledger; the kernel remains the sole funding authority.
func (m *Manager) PrivateGroupingEnvelopeContext(ctx context.Context, e PrivateGroupingResourceEnvelope, deadline time.Time) (context.Context, error) {
	if ctx == nil || ctx.Err() != nil || !m.privateGroupingEnvelopeMatches(e) || !time.Now().Before(deadline) {
		return nil, ErrPrivateGroupingOwned
	}
	if s, _ := ctx.Value(privateGroupingResourceKey{}).(*privateGroupingResources); s != nil {
		if s.manager != m || s.envelope != e || !s.deadline.Equal(deadline) {
			return nil, ErrPrivateGroupingOwned
		}
		return ctx, nil
	}
	b := e.Bounds()
	return context.WithValue(ctx, privateGroupingResourceKey{}, &privateGroupingResources{manager: m, envelope: e, deadline: deadline, remaining: b.WireBytes}), nil
}

type privateGroupingResourceKey struct{}
type privateGroupingResources struct {
	mu               sync.Mutex
	manager          *Manager
	envelope         PrivateGroupingResourceEnvelope
	deadline         time.Time
	remaining        uint64
	methods, frames  uint64
	supplied, failed bool
}

func privateGroupingResourcesFrom(ctx context.Context) *privateGroupingResources {
	if ctx == nil {
		return nil
	}
	s, _ := ctx.Value(privateGroupingResourceKey{}).(*privateGroupingResources)
	return s
}
func (s *privateGroupingResources) valid(ctx context.Context, m *Manager) bool {
	return s != nil && s.manager == m && ctx != nil && ctx.Err() == nil && time.Now().Before(s.deadline) && m.privateGroupingEnvelopeMatches(s.envelope)
}
func (s *privateGroupingResources) debit(n uint64) bool {
	if s.failed || n > s.remaining {
		s.failed = true
		return false
	}
	s.remaining -= n
	return true
}
func (s *privateGroupingResources) prepare(ctx context.Context, m *Manager, req DocumentRequest) bool {
	if !s.valid(ctx, m) || !req.PreparedNoFollow || !req.CaptureSupply || !PrivateGroupingFieldValid(req.URI) || (req.LanguageID != "" && !PrivateGroupingFieldValid(req.LanguageID)) {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.supplied {
		s.failed = true
		return false
	}
	s.supplied = true
	return s.debit(6*((16<<10)+2*4096) + 8192)
}
func (s *privateGroupingResources) admit(ctx context.Context, m *Manager, req RoundTripRequest) bool {
	if !s.valid(ctx, m) || !req.Deadline.Equal(s.deadline) || len(req.Params) > 32<<10 || req.MaxMessages != 4 || req.MaxBytes != 65536 || !req.CaptureOwnedMethodPair || req.ExpectedOwnedDocument == nil || req.directionalV5 != nil || req.callHierarchyLayer1GenericAttempt != nil {
		return false
	}
	switch req.Method {
	case "textDocument/documentSymbol", "textDocument/references", "textDocument/definition":
	default:
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.supplied || s.methods >= 3 {
		s.failed = true
		return false
	}
	s.methods++
	return s.debit(6*(32<<10) + 4096 + 4096)
}
func (s *privateGroupingResources) beginFrame() (int64, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.failed || s.frames >= 27 || s.remaining == 0 {
		s.failed = true
		return 0, false
	}
	s.frames++
	n := s.envelope.Bounds().FrameBytes
	if n > s.remaining {
		n = s.remaining
	}
	return int64(n), true
}

type privateGroupingResourceReader struct {
	source io.Reader
	state  *privateGroupingResources
	ctx    context.Context
}

func (r privateGroupingResourceReader) Read(p []byte) (int, error) {
	if !r.state.valid(r.ctx, r.state.manager) {
		return 0, ErrPrivateGroupingOwned
	}
	r.state.mu.Lock()
	defer r.state.mu.Unlock()
	if r.state.failed || r.state.remaining == 0 {
		r.state.failed = true
		return 0, ErrPrivateGroupingOwned
	}
	if uint64(len(p)) > r.state.remaining {
		p = p[:int(r.state.remaining)]
	}
	n, err := r.source.Read(p)
	if n < 0 || n > len(p) {
		r.state.failed = true
		return 0, ErrPrivateGroupingOwned
	}
	r.state.remaining -= uint64(n)
	if err != nil {
		r.state.failed = true
	}
	return n, err
}
