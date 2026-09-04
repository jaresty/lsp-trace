//go:build darwin

package sessionruntime

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"lsp-trace/internal/managedprocess"
)

const localDarwinCleanupMargin = 5 * time.Second

func localDarwinTestDeadline(t *testing.T) time.Time {
	t.Helper()
	deadline, ok := t.Deadline()
	if !ok {
		t.Fatal("ASSERT_SESSIONRUNTIME_LOCAL_DARWIN_TEST_DEADLINE: go test deadline unavailable")
	}
	deadline = deadline.Add(-localDarwinCleanupMargin)
	if !time.Now().Before(deadline) {
		t.Fatalf("ASSERT_SESSIONRUNTIME_LOCAL_DARWIN_TEST_DEADLINE: cleanup margin exhausted: %s", deadline)
	}
	return deadline
}

func waitForFakeLSPScheduled(t *testing.T, marker string, deadline time.Time) {
	t.Helper()
	for {
		contents, err := os.ReadFile(marker)
		if err == nil {
			if strings.TrimSpace(string(contents)) != "scheduled" {
				t.Fatalf("ASSERT_SESSIONRUNTIME_LOCAL_DARWIN_CHILD_SCHEDULED: marker=%q", contents)
			}
			return
		}
		if !os.IsNotExist(err) {
			t.Fatalf("ASSERT_SESSIONRUNTIME_LOCAL_DARWIN_CHILD_SCHEDULED: marker read: %v", err)
		}
		if !time.Now().Before(deadline) {
			t.Fatal("ASSERT_SESSIONRUNTIME_LOCAL_DARWIN_CHILD_SCHEDULED: test deadline exhausted")
		}
		time.Sleep(time.Millisecond)
	}
}

func TestFakeLSPSchedulingMarker(t *testing.T) {
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
	marker := filepath.Join(t.TempDir(), "scheduled")
	cmd := exec.Command(binary)
	cmd.Env = append(os.Environ(), "LSP_TRACE_FAKE_LSP_SCHEDULED="+marker)
	if err := cmd.Run(); err != nil {
		t.Fatalf("run fake-lsp: %v", err)
	}
	contents, err := os.ReadFile(marker)
	if err != nil || strings.TrimSpace(string(contents)) != "scheduled" {
		t.Fatalf("ASSERT_SESSIONRUNTIME_LOCAL_DARWIN_CHILD_SCHEDULED: marker=%q err=%v", contents, err)
	}
}

func TestExplicitLocalDarwinSupervisorRunsFakeLSP(t *testing.T) {
	supervisor, err := managedprocess.NewLocalDarwinSupervisor(managedprocess.Options{StderrLimit: 4096, GracePeriod: 100 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	manager, err := New(Config{
		Limits:  Limits{MaxSessions: 1, MaxRequests: 2, MaxChildren: 1, MaxCancels: 2, MaxTombstones: 2, MaxObservations: 16},
		Starter: ManagedStarter{Manager: supervisor},
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
	marker := filepath.Join(t.TempDir(), "scheduled")
	deadline := localDarwinTestDeadline(t)
	started := manager.Start(context.Background(), StartRequest{
		Profile: profile(t),
		Process: managedprocess.Spec{Path: binary, Dir: root, Env: append(os.Environ(), "LSP_TRACE_FAKE_LSP_SCHEDULED="+marker)},
	})
	if started.Failure != "" || started.Start.Evidence != managedprocess.LocalDarwinSupervisionOnly {
		t.Fatalf("ASSERT_SESSIONRUNTIME_LOCAL_DARWIN_FAKE_LSP: %+v", started)
	}
	waitForFakeLSPScheduled(t, marker, deadline)
	pending := manager.BeginReadiness(context.Background(), started.SessionID, started.Generation, deadline)
	if pending.State != ReadinessPending {
		t.Fatalf("ASSERT_SESSIONRUNTIME_LOCAL_DARWIN_READINESS_PENDING: %+v", pending)
	}
	ready, found := manager.WaitReadiness(context.Background(), pending.ID)
	if !found || ready.State != ReadinessReady || ready.Failure != "" {
		t.Fatalf("ASSERT_SESSIONRUNTIME_LOCAL_DARWIN_CORRELATED_READY: found=%v snapshot=%+v", found, ready)
	}
	again, found := manager.Readiness(pending.ID)
	if !found || again != ready {
		t.Fatalf("ASSERT_SESSIONRUNTIME_LOCAL_DARWIN_READINESS_IMMUTABLE: found=%v first=%+v again=%+v", found, ready, again)
	}
	if census := manager.Census(); census.Workers != 0 {
		t.Fatalf("ASSERT_SESSIONRUNTIME_LOCAL_DARWIN_LEAK_FREE: %+v", census)
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
