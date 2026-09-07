//go:build darwin || linux || freebsd || netbsd || openbsd || dragonfly

package integratedconformance

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	executionruntime "lsp-trace/internal/execution"
	"lsp-trace/internal/operation"
)

// A child process bounds the RED witness without abandoning a blocked goroutine.
// No FIFO writer is opened: even the open, not merely a subsequent read, must
// finish without waiting for a peer. The execution deadline cannot rescue an
// ordinary blocking FIFO open.
func TestCustodyRepairFIFODeadline(t *testing.T) {
	if root := os.Getenv("LSP_TRACE_FIFO_REPAIR_ROOT"); root != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()
		raw, _ := json.Marshal(map[string]any{"root": filepath.Join(root, "out"), "operational": map[string]any{"source_root": root, "require_authenticated": true, "inputs": []map[string]string{{"path": "ordinary", "class": "SOURCE"}, {"path": "fifo", "class": "SOURCE"}}}})
		_, failure := executionruntime.NewProductionExecutor().Execute(ctx, operation.Request{Name: operation.CustodyExecute, RequestID: "fifo", Input: raw})
		if failure == nil {
			t.Fatal("required failed read must not publish")
		}
		retained := strings.Join(failure.Diagnostics, " ")
		if !strings.Contains(retained, `"path":"ordinary"`) || !strings.Contains(retained, `"acquisition_status":"UNREADABLE"`) || !strings.Contains(retained, "regular file") {
			t.Fatalf("missing acquired and rejected evidence: %s", retained)
		}
		return
	}
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "ordinary"), []byte("retained bytes"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, "out"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Mkfifo(filepath.Join(root, "fifo"), 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestCustodyRepairFIFODeadline$")
	cmd.Env = append(os.Environ(), "LSP_TRACE_FIFO_REPAIR_ROOT="+root)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("ASSERT_FIFO_DEADLINE: child failed or blocked beyond cancellation: %v %s", err, out)
	}
	entries, err := os.ReadDir(filepath.Join(root, "out"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatal("failed required acquisition published")
	}
}
