package sessionruntime

import (
	"os"
	"strings"
	"testing"
	"unsafe"

	"lsp-trace/internal/manageddiagnostic"
	"lsp-trace/internal/session"
)

func TestManagerDiagnosticRepresentationFollowsManagedDiagnostic(t *testing.T) {
	if managerDiagnosticRepresentationSupported() && !manageddiagnostic.EventRepresentationSupported() {
		t.Fatal("ASSERT_C15_MANAGER_DIAGNOSTIC_REPRESENTATION_GATE_AGREEMENT")
	}
	for _, sourceFile := range []string{"diagnostic_backing_c15_private.go", "diagnostic_history_storage_c15_private.go"} {
		source, err := os.ReadFile(sourceFile)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(source), "manageddiagnostic.EventRepresentationSupported()") {
			t.Fatalf("ASSERT_C15_DIAGNOSTIC_AUTHORITATIVE_PLATFORM_PREDICATE file=%s", sourceFile)
		}
	}
}

func TestManagerDiagnosticOwnerLayoutEqualityAndPlusOne(t *testing.T) {
	if unsafe.Sizeof(managerDiagnosticEntry{}) != 24 || unsafe.Alignof(managerDiagnosticEntry{}) != 8 ||
		unsafe.Sizeof(managerDiagnosticLease{}) != 24 || unsafe.Alignof(managerDiagnosticLease{}) != 8 ||
		unsafe.Sizeof(managerDiagnosticOwner{}) != 24608 || unsafe.Alignof(managerDiagnosticOwner{}) != 8 ||
		unsafe.Offsetof(managerDiagnosticOwner{}.entries) != 24 || unsafe.Offsetof(managerDiagnosticOwner{}.backingReleased) != 24600 ||
		managerDiagnosticOwnerBytes != 24608 || managerDiagnosticOwnerSelfBytes != 32 ||
		managerDiagnosticSlots != 1024 || managerDiagnosticTableBytes != 24576 {
		t.Fatal("ASSERT_C15_MANAGER_DIAGNOSTIC_LAYOUT")
	}
	requireC15DiagnosticRepresentation(t)
	owner, failure := newManagerDiagnosticOwner()
	if failure != "" {
		t.Fatal(failure)
	}
	if got := owner.snapshot(); got != (privateB4ByteLedgerSnapshot{Live: managerDiagnosticFixedBytes(), Cumulative: managerDiagnosticFixedBytes()}) {
		t.Fatalf("ASSERT_C15_MANAGER_DIAGNOSTIC_TABLE snapshot=%+v", got)
	}
	payload := managerDiagnosticMaxOwnedBytes - managerDiagnosticFixedBytes()
	lease, failure := owner.reserve(payload)
	if failure != "" || !lease.active || owner.snapshot() != (privateB4ByteLedgerSnapshot{Live: managerDiagnosticMaxOwnedBytes, Cumulative: managerDiagnosticMaxOwnedBytes}) {
		t.Fatalf("ASSERT_C15_MANAGER_DIAGNOSTIC_EQUALITY failure=%q snapshot=%+v", failure, owner.snapshot())
	}
	before := owner.snapshot()
	if _, failure = owner.reserve(1); failure != session.ResourceExhausted || owner.snapshot() != before {
		t.Fatalf("ASSERT_C15_MANAGER_DIAGNOSTIC_PLUS_ONE failure=%q before=%+v after=%+v", failure, before, owner.snapshot())
	}
	lease.release()
	if got := owner.snapshot(); got.Live != managerDiagnosticFixedBytes() || got.Cumulative != managerDiagnosticMaxOwnedBytes {
		t.Fatalf("ASSERT_C15_MANAGER_DIAGNOSTIC_RELEASE snapshot=%+v", got)
	}
	if !owner.releaseTableBacking() || owner.snapshot().Live != 0 || owner.releaseTableBacking() {
		t.Fatal("ASSERT_C15_MANAGER_DIAGNOSTIC_TABLE_RELEASE_ONCE")
	}
}
