package sessionruntime

import (
	"lsp-trace/internal/lspwire"
	"testing"
)

// The live STOP packet covers invocation; this checks only the exact locked
// retirement filter, repeated subtraction and isolation of other live leases.
func TestADR0011PrivateStoppedGenerationRetirementIsolation(t *testing.T) {
	stopped := &runtimeSession{record: Record{Generation: 1}}
	other := &runtimeSession{record: Record{Generation: 1}}
	entry := func(token byte, r *runtimeSession, id string, generation uint64, charge int64) *privateB4Reservation {
		var key [32]byte
		key[0] = token
		return &privateB4Reservation{token: key, session: r, selection: B4DefinitionSelectionKey{SessionID: id, Key: lspwire.RequestKey{Generation: generation, ID: uint64(token)}}, maxCharge: charge}
	}
	target := entry(1, stopped, "stopped", 1, 9558)
	nextGeneration := entry(2, stopped, "stopped", 2, 31)
	otherActive := entry(3, other, "other", 1, 17)
	otherIdentity := entry(4, stopped, "other", 1, 23)
	m := &Manager{privateB4Leases: map[[32]byte]*privateB4Reservation{
		target.token: target, nextGeneration.token: nextGeneration, otherActive.token: otherActive, otherIdentity.token: otherIdentity,
	}, privateB4Bytes: 9558 + 31 + 17 + 23}
	m.mu.Lock()
	m.retirePrivateB4StoppedLocked("stopped", 1, stopped)
	if len(m.privateB4Leases) != 3 || m.privateB4Bytes != 71 || m.privateB4Leases[target.token] != nil ||
		m.privateB4Leases[nextGeneration.token] != nextGeneration || m.privateB4Leases[otherActive.token] != otherActive || m.privateB4Leases[otherIdentity.token] != otherIdentity {
		m.mu.Unlock()
		t.Fatalf("first STOP retirement crossed identity or charge: slots=%d bytes=%d", len(m.privateB4Leases), m.privateB4Bytes)
	}
	m.retirePrivateB4StoppedLocked("stopped", 1, stopped)
	m.retirePrivateB4StoppedLocked("stopped", 2, other)
	if len(m.privateB4Leases) != 3 || m.privateB4Bytes != 71 {
		m.mu.Unlock()
		t.Fatalf("repeat/wrong session changed unrelated charge: slots=%d bytes=%d", len(m.privateB4Leases), m.privateB4Bytes)
	}
	m.mu.Unlock()
}
