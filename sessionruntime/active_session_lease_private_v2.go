package sessionruntime

import (
	"context"

	"lsp-trace/internal/session"
)

// CheckActiveSessionLeasePrivateV2 is a read-only, stronger private-producer
// precondition. Unlike ordinary lease-compatible admission, it REQUIRES the
// Manager's currently active opaque token. It grants no lease or new authority,
// performs no IO, and does not alter validSessionLeaseLocked's tokenless rules.
func (m *Manager) CheckActiveSessionLeasePrivateV2(ctx context.Context, req SessionLeaseRequest) session.Failure {
	if ctx == nil || m == nil {
		return session.LifecycleConflict
	}
	if failure := contextFailure(ctx); failure != "" {
		return failure
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if failure := contextFailure(ctx); failure != "" {
		return failure
	}
	r := m.sessions[req.SessionID]
	if r == nil || r.record.SessionID != req.SessionID {
		return session.SessionNotFound
	}
	if r.record.Generation != req.Generation {
		return session.StaleGeneration
	}
	if r.record.State == session.Poisoned {
		return session.SessionPoisoned
	}
	if m.closed || r.record.State != session.Ready || r.record.Profile.Workspace().String() != req.Workspace || r.protocolOwned || r.lifecycleOwned || r.leaseClosing || r.leaseCount != 1 || r.leaseToken == nil || !m.validSessionLeaseLocked(ctx, r) {
		return session.LifecycleConflict
	}
	return ""
}
