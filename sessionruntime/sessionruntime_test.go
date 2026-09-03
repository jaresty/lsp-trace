package sessionruntime

import (
	"context"
	"sync"
	"testing"
	"time"

	"lsp-trace/internal/containment"
	"lsp-trace/internal/managedprocess"
	"lsp-trace/internal/runtimeprofile"
	"lsp-trace/internal/session"
)

func profile(t *testing.T) runtimeprofile.Profile {
	t.Helper()
	v, err := runtimeprofile.Validate(runtimeprofile.Selector{TrustDomain: "test", Workspace: "/workspace", Profile: "go", EnvironmentReference: "local"})
	if err != nil {
		t.Fatal(err)
	}
	return runtimeprofile.Resolve(v)
}

func TestUnavailableHasNoSessionOrProcessSideEffects(t *testing.T) {
	m, err := New(Config{Limits: Limits{MaxSessions: 2, MaxRequests: 2, MaxChildren: 2, MaxCancels: 2, MaxTombstones: 2, MaxObservations: 8}, Starter: ManagedStarter{Manager: managedprocess.New(containment.NewRuntimeGate(), managedprocess.Options{})}})
	if err != nil {
		t.Fatal(err)
	}
	before := m.Census()
	got := m.Start(context.Background(), StartRequest{Profile: profile(t), Process: managedprocess.Spec{Path: "must-not-run"}})
	if got.Failure != session.ProcessContainmentUnavailable {
		t.Fatalf("assert unavailable side-effect-free: failure=%q", got.Failure)
	}
	if after := m.Census(); after != before {
		t.Fatalf("assert unavailable side-effect-free: census changed: before=%+v after=%+v", before, after)
	}
}

func TestDeadlineWinsBeforeStart(t *testing.T) {
	m, err := New(Config{Limits: Limits{MaxSessions: 1, MaxRequests: 1, MaxChildren: 1, MaxCancels: 1, MaxTombstones: 1, MaxObservations: 4}, Starter: ManagedStarter{Manager: managedprocess.New(containment.NewRuntimeGate(), managedprocess.Options{})}})
	if err != nil {
		t.Fatal(err)
	}
	got := m.Start(context.Background(), StartRequest{Profile: profile(t), Deadline: time.Now().Add(-time.Second)})
	if got.Failure != session.RequestTimeout {
		t.Fatalf("assert terminal arbitration: failure=%q", got.Failure)
	}
	if m.Census().Sessions != 0 {
		t.Fatal("assert terminal arbitration: timed out start created session")
	}
}

func TestLimitsMustBePositive(t *testing.T) {
	_, err := New(Config{})
	if err == nil {
		t.Fatal("assert bounded census: zero limits accepted")
	}
}

type referenceStarter struct{ starts int }
type referenceChild struct{}

type blockingChild struct {
	release chan struct{}
}

func (c *blockingChild) Teardown(context.Context) managedprocess.TeardownObservation {
	<-c.release
	return managedprocess.TeardownObservation{Death: managedprocess.DeathObservation{Kind: managedprocess.DeathExited, Reap: managedprocess.ReapObservation{Kind: managedprocess.ReapComplete}}}
}
func (*blockingChild) Close() managedprocess.ResourceObservation {
	return managedprocess.ResourceObservation{Kind: managedprocess.ResourcesClosed}
}

type blockingStarter struct{ child *blockingChild }

func (s *blockingStarter) Start(context.Context, managedprocess.Spec) (Child, managedprocess.StartObservation) {
	return s.child, managedprocess.StartObservation{Kind: managedprocess.StartStarted, Evidence: "blocking seam"}
}

func TestStopAcceptanceReturnsWhileTeardownPending(t *testing.T) {
	child := &blockingChild{release: make(chan struct{})}
	m, err := New(Config{Limits: Limits{MaxSessions: 1, MaxRequests: 1, MaxChildren: 1, MaxCancels: 1, MaxTombstones: 1, MaxObservations: 8}, Starter: &blockingStarter{child: child}})
	if err != nil {
		t.Fatal(err)
	}
	started := m.Start(context.Background(), StartRequest{Profile: profile(t), Process: managedprocess.Spec{Path: "blocking"}})
	returned := make(chan struct{})
	go func() {
		m.Stop(context.Background(), started.SessionID, "caller")
		close(returned)
	}()
	select {
	case <-returned:
	case <-time.After(100 * time.Millisecond):
		close(child.release)
		<-returned
		t.Fatal("stop acceptance blocked on child teardown")
	}
	close(child.release)
}

func (s *referenceStarter) Start(context.Context, managedprocess.Spec) (Child, managedprocess.StartObservation) {
	s.starts++
	return referenceChild{}, managedprocess.StartObservation{Kind: managedprocess.StartStarted, Evidence: "reference seam only"}
}
func (referenceChild) Teardown(context.Context) managedprocess.TeardownObservation {
	return managedprocess.TeardownObservation{Death: managedprocess.DeathObservation{Kind: managedprocess.DeathExited, Reap: managedprocess.ReapObservation{Kind: managedprocess.ReapComplete}}}
}
func (referenceChild) Close() managedprocess.ResourceObservation {
	return managedprocess.ResourceObservation{Kind: managedprocess.ResourcesClosed}
}

func waitOperation(t *testing.T, m *Manager, id string, want OperationState) OperationSnapshot {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if got, ok := m.Operation(id); ok && got.State == want {
			return got
		}
		time.Sleep(time.Millisecond)
	}
	got, ok := m.Operation(id)
	t.Fatalf("operation did not reach %s: found=%v snapshot=%+v", want, ok, got)
	return OperationSnapshot{}
}

func TestRestartOperationPendingReplayOrderingAndCompletion(t *testing.T) {
	first := &blockingChild{release: make(chan struct{})}
	starter := &sequenceStarter{children: []Child{first, referenceChild{}}}
	m, err := New(Config{Limits: Limits{MaxSessions: 1, MaxRequests: 1, MaxChildren: 1, MaxCancels: 1, MaxTombstones: 1, MaxObservations: 16, MaxOperations: 2}, Starter: starter})
	if err != nil {
		t.Fatal(err)
	}
	started := m.Start(context.Background(), StartRequest{Profile: profile(t), Process: managedprocess.Spec{Path: "blocking"}})
	accepted := m.Restart(context.Background(), started.SessionID, "caller")
	if accepted.IntentID == "" {
		t.Fatal("restart acceptance omitted immutable operation ID")
	}
	replayed := m.Restart(context.Background(), started.SessionID, "caller")
	if replayed.IntentID != accepted.IntentID {
		t.Fatalf("caller replay changed operation ID: first=%q replay=%q", accepted.IntentID, replayed.IntentID)
	}
	pending, ok := m.Operation(accepted.IntentID)
	if !ok || pending.State != OperationPending || pending.Generation != 1 {
		t.Fatalf("restart operation not queryable pending: found=%v snapshot=%+v", ok, pending)
	}
	if got := starter.Starts(); got != 1 {
		t.Fatalf("successor started before predecessor terminal: starts=%d", got)
	}
	if c := m.Census(); c.Operations != 1 || c.Workers != 1 {
		t.Fatalf("operation accounting mismatch while pending: %+v", c)
	}
	close(first.release)
	terminal := waitOperation(t, m, accepted.IntentID, OperationComplete)
	if starter.Starts() != 2 || terminal.Generation != 1 {
		t.Fatalf("restart completion/order mismatch: starts=%d snapshot=%+v", starter.Starts(), terminal)
	}
	again, _ := m.Operation(accepted.IntentID)
	if again != terminal {
		t.Fatalf("terminal snapshot mutated: first=%+v again=%+v", terminal, again)
	}
	if err := m.Shutdown(context.Background()); err != nil || m.Census().Workers != 0 {
		t.Fatalf("shutdown did not own workers: err=%v census=%+v", err, m.Census())
	}
}

type sequenceStarter struct {
	mu       sync.Mutex
	children []Child
	starts   int
}

func (s *sequenceStarter) Start(context.Context, managedprocess.Spec) (Child, managedprocess.StartObservation) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.starts >= len(s.children) {
		return nil, managedprocess.StartObservation{}
	}
	child := s.children[s.starts]
	s.starts++
	return child, managedprocess.StartObservation{Kind: managedprocess.StartStarted, Evidence: "sequence seam"}
}
func (s *sequenceStarter) Starts() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.starts
}

type failedChild struct{}

func (failedChild) Teardown(context.Context) managedprocess.TeardownObservation {
	return managedprocess.TeardownObservation{}
}
func (failedChild) Close() managedprocess.ResourceObservation {
	return managedprocess.ResourceObservation{}
}

func TestFailedTerminationRemainsQueryableAndPoisoned(t *testing.T) {
	starter := &sequenceStarter{children: []Child{failedChild{}}}
	m, err := New(Config{Limits: Limits{MaxSessions: 1, MaxRequests: 1, MaxChildren: 1, MaxCancels: 1, MaxTombstones: 1, MaxObservations: 8, MaxOperations: 1}, Starter: starter})
	if err != nil {
		t.Fatal(err)
	}
	started := m.Start(context.Background(), StartRequest{Profile: profile(t)})
	accepted := m.Stop(context.Background(), started.SessionID, "caller")
	failed := waitOperation(t, m, accepted.IntentID, OperationFailed)
	if failed.Failure == "" {
		t.Fatalf("failed termination omitted failure: %+v", failed)
	}
	records := m.Records()
	if len(records) != 1 || records[0].State != session.Poisoned {
		t.Fatalf("failed termination deleted or failed to poison authority: %+v", records)
	}
	if again, ok := m.Operation(accepted.IntentID); !ok || again != failed {
		t.Fatalf("failed operation not immutable/queryable: found=%v first=%+v again=%+v", ok, failed, again)
	}
}

func TestReferenceSeamIdentityNoOverlapAndRestartObservations(t *testing.T) {
	starter := &referenceStarter{}
	m, err := New(Config{Limits: Limits{MaxSessions: 1, MaxRequests: 2, MaxChildren: 1, MaxCancels: 2, MaxTombstones: 2, MaxObservations: 16}, Starter: starter})
	if err != nil {
		t.Fatal(err)
	}
	p := profile(t)
	first := m.Start(context.Background(), StartRequest{Profile: p, Process: managedprocess.Spec{Path: "reference"}})
	if first.SessionID != p.SessionKey().String() || first.Generation != 1 {
		t.Fatalf("assert trusted identity: %+v", first)
	}
	second := m.Start(context.Background(), StartRequest{Profile: p})
	if second.Generation != 1 || starter.starts != 1 || m.Census().Generations != 1 {
		t.Fatalf("assert no generation overlap: second=%+v starts=%d census=%+v", second, starter.starts, m.Census())
	}
	if got := m.ObserveInitialization(first.SessionID, 1, true); got.State != session.Ready {
		t.Fatalf("assert initialization observation: %+v", got)
	}
	restart := m.Restart(context.Background(), first.SessionID, "caller-1")
	if restart.Failure != "" || restart.Generation != 1 || restart.IntentID == "" {
		t.Fatalf("assert restart acceptance: %+v", restart)
	}
	waitOperation(t, m, restart.IntentID, OperationComplete)
	if starter.starts != 2 || m.Census().Generations != 1 {
		t.Fatalf("assert restart no overlap: starts=%d census=%+v", starter.starts, m.Census())
	}
	kinds := map[string]bool{}
	for _, o := range m.Observations() {
		kinds[o.Kind] = true
	}
	for _, kind := range []string{"startup", "initialization", "restart", "teardown"} {
		if !kinds[kind] {
			t.Fatalf("assert lifecycle observations: missing %s in %+v", kind, m.Observations())
		}
	}
}
