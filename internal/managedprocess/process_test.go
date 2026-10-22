package managedprocess

import (
	"context"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"lsp-trace/internal/containment"
)

func TestUnavailableStartsNothing(t *testing.T) {
	var factoryCalls atomic.Int32
	m := New(containment.NewRuntimeGate(), Options{StderrLimit: 32})
	m.factory = func(context.Context, Spec) (*exec.Cmd, error) {
		factoryCalls.Add(1)
		return exec.Command("definitely-not-run"), nil
	}

	p, observation := m.Start(context.Background(), Spec{Path: "definitely-not-run"})
	if p != nil || observation.Kind != StartUnavailable || factoryCalls.Load() != 0 {
		t.Fatalf("ASSERT_P1_UNAVAILABLE_ZERO_EFFECTS: process=%v observation=%+v factory_calls=%d", p, observation, factoryCalls.Load())
	}
}

func TestBoundedStderrAndTypedDeathReapClosure(t *testing.T) {
	m := newHermeticManager(Options{StderrLimit: 5})
	p, started := m.Start(context.Background(), helperSpec("stderr-exit"))
	if p == nil || started.Kind != StartStarted {
		t.Fatalf("ASSERT_P3_TYPED_START: process=%v observation=%+v", p, started)
	}

	death := p.Wait()
	if death.Kind != DeathExited || death.ExitCode != 7 {
		t.Fatalf("ASSERT_P3_TYPED_DEATH: %+v", death)
	}
	if string(death.Stderr.Bytes) != "abcde" || !death.Stderr.Truncated {
		t.Fatalf("ASSERT_P2_BOUNDED_STDERR: %+v", death.Stderr)
	}
	if death.Reap.Kind != ReapComplete {
		t.Fatalf("ASSERT_P3_TYPED_REAP: %+v", death.Reap)
	}
	first := p.Close()
	second := p.Close()
	if first.Kind != ResourcesClosed || second != first {
		t.Fatalf("ASSERT_P5_DETERMINISTIC_CLOSURE: first=%+v second=%+v", first, second)
	}
}

type blockingWriter struct {
	started chan struct{}
	release chan struct{}
	once    sync.Once
	bytes   []byte
}

func (w *blockingWriter) Write(p []byte) (int, error) {
	w.once.Do(func() {
		close(w.started)
		<-w.release
	})
	w.bytes = append(w.bytes, p...)
	return len(p), nil
}

type countingWriter struct {
	calls atomic.Int32
}

func (w *countingWriter) Write(p []byte) (int, error) {
	w.calls.Add(1)
	return len(p), nil
}

func TestStartFailureCleansUpBothStdoutModes(t *testing.T) {
	for _, tc := range []struct {
		name        string
		start       func(*Manager, io.Writer) (*Process, StartObservation)
		writerCalls int32
	}{
		{
			name: "legacy pipe",
			start: func(m *Manager, _ io.Writer) (*Process, StartObservation) {
				return m.Start(context.Background(), Spec{})
			},
		},
		{
			name: "provided writer",
			start: func(m *Manager, stdout io.Writer) (*Process, StartObservation) {
				return m.StartWithStdout(context.Background(), Spec{}, stdout)
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			writer := &countingWriter{}
			m := newHermeticManager(Options{StderrLimit: 16})
			var cmd *exec.Cmd
			m.factory = func(context.Context, Spec) (*exec.Cmd, error) {
				cmd = exec.Command("/managedprocess/path-that-does-not-exist")
				return cmd, nil
			}

			p, observation := tc.start(m, writer)
			if p != nil || observation.Kind != StartFailed || observation.Reason != "process start failed" || observation.Err == nil {
				t.Fatalf("ASSERT_PRESTART_TYPED_FAILURE: process=%v observation=%+v", p, observation)
			}
			if cmd == nil || cmd.Process != nil {
				t.Fatalf("ASSERT_PRESTART_NO_PROCESS_LEAK: command=%v", cmd)
			}
			if calls := writer.calls.Load(); calls != tc.writerCalls {
				t.Fatalf("ASSERT_PRESTART_WRITER_NOT_CALLED: calls=%d want=%d", calls, tc.writerCalls)
			}
		})
	}
}

func TestWaitJoinsStdoutDelivery(t *testing.T) {
	writer := &blockingWriter{started: make(chan struct{}), release: make(chan struct{})}
	m := newHermeticManager(Options{StderrLimit: 16})
	spec := helperSpec("stdout-sentinel")
	spec.Env = append(os.Environ(), "GORACE=atexit_sleep_ms=0")
	p, started := m.StartWithStdout(context.Background(), spec, writer)
	if p == nil || started.Kind != StartStarted {
		t.Fatalf("ASSERT_STDOUT_DELIVERY_STARTED: process=%v observation=%+v", p, started)
	}
	<-writer.started
	if observed := p.Observe(); observed.Kind != SurvivorRunning {
		t.Fatalf("ASSERT_STDOUT_DELIVERY_RUNNING_WHILE_BLOCKED: %+v", observed)
	}
	waitDone := make(chan DeathObservation, 1)
	go func() { waitDone <- p.Wait() }()
	close(writer.release)
	var death DeathObservation
	select {
	case death = <-waitDone:
	case <-time.After(time.Second):
		t.Fatal("ASSERT_STDOUT_DELIVERY_BOUNDED_COMPLETION")
	}
	if death.Kind != DeathExited || death.ExitCode != 0 || death.StdoutDelivery.Err != nil || string(writer.bytes) != "stdout-sentinel" {
		t.Fatalf("ASSERT_STDOUT_DELIVERY_EXACT_ONCE: death=%+v bytes=%q", death, writer.bytes)
	}
}

func TestTypedSurvivorAndPhasedTeardown(t *testing.T) {
	m := newHermeticManager(Options{StderrLimit: 16, GracePeriod: 20 * time.Millisecond})
	p, started := m.Start(context.Background(), helperSpec("survive"))
	if p == nil || started.Kind != StartStarted {
		t.Fatalf("ASSERT_P3_TYPED_START: process=%v observation=%+v", p, started)
	}

	if survivor := p.Observe(); survivor.Kind != SurvivorRunning {
		t.Fatalf("ASSERT_P3_TYPED_SURVIVOR: %+v", survivor)
	}
	teardown := p.Teardown(context.Background())
	if len(teardown.Phases) < 2 || teardown.Phases[0] != PhaseInterrupt || teardown.Phases[len(teardown.Phases)-1] != PhaseReap {
		t.Fatalf("ASSERT_P4_PHASED_TEARDOWN: %+v", teardown)
	}
	if teardown.Death.Reap.Kind != ReapComplete {
		t.Fatalf("ASSERT_P4_PHASED_TEARDOWN_REAP: %+v", teardown)
	}
}

func TestNoNativeOrReadinessClaims(t *testing.T) {
	claims := []string{
		StartObservation{}.Evidence,
		DeathObservation{}.Evidence,
		SurvivorObservation{}.Evidence,
		ReapObservation{}.Evidence,
		ResourceObservation{}.Evidence,
	}
	for _, claim := range claims {
		lower := strings.ToLower(claim)
		if strings.Contains(lower, "native containment supported") || strings.Contains(lower, "stage 2 ready") {
			t.Fatalf("ASSERT_P6_EVIDENCE_CEILING: %q", claim)
		}
	}
}

func helperSpec(mode string) Spec {
	return Spec{Path: "__managedprocess_helper__", Args: []string{"-test.run=TestManagedProcessHelper", "--", mode}}
}

func TestManagedProcessHelper(t *testing.T) {
	var args []string
	for i, arg := range os.Args {
		if arg == "--" && i+1 < len(os.Args) {
			args = os.Args[i+1:]
			break
		}
	}
	if len(args) == 0 {
		return
	}
	mode := args[0]
	switch mode {
	case "stderr-exit":
		_, _ = io.WriteString(os.Stderr, "abcdefghij")
		os.Exit(7)
	case "stdout-sentinel":
		_, _ = io.WriteString(os.Stdout, "stdout-sentinel")
		os.Exit(0)
	case "survive":
		signal.Ignore(os.Interrupt)
		for {
			time.Sleep(time.Second)
		}
	}
}
