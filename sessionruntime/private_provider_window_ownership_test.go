package sessionruntime

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestPrivateProviderWindowLateStarterRetainsPlaceholder(t *testing.T) {
	m, start, ctx, s := windowTestSetup(t, PrivateProviderWindowPlan{Definitions: true}, "")
	s.delay = make(chan struct{})
	s.entered = make(chan struct{})
	done := make(chan StartResult, 1)
	go func() { done <- m.Start(ctx, start) }()
	<-s.entered
	w := privateProviderWindowFrom(ctx)
	w.cancel()
	select {
	case got := <-done:
		if got.Failure != privateProviderWindowUnjoined {
			t.Fatal("ASSERT_WINDOW_LATE_START_RETURN")
		}
	case <-time.After(time.Second):
		t.Fatal("ASSERT_WINDOW_START_CALLER_ISOLATION")
	}
	if m.Census().Workers == 0 {
		t.Fatal("ASSERT_WINDOW_START_PLACEHOLDER")
	}
	close(s.delay)
	select {
	case <-w.finishDone:
	case <-time.After(2 * time.Second):
		t.Fatal("ASSERT_WINDOW_LATE_START_JOIN")
	}
	if got, err := m.FinishPrivateProviderWindow(ctx); err == nil || got.Completed {
		t.Fatal("ASSERT_WINDOW_LATE_START_NO_SUCCESS")
	}
}
func TestPrivateProviderWindowStalledShutdownAndReaderJoin(t *testing.T) {
	for _, stall := range []string{"shutdown-write", "shutdown", "reader-join"} {
		t.Run(stall, func(t *testing.T) {
			plan := PrivateProviderWindowPlan{Definitions: true}
			m, start, ctx, s := windowTestSetup(t, plan, stall)
			w := privateProviderWindowFrom(ctx)
			if stall == "reader-join" {
				st := m.Start(ctx, start)
				op := m.BeginReadiness(ctx, st.SessionID, st.Generation, start.Deadline)
				_ = op
				<-s.child.entered
			} else {
				windowTestReady(t, m, start, ctx)
				windowTestQueries(t, m, start, ctx, s.child, plan)
			}
			returned := make(chan error, 1)
			go func() { _, e := m.FinishPrivateProviderWindow(ctx); returned <- e }()
			if stall != "reader-join" {
				<-s.child.entered
			}
			w.cancel()
			select {
			case err := <-returned:
				if err != ErrPrivateProviderWindowUnjoined {
					t.Fatal("ASSERT_WINDOW_SHUTDOWN_NOT_COMPLETED")
				}
			case <-time.After(time.Second):
				t.Fatal("ASSERT_WINDOW_SHUTDOWN_CALLER_ISOLATION")
			}
			if m.Census().Workers == 0 {
				t.Fatal("ASSERT_WINDOW_SHUTDOWN_RETAINED_WORKER")
			}
			close(s.child.release)
			select {
			case <-w.finishDone:
			case <-time.After(2 * time.Second):
				t.Fatal("ASSERT_WINDOW_SHUTDOWN_ACTUAL_JOIN")
			}
			if got, e := m.FinishPrivateProviderWindow(ctx); e == nil || got.Completed {
				t.Fatal("ASSERT_WINDOW_EXPIRED_RECONCILIATION_NO_SUCCESS")
			}
		})
	}
}
func TestPrivateProviderWindowFailedClosureCannotComplete(t *testing.T) {
	plan := PrivateProviderWindowPlan{Definitions: true}
	m, start, ctx, s := windowTestSetup(t, plan, "")
	s.child.closeFailure = true
	windowTestReady(t, m, start, ctx)
	windowTestQueries(t, m, start, ctx, s.child, plan)
	got, err := m.FinishPrivateProviderWindow(ctx)
	if err == nil || got.Completed {
		t.Fatal("ASSERT_WINDOW_ALL_RESOURCES_REQUIRED")
	}
	w := privateProviderWindowFrom(ctx)
	w.mu.Lock()
	released := w.released
	w.mu.Unlock()
	if released || m.Census().Workers == 0 {
		t.Fatal("ASSERT_WINDOW_UNCLOSED_OWNERSHIP_RETAINED")
	}
	m.mu.Lock()
	r := m.sessions[w.start.SessionID]
	retained := r != nil && r.protocolOwned && r.lifecycleOwned
	m.mu.Unlock()
	if !retained {
		t.Fatal("ASSERT_WINDOW_UNCLOSED_PROTOCOL_SESSION_RETAINED")
	}
}
func TestPrivateProviderWindowReservationBeforeBackend(t *testing.T) {
	plan := PrivateProviderWindowPlan{Definitions: true}
	m, start, ctx, s := windowTestSetup(t, plan, "")
	w := privateProviderWindowFrom(ctx)
	var mu sync.Mutex
	attempted := uint64(0)
	paid := true
	s.child.beforeWrite = func(n int) {
		mu.Lock()
		defer mu.Unlock()
		attempted += uint64(n)
		w.mu.Lock()
		reserved := w.reservedWrite
		w.mu.Unlock()
		if reserved < attempted {
			paid = false
		}
	}
	windowTestReady(t, m, start, ctx)
	windowTestQueries(t, m, start, ctx, s.child, plan)
	if _, err := m.FinishPrivateProviderWindow(ctx); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	defer mu.Unlock()
	if !paid || attempted == 0 {
		t.Fatal("ASSERT_WINDOW_RESERVED_BEFORE_BACKEND")
	}
}
func TestPrivateProviderWindowContextDeadlineCannotRenew(t *testing.T) {
	m, start, ctx, _ := windowTestSetup(t, PrivateProviderWindowPlan{Definitions: true}, "")
	w := privateProviderWindowFrom(ctx)
	earlier, cancel := context.WithDeadline(context.Background(), start.Deadline.Add(-time.Second))
	defer cancel()
	if _, err := m.PrivateProviderWindowContext(earlier, PrivateProviderWindowEnvelope{w.spec}, start.Deadline); err == nil {
		t.Fatal("ASSERT_WINDOW_PARENT_DEADLINE_BOUND")
	}
	_, _ = m.FinishPrivateProviderWindow(ctx)
}

func TestPrivateProviderWindowActualLeaseJoinBeforeCompletion(t *testing.T) {
	for _, expire := range []bool{false, true} {
		t.Run(map[bool]string{false: "joined", true: "expired"}[expire], func(t *testing.T) {
			plan := PrivateProviderWindowPlan{Definitions: true}
			m, start, ctx, s := windowTestSetup(t, plan, "")
			st := windowTestReady(t, m, start, ctx)
			windowTestQueries(t, m, start, ctx, s.child, plan)
			entered, release := make(chan struct{}), make(chan struct{})
			var once sync.Once
			releaseLease := func() { once.Do(func() { close(release) }) }
			t.Cleanup(releaseLease)
			leaseReturned := make(chan error, 1)
			go func() {
				leaseReturned <- m.WithReadySessionLease(ctx, SessionLeaseRequest{SessionID: st.SessionID, Generation: st.Generation, Workspace: start.Profile.Workspace().String()}, func(context.Context) error { close(entered); <-release; return nil })
			}()
			<-entered
			type outcome struct {
				completion PrivateProviderWindowCompletion
				err        error
			}
			returned := make(chan outcome, 1)
			go func() { c, e := m.FinishPrivateProviderWindow(ctx); returned <- outcome{c, e} }()
			select {
			case <-returned:
				t.Fatal("ASSERT_WINDOW_LEASE_PREMATURE_COMPLETION")
			case <-time.After(10 * time.Millisecond):
			}
			w := privateProviderWindowFrom(ctx)
			w.mu.Lock()
			released := w.released
			w.mu.Unlock()
			m.mu.Lock()
			r := m.sessions[st.SessionID]
			retained := r != nil && r.leaseCount == 1
			m.mu.Unlock()
			if released || !retained || m.Census().Workers < 2 {
				t.Fatal("ASSERT_WINDOW_ACTIVE_LEASE_OWNER_RETAINED")
			}
			if expire {
				w.cancel()
			}
			releaseLease()
			<-leaseReturned
			select {
			case got := <-returned:
				if expire {
					if got.err == nil || got.completion.Completed {
						t.Fatal("ASSERT_WINDOW_LEASE_EXPIRY_IRREVERSIBLE")
					}
				} else if got.err != nil || !got.completion.Completed {
					t.Fatalf("ASSERT_WINDOW_POST_LEASE_COMPLETES: %+v %v", got.completion, got.err)
				}
			case <-time.After(2 * time.Second):
				t.Fatal("ASSERT_WINDOW_LEASE_CALLER_RETURN")
			}
			select {
			case <-w.finishDone:
			case <-time.After(2 * time.Second):
				t.Fatal("ASSERT_WINDOW_LEASE_CLEANUP_JOIN")
			}
			if !expire {
				w.mu.Lock()
				joined := w.lifecycleJoined
				w.mu.Unlock()
				if !joined || m.Census().Workers != 0 {
					t.Fatal("ASSERT_WINDOW_LIFECYCLE_JOIN_HOOK")
				}
			}
		})
	}
}
