//go:build darwin

package sessionruntime

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"lsp-trace/internal/managedprocess"
	"lsp-trace/internal/session"
)

func TestSupplyStalledProcessHelper(t *testing.T) {
	if os.Getenv("LSP_TRACE_SUPPLY_STALL_HELPER") != "1" {
		return
	}
	if err := os.WriteFile(os.Getenv("LSP_TRACE_SUPPLY_STALL_MARKER"), []byte("scheduled"), 0600); err != nil {
		os.Exit(2)
	}
	// Deliberately never read stdin. The supervisor owns termination and reap.
	for {
		time.Sleep(time.Hour)
	}
}

func TestSupplyCancellationReapsOwnedProcess(t *testing.T) {
	m, req, _, _ := supplyFixture(t, bytes.Repeat([]byte("x"), MaxDocumentSupplyBytes))
	supervisor, err := managedprocess.NewLocalDarwinSupervisor(managedprocess.Options{StderrLimit: 1024, GracePeriod: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(t.TempDir(), "scheduled")
	child, started := supervisor.Start(context.Background(), managedprocess.Spec{Path: executable, Args: []string{"-test.run=^TestSupplyStalledProcessHelper$"}, Env: append(os.Environ(), "LSP_TRACE_SUPPLY_STALL_HELPER=1", "LSP_TRACE_SUPPLY_STALL_MARKER="+marker)})
	if started.Kind != managedprocess.StartStarted {
		t.Fatal(started)
	}
	defer child.Close()
	defer child.Teardown(context.Background())
	waitForFakeLSPScheduled(t, marker, localDarwinTestDeadline(t))
	m.sessions[req.SessionID].process = child
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	done := make(chan DocumentResult, 1)
	go func() { done <- m.PrepareDocument(ctx, req) }()
	select {
	case got := <-done:
		if got.Failure != session.RequestTimeout || got.Supply != nil || got.Version != 0 || child.Observe().Kind != managedprocess.SurvivorDead || m.Census().Workers != 0 {
			t.Fatalf("ASSERT_SUPPLY_PROCESS_REAP: result=%+v survivor=%+v census=%+v", got, child.Observe(), m.Census())
		}
		t.Log("ASSERT_SUPPLY_PROCESS_REAP: PASS")
	case <-time.After(time.Second):
		child.Close()
		child.Teardown(context.Background())
		<-done
		t.Fatal("ASSERT_SUPPLY_PROCESS_REAP: deadline did not interrupt and reap owned process")
	}
	// Confirm cancellation really admitted transport work rather than expiring
	// during the bounded source read. The retired owner is installed only then.
	if m.sessions[req.SessionID].retired == nil {
		t.Fatal("ASSERT_SUPPLY_PROCESS_REAP: transport cancellation was not exercised")
	}
}
