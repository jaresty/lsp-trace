package sessionruntime

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"lsp-trace/internal/manageddiagnostic"
	"lsp-trace/internal/managedprocess"
	"lsp-trace/internal/session"
)

func TestRoundTripDiagnosticCallbackRunsWithoutManagerLock(t *testing.T) {
	m, s, _ := roundTripManager(t, "success")
	callbackDone := make(chan struct{})
	resultDone := make(chan RoundTripResult, 1)
	go func() {
		resultDone <- m.RoundTrip(context.Background(), RoundTripRequest{
			SessionID: s.SessionID, Generation: s.Generation, Method: "test/method",
			Params: json.RawMessage(`{"x":1}`), Deadline: time.Now().Add(time.Second),
			MaxMessages: 3, MaxBytes: 4096,
			DiagnosticObserver: func(manageddiagnostic.Record) {
				_ = m.Records()
				close(callbackDone)
			},
		})
	}()
	select {
	case <-callbackDone:
	case <-time.After(250 * time.Millisecond):
		t.Fatal("ASSERT_RUNTIME_CALLBACK_OUTSIDE_MANAGER_LOCK: callback reentry blocked")
	}
	select {
	case got := <-resultDone:
		if got.Failure != "" {
			t.Fatalf("ASSERT_RUNTIME_CALLBACK_OUTSIDE_MANAGER_LOCK: %+v", got)
		}
	case <-time.After(250 * time.Millisecond):
		t.Fatal("ASSERT_RUNTIME_CALLBACK_OUTSIDE_MANAGER_LOCK: round trip blocked")
	}
}

func TestDiagnosticOperationSnapshotRequiresExactAttemptAndIsImmutable(t *testing.T) {
	executable := t.TempDir() + "/server"
	if err := os.WriteFile(executable, []byte("fixture executable"), 0700); err != nil {
		t.Fatal(err)
	}
	child := newRoundTripChild("success")
	store := manageddiagnostic.NewStore(manageddiagnostic.Bounds{MaxRecords: 64, MaxBytes: 65536})
	m, err := New(Config{Limits: Limits{MaxSessions: 1, MaxRequests: 1, MaxChildren: 2, MaxCancels: 2, MaxTombstones: 4, MaxObservations: 64}, Starter: oneChildStarter{child}, Diagnostics: store})
	if err != nil {
		t.Fatal(err)
	}
	started := m.Start(context.Background(), StartRequest{Profile: profile(t), Process: managedprocess.Spec{Path: executable, Args: []string{"--stdio"}, Env: []string{"TOKEN=secret"}}})
	if started.Failure != "" || started.DiagnosticGeneration.Session.AttemptID != started.AttemptID {
		t.Fatalf("ASSERT_RUNTIME_ATTEMPT_GENERATION_HANDLE: %+v", started)
	}
	if ready := m.ObserveInitialization(started.SessionID, started.Generation, true); ready.State != session.Ready {
		t.Fatal(ready)
	}
	result := m.RoundTrip(context.Background(), RoundTripRequest{SessionID: started.SessionID, Generation: started.Generation, Method: "test/method", Params: json.RawMessage(`{"x":1}`), Deadline: time.Now().Add(time.Second), MaxMessages: 3, MaxBytes: 4096})
	if result.Failure != "" || result.DiagnosticOperation.Sequence == 0 {
		t.Fatalf("ASSERT_RUNTIME_OPERATION_HANDLE: %+v", result)
	}
	if _, ok := m.DiagnosticSnapshotFor(manageddiagnostic.StartupAttemptID("foreign"), result.DiagnosticOperation); ok {
		t.Fatal("ASSERT_RUNTIME_SNAPSHOT_ATTEMPT_AUTHORITY: foreign attempt authorized")
	}
	snapshot, ok := m.DiagnosticSnapshotFor(started.AttemptID, result.DiagnosticOperation)
	if !ok || !snapshot.Events.Closed || len(snapshot.Events.Events) == 0 || snapshot.ProcessIdentity.ExecutableStatus != managedprocess.IdentityObserved {
		t.Fatalf("ASSERT_RUNTIME_CLOSED_IDENTITY_SNAPSHOT: ok=%t snapshot=%+v", ok, snapshot)
	}
	snapshot.Events.Events[0].Code = 0xffff
	again, ok := m.DiagnosticSnapshotFor(started.AttemptID, result.DiagnosticOperation)
	if !ok || again.Events.Events[0].Code == 0xffff {
		t.Fatal("ASSERT_RUNTIME_IMMUTABLE_SNAPSHOT")
	}
}

func TestReadinessReturnsClosedAttemptAuthorizedOperation(t *testing.T) {
	child := newReadinessChild("static-call-hierarchy")
	store := manageddiagnostic.NewStore(manageddiagnostic.Bounds{MaxRecords: 64, MaxBytes: 65536})
	m, err := New(Config{Limits: Limits{MaxSessions: 1, MaxRequests: 1, MaxChildren: 2, MaxCancels: 2, MaxTombstones: 4, MaxObservations: 64}, Starter: oneChildStarter{child}, Diagnostics: store})
	if err != nil {
		t.Fatal(err)
	}
	started := m.Start(context.Background(), StartRequest{Profile: profile(t)})
	pending := m.BeginReadiness(context.Background(), started.SessionID, started.Generation, time.Now().Add(time.Second))
	ready, found := m.WaitReadiness(context.Background(), pending.ID)
	if !found || ready.State != ReadinessReady || !ready.Metadata.CallHierarchySupport {
		t.Fatalf("ASSERT_RUNTIME_READINESS_CAPABILITY_PROJECTION: %+v found=%t", ready, found)
	}
	snapshot, ok := m.DiagnosticSnapshotFor(started.AttemptID, ready.DiagnosticOperation)
	if !ok || !snapshot.Events.Closed || snapshot.Operation.Generation != started.DiagnosticGeneration {
		t.Fatalf("ASSERT_RUNTIME_READINESS_CLOSED_SNAPSHOT: ok=%t snapshot=%+v", ok, snapshot)
	}
}

func TestDiagnosticHandlesAreExcludedFromJSONBytes(t *testing.T) {
	h := DiagnosticOperationHandle{Sequence: 7}
	for name, value := range map[string]any{
		"start":     StartResult{DiagnosticGeneration: DiagnosticGenerationHandle{Generation: 7}},
		"readiness": ReadinessSnapshot{DiagnosticOperation: h},
		"document":  DocumentResult{DiagnosticOperation: h},
		"roundtrip": RoundTripResult{DiagnosticOperation: h},
	} {
		raw, err := json.Marshal(value)
		if err != nil || string(raw) == "" || bytes.Contains(raw, []byte("Diagnostic")) {
			t.Fatalf("ASSERT_RUNTIME_PUBLIC_BYTES_EXCLUDE_HANDLES: %s %s %v", name, raw, err)
		}
	}
}

func TestNilDiagnosticsPreservesZeroHandles(t *testing.T) {
	m, s, _ := roundTripManager(t, "success")
	got := m.RoundTrip(context.Background(), RoundTripRequest{SessionID: s.SessionID, Generation: s.Generation, Method: "test/method", Params: json.RawMessage(`{"x":1}`), Deadline: time.Now().Add(time.Second), MaxMessages: 3, MaxBytes: 4096})
	if got.Failure != "" || got.DiagnosticOperation != (DiagnosticOperationHandle{}) {
		t.Fatalf("ASSERT_RUNTIME_NIL_DIAGNOSTICS_IDENTICAL: %+v", got)
	}
}
