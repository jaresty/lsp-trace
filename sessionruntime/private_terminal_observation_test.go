package sessionruntime

import (
	"context"
	"testing"

	"lsp-trace/internal/session"
)

func TestPrepareDocumentLifecycleSubcauseExactPredicatesAndPrecedence(t *testing.T) {
	m, started := readyLeaseManager(t, 4)
	req := DocumentRequest{SessionID: started.SessionID, Generation: started.Generation, URI: "file:///workspace/a.go"}

	var copied context.Context
	if err := m.WithReadySessionLease(context.Background(), SessionLeaseRequest{SessionID: started.SessionID, Generation: started.Generation, Workspace: "/workspace"}, func(ctx context.Context) error {
		copied = ctx
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	got := m.PrepareDocument(copied, req)
	if got.Failure != session.LifecycleConflict || got.LifecycleSubcause != LifecycleSubcauseLeaseTokenMismatch {
		t.Fatalf("ASSERT_PRIVATE_TERMINAL_MANAGER_LEASE_TOKEN_MISMATCH got=%+v", got)
	}

	stale := req
	stale.Generation++
	got = m.PrepareDocument(context.Background(), stale)
	if got.Failure != session.StaleGeneration || got.LifecycleSubcause != "" {
		t.Fatalf("ASSERT_PRIVATE_TERMINAL_STALE_GENERATION_NOT_RELABELED got=%+v", got)
	}

	m.mu.Lock()
	r := m.sessions[started.SessionID]
	r.leaseCount, r.leaseToken, r.leaseClosing = 0, nil, false
	m.mu.Unlock()
	if err := m.WithReadySessionLease(context.Background(), SessionLeaseRequest{SessionID: started.SessionID, Generation: started.Generation, Workspace: "/workspace"}, func(ctx context.Context) error {
		got = m.PrepareDocument(ctx, req)
		if got.Failure == session.LifecycleConflict && got.LifecycleSubcause != "" {
			t.Fatalf("ASSERT_PRIVATE_TERMINAL_VALID_LEASE_ABSENCE_NOT_CONFLICT got=%+v", got)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestPrepareDocumentLifecycleSubcausePackageOwnedMatrix(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*Manager, *runtimeSession)
		want   LifecycleSubcause
	}{
		{"manager-closed-precedes-state", func(m *Manager, r *runtimeSession) { m.closed = true; r.record.State = session.Starting }, LifecycleSubcauseManagerClosed},
		{"session-not-ready", func(_ *Manager, r *runtimeSession) { r.record.State = session.Starting }, LifecycleSubcauseSessionNotReady},
		{"protocol-owned-precedes-lease", func(_ *Manager, r *runtimeSession) { r.protocolOwned = true; r.leaseClosing = true }, LifecycleSubcauseProtocolOwned},
		{"lease-closing", func(_ *Manager, r *runtimeSession) { r.leaseClosing = true }, LifecycleSubcauseLeaseClosing},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m, started := readyLeaseManager(t, 4)
			var copied context.Context
			if err := m.WithReadySessionLease(context.Background(), SessionLeaseRequest{SessionID: started.SessionID, Generation: started.Generation, Workspace: "/workspace"}, func(ctx context.Context) error { copied = ctx; return nil }); err != nil {
				t.Fatal(err)
			}
			m.mu.Lock()
			r := m.sessions[started.SessionID]
			tc.mutate(m, r)
			m.mu.Unlock()
			got := m.PrepareDocument(copied, DocumentRequest{SessionID: started.SessionID, Generation: started.Generation, URI: "file:///workspace/a.go"})
			if got.Failure != session.LifecycleConflict || got.LifecycleSubcause != tc.want {
				t.Fatalf("ASSERT_PRIVATE_TERMINAL_MANAGER_MATRIX case=%s got=%+v want=%s", tc.name, got, tc.want)
			}
		})
	}

	m, req, _, _ := supplyFixture(t, []byte("package p\n"))
	first := m.PrepareDocument(context.Background(), req)
	if first.Failure != "" {
		t.Fatal(first)
	}
	req.LanguageID = "typescript"
	if got := m.PrepareDocument(context.Background(), req); got.Failure != session.LifecycleConflict || got.LifecycleSubcause != LifecycleSubcauseLanguageIDConflict {
		t.Fatalf("ASSERT_PRIVATE_TERMINAL_MANAGER_LANGUAGE_CONFLICT got=%+v", got)
	}
}

func TestLifecycleSubcauseProducerVocabularyClosure(t *testing.T) {
	want := []LifecycleSubcause{
		LifecycleSubcauseManagerClosed, LifecycleSubcauseSessionNotReady,
		LifecycleSubcauseProtocolOwned, LifecycleSubcauseLeaseClosing,
		LifecycleSubcauseLeaseTokenMismatch, LifecycleSubcauseLanguageIDConflict,
	}
	got := ProducedLifecycleSubcauses()
	if len(got) != len(want) {
		t.Fatalf("ASSERT_PRIVATE_TERMINAL_MANAGER_PRODUCER_CLOSURE got=%v want=%v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("ASSERT_PRIVATE_TERMINAL_MANAGER_PRODUCER_CLOSURE got=%v want=%v", got, want)
		}
	}
}
