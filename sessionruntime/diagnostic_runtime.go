package sessionruntime

import (
	"lsp-trace/internal/manageddiagnostic"
	"lsp-trace/internal/managedprocess"
)

// Diagnostic handles are opaque manager-owned capabilities. They contain no
// command, path, environment, document, or protocol payload bytes.
type diagnosticCapability struct{ identity byte }

type DiagnosticSessionHandle struct {
	attemptID  manageddiagnostic.StartupAttemptID
	sessionID  string
	capability *diagnosticCapability
}

type DiagnosticGenerationHandle struct {
	session    DiagnosticSessionHandle
	generation uint64
}

type DiagnosticOperationHandle struct {
	generation DiagnosticGenerationHandle
	sequence   uint64
}

type DiagnosticSnapshot struct {
	Operation       DiagnosticOperationHandle
	Events          manageddiagnostic.EventSnapshot
	ProcessIdentity managedprocess.Identity
}

type diagnosticOperation struct {
	collector  *manageddiagnostic.EventCollector
	identity   managedprocess.Identity
	attemptID  manageddiagnostic.StartupAttemptID
	sessionID  string
	generation uint64
	sequence   uint64
	capability *diagnosticCapability
}

const (
	diagnosticEventBegin uint16 = iota + 1
	diagnosticEventWriteAttempt
	diagnosticEventWriteComplete
	diagnosticEventReadComplete
	diagnosticEventMatched
	diagnosticEventUnmatched
	diagnosticEventLate
	diagnosticEventInitialized
	diagnosticEventCacheHit
	diagnosticEventTerminalResponse
	diagnosticEventTerminalDeadline
	diagnosticEventTerminalFailure
)

func (m *Manager) newDiagnosticGeneration(attempt manageddiagnostic.StartupAttemptID, session string, generation uint64) DiagnosticGenerationHandle {
	return DiagnosticGenerationHandle{session: DiagnosticSessionHandle{attemptID: attempt, sessionID: session, capability: new(diagnosticCapability)}, generation: generation}
}

func (m *Manager) newDiagnosticOperation(g DiagnosticGenerationHandle, identity managedprocess.Identity) (DiagnosticOperationHandle, *manageddiagnostic.EventCollector) {
	if m.diagnostics == nil {
		return DiagnosticOperationHandle{}, nil
	}
	m.mu.Lock()
	m.diagnosticSequence++
	h := DiagnosticOperationHandle{generation: g, sequence: m.diagnosticSequence}
	c := manageddiagnostic.NewEventCollector(m.limits.MaxObservations, m.now)
	m.diagnosticOperations[h] = diagnosticOperation{collector: c, identity: identity, attemptID: g.session.attemptID, sessionID: g.session.sessionID, generation: g.generation, sequence: m.diagnosticSequence, capability: g.session.capability}
	m.diagnosticOrder = append(m.diagnosticOrder, h)
	m.trimDiagnosticOperationsLocked()
	m.mu.Unlock()
	c.Record(diagnosticEventBegin, 0, false)
	return h, c
}

func (m *Manager) trimDiagnosticOperationsLocked() {
	for len(m.diagnosticOrder) > m.limits.MaxOperations {
		evicted := -1
		for i, h := range m.diagnosticOrder {
			if op := m.diagnosticOperations[h]; op.collector == nil || op.collector.Snapshot().Closed {
				evicted = i
				break
			}
		}
		if evicted < 0 {
			return
		}
		h := m.diagnosticOrder[evicted]
		delete(m.diagnosticOperations, h)
		m.diagnosticEvictions++
		copy(m.diagnosticOrder[evicted:], m.diagnosticOrder[evicted+1:])
		m.diagnosticOrder = m.diagnosticOrder[:len(m.diagnosticOrder)-1]
	}
}

func (m *Manager) completeDiagnosticOperationLocked(h DiagnosticOperationHandle, c *manageddiagnostic.EventCollector, terminal uint16) {
	if c == nil {
		return
	}
	c.Terminal(terminal)
	m.trimDiagnosticOperationsLocked()
}

func (m *Manager) completeDiagnosticOperation(h DiagnosticOperationHandle, c *manageddiagnostic.EventCollector, terminal uint16) {
	m.mu.Lock()
	m.completeDiagnosticOperationLocked(h, c, terminal)
	m.mu.Unlock()
}

// DiagnosticSnapshotFor returns a cloned, closed snapshot only when the exact
// manager-attempt capability owns the requested operation.
func (m *Manager) DiagnosticSnapshotFor(attempt manageddiagnostic.StartupAttemptID, h DiagnosticOperationHandle) (DiagnosticSnapshot, bool) {
	if attempt == "" {
		return DiagnosticSnapshot{}, false
	}
	m.mu.Lock()
	op, ok := m.diagnosticOperations[h]
	m.mu.Unlock()
	if !ok || op.attemptID != attempt || op.attemptID != h.generation.session.attemptID || op.sessionID != h.generation.session.sessionID || op.generation != h.generation.generation || op.sequence != h.sequence || h.generation.session.capability == nil || h.generation.session.capability != op.capability {
		return DiagnosticSnapshot{}, false
	}
	s := op.collector.Snapshot()
	if !s.Closed {
		return DiagnosticSnapshot{}, false
	}
	return DiagnosticSnapshot{Operation: h, Events: s, ProcessIdentity: op.identity}, true
}
