// Package integratedconformance contains disabled, hermetic reference tests only.
// It owns no production registration, advertisement, containment, or Stage 2 authority.
package integratedconformance

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"lsp-trace/internal/containment"
	"lsp-trace/internal/lspwire"
	"lsp-trace/internal/managedprocess"
	"lsp-trace/internal/mcp"
	"lsp-trace/internal/operation"
	"lsp-trace/internal/runtimeprofile"
	"lsp-trace/internal/session"
	"lsp-trace/lifecycleops"
	"lsp-trace/sessionruntime"
)

const (
	evidenceCeiling       = "DISABLED_HERMETIC_REFERENCE_ONLY"
	localDarwinEvidence   = "LOCAL_DARWIN_SUPERVISION_ONLY"
	productionUnavailable = "UNAVAILABLE"
)

var exactGaps = []string{
	"Production-equivalent fake-process startup is not composable: the sealed production containment gate is unavailable and the hermetic starter has no production authority.",
}

type processChild struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser
	stderr *bounded
	done   chan struct{}
	death  managedprocess.DeathObservation
	once   sync.Once
}

type bounded struct {
	mu        sync.Mutex
	limit     int
	bytes     []byte
	truncated bool
}

func (b *bounded) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	n := len(p)
	remaining := b.limit - len(b.bytes)
	if remaining > 0 {
		keep := n
		if keep > remaining {
			keep = remaining
		}
		b.bytes = append(b.bytes, p[:keep]...)
	}
	if n > remaining {
		b.truncated = true
	}
	return n, nil
}

func (b *bounded) snapshot() managedprocess.StderrObservation {
	b.mu.Lock()
	defer b.mu.Unlock()
	return managedprocess.StderrObservation{Bytes: append([]byte(nil), b.bytes...), Truncated: b.truncated}
}

type execStarter struct {
	path        string
	stderrLimit int
	mu          sync.Mutex
	children    []*processChild
}

func (s *execStarter) Start(ctx context.Context, _ managedprocess.Spec) (sessionruntime.Child, managedprocess.StartObservation) {
	cmd := exec.CommandContext(ctx, s.path)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, managedprocess.StartObservation{Kind: managedprocess.StartFailed, Err: err}
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		_ = stdin.Close()
		return nil, managedprocess.StartObservation{Kind: managedprocess.StartFailed, Err: err}
	}
	stderr := &bounded{limit: s.stderrLimit}
	cmd.Stderr = stderr
	if err := cmd.Start(); err != nil {
		_ = stdin.Close()
		_ = stdout.Close()
		return nil, managedprocess.StartObservation{Kind: managedprocess.StartFailed, Err: err}
	}
	child := &processChild{cmd: cmd, stdin: stdin, stdout: stdout, stderr: stderr, done: make(chan struct{})}
	go func() {
		err := cmd.Wait()
		child.death = managedprocess.DeathObservation{Kind: managedprocess.DeathExited, ExitCode: cmd.ProcessState.ExitCode(), Err: err, Stderr: stderr.snapshot(), Reap: managedprocess.ReapObservation{Kind: managedprocess.ReapComplete, Evidence: evidenceCeiling}, Evidence: evidenceCeiling}
		close(child.done)
	}()
	s.mu.Lock()
	s.children = append(s.children, child)
	s.mu.Unlock()
	return child, managedprocess.StartObservation{Kind: managedprocess.StartStarted, Evidence: evidenceCeiling}
}

func (p *processChild) Stdin() io.WriteCloser { return p.stdin }
func (p *processChild) Stdout() io.ReadCloser { return p.stdout }

func (p *processChild) Teardown(ctx context.Context) managedprocess.TeardownObservation {
	phases := []managedprocess.TeardownPhase{managedprocess.PhaseInterrupt}
	select {
	case <-p.done:
		return managedprocess.TeardownObservation{Phases: append(phases, managedprocess.PhaseReap), Death: p.death}
	default:
	}
	_ = p.cmd.Process.Signal(os.Interrupt)
	phases = append(phases, managedprocess.PhaseWait)
	select {
	case <-p.done:
	case <-ctx.Done():
		phases = append(phases, managedprocess.PhaseKill)
		_ = p.cmd.Process.Kill()
		<-p.done
	case <-time.After(300 * time.Millisecond):
		phases = append(phases, managedprocess.PhaseKill)
		_ = p.cmd.Process.Kill()
		<-p.done
	}
	return managedprocess.TeardownObservation{Phases: append(phases, managedprocess.PhaseReap), Death: p.death}
}

func (p *processChild) Close() managedprocess.ResourceObservation {
	p.once.Do(func() { _ = p.stdin.Close(); _ = p.stdout.Close() })
	return managedprocess.ResourceObservation{Kind: managedprocess.ResourcesClosed, Evidence: evidenceCeiling}
}

func (p *processChild) wait() managedprocess.DeathObservation { <-p.done; return p.death }

func repositoryRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller path unavailable")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}

func buildFake(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "fake-lsp")
	cmd := exec.Command("go", "build", "-o", path, "./cmd/fake-lsp")
	cmd.Dir = repositoryRoot(t)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build fake LSP: %v: %s", err, out)
	}
	return path
}

func profile(t *testing.T, workspace string) runtimeprofile.Profile {
	t.Helper()
	validated, err := runtimeprofile.Validate(runtimeprofile.Selector{TrustDomain: "integrated-test", Workspace: workspace, Profile: "fake-lsp", EnvironmentReference: "hermetic"})
	if err != nil {
		t.Fatal(err)
	}
	return runtimeprofile.Resolve(validated)
}

func limits(maxSessions, maxRequests, maxChildren int) sessionruntime.Limits {
	return sessionruntime.Limits{MaxSessions: maxSessions, MaxRequests: maxRequests, MaxChildren: maxChildren, MaxCancels: 4, MaxTombstones: 4, MaxObservations: 64, MaxOperations: 4}
}

func writeMessage(t *testing.T, w io.Writer, m lspwire.Message) {
	t.Helper()
	if err := lspwire.NewWriter(w, lspwire.DefaultLimits()).Write(m); err != nil {
		t.Fatal(err)
	}
}

func rejectPerturbation(t *testing.T, assertion string) {
	t.Helper()
	if os.Getenv("LSP_TRACE_CONFORMANCE_PERTURB") == assertion {
		t.Fatalf("%s: FAIL injected minimal wrong state", assertion)
	}
}

func TestDisabledIntegratedConformance(t *testing.T) {
	fake := buildFake(t)

	t.Run("ASSERT_LOCAL_DARWIN_START_CORRELATED_READY", func(t *testing.T) {
		rejectPerturbation(t, "ASSERT_LOCAL_DARWIN_START_CORRELATED_READY")
		if runtime.GOOS != "darwin" {
			t.Skip("LOCAL_DARWIN_SUPERVISION_ONLY")
		}
		supervisor, err := managedprocess.NewLocalDarwinSupervisor(managedprocess.Options{StderrLimit: 4096, GracePeriod: 100 * time.Millisecond})
		if err != nil {
			t.Fatal(err)
		}
		manager, err := sessionruntime.New(sessionruntime.Config{Limits: limits(1, 2, 1), Starter: sessionruntime.ManagedStarter{Manager: supervisor}, ReadinessTimeout: 5 * time.Second})
		if err != nil {
			t.Fatal(err)
		}
		started := manager.Start(context.Background(), sessionruntime.StartRequest{Profile: profile(t, t.TempDir()), Process: managedprocess.Spec{Path: fake, Dir: repositoryRoot(t)}})
		if started.Failure != "" || started.State != session.Initializing || started.Generation != 1 || started.Start.Evidence != localDarwinEvidence {
			t.Fatalf("ASSERT_LOCAL_DARWIN_START_CORRELATED_READY: start=%+v", started)
		}
		pending := manager.BeginReadiness(context.Background(), started.SessionID, started.Generation, time.Now().Add(5*time.Second))
		if pending.State != sessionruntime.ReadinessPending || pending.SessionID != started.SessionID || pending.Generation != started.Generation {
			t.Fatalf("ASSERT_LOCAL_DARWIN_START_CORRELATED_READY: pending=%+v start=%+v", pending, started)
		}
		waitContext, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		ready, found := manager.WaitReadiness(waitContext, pending.ID)
		if !found || ready.ID != pending.ID || ready.SessionID != started.SessionID || ready.Generation != started.Generation || ready.State != sessionruntime.ReadinessReady || ready.Failure != "" {
			t.Fatalf("ASSERT_LOCAL_DARWIN_START_CORRELATED_READY: found=%v ready=%+v pending=%+v", found, ready, pending)
		}
		if census := manager.Census(); census.Workers != 0 {
			t.Fatalf("ASSERT_LOCAL_DARWIN_START_CORRELATED_READY: census=%+v", census)
		}
		accepted := manager.Stop(context.Background(), started.SessionID, "darwin-readiness-cleanup")
		if accepted.Failure != "" {
			t.Fatalf("ASSERT_LOCAL_DARWIN_START_CORRELATED_READY: cleanup=%+v", accepted)
		}
		shutdownContext, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()
		if err := manager.Shutdown(shutdownContext); err != nil {
			t.Fatalf("ASSERT_LOCAL_DARWIN_START_CORRELATED_READY: shutdown=%v", err)
		}
		t.Log("PASS ASSERT_LOCAL_DARWIN_START_CORRELATED_READY")
	})

	t.Run("ASSERT_DISABLED_DISPATCH_LIFECYCLE_TERMINAL_GENERATION", func(t *testing.T) {
		rejectPerturbation(t, "ASSERT_DISABLED_DISPATCH_LIFECYCLE_TERMINAL_GENERATION")
		starter := &cleanStarter{}
		manager, err := sessionruntime.New(sessionruntime.Config{Limits: limits(1, 2, 1), Starter: starter})
		if err != nil {
			t.Fatal(err)
		}
		started := manager.Start(context.Background(), sessionruntime.StartRequest{Profile: profile(t, t.TempDir())})
		if started.Failure != "" {
			t.Fatal(started.Failure)
		}
		manager.ObserveInitialization(started.SessionID, started.Generation, true)
		executor := lifecycleops.NewExecutor(lifecycleops.New(manager))
		status := executeLifecycle(t, executor, lifecycleops.OperationStatus, started.SessionID, 1, "")
		if record, ok := status.Value.(sessionruntime.Record); !ok || record.State != session.Ready || record.Generation != 1 {
			t.Fatalf("ASSERT_DISABLED_DISPATCH_LIFECYCLE_TERMINAL_GENERATION: status=%+v", status.Value)
		}
		restartResult := executeLifecycle(t, executor, lifecycleops.OperationRestart, started.SessionID, 1, "restart")
		restartAcceptance, ok := restartResult.Value.(lifecycleops.Acceptance)
		if !ok || !restartAcceptance.Pending {
			t.Fatalf("ASSERT_DISABLED_DISPATCH_LIFECYCLE_TERMINAL_GENERATION: restart=%+v", restartResult.Value)
		}
		restartTerminal := waitLifecycleTerminal(t, lifecycleops.New(manager), restartAcceptance.OperationID)
		if restartTerminal.State != lifecycleops.Complete || !restartTerminal.Restart || restartTerminal.Generation != 1 {
			t.Fatalf("ASSERT_DISABLED_DISPATCH_LIFECYCLE_TERMINAL_GENERATION: restart terminal=%+v", restartTerminal)
		}
		current, failure := lifecycleops.New(manager).Status(started.SessionID, 2)
		if failure != "" || current.Generation != 2 || current.State != session.Ready {
			t.Fatalf("ASSERT_DISABLED_DISPATCH_LIFECYCLE_TERMINAL_GENERATION: current=%+v failure=%s", current, failure)
		}
		_, staleFailure := executor.Execute(context.Background(), operation.Request{Name: lifecycleops.OperationStatus, RequestID: "status-stale", Input: json.RawMessage(`{"session_id":"` + started.SessionID + `","generation":1}`)})
		if staleFailure == nil || staleFailure.Code != string(lifecycleops.FailureStaleGeneration) {
			t.Fatalf("ASSERT_DISABLED_DISPATCH_LIFECYCLE_TERMINAL_GENERATION: stale=%+v", staleFailure)
		}
		stopResult := executeLifecycle(t, executor, lifecycleops.OperationStop, started.SessionID, 2, "stop")
		stopAcceptance, ok := stopResult.Value.(lifecycleops.Acceptance)
		if !ok || !stopAcceptance.Pending {
			t.Fatalf("ASSERT_DISABLED_DISPATCH_LIFECYCLE_TERMINAL_GENERATION: stop=%+v", stopResult.Value)
		}
		stopTerminal := waitLifecycleTerminal(t, lifecycleops.New(manager), stopAcceptance.OperationID)
		if stopTerminal.State != lifecycleops.Complete || stopTerminal.Restart || stopTerminal.Generation != 2 {
			t.Fatalf("ASSERT_DISABLED_DISPATCH_LIFECYCLE_TERMINAL_GENERATION: stop terminal=%+v", stopTerminal)
		}
		if census := manager.Census(); census.Sessions != 0 || census.Children != 0 || census.Workers != 0 {
			t.Fatalf("ASSERT_DISABLED_DISPATCH_LIFECYCLE_TERMINAL_GENERATION: census=%+v", census)
		}
		t.Log("PASS ASSERT_DISABLED_DISPATCH_LIFECYCLE_TERMINAL_GENERATION")
	})

	t.Run("ASSERT_READINESS_FAULT_TERMINALS_CLEANUP", func(t *testing.T) {
		rejectPerturbation(t, "ASSERT_READINESS_FAULT_TERMINALS_CLEANUP")
		for _, tc := range []readinessCase{
			{name: "timeout", mode: "hang", fail: session.InitializationTimeout},
			{name: "cancellation", mode: "hang", fail: session.RequestCancelled},
			{name: "EOF", mode: "eof", fail: session.InitializationFailure},
		} {
			t.Run(tc.name, func(t *testing.T) {
				child := newReadinessFixture(tc.mode)
				manager, err := sessionruntime.New(sessionruntime.Config{Limits: limits(1, 1, 1), Starter: &fixedStarter{child: child}, ReadinessTimeout: 20 * time.Millisecond})
				if err != nil {
					t.Fatal(err)
				}
				started := manager.Start(context.Background(), sessionruntime.StartRequest{Profile: profile(t, t.TempDir())})
				readinessContext := context.Background()
				deadline := time.Time{}
				if tc.name == "cancellation" {
					var cancel context.CancelFunc
					readinessContext, cancel = context.WithCancel(context.Background())
					defer cancel()
					pending := manager.BeginReadiness(readinessContext, started.SessionID, started.Generation, time.Now().Add(time.Second))
					cancel()
					terminal, _ := manager.WaitReadiness(context.Background(), pending.ID)
					assertReadinessFaultCleanup(t, tc, terminal, manager, child)
					return
				}
				if tc.name == "timeout" {
					deadline = time.Now().Add(20 * time.Millisecond)
				}
				pending := manager.BeginReadiness(readinessContext, started.SessionID, started.Generation, deadline)
				terminal, _ := manager.WaitReadiness(context.Background(), pending.ID)
				assertReadinessFaultCleanup(t, tc, terminal, manager, child)
			})
		}

		crashed := startDirect(t, fake, 32)
		writeMessage(t, crashed.stdin, lspwire.Message{JSONRPC: lspwire.Version, Method: "fixture/crash", Params: json.RawMessage(`{}`)})
		death := crashed.wait()
		resources := crashed.Close()
		if death.ExitCode != 86 || death.Reap.Kind != managedprocess.ReapComplete || resources.Kind != managedprocess.ResourcesClosed {
			t.Fatalf("ASSERT_READINESS_FAULT_TERMINALS_CLEANUP: crash death=%+v resources=%+v", death, resources)
		}
		t.Log("PASS ASSERT_READINESS_FAULT_TERMINALS_CLEANUP")
	})

	t.Run("ASSERT_INTEGRATED_FAKE_LSP_FRAMING", func(t *testing.T) {
		rejectPerturbation(t, "ASSERT_INTEGRATED_FAKE_LSP_FRAMING")
		starter := &execStarter{path: fake, stderrLimit: 64}
		manager, err := sessionruntime.New(sessionruntime.Config{Limits: limits(1, 2, 1), Starter: starter})
		if err != nil {
			t.Fatal(err)
		}
		started := manager.Start(context.Background(), sessionruntime.StartRequest{Profile: profile(t, t.TempDir())})
		if started.State != session.Initializing || started.Generation != 1 {
			t.Fatalf("start=%+v", started)
		}
		child := starter.children[0]
		key, failure := manager.BeginRequest(started.SessionID, 1, time.Time{})
		if failure != "" {
			t.Fatal(failure)
		}
		request, _ := lspwire.InitializeRequest(key, lspwire.InitializeConfig{ProcessID: 42, RootURI: "file:///workspace", ClientName: "integrated-harness", ClientVersion: "1", Trace: "off"})
		writeMessage(t, child.stdin, request)
		response, err := lspwire.NewReader(child.stdout, lspwire.DefaultLimits()).Read()
		if err != nil || response.Kind() != lspwire.KindSuccessResponse || string(response.ID) != "1" {
			t.Fatalf("response=%+v err=%v", response, err)
		}
		if got := manager.CompleteResponse(started.SessionID, key); got != lspwire.ResponseAccepted {
			t.Fatalf("disposition=%v", got)
		}
		if initialized := manager.ObserveInitialization(started.SessionID, 1, true); initialized.State != session.Ready {
			t.Fatalf("initialized=%+v", initialized)
		}
		t.Log("PASS ASSERT_INTEGRATED_FAKE_LSP_FRAMING")
	})

	t.Run("ASSERT_DETERMINISTIC_INITIALIZATION_BYTES", func(t *testing.T) {
		rejectPerturbation(t, "ASSERT_DETERMINISTIC_INITIALIZATION_BYTES")
		key := lspwire.RequestKey{Generation: 1, ID: 7}
		cfg := lspwire.InitializeConfig{ProcessID: 42, RootURI: "file:///workspace", ClientName: "integrated-harness", ClientVersion: "1", Trace: "off"}
		messageA, _ := lspwire.InitializeRequest(key, cfg)
		messageB, _ := lspwire.InitializeRequest(key, cfg)
		var a, b bytes.Buffer
		writeMessage(t, &a, messageA)
		writeMessage(t, &b, messageB)
		if !bytes.Equal(a.Bytes(), b.Bytes()) {
			t.Fatalf("frames differ:\n%x\n%x", a.Bytes(), b.Bytes())
		}
		const expected = `{"jsonrpc":"2.0","id":7,"method":"initialize","params":{"processId":42,"clientInfo":{"name":"integrated-harness","version":"1"},"rootUri":"file:///workspace","capabilities":{},"trace":"off"}}`
		if !bytes.HasSuffix(a.Bytes(), []byte(expected)) {
			t.Fatalf("unexpected frame %q", a.Bytes())
		}
		t.Log("PASS ASSERT_DETERMINISTIC_INITIALIZATION_BYTES")
	})

	t.Run("ASSERT_LIFECYCLE_PENDING_COMPLETE_FAILED", func(t *testing.T) {
		rejectPerturbation(t, "ASSERT_LIFECYCLE_PENDING_COMPLETE_FAILED")
		starter := &execStarter{path: fake, stderrLimit: 64}
		manager, _ := sessionruntime.New(sessionruntime.Config{Limits: limits(2, 1, 2), Starter: starter})
		started := manager.Start(context.Background(), sessionruntime.StartRequest{Profile: profile(t, filepath.Join(t.TempDir(), "complete"))})
		manager.ObserveInitialization(started.SessionID, 1, true)
		service := lifecycleops.New(manager)
		accepted := service.Stop(context.Background(), lifecycleops.LifecycleRequest{SessionID: started.SessionID, Generation: 1, CallerID: "complete"})
		pending, failure := service.OperationStatus(accepted.OperationID)
		if failure != "" || pending.State != lifecycleops.Pending {
			t.Fatalf("pending=%+v failure=%s", pending, failure)
		}
		deadline := time.Now().Add(3 * time.Second)
		for pending.State == lifecycleops.Pending && time.Now().Before(deadline) {
			time.Sleep(time.Millisecond)
			pending, _ = service.OperationStatus(accepted.OperationID)
		}
		if pending.State != lifecycleops.Complete {
			t.Fatalf("complete=%+v", pending)
		}

		failedStarter := &fixedStarter{child: failingChild{}}
		failedManager, _ := sessionruntime.New(sessionruntime.Config{Limits: limits(1, 1, 1), Starter: failedStarter})
		failedStart := failedManager.Start(context.Background(), sessionruntime.StartRequest{Profile: profile(t, filepath.Join(t.TempDir(), "failed"))})
		failedManager.ObserveInitialization(failedStart.SessionID, 1, true)
		failedService := lifecycleops.New(failedManager)
		failedAcceptance := failedService.Stop(context.Background(), lifecycleops.LifecycleRequest{SessionID: failedStart.SessionID, Generation: 1, CallerID: "failed"})
		var failed lifecycleops.OperationSnapshot
		for deadline := time.Now().Add(time.Second); time.Now().Before(deadline); {
			failed, _ = failedService.OperationStatus(failedAcceptance.OperationID)
			if failed.State != lifecycleops.Pending {
				break
			}
			time.Sleep(time.Millisecond)
		}
		if failed.State != lifecycleops.Failed || failed.Failure != lifecycleops.FailureReapIncomplete {
			t.Fatalf("failed=%+v", failed)
		}
		t.Log("PASS ASSERT_LIFECYCLE_PENDING_COMPLETE_FAILED")
	})

	t.Run("ASSERT_RUNTIME_CANCELLATION_CROSSES_SESSION_WIRE", func(t *testing.T) {
		rejectPerturbation(t, "ASSERT_RUNTIME_CANCELLATION_CROSSES_SESSION_WIRE")
		starter := &execStarter{path: fake, stderrLimit: 32}
		manager, _ := sessionruntime.New(sessionruntime.Config{Limits: limits(1, 2, 1), Starter: starter})
		started := manager.Start(context.Background(), sessionruntime.StartRequest{Profile: profile(t, t.TempDir())})
		child := starter.children[0]
		key, failure := manager.BeginRequest(started.SessionID, started.Generation, time.Time{})
		if failure != "" {
			t.Fatal(failure)
		}
		writeMessage(t, child.stdin, lspwire.Message{JSONRPC: lspwire.Version, ID: json.RawMessage(`1`), Method: "fixture/hang", Params: json.RawMessage(`{}`)})
		first, err := manager.CancelRequest(started.SessionID, key)
		second, secondErr := manager.CancelRequest(started.SessionID, key)
		if err != nil || secondErr != nil || first != lspwire.CancelWritten || second != lspwire.CancelAlreadyWritten {
			t.Fatalf("ASSERT_RUNTIME_CANCELLATION_CROSSES_SESSION_WIRE: first=%v second=%v err=%v secondErr=%v", first, second, err, secondErr)
		}
		observed, err := lspwire.NewReader(child.stdout, lspwire.DefaultLimits()).Read()
		if err != nil || observed.Method != "fixture/cancelObserved" {
			t.Fatalf("cancel observation=%+v err=%v", observed, err)
		}
		writeMessage(t, child.stdin, lspwire.Message{JSONRPC: lspwire.Version, Method: "fixture/lateReply", Params: json.RawMessage(`{"id":1,"result":{"late":true}}`)})
		late, err := lspwire.NewReader(child.stdout, lspwire.DefaultLimits()).Read()
		if err != nil || manager.CompleteResponse(started.SessionID, lspwire.ResponseKey{Generation: 1, ID: 1}) != lspwire.ResponseAccepted || manager.CompleteResponse(started.SessionID, lspwire.ResponseKey{Generation: 1, ID: 1}) != lspwire.ResponseDuplicate || string(late.ID) != "1" {
			t.Fatalf("late=%+v err=%v", late, err)
		}
		child.Teardown(context.Background())
		child.Close()

		malformed := startDirect(t, fake, 32)
		writeMessage(t, malformed.stdin, lspwire.Message{JSONRPC: lspwire.Version, ID: json.RawMessage(`2`), Method: "fixture/malformed", Params: json.RawMessage(`{}`)})
		if _, err := lspwire.NewReader(malformed.stdout, lspwire.DefaultLimits()).Read(); !errors.Is(err, lspwire.ErrInvalidContentLength) {
			t.Fatalf("malformed err=%v", err)
		}
		malformed.Teardown(context.Background())
		malformed.Close()

		crashed := startDirect(t, fake, 32)
		writeMessage(t, crashed.stdin, lspwire.Message{JSONRPC: lspwire.Version, Method: "fixture/stderr", Params: json.RawMessage(`{"text":"abcdefghijklmnopqrstuvwxyz0123456789"}`)})
		writeMessage(t, crashed.stdin, lspwire.Message{JSONRPC: lspwire.Version, Method: "fixture/crash", Params: json.RawMessage(`{}`)})
		death := crashed.wait()
		if death.ExitCode != 86 || len(death.Stderr.Bytes) != 32 || !death.Stderr.Truncated {
			t.Fatalf("death=%+v", death)
		}
		crashed.Close()
		t.Log("PASS ASSERT_RUNTIME_CANCELLATION_CROSSES_SESSION_WIRE")
	})

	t.Run("ASSERT_RESTART_CAPACITY_SHUTDOWN_ORDER", func(t *testing.T) {
		rejectPerturbation(t, "ASSERT_RESTART_CAPACITY_SHUTDOWN_ORDER")
		starter := &execStarter{path: fake, stderrLimit: 64}
		manager, _ := sessionruntime.New(sessionruntime.Config{Limits: limits(1, 1, 1), Starter: starter})
		first := manager.Start(context.Background(), sessionruntime.StartRequest{Profile: profile(t, filepath.Join(t.TempDir(), "one"))})
		manager.ObserveInitialization(first.SessionID, 1, true)
		pressure := manager.Start(context.Background(), sessionruntime.StartRequest{Profile: profile(t, filepath.Join(t.TempDir(), "two"))})
		if pressure.Failure != session.ResourceExhausted {
			t.Fatalf("pressure=%+v", pressure)
		}
		restart := lifecycleops.New(manager).Restart(context.Background(), lifecycleops.LifecycleRequest{SessionID: first.SessionID, Generation: 1, CallerID: "restart"})
		var terminal lifecycleops.OperationSnapshot
		for deadline := time.Now().Add(3 * time.Second); time.Now().Before(deadline); {
			terminal, _ = lifecycleops.New(manager).OperationStatus(restart.OperationID)
			if terminal.State != lifecycleops.Pending {
				break
			}
			time.Sleep(time.Millisecond)
		}
		if terminal.State != lifecycleops.Complete || terminal.Generation != 1 {
			t.Fatalf("restart terminal=%+v", terminal)
		}
		observations := manager.Observations()
		teardown, startup := -1, -1
		for i, observation := range observations {
			if observation.Kind == "teardown" {
				teardown = i
			}
			if observation.Kind == "startup" && observation.Generation == 2 {
				startup = i
			}
		}
		if teardown < 0 || startup <= teardown {
			t.Fatalf("restart ordering=%+v", observations)
		}
		stop := lifecycleops.New(manager).Stop(context.Background(), lifecycleops.LifecycleRequest{SessionID: first.SessionID, Generation: 2, CallerID: "stop"})
		for deadline := time.Now().Add(3 * time.Second); time.Now().Before(deadline); {
			terminal, _ = lifecycleops.New(manager).OperationStatus(stop.OperationID)
			if terminal.State != lifecycleops.Pending {
				break
			}
			time.Sleep(time.Millisecond)
		}
		if terminal.State != lifecycleops.Complete {
			t.Fatalf("stop terminal=%+v", terminal)
		}
		if err := manager.Shutdown(context.Background()); err != nil {
			t.Fatal(err)
		}
		t.Log("PASS ASSERT_RESTART_CAPACITY_SHUTDOWN_ORDER")
	})

	t.Run("ASSERT_GRACEFUL_SHUTDOWN_EXIT_BEFORE_FORCE", func(t *testing.T) {
		rejectPerturbation(t, "ASSERT_GRACEFUL_SHUTDOWN_EXIT_BEFORE_FORCE")
		starter := &execStarter{path: fake, stderrLimit: 64}
		manager, _ := sessionruntime.New(sessionruntime.Config{Limits: limits(1, 1, 1), Starter: starter})
		started := manager.Start(context.Background(), sessionruntime.StartRequest{Profile: profile(t, t.TempDir())})
		accepted := manager.Stop(context.Background(), started.SessionID, "graceful")
		deadline := time.Now().Add(3 * time.Second)
		for time.Now().Before(deadline) {
			if terminal, ok := manager.Operation(accepted.IntentID); ok && terminal.State != sessionruntime.OperationPending {
				if terminal.State != sessionruntime.OperationComplete {
					t.Fatalf("ASSERT_GRACEFUL_SHUTDOWN_EXIT_BEFORE_FORCE: terminal=%+v", terminal)
				}
				t.Log("PASS ASSERT_GRACEFUL_SHUTDOWN_EXIT_BEFORE_FORCE")
				return
			}
			time.Sleep(time.Millisecond)
		}
		t.Fatal("ASSERT_GRACEFUL_SHUTDOWN_EXIT_BEFORE_FORCE: timeout")
	})

	t.Run("ASSERT_POST_STOP_NEW_SESSION_CAPACITY_RETRY", func(t *testing.T) {
		rejectPerturbation(t, "ASSERT_POST_STOP_NEW_SESSION_CAPACITY_RETRY")
		starter := &execStarter{path: fake, stderrLimit: 64}
		manager, _ := sessionruntime.New(sessionruntime.Config{Limits: limits(1, 1, 1), Starter: starter})
		first := manager.Start(context.Background(), sessionruntime.StartRequest{Profile: profile(t, filepath.Join(t.TempDir(), "one"))})
		accepted := manager.Stop(context.Background(), first.SessionID, "stop")
		deadline := time.Now().Add(3 * time.Second)
		for time.Now().Before(deadline) {
			if terminal, ok := manager.Operation(accepted.IntentID); ok && terminal.State != sessionruntime.OperationPending {
				break
			}
			time.Sleep(time.Millisecond)
		}
		second := manager.Start(context.Background(), sessionruntime.StartRequest{Profile: profile(t, filepath.Join(t.TempDir(), "two"))})
		if second.Failure != "" {
			t.Fatalf("ASSERT_POST_STOP_NEW_SESSION_CAPACITY_RETRY: result=%+v", second)
		}
		t.Log("PASS ASSERT_POST_STOP_NEW_SESSION_CAPACITY_RETRY")
	})

	t.Run("ASSERT_DISABLED_LIFECYCLE_DIRECT_DISPATCH", func(t *testing.T) {
		rejectPerturbation(t, "ASSERT_DISABLED_LIFECYCLE_DIRECT_DISPATCH")
		for _, gap := range exactGaps {
			if strings.Contains(gap, "MCP lifecycle dispatch") {
				t.Fatalf("disabled direct lifecycle dispatch remains recorded as a gap: %s", gap)
			}
		}
		manager, _ := sessionruntime.New(sessionruntime.Config{Limits: limits(1, 1, 1), Starter: &fixedStarter{child: failingChild{}}})
		executor := lifecycleops.NewExecutor(lifecycleops.New(manager))
		result, failure := executor.Execute(context.Background(), operation.Request{Name: lifecycleops.OperationList, RequestID: "disabled-direct-1", Input: json.RawMessage(`{}`)})
		if failure != nil {
			t.Fatalf("direct dispatch failure=%v", failure)
		}
		if _, ok := result.Value.(lifecycleops.ListSnapshot); !ok {
			t.Fatalf("direct dispatch result=%T", result.Value)
		}
		t.Log("PASS ASSERT_DISABLED_LIFECYCLE_DIRECT_DISPATCH")
	})

	t.Run("ASSERT_EXACT_CROSS_SEAM_GAPS", func(t *testing.T) {
		rejectPerturbation(t, "ASSERT_EXACT_CROSS_SEAM_GAPS")
		if len(exactGaps) != 1 {
			t.Fatalf("gaps=%v", exactGaps)
		}
		for _, required := range []string{"Production-equivalent fake-process startup"} {
			found := false
			for _, gap := range exactGaps {
				found = found || strings.Contains(gap, required)
			}
			if !found {
				t.Fatalf("missing exact gap %q", required)
			}
		}
		t.Log("PASS ASSERT_EXACT_CROSS_SEAM_GAPS")
	})

	t.Run("ASSERT_PACKAGE_OWNERSHIP_ONLY", func(t *testing.T) {
		rejectPerturbation(t, "ASSERT_PACKAGE_OWNERSHIP_ONLY")
		root := repositoryRoot(t)
		cmd := exec.Command("git", "status", "--porcelain=v1", "--untracked-files=all")
		cmd.Dir = root
		out, err := cmd.Output()
		if err != nil {
			t.Fatal(err)
		}
		for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
			if line == "" {
				continue
			}
			path := strings.TrimSpace(line[2:])
			owned := path == "sessionruntime/sessionruntime.go" || path == "sessionruntime/sessionruntime_test.go" ||
				path == "internal/session/manager.go" || path == "internal/session/manager_test.go" ||
				path == "cmd/fake-lsp/main.go" || path == "lifecycleops/executor.go" || path == "lifecycleops/executor_test.go" ||
				path == "README.md" || path == "scripts/check-docs.sh" || path == "docs/adr/0003-always-local-stage2.md" || path == "docs/adr/0003-persistent-mcp-language-server-sessions.md" ||
				path == "cmd/lsp-trace/SKILL.md" || path == "cmd/lsp-trace-mcp/main.go" || path == "cmd/lsp-trace-mcp/main_test.go" || path == "cmd/lsp-trace-mcp/process_integration_test.go" ||
				path == "internal/mcp/registry.go" || path == "internal/mcp/registry_test.go" || path == "internal/mcp/transport.go" || path == "internal/mcp/transport_test.go" || strings.HasPrefix(path, "internal/integratedconformance/")
			if !owned {
				t.Fatalf("unowned path %q", path)
			}
		}
		t.Log("PASS ASSERT_PACKAGE_OWNERSHIP_ONLY")
	})

	t.Run("ASSERT_REFERENCE_NOT_PRODUCTION_AUTHORITY", func(t *testing.T) {
		rejectPerturbation(t, "ASSERT_REFERENCE_NOT_PRODUCTION_AUTHORITY")
		if evidenceCeiling != "DISABLED_HERMETIC_REFERENCE_ONLY" || localDarwinEvidence != "LOCAL_DARWIN_SUPERVISION_ONLY" || productionUnavailable != "UNAVAILABLE" {
			t.Fatalf("evidence labels: reference=%q darwin=%q production=%q", evidenceCeiling, localDarwinEvidence, productionUnavailable)
		}
		if reflect.TypeOf(execStarter{}).AssignableTo(reflect.TypeOf(containment.RuntimeGate{})) {
			t.Fatal("hermetic starter acquired production authority")
		}
		t.Log("PASS ASSERT_REFERENCE_NOT_PRODUCTION_AUTHORITY")
	})

	t.Run("ASSERT_ALWAYS_LOCAL_TEN_WITH_UNSUPPORTED_START_ZERO_EFFECTS", func(t *testing.T) {
		rejectPerturbation(t, "ASSERT_ALWAYS_LOCAL_TEN_WITH_UNSUPPORTED_START_ZERO_EFFECTS")
		registry := mcp.NewRegistry(true)
		if got := len(registry.Advertised()); got != 10 {
			t.Fatalf("advertised=%d", got)
		}
		for _, name := range []string{"lsp_session_v1_list", "lsp_session_v1_status", "lsp_session_v1_restart", "lsp_session_v1_stop"} {
			tool, ok := registry.Resolve(name)
			if !ok || tool.Availability != mcp.Enabled {
				t.Fatalf("enabled lifecycle %s=%+v ok=%v", name, tool, ok)
			}
		}
		starter := sessionruntime.ManagedStarter{Manager: managedprocess.New(containment.NewRuntimeGate(), managedprocess.Options{})}
		manager, _ := sessionruntime.New(sessionruntime.Config{Limits: limits(1, 1, 1), Starter: starter})
		before := manager.Census()
		result := manager.Start(context.Background(), sessionruntime.StartRequest{Profile: profile(t, t.TempDir()), Process: managedprocess.Spec{Path: fake}})
		after := manager.Census()
		if result.Failure != session.ProcessContainmentUnavailable || before != after || after != (sessionruntime.Census{}) {
			t.Fatalf("result=%+v before=%+v after=%+v", result, before, after)
		}
		t.Log("PASS ASSERT_ALWAYS_LOCAL_TEN_WITH_UNSUPPORTED_START_ZERO_EFFECTS")
	})
}

func executeLifecycle(t *testing.T, executor *lifecycleops.Executor, name operation.Name, sessionID string, generation uint64, callerID string) operation.Result {
	t.Helper()
	input := map[string]any{"session_id": sessionID, "generation": generation}
	if callerID != "" {
		input["caller_id"] = callerID
	}
	raw, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	result, failure := executor.Execute(context.Background(), operation.Request{Name: name, RequestID: string(name), Input: raw})
	if failure != nil {
		t.Fatalf("%s direct dispatch: %+v", name, failure)
	}
	return result
}

func waitLifecycleTerminal(t *testing.T, service *lifecycleops.Service, operationID string) lifecycleops.OperationSnapshot {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		snapshot, failure := service.OperationStatus(operationID)
		if failure != "" {
			t.Fatalf("operation %s: %s", operationID, failure)
		}
		if snapshot.State != lifecycleops.Pending {
			return snapshot
		}
		runtime.Gosched()
	}
	t.Fatalf("operation %s remained pending", operationID)
	return lifecycleops.OperationSnapshot{}
}

type readinessCase struct {
	name string
	mode string
	fail session.Failure
}

func assertReadinessFaultCleanup(t *testing.T, tc readinessCase, terminal sessionruntime.ReadinessSnapshot, manager *sessionruntime.Manager, child *readinessFixture) {
	t.Helper()
	child.mu.Lock()
	teardowns, closes := child.teardowns, child.closes
	child.mu.Unlock()
	if terminal.State != sessionruntime.ReadinessFailed || terminal.Failure != tc.fail || manager.Census().Workers != 0 || teardowns != 1 || closes != 1 {
		t.Fatalf("ASSERT_READINESS_FAULT_TERMINALS_CLEANUP: case=%s terminal=%+v census=%+v teardown=%d close=%d", tc.name, terminal, manager.Census(), teardowns, closes)
	}
}

type readinessFixture struct {
	input             *io.PipeReader
	stdin             *io.PipeWriter
	output            *io.PipeWriter
	stdout            *io.PipeReader
	mode              string
	mu                sync.Mutex
	teardowns, closes int
}

func newReadinessFixture(mode string) *readinessFixture {
	input, stdin := io.Pipe()
	stdout, output := io.Pipe()
	child := &readinessFixture{input: input, stdin: stdin, output: output, stdout: stdout, mode: mode}
	go func() {
		message, err := lspwire.NewReader(input, lspwire.DefaultLimits()).Read()
		if err != nil || message.Method != "initialize" {
			return
		}
		if mode == "eof" {
			_ = output.Close()
		}
	}()
	return child
}

func (c *readinessFixture) Stdin() io.WriteCloser { return c.stdin }
func (c *readinessFixture) Stdout() io.ReadCloser { return c.stdout }
func (c *readinessFixture) Teardown(context.Context) managedprocess.TeardownObservation {
	c.mu.Lock()
	c.teardowns++
	c.mu.Unlock()
	_ = c.stdin.Close()
	_ = c.input.Close()
	_ = c.output.Close()
	return managedprocess.TeardownObservation{Death: managedprocess.DeathObservation{Kind: managedprocess.DeathExited, Reap: managedprocess.ReapObservation{Kind: managedprocess.ReapComplete, Evidence: evidenceCeiling}}}
}
func (c *readinessFixture) Close() managedprocess.ResourceObservation {
	c.mu.Lock()
	c.closes++
	c.mu.Unlock()
	_ = c.stdout.Close()
	return managedprocess.ResourceObservation{Kind: managedprocess.ResourcesClosed, Evidence: evidenceCeiling}
}

type cleanStarter struct{}

func (*cleanStarter) Start(context.Context, managedprocess.Spec) (sessionruntime.Child, managedprocess.StartObservation) {
	return cleanChild{}, managedprocess.StartObservation{Kind: managedprocess.StartStarted, Evidence: evidenceCeiling}
}

type cleanChild struct{}

func (cleanChild) Teardown(context.Context) managedprocess.TeardownObservation {
	return managedprocess.TeardownObservation{Death: managedprocess.DeathObservation{Kind: managedprocess.DeathExited, Reap: managedprocess.ReapObservation{Kind: managedprocess.ReapComplete, Evidence: evidenceCeiling}}}
}
func (cleanChild) Close() managedprocess.ResourceObservation {
	return managedprocess.ResourceObservation{Kind: managedprocess.ResourcesClosed, Evidence: evidenceCeiling}
}

type fixedStarter struct{ child sessionruntime.Child }

func (s *fixedStarter) Start(context.Context, managedprocess.Spec) (sessionruntime.Child, managedprocess.StartObservation) {
	return s.child, managedprocess.StartObservation{Kind: managedprocess.StartStarted, Evidence: evidenceCeiling}
}

type failingChild struct{}

func (failingChild) Teardown(context.Context) managedprocess.TeardownObservation {
	return managedprocess.TeardownObservation{Death: managedprocess.DeathObservation{Reap: managedprocess.ReapObservation{Kind: managedprocess.ReapFailed}}}
}
func (failingChild) Close() managedprocess.ResourceObservation {
	return managedprocess.ResourceObservation{Kind: managedprocess.ResourcesClosed}
}

func startDirect(t *testing.T, path string, stderrLimit int) *processChild {
	t.Helper()
	starter := &execStarter{path: path, stderrLimit: stderrLimit}
	child, observation := starter.Start(context.Background(), managedprocess.Spec{})
	if observation.Kind != managedprocess.StartStarted {
		t.Fatalf("start=%+v", observation)
	}
	return child.(*processChild)
}
