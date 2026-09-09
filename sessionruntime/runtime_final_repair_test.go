package sessionruntime

import (
	"context"
	"reflect"
	"testing"
	"time"

	"lsp-trace/internal/manageddiagnostic"
	"lsp-trace/internal/managedprocess"
)

func TestDiagnosticHandlesExposeNoConstructibleSemanticFields(t *testing.T) {
	for _, typ := range []reflect.Type{
		reflect.TypeOf(DiagnosticSessionHandle{}),
		reflect.TypeOf(DiagnosticGenerationHandle{}),
		reflect.TypeOf(DiagnosticOperationHandle{}),
	} {
		for i := 0; i < typ.NumField(); i++ {
			if typ.Field(i).IsExported() {
				t.Fatalf("ASSERT_RUNTIME_HANDLE_UNFORGEABLE: %s.%s is exported", typ, typ.Field(i).Name)
			}
		}
	}
}

func TestReadinessCancellationJoinsReaderBeforeDiagnosticClose(t *testing.T) {
	child := newReadinessChild("hang")
	store := manageddiagnostic.NewStore(manageddiagnostic.Bounds{MaxRecords: 16, MaxBytes: 16384})
	m, err := New(Config{Limits: Limits{MaxSessions: 1, MaxRequests: 1, MaxChildren: 2, MaxCancels: 2, MaxTombstones: 4, MaxObservations: 32, MaxOperations: 4}, Starter: oneChildStarter{child}, Diagnostics: store})
	if err != nil {
		t.Fatal(err)
	}
	started := m.Start(context.Background(), StartRequest{Profile: profile(t)})
	ctx, cancel := context.WithCancel(context.Background())
	pending := m.BeginReadiness(ctx, started.SessionID, started.Generation, time.Now().Add(time.Second))
	cancel()
	terminal, found := m.WaitReadiness(context.Background(), pending.ID)
	if !found {
		t.Fatal("ASSERT_RUNTIME_READINESS_READER_JOINED: missing terminal")
	}
	snapshot, ok := m.DiagnosticSnapshotFor(started.AttemptID, terminal.DiagnosticOperation)
	if !ok || !snapshot.Events.Closed {
		t.Fatalf("ASSERT_RUNTIME_READINESS_READER_JOINED: ok=%t snapshot=%+v", ok, snapshot)
	}
	late := false
	for _, event := range snapshot.Events.Events {
		if event.Code == diagnosticEventLate {
			late = true
		}
	}
	if !late {
		t.Fatal("ASSERT_RUNTIME_READINESS_LATE_ACCOUNTED: no late shutdown event")
	}
}

func TestDiagnosticCompletedHistoryContinuouslyBounded(t *testing.T) {
	m := &Manager{
		limits:               Limits{MaxOperations: 3, MaxObservations: 4},
		diagnostics:          manageddiagnostic.NewStore(manageddiagnostic.Bounds{MaxRecords: 1, MaxBytes: 1024}),
		diagnosticOperations: make(map[DiagnosticOperationHandle]diagnosticOperation),
	}
	g := m.newDiagnosticGeneration("attempt", "session", 1)
	type pending struct {
		h DiagnosticOperationHandle
		c *manageddiagnostic.EventCollector
	}
	pendingOps := make([]pending, 4)
	for i := range pendingOps {
		pendingOps[i].h, pendingOps[i].c = m.newDiagnosticOperation(g, managedprocess.Identity{})
	}
	for _, op := range pendingOps {
		m.completeDiagnosticOperation(op.h, op.c, diagnosticEventTerminalResponse)
	}
	for i := 0; i < 1000000; i++ {
		h, c := m.newDiagnosticOperation(g, managedprocess.Identity{})
		m.completeDiagnosticOperation(h, c, diagnosticEventTerminalResponse)
		if len(m.diagnosticOrder) > m.limits.MaxOperations || len(m.diagnosticOperations) > m.limits.MaxOperations {
			t.Fatalf("ASSERT_RUNTIME_HISTORY_CONTINUOUS_BOUND: iteration=%d order=%d map=%d", i, len(m.diagnosticOrder), len(m.diagnosticOperations))
		}
	}
	const created = 4 + 1000000
	if want := uint64(created - m.limits.MaxOperations); m.diagnosticEvictions != want {
		t.Fatalf("ASSERT_RUNTIME_HISTORY_EXACT_EVICTION_COUNT: got=%d want=%d", m.diagnosticEvictions, want)
	}
}
