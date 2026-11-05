//go:build darwin && arm64

package programc

import (
	"encoding/json"
	"errors"
	"os"
	"sync"
	"syscall"
	"testing"
	"time"
)

// Test-only bridge: this file is absent from ordinary programc imports/binaries.
// It observes existing fixed lifecycle notifications and never replaces work,
// report validation, authorization, completion or the production runner.
type TypedOwnerTraceForTest struct{ Delivered, Reaped, BeforeVerify, AfterVerify int }

func TypedOwnerStageBundleForTest(t *testing.T) GroupingCaseBundlePrivate {
	return typedStageBundlePrivateTest(t)
}

func VerifyTypedOwnerGoneForTest(t *testing.T, path string) {
	t.Helper()
	raw, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	var p watchdogPIDs
	if e = json.Unmarshal(raw, &p); e != nil {
		t.Fatal(e)
	}
	if sameProcessIdentity(p.Worker) {
		t.Fatal("ASSERT_TYPED_OWNER_REAL_WORKER_GONE")
	}
}

// Only a scoped TEST helper alters the existing kill seam. Fixed production work
// still runs. A verified recorded direct child is killed/reaped by this helper on
// exit, explicitly OUTSIDE the supervisor observation: no false supervisor reap.
func WithTypedOwnerCleanupFaultForTest(t *testing.T, pidfile string, run func()) {
	t.Helper()
	prior := supervisorGroupKill
	supervisorGroupKill = func(pid int, sig syscall.Signal) error { return errors.New("controlled kill uncertainty") }
	defer func() {
		supervisorGroupKill = prior
		raw, e := os.ReadFile(pidfile)
		if e != nil {
			t.Fatal(e)
		}
		var p watchdogPIDs
		if e = json.Unmarshal(raw, &p); e != nil {
			t.Fatal(e)
		}
		if !sameProcessIdentity(p.Worker) || p.Worker.PGID != p.Worker.PID {
			t.Fatal("test outer cleanup identity unavailable")
		}
		if e = syscall.Kill(-p.Worker.PGID, syscall.SIGKILL); e != nil {
			t.Fatal(e)
		}
		process, e := os.FindProcess(p.Worker.PID)
		if e != nil {
			t.Fatal(e)
		}
		done := make(chan error, 1)
		go func() { _, e := process.Wait(); done <- e }()
		select {
		case e = <-done:
			if e != nil {
				t.Fatal(e)
			}
		case <-time.After(3 * time.Second):
			t.Fatal("test outer cleanup reap timeout")
		}
		VerifyTypedOwnerGoneForTest(t, pidfile)
	}()
	run()
}

func ObserveTypedOwnerForTest(t *testing.T) func() TypedOwnerTraceForTest {
	t.Helper()
	var mu sync.Mutex
	var trace TypedOwnerTraceForTest
	prior := supervisorObserverPrivate
	supervisorObserverPrivate = func(s supervisorStagePrivate) {
		mu.Lock()
		defer mu.Unlock()
		switch s {
		case supervisorDeliveryFinishedPrivate:
			trace.Delivered++
		case supervisorReapedPrivate:
			trace.Reaped++
		case supervisorBeforeVerifyPrivate:
			trace.BeforeVerify++
		case supervisorAfterVerifyPrivate:
			trace.AfterVerify++
		}
	}
	t.Cleanup(func() { supervisorObserverPrivate = prior })
	return func() TypedOwnerTraceForTest { mu.Lock(); defer mu.Unlock(); return trace }
}
