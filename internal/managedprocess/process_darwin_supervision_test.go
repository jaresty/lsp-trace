//go:build darwin

package managedprocess

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestLocalDarwinSupervisionUsesGroupAndBoundedEvidence(t *testing.T) {
	m := newHermeticManager(Options{StderrLimit: 64, GracePeriod: 20 * time.Millisecond})
	p, started := m.Start(context.Background(), helperSpec("survive"))
	if p == nil || started.Kind != StartStarted {
		t.Fatalf("ASSERT_LOCAL_DARWIN_START: process=%v observation=%+v", p, started)
	}
	if started.Evidence != LocalDarwinSupervisionOnly {
		t.Fatalf("ASSERT_LOCAL_DARWIN_LABEL: evidence=%q", started.Evidence)
	}
	// Allow the helper to install its interrupt handler before exercising escalation.
	time.Sleep(50 * time.Millisecond)
	teardown := p.Teardown(context.Background())
	if !teardown.Census.Bounded || teardown.Census.Limit <= 0 {
		t.Fatalf("ASSERT_LOCAL_DARWIN_BOUNDED_CENSUS: %+v", teardown.Census)
	}
	if teardown.Census.ProcessGroupID != p.cmd.Process.Pid {
		t.Fatalf("ASSERT_LOCAL_DARWIN_NEW_PROCESS_GROUP: census=%+v pid=%d", teardown.Census, p.cmd.Process.Pid)
	}
	if teardown.Death.Reap.Kind != ReapComplete || teardown.Death.Evidence != LocalDarwinSupervisionOnly {
		t.Fatalf("ASSERT_LOCAL_DARWIN_BOUNDED_REAP: %+v", teardown.Death)
	}
	if !containsPhase(teardown.Phases, PhaseInterrupt) || !containsPhase(teardown.Phases, PhaseKill) || !containsPhase(teardown.Phases, PhaseReap) {
		t.Fatalf("ASSERT_LOCAL_DARWIN_GROUP_ESCALATION: %+v", teardown)
	}
	closed := p.Close()
	if closed.Kind != ResourcesClosed || closed.Evidence != LocalDarwinSupervisionOnly {
		t.Fatalf("ASSERT_LOCAL_DARWIN_RESOURCE_CLOSURE: %+v", closed)
	}
}

func TestLocalDarwinEvidenceStatesNonContainmentLimitations(t *testing.T) {
	m := newHermeticManager(Options{})
	_, started := m.Start(context.Background(), helperSpec("stderr-exit"))
	for _, required := range []string{"does not prove anti-escape", "owner-death cleanup", "descendant containment", "VERIFIED"} {
		if !strings.Contains(started.Limitations, required) {
			t.Fatalf("ASSERT_LOCAL_DARWIN_NON_CONTAINMENT: missing=%q limitations=%q", required, started.Limitations)
		}
	}
}

func containsPhase(phases []TeardownPhase, want TeardownPhase) bool {
	for _, phase := range phases {
		if phase == want {
			return true
		}
	}
	return false
}
