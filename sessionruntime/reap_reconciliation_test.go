package sessionruntime

import (
	"context"
	"errors"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"lsp-trace/internal/managedprocess"
	"lsp-trace/internal/runtimeprofile"
	"lsp-trace/internal/session"
)

type reapJoinChild struct {
	join       chan managedprocess.TeardownObservation
	teardowns  atomic.Int32
	joins      atomic.Int32
	closeCalls atomic.Int32
}

func newReapJoinChild() *reapJoinChild {
	return &reapJoinChild{join: make(chan managedprocess.TeardownObservation, 1)}
}

func (c *reapJoinChild) Teardown(context.Context) managedprocess.TeardownObservation {
	c.teardowns.Add(1)
	return managedprocess.TeardownObservation{Death: managedprocess.DeathObservation{
		Kind: managedprocess.DeathUnknown,
		Reap: managedprocess.ReapObservation{Kind: managedprocess.ReapFailed, Err: errors.New("owned wait completion unresolved")},
	}}
}

func (c *reapJoinChild) Close() managedprocess.ResourceObservation {
	c.closeCalls.Add(1)
	return managedprocess.ResourceObservation{Kind: managedprocess.ResourcesClosed}
}

func (c *reapJoinChild) ReconcileReap(ctx context.Context) managedprocess.TeardownObservation {
	c.joins.Add(1)
	select {
	case observation := <-c.join:
		return observation
	case <-ctx.Done():
		return managedprocess.TeardownObservation{Death: managedprocess.DeathObservation{
			Kind: managedprocess.DeathUnknown,
			Reap: managedprocess.ReapObservation{Kind: managedprocess.ReapFailed, Err: context.Cause(ctx)},
		}}
	}
}

func completedReapObservation() managedprocess.TeardownObservation {
	return managedprocess.TeardownObservation{
		Phases: []managedprocess.TeardownPhase{managedprocess.PhaseReap},
		Death: managedprocess.DeathObservation{
			Kind: managedprocess.DeathExited,
			Reap: managedprocess.ReapObservation{Kind: managedprocess.ReapComplete},
		},
		Census: managedprocess.GroupCensusObservation{Bounded: true, Limit: 64, Members: 0},
	}
}

func recordByID(t *testing.T, m *Manager, id string) Record {
	t.Helper()
	for _, record := range m.Records() {
		if record.SessionID == id {
			return record
		}
	}
	t.Fatalf("record %q not found", id)
	return Record{}
}

func reapManager(t *testing.T, maxSessions int, children ...Child) (*Manager, *sequenceStarter) {
	t.Helper()
	starter := &sequenceStarter{children: children}
	m, err := New(Config{Limits: Limits{
		MaxSessions: maxSessions, MaxRequests: 2, MaxChildren: 2,
		MaxCancels: 2, MaxTombstones: 2, MaxObservations: 64, MaxOperations: 16,
	}, Starter: starter})
	if err != nil {
		t.Fatal(err)
	}
	return m, starter
}

func TestExactRestartReconcilesConfirmedOwnedReap(t *testing.T) {
	child := newReapJoinChild()
	m, starter := reapManager(t, 1, child, referenceChild{})
	started := m.Start(context.Background(), StartRequest{Profile: profile(t)})
	failedStop := m.Stop(context.Background(), started.SessionID, "stop-owner")
	failed := waitOperation(t, m, failedStop.IntentID, OperationFailed)
	if failed.Failure != session.SessionReapIncomplete {
		t.Fatalf("ASSERT_REAP_RECONCILIATION_PRECONDITION: %+v", failed)
	}
	child.join <- completedReapObservation()

	restart := m.Restart(context.Background(), started.SessionID, "restart-owner")
	if restart.Failure != "" || restart.IntentID == "" {
		t.Fatalf("ASSERT_REAP_RECONCILIATION_CONFIRMED_RESTART_ACCEPTED: %+v", restart)
	}
	terminal := waitOperation(t, m, restart.IntentID, OperationComplete)
	records := m.Records()
	if len(records) != 1 || records[0].Generation != 2 || terminal.Failure != "" || child.joins.Load() != 1 || child.teardowns.Load() != 1 || child.closeCalls.Load() != 1 || starter.Starts() != 2 {
		t.Fatalf("ASSERT_REAP_RECONCILIATION_EXACT_ONCE_SUCCESSOR: terminal=%+v records=%+v joins=%d teardowns=%d closes=%d starts=%d", terminal, records, child.joins.Load(), child.teardowns.Load(), child.closeCalls.Load(), starter.Starts())
	}
}

func TestExactReapReconciliationFailsClosedForTimeoutAndHelpers(t *testing.T) {
	for _, tc := range []struct {
		name        string
		observation *managedprocess.TeardownObservation
	}{
		{name: "timeout"},
		{name: "helper-present", observation: func() *managedprocess.TeardownObservation {
			o := completedReapObservation()
			o.Census.Members = 1
			return &o
		}()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			child := newReapJoinChild()
			m, starter := reapManager(t, 1, child, referenceChild{})
			started := m.Start(context.Background(), StartRequest{Profile: profile(t)})
			stop := m.Stop(context.Background(), started.SessionID, "stop-owner")
			waitOperation(t, m, stop.IntentID, OperationFailed)
			if tc.observation != nil {
				child.join <- *tc.observation
			}
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
			defer cancel()
			restart := m.Restart(ctx, started.SessionID, "restart-owner")
			if restart.Failure != session.SessionReapIncomplete || restart.IntentID != "" || starter.Starts() != 1 || child.teardowns.Load() != 1 {
				t.Fatalf("ASSERT_REAP_RECONCILIATION_FAILS_CLOSED_%s: restart=%+v starts=%d teardown=%d joins=%d", tc.name, restart, starter.Starts(), child.teardowns.Load(), child.joins.Load())
			}
		})
	}
}

func TestExactReapReconciliationDoesNotMutateSiblingOrReplacement(t *testing.T) {
	child := newReapJoinChild()
	m, _ := reapManager(t, 2, child, referenceChild{}, referenceChild{})
	first := m.Start(context.Background(), StartRequest{Profile: profile(t)})
	otherValidated, err := runtimeprofile.Validate(runtimeprofile.Selector{TrustDomain: "test", Workspace: "/sibling", Profile: "go", EnvironmentReference: "local"})
	if err != nil {
		t.Fatal(err)
	}
	sibling := m.Start(context.Background(), StartRequest{Profile: runtimeprofile.Resolve(otherValidated)})
	stop := m.Stop(context.Background(), first.SessionID, "stop-owner")
	waitOperation(t, m, stop.IntentID, OperationFailed)
	beforeSibling := recordByID(t, m, sibling.SessionID)

	// Simulate a replacement installed before the stale owned completion arrives.
	m.mu.Lock()
	original := m.sessions[first.SessionID].process
	replacement := referenceChild{}
	m.sessions[first.SessionID].process = replacement
	m.mu.Unlock()
	child.join <- completedReapObservation()

	restart := m.Restart(context.Background(), first.SessionID, "restart-owner")
	afterSibling := recordByID(t, m, sibling.SessionID)
	m.mu.Lock()
	current := m.sessions[first.SessionID].process
	m.mu.Unlock()
	if restart.Failure != session.SessionReapIncomplete || child.joins.Load() != 0 || current != replacement || original == current || !reflect.DeepEqual(beforeSibling, afterSibling) {
		t.Fatalf("ASSERT_REAP_RECONCILIATION_STALE_IDENTITY_AND_SIBLING_ISOLATION: restart=%+v joins=%d current=%T sibling_before=%+v sibling_after=%+v", restart, child.joins.Load(), current, beforeSibling, afterSibling)
	}
}

func TestConcurrentStopRestartHasSingleReapReconciliationOwner(t *testing.T) {
	child := newReapJoinChild()
	m, starter := reapManager(t, 1, child, referenceChild{})
	started := m.Start(context.Background(), StartRequest{Profile: profile(t)})
	stop := m.Stop(context.Background(), started.SessionID, "stop-owner")
	waitOperation(t, m, stop.IntentID, OperationFailed)
	child.join <- completedReapObservation()

	const callers = 16
	results := make(chan session.LifecycleResult, callers)
	var wg sync.WaitGroup
	for i := 0; i < callers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if i%2 == 0 {
				results <- m.Restart(context.Background(), started.SessionID, "restart-racer")
				return
			}
			results <- m.Stop(context.Background(), started.SessionID, "stop-racer")
		}(i)
	}
	wg.Wait()
	close(results)
	for result := range results {
		if result.Failure != "" && result.Failure != session.LifecycleConflict && result.Failure != session.SessionReapIncomplete && result.Failure != session.StaleGeneration {
			t.Fatalf("ASSERT_REAP_RECONCILIATION_CONCURRENT_TYPED_RESULT: %+v", result)
		}
	}
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) && starter.Starts() < 2 {
		time.Sleep(time.Millisecond)
	}
	if child.joins.Load() != 1 || child.teardowns.Load() != 1 || starter.Starts() > 2 {
		t.Fatalf("ASSERT_REAP_RECONCILIATION_CONCURRENT_SINGLE_OWNER: joins=%d teardowns=%d starts=%d census=%+v", child.joins.Load(), child.teardowns.Load(), starter.Starts(), m.Census())
	}
}
