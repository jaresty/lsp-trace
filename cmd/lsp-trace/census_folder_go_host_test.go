//go:build darwin && lsp_trace_controlled_describe_fixture

package main

// This client uses only Go's standard-library subprocess/socket primitives.
// The child executes the real normal-folder command with controlled LSP wire;
// semantic replies are fixtures, not model or independent-review qualification.
import (
	"bytes"
	"context"
	"net"
	"os"
	"os/exec"
	"runtime/debug"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestCensusFolderHostNeutralBuild(t *testing.T) {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		t.Fatal("build dependency evidence missing")
	}
	for _, dep := range info.Deps {
		for _, forbidden := range []string{"pi-coding-agent", "pi-ai", "nodejs", "python"} {
			if strings.Contains(strings.ToLower(dep.Path), forbidden) {
				t.Fatal("ASSERT_GO_RUNTIME_DEPENDENCY", dep.Path)
			}
		}
	}
	t.Log("ASSERT_GO_RUNTIME_DEPENDENCY: PASS Go build graph has no Pi/Node/Python modules; runtime exercised by empty-PATH child")
}

func TestCensusFolderGoHost(t *testing.T) {
	for _, mode := range []string{"same-thread", "subagents", "missing", "disconnect", "stale", "mismatch", "malformed", "withheld", "cancel", "delayed"} {
		t.Run(mode, func(t *testing.T) {
			pair, err := syscall.Socketpair(syscall.AF_UNIX, syscall.SOCK_STREAM, 0)
			if err != nil {
				t.Fatal(err)
			}
			child := os.NewFile(uintptr(pair[0]), "go-caller-child")
			parent := os.NewFile(uintptr(pair[1]), "go-caller-parent")
			conn, err := net.FileConn(parent)
			parent.Close()
			if err != nil {
				child.Close()
				t.Fatal(err)
			}
			defer conn.Close()
			defer child.Close()
			exe, err := os.Executable()
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, exe, "-test.run=^TestCensusFolderManagedHostProcess$", "-test.v")
			// No PATH-discovered Pi, node, python, provider binary or credentials exists
			// in this environment. Native provider transport is explicitly the fixture.
			home := t.TempDir()
			if mode == "missing" {
				home = censusF11ProtectedHome(t)
			}
			cmd.Env = []string{"PATH=/nonexistent-census-host-bin", "HOME=" + home, "TMPDIR=" + os.TempDir(), "GOPROXY=off", "GOSUMDB=off", "GOTOOLCHAIN=local", "GOTELEMETRY=off", "LSP_TRACE_CALLER_FD=3", "CENSUS_MANAGED_PROCESS_FIXTURE=1"}
			negative := mode != "same-thread" && mode != "subagents" && mode != "delayed"
			if negative {
				cmd.Env = append(cmd.Env, "CENSUS_MANAGED_PROCESS_REFUSAL="+mode)
			}
			cmd.ExtraFiles = []*os.File{child}
			var output bytes.Buffer
			cmd.Stdout = &output
			cmd.Stderr = &output
			if err := cmd.Start(); err != nil {
				t.Fatal(err)
			}
			child.Close()
			// The running process remains an owned exec.Cmd while control returns to
			// this client; a later write answers a task without restarting the process.
			joined := false
			defer func() {
				if !joined {
					cmd.Process.Kill()
					cmd.Wait()
				}
			}()
			var result folderCallerResult
			if mode != "missing" {
				var cancelledAt time.Time
				result = serveFolderCallerFixture(conn, mode == "subagents", false, mode, func() { cancelledAt = time.Now(); _ = cmd.Process.Signal(syscall.SIGTERM) })
				if mode == "cancel" && (cancelledAt.IsZero() || time.Since(cancelledAt) > 5*time.Second) {
					t.Error("ASSERT_GO_CANCEL: did not promptly join")
				}
			}
			err = cmd.Wait()
			joined = true
			if err != nil {
				t.Fatalf("ASSERT_GO_ONLY_HOST: %s: %v\n%s\ncaller: %v", mode, err, output.String(), result.err)
			}
			if mode == "missing" {
				if !strings.Contains(output.String(), "ASSERT_OWNER_START_NO_ACQUISITION: PASS") {
					t.Fatal("ordinary owner-start contract missing", output.String())
				}
			} else if negative {
				if !strings.Contains(output.String(), "ASSERT_GO_MANAGED_REFUSAL: PASS") {
					t.Fatal(output.String())
				}
			} else if result.err != nil || !strings.Contains(output.String(), "ASSERT_FOLDER_PUBLICATION_REOPEN: PASS exact restored report") {
				t.Fatalf("ASSERT_GO_ONLY_HOST: publication missing: %v\n%s", result.err, output.String())
			}
			t.Log("ASSERT_GO_ONLY_HOST: PASS", mode, "owned and joined; PATH excludes optional runtimes")
		})
	}
}
