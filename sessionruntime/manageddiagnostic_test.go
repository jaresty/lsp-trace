package sessionruntime

import (
	"context"
	"strings"
	"testing"
	"time"

	"lsp-trace/internal/manageddiagnostic"
	"lsp-trace/internal/session"
)

func diagnosticManager(t *testing.T, mode string) (*Manager, StartResult, *manageddiagnostic.Store) {
	t.Helper()
	store := manageddiagnostic.NewStore(manageddiagnostic.Bounds{MaxRecords: 16, MaxBytes: 64 << 10})
	m, err := New(Config{Limits: Limits{MaxSessions: 1, MaxRequests: 1, MaxChildren: 2, MaxCancels: 2, MaxTombstones: 4, MaxObservations: 64, MaxOperations: 2}, Starter: oneChildStarter{newRoundTripChild(mode)}, Diagnostics: store})
	if err != nil {
		t.Fatal(err)
	}
	s := m.Start(context.Background(), StartRequest{Profile: profile(t)})
	if got := m.ObserveInitialization(s.SessionID, s.Generation, true); got.State != session.Ready {
		t.Fatal(got)
	}
	return m, s, store
}

func TestManagedDiagnosticRoundTripActualFrameAndSafeJoin(t *testing.T) {
	m, s, _ := diagnosticManager(t, "success")
	got := m.RoundTrip(context.Background(), RoundTripRequest{SessionID: s.SessionID, Generation: s.Generation, Method: "textDocument/prepareCallHierarchy", Params: []byte(`{"uri":"SECRET_PATH"}`), Deadline: time.Now().Add(time.Second), MaxMessages: 3, MaxBytes: 4096, DiagnosticSequence: 1, DiagnosticTargetID: "target-opaque"})
	if got.Failure != "" {
		t.Fatal(got.Failure)
	}
	q := m.Diagnostics(s.SessionID, s.Generation)
	if q.Status != manageddiagnostic.QueryAvailable || len(q.Records) != 1 {
		t.Fatalf("ASSERT_RUNTIME_DIAGNOSTIC_EXACT_GENERATION: %+v", q)
	}
	r := q.Records[0]
	if r.Request.ProtocolID.Value != got.Key.ID || r.Request.Method.Value != "textDocument/prepareCallHierarchy" || r.Request.TargetID.Value != "target-opaque" || r.Write.Bytes != got.RequestBytes {
		t.Fatalf("ASSERT_RUNTIME_DIAGNOSTIC_ACTUAL_FRAME: %+v result=%+v", r, got)
	}
	if strings.Contains(strings.ToLower(r.Reason.Value), "secret") {
		t.Fatal("ASSERT_RUNTIME_DIAGNOSTIC_NO_PARAMS")
	}
	if other := m.Diagnostics(s.SessionID, s.Generation+1); other.Status != manageddiagnostic.QueryUnavailable {
		t.Fatalf("ASSERT_RUNTIME_DIAGNOSTIC_NO_REPLACEMENT: %+v", other)
	}
}

func TestManagedDiagnosticEOFMakesTransportClosedNotExit(t *testing.T) {
	m, s, _ := diagnosticManager(t, "eof")
	got := m.RoundTrip(context.Background(), RoundTripRequest{SessionID: s.SessionID, Generation: s.Generation, Method: "x", Deadline: time.Now().Add(time.Second), MaxMessages: 1, MaxBytes: 1024, DiagnosticSequence: 1})
	if got.Failure != session.SessionCrashed {
		t.Fatal(got.Failure)
	}
	r := m.Diagnostics(s.SessionID, s.Generation).Records[0]
	if r.Terminal != manageddiagnostic.TerminalTransportClosed || r.ProcessExit.Status != manageddiagnostic.Unavailable {
		t.Fatalf("ASSERT_RUNTIME_EOF_NOT_PROCESS_EXIT: %+v", r)
	}
}
