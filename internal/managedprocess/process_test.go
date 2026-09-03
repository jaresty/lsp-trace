package managedprocess

import (
	"context"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"strings"
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
	case "survive":
		signal.Ignore(os.Interrupt)
		for {
			time.Sleep(time.Second)
		}
	}
}
