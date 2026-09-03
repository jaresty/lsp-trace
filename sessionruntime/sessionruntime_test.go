package sessionruntime

import (
	"context"
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
	if got := m.Restart(context.Background(), first.SessionID, "caller-1"); got.Failure != "" || got.Generation != 2 {
		t.Fatalf("assert restart observation: %+v", got)
	}
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
