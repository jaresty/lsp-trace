package sessionruntime

import (
	"sync"
	"testing"

	"lsp-trace/internal/managedprocess"
)

func TestPrivateDiagnosticSnapshotSetAndConcurrentRecycle(t *testing.T) {
	m := newC15DiagnosticManager(t)
	g := m.newDiagnosticGeneration("attempt", "session", 1)
	m.sessions = map[string]*runtimeSession{"session": {record: Record{Generation: 1}, attemptID: "attempt", diagnosticGeneration: g}}
	account := mustPrivateB4ByteAccountV2(t)
	h, c := m.newPrivateB4DiagnosticOperation(g, managedprocess.Identity{}, account)
	m.describeDiagnosticOperation(h, "method", "file:///set.go", SessionMetadata{})
	account.requestTerminalRelease()
	m.completeDiagnosticOperation(h, c, diagnosticEventTerminalResponse)
	set, ok := m.DiagnosticSnapshotSetFor("attempt", g, []DiagnosticOperationHandle{h}, 10)
	if !ok || len(set.Operations()) != 1 {
		t.Fatal("ASSERT_C15_PRIVATE_SNAPSHOT_SET")
	}
	before := m.managerDiagnosticOwner.snapshot()
	oldCapacity := m.privateDiagnosticHistory.resolve(h.private, h.sequence).retainedBacking.entry().capacity
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 500; i++ {
			if s, ok := m.DiagnosticSnapshotSetFor("attempt", g, []DiagnosticOperationHandle{h}, 10); ok && len(s.Operations()) != 1 {
				t.Error("ASSERT_C15_PRIVATE_SNAPSHOT_SET_CONCURRENT")
			}
		}
	}()
	m.mu.Lock()
	if !m.privateDiagnosticHistory.releaseSlot(h.private) {
		t.Fatal("ASSERT_C15_PRIVATE_SNAPSHOT_SET_RELEASE")
	}
	m.mu.Unlock()
	account2 := mustPrivateB4ByteAccountV2(t)
	h2, c2 := m.newPrivateB4DiagnosticOperation(g, managedprocess.Identity{}, account2)
	m.describeDiagnosticOperation(h2, "next", "file:///next.go", SessionMetadata{})
	account2.requestTerminalRelease()
	m.completeDiagnosticOperation(h2, c2, diagnosticEventTerminalResponse)
	wg.Wait()
	if _, ok := m.DiagnosticSnapshotSetFor("attempt", g, []DiagnosticOperationHandle{h}, 10); ok {
		t.Fatal("ASSERT_C15_PRIVATE_SNAPSHOT_SET_STALE")
	}
	if _, ok := m.DiagnosticSnapshotSetFor("attempt", g, []DiagnosticOperationHandle{h2}, 10); !ok {
		t.Fatal("ASSERT_C15_PRIVATE_SNAPSHOT_SET_REUSED")
	}
	after := m.managerDiagnosticOwner.snapshot()
	newCapacity := m.privateDiagnosticHistory.resolve(h2.private, h2.sequence).retainedBacking.entry().capacity
	if after.Live != before.Live-oldCapacity+newCapacity || after.Cumulative != before.Cumulative+newCapacity {
		t.Fatalf("ASSERT_C15_PRIVATE_SNAPSHOT_SET_ACCOUNTING before=%+v after=%+v old=%d new=%d", before, after, oldCapacity, newCapacity)
	}
}
