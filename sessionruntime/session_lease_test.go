package sessionruntime

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"lsp-trace/internal/managedprocess"
	"lsp-trace/internal/runtimeprofile"
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

func TestSessionLeaseStalePoisonPreservesCurrentLease(t *testing.T) {
	m, started, child := roundTripManager(t, "success")
	defer child.Close()
	req := SessionLeaseRequest{SessionID: started.SessionID, Generation: started.Generation, Workspace: "/workspace"}
	if err := m.WithReadySessionLease(context.Background(), req, func(ctx context.Context) error {
		before := m.Census()
		got := m.Poison(req.SessionID, req.Generation+1)
		if got.Failure != session.StaleGeneration || got.Generation != req.Generation || got.State != session.Ready {
			t.Fatalf("stale poison taxonomy: %+v", got)
		}
		select {
		case <-ctx.Done():
			t.Fatalf("stale poison cancelled current lease: %v", context.Cause(ctx))
		default:
		}
		if after := m.Census(); after.Workers != before.Workers {
			t.Fatalf("stale poison changed worker count: before=%+v after=%+v", before, after)
		}
		m.mu.Lock()
		r := m.sessions[req.SessionID]
		unchanged := r != nil && r.leaseCount == 1 && r.leaseToken != nil && r.leaseCancel != nil && !r.leaseClosing
		m.mu.Unlock()
		if !unchanged {
			t.Fatal("stale poison changed lease token, cancel, count, or closing state")
		}
		gotRoundTrip := m.RoundTrip(ctx, RoundTripRequest{SessionID: req.SessionID, Generation: req.Generation, Method: "textDocument/references", Params: json.RawMessage(`{}`), Deadline: time.Now().Add(time.Second), MaxMessages: 3, MaxBytes: 4096})
		if gotRoundTrip.Failure != "" {
			t.Fatalf("sequential leased operation after stale poison: %+v", gotRoundTrip)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if c := m.Census(); c.Workers != 0 {
		t.Fatalf("final worker count: %+v", c)
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

func TestSessionLeaseRejectsCopiedContextForAnotherManagerAndCancelledInput(t *testing.T) {
	m, started := readyLeaseManager(t, 4)
	other, otherStarted := readyLeaseManager(t, 4)
	req := SessionLeaseRequest{SessionID: started.SessionID, Generation: started.Generation, Workspace: "/workspace"}
	var copied context.Context
	if err := m.WithReadySessionLease(context.Background(), req, func(ctx context.Context) error {
		copied = ctx
		got := other.PrepareDocument(ctx, DocumentRequest{SessionID: otherStarted.SessionID, Generation: otherStarted.Generation, URI: "file:///workspace/a.go"})
		if got.Failure != session.LifecycleConflict {
			t.Fatalf("cross-manager copied lease: %+v", got)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if copied == nil {
		t.Fatal("missing copied context")
	}
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	called := false
	err := m.WithReadySessionLease(cancelled, req, func(ctx context.Context) error {
		called = true
		return context.Cause(ctx)
	})
	if !called || !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled input callback: called=%t err=%v", called, err)
	}
	if c := m.Census(); c.Workers != 0 {
		t.Fatalf("cancelled input exact release: %+v", c)
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

func controlledLeaseManager(t *testing.T) (*Manager, StartResult, *roundTripChild) {
	t.Helper()
	child := newRoundTripChild("controlled-success")
	child.requestSeen = make(chan struct{}, 1)
	child.releaseResponse = make(chan struct{})
	m, err := New(Config{Limits: Limits{MaxSessions: 1, MaxRequests: 2, MaxChildren: 3, MaxCancels: 2, MaxTombstones: 4, MaxObservations: 64, MaxOperations: 2}, Starter: oneChildStarter{child}})
	if err != nil {
		t.Fatal(err)
	}
	started := m.Start(context.Background(), StartRequest{Profile: profile(t)})
	if got := m.ObserveInitialization(started.SessionID, started.Generation, true); got.State != session.Ready {
		t.Fatal(got)
	}
	return m, started, child
}

func leasedRoundTripRequest(started StartResult) RoundTripRequest {
	return RoundTripRequest{SessionID: started.SessionID, Generation: started.Generation, Method: "textDocument/references", Params: json.RawMessage(`{}`), Deadline: time.Now().Add(time.Second), MaxMessages: 1, MaxBytes: 4096}
}

func TestSessionLeaseCallbackReturnKeepsActiveRequestOwned(t *testing.T) {
	m, started, child := controlledLeaseManager(t)
	defer child.Close()
	lease := SessionLeaseRequest{SessionID: started.SessionID, Generation: started.Generation, Workspace: "/workspace"}
	result := make(chan RoundTripResult, 1)
	if err := m.WithReadySessionLease(context.Background(), lease, func(ctx context.Context) error {
		go func() { result <- m.RoundTrip(ctx, leasedRoundTripRequest(started)) }()
		<-child.requestSeen
		if c := m.Census(); c.Workers != 2 {
			t.Fatalf("lease plus request workers: %+v", c)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	m.mu.Lock()
	r := m.sessions[started.SessionID]
	requests, owned, workers, leases := len(r.requests), r.protocolOwned, m.workers, r.leaseCount
	m.mu.Unlock()
	if requests != 1 || !owned || workers != 1 || leases != 0 {
		t.Fatalf("callback return continuity requests=%d owned=%t workers=%d leases=%d", requests, owned, workers, leases)
	}
	if got := m.Stop(context.Background(), started.SessionID, "stop"); got.Failure != session.LifecycleConflict {
		t.Fatalf("lifecycle replacement gap: %+v", got)
	}
	if err := m.WithReadySessionLease(context.Background(), lease, func(context.Context) error { return nil }); !errors.Is(err, errSessionLeaseConflict) {
		t.Fatalf("replacement lease admitted during request: %v", err)
	}
	close(child.releaseResponse)
	if got := <-result; got.Failure != "" {
		t.Fatalf("active request terminal: %+v", got)
	}
	if c := m.Census(); c.Workers != 0 {
		t.Fatalf("final workers: %+v", c)
	}
}

func TestSessionLeaseConcurrentRoundTripRefusesProtocolOwned(t *testing.T) {
	m, started, child := controlledLeaseManager(t)
	defer child.Close()
	lease := SessionLeaseRequest{SessionID: started.SessionID, Generation: started.Generation, Workspace: "/workspace"}
	if err := m.WithReadySessionLease(context.Background(), lease, func(ctx context.Context) error {
		result := make(chan RoundTripResult, 1)
		go func() { result <- m.RoundTrip(ctx, leasedRoundTripRequest(started)) }()
		<-child.requestSeen
		if got := m.RoundTrip(ctx, leasedRoundTripRequest(started)); got.Failure != session.LifecycleConflict {
			t.Fatalf("concurrent protocolOwned refusal: %+v", got)
		}
		close(child.releaseResponse)
		if got := <-result; got.Failure != "" {
			t.Fatalf("first request terminal: %+v", got)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if c := m.Census(); c.Workers != 0 {
		t.Fatalf("final workers: %+v", c)
	}
}

func TestSessionLeaseSequentialPrepareAndRoundTrip(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "a.go")
	if err := os.WriteFile(path, []byte("package fixture\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	selector, err := runtimeprofile.Validate(runtimeprofile.Selector{TrustDomain: "test", Workspace: root, Profile: "go", EnvironmentReference: "local"})
	if err != nil {
		t.Fatal(err)
	}
	child := newRoundTripChild("references-empty")
	defer child.Close()
	m, err := New(Config{Limits: Limits{MaxSessions: 1, MaxRequests: 2, MaxChildren: 3, MaxCancels: 2, MaxTombstones: 4, MaxObservations: 64, MaxOperations: 2}, Starter: oneChildStarter{child}})
	if err != nil {
		t.Fatal(err)
	}
	started := m.Start(context.Background(), StartRequest{Profile: runtimeprofile.Resolve(selector)})
	if got := m.ObserveInitialization(started.SessionID, started.Generation, true); got.State != session.Ready {
		t.Fatal(got)
	}
	lease := SessionLeaseRequest{SessionID: started.SessionID, Generation: started.Generation, Workspace: root}
	if err := m.WithReadySessionLease(context.Background(), lease, func(ctx context.Context) error {
		uri := "file://" + filepath.ToSlash(path)
		prepared := m.PrepareDocument(ctx, DocumentRequest{SessionID: started.SessionID, Generation: started.Generation, URI: uri})
		if prepared.Failure != "" || prepared.Version != 1 {
			t.Fatalf("leased prepare: %+v", prepared)
		}
		got := m.RoundTrip(ctx, RoundTripRequest{SessionID: started.SessionID, Generation: started.Generation, Method: "textDocument/references", Params: json.RawMessage(`{}`), Deadline: time.Now().Add(time.Second), MaxMessages: 1, MaxBytes: 4096})
		if got.Failure != "" || got.Key.Generation != started.Generation || got.Key.ID == 0 || got.RequestMessages != 1 || got.Messages != 1 {
			t.Fatalf("leased round trip keys/accounting: %+v", got)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if c := m.Census(); c.Workers != 0 {
		t.Fatalf("final workers: %+v", c)
	}
}

func TestSessionLeaseExactPoisonCancelsActiveRoundTripOnce(t *testing.T) {
	m, started, child := controlledLeaseManager(t)
	defer child.Close()
	lease := SessionLeaseRequest{SessionID: started.SessionID, Generation: started.Generation, Workspace: "/workspace"}
	if err := m.WithReadySessionLease(context.Background(), lease, func(ctx context.Context) error {
		result := make(chan RoundTripResult, 1)
		go func() { result <- m.RoundTrip(ctx, leasedRoundTripRequest(started)) }()
		<-child.requestSeen
		poisoned := m.Poison(started.SessionID, started.Generation)
		if poisoned.Failure != session.SessionPoisoned || poisoned.State != session.Poisoned {
			t.Fatalf("exact poison: %+v", poisoned)
		}
		close(child.releaseResponse)
		got := <-result
		if got.Failure != session.RequestCancelled {
			t.Fatalf("poisoned request terminal: %+v", got)
		}
		m.mu.Lock()
		r := m.sessions[started.SessionID]
		requests, owned := len(r.requests), r.protocolOwned
		m.mu.Unlock()
		if requests != 0 || owned {
			t.Fatalf("request terminal not exact requests=%d owned=%t", requests, owned)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if c := m.Census(); c.Workers != 0 {
		t.Fatalf("final workers: %+v", c)
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
