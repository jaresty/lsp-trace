package sessionruntime

import "lsp-trace/internal/managedprocess"

// exactOwner checks configuration without nesting its Manager lock, then binds
// the actual selected record under Manager -> window lock order. Neither lock
// is held over source acquisition or backend I/O.
func (w *privateProviderWindow) exactOwner(m *Manager, id string, generation uint64) bool {
	if !w.spec.matches(m) {
		return false
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return w.exactOwnerLocked(m, m.sessions[id], id, generation)
}

// Manager.mu is held by the caller; all actual record-selection sites use this
// predicate before copies, counters, pending admission, or provider effects.
func (w *privateProviderWindow) exactOwnerLocked(m *Manager, r *runtimeSession, id string, generation uint64) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.spec.manager == m && m.limits == w.spec.limits && m.wire == w.spec.configured && m.readinessTimeout == w.spec.readiness &&
		r != nil && r == w.ownedRecord && r.record.SessionID == id && r.record.Generation == generation &&
		generation != 0 && w.start.Failure == "" && w.start.Start.Kind == managedprocess.StartStarted &&
		w.start.SessionID == id && w.start.Generation == generation &&
		w.child != nil && r.process == w.child && w.child.window == w
}

// privateWindowGenerationLocked identifies the retained private generation even
// after its process pointer is retired or cleared. Manager.mu is held; never
// invert the Manager -> window lock order or infer ownership from scalar IDs.
func (m *Manager) privateWindowGenerationLocked(r *runtimeSession) bool {
	if r == nil {
		return false
	}
	if _, ok := r.process.(*privateProviderWindowChild); ok {
		return true
	}
	if r.retired != nil {
		if _, ok := r.retired.child.(*privateProviderWindowChild); ok {
			return true
		}
	}
	if w := m.privateProviderWindow; w != nil {
		w.mu.Lock()
		owned := w.ownedRecord == r
		w.mu.Unlock()
		return owned
	}
	return false
}

// Admission occurs before shared-transaction copies, pending IDs or effects.
func (w *privateProviderWindow) admitTransaction(m *Manager, req RoundTripRequest) bool {
	if !w.exactOwner(m, req.SessionID, req.Generation) || !req.Deadline.Equal(w.deadline) || len(req.Params) > 32<<10 || req.MaxMessages != 4 || req.MaxBytes <= 0 || uint64(req.MaxBytes) > w.spec.bounds.BodyBytes || len(req.SessionID) > 4096 || len(req.DiagnosticTargetID) > 4096 || len(req.DiagnosticCallerID) > 4096 {
		return false
	}
	w.mu.Lock()
	same := req.SessionID == w.start.SessionID && req.Generation == w.start.Generation
	w.mu.Unlock()
	if !same {
		return false
	}
	caps := []int64{req.CaptureMethodRequestFrameMaxBytes, req.CaptureDocumentSymbolRequestFrameMaxBytes, req.CaptureDocumentSymbolResponseFrameMaxBytes, req.CaptureDefinitionResponseFrameMaxBytes, req.CaptureReferencesResponseFrameMaxBytes}
	for _, n := range caps {
		if n < 0 || uint64(n) > w.spec.bounds.FrameBytes {
			return false
		}
	}
	switch req.Method {
	case "textDocument/prepareCallHierarchy", "callHierarchy/incomingCalls", "callHierarchy/outgoingCalls":
		if req.directionalV5 == nil {
			return false
		}
	}
	return w.enter(req.Method)
}
