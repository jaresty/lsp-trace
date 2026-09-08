package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// TestDeadlineHelper uses this test executable, never a shell or installed provider.
func TestDeadlineHelper(t *testing.T) {
	mode := os.Getenv("PROVIDER_DEADLINE_HELPER")
	if mode == "" {
		return
	}
	signal.Ignore(os.Interrupt) // Exercise bounded escalation, not just cooperative exit.
	if mode != "write-blocked" {
		if _, err := io.Copy(io.Discard, os.Stdin); err != nil {
			os.Exit(31)
		}
	}
	switch mode {
	case "response-live", "success", "abnormal", "response-bound":
		fmt.Fprint(os.Stdout, "Content-Length: 11\r\n\r\n{\"ok\":true}")
	case "malformed":
		fmt.Fprint(os.Stdout, "bad\r\n\r\n{}")
	}
	if mode != "open" {
		_ = os.Stdout.Close()
	}
	marker := os.Getenv("PROVIDER_DEADLINE_MARKER")
	markerTemp := marker + ".tmp"
	defer os.Remove(markerTemp)
	file, err := os.OpenFile(markerTemp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		os.Exit(32)
	}
	if _, err := file.WriteString(strconv.Itoa(os.Getpid())); err != nil {
		_ = file.Close()
		os.Exit(32)
	}
	if err := file.Close(); err != nil {
		os.Exit(32)
	}
	if err := os.Rename(markerTemp, marker); err != nil {
		os.Exit(32)
	}
	switch mode {
	case "success", "malformed", "response-bound":
		os.Exit(0)
	case "abnormal":
		os.Exit(7)
	}
	for {
		time.Sleep(time.Hour)
	}
}

// Wait has finished but publication is modeled separately, as in Execute.
func TestDeadlineAlreadyExited(t *testing.T) {
	cmd := exec.Command(os.Args[0], "-test.run=^TestDeadlineHelper$")
	cmd.Env = append(os.Environ(), "PROVIDER_DEADLINE_HELPER=abnormal", "PROVIDER_DEADLINE_MARKER="+filepath.Join(t.TempDir(), "pid"), "GORACE=atexit_sleep_ms=0")
	err := cmd.Run()
	if cmd.ProcessState == nil || cmd.ProcessState.ExitCode() != 7 {
		t.Fatalf("SETUP: helper must exit 7: %v", err)
	}
	waited := make(chan error, 1)
	waited <- err
	var waitErr error
	terminated := terminate(cmd, waited, 20*time.Millisecond, &waitErr)
	if terminated || waitErr != err || len(waited) != 0 {
		t.Fatalf("ASSERT_ALREADY_EXITED: terminated=%t waitErr=%v pending=%d", terminated, waitErr, len(waited))
	}
	t.Log("ASSERT_ALREADY_EXITED PASS normal exit-code 7 preserved; one wait result consumed")
}

func TestDeadlineCompletion(t *testing.T) {
	for _, tc := range []struct {
		mode   string
		cancel bool
		want   FailureKind
	}{
		{"open", false, TimedOut},
		{"closed", false, TimedOut},
		{"response-live", false, TimedOut},
		{"write-blocked", false, TimedOut},
		{"closed", true, Canceled},
		{"write-blocked", true, Canceled},
		{"success", false, ""},
		{"abnormal", false, AbnormalExit},
		{"malformed", false, ProtocolFailed},
		{"response-bound", false, LimitExceeded},
	} {
		name := tc.mode
		if tc.cancel {
			name += "-cancel"
		}
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			marker := filepath.Join(dir, "helper.pid")
			markerTemp := marker + ".tmp"
			t.Cleanup(func() {
				_ = os.Remove(marker)
				_ = os.Remove(markerTemp)
			})
			registry := NewRegistry()
			err := registry.Register(Registration{ID: "deadline", Path: os.Args[0], Args: []string{"-test.run=^TestDeadlineHelper$"}, Env: append(os.Environ(), "PROVIDER_DEADLINE_HELPER="+tc.mode, "PROVIDER_DEADLINE_MARKER="+marker, "GORACE=atexit_sleep_ms=0")})
			if err != nil {
				t.Fatal(err)
			}
			limits := Limits{RequestBytes: 4096, ResponseBytes: 4096, ProtocolMessages: 1, StderrBytes: 128, WallTime: 200 * time.Millisecond, TerminationGrace: 20 * time.Millisecond}
			request := json.RawMessage(`{}`)
			if tc.mode == "write-blocked" {
				request = json.RawMessage(`"` + strings.Repeat("x", 4<<20) + `"`)
				limits.RequestBytes = len(request)
			}
			if tc.mode == "response-bound" {
				limits.ResponseBytes = 4
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			done := make(chan Receipt, 1)
			start := time.Now()
			go func() { done <- NewRuntime(registry).Execute(ctx, "deadline", request, limits) }()
			// The marker proves the helper reached the intended stdout/stdin state.
			var pid int
			for until := time.Now().Add(3 * time.Second); time.Now().Before(until); {
				if b, err := os.ReadFile(marker); err == nil {
					if parsed, err := strconv.Atoi(string(b)); err == nil && parsed > 0 {
						pid = parsed
						break
					}
				}
				time.Sleep(time.Millisecond)
			}
			if pid == 0 {
				cancel()
				t.Fatal("SETUP: helper did not reach marker")
			}
			child, err := os.FindProcess(pid)
			if err != nil {
				t.Fatal(err)
			}
			defer child.Release()
			// Last-resort cleanup is a failing observation, never timeout success.
			if tc.cancel {
				cancel()
			}
			var got Receipt
			exceeded := false
			select {
			case got = <-done:
			case <-time.After(time.Second):
				exceeded = true
				if err := child.Kill(); err != nil {
					t.Errorf("external cleanup kill: %v", err)
				}
				select {
				case got = <-done:
				case <-time.After(3 * time.Second):
					t.Fatal("CLEANUP: runtime failed to reap after external kill")
				}
			}
			if exceeded {
				t.Error("ASSERT_CONTINUOUS_DEADLINE: runtime required external cleanup after stdout completion")
			}
			if !got.Reaped || child.Signal(syscall.Signal(0)) == nil {
				_ = child.Kill()
				t.Errorf("ASSERT_REAP: helper remains alive or unreaped: %+v", got)
			} else {
				t.Logf("ASSERT_REAP PASS pid=%d reaped=true alive=false external_cleanup=%t", pid, exceeded)
			}
			if !exceeded {
				if tc.want == "" {
					if got.Failure != nil || string(got.Response) != `{"ok":true}` || got.Terminated || got.ExitCode != 0 {
						t.Errorf("ASSERT_COMPLETION_CLASSIFICATION: %+v", got)
					}
				} else if got.Failure == nil || got.Failure.Kind != tc.want {
					t.Errorf("ASSERT_COMPLETION_CLASSIFICATION: want=%s got=%+v", tc.want, got)
				}
				if tc.want == TimedOut || tc.want == Canceled {
					if !got.Terminated {
						t.Error("ASSERT_CONTINUOUS_DEADLINE: cancellation did not terminate live helper")
					}
					t.Logf("ASSERT_CONTINUOUS_DEADLINE PASS elapsed=%s kind=%s", time.Since(start), tc.want)
				}
				if tc.want == AbnormalExit && (got.ExitCode != 7 || got.Terminated) {
					t.Errorf("ASSERT_EXIT_CLASSIFICATION: %+v", got)
				}
			}
		})
	}
}
