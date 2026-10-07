package sessionruntime

import (
	"context"
	"reflect"
	"testing"
	"time"

	"lsp-trace/internal/lspwire"
	"lsp-trace/internal/session"
)

type c08PreflightSnapshot struct {
	workers, observations, requests, tombstones int
	receiptOwners, borrowers                    uint64
	protocolOwned, lifecycleOwned               bool
	attemptID                                   string
	attemptStarted                              time.Time
	generation                                  uint64
	pendingKeyProxy                             lspwire.RequestKey
	reservationZero                             bool
}

func c08PreflightState(t *testing.T, m *Manager, id string, transaction *privateB4Transaction) c08PreflightSnapshot {
	t.Helper()
	m.mu.Lock()
	defer m.mu.Unlock()
	r := m.sessions[id]
	if r == nil || r.readinessHistory == nil || transaction == nil || transaction.legacy == nil {
		t.Fatal("ASSERT_C08_PREFLIGHT_FIXTURE")
	}
	return c08PreflightSnapshot{
		workers: m.workers, observations: len(m.observations), requests: len(r.requests), tombstones: r.pending.TombstoneCount(),
		receiptOwners: r.readinessHistory.receiptOwners, borrowers: r.readinessHistory.borrowers,
		protocolOwned: r.protocolOwned, lifecycleOwned: r.lifecycleOwned, attemptID: string(r.attemptID), attemptStarted: r.attemptStarted,
		generation: r.record.Generation, pendingKeyProxy: transaction.legacy.selection.Key,
		reservationZero: reflect.DeepEqual(*transaction.legacy, privateB4Reservation{}) && transaction.c16 == nil && transaction.c17 == nil,
	}
}

func TestC08PrivateP2AttemptStartsAtStartupPreflight(t *testing.T) {
	base := time.Unix(1700001000, 0)
	setupDelay := 10 * time.Second
	for _, tc := range []struct {
		name   string
		offset time.Duration
		want   session.Failure
	}{
		{name: "before", offset: -time.Nanosecond, want: DocumentSupplyUnavailable},
		{name: "equality", offset: 0, want: session.RequestTimeout},
		{name: "plus-one", offset: time.Nanosecond, want: session.RequestTimeout},
	} {
		t.Run(tc.name, func(t *testing.T) {
			child := newReadinessChild("ready")
			startCalls := 0
			now := func() time.Time {
				startCalls++
				switch startCalls {
				case 1:
					return base
				case 2:
					return base.Add(setupDelay)
				default:
					return base.Add(setupDelay)
				}
			}
			m, err := New(Config{Limits: Limits{MaxSessions: 1, MaxRequests: 1, MaxChildren: 2, MaxCancels: 1, MaxTombstones: 2, MaxObservations: 32, MaxOperations: 2}, Starter: oneChildStarter{child}, Now: now})
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = m.Shutdown(context.Background()) })
			started := m.Start(context.Background(), StartRequest{Profile: profile(t)})
			if started.Failure != "" {
				t.Fatalf("ASSERT_C08_PREFLIGHT_START: %+v", started)
			}
			pending := m.BeginReadiness(context.Background(), started.SessionID, started.Generation, time.Time{})
			if ready, found := m.WaitReadiness(context.Background(), pending.ID); !found || ready.State != ReadinessReady || ready.Failure != "" {
				t.Fatalf("ASSERT_C08_PREFLIGHT_READINESS: %+v found=%v", ready, found)
			}

			requestCalls := 0
			m.now = func() time.Time {
				requestCalls++
				switch requestCalls {
				case 1:
					return base.Add(59 * time.Second)
				case 2:
					return base.Add(privateP2AttemptLimit + tc.offset)
				default:
					t.Fatalf("ASSERT_C08_PREFLIGHT_CLOCK_CALLS calls=%d", requestCalls)
					return time.Time{}
				}
			}
			wrong := OwnedDocumentBinding{URI: "file:///c08-not-open.go", Version: 1, SHA256: "sha256:0000000000000000000000000000000000000000000000000000000000000000"}
			req := RoundTripRequest{SessionID: started.SessionID, Generation: started.Generation, Method: "textDocument/definition", Params: []byte(`{}`), ADR0011PrivateP2: true, ExpectedOwnedDocument: &wrong, MaxMessages: 1, MaxBytes: 1}
			transaction := &privateB4Transaction{legacy: &privateB4Reservation{}}
			before := c08PreflightState(t, m, started.SessionID, transaction)
			if !before.reservationZero {
				t.Fatal("ASSERT_C08_PREFLIGHT_TRANSACTION_BASELINE")
			}
			got := m.roundTripWithPrivateTransaction(context.Background(), req, nil, transaction)
			after := c08PreflightState(t, m, started.SessionID, transaction)
			if got.Failure != tc.want {
				t.Fatalf("ASSERT_C08_PREFLIGHT_BOUNDARY case=%s got=%s want=%s installed_epoch=%v", tc.name, got.Failure, tc.want, before.attemptStarted)
			}
			if requestCalls != 2 {
				t.Fatalf("ASSERT_C08_PREFLIGHT_CLOCK_CALLS calls=%d", requestCalls)
			}
			if !reflect.DeepEqual(before, after) {
				t.Fatalf("ASSERT_C08_PREFLIGHT_ZERO_MUTATION case=%s before=%+v after=%+v", tc.name, before, after)
			}
		})
	}
}
