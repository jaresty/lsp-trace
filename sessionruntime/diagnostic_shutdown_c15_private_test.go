package sessionruntime

import (
	"context"
	"testing"

	"lsp-trace/internal/managedprocess"
)

func TestPrivateDiagnosticShutdownRacesCompletionWithoutLeak(t *testing.T) {
	for i := 0; i < 50; i++ {
		m := newC15DiagnosticManager(t)
		g := m.newDiagnosticGeneration("attempt", "session", 1)
		account := mustPrivateB4ByteAccountV2(t)
		h, c := m.newPrivateB4DiagnosticOperation(g, managedprocess.Identity{}, account)
		m.describeDiagnosticOperation(h, "method", "file:///race.go", SessionMetadata{})
		account.requestTerminalRelease()
		start := make(chan struct{})
		done := make(chan struct{}, 2)
		go func() {
			<-start
			m.completeDiagnosticOperation(h, c, diagnosticEventTerminalResponse)
			done <- struct{}{}
		}()
		go func() {
			<-start
			if err := m.Shutdown(context.Background()); err != nil {
				t.Error(err)
			}
			done <- struct{}{}
		}()
		close(start)
		<-done
		<-done
		if account.snapshot().Live != 0 || m.managerDiagnosticOwner.snapshot().Live != 0 || m.privateDiagnosticHistory.count != 0 {
			t.Fatalf("ASSERT_C15_PRIVATE_SHUTDOWN_COMPLETION_RACE iteration=%d account=%+v manager=%+v", i, account.snapshot(), m.managerDiagnosticOwner.snapshot())
		}
	}
}

func TestPrivateDiagnosticShutdownReleasesActiveAndRetainedExactlyOnce(t *testing.T) {
	m := newC15DiagnosticManager(t)
	g := m.newDiagnosticGeneration("attempt", "session", 1)
	activeAccount := mustPrivateB4ByteAccountV2(t)
	activeHandle, _ := m.newPrivateB4DiagnosticOperation(g, managedprocess.Identity{}, activeAccount)
	m.describeDiagnosticOperation(activeHandle, "active", "file:///active.go", SessionMetadata{ProviderName: "p"})
	retainedAccount := mustPrivateB4ByteAccountV2(t)
	retainedHandle, retainedCollector := m.newPrivateB4DiagnosticOperation(g, managedprocess.Identity{}, retainedAccount)
	m.describeDiagnosticOperation(retainedHandle, "retained", "file:///retained.go", SessionMetadata{ProviderName: "p"})
	retainedAccount.requestTerminalRelease()
	m.completeDiagnosticOperation(retainedHandle, retainedCollector, diagnosticEventTerminalResponse)
	if activeAccount.snapshot().Live == 0 || retainedAccount.snapshot().Live != 0 {
		t.Fatal("ASSERT_C15_PRIVATE_SHUTDOWN_PRECONDITION")
	}
	if err := m.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
	if activeAccount.snapshot().Live != 0 || retainedAccount.snapshot().Live != 0 || m.managerDiagnosticOwner.snapshot().Live != 0 || m.privateDiagnosticHistory.count != 0 || !m.privateDiagnosticHistory.backingReleased {
		t.Fatalf("ASSERT_C15_PRIVATE_SHUTDOWN_RELEASE active=%+v retained=%+v manager=%+v count=%d", activeAccount.snapshot(), retainedAccount.snapshot(), m.managerDiagnosticOwner.snapshot(), m.privateDiagnosticHistory.count)
	}
	if _, ok := m.DiagnosticSnapshotFor("attempt", activeHandle); ok {
		t.Fatal("ASSERT_C15_PRIVATE_SHUTDOWN_ACTIVE_HANDLE")
	}
	if _, ok := m.DiagnosticSnapshotFor("attempt", retainedHandle); ok {
		t.Fatal("ASSERT_C15_PRIVATE_SHUTDOWN_RETAINED_HANDLE")
	}
	beforeActive, beforeRetained, beforeManager := activeAccount.snapshot(), retainedAccount.snapshot(), m.managerDiagnosticOwner.snapshot()
	if err := m.Shutdown(context.Background()); err != nil || activeAccount.snapshot() != beforeActive || retainedAccount.snapshot() != beforeRetained || m.managerDiagnosticOwner.snapshot() != beforeManager {
		t.Fatal("ASSERT_C15_PRIVATE_SHUTDOWN_REPLAY")
	}
}
