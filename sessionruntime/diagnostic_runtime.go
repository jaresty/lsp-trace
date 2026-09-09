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
	Operation       DiagnosticOperationHandle       `json:"-"`
	Events          manageddiagnostic.EventSnapshot `json:"-"`
	ProcessIdentity managedprocess.Identity         `json:"-"`
	Method          string                          `json:"-"`
	DocumentURI     string                          `json:"-"`
	Initialize      SessionMetadata                 `json:"-"`
}

// DiagnosticSnapshotSet is an immutable manager-certified, source-bounded view
// of exact-generation operation snapshots. Callers cannot construct a valid set.
type DiagnosticSnapshotSet struct {
	attempt    manageddiagnostic.StartupAttemptID
	sessionID  string
	generation uint64
	operations []DiagnosticSnapshot
	omitted    uint64
	certified  *diagnosticCapability
}

func (s DiagnosticSnapshotSet) AttemptID() manageddiagnostic.StartupAttemptID { return s.attempt }
func (s DiagnosticSnapshotSet) SessionID() string                             { return s.sessionID }
func (s DiagnosticSnapshotSet) Generation() uint64                            { return s.generation }
func (s DiagnosticSnapshotSet) Omitted() uint64                               { return s.omitted }
func (s DiagnosticSnapshotSet) Operations() []DiagnosticSnapshot {
	out := append([]DiagnosticSnapshot(nil), s.operations...)
	for i := range out {
		out[i].Events.Events = append([]manageddiagnostic.Event(nil), out[i].Events.Events...)
	}
	return out
}
func (s DiagnosticSnapshotSet) Certified() bool { return s.certified != nil }
func (s DiagnosticSnapshot) Handle() uint64     { return s.Operation.sequence }

// DiagnosticEventName closes projection over manager-owned event vocabulary.
func DiagnosticEventName(code uint16) string {
	switch code {
	case diagnosticEventBegin:
		return "BEGIN"
	case diagnosticEventWriteAttempt:
		return "WRITE_ATTEMPT"
	case diagnosticEventWriteComplete:
		return "WRITE_COMPLETE"
	case diagnosticEventReadComplete:
		return "READ_COMPLETE"
	case diagnosticEventMatched:
		return "CORRELATED"
	case diagnosticEventUnmatched:
		return "UNCORRELATED"
	case diagnosticEventLate:
		return "LATE"
	case diagnosticEventInitialized:
		return "INITIALIZED"
	case diagnosticEventCacheHit:
		return "CACHE_HIT"
	case diagnosticEventTerminalResponse:
		return "TERMINAL_RESPONSE"
	case diagnosticEventTerminalDeadline:
		return "TERMINAL_DEADLINE"
	case diagnosticEventTerminalFailure:
		return "TERMINAL_FAILURE"
	case diagnosticEventResponseDecoded:
		return "RESPONSE_DECODED"
	case diagnosticEventSemanticMatched:
		return "SEMANTIC_MATCHED"
	case diagnosticEventSemanticUnmatched:
		return "SEMANTIC_UNMATCHED"
	default:
		return ""
	}
}

type diagnosticOperation struct {
	collector   *manageddiagnostic.EventCollector
	identity    managedprocess.Identity
	attemptID   manageddiagnostic.StartupAttemptID
	sessionID   string
	generation  uint64
	sequence    uint64
	capability  *diagnosticCapability
	method      string
	documentURI string
	initialize  SessionMetadata
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
	diagnosticEventResponseDecoded
	diagnosticEventSemanticMatched
	diagnosticEventSemanticUnmatched
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
	return DiagnosticSnapshot{Operation: h, Events: s, ProcessIdentity: op.identity, Method: op.method, DocumentURI: op.documentURI, Initialize: op.initialize}, true
}

func (m *Manager) describeDiagnosticOperation(h DiagnosticOperationHandle, method, documentURI string, initialize SessionMetadata) {
	if h == (DiagnosticOperationHandle{}) {
		return
	}
	m.mu.Lock()
	if op, ok := m.diagnosticOperations[h]; ok {
		op.method, op.documentURI, op.initialize = method, documentURI, initialize
		m.diagnosticOperations[h] = op
	}
	m.mu.Unlock()
}

// DiagnosticSnapshotSetFor rejects forged, stale, cross-attempt, duplicate, open,
// and over-bound sources before returning an immutable certified snapshot set.
func (m *Manager) DiagnosticSnapshotSetFor(attempt manageddiagnostic.StartupAttemptID, generation DiagnosticGenerationHandle, handles []DiagnosticOperationHandle, maxRecords int) (DiagnosticSnapshotSet, bool) {
	if attempt == "" || generation.session.capability == nil || generation.session.attemptID != attempt || maxRecords < 1 || len(handles) > maxRecords {
		return DiagnosticSnapshotSet{}, false
	}
	m.mu.Lock()
	current := m.sessions[generation.session.sessionID]
	currentGeneration := current != nil && current.record.Generation == generation.generation && current.attemptID == attempt && current.diagnosticGeneration == generation
	m.mu.Unlock()
	if !currentGeneration {
		return DiagnosticSnapshotSet{}, false
	}
	seen := make(map[DiagnosticOperationHandle]bool, len(handles))
	out := make([]DiagnosticSnapshot, 0, len(handles))
	var omitted uint64
	retained := 0
	for _, h := range handles {
		if seen[h] || h.generation != generation {
			return DiagnosticSnapshotSet{}, false
		}
		seen[h] = true
		s, ok := m.DiagnosticSnapshotFor(attempt, h)
		if !ok {
			return DiagnosticSnapshotSet{}, false
		}
		retained += 1 + len(s.Events.Events)
		if retained > maxRecords {
			return DiagnosticSnapshotSet{}, false
		}
		omitted += s.Events.Omitted + s.Events.Late
		out = append(out, s)
	}
	return DiagnosticSnapshotSet{attempt: attempt, sessionID: generation.session.sessionID, generation: generation.generation, operations: out, omitted: omitted, certified: generation.session.capability}, true
}
