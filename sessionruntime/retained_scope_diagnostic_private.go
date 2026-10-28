package sessionruntime

// RetainedScopeDiagnosticPrivate is a non-authoritative observation. All string
// values are closed literals supplied by Manager, never caller text or URIs.
// It deliberately carries no capability, source bytes or session credentials.
type RetainedScopeDiagnosticPrivate struct {
	Phase    string `json:"p"`
	Method   string `json:"m"`
	Ordinal  uint64 `json:"o"`
	URIClass string `json:"u"`
	Reason   string `json:"r"`
}

// RetainedScopeDiagnosticsPrivate returns a detached bounded snapshot. Overflow
// is explicit; observations neither mint authority nor change guard eligibility.
// The caller must already hold the exact Manager-minted capability.
func (m *Manager) RetainedScopeDiagnosticsPrivate(g *RetainedScopeGuardPrivate) ([]RetainedScopeDiagnosticPrivate, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if g == nil || g.manager != m || g.session == nil || g.session.retainedScopePrivate != g {
		return nil, false
	}
	return append([]RetainedScopeDiagnosticPrivate(nil), g.diagnostics...), g.diagnosticOverflow
}
