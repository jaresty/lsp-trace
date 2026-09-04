//go:build darwin

package sessionruntime

import (
	"context"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"lsp-trace/internal/managedprocess"
)

const localDarwinReadinessBudget = 5 * time.Second

func TestLocalDarwinReadinessBudget(t *testing.T) {
	const want = 5 * time.Second
	if localDarwinReadinessBudget != want {
		t.Fatalf("ASSERT_SESSIONRUNTIME_LOCAL_DARWIN_READINESS_BUDGET: got=%s want=%s", localDarwinReadinessBudget, want)
	}
}

func TestExplicitLocalDarwinSupervisorRunsFakeLSP(t *testing.T) {
	supervisor, err := managedprocess.NewLocalDarwinSupervisor(managedprocess.Options{StderrLimit: 4096, GracePeriod: 100 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	manager, err := New(Config{
		Limits:           Limits{MaxSessions: 1, MaxRequests: 2, MaxChildren: 1, MaxCancels: 2, MaxTombstones: 2, MaxObservations: 16},
		Starter:          ManagedStarter{Manager: supervisor},
		ReadinessTimeout: localDarwinReadinessBudget,
	})
	if err != nil {
		t.Fatal(err)
	}
	root, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	binary := filepath.Join(t.TempDir(), "fake-lsp")
	build := exec.Command("go", "build", "-o", binary, "./cmd/fake-lsp")
	build.Dir = root
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build fake-lsp: %v: %s", err, output)
	}
	started := manager.Start(context.Background(), StartRequest{
		Profile: profile(t),
		Process: managedprocess.Spec{Path: binary, Dir: root},
	})
	if started.Failure != "" || started.Start.Evidence != managedprocess.LocalDarwinSupervisionOnly {
		t.Fatalf("ASSERT_SESSIONRUNTIME_LOCAL_DARWIN_FAKE_LSP: %+v", started)
	}
	pending := manager.BeginReadiness(context.Background(), started.SessionID, started.Generation, time.Time{})
	if pending.State != ReadinessPending {
		t.Fatalf("ASSERT_SESSIONRUNTIME_LOCAL_DARWIN_READINESS_PENDING: %+v", pending)
	}
	readinessContext, cancelReadiness := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancelReadiness()
	ready, found := manager.WaitReadiness(readinessContext, pending.ID)
	if !found || ready.State != ReadinessReady || ready.Failure != "" {
		t.Fatalf("ASSERT_SESSIONRUNTIME_LOCAL_DARWIN_READINESS_READY: found=%v snapshot=%+v", found, ready)
	}
	again, found := manager.Readiness(pending.ID)
	if !found || again != ready {
		t.Fatalf("ASSERT_SESSIONRUNTIME_LOCAL_DARWIN_READINESS_IMMUTABLE: found=%v first=%+v again=%+v", found, ready, again)
	}
	if census := manager.Census(); census.Workers != 0 {
		t.Fatalf("ASSERT_SESSIONRUNTIME_LOCAL_DARWIN_READINESS_LEAK_FREE: %+v", census)
	}
	accepted := manager.Stop(context.Background(), started.SessionID, "local-test")
	if accepted.Failure != "" {
		t.Fatalf("ASSERT_SESSIONRUNTIME_LOCAL_DARWIN_STOP_ACCEPTED: %+v", accepted)
	}
	terminal := waitOperation(t, manager, accepted.IntentID, OperationComplete)
	if terminal.Failure != "" {
		t.Fatalf("ASSERT_SESSIONRUNTIME_LOCAL_DARWIN_STOP_COMPLETE: %+v", terminal)
	}
}
