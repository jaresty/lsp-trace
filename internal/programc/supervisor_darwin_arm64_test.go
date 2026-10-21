//go:build darwin && arm64

package programc

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
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

const testWorkerWatchdogEnvironment = "PROGRAMC_TEST_PARENT_WATCHDOG"

func TestMain(m *testing.M) {
	if len(os.Args) == 2 && os.Args[1] == workerArgument {
		if mode := os.Getenv("PROGRAMC_TEST_WORKER"); mode != "" {
			if os.Getenv(testWorkerWatchdogEnvironment) == "1" {
				startTestWorkerParentWatchdog(os.Getppid())
			}
			if err := writeTestWorkerPIDRecord(processIdentity{}); err != nil {
				fmt.Fprintf(os.Stderr, "record test worker identity: %v\n", err)
				os.Exit(12)
			}
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
	if os.Getenv("PROGRAMC_TEST_WATCHDOG_DRIVER") == "timeout" {
		if err := prepareTimeoutWatchdogDriver(); err != nil {
			fmt.Fprintf(os.Stderr, "prepare timeout watchdog driver: %v\n", err)
			os.Exit(13)
		}
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
			self := os.Getpid()
			pgid, err := syscall.Getpgid(self)
			if err != nil || pgid != self {
				// A direct invocation may share an unrelated caller's process
				// group. Never use a negative PID unless this worker owns its group.
				os.Exit(126)
			}
			// kill(2) selects every member before SIGKILL terminates this process.
			_ = syscall.Kill(-pgid, syscall.SIGKILL)
			os.Exit(125)
		}
	}()
}

type processIdentity struct {
	PID   int    `json:"pid"`
	PGID  int    `json:"pgid"`
	Start string `json:"start"`
}

type watchdogPIDs struct {
	Worker     processIdentity `json:"worker"`
	Descendant processIdentity `json:"descendant"`
}

func readProcessIdentity(pid int) (processIdentity, error) {
	if pid <= 1 {
		return processIdentity{}, fmt.Errorf("unsafe pid %d", pid)
	}
	out, err := exec.Command("/bin/ps", "-o", "pgid=", "-o", "lstart=", "-p", strconv.Itoa(pid)).Output()
	if err != nil {
		return processIdentity{}, err
	}
	fields := strings.Fields(string(out))
	if len(fields) < 6 {
		return processIdentity{}, fmt.Errorf("incomplete process identity %q", out)
	}
	pgid, err := strconv.Atoi(fields[0])
	if err != nil || pgid <= 1 {
		return processIdentity{}, fmt.Errorf("invalid process group %q", fields[0])
	}
	return processIdentity{PID: pid, PGID: pgid, Start: strings.Join(fields[1:], " ")}, nil
}

func sameProcessIdentity(recorded processIdentity) bool {
	current, err := readProcessIdentity(recorded.PID)
	return err == nil && current == recorded
}

func writeTestWorkerPIDRecord(descendant processIdentity) error {
	path := os.Getenv("PROGRAMC_TEST_PIDFILE")
	if path == "" {
		return nil
	}
	worker, err := readProcessIdentity(os.Getpid())
	if err != nil {
		return err
	}
	record := watchdogPIDs{Worker: worker, Descendant: descendant}
	temporary := path + ".tmp"
	file, err := os.OpenFile(temporary, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	if err = json.NewEncoder(file).Encode(record); err == nil {
		err = file.Sync()
	}
	closeErr := file.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	if err = os.Rename(temporary, path); err != nil {
		return err
	}
	directory, err := os.Open(filepath.Dir(path))
	if err != nil {
		return err
	}
	err = directory.Sync()
	closeErr = directory.Close()
	if err == nil {
		err = closeErr
	}
	return err
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
		descendant, err := readProcessIdentity(cmd.Process.Pid)
		if err != nil || descendant.PGID != os.Getpid() {
			_ = cmd.Process.Kill()
			fmt.Fprintf(os.Stderr, "invalid watchdog descendant identity: %+v err=%v\n", descendant, err)
			os.Exit(11)
		}
		if err := writeTestWorkerPIDRecord(descendant); err != nil {
			_ = cmd.Process.Kill()
			fmt.Fprintf(os.Stderr, "record watchdog readiness: %v\n", err)
			os.Exit(12)
		}
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
	t.Setenv(testWorkerWatchdogEnvironment, "1")
	oldWall, oldRSS, oldPoll, oldOutput, oldPS := supervisorWall, supervisorRSSBytes, supervisorPoll, supervisorOutputBytes, supervisorPSPath
	oldKill, oldState, oldNow, oldWait, oldStart := supervisorGroupKill, supervisorGroupState, supervisorCleanupNow, supervisorCleanupWait, supervisorProcessStart
	t.Cleanup(func() {
		supervisorWall, supervisorRSSBytes, supervisorPoll, supervisorOutputBytes, supervisorPSPath = oldWall, oldRSS, oldPoll, oldOutput, oldPS
		supervisorGroupKill, supervisorGroupState, supervisorCleanupNow, supervisorCleanupWait, supervisorProcessStart = oldKill, oldState, oldNow, oldWait, oldStart
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

func TestTerminateWorkerGroupConfirmsLiveDescendantGone(t *testing.T) {
	withSupervisorDefaults(t)
	kills := 0
	supervisorGroupKill = func(pid int, signal syscall.Signal) error {
		if pid != -42 || signal != syscall.SIGKILL {
			t.Fatalf("unrelated signal target pid=%d signal=%v", pid, signal)
		}
		kills++
		return nil
	}
	observations := 0
	supervisorGroupState = func(int) ([]processGroupMember, error) {
		observations++
		if observations == 1 {
			return []processGroupMember{{PID: 43, PGID: 42, State: "S"}}, nil
		}
		return nil, nil
	}
	done := make(chan error, 1)
	done <- nil
	if err := terminateWorkerGroup(processGroupIdentity{PID: 42, PGID: 42, Start: "start"}, done); err != nil {
		t.Fatal(err)
	}
	if kills != 1 || observations != 2 {
		t.Fatalf("ASSERT_GROUP_LIVE_THEN_GONE kills=%d observations=%d", kills, observations)
	}
	t.Log("ASSERT_GROUP_LIVE_THEN_GONE: PASS")
}

func TestTerminateWorkerGroupTerminalAndFailureStates(t *testing.T) {
	identity := processGroupIdentity{PID: 42, PGID: 42, Start: "start"}
	for _, tc := range []struct {
		name     string
		members  []processGroupMember
		stateErr error
	}{
		{name: "group-gone"},
		{name: "zombie-only", members: []processGroupMember{{PID: 43, PGID: 42, State: "Z"}}},
		{name: "ps-malformed", stateErr: errors.New("malformed /bin/ps process-group output")},
		{name: "ps-unavailable", stateErr: errors.New("ps unavailable")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			withSupervisorDefaults(t)
			kills := 0
			supervisorGroupKill = func(int, syscall.Signal) error { kills++; return nil }
			supervisorGroupState = func(int) ([]processGroupMember, error) { return tc.members, tc.stateErr }
			err := terminateWorkerGroup(identity, nil)
			if (tc.stateErr == nil && err != nil) || (tc.stateErr != nil && err == nil) || kills != 0 {
				t.Fatalf("ASSERT_GROUP_TERMINAL_%s err=%v kills=%d", tc.name, err, kills)
			}
			t.Logf("ASSERT_GROUP_TERMINAL_%s: PASS", tc.name)
		})
	}
}

func TestTerminateWorkerGroupPersistentLiveIsBounded(t *testing.T) {
	withSupervisorDefaults(t)
	supervisorProcessStart = func(int) (string, error) { return "start", nil }
	now := time.Unix(0, 0)
	supervisorCleanupNow = func() time.Time { return now }
	supervisorCleanupWait = func(d time.Duration) { now = now.Add(d) }
	supervisorGroupState = func(int) ([]processGroupMember, error) {
		return []processGroupMember{{PID: 42, PGID: 42, State: "S", Start: "start"}}, nil
	}
	kills := 0
	supervisorGroupKill = func(pid int, _ syscall.Signal) error {
		if pid != -42 {
			t.Fatalf("unrelated pid %d", pid)
		}
		kills++
		return nil
	}
	err := terminateWorkerGroup(processGroupIdentity{PID: 42, PGID: 42, Start: "start"}, nil)
	if err == nil || kills == 0 || now.Sub(time.Unix(0, 0)) > time.Second+supervisorPoll {
		t.Fatalf("ASSERT_GROUP_PERSISTENT_LIVE_BOUNDED err=%v kills=%d elapsed=%s", err, kills, now.Sub(time.Unix(0, 0)))
	}
	t.Log("ASSERT_GROUP_PERSISTENT_LIVE_BOUNDED: PASS")
}

func TestTerminateWorkerGroupRejectsChangedIdentityWithoutSignal(t *testing.T) {
	withSupervisorDefaults(t)
	supervisorProcessStart = func(int) (string, error) { return "reused", nil }
	supervisorGroupState = func(int) ([]processGroupMember, error) {
		return []processGroupMember{{PID: 42, PGID: 42, State: "S", Start: "reused"}}, nil
	}
	kills := 0
	supervisorGroupKill = func(int, syscall.Signal) error { kills++; return nil }
	err := terminateWorkerGroup(processGroupIdentity{PID: 42, PGID: 42, Start: "original"}, nil)
	if err == nil || kills != 0 {
		t.Fatalf("ASSERT_GROUP_REUSED_IDENTITY_NOT_SIGNALED err=%v kills=%d", err, kills)
	}
	t.Log("ASSERT_GROUP_REUSED_IDENTITY_NOT_SIGNALED: PASS")
}

func TestTerminateWorkerGroupESRCHIsComplete(t *testing.T) {
	withSupervisorDefaults(t)
	supervisorProcessStart = func(int) (string, error) { return "start", nil }
	supervisorGroupState = func(int) ([]processGroupMember, error) {
		return []processGroupMember{{PID: 42, PGID: 42, State: "S", Start: "start"}}, nil
	}
	supervisorGroupKill = func(int, syscall.Signal) error { return syscall.ESRCH }
	if err := terminateWorkerGroup(processGroupIdentity{PID: 42, PGID: 42, Start: "start"}, nil); err != nil {
		t.Fatal(err)
	}
	t.Log("ASSERT_GROUP_ESRCH_COMPLETE: PASS")
}

func readWatchdogPIDs(path string, requireDescendant bool) (watchdogPIDs, error) {
	var p watchdogPIDs
	file, err := os.Open(path)
	if err != nil {
		return p, err
	}
	err = json.NewDecoder(file).Decode(&p)
	closeErr := file.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return p, err
	}
	if p.Worker.PID <= 1 || p.Worker.PGID <= 1 || p.Worker.Start == "" {
		return p, fmt.Errorf("invalid worker identity %+v", p.Worker)
	}
	if requireDescendant && (p.Descendant.PID <= 1 || p.Descendant.PGID != p.Worker.PID || p.Descendant.Start == "") {
		return p, fmt.Errorf("descendant not ready %+v", p.Descendant)
	}
	return p, nil
}

func waitForWorkerRecord(path string, requireDescendant bool, deadline time.Time) (watchdogPIDs, error) {
	var last error
	for time.Now().Before(deadline) {
		p, err := readWatchdogPIDs(path, requireDescendant)
		if err == nil && sameProcessIdentity(p.Worker) && (!requireDescendant || sameProcessIdentity(p.Descendant)) {
			return p, nil
		}
		last = err
		time.Sleep(10 * time.Millisecond)
	}
	return watchdogPIDs{}, fmt.Errorf("watchdog identity record unavailable: %w", last)
}

func waitForWatchdogReadiness(path string, deadline time.Time) (watchdogPIDs, error) {
	p, err := waitForWorkerRecord(path, true, deadline)
	if err != nil {
		return p, err
	}
	if p.Worker.PGID != p.Worker.PID || p.Descendant.PID == 0 || p.Descendant.PGID != p.Worker.PID {
		return watchdogPIDs{}, fmt.Errorf("invalid watchdog readiness %+v", p)
	}
	return p, nil
}

func recordedIdentityGone(recorded processIdentity) bool {
	if recorded.PID == 0 {
		return true
	}
	if !sameProcessIdentity(recorded) {
		return true
	}
	return processIsZombie(recorded.PID)
}

func signalRecordedProcess(recorded processIdentity) {
	if sameProcessIdentity(recorded) {
		_ = syscall.Kill(recorded.PID, syscall.SIGKILL)
	}
}

func signalRecordedWorkerGroup(recorded watchdogPIDs) {
	workerOwnsGroup := sameProcessIdentity(recorded.Worker) && recorded.Worker.PGID == recorded.Worker.PID
	descendantAnchorsGroup := sameProcessIdentity(recorded.Descendant) && recorded.Descendant.PGID == recorded.Worker.PID
	if workerOwnsGroup || descendantAnchorsGroup {
		_ = syscall.Kill(-recorded.Worker.PID, syscall.SIGKILL)
	}
}

func waitProcessesGone(p watchdogPIDs, deadline time.Time) error {
	for time.Now().Before(deadline) {
		if recordedIdentityGone(p.Worker) && recordedIdentityGone(p.Descendant) {
			return nil
		}
		time.Sleep(10 * time.Millisecond)
	}
	return fmt.Errorf("recorded identities remain: worker=%+v current=%t descendant=%+v current=%t", p.Worker, sameProcessIdentity(p.Worker), p.Descendant, sameProcessIdentity(p.Descendant))
}

func prepareTimeoutWatchdogDriver() error {
	supervisorWall = 30 * time.Second
	supervisorRSSBytes = 1 << 40
	supervisorPoll = 5 * time.Millisecond
	if err := os.Setenv("PROGRAMC_TEST_WORKER", "watchdog-child"); err != nil {
		return err
	}
	if err := os.Setenv(testWorkerWatchdogEnvironment, "1"); err != nil {
		return err
	}
	go func() { _, _, _ = ComputeSupervised(context.Background(), nil, 1) }()
	p, err := waitForWatchdogReadiness(os.Getenv("PROGRAMC_TEST_PIDFILE"), time.Now().Add(5*time.Second))
	if err != nil {
		return err
	}
	fmt.Printf("WATCHDOG_READY worker=%d descendant=%d\n", p.Worker.PID, p.Descendant.PID)
	return nil
}

func TestWatchdogDriver(t *testing.T) {
	mode := os.Getenv("PROGRAMC_TEST_WATCHDOG_DRIVER")
	if mode == "" {
		t.Skip("subprocess driver")
	}
	withSupervisorDefaults(t)
	switch mode {
	case "timeout":
		select {}
	case "failure":
		requireCode(t, supervisedFailure(t, "nonzero", context.Background()), CodeNonzeroExit)
	case "success":
		supervisorWall = 10 * time.Second
		t.Setenv("PROGRAMC_TEST_WORKER", "delayed-private")
		input := validV5(t, nil, nil)
		if _, _, failed := ComputeSupervised(context.Background(), input, 1); failed != nil {
			t.Fatal(failed)
		}
	case "nonleader-parent":
		cmd := exec.Command(os.Args[0], workerArgument)
		cmd.Env = append(os.Environ(), "PROGRAMC_TEST_WORKER=hang", testWorkerWatchdogEnvironment+"=1")
		cmd.Stdin = strings.NewReader("")
		if err := cmd.Start(); err != nil {
			t.Fatal(err)
		}
		p, err := waitForWorkerRecord(os.Getenv("PROGRAMC_TEST_PIDFILE"), false, time.Now().Add(5*time.Second))
		if err != nil || p.Worker.PGID == p.Worker.PID {
			t.Fatalf("expected non-leader worker identity, got %+v err=%v", p.Worker, err)
		}
	default:
		t.Fatalf("unknown watchdog driver mode %q", mode)
	}
}

func TestWorkerParentWatchdogRejectsNonLeaderInvocation(t *testing.T) {
	pidfile := filepath.Join(t.TempDir(), "pids")
	cmd := exec.Command(os.Args[0], "-test.run=^TestWatchdogDriver$", "-test.timeout=5s")
	cmd.Env = append(os.Environ(), "PROGRAMC_TEST_WATCHDOG_DRIVER=nonleader-parent", "PROGRAMC_TEST_PIDFILE="+pidfile)
	if err := cmd.Run(); err != nil {
		t.Fatalf("non-leader parent driver failed: %v", err)
	}
	p, err := readWatchdogPIDs(pidfile, false)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { signalRecordedProcess(p.Worker) })
	if p.Worker.PGID == p.Worker.PID {
		t.Fatalf("adversarial worker unexpectedly led group: %+v", p.Worker)
	}
	if err := waitProcessesGone(p, time.Now().Add(2*time.Second)); err != nil {
		t.Fatal(err)
	}
	t.Log("ASSERT_TESTMAIN_PARENT_WATCHDOG_NONLEADER_SAFE_EXIT: PASS")
}

func TestWatchdogCleanupRejectsChangedProcessIdentity(t *testing.T) {
	cmd := exec.Command("/bin/sleep", "30")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	original, err := readProcessIdentity(cmd.Process.Pid)
	if err != nil || original.PGID != original.PID {
		_ = cmd.Process.Kill()
		t.Fatalf("invalid isolated identity %+v: %v", original, err)
	}
	t.Cleanup(func() { signalRecordedWorkerGroup(watchdogPIDs{Worker: original}) })
	reused := original
	reused.Start += " reused"
	if !recordedIdentityGone(reused) {
		t.Fatalf("changed start identity was not treated as gone: %+v", reused)
	}
	signalRecordedWorkerGroup(watchdogPIDs{Worker: reused})
	if !sameProcessIdentity(original) {
		t.Fatal("identity-mismatched cleanup signaled the live original process")
	}
	t.Log("ASSERT_TESTMAIN_PARENT_WATCHDOG_REUSED_IDENTITY_NOT_SIGNALED: PASS")
}

func TestWorkerParentWatchdogPreventsTimeoutOrphans(t *testing.T) {
	for _, tc := range []struct {
		mode        string
		testTimeout time.Duration
		wantTimeout bool
	}{{"timeout", 350 * time.Millisecond, true}, {"failure", 5 * time.Second, false}, {"success", 8 * time.Second, false}} {
		t.Run(tc.mode, func(t *testing.T) {
			pidfile := filepath.Join(t.TempDir(), "pids")
			cmd := exec.Command(os.Args[0], "-test.run=^TestWatchdogDriver$", "-test.timeout="+tc.testTimeout.String())
			cmd.Env = append(os.Environ(), "PROGRAMC_TEST_WATCHDOG_DRIVER="+tc.mode, "PROGRAMC_TEST_PIDFILE="+pidfile)
			cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
			var output bytes.Buffer
			cmd.Stdout, cmd.Stderr = &output, &output
			if err := cmd.Start(); err != nil {
				t.Fatal(err)
			}
			driver, err := readProcessIdentity(cmd.Process.Pid)
			if err != nil || driver.PGID != driver.PID {
				t.Fatalf("invalid driver identity %+v: %v", driver, err)
			}
			var observed watchdogPIDs
			t.Cleanup(func() {
				signalRecordedWorkerGroup(watchdogPIDs{Worker: driver})
				signalRecordedWorkerGroup(observed)
			})
			err = cmd.Wait()
			if tc.wantTimeout {
				if err == nil || !strings.Contains(output.String(), "WATCHDOG_READY") || !strings.Contains(output.String(), "test timed out") {
					t.Fatalf("expected readiness followed by test timeout, err=%v output=%s", err, output.String())
				}
				observed, err = readWatchdogPIDs(pidfile, true)
				if err != nil {
					t.Fatalf("watchdog readiness record unavailable: %v; output=%s", err, output.String())
				}
				if observed.Descendant.PID == 0 || observed.Descendant.PGID != observed.Worker.PID {
					t.Fatalf("invalid descendant readiness %+v", observed)
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
			observed, err = readWatchdogPIDs(pidfile, false)
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
