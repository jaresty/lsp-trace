package sessionruntime

import (
	"math"
	"strings"

	"lsp-trace/internal/manageddiagnostic"
	"lsp-trace/internal/managedprocess"
	"lsp-trace/internal/session"
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
	private    privateDiagnosticHistoryRef
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
	backing     managerDiagnosticLease
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
	return m.newDiagnosticOperationWithOwner(g, identity, nil)
}

func (m *Manager) newPrivateB4DiagnosticOperation(g DiagnosticGenerationHandle, identity managedprocess.Identity, account *privateB4ByteAccountV2) (DiagnosticOperationHandle, *manageddiagnostic.EventCollector) {
	if m.diagnostics == nil || account == nil {
		return DiagnosticOperationHandle{}, nil
	}
	m.mu.Lock()
	if m.privateDiagnosticHistory == nil {
		m.privateDiagnosticHistory = new(privateDiagnosticHistory)
	}
	if m.diagnosticSequence == ^uint64(0) {
		if m.diagnosticOmissions != ^uint64(0) {
			m.diagnosticOmissions++
		}
		m.mu.Unlock()
		return DiagnosticOperationHandle{}, nil
	}
	sequence := m.diagnosticSequence + 1
	ref, failure := m.privateDiagnosticHistory.reserveSlot(sequence)
	if failure != "" {
		if m.diagnosticOmissions != ^uint64(0) {
			m.diagnosticOmissions++
		}
		m.mu.Unlock()
		return DiagnosticOperationHandle{}, nil
	}
	c, err := manageddiagnostic.NewOwnedEventCollector(m.limits.MaxObservations, m.now, privateB4EventBackingOwner{account: account})
	if err != nil {
		m.privateDiagnosticHistory.releaseSlot(ref)
		if m.diagnosticOmissions != ^uint64(0) {
			m.diagnosticOmissions++
		}
		m.mu.Unlock()
		return DiagnosticOperationHandle{}, nil
	}
	m.diagnosticSequence = sequence
	s := m.privateDiagnosticHistory.resolve(ref, sequence)
	s.collector, s.identity, s.attemptID, s.sessionID, s.diagnosticGeneration, s.capability, s.account = c, identity, g.session.attemptID, g.session.sessionID, g.generation, g.session.capability, account
	h := DiagnosticOperationHandle{generation: g, sequence: sequence, private: ref}
	m.mu.Unlock()
	c.Record(diagnosticEventBegin, 0, false)
	return h, c
}

func (m *Manager) newDiagnosticOperationWithOwner(g DiagnosticGenerationHandle, identity managedprocess.Identity, owner manageddiagnostic.EventBackingOwner) (DiagnosticOperationHandle, *manageddiagnostic.EventCollector) {
	if m.diagnostics == nil {
		return DiagnosticOperationHandle{}, nil
	}
	m.mu.Lock()
	m.diagnosticSequence++
	h := DiagnosticOperationHandle{generation: g, sequence: m.diagnosticSequence}
	var c *manageddiagnostic.EventCollector
	if owner == nil {
		c = manageddiagnostic.NewEventCollector(m.limits.MaxObservations, m.now)
	} else {
		var err error
		c, err = manageddiagnostic.NewOwnedEventCollector(m.limits.MaxObservations, m.now, owner)
		if err != nil {
			if m.diagnosticOmissions != ^uint64(0) {
				m.diagnosticOmissions++
			}
			m.mu.Unlock()
			return DiagnosticOperationHandle{}, nil
		}
	}
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
		op := m.diagnosticOperations[h]
		op.backing.release()
		delete(m.diagnosticOperations, h)
		m.diagnosticEvictions++
		copy(m.diagnosticOrder[evicted:], m.diagnosticOrder[evicted+1:])
		m.diagnosticOrder = m.diagnosticOrder[:len(m.diagnosticOrder)-1]
	}
}

func (m *Manager) evictOldestRetainedDiagnosticLocked(exclude DiagnosticOperationHandle) bool {
	for i, candidate := range m.diagnosticOrder {
		if candidate == exclude {
			continue
		}
		op, ok := m.diagnosticOperations[candidate]
		if !ok || op.collector == nil || !op.collector.Snapshot().Closed || !op.backing.active {
			continue
		}
		op.backing.release()
		delete(m.diagnosticOperations, candidate)
		copy(m.diagnosticOrder[i:], m.diagnosticOrder[i+1:])
		m.diagnosticOrder = m.diagnosticOrder[:len(m.diagnosticOrder)-1]
		m.diagnosticByteEvictions++
		return true
	}
	return false
}

func (m *Manager) completeDiagnosticOperationLocked(h DiagnosticOperationHandle, c *manageddiagnostic.EventCollector, terminal uint16) {
	if c == nil {
		return
	}
	if h.private.qualified {
		slot := m.privateDiagnosticHistory.resolve(h.private, h.sequence)
		if slot == nil {
			return
		}
		c.Terminal(terminal)
		backing, ok := c.DetachEventBacking()
		if !ok {
			return
		}
		lease, typed := backing.(*privateB4EventBackingLease)
		if !typed {
			backing.ReleaseEventBacking()
			m.privateDiagnosticHistory.releaseSlot(h.private)
			return
		}
		var retained managerDiagnosticLease
		for {
			var failure session.Failure
			retained, failure = lease.charge.account.transferDiagnosticBackingPair(&lease.charge, &slot.metadataSource, m.managerDiagnosticOwner)
			if failure == "" {
				break
			}
			oldest, found := m.privateDiagnosticHistory.oldestClosed(h.private)
			if !found {
				lease.ReleaseEventBacking()
				m.privateDiagnosticHistory.releaseSlot(h.private)
				if m.diagnosticOmissions != ^uint64(0) {
					m.diagnosticOmissions++
				}
				return
			}
			m.privateDiagnosticHistory.releaseSlot(oldest)
			if m.diagnosticByteEvictions != ^uint64(0) {
				m.diagnosticByteEvictions++
			}
		}
		slot = m.privateDiagnosticHistory.resolve(h.private, h.sequence)
		if slot == nil {
			retained.release()
			return
		}
		slot.retainedBacking = retained
		slot.closed = true
		return
	}
	c.Terminal(terminal)
	if backing, ok := c.DetachEventBacking(); ok {
		lease, typed := backing.(*privateB4EventBackingLease)
		if !typed {
			backing.ReleaseEventBacking()
		} else {
			if m.managerDiagnosticOwner == nil {
				m.managerDiagnosticOwner, _ = newManagerDiagnosticOwner()
			}
			for {
				if _, preflight := m.managerDiagnosticOwner.preflightSlot(lease.bytes); preflight == "" {
					break
				}
				if !m.evictOldestRetainedDiagnosticLocked(h) {
					break
				}
			}
			retained, failure := lease.charge.account.transferDiagnosticBacking(&lease.charge, m.managerDiagnosticOwner)
			if failure != "" {
				lease.ReleaseEventBacking()
				if m.diagnosticOmissions != ^uint64(0) {
					m.diagnosticOmissions++
				}
				delete(m.diagnosticOperations, h)
				for i, candidate := range m.diagnosticOrder {
					if candidate == h {
						copy(m.diagnosticOrder[i:], m.diagnosticOrder[i+1:])
						m.diagnosticOrder = m.diagnosticOrder[:len(m.diagnosticOrder)-1]
						break
					}
				}
				return
			}
			op := m.diagnosticOperations[h]
			op.backing = retained
			m.diagnosticOperations[h] = op
		}
	}
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
	if h.private.qualified {
		op := m.privateDiagnosticHistory.resolve(h.private, h.sequence)
		if op == nil || !op.closed || op.attemptID != attempt || op.attemptID != h.generation.session.attemptID || op.sessionID != h.generation.session.sessionID || op.diagnosticGeneration != h.generation.generation || h.generation.session.capability == nil || h.generation.session.capability != op.capability {
			m.mu.Unlock()
			return DiagnosticSnapshot{}, false
		}
		s := op.collector.Snapshot()
		initialize := op.initialize
		initialize.PositionEncoding, initialize.ProviderName, initialize.ProviderVersion, initialize.ServerCommand = strings.Clone(initialize.PositionEncoding), strings.Clone(initialize.ProviderName), strings.Clone(initialize.ProviderVersion), strings.Clone(initialize.ServerCommand)
		out := DiagnosticSnapshot{Operation: h, Events: s, ProcessIdentity: op.identity, Method: strings.Clone(op.method), DocumentURI: strings.Clone(op.documentURI), Initialize: initialize}
		m.mu.Unlock()
		return out, s.Closed
	}
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
	defer m.mu.Unlock()
	if h.private.qualified {
		s := m.privateDiagnosticHistory.resolve(h.private, h.sequence)
		if s == nil || s.closed {
			return
		}
		parts := [...]string{method, documentURI, initialize.PositionEncoding, initialize.ProviderName, initialize.ProviderVersion, initialize.ServerCommand}
		var bytes uint64
		for _, part := range parts {
			if uint64(len(part)) > math.MaxUint64-bytes {
				if !s.described {
					m.omitOpenPrivateDiagnosticLocked(h, s)
				}
				return
			}
			bytes += uint64(len(part))
		}
		var source privateB4ByteLeaseV2
		if bytes != 0 {
			var failure session.Failure
			source, failure = s.account.reserve(bytes)
			if failure != "" {
				if !s.described {
					m.omitOpenPrivateDiagnosticLocked(h, s)
				}
				return
			}
		}
		owned := initialize
		owned.PositionEncoding, owned.ProviderName, owned.ProviderVersion, owned.ServerCommand = strings.Clone(initialize.PositionEncoding), strings.Clone(initialize.ProviderName), strings.Clone(initialize.ProviderVersion), strings.Clone(initialize.ServerCommand)
		old := s.metadataSource
		s.method, s.documentURI, s.initialize, s.metadataSource, s.described = strings.Clone(method), strings.Clone(documentURI), owned, source, true
		old.release()
		return
	}
	if op, ok := m.diagnosticOperations[h]; ok {
		op.method, op.documentURI, op.initialize = method, documentURI, initialize
		m.diagnosticOperations[h] = op
	}
}

func (m *Manager) omitOpenPrivateDiagnosticLocked(h DiagnosticOperationHandle, s *privateDiagnosticHistorySlot) {
	s.collector.Terminal(diagnosticEventTerminalResponse)
	if backing, ok := s.collector.DetachEventBacking(); ok {
		backing.ReleaseEventBacking()
	}
	m.privateDiagnosticHistory.releaseSlot(h.private)
	if m.diagnosticOmissions != ^uint64(0) {
		m.diagnosticOmissions++
	}
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
