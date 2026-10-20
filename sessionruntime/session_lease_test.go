package sessionruntime

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"lsp-trace/internal/managedprocess"
	"lsp-trace/internal/session"
)

func readyLeaseManager(t *testing.T, maxChildren int) (*Manager, StartResult) {
	t.Helper()
	m, err := New(Config{Limits: Limits{MaxSessions: 2, MaxRequests: 2, MaxChildren: maxChildren, MaxCancels: 2, MaxTombstones: 2, MaxObservations: 32, MaxOperations: 4}, Starter: &referenceStarter{}})
	if err != nil {
		t.Fatal(err)
	}
	started := m.Start(context.Background(), StartRequest{Profile: profile(t), Process: managedprocess.Spec{Path: "reference"}})
	if got := m.ObserveInitialization(started.SessionID, started.Generation, true); got.Failure != "" || got.State != session.Ready {
		t.Fatalf("ready session: %+v", got)
	}
	return m, started
}

func TestSessionLeaseAdmissionAndExclusion(t *testing.T) {
	m, started := readyLeaseManager(t, 4)
	req := SessionLeaseRequest{SessionID: started.SessionID, Generation: started.Generation, Workspace: "/workspace"}
	if got := m.WithReadySessionLease(context.Background(), SessionLeaseRequest{SessionID: req.SessionID, Generation: 2, Workspace: req.Workspace}, func(context.Context) error { return nil }); !errors.Is(got, errSessionLeaseConflict) {
		t.Fatalf("wrong generation: %v", got)
	}
	if got := m.WithReadySessionLease(context.Background(), SessionLeaseRequest{SessionID: req.SessionID, Generation: req.Generation, Workspace: "/other"}, func(context.Context) error { return nil }); !errors.Is(got, errSessionLeaseConflict) {
		t.Fatalf("wrong workspace: %v", got)
	}
	if err := m.WithReadySessionLease(context.Background(), req, func(ctx context.Context) error {
		if c := m.Census(); c.Workers != 1 {
			t.Fatalf("lease worker count: %+v", c)
		}
		if got := m.WithReadySessionLease(ctx, req, func(context.Context) error { return nil }); !errors.Is(got, errSessionLeaseConflict) {
			t.Fatalf("nested lease: %v", got)
		}
		if got := m.PrepareDocument(context.Background(), DocumentRequest{SessionID: req.SessionID, Generation: req.Generation, URI: "file:///workspace/a.go"}); got.Failure != session.LifecycleConflict {
			t.Fatalf("unleased prepare: %+v", got)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if c := m.Census(); c.Workers != 0 {
		t.Fatalf("released worker count: %+v", c)
	}
}

func TestSessionLeaseReleaseAndStaleContext(t *testing.T) {
	m, started := readyLeaseManager(t, 4)
	req := SessionLeaseRequest{SessionID: started.SessionID, Generation: started.Generation, Workspace: "/workspace"}
	var copied context.Context
	if err := m.WithReadySessionLease(context.Background(), req, func(ctx context.Context) error { copied = ctx; return nil }); err != nil {
		t.Fatal(err)
	}
	got := m.PrepareDocument(copied, DocumentRequest{SessionID: req.SessionID, Generation: req.Generation, URI: "file:///workspace/a.go"})
	if got.Failure != session.LifecycleConflict {
		t.Fatalf("stale copied context: %+v", got)
	}
}

func TestSessionLeaseRejectsNilClosedAndNonReady(t *testing.T) {
	m, started := readyLeaseManager(t, 4)
	req := SessionLeaseRequest{SessionID: started.SessionID, Generation: started.Generation, Workspace: "/workspace"}
	if err := m.WithReadySessionLease(nil, req, func(context.Context) error { return nil }); !errors.Is(err, errSessionLeaseConflict) {
		t.Fatalf("nil context: %v", err)
	}
	if err := m.WithReadySessionLease(context.Background(), req, nil); !errors.Is(err, errSessionLeaseConflict) {
		t.Fatalf("nil callback: %v", err)
	}
	m2, err := New(Config{Limits: Limits{MaxSessions: 1, MaxRequests: 1, MaxChildren: 2, MaxCancels: 1, MaxTombstones: 1, MaxObservations: 8}, Starter: &referenceStarter{}})
	if err != nil {
		t.Fatal(err)
	}
	notReady := m2.Start(context.Background(), StartRequest{Profile: profile(t), Process: managedprocess.Spec{Path: "reference"}})
	if err := m2.WithReadySessionLease(context.Background(), SessionLeaseRequest{SessionID: notReady.SessionID, Generation: 1, Workspace: "/workspace"}, func(context.Context) error { return nil }); !errors.Is(err, errSessionLeaseConflict) {
		t.Fatalf("non-ready: %v", err)
	}
	if err := m.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := m.WithReadySessionLease(context.Background(), req, func(context.Context) error { return nil }); !errors.Is(err, errSessionLeaseConflict) {
		t.Fatalf("closed manager: %v", err)
	}
}

func TestSessionLeaseReleaseOnPanicAndGoexit(t *testing.T) {
	m, started := readyLeaseManager(t, 4)
	req := SessionLeaseRequest{SessionID: started.SessionID, Generation: started.Generation, Workspace: "/workspace"}
	func() {
		defer func() {
			if recover() != "lease panic" {
				t.Fatal("panic was recovered or changed by lease")
			}
		}()
		_ = m.WithReadySessionLease(context.Background(), req, func(context.Context) error { panic("lease panic") })
	}()
	if c := m.Census(); c.Workers != 0 {
		t.Fatalf("panic release: %+v", c)
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = m.WithReadySessionLease(context.Background(), req, func(context.Context) error {
			runtime.Goexit()
			return nil
		})
	}()
	<-done
	if c := m.Census(); c.Workers != 0 {
		t.Fatalf("Goexit release: %+v", c)
	}
}

func TestSessionLeaseConcurrentConflict(t *testing.T) {
	m, started := readyLeaseManager(t, 4)
	req := SessionLeaseRequest{SessionID: started.SessionID, Generation: started.Generation, Workspace: "/workspace"}
	entered := make(chan struct{})
	release := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		done <- m.WithReadySessionLease(context.Background(), req, func(context.Context) error {
			close(entered)
			<-release
			return nil
		})
	}()
	<-entered
	if err := m.WithReadySessionLease(context.Background(), req, func(context.Context) error { return nil }); !errors.Is(err, errSessionLeaseConflict) {
		t.Fatalf("concurrent lease: %v", err)
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestSessionLeaseSourceGuardsAllSessionMapMutations(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("source location unavailable")
	}
	source, err := os.ReadFile(filepath.Join(filepath.Dir(file), "sessionruntime.go"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(source)
	if got := strings.Count(text, "m.sessions[id] = r"); got != 2 {
		t.Fatalf("session map assignments changed: %d", got)
	}
	if got := strings.Count(text, "delete(m.sessions, operation.SessionID)"); got != 1 {
		t.Fatalf("session map deletions changed: %d", got)
	}
	for _, guard := range []string{
		"if current := m.sessions[id]; current != nil",
		"if m.sessions[id] != nil || len(m.sessions) >= m.limits.MaxSessions",
		"if m.closed || r.leaseCount != 0 || (r.protocolOwned && !r.lifecycleOwned)",
	} {
		if !strings.Contains(text, guard) {
			t.Fatalf("missing lease mutation guard %q", guard)
		}
	}
}

func TestSessionLeaseLifecycleConflictAndShutdownCancellation(t *testing.T) {
	m, started := readyLeaseManager(t, 4)
	req := SessionLeaseRequest{SessionID: started.SessionID, Generation: started.Generation, Workspace: "/workspace"}
	entered := make(chan context.Context, 1)
	release := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		done <- m.WithReadySessionLease(context.Background(), req, func(ctx context.Context) error {
			entered <- ctx
			<-release
			return nil
		})
	}()
	leaseCtx := <-entered
	if got := m.Stop(context.Background(), req.SessionID, "stop"); got.Failure != session.LifecycleConflict {
		t.Fatalf("stop under lease: %+v", got)
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if err := m.Shutdown(shutdownCtx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("shutdown timeout: %v", err)
	}
	select {
	case <-leaseCtx.Done():
	case <-time.After(time.Second):
		t.Fatal("shutdown did not cancel lease context")
	}
	if c := m.Census(); c.Workers != 1 {
		t.Fatalf("shutdown released lease early: %+v", c)
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}
