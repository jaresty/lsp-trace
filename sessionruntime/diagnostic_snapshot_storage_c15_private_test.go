package sessionruntime

import (
	"testing"

	"lsp-trace/internal/managedprocess"
)

func TestPrivateDiagnosticSnapshotCopiesAndStaleRejection(t *testing.T) {
	m := newC15DiagnosticManager(t)
	g := m.newDiagnosticGeneration("attempt", "session", 1)
	account := mustPrivateB4ByteAccountV2(t)
	h, c := m.newPrivateB4DiagnosticOperation(g, managedprocess.Identity{}, account)
	m.describeDiagnosticOperation(h, "method", "file:///snapshot.go", SessionMetadata{PositionEncoding: "utf-16", ProviderName: "provider"})
	account.requestTerminalRelease()
	m.completeDiagnosticOperation(h, c, diagnosticEventTerminalResponse)
	before := m.managerDiagnosticOwner.snapshot()
	first, ok := m.DiagnosticSnapshotFor("attempt", h)
	if !ok || len(first.Events.Events) == 0 {
		t.Fatal("ASSERT_C15_PRIVATE_SNAPSHOT_FIRST")
	}
	first.Events.Events[0].Code = ^uint16(0)
	first.Method = "changed"
	first.Initialize.ProviderName = "changed"
	second, ok := m.DiagnosticSnapshotFor("attempt", h)
	if !ok || second.Events.Events[0].Code == ^uint16(0) || second.Method != "method" || second.Initialize.ProviderName != "provider" || m.managerDiagnosticOwner.snapshot() != before {
		t.Fatal("ASSERT_C15_PRIVATE_SNAPSHOT_ISOLATION")
	}
	m.mu.Lock()
	if !m.privateDiagnosticHistory.releaseSlot(h.private) {
		t.Fatal("ASSERT_C15_PRIVATE_SNAPSHOT_RELEASE")
	}
	m.mu.Unlock()
	if _, ok := m.DiagnosticSnapshotFor("attempt", h); ok {
		t.Fatal("ASSERT_C15_PRIVATE_SNAPSHOT_STALE")
	}
	account2 := mustPrivateB4ByteAccountV2(t)
	h2, c2 := m.newPrivateB4DiagnosticOperation(g, managedprocess.Identity{}, account2)
	if h2.private.slot != h.private.slot || h2.private.generation == h.private.generation {
		t.Fatal("ASSERT_C15_PRIVATE_SNAPSHOT_RECYCLE")
	}
	if _, ok := m.DiagnosticSnapshotFor("attempt", h); ok {
		t.Fatal("ASSERT_C15_PRIVATE_SNAPSHOT_RECYCLED_STALE")
	}
	account2.requestTerminalRelease()
	m.completeDiagnosticOperation(h2, c2, diagnosticEventTerminalResponse)
}
