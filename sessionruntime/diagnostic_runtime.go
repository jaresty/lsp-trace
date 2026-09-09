package sessionruntime

import (
	"lsp-trace/internal/manageddiagnostic"
	"lsp-trace/internal/managedprocess"
)

// Diagnostic handles are opaque manager-owned capabilities. They contain no
// command, path, environment, document, or protocol payload bytes.
type DiagnosticSessionHandle struct {
	AttemptID manageddiagnostic.StartupAttemptID
	SessionID string
}

type DiagnosticGenerationHandle struct {
	Session    DiagnosticSessionHandle
	Generation uint64
}

type DiagnosticOperationHandle struct {
	Generation DiagnosticGenerationHandle
	Sequence   uint64
}

type DiagnosticSnapshot struct {
	Operation       DiagnosticOperationHandle
	Events          manageddiagnostic.EventSnapshot
	ProcessIdentity managedprocess.Identity
}

type diagnosticOperation struct {
	collector *manageddiagnostic.EventCollector
	identity  managedprocess.Identity
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

func (m *Manager) newDiagnosticOperation(g DiagnosticGenerationHandle, identity managedprocess.Identity) (DiagnosticOperationHandle, *manageddiagnostic.EventCollector) {
	if m.diagnostics == nil {
		return DiagnosticOperationHandle{}, nil
	}
	m.mu.Lock()
	m.diagnosticSequence++
	h := DiagnosticOperationHandle{Generation: g, Sequence: m.diagnosticSequence}
	c := manageddiagnostic.NewEventCollector(m.limits.MaxObservations, m.now)
	m.diagnosticOperations[h] = diagnosticOperation{collector: c, identity: identity}
	m.diagnosticOrder = append(m.diagnosticOrder, h)
	for len(m.diagnosticOrder) > m.limits.MaxOperations {
		oldest := m.diagnosticOrder[0]
		if old := m.diagnosticOperations[oldest]; old.collector != nil && !old.collector.Snapshot().Closed {
			break
		}
		delete(m.diagnosticOperations, oldest)
		m.diagnosticOrder = m.diagnosticOrder[1:]
	}
	m.mu.Unlock()
	c.Record(diagnosticEventBegin, 0, false)
	return h, c
}

// DiagnosticSnapshotFor returns a cloned, closed snapshot only when the exact
// manager-attempt capability owns the requested operation.
func (m *Manager) DiagnosticSnapshotFor(attempt manageddiagnostic.StartupAttemptID, h DiagnosticOperationHandle) (DiagnosticSnapshot, bool) {
	if attempt == "" || h.Generation.Session.AttemptID != attempt {
		return DiagnosticSnapshot{}, false
	}
	m.mu.Lock()
	op, ok := m.diagnosticOperations[h]
	m.mu.Unlock()
	if !ok {
		return DiagnosticSnapshot{}, false
	}
	s := op.collector.Snapshot()
	if !s.Closed {
		return DiagnosticSnapshot{}, false
	}
	return DiagnosticSnapshot{Operation: h, Events: s, ProcessIdentity: op.identity}, true
}
