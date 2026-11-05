package sessionruntime

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"lsp-trace/internal/managedprocess"
	"lsp-trace/internal/runtimeprofile"
	"lsp-trace/internal/session"
)

// Gate only actual owned backend callbacks; protocol work remains real io.Pipe.
type startupCleanupChild struct {
	*windowPipeChild
	phase            string
	entered, release chan struct{}
}

func (c *startupCleanupChild) Teardown(ctx context.Context) managedprocess.TeardownObservation {
	if c.phase == "teardown" {
		close(c.entered)
		<-c.release
	}
	return c.windowPipeChild.Teardown(ctx)
}
func (c *startupCleanupChild) Close() managedprocess.ResourceObservation {
	if c.phase == "close" {
		close(c.entered)
		<-c.release
	}
	return c.windowPipeChild.Close()
}

type startupCleanupStarter struct {
	child            *startupCleanupChild
	ordinary         []*windowPipeChild
	entered, release chan struct{}
}

func (*startupCleanupStarter) privateProviderWindowControlledBackend() {}
func (s *startupCleanupStarter) Start(ctx context.Context, _ managedprocess.Spec) (Child, managedprocess.StartObservation) {
	if privateProviderWindowFrom(ctx) == nil {
		child := s.ordinary[0]
		s.ordinary = s.ordinary[1:]
		return child, managedprocess.StartObservation{Kind: managedprocess.StartStarted}
	}
	close(s.entered)
	<-s.release
	return s.child, managedprocess.StartObservation{Kind: managedprocess.StartStarted}
}

type startupIdentityError struct{}

func (startupIdentityError) Read([]byte) (int, error) {
	return 0, errors.New("controlled identity failure")
}

func TestPrivateProviderWindowStartupCleanup(t *testing.T) {
	for _, mode := range []string{"collision", "capacity", "algebra", "history", "identity", "identity-close-failure"} {
		for _, phase := range []string{"teardown", "close"} {
			t.Run(mode+"/"+phase, func(t *testing.T) {
				m, req, ctx, original := windowTestSetup(t, PrivateProviderWindowPlan{Definitions: true}, "")
				w := privateProviderWindowFrom(ctx)
				child := &startupCleanupChild{windowPipeChild: original.child, phase: phase, entered: make(chan struct{}), release: make(chan struct{})}
				ordinary := newWindowPipeChild(original.child.uri, "")
				ordinaryChildren := []*windowPipeChild{ordinary, newWindowPipeChild(original.child.uri, "")}
				for _, c := range ordinaryChildren {
					go c.serve()
					defer func() {
						_ = c.input.Close()
						_ = c.output.Close()
						_ = c.serverInput.Close()
						_ = c.serverOutput.Close()
						<-c.done
					}()
				}
				s := &startupCleanupStarter{child: child, ordinary: ordinaryChildren, entered: make(chan struct{}), release: make(chan struct{})}
				m.starter = s
				w.spec.controlled = s
				failedMode := mode == "identity" || mode == "identity-close-failure" || mode == "history"
				if mode == "identity" || mode == "identity-close-failure" {
					m.transientIdentityRandom = startupIdentityError{}
				}
				if mode == "history" {
					m.readinessHistoryRandom = startupIdentityError{}
				}
				if mode == "identity-close-failure" {
					child.closeFailure = true
				}
				caller, cancel := context.WithCancel(ctx)
				defer cancel()
				returned := make(chan StartResult, 1)
				go func() { returned <- m.Start(caller, req) }()
				<-s.entered
				var ordinaryRecord *runtimeSession
				id := req.Profile.SessionKey().String()
				if mode == "collision" {
					st := m.Start(context.Background(), req)
					if st.Failure != "" {
						t.Fatalf("ordinary collision setup: %+v", st)
					}
					m.mu.Lock()
					ordinaryRecord = m.sessions[id]
					m.mu.Unlock()
				}
				if mode == "capacity" {
					// Fill actual ordinary admissions while the private Starter is delayed.
					for _, domain := range []string{"capacity-a", "capacity-b"} {
						selector, err := runtimeprofile.Validate(runtimeprofile.Selector{TrustDomain: domain, Workspace: req.Profile.Workspace().String(), Profile: "go", EnvironmentReference: "local"})
						if err != nil {
							t.Fatal(err)
						}
						other := req
						other.Profile = runtimeprofile.Resolve(selector)
						if st := m.Start(context.Background(), other); st.Failure != "" {
							t.Fatalf("ordinary capacity setup: %+v", st)
						}
					}
				}
				if mode == "algebra" {
					for _, key := range []string{"algebra-a", "algebra-b"} {
						if got := m.algebra.Admit(key, 0); got.Kind != session.AdmissionFree {
							t.Fatalf("algebra fixture: %+v", got)
						}
					}
				}
				close(s.release)
				<-child.entered
				// Always release/join before reporting errors: RED cannot strand Manager.mu.
				inspected := make(chan bool, 1)
				go func() {
					_ = m.Census()
					m.mu.Lock()
					r := m.sessions[id]
					ok := m.workers > 0
					if mode == "collision" {
						ok = ok && r == ordinaryRecord && r.process == ordinary && r.record.State == session.Initializing
					}
					if failedMode {
						ok = ok && r != nil && r.process == w.child && r.record.State != session.Ready && r.protocolOwned
					}
					m.mu.Unlock()
					inspected <- ok
				}()
				responsive := false
				valid := false
				select {
				case valid = <-inspected:
					responsive = true
				case <-time.After(250 * time.Millisecond):
				}
				if responsive && failedMode {
					got := m.Start(context.Background(), req)
					valid = valid && got.Failure != ""
					ready := m.BeginReadiness(context.Background(), id, 1, time.Now().Add(time.Second))
					valid = valid && ready.Failure != ""
					stop := m.Stop(context.Background(), id, "competing-cleanup")
					valid = valid && stop.Failure != "" && stop.IntentID == ""
				}
				cancel()
				callerResult := <-returned
				w.mu.Lock()
				tracked := w.active == 1 && w.child != nil && !w.released
				w.mu.Unlock()
				close(child.release)
				if !responsive {
					valid = <-inspected
				}
				// The actual start callback, not caller cancellation, publishes active=0.
				w.mu.Lock()
				for w.active != 0 {
					changed := w.changed
					w.mu.Unlock()
					<-changed
					w.mu.Lock()
				}
				actual := w.start
				w.mu.Unlock()
				if !responsive {
					t.Error("ASSERT_STARTUP_CLEANUP_RESPONSIVE: Manager blocked during private backend cleanup")
				} else {
					t.Log("ASSERT_STARTUP_CLEANUP_RESPONSIVE: PASS")
				}
				if !tracked || callerResult.Failure == "" {
					t.Errorf("ASSERT_STARTUP_CLEANUP_RETAINED: tracked=%t caller=%+v", tracked, callerResult)
				} else {
					t.Log("ASSERT_STARTUP_CLEANUP_RETAINED: PASS")
				}
				if responsive && !valid {
					t.Error("ASSERT_STARTUP_CLEANUP_RESERVED: failed key was not exclusively nonready")
				} else if responsive {
					t.Log("ASSERT_STARTUP_CLEANUP_RESERVED: PASS")
				}
				m.mu.Lock()
				r := m.sessions[id]
				committed := true
				if mode == "collision" {
					committed = r == ordinaryRecord && r.process == ordinary
				}
				if mode == "identity" || mode == "history" {
					committed = r == nil && actual.Failure != ""
				}
				if mode == "identity-close-failure" {
					committed = r != nil && r.record.State == session.Poisoned && r.process == w.child && actual.Failure != ""
				}
				m.mu.Unlock()
				if !committed {
					t.Error("ASSERT_STARTUP_CLEANUP_EXACT_COMMIT: wrong retained record")
				} else {
					t.Log("ASSERT_STARTUP_CLEANUP_EXACT_COMMIT: PASS")
				}
				if mode == "identity-close-failure" {
					// Observe the actual finisher's terminal decision, not an early snapshot.
					<-w.finishDone
					m.mu.Lock()
					workerRetained := m.workers > 0
					m.mu.Unlock()
					w.mu.Lock()
					retained := !w.released && workerRetained && !w.completion.Completed
					w.mu.Unlock()
					if !retained {
						t.Error("ASSERT_STARTUP_CLEANUP_FAILURE_RETAINED: released failed cleanup")
					}
				}
			})
		}
	}
}

var _ io.Reader = startupIdentityError{}
