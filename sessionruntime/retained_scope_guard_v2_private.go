package sessionruntime

import (
	"context"
	"errors"

	"lsp-trace/internal/session"
)

// A profile is immutable after minting. Zero preserves the legacy V1 domain.
type retainedScopeProfilePrivate uint8

const (
	retainedScopeV1Private retainedScopeProfilePrivate = iota
	retainedScopeV2Private
)

// RetainedScopeGuardPrivateV2 is an opaque exact-instance equality capability.
// It is not interchangeable with the fixed V1 source-policy capability. Copying
// the pointed-to wrapper invalidates it; copying its pointer preserves identity.
// No API exports its common lifecycle base.
type RetainedScopeGuardPrivateV2 struct{ base *RetainedScopeGuardPrivate }

// MarshalJSON refuses both value and non-nil pointer encoding, including zero
// values. encoding/json retains its standard null encoding for nil pointers.
func (RetainedScopeGuardPrivateV2) MarshalJSON() ([]byte, error) {
	return nil, errors.New("private retained scope V2 capability is not serializable")
}

// MembersPrivate returns detached expectations. Go strings are immutable; byte
// slices are copied. Neither expectations nor this getter supply source bytes.
func (g *RetainedScopeGuardPrivateV2) MembersPrivate() []RetainedScopeMemberPrivate {
	base, valid := resolveRetainedScopePrivate(RoundTripRequest{RetainedScopePrivateV2: g})
	if !valid || base == nil {
		return nil
	}
	return cloneRetainedScopeMembersPrivate(base.members)
}

// BindRetainedScopePrivateV2 binds 1..8 nominations and 0..8 context members,
// at most 16 total. All source must already have genuine captured supply.
// Admission does no filesystem, provider, or wire IO and never supplies Expected.
func (m *Manager) BindRetainedScopePrivateV2(ctx context.Context, id string, generation uint64, members []RetainedScopeMemberPrivate) (*RetainedScopeGuardPrivateV2, session.Failure) {
	base, failure := m.bindRetainedScopePrivate(ctx, id, generation, members, retainedScopeV2Private)
	if failure != "" {
		return nil, failure
	}
	return base.v2Owner, ""
}

// resolveRetainedScopePrivate validates only immutable attachment identity. Live
// Manager, session, generation and lease checks remain in the locked matcher.
// An invalid attachment is never treated as absence or translated to V1.
func resolveRetainedScopePrivate(req RoundTripRequest) (*RetainedScopeGuardPrivate, bool) {
	if req.RetainedScopePrivate != nil && req.RetainedScopePrivateV2 != nil {
		return nil, false
	}
	if g := req.RetainedScopePrivateV2; g != nil {
		if g.base == nil || g.base.profile != retainedScopeV2Private || g.base.v2Owner != g {
			return nil, false
		}
		return g.base, true
	}
	if g := req.RetainedScopePrivate; g != nil {
		if g.profile != retainedScopeV1Private {
			return nil, false
		}
		return g, true
	}
	return nil, true
}

// CheckRetainedScopePrivateV2 checks every member without IO or authority uplift.
func (m *Manager) CheckRetainedScopePrivateV2(ctx context.Context, id string, generation uint64, g *RetainedScopeGuardPrivateV2) session.Failure {
	return m.checkRetainedScopePrivate(ctx, id, generation, RoundTripRequest{RetainedScopePrivateV2: g})
}

// RetainedScopeDiagnosticsPrivateV2 returns the common bounded event snapshot.
// Identity governs access, so a frozen-state mismatch remains observable.
func (m *Manager) RetainedScopeDiagnosticsPrivateV2(g *RetainedScopeGuardPrivateV2) ([]RetainedScopeDiagnosticPrivate, bool) {
	if m == nil {
		return nil, false
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	base, valid := resolveRetainedScopePrivate(RoundTripRequest{RetainedScopePrivateV2: g})
	if !valid || base == nil || base.manager != m || base.session == nil || base.session.retainedScopePrivate != base {
		return nil, false
	}
	return append([]RetainedScopeDiagnosticPrivate(nil), base.diagnostics...), base.diagnosticOverflow
}
