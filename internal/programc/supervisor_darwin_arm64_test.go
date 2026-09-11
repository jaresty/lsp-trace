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
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"lsp-trace/internal/graph"
)

func TestMain(m *testing.M) {
	if len(os.Args) == 2 && os.Args[1] == workerArgument {
		if os.Getenv("PROGRAMC_TEST_WORKER") != "" {
			testWorkerMode()
			return
		}
		_, code := RunPrivateWorker(os.Args[1:], os.Stdin, os.Stdout, os.Stderr)
		os.Exit(code)
	}
	os.Exit(m.Run())
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
