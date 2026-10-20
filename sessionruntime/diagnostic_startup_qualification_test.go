package sessionruntime

import (
	"context"
	"os"
	"strings"
	"testing"

	"lsp-trace/internal/manageddiagnostic"
	"lsp-trace/internal/managedprocess"
)

func diagnosticStartupTestConfig(diagnostics *manageddiagnostic.Store) Config {
	return Config{
		Limits: Limits{
			MaxSessions: 1, MaxRequests: 1, MaxChildren: 1, MaxCancels: 1,
			MaxTombstones: 1, MaxObservations: 8, MaxOperations: 2,
		},
		Starter:     oneChildStarter{referenceChild{}},
		Diagnostics: diagnostics,
	}
}

func TestManagerStartupWithoutDiagnosticsLeavesPrivateDiagnosticsUnallocated(t *testing.T) {
	m, err := New(diagnosticStartupTestConfig(nil))
	if err != nil {
		t.Fatalf("ASSERT_DIAGNOSTIC_DEFAULT_OFF_STARTUP: %v", err)
	}
	if m.managerDiagnosticOwner != nil || m.privateDiagnosticHistory != nil {
		t.Fatalf("ASSERT_DIAGNOSTIC_DEFAULT_OFF_NO_PRIVATE_TABLES owner=%t history=%t", m.managerDiagnosticOwner != nil, m.privateDiagnosticHistory != nil)
	}
	if err := m.Shutdown(context.Background()); err != nil {
		t.Fatalf("ASSERT_DIAGNOSTIC_DEFAULT_OFF_SHUTDOWN_FIRST: %v", err)
	}
	if err := m.Shutdown(context.Background()); err != nil {
		t.Fatalf("ASSERT_DIAGNOSTIC_DEFAULT_OFF_SHUTDOWN_IDEMPOTENT: %v", err)
	}
}

func TestManagerUnsupportedDiagnosticRepresentationKeepsPortableStoreOnly(t *testing.T) {
	if managerDiagnosticRepresentationSupported() && privateDiagnosticHistoryRepresentationSupported() {
		t.Skip("requires the real unsupported diagnostic representation")
	}
	store := manageddiagnostic.NewStore(manageddiagnostic.Bounds{MaxRecords: 8, MaxBytes: 16 << 10})
	m, err := New(diagnosticStartupTestConfig(store))
	if err != nil {
		t.Fatalf("ASSERT_DIAGNOSTIC_UNSUPPORTED_STARTUP: %v", err)
	}
	if m.diagnostics != store || m.managerDiagnosticOwner != nil || m.privateDiagnosticHistory != nil {
		t.Fatalf("ASSERT_DIAGNOSTIC_UNSUPPORTED_PORTABLE_ONLY store=%t owner=%t history=%t", m.diagnostics == store, m.managerDiagnosticOwner != nil, m.privateDiagnosticHistory != nil)
	}
	if !m.RecordCapabilityCheck(manageddiagnostic.CapabilityObservation{SessionID: "portable", Generation: 1, Sequence: 1, Supported: true}) {
		t.Fatal("ASSERT_DIAGNOSTIC_UNSUPPORTED_PORTABLE_RECORD")
	}
	if got := m.Diagnostics("portable", 1); got.Status != manageddiagnostic.QueryAvailable || len(got.Records) != 1 {
		t.Fatalf("ASSERT_DIAGNOSTIC_UNSUPPORTED_PORTABLE_QUERY: %+v", got)
	}
	if err := m.Shutdown(context.Background()); err != nil {
		t.Fatalf("ASSERT_DIAGNOSTIC_UNSUPPORTED_SHUTDOWN: %v", err)
	}
}

func TestDiagnosticOperationRequiresQualifiedPrivateBacking(t *testing.T) {
	if managerDiagnosticRepresentationSupported() && privateDiagnosticHistoryRepresentationSupported() {
		t.Skip("requires the real unsupported diagnostic representation")
	}
	store := manageddiagnostic.NewStore(manageddiagnostic.Bounds{MaxRecords: 8, MaxBytes: 16 << 10})
	m, err := New(diagnosticStartupTestConfig(store))
	if err != nil {
		t.Fatalf("ASSERT_DIAGNOSTIC_OPERATION_UNSUPPORTED_STARTUP: %v", err)
	}
	generation := m.newDiagnosticGeneration("attempt", "session", 1)
	if handle, collector := m.newDiagnosticOperationWithOwner(generation, managedprocess.Identity{}, nil); handle != (DiagnosticOperationHandle{}) || collector != nil {
		t.Fatalf("ASSERT_DIAGNOSTIC_OPERATION_NO_FALLBACK handle=%+v collector=%t", handle, collector != nil)
	}
	if handle, collector := m.newPrivateB4DiagnosticOperation(generation, managedprocess.Identity{}, &privateB4ByteAccountV2{}); handle != (DiagnosticOperationHandle{}) || collector != nil {
		t.Fatalf("ASSERT_DIAGNOSTIC_PRIVATE_OPERATION_NO_FALLBACK handle=%+v collector=%t", handle, collector != nil)
	}
	if m.diagnosticSequence != 0 || len(m.diagnosticOperations) != 0 || len(m.diagnosticOrder) != 0 {
		t.Fatalf("ASSERT_DIAGNOSTIC_OPERATION_NO_ALLOCATION sequence=%d operations=%d order=%d", m.diagnosticSequence, len(m.diagnosticOperations), len(m.diagnosticOrder))
	}
	if _, ok := m.DiagnosticSnapshotFor("attempt", DiagnosticOperationHandle{}); ok {
		t.Fatal("ASSERT_DIAGNOSTIC_OPERATION_NO_SNAPSHOT")
	}
}

func TestDiagnosticStartupQualificationGuardIsSourceVisible(t *testing.T) {
	checks := map[string][]string{
		"sessionruntime.go":     {"c.Diagnostics != nil", "managerDiagnosticRepresentationSupported()", "privateDiagnosticHistoryRepresentationSupported()"},
		"diagnostic_runtime.go": {"m.managerDiagnosticOwner == nil", "m.privateDiagnosticHistory == nil"},
	}
	for path, fragments := range checks {
		source, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		for _, fragment := range fragments {
			if !strings.Contains(string(source), fragment) {
				t.Fatalf("ASSERT_DIAGNOSTIC_STARTUP_SOURCE_GUARD file=%s missing=%q", path, fragment)
			}
		}
	}
}
