//go:build darwin || linux

package censuslineagepilot_test

import (
	"context"
	"encoding/json"
	"os"
	"sync"
	"testing"
	"time"

	"lsp-trace/sessionruntime"
)

// Both the successful and resource-close-failed cases hold the actual Manager's
// asynchronous Stop worker at owned Teardown. No replacement Manager or public
// lifecycle callback can manufacture completion for the caller.
func TestVariableCallerV2JoinedAsyncStop(t *testing.T) {
	for _, kind := range []string{"success", "shutdown-close"} {
		t.Run(kind, func(t *testing.T) {
			c, path, git := variableOwnedConfig(t, 1, 0)
			providerKind := ""
			if kind == "shutdown-close" {
				providerKind = kind
			}
			factory, starts, child, _ := variableOwnedFactory(t, providerKind)
			child.teardownEntered = make(chan struct{})
			child.teardownRelease = make(chan struct{})
			var releaseOnce sync.Once
			release := func() { releaseOnce.Do(func() { close(child.teardownRelease) }) }
			defer release()
			type result struct {
				report variableCallerReport
				err    error
			}
			done := make(chan result, 1)
			go func() {
				r, e := variableOwnedRun(context.Background(), "execute-once", path, git, factory, variableClose)
				done <- result{r, e}
			}()
			select {
			case <-child.teardownEntered:
			case <-time.After(3 * time.Second):
				t.Fatal("owned asynchronous Stop did not enter teardown")
			}
			select {
			case <-done:
				t.Fatal("caller returned before asynchronous Stop joined")
			default:
			}
			if _, e := os.Stat(c.AttemptMarkerPath); e != nil {
				t.Fatal("attempt must already be claimed", e)
			}
			if _, e := os.Stat(c.ReportPath); !os.IsNotExist(e) {
				t.Fatal("report written before cleanup completed", e)
			}
			for _, root := range append(c.ProducerOutputs[:], c.PostOutputs[:]...) {
				entries, e := os.ReadDir(root)
				if e != nil || len(entries) != 0 {
					t.Fatal("retention occurred before cleanup completion", root, e)
				}
			}
			release()
			var out result
			select {
			case out = <-done:
			case <-time.After(3 * time.Second):
				t.Fatal("owned joined cleanup did not return")
			}
			cleanup := out.report.Cleanup
			if starts.Load() != 1 || cleanup.StopCalls != 1 || cleanup.ShutdownCalls != 1 || cleanup.TerminalReads != 1 || !cleanup.ShutdownJoined || !cleanup.TerminalFound || child.shutdownRequests.Load() != 1 || child.teardowns.Load() != 1 || !child.closed.Load() || child.writesAfterClose.Load() != 0 {
				t.Fatalf("ASSERT_JOINED_CLEANUP_EXACT_ONCE: starts=%d shutdown=%d teardown=%d after-close=%d cleanup=%+v", starts.Load(), child.shutdownRequests.Load(), child.teardowns.Load(), child.writesAfterClose.Load(), cleanup)
			}
			// Zero writes after child close is stronger than zero after Shutdown returns:
			// actual child close occurred inside the Stop worker that Shutdown joined.
			if cleanup.Terminal.SessionID != c.Session.SessionID || cleanup.Terminal.Generation != c.Session.Generation {
				t.Fatal("cleanup identity not pinned to config")
			}
			if kind == "shutdown-close" {
				if out.err == nil || cleanup.Terminal.State != sessionruntime.OperationFailed || cleanup.Status != "UNVERIFIED_FAILED_SPENT" || len(out.report.CloseErrors) == 0 || out.report.Phase != "FAILED_SPENT" || out.report.Outcome != nil {
					t.Fatalf("ASSERT_JOINED_CLOSE_FAILURE_VISIBLE: err=%v cleanup=%+v phase=%s", out.err, cleanup, out.report.Phase)
				}
				t.Log("ASSERT_JOINED_CLOSE_FAILURE_VISIBLE: PASS; Shutdown joined; readonly Operation is FAILED; final report is FAILED_SPENT")
			} else {
				if out.err != nil || cleanup.Status != "JOINED_STOP_VERIFIED" || cleanup.Terminal.State != sessionruntime.OperationComplete || out.report.Phase != "PENDING_INTERPRETATION_PRIVATE_V2" {
					t.Fatal("successful joined cleanup", out.err, cleanup)
				}
			}
			raw, e := os.ReadFile(c.ReportPath)
			if e != nil {
				t.Fatal(e)
			}
			var retained variableCallerReport
			if json.Unmarshal(raw, &retained) != nil || retained.Phase != out.report.Phase || retained.Cleanup.Terminal.State != cleanup.Terminal.State || len(retained.CloseErrors) != len(out.report.CloseErrors) {
				t.Fatal("cleanup observation/errors missing from final report")
			}
			before := child.traffic.Load()
			if _, e := variableOwnedRun(context.Background(), "execute-once", path, git, factory, variableClose); e == nil || starts.Load() != 1 || child.traffic.Load() != before || child.writesAfterClose.Load() != 0 {
				t.Fatal("spent cleanup attempt caused further provider/lifecycle work")
			}
			t.Log("ASSERT_JOINED_CLEANUP_EXACT_ONCE: PASS; starts=1 Stop=1 Shutdown=1 readonly Operation=1 provider writes after joined shutdown=0")
		})
	}
}

func TestVariableCallerV2TerminalCleanupUnknownRefused(t *testing.T) {
	_, path, git := variableOwnedConfig(t, 1, 0)
	factory, _, child, _ := variableOwnedFactory(t, "")
	report, e := variableOwnedRun(context.Background(), "execute-once", path, git, factory, variableClose)
	if e != nil {
		t.Fatal(e)
	}
	original := report.Cleanup
	if e := variableTerminalCleanup(original); e != nil {
		t.Fatal("actual owned successful terminal prerequisite", e)
	}
	before := child.traffic.Load()
	for _, kind := range []string{"missing", "pending", "unknown-state", "failed-empty-cause", "complete-with-failure", "not-joined", "empty-intent", "foreign-intent", "foreign-session", "wrong-generation", "restart", "foreign-caller", "zero-stop", "second-stop", "second-shutdown", "second-read"} {
		t.Run(kind, func(t *testing.T) {
			o := original
			switch kind {
			case "missing":
				o.TerminalFound = false
			case "pending":
				o.Terminal.State = sessionruntime.OperationPending
			case "unknown-state":
				o.Terminal.State = sessionruntime.OperationState("UNKNOWN_OWNED_STATUS")
			case "failed-empty-cause":
				o.Terminal.State = sessionruntime.OperationFailed
			case "complete-with-failure":
				o.Terminal.Failure = "OWNED_TERMINAL_FAILURE"
			case "not-joined":
				o.ShutdownJoined = false
			case "empty-intent":
				o.StopIntentID = ""
			case "foreign-intent":
				o.Terminal.ID += "-foreign"
			case "foreign-session":
				o.Terminal.SessionID += "-foreign"
			case "wrong-generation":
				o.Terminal.Generation++
			case "restart":
				o.Terminal.Restart = true
			case "foreign-caller":
				o.Terminal.CallerID = "foreign"
			case "zero-stop":
				o.StopCalls = 0
			case "second-stop":
				o.StopCalls = 2
			case "second-shutdown":
				o.ShutdownCalls = 2
			case "second-read":
				o.TerminalReads = 2
			}
			if e := variableTerminalCleanup(o); e == nil {
				t.Fatalf("ASSERT_CLEANUP_STATUS_CONSERVATIVE: %s accepted", kind)
			}
			if e := variableTerminalCleanup(original); e != nil || child.traffic.Load() != before || child.writesAfterClose.Load() != 0 {
				t.Fatal("readonly cleanup classification mutated runtime/provider", e)
			}
			t.Log("ASSERT_CLEANUP_STATUS_CONSERVATIVE: PASS; " + kind + " refused with no provider work")
		})
	}
}
