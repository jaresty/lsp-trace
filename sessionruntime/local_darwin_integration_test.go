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
	started := manager.Start(context.Background(), StartRequest{
		Profile: profile(t),
		Process: managedprocess.Spec{Path: binary, Dir: root},
	})
	if started.Failure != "" || started.Start.Evidence != managedprocess.LocalDarwinSupervisionOnly {
		t.Fatalf("ASSERT_SESSIONRUNTIME_LOCAL_DARWIN_FAKE_LSP: %+v", started)
	}
	// The process is started, but sessionruntime has no public initialization handshake yet.
	// Bound this fixture-only settle period before exercising graceful shutdown.
	time.Sleep(100 * time.Millisecond)
	accepted := manager.Stop(context.Background(), started.SessionID, "local-test")
	if accepted.Failure != "" {
		t.Fatalf("ASSERT_SESSIONRUNTIME_LOCAL_DARWIN_STOP_ACCEPTED: %+v", accepted)
	}
	terminal := waitOperation(t, manager, accepted.IntentID, OperationComplete)
	if terminal.Failure != "" {
		t.Fatalf("ASSERT_SESSIONRUNTIME_LOCAL_DARWIN_STOP_COMPLETE: %+v", terminal)
	}
}
