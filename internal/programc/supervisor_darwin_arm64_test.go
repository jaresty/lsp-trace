//go:build darwin && arm64

package programc

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"lsp-trace/internal/graph"
)

func TestMain(m *testing.M) {
	if len(os.Args) == 2 && os.Args[1] == workerArgument {
		parentPID := os.Getppid()
		startTestWorkerParentWatchdog(parentPID)
		writeTestWorkerPIDRecord(0, 0)
		if mode := os.Getenv("PROGRAMC_TEST_WORKER"); mode != "" {
			if mode == "delayed-private" {
				// initialTreeRSS has a two-second acquisition bound. Keep this
				// fixture alive beyond that entire window so the test never
				// depends on /bin/ps scheduling latency.
				time.Sleep(3 * time.Second)
			} else {
				testWorkerMode()
				return
			}
		}
		_, code := RunPrivateWorker(os.Args[1:], os.Stdin, os.Stdout, os.Stderr)
		os.Exit(code)
	}
	os.Exit(m.Run())
}

func startTestWorkerParentWatchdog(parentPID int) {
	go func() {
		ticker := time.NewTicker(10 * time.Millisecond)
		defer ticker.Stop()
		for range ticker.C {
			if os.Getppid() == parentPID {
				continue
			}
			// The production supervisor starts the worker with Setpgid, making
			// its PID the process-group ID. kill(2) selects and signals the
			// complete group before SIGKILL can terminate this goroutine's process.
			_ = syscall.Kill(-os.Getpid(), syscall.SIGKILL)
			os.Exit(125)
		}
	}()
}

func writeTestWorkerPIDRecord(descendantPID, descendantPGID int) {
	path := os.Getenv("PROGRAMC_TEST_PIDFILE")
	if path == "" {
		return
	}
	workerPGID, err := syscall.Getpgid(os.Getpid())
	if err != nil {
		return
	}
	record := fmt.Sprintf("worker_pid=%d worker_pgid=%d descendant_pid=%d descendant_pgid=%d\n", os.Getpid(), workerPGID, descendantPID, descendantPGID)
	temporary := path + ".tmp"
	if os.WriteFile(temporary, []byte(record), 0600) == nil {
		_ = os.Rename(temporary, path)
	}
}

func testWorkerMode() {
	_, _ = io.Copy(io.Discard, os.Stdin)
	switch os.Getenv("PROGRAMC_TEST_WORKER") {
	case "hang":
		for {
			time.Sleep(time.Second)
		}
	case "child":
		cmd := exec.Command("/bin/sh", "-c", "sleep 30 & echo $! > \"$PROGRAMC_TEST_PIDFILE\"; wait")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		_ = cmd.Run()
		os.Exit(9)
	case "watchdog-child":
		cmd := exec.Command("/bin/sleep", "30")
		if err := cmd.Start(); err != nil {
			fmt.Fprintf(os.Stderr, "start watchdog descendant: %v\n", err)
			os.Exit(10)
		}
		descendantPGID, err := syscall.Getpgid(cmd.Process.Pid)
		if err != nil {
			_ = cmd.Process.Kill()
			os.Exit(11)
		}
		writeTestWorkerPIDRecord(cmd.Process.Pid, descendantPGID)
		_ = cmd.Wait()
		os.Exit(9)
	case "nonzero":
		fmt.Fprintln(os.Stderr, "controlled nonzero")
		os.Exit(23)
	case "panic":
		panic("controlled panic")
	case "malformed":
		fmt.Fprint(os.Stdout, "not-json")
		return
	case "oversized":
		_, _ = io.WriteString(os.Stdout, strings.Repeat("x", 4096))
		return
	}
}
func withSupervisorDefaults(t *testing.T) {
	t.Helper()
	oldWall, oldRSS, oldPoll, oldOutput, oldPS := supervisorWall, supervisorRSSBytes, supervisorPoll, supervisorOutputBytes, supervisorPSPath
	t.Cleanup(func() {
		supervisorWall, supervisorRSSBytes, supervisorPoll, supervisorOutputBytes, supervisorPSPath = oldWall, oldRSS, oldPoll, oldOutput, oldPS
	})
	supervisorWall = 2 * time.Second
	supervisorRSSBytes = 1 << 40 // the linked test binary exceeds the production worker's RSS
	supervisorPoll = 5 * time.Millisecond
}
func supervisedFailure(t *testing.T, mode string, ctx context.Context) *SupervisionFailure {
	t.Helper()
	withSupervisorDefaults(t)
	t.Setenv("PROGRAMC_TEST_WORKER", mode)
	_, _, failed := ComputeSupervised(ctx, []byte("ignored"), 7)
	return failed
}
func requireCode(t *testing.T, failed *SupervisionFailure, code SupervisionCode) {
	t.Helper()
	if failed == nil || failed.Code != code {
		t.Fatalf("ASSERT_%s: failure=%v", code, failed)
	}
	t.Logf("ASSERT_%s: PASS", code)
}

func TestSupervisedSuccessPreservesExactOutcome(t *testing.T) {
	withSupervisorDefaults(t)
	supervisorWall = 10 * time.Second
	// Keep the worker alive long enough for the mandatory first /bin/ps RSS
	// observation; successful workers without that observation must fail closed.
	t.Setenv("PROGRAMC_TEST_WORKER", "delayed-private")
	a, b := node("a", 0), node("b", 1)
	input := validV5(t, []graph.Node{a, b}, []graph.Edge{{CallerNodeID: a.ID, CalleeNodeID: b.ID, CallSites: []graph.Range{{}}}})
	want, baseFailure := Compute(input, 19)
	if baseFailure != nil {
		t.Fatal(baseFailure)
	}
	got, observation, failed := ComputeSupervised(context.Background(), input, 19)
	if failed != nil {
		t.Fatal(failed)
	}
	if got.LogicalDigest != want.LogicalDigest || !equalCommunities(got.Communities, want.Communities) || !bytes.Equal(got.Source.InputBytes(), want.Source.InputBytes()) || !bytes.Equal(got.Source.GraphV5Bytes(), want.Source.GraphV5Bytes()) || observation.Samples < 1 || observation.Ceiling != ObservationCeiling {
		t.Fatalf("ASSERT_SUPERVISED_SUCCESS_EXACT got=%#v want=%#v observation=%+v", got, want, observation)
	}
	t.Log("ASSERT_SUPERVISED_SUCCESS_EXACT: PASS")
}
func TestSupervisedTimeoutCancellationAndExitKinds(t *testing.T) {
	withSupervisorDefaults(t)
	supervisorWall = 50 * time.Millisecond
	t.Setenv("PROGRAMC_TEST_WORKER", "hang")
	_, _, failed := ComputeSupervised(context.Background(), nil, 1)
	requireCode(t, failed, CodeTimeout)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	requireCode(t, supervisedFailure(t, "hang", ctx), CodeCancelled)
	requireCode(t, supervisedFailure(t, "nonzero", context.Background()), CodeNonzeroExit)
	requireCode(t, supervisedFailure(t, "panic", context.Background()), CodePanic)
	requireCode(t, supervisedFailure(t, "malformed", context.Background()), CodeMalformedOutput)
	withSupervisorDefaults(t)
	supervisorOutputBytes = 128
	t.Setenv("PROGRAMC_TEST_WORKER", "oversized")
	_, _, failed = ComputeSupervised(context.Background(), nil, 1)
	requireCode(t, failed, CodeOutputLimit)
}
func TestProcessGroupCleanup(t *testing.T) {
	withSupervisorDefaults(t)
	pidfile := t.TempDir() + "/pid"
	t.Setenv("PROGRAMC_TEST_WORKER", "child")
	t.Setenv("PROGRAMC_TEST_PIDFILE", pidfile)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	_, _, failed := ComputeSupervised(ctx, nil, 1)
	requireCode(t, failed, CodeCancelled)
	deadline := time.Now().Add(5 * time.Second)
	for {
		raw, err := os.ReadFile(pidfile)
		if err == nil {
			pid, _ := strconv.Atoi(strings.TrimSpace(string(raw)))
			err = syscall.Kill(pid, 0)
			if err == syscall.ESRCH || processIsZombie(pid) {
				t.Log("ASSERT_PROCESS_GROUP_CLEANUP: PASS")
				return
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("ASSERT_PROCESS_GROUP_CLEANUP: descendant remains running or pid unavailable: %v", err)
		}
		time.Sleep(10 * time.Millisecond)
	}
}
func processIsZombie(pid int) bool {
	out, err := exec.Command("/bin/ps", "-o", "state=", "-p", strconv.Itoa(pid)).Output()
	return err == nil && strings.HasPrefix(strings.TrimSpace(string(out)), "Z")
}

type watchdogPIDs struct {
	workerPID, workerPGID, descendantPID, descendantPGID int
}

func readWatchdogPIDs(path string) (watchdogPIDs, error) {
	var p watchdogPIDs
	raw, err := os.ReadFile(path)
	if err != nil {
		return p, err
	}
	_, err = fmt.Sscanf(string(raw), "worker_pid=%d worker_pgid=%d descendant_pid=%d descendant_pgid=%d", &p.workerPID, &p.workerPGID, &p.descendantPID, &p.descendantPGID)
	validDescendant := (p.descendantPID == 0 && p.descendantPGID == 0) || (p.descendantPID > 1 && p.descendantPGID == p.workerPGID)
	if err != nil || p.workerPID <= 1 || p.workerPGID != p.workerPID || !validDescendant {
		return p, fmt.Errorf("invalid watchdog PID record %q: %w", raw, err)
	}
	return p, nil
}

func processGone(pid int) bool {
	return syscall.Kill(pid, 0) == syscall.ESRCH || processIsZombie(pid)
}

func waitProcessesGone(p watchdogPIDs, deadline time.Time) error {
	for time.Now().Before(deadline) {
		descendantGone := p.descendantPID == 0 || processGone(p.descendantPID)
		if processGone(p.workerPID) && descendantGone {
			return nil
		}
		time.Sleep(10 * time.Millisecond)
	}
	descendantGone := p.descendantPID == 0 || processGone(p.descendantPID)
	return fmt.Errorf("processes remain: worker pid=%d gone=%t descendant pid=%d gone=%t", p.workerPID, processGone(p.workerPID), p.descendantPID, descendantGone)
}

func TestWatchdogDriver(t *testing.T) {
	mode := os.Getenv("PROGRAMC_TEST_WATCHDOG_DRIVER")
	if mode == "" {
		t.Skip("subprocess driver")
	}
	withSupervisorDefaults(t)
	switch mode {
	case "timeout":
		supervisorWall = 30 * time.Second
		t.Setenv("PROGRAMC_TEST_WORKER", "watchdog-child")
		t.Setenv("PROGRAMC_TEST_PIDFILE", os.Getenv("PROGRAMC_TEST_PIDFILE"))
		_, _, _ = ComputeSupervised(context.Background(), nil, 1)
	case "failure":
		requireCode(t, supervisedFailure(t, "nonzero", context.Background()), CodeNonzeroExit)
	case "success":
		supervisorWall = 10 * time.Second
		t.Setenv("PROGRAMC_TEST_WORKER", "delayed-private")
		input := validV5(t, nil, nil)
		if _, _, failed := ComputeSupervised(context.Background(), input, 1); failed != nil {
			t.Fatal(failed)
		}
	default:
		t.Fatalf("unknown watchdog driver mode %q", mode)
	}
}

func TestWorkerParentWatchdogPreventsTimeoutOrphans(t *testing.T) {
	for _, tc := range []struct {
		mode        string
		testTimeout time.Duration
		wantTimeout bool
	}{{"timeout", 350 * time.Millisecond, true}, {"failure", 5 * time.Second, false}, {"success", 8 * time.Second, false}} {
		t.Run(tc.mode, func(t *testing.T) {
			dir := t.TempDir()
			pidfile := filepath.Join(dir, "pids")
			cmd := exec.Command(os.Args[0], "-test.run=^TestWatchdogDriver$", "-test.timeout="+tc.testTimeout.String())
			cmd.Env = append(os.Environ(), "PROGRAMC_TEST_WATCHDOG_DRIVER="+tc.mode, "PROGRAMC_TEST_PIDFILE="+pidfile)
			cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
			var output bytes.Buffer
			cmd.Stdout, cmd.Stderr = &output, &output
			if err := cmd.Start(); err != nil {
				t.Fatal(err)
			}
			driverPGID := cmd.Process.Pid
			var observed watchdogPIDs
			t.Cleanup(func() {
				_ = syscall.Kill(-driverPGID, syscall.SIGKILL)
				if observed.workerPGID > 1 {
					_ = syscall.Kill(-observed.workerPGID, syscall.SIGKILL)
				}
			})
			err := cmd.Wait()
			if tc.wantTimeout {
				if err == nil || !strings.Contains(output.String(), "test timed out") {
					t.Fatalf("expected test timeout, err=%v output=%s", err, output.String())
				}
				deadline := time.Now().Add(2 * time.Second)
				for time.Now().Before(deadline) {
					observed, err = readWatchdogPIDs(pidfile)
					if err == nil {
						break
					}
					time.Sleep(10 * time.Millisecond)
				}
				if err != nil {
					t.Fatalf("watchdog PID record unavailable: %v; output=%s", err, output.String())
				}
				if err := waitProcessesGone(observed, time.Now().Add(2*time.Second)); err != nil {
					t.Fatal(err)
				}
				t.Log("ASSERT_TESTMAIN_PARENT_WATCHDOG_TIMEOUT_CLEANUP: PASS")
				return
			}
			if err != nil {
				t.Fatalf("driver %s failed: %v output=%s", tc.mode, err, output.String())
			}
			observed, err = readWatchdogPIDs(pidfile)
			if err != nil {
				t.Fatalf("driver %s PID record unavailable: %v", tc.mode, err)
			}
			if err := waitProcessesGone(observed, time.Now().Add(2*time.Second)); err != nil {
				t.Fatal(err)
			}
			t.Logf("ASSERT_TESTMAIN_PARENT_WATCHDOG_%s_ZERO_SENTINELS: PASS", strings.ToUpper(tc.mode))
		})
	}
}

func TestSampledAggregateRSSLimit(t *testing.T) {
	withSupervisorDefaults(t)
	supervisorRSSBytes = 1
	t.Setenv("PROGRAMC_TEST_WORKER", "hang")
	_, _, failed := ComputeSupervised(context.Background(), nil, 1)
	requireCode(t, failed, CodeMemoryLimit)
}
func TestMemoryObservationFailsClosed(t *testing.T) {
	for _, tc := range []struct{ name, body string }{{"malformed", "#!/bin/sh\necho malformed\n"}, {"unavailable", "#!/bin/sh\nexit 1\n"}} {
		t.Run(tc.name, func(t *testing.T) {
			withSupervisorDefaults(t)
			path := t.TempDir() + "/ps"
			if err := os.WriteFile(path, []byte(tc.body), 0700); err != nil {
				t.Fatal(err)
			}
			supervisorPSPath = path
			t.Setenv("PROGRAMC_TEST_WORKER", "hang")
			_, _, failed := ComputeSupervised(context.Background(), nil, 1)
			requireCode(t, failed, CodeMemoryUnsupported)
		})
	}
}
func TestTreeRSSAggregatesDescendants(t *testing.T) {
	withSupervisorDefaults(t)
	path := t.TempDir() + "/ps"
	body := "#!/bin/sh\nprintf '100 1 3\\n101 100 5\\n102 101 7\\n200 1 99\\n'\n"
	if err := os.WriteFile(path, []byte(body), 0700); err != nil {
		t.Fatal(err)
	}
	supervisorPSPath = path
	got, err := treeRSS(100)
	if err != nil || got != 15*1024 {
		t.Fatalf("ASSERT_DESCENDANT_RSS_AGGREGATION got=%d err=%v", got, err)
	}
	t.Log("ASSERT_DESCENDANT_RSS_AGGREGATION: PASS")
}
func TestWorkerResponseStrictlyRejectsExtraJSON(t *testing.T) {
	var response workerResponse
	err := strictDecode(strings.NewReader(`{"version":"`+workerVersion+`"}{}`), 1024, &response)
	if err == nil {
		t.Fatal("ASSERT_WORKER_OUTPUT_STRICT")
	}
	t.Log("ASSERT_WORKER_OUTPUT_STRICT: PASS")
}
func TestWorkerFailureWireIsBounded(t *testing.T) {
	response := workerResponse{Version: workerVersion, Failure: failure(CodeNonzeroExit, fmt.Errorf("%s", strings.Repeat("sensitive", 1000)))}
	raw, _ := json.Marshal(response)
	if len(raw) > 2048 {
		t.Fatalf("ASSERT_SENSITIVE_FAILURE_BOUND len=%d", len(raw))
	}
	t.Log("ASSERT_SENSITIVE_FAILURE_BOUND: PASS")
}
