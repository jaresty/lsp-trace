package sessionruntime

import (
	"testing"

	"lsp-trace/internal/session"
)

func TestPrivateB4DiagnosticTransferRejectsAliasAndCompletesTerminalJoin(t *testing.T) {
	account := mustPrivateB4ByteAccountV2(t)
	source, failure := account.reserve(40)
	if failure != "" {
		t.Fatal(failure)
	}
	alias := source.alias()
	manager, _ := newManagerDiagnosticOwner()
	beforeV2, beforeManager := account.snapshot(), manager.snapshot()
	if got, failure := account.transferDiagnosticBacking(&source, manager); failure != session.ResourceExhausted || got.active || account.snapshot() != beforeV2 || manager.snapshot() != beforeManager {
		t.Fatalf("ASSERT_C15_DIAGNOSTIC_TRANSFER_ALIAS failure=%q v2=%+v manager=%+v", failure, account.snapshot(), manager.snapshot())
	}
	alias.release()
	account.requestTerminalRelease()
	retained, failure := account.transferDiagnosticBacking(&source, manager)
	if failure != "" || !retained.active || account.snapshot().Live != 0 || !account.backingReleased {
		t.Fatalf("ASSERT_C15_DIAGNOSTIC_TRANSFER_TERMINAL_JOIN failure=%q snapshot=%+v", failure, account.snapshot())
	}
	retained.release()
}

func TestPrivateB4DiagnosticTransferAtomicEqualityAndPlusOne(t *testing.T) {
	account := mustPrivateB4ByteAccountV2(t)
	source, failure := account.reserve(40)
	if failure != "" {
		t.Fatal(failure)
	}
	manager, failure := newManagerDiagnosticOwner()
	if failure != "" {
		t.Fatal(failure)
	}
	filler, failure := manager.reserve(managerDiagnosticMaxOwnedBytes - managerDiagnosticFixedBytes() - 40)
	if failure != "" {
		t.Fatal(failure)
	}
	beforeV2, beforeManager := account.snapshot(), manager.snapshot()
	retained, failure := account.transferDiagnosticBacking(&source, manager)
	if failure != "" || !retained.active || source.lease.active {
		t.Fatalf("ASSERT_C15_DIAGNOSTIC_TRANSFER_EQUALITY failure=%q retained=%+v", failure, retained)
	}
	afterV2, afterManager := account.snapshot(), manager.snapshot()
	if beforeV2.Live+beforeManager.Live != afterV2.Live+afterManager.Live || afterV2.Live != beforeV2.Live-40 || afterManager.Live != beforeManager.Live+40 || afterManager.Cumulative != beforeManager.Cumulative+40 {
		t.Fatalf("ASSERT_C15_DIAGNOSTIC_TRANSFER_CONSERVATION v2=%+v/%+v manager=%+v/%+v", beforeV2, afterV2, beforeManager, afterManager)
	}
	retained.release()
	filler.release()

	account2 := mustPrivateB4ByteAccountV2(t)
	source2, _ := account2.reserve(40)
	manager2, _ := newManagerDiagnosticOwner()
	fill2, _ := manager2.reserve(managerDiagnosticMaxOwnedBytes - managerDiagnosticFixedBytes() - 39)
	beforeV2, beforeManager = account2.snapshot(), manager2.snapshot()
	if got, failure := account2.transferDiagnosticBacking(&source2, manager2); failure != session.ResourceExhausted || got.active || account2.snapshot() != beforeV2 || manager2.snapshot() != beforeManager || !source2.lease.active {
		t.Fatalf("ASSERT_C15_DIAGNOSTIC_TRANSFER_PLUS_ONE failure=%q v2=%+v manager=%+v", failure, account2.snapshot(), manager2.snapshot())
	}
	fill2.release()
	source2.release()
}
