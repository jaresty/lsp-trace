package sessionruntime

import (
	"context"
	"testing"
	"time"

	"lsp-trace/internal/manageddiagnostic"
	"lsp-trace/internal/managedprocess"
)

func newC15DiagnosticManager(t *testing.T) *Manager {
	t.Helper()
	if !managerDiagnosticRepresentationSupported() || !privateDiagnosticHistoryRepresentationSupported() {
		t.Skip("C15 diagnostic storage requires the qualified runtime representation")
	}
	owner, failure := newManagerDiagnosticOwner()
	if failure != "" {
		t.Fatal(failure)
	}
	return &Manager{limits: Limits{MaxOperations: 4, MaxObservations: 2}, now: time.Now, diagnostics: manageddiagnostic.NewStore(manageddiagnostic.Bounds{MaxRecords: 1, MaxBytes: 1024}), diagnosticOperations: make(map[DiagnosticOperationHandle]diagnosticOperation), managerDiagnosticOwner: owner, privateDiagnosticHistory: new(privateDiagnosticHistory), workerDone: make(chan struct{}, 1)}
}

func TestC15DiagnosticHistoryBytePressureEvictsOldestFIFO(t *testing.T) {
	m := newC15DiagnosticManager(t)
	bytes := uint64(2) * manageddiagnostic.EventElementBytes()
	filler, failure := m.managerDiagnosticOwner.reserve(managerDiagnosticMaxOwnedBytes - managerDiagnosticFixedBytes() - bytes)
	if failure != "" {
		t.Fatal(failure)
	}
	g := m.newDiagnosticGeneration("attempt", "session", 1)
	account1 := mustPrivateB4ByteAccountV2(t)
	h1, c1 := m.newPrivateB4DiagnosticOperation(g, managedprocess.Identity{}, account1)
	account1.requestTerminalRelease()
	m.completeDiagnosticOperation(h1, c1, diagnosticEventTerminalResponse)
	if slot := m.privateDiagnosticHistory.resolve(h1.private, h1.sequence); slot == nil || !slot.closed || account1.snapshot().Live != 0 {
		t.Fatal("ASSERT_C15_DIAGNOSTIC_FIRST_RETAINED")
	}
	account2 := mustPrivateB4ByteAccountV2(t)
	h2, c2 := m.newPrivateB4DiagnosticOperation(g, managedprocess.Identity{}, account2)
	account2.requestTerminalRelease()
	beforeReplacement := m.managerDiagnosticOwner.snapshot()
	m.completeDiagnosticOperation(h2, c2, diagnosticEventTerminalResponse)
	afterReplacement := m.managerDiagnosticOwner.snapshot()
	if afterReplacement.Live != beforeReplacement.Live || afterReplacement.Cumulative != beforeReplacement.Cumulative+bytes {
		t.Fatalf("ASSERT_C15_DIAGNOSTIC_FIFO_EXACT_RELEASE before=%+v after=%+v bytes=%d", beforeReplacement, afterReplacement, bytes)
	}
	if old := m.privateDiagnosticHistory.resolve(h1.private, h1.sequence); old != nil {
		t.Fatal("ASSERT_C15_DIAGNOSTIC_FIFO_OLDEST_RETAINED")
	}
	if current := m.privateDiagnosticHistory.resolve(h2.private, h2.sequence); current == nil || m.diagnosticByteEvictions != 1 || m.diagnosticOmissions != 0 || account2.snapshot().Live != 0 {
		t.Fatalf("ASSERT_C15_DIAGNOSTIC_FIFO current=%t evictions=%d omissions=%d", current != nil, m.diagnosticByteEvictions, m.diagnosticOmissions)
	}
	beforeManager := m.managerDiagnosticOwner.snapshot()
	m.completeDiagnosticOperation(h2, c2, diagnosticEventTerminalResponse)
	if m.managerDiagnosticOwner.snapshot() != beforeManager || m.diagnosticByteEvictions != 1 || m.diagnosticOmissions != 0 {
		t.Fatal("ASSERT_C15_DIAGNOSTIC_TERMINAL_REPLAY")
	}
	filler.release()
}

func TestC15DiagnosticHistoryOmissionAndShutdownRelease(t *testing.T) {
	m := newC15DiagnosticManager(t)
	bytes := uint64(2) * manageddiagnostic.EventElementBytes()
	filler, failure := m.managerDiagnosticOwner.reserve(managerDiagnosticMaxOwnedBytes - managerDiagnosticFixedBytes() - bytes + 1)
	if failure != "" {
		t.Fatal(failure)
	}
	g := m.newDiagnosticGeneration("attempt", "session", 1)
	account := mustPrivateB4ByteAccountV2(t)
	h, c := m.newPrivateB4DiagnosticOperation(g, managedprocess.Identity{}, account)
	account.requestTerminalRelease()
	m.completeDiagnosticOperation(h, c, diagnosticEventTerminalResponse)
	if retained := m.privateDiagnosticHistory.resolve(h.private, h.sequence); retained != nil || m.diagnosticOmissions != 1 || account.snapshot().Live != 0 {
		t.Fatalf("ASSERT_C15_DIAGNOSTIC_OMISSION retained=%t omissions=%d account=%+v", retained != nil, m.diagnosticOmissions, account.snapshot())
	}
	m.diagnosticOmissions = ^uint64(0)
	accountSaturated := mustPrivateB4ByteAccountV2(t)
	hs, cs := m.newPrivateB4DiagnosticOperation(g, managedprocess.Identity{}, accountSaturated)
	accountSaturated.requestTerminalRelease()
	m.completeDiagnosticOperation(hs, cs, diagnosticEventTerminalResponse)
	if m.diagnosticOmissions != ^uint64(0) || accountSaturated.snapshot().Live != 0 {
		t.Fatal("ASSERT_C15_DIAGNOSTIC_OMISSION_SATURATES")
	}
	filler.release()

	account2 := mustPrivateB4ByteAccountV2(t)
	h2, c2 := m.newPrivateB4DiagnosticOperation(g, managedprocess.Identity{}, account2)
	account2.requestTerminalRelease()
	m.completeDiagnosticOperation(h2, c2, diagnosticEventTerminalResponse)
	if err := m.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(m.diagnosticOperations) != 0 || len(m.diagnosticOrder) != 0 || m.managerDiagnosticOwner.snapshot().Live != 0 {
		t.Fatalf("ASSERT_C15_DIAGNOSTIC_SHUTDOWN snapshot=%+v", m.managerDiagnosticOwner.snapshot())
	}
	if err := m.Shutdown(context.Background()); err != nil || m.managerDiagnosticOwner.snapshot().Live != 0 {
		t.Fatal("ASSERT_C15_DIAGNOSTIC_SHUTDOWN_REPLAY")
	}
}
