package lifecycleops

import (
	"context"
	"reflect"
	"sync"
	"testing"
	"time"

	"lsp-trace/internal/managedprocess"
	"lsp-trace/internal/runtimeprofile"
	"lsp-trace/internal/session"
	"lsp-trace/sessionruntime"
)

type fakeRuntime struct {
	mu           sync.Mutex
	records      []sessionruntime.Record
	observations []sessionruntime.Observation
	census       sessionruntime.Census
	operations   map[string]sessionruntime.OperationSnapshot
	result       session.LifecycleResult
	calls        int
	stopIDs      []string
	restartIDs   []string
}

func (f *fakeRuntime) Records() []sessionruntime.Record {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]sessionruntime.Record(nil), f.records...)
}
func (f *fakeRuntime) Observations() []sessionruntime.Observation {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]sessionruntime.Observation(nil), f.observations...)
}
func (f *fakeRuntime) Census() sessionruntime.Census {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.census
}
func (f *fakeRuntime) Operation(id string) (sessionruntime.OperationSnapshot, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	o, ok := f.operations[id]
	return o, ok
}
func (f *fakeRuntime) Stop(_ context.Context, id string, _ string) session.LifecycleResult {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	f.stopIDs = append(f.stopIDs, id)
	return f.result
}
func (f *fakeRuntime) Restart(_ context.Context, id string, _ string) session.LifecycleResult {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	f.restartIDs = append(f.restartIDs, id)
	return f.result
}

type selectorRuntime struct {
	*fakeRuntime
	aliases map[string]string
}

func (f *selectorRuntime) ResolveSessionSelector(id string, generation uint64) (string, uint64, session.Failure) {
	if canonical, ok := f.aliases[id]; ok {
		id = canonical
	} else if id != "canonical" {
		return "", 0, session.SessionNotFound
	}
	if generation != 0 {
		return id, generation, ""
	}
	var match *sessionruntime.Record
	for _, candidate := range f.Records() {
		if candidate.SessionID != id || candidate.State != session.Ready {
			continue
		}
		if match != nil {
			return "", 0, session.Failure("AMBIGUOUS_SESSION_SELECTOR")
		}
		copy := candidate
		match = &copy
	}
	if match == nil {
		return "", 0, session.Failure("SESSION_NOT_READY")
	}
	return match.SessionID, match.Generation, ""
}

func record(id string, generation uint64) sessionruntime.Record {
	return sessionruntime.Record{SessionID: id, Generation: generation, State: session.Ready, Profile: runtimeprofile.Profile{}}
}

func TestListIsImmutableDeterministicAndBounded(t *testing.T) {
	f := &fakeRuntime{records: []sessionruntime.Record{record("z", 1), record("a", 2)}, observations: []sessionruntime.Observation{{Sequence: 2}, {Sequence: 1}}, census: sessionruntime.Census{Sessions: 2, Observations: 2}}
	s := New(f)
	first := s.List()
	if len(first.Sessions) != 2 {
		t.Fatalf("ASSERT list immutable deterministic bounded: count=%d", len(first.Sessions))
	}
	if got := []string{first.Sessions[0].SessionID, first.Sessions[1].SessionID}; !reflect.DeepEqual(got, []string{"a", "z"}) {
		t.Fatalf("ASSERT list immutable deterministic bounded: order=%v", got)
	}
	if first.Observations[0].Sequence != 1 || first.Census != f.census {
		t.Fatalf("ASSERT list immutable deterministic bounded: projection=%+v", first)
	}
	first.Sessions[0].SessionID = "mutated"
	first.Observations[0].Kind = "mutated"
	again := s.List()
	if again.Sessions[0].SessionID != "a" || again.Observations[0].Kind == "mutated" {
		t.Fatalf("ASSERT list immutable deterministic bounded: aliased=%+v", again)
	}
	t.Log("PASS ASSERT list immutable deterministic bounded")
}

func TestStatusValidatesUnknownAndStaleGeneration(t *testing.T) {
	s := New(&fakeRuntime{records: []sessionruntime.Record{record("known", 4)}})
	if _, failure := s.Status("missing", 1); failure != FailureSessionNotFound {
		t.Fatalf("ASSERT status identity generation: unknown=%q", failure)
	}
	if _, failure := s.Status("known", 3); failure != FailureStaleGeneration {
		t.Fatalf("ASSERT status identity generation: stale=%q", failure)
	}
	if got, failure := s.Status("known", 4); failure != FailureNone || got.Generation != 4 {
		t.Fatalf("ASSERT status identity generation: valid=%+v failure=%q", got, failure)
	}
	t.Log("PASS ASSERT status identity generation")
}

func TestLifecycleSelectorStatusAliasSemantics(t *testing.T) {
	f := &selectorRuntime{
		fakeRuntime: &fakeRuntime{records: []sessionruntime.Record{record("canonical", 7)}},
		aliases:     map[string]string{"project": "canonical"},
	}
	s := New(f)

	got, failure := s.Status("project", 0)
	if failure != FailureNone || got.SessionID != "canonical" || got.Generation != 7 {
		t.Fatalf("ASSERT_LIFECYCLE_SELECTOR_STATUS_ALIAS_OMITTED_UNIQUE_READY: got=%+v failure=%q", got, failure)
	}
	t.Log("PASS ASSERT_LIFECYCLE_SELECTOR_STATUS_ALIAS_OMITTED_UNIQUE_READY")
	got, failure = s.Status("project", 7)
	if failure != FailureNone || got.SessionID != "canonical" || got.Generation != 7 {
		t.Fatalf("ASSERT_LIFECYCLE_SELECTOR_STATUS_ALIAS_EXPLICIT_GENERATION_EXACT: got=%+v failure=%q", got, failure)
	}
	t.Log("PASS ASSERT_LIFECYCLE_SELECTOR_STATUS_ALIAS_EXPLICIT_GENERATION_EXACT")
	if _, failure = s.Status("unknown", 0); failure != FailureSessionNotFound {
		t.Fatalf("ASSERT_LIFECYCLE_SELECTOR_UNKNOWN_ALIAS_HONEST_FAILURE: failure=%q", failure)
	}
	t.Log("PASS ASSERT_LIFECYCLE_SELECTOR_UNKNOWN_ALIAS_HONEST_FAILURE")
	got, failure = s.Status("canonical", 7)
	if failure != FailureNone || got.SessionID != "canonical" {
		t.Fatalf("ASSERT_LIFECYCLE_SELECTOR_CANONICAL_SESSION_REACHES_RUNTIME: got=%+v failure=%q", got, failure)
	}
	t.Log("PASS ASSERT_LIFECYCLE_SELECTOR_CANONICAL_SESSION_REACHES_RUNTIME")
}

func TestLifecycleSelectorStopAndRestartUseCanonicalIdentity(t *testing.T) {
	f := &selectorRuntime{
		fakeRuntime: &fakeRuntime{
			records: []sessionruntime.Record{record("canonical", 7)},
			result:  session.LifecycleResult{IntentID: "intent", Generation: 7, State: session.Stopping},
		},
		aliases: map[string]string{"project": "canonical"},
	}
	s := New(f)

	if got := s.Stop(context.Background(), LifecycleRequest{SessionID: "project", CallerID: "caller"}); got.Failure != FailureNone || got.Generation != 7 || !reflect.DeepEqual(f.stopIDs, []string{"canonical"}) {
		t.Fatalf("ASSERT_LIFECYCLE_SELECTOR_STOP_ALIAS_CANONICAL_ID_GENERATION: got=%+v ids=%v", got, f.stopIDs)
	}
	t.Log("PASS ASSERT_LIFECYCLE_SELECTOR_STOP_ALIAS_CANONICAL_ID_GENERATION")
	if got := s.Restart(context.Background(), LifecycleRequest{SessionID: "project", Generation: 7, CallerID: "caller"}); got.Failure != FailureNone || got.Generation != 7 || !reflect.DeepEqual(f.restartIDs, []string{"canonical"}) {
		t.Fatalf("ASSERT_LIFECYCLE_SELECTOR_RESTART_ALIAS_CANONICAL_ID_GENERATION: got=%+v ids=%v", got, f.restartIDs)
	}
	t.Log("PASS ASSERT_LIFECYCLE_SELECTOR_RESTART_ALIAS_CANONICAL_ID_GENERATION")
}

func TestAcceptanceIsBoundedDelegatedAndStable(t *testing.T) {
	f := &fakeRuntime{records: []sessionruntime.Record{record("s", 1)}, result: session.LifecycleResult{IntentID: "intent-1", Generation: 1, State: session.Stopping, Joined: true, Replayed: true}}
	s := New(f)
	deadline := time.Now().Add(100 * time.Millisecond)
	for _, restart := range []bool{false, true} {
		request := LifecycleRequest{SessionID: "s", Generation: 1, CallerID: "caller"}
		var got Acceptance
		if restart {
			got = s.Restart(context.Background(), request)
		} else {
			got = s.Stop(context.Background(), request)
		}
		if got.Failure != FailureNone || got.OperationID != "intent-1" || !got.Pending || !got.Joined || !got.Replayed {
			t.Fatalf("ASSERT acceptance bounded delegated replay join: restart=%v result=%+v", restart, got)
		}
	}
	if time.Now().After(deadline) || f.calls != 2 {
		t.Fatalf("ASSERT acceptance bounded delegated replay join: calls=%d", f.calls)
	}
	t.Log("PASS ASSERT acceptance bounded delegated replay join")
}

func TestAcceptanceValidationAndFailureMappingArePureAndRetryable(t *testing.T) {
	f := &fakeRuntime{records: []sessionruntime.Record{record("s", 2)}, result: session.LifecycleResult{Failure: session.ResourceExhausted}}
	s := New(f)
	if got := s.Stop(context.Background(), LifecycleRequest{SessionID: "missing", Generation: 2}); got.Failure != FailureSessionNotFound || f.calls != 0 {
		t.Fatalf("ASSERT failure mapping pure retryable: unknown=%+v calls=%d", got, f.calls)
	}
	if got := s.Stop(context.Background(), LifecycleRequest{SessionID: "s", Generation: 1}); got.Failure != FailureStaleGeneration || f.calls != 0 {
		t.Fatalf("ASSERT failure mapping pure retryable: stale=%+v calls=%d", got, f.calls)
	}
	for i := 0; i < 2; i++ {
		if got := s.Stop(context.Background(), LifecycleRequest{SessionID: "s", Generation: 2}); got.Failure != FailureCapacityExhausted || got.OperationID != "" {
			t.Fatalf("ASSERT failure mapping pure retryable: capacity=%+v", got)
		}
	}
	f.result = session.LifecycleResult{Failure: session.ProcessContainmentUnavailable}
	if got := s.Stop(context.Background(), LifecycleRequest{SessionID: "s", Generation: 2}); got.Failure != FailureContainmentUnavailable || got.OperationID != "" {
		t.Fatalf("ASSERT failure mapping pure retryable: containment=%+v", got)
	}
	t.Log("PASS ASSERT failure mapping pure retryable")
}

func TestOperationStatusProjectsPendingCompleteFailedAndUnknown(t *testing.T) {
	f := &fakeRuntime{operations: map[string]sessionruntime.OperationSnapshot{
		"p": {ID: "p", State: sessionruntime.OperationPending},
		"c": {ID: "c", State: sessionruntime.OperationComplete},
		"f": {ID: "f", State: sessionruntime.OperationFailed, Failure: session.SessionReapIncomplete},
	}}
	s := New(f)
	for id, want := range map[string]OperationState{"p": Pending, "c": Complete, "f": Failed} {
		got, failure := s.OperationStatus(id)
		if failure != FailureNone || got.State != want {
			t.Fatalf("ASSERT operation status projection: id=%s got=%+v failure=%q", id, got, failure)
		}
	}
	failed, _ := s.OperationStatus("f")
	if failed.Failure != FailureReapIncomplete {
		t.Fatalf("ASSERT operation status projection: failed mapping=%q", failed.Failure)
	}
	if _, failure := s.OperationStatus("missing"); failure != FailureOperationNotFound {
		t.Fatalf("ASSERT operation status projection: unknown=%q", failure)
	}
	t.Log("PASS ASSERT operation status projection")
}

func TestConcurrentCallersOnlyConsumeRuntimeSnapshots(t *testing.T) {
	f := &fakeRuntime{records: []sessionruntime.Record{record("s", 1)}, observations: []sessionruntime.Observation{{Sequence: 1}}, operations: map[string]sessionruntime.OperationSnapshot{"p": {ID: "p", State: sessionruntime.OperationPending}}, result: session.LifecycleResult{IntentID: "p", Generation: 1}}
	s := New(f)
	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = s.List()
			_, _ = s.Status("s", 1)
			_ = s.Stop(context.Background(), LifecycleRequest{SessionID: "s", Generation: 1, CallerID: "caller"})
			_, _ = s.OperationStatus("p")
		}()
	}
	wg.Wait()
	if f.calls != 32 {
		t.Fatalf("ASSERT concurrent snapshot race freedom: calls=%d", f.calls)
	}
	t.Log("PASS ASSERT concurrent snapshot race freedom")
}

type liveChild struct{ release chan struct{} }

func (c *liveChild) Teardown(context.Context) managedprocess.TeardownObservation {
	<-c.release
	return managedprocess.TeardownObservation{Death: managedprocess.DeathObservation{Kind: managedprocess.DeathExited, Reap: managedprocess.ReapObservation{Kind: managedprocess.ReapComplete}}}
}
func (*liveChild) Close() managedprocess.ResourceObservation {
	return managedprocess.ResourceObservation{Kind: managedprocess.ResourcesClosed}
}

type liveStarter struct{ child *liveChild }

func (s liveStarter) Start(context.Context, managedprocess.Spec) (sessionruntime.Child, managedprocess.StartObservation) {
	return s.child, managedprocess.StartObservation{Kind: managedprocess.StartStarted}
}

func TestLiveRuntimePendingAndTerminalProjection(t *testing.T) {
	child := &liveChild{release: make(chan struct{})}
	runtime, err := sessionruntime.New(sessionruntime.Config{Limits: sessionruntime.Limits{MaxSessions: 1, MaxRequests: 1, MaxChildren: 1, MaxCancels: 1, MaxTombstones: 1, MaxObservations: 8, MaxOperations: 1}, Starter: liveStarter{child: child}})
	if err != nil {
		t.Fatal(err)
	}
	validated, err := runtimeprofile.Validate(runtimeprofile.Selector{TrustDomain: "test", Workspace: "/workspace", Profile: "go", EnvironmentReference: "local"})
	if err != nil {
		t.Fatal(err)
	}
	started := runtime.Start(context.Background(), sessionruntime.StartRequest{Profile: runtimeprofile.Resolve(validated)})
	service := New(runtime)
	accepted := service.Stop(context.Background(), LifecycleRequest{SessionID: started.SessionID, Generation: started.Generation, CallerID: "caller"})
	pending, failure := service.OperationStatus(accepted.OperationID)
	if failure != FailureNone || pending.State != Pending {
		t.Fatalf("ASSERT live pending terminal runtime projection: pending=%+v failure=%q", pending, failure)
	}
	close(child.release)
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		terminal, terminalFailure := service.OperationStatus(accepted.OperationID)
		if terminalFailure == FailureNone && terminal.State == Complete {
			t.Log("PASS ASSERT live pending terminal runtime projection")
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("ASSERT live pending terminal runtime projection: did not complete")
}
