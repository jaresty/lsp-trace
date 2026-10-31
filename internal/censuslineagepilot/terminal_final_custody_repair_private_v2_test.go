//go:build darwin || linux

package censuslineagepilot

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"
)

// A synchronous fixture intervention at final custody, not a concurrent race.
// Err is already an observation boundary; the replacement happens before its
// first open, after retention has read back BOTH copies. No production hook.
type terminalFinalFIFOContextPrivateV2 struct {
	context.Context
	path    string
	swapped bool
	failure error
}

func (c *terminalFinalFIFOContextPrivateV2) Err() error {
	pcs := make([]uintptr, 32)
	n := runtime.Callers(1, pcs)
	frames := runtime.CallersFrames(pcs[:n])
	atFinal := false
	for {
		f, more := frames.Next()
		if strings.Contains(f.Function, "observeContentPairCustodyPrivate") || strings.Contains(f.Function, "observeTerminalPairCustodyPrivateV2") {
			atFinal = true
		}
		if !more {
			break
		}
	}
	if atFinal && !c.swapped {
		c.swapped = true
		c.failure = os.Remove(c.path)
		if c.failure == nil {
			c.failure = syscall.Mkfifo(c.path, 0600)
		}
		fmt.Println("FINAL_CUSTODY_FIFO_INSTALLED", c.failure)
	}
	if c.failure != nil {
		return c.failure
	}
	return c.Context.Err()
}
func TestTerminalSixFinalCustodyFIFOPrivateV2(t *testing.T) {
	if os.Getenv("LSP_TRACE_TERMINAL_FINAL_FIFO_CHILD") == "1" {
		// Accepted control exercises the same final boundary with regular pair objects.
		_, control := terminalFixturePrivateV2(t, 84, func(ctx context.Context, f FrozenScopePrivateV2, o TerminalDiscoveryOutcomePrivateV2) {
			roots, _, _ := packetRootsPrivateV2(t, f)
			if _, e := RetainTerminalDiscoveryPrivateV2(ctx, f, o, roots); e != nil {
				t.Fatal("regular control", e)
			}
			fmt.Println("ASSERT_SIX_FINAL_CUSTODY_CONTROL PASS")
		})
		_, _, _, e := control(false)
		if e != nil {
			t.Fatal(e)
		}
		_, run := terminalFixturePrivateV2(t, 84, func(ctx context.Context, f FrozenScopePrivateV2, o TerminalDiscoveryOutcomePrivateV2) {
			roots, primary, _ := packetRootsPrivateV2(t, f)
			attack := &terminalFinalFIFOContextPrivateV2{Context: ctx, path: filepath.Join(primary, Digest(o.state.raw))}
			_, e := RetainTerminalDiscoveryPrivateV2(attack, f, o, roots)
			if !attack.swapped || attack.failure != nil || e == nil {
				t.Fatalf("ASSERT_SIX_FINAL_FIFO_REFUSED FAIL swapped=%t error=%v fixture=%v", attack.swapped, e, attack.failure)
			}
			fmt.Println("ASSERT_SIX_FINAL_FIFO_REFUSED PASS")
		})
		_, _, _, e = run(false)
		if e != nil {
			t.Fatal(e)
		}
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestTerminalSixFinalCustodyFIFOPrivateV2$", "-test.v")
	cmd.Env = append(os.Environ(), "LSP_TRACE_TERMINAL_FINAL_FIFO_CHILD=1")
	out, e := cmd.CombinedOutput() // Run includes the only Wait, including killed children.
	joined := cmd.ProcessState != nil
	t.Logf("FINAL_FIFO_CHILD pid=%d joined=%t killed=%t error=%v output=%s", cmd.Process.Pid, joined, ctx.Err() != nil, e, out)
	if !joined || e != nil || !strings.Contains(string(out), "ASSERT_SIX_FINAL_FIFO_REFUSED PASS") {
		t.Error("ASSERT_SIX_FINAL_FIFO_REFUSED FAIL")
	} else {
		t.Log("ASSERT_SIX_FINAL_FIFO_REFUSED PASS")
	}
}
