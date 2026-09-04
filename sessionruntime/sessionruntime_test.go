package sessionruntime

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"reflect"
	"sync"
	"testing"
	"time"

	"lsp-trace/internal/containment"
	"lsp-trace/internal/lspwire"
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

func TestReadinessOperationSurfaceExists(t *testing.T) {
	for _, name := range []string{"BeginReadiness", "Readiness", "WaitReadiness"} {
		if _, ok := reflect.TypeOf((*Manager)(nil)).MethodByName(name); !ok {
			t.Fatalf("ASSERT_READINESS_OPERATION_SURFACE: missing %s", name)
		}
	}
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
	entered chan struct{}
	release chan struct{}
}

func (c *blockingChild) Teardown(context.Context) managedprocess.TeardownObservation {
	if c.entered != nil {
		close(c.entered)
	}
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

func TestLifecycleCapacityRejectionIsAtomicAndRetryable(t *testing.T) {
	childA := &blockingChild{release: make(chan struct{})}
	childB := &blockingChild{entered: make(chan struct{}), release: make(chan struct{})}
	starter := &sequenceStarter{children: []Child{childA, childB}}
	m, err := New(Config{Limits: Limits{MaxSessions: 2, MaxRequests: 1, MaxChildren: 1, MaxCancels: 1, MaxTombstones: 1, MaxObservations: 16, MaxOperations: 2}, Starter: starter})
	if err != nil {
		t.Fatal(err)
	}
	profileA := profile(t)
	validatedB, err := runtimeprofile.Validate(runtimeprofile.Selector{TrustDomain: "test", Workspace: "/workspace-b", Profile: "go", EnvironmentReference: "local"})
	if err != nil {
		t.Fatal(err)
	}
	profileB := runtimeprofile.Resolve(validatedB)
	startedA := m.Start(context.Background(), StartRequest{Profile: profileA})
	startedB := m.Start(context.Background(), StartRequest{Profile: profileB})
	operationA := m.Stop(context.Background(), startedA.SessionID, "caller-a")
	if operationA.IntentID == "" {
		t.Fatal("session A stop omitted operation ID")
	}
	for _, caller := range []string{"caller-b-1", "caller-b-2"} {
		rejected := m.Stop(context.Background(), startedB.SessionID, caller)
		if rejected.Failure != session.ResourceExhausted || rejected.IntentID != "" {
			t.Fatalf("capacity rejection mutated arbitration or exposed phantom operation: caller=%s result=%+v", caller, rejected)
		}
	}
	if c := m.Census(); c.Operations != 1 || c.Workers != 1 {
		t.Fatalf("capacity rejection leaked reservation: %+v", c)
	}
	close(childA.release)
	waitOperation(t, m, operationA.IntentID, OperationComplete)
	retry := m.Stop(context.Background(), startedB.SessionID, "caller-b-1")
	if retry.Failure != "" || retry.IntentID == "" {
		t.Fatalf("ASSERT_CAPACITY_REJECTION_ATOMIC_RETRYABLE: %+v", retry)
	}
	<-childB.entered
	joined := m.Stop(context.Background(), startedB.SessionID, "caller-b-2")
	if joined.Failure != "" || joined.IntentID != retry.IntentID {
		t.Fatalf("ASSERT_DISTINCT_CALLER_JOINS_ACCEPTED_OPERATION: retry=%+v joined=%+v", retry, joined)
	}
	replayed := m.Stop(context.Background(), startedB.SessionID, "caller-b-1")
	if replayed.Failure != "" || replayed.IntentID != retry.IntentID {
		t.Fatalf("ASSERT_SAME_CALLER_REPLAY_ACCEPTED_IDENTITY: retry=%+v replay=%+v", retry, replayed)
	}
	close(childB.release)
	terminal := waitOperation(t, m, retry.IntentID, OperationComplete)
	again, found := m.Operation(retry.IntentID)
	if !found || terminal.ID != retry.IntentID || terminal.SessionID != startedB.SessionID || terminal.CallerID != "caller-b-1" || terminal.Generation != retry.Generation || terminal.Restart || terminal.State != OperationComplete || again != terminal {
		t.Fatalf("ASSERT_TERMINAL_OPERATION_SNAPSHOT_IMMUTABLE: found=%v accepted=%+v first=%+v again=%+v", found, retry, terminal, again)
	}
	if c := m.Census(); c.Workers != 0 || c.Operations > 2 {
		t.Fatalf("terminal operation accounting leaked: %+v", c)
	}
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

type recordingWriteCloser struct {
	io.WriteCloser
	events chan string
}

func (w recordingWriteCloser) Write(p []byte) (int, error) {
	if bytes.Contains(p, []byte(`"method":"shutdown"`)) {
		w.events <- "shutdown"
	}
	if bytes.Contains(p, []byte(`"method":"exit"`)) {
		w.events <- "exit"
	}
	if bytes.Contains(p, []byte(`"method":"$/cancelRequest"`)) {
		w.events <- "$/cancelRequest"
	}
	return w.WriteCloser.Write(p)
}

type protocolTestChild struct {
	input  *io.PipeReader
	stdin  *io.PipeWriter
	output *io.PipeWriter
	stdout *io.PipeReader
	events chan string
	fail   bool
}

func newWireChild(t *testing.T, fail bool) *protocolTestChild {
	t.Helper()
	input, stdin := io.Pipe()
	stdout, output := io.Pipe()
	c := &protocolTestChild{input: input, stdin: stdin, output: output, stdout: stdout, events: make(chan string, 8), fail: fail}
	go func() {
		r := lspwire.NewReader(input, lspwire.DefaultLimits())
		w := lspwire.NewWriter(output, lspwire.DefaultLimits())
		for {
			msg, err := r.Read()
			if err != nil {
				return
			}
			if msg.Method == "shutdown" && !fail {
				_ = w.Write(lspwire.Message{JSONRPC: lspwire.Version, ID: msg.ID, Result: []byte("null")})
			}
			if msg.Method == "exit" {
				return
			}
		}
	}()
	return c
}

func (c *protocolTestChild) Stdin() io.WriteCloser { return recordingWriteCloser{c.stdin, c.events} }
func (c *protocolTestChild) Stdout() io.ReadCloser { return c.stdout }
func (c *protocolTestChild) Teardown(context.Context) managedprocess.TeardownObservation {
	c.events <- "forceful"
	_ = c.stdin.Close()
	_ = c.input.Close()
	_ = c.output.Close()
	return managedprocess.TeardownObservation{Death: managedprocess.DeathObservation{Kind: managedprocess.DeathExited, Reap: managedprocess.ReapObservation{Kind: managedprocess.ReapComplete}}}
}
func (c *protocolTestChild) Close() managedprocess.ResourceObservation {
	_ = c.stdout.Close()
	return managedprocess.ResourceObservation{Kind: managedprocess.ResourcesClosed}
}

type oneChildStarter struct{ child Child }

func (s oneChildStarter) Start(context.Context, managedprocess.Spec) (Child, managedprocess.StartObservation) {
	return s.child, managedprocess.StartObservation{Kind: managedprocess.StartStarted}
}

type readinessChild struct {
	input  *io.PipeReader
	stdin  *io.PipeWriter
	output *io.PipeWriter
	stdout *io.PipeReader
	mode   string
}

func newReadinessChild(mode string) *readinessChild {
	input, stdin := io.Pipe()
	stdout, output := io.Pipe()
	c := &readinessChild{input: input, stdin: stdin, output: output, stdout: stdout, mode: mode}
	go func() {
		message, err := lspwire.NewReader(input, lspwire.DefaultLimits()).Read()
		if err != nil || message.Method != "initialize" {
			return
		}
		switch mode {
		case "ready":
			_ = lspwire.NewWriter(output, lspwire.DefaultLimits()).Write(lspwire.Message{JSONRPC: lspwire.Version, ID: message.ID, Result: json.RawMessage(`{"capabilities":{}}`)})
		case "error":
			_ = lspwire.NewWriter(output, lspwire.DefaultLimits()).Write(lspwire.Message{JSONRPC: lspwire.Version, ID: message.ID, Error: &lspwire.RPCError{Code: -32603, Message: "fixture error"}})
		case "malformed":
			_, _ = io.WriteString(output, "Content-Length: nope\r\n\r\n{}")
		case "death":
			_ = output.Close()
		case "hang":
		}
	}()
	return c
}

func (c *readinessChild) Stdin() io.WriteCloser { return c.stdin }
func (c *readinessChild) Stdout() io.ReadCloser { return c.stdout }
func (c *readinessChild) Teardown(context.Context) managedprocess.TeardownObservation {
	_ = c.stdin.Close()
	_ = c.input.Close()
	_ = c.output.Close()
	return managedprocess.TeardownObservation{Death: managedprocess.DeathObservation{Kind: managedprocess.DeathExited, Reap: managedprocess.ReapObservation{Kind: managedprocess.ReapComplete}}}
}
func (c *readinessChild) Close() managedprocess.ResourceObservation {
	_ = c.stdout.Close()
	return managedprocess.ResourceObservation{Kind: managedprocess.ResourcesClosed}
}

func readinessManager(t *testing.T, child Child) (*Manager, StartResult) {
	t.Helper()
	m, err := New(Config{Limits: Limits{MaxSessions: 1, MaxRequests: 1, MaxChildren: 2, MaxCancels: 1, MaxTombstones: 2, MaxObservations: 32, MaxOperations: 2}, Starter: oneChildStarter{child}})
	if err != nil {
		t.Fatal(err)
	}
	return m, m.Start(context.Background(), StartRequest{Profile: profile(t)})
}

func TestReadinessProtocolOutcomesAreBoundedAndImmutable(t *testing.T) {
	for _, tc := range []struct {
		mode string
		want ReadinessState
		fail session.Failure
	}{{"ready", ReadinessReady, ""}, {"error", ReadinessFailed, session.InitializationFailure}, {"malformed", ReadinessFailed, session.InitializationFailure}, {"death", ReadinessFailed, session.InitializationFailure}} {
		t.Run(tc.mode, func(t *testing.T) {
			m, started := readinessManager(t, newReadinessChild(tc.mode))
			pending := m.BeginReadiness(context.Background(), started.SessionID, started.Generation, time.Now().Add(time.Second))
			if pending.State != ReadinessPending {
				t.Fatalf("ASSERT_READINESS_PENDING: snapshot=%+v", pending)
			}
			terminal, ok := m.WaitReadiness(context.Background(), pending.ID)
			if !ok || terminal.State != tc.want || terminal.Failure != tc.fail {
				t.Fatalf("ASSERT_READINESS_PROTOCOL_TERMINAL_%s: snapshot=%+v found=%v", tc.mode, terminal, ok)
			}
			again, _ := m.Readiness(pending.ID)
			if again != terminal {
				t.Fatalf("ASSERT_READINESS_IMMUTABLE: first=%+v again=%+v", terminal, again)
			}
		})
	}
}

func TestReadinessTimeoutAndCancellationAreTerminal(t *testing.T) {
	for _, tc := range []struct {
		name  string
		begin func(*Manager, StartResult) ReadinessSnapshot
		fail  session.Failure
	}{
		{"timeout", func(m *Manager, s StartResult) ReadinessSnapshot {
			return m.BeginReadiness(context.Background(), s.SessionID, s.Generation, time.Now().Add(10*time.Millisecond))
		}, session.InitializationTimeout},
		{"cancel", func(m *Manager, s StartResult) ReadinessSnapshot {
			ctx, cancel := context.WithCancel(context.Background())
			pending := m.BeginReadiness(ctx, s.SessionID, s.Generation, time.Now().Add(time.Second))
			cancel()
			return pending
		}, session.RequestCancelled},
	} {
		t.Run(tc.name, func(t *testing.T) {
			child := newReadinessChild("hang")
			m, started := readinessManager(t, child)
			pending := tc.begin(m, started)
			terminal, _ := m.WaitReadiness(context.Background(), pending.ID)
			if terminal.State != ReadinessFailed || terminal.Failure != tc.fail || m.Census().Workers != 0 {
				t.Fatalf("ASSERT_READINESS_BOUNDED_%s: snapshot=%+v census=%+v", tc.name, terminal, m.Census())
			}
			_ = child.Teardown(context.Background())
			_ = child.Close()
		})
	}
}

func TestReadinessPendingComposesWithLifecycle(t *testing.T) {
	for _, tc := range []struct {
		name string
		call func(*Manager, string) session.LifecycleResult
	}{
		{"stop", func(m *Manager, id string) session.LifecycleResult { return m.Stop(context.Background(), id, "caller") }},
		{"restart", func(m *Manager, id string) session.LifecycleResult {
			return m.Restart(context.Background(), id, "caller")
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			child := newReadinessChild("hang")
			m, started := readinessManager(t, child)
			ctx, cancel := context.WithCancel(context.Background())
			pending := m.BeginReadiness(ctx, started.SessionID, started.Generation, time.Now().Add(time.Second))
			lifecycle := tc.call(m, started.SessionID)
			if lifecycle.Failure != session.LifecycleConflict || lifecycle.IntentID != "" {
				t.Fatalf("ASSERT_READINESS_LIFECYCLE_REJECTS_STREAM_OVERLAP: operation=%+v", lifecycle)
			}
			stillPending, _ := m.Readiness(pending.ID)
			if stillPending.State != ReadinessPending {
				t.Fatalf("ASSERT_READINESS_LIFECYCLE_NO_STALE_READY: readiness=%+v", stillPending)
			}
			cancel()
			terminal, _ := m.WaitReadiness(context.Background(), pending.ID)
			if terminal.State != ReadinessFailed || terminal.Failure != session.RequestCancelled || m.Census().Workers != 0 {
				t.Fatalf("ASSERT_READINESS_LIFECYCLE_LEAK_FREE: readiness=%+v census=%+v", terminal, m.Census())
			}
			_ = child.Teardown(context.Background())
			_ = child.Close()
		})
	}
}

func TestRuntimeCancellationWritesExactlyOnceToActiveChild(t *testing.T) {
	child := newWireChild(t, false)
	m, _ := New(Config{Limits: Limits{MaxSessions: 1, MaxRequests: 1, MaxChildren: 1, MaxCancels: 2, MaxTombstones: 2, MaxObservations: 16}, Starter: oneChildStarter{child}})
	started := m.Start(context.Background(), StartRequest{Profile: profile(t)})
	key, failure := m.BeginRequest(started.SessionID, started.Generation, time.Time{})
	if failure != "" {
		t.Fatal(failure)
	}
	api, ok := any(m).(interface {
		CancelRequest(string, lspwire.RequestKey) (lspwire.CancelState, error)
	})
	if !ok {
		t.Fatal("ASSERT_RUNTIME_CANCEL_ACTIVE_WIRE: manager cancellation seam absent")
	}
	first, err := api.CancelRequest(started.SessionID, key)
	second, secondErr := api.CancelRequest(started.SessionID, key)
	if err != nil || secondErr != nil || first != lspwire.CancelWritten || second != lspwire.CancelAlreadyWritten {
		t.Fatalf("ASSERT_RUNTIME_CANCEL_EXACTLY_ONCE: first=%v second=%v err=%v secondErr=%v", first, second, err, secondErr)
	}
	if got := <-child.events; got != "$/cancelRequest" {
		t.Fatalf("ASSERT_RUNTIME_CANCEL_ACTIVE_WIRE: method=%q", got)
	}
	select {
	case got := <-child.events:
		t.Fatalf("ASSERT_RUNTIME_CANCEL_EXACTLY_ONCE: duplicate=%q", got)
	case <-time.After(20 * time.Millisecond):
	}
}

func TestGracefulShutdownExitOrderAndHonestFallback(t *testing.T) {
	for _, tc := range []struct {
		name string
		fail bool
		want OperationState
	}{{"success", false, OperationComplete}, {"failure", true, OperationFailed}} {
		t.Run(tc.name, func(t *testing.T) {
			child := newWireChild(t, tc.fail)
			m, _ := New(Config{Limits: Limits{MaxSessions: 1, MaxRequests: 1, MaxChildren: 1, MaxCancels: 1, MaxTombstones: 2, MaxObservations: 16}, Starter: oneChildStarter{child}})
			started := m.Start(context.Background(), StartRequest{Profile: profile(t)})
			accepted := m.Stop(context.Background(), started.SessionID, "caller")
			terminal := waitOperation(t, m, accepted.IntentID, tc.want)
			var events []string
			for len(child.events) > 0 {
				events = append(events, <-child.events)
			}
			joined := bytes.Join(func() [][]byte {
				out := make([][]byte, len(events))
				for i, v := range events {
					out[i] = []byte(v)
				}
				return out
			}(), []byte(","))
			if tc.fail {
				if !bytes.Contains(joined, []byte("shutdown,forceful")) || terminal.Failure == "" {
					t.Fatalf("ASSERT_GRACEFUL_FALLBACK_HONEST: events=%v terminal=%+v", events, terminal)
				}
			} else if !bytes.Contains(joined, []byte("shutdown,exit,forceful")) {
				t.Fatalf("ASSERT_GRACEFUL_SHUTDOWN_EXIT_ORDER: events=%v", events)
			}
		})
	}
}

func TestConfirmedStopReleasesCapacityForDistinctSession(t *testing.T) {
	starter := &sequenceStarter{children: []Child{referenceChild{}, referenceChild{}}}
	m, _ := New(Config{Limits: Limits{MaxSessions: 1, MaxRequests: 1, MaxChildren: 1, MaxCancels: 1, MaxTombstones: 1, MaxObservations: 16}, Starter: starter})
	first := m.Start(context.Background(), StartRequest{Profile: profile(t)})
	stop := m.Stop(context.Background(), first.SessionID, "caller")
	waitOperation(t, m, stop.IntentID, OperationComplete)
	validated, _ := runtimeprofile.Validate(runtimeprofile.Selector{TrustDomain: "test", Workspace: "/other", Profile: "go", EnvironmentReference: "local"})
	second := m.Start(context.Background(), StartRequest{Profile: runtimeprofile.Resolve(validated)})
	if second.Failure != "" {
		t.Fatalf("ASSERT_POST_STOP_CAPACITY_RETRY: result=%+v", second)
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
	restarted := m.Records()[0]
	if restarted.Generation != 2 || restarted.State != session.Initializing {
		t.Fatalf("ASSERT_RESTART_GENERATION_INITIALIZING: status=%+v; generation n+1 must not be READY without correlated initialize metadata", restarted)
	}
	if stale := m.ObserveInitialization(first.SessionID, 1, true); stale.Failure != session.StaleGeneration || m.Records()[0].State != session.Initializing {
		t.Fatalf("ASSERT_RESTART_REJECTS_STALE_INITIALIZATION: result=%+v status=%+v", stale, m.Records()[0])
	}
	if initialized := m.ObserveInitialization(first.SessionID, 2, true); initialized.Failure != "" || initialized.State != session.Ready || m.Records()[0].State != session.Ready {
		t.Fatalf("ASSERT_RESTART_EXACT_GENERATION_READY: result=%+v status=%+v", initialized, m.Records()[0])
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
