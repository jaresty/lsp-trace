//go:build darwin || linux

package censuslineagepilot_test

import (
	"context"
	"encoding/json"
	pilot "lsp-trace/internal/censuslineagepilot"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestVariableCallerHostLaunchStopJoined(t *testing.T) {
	python, e := exec.LookPath("python3")
	if e != nil {
		t.Fatal(e)
	}
	repo, _ := filepath.Abs("../..")
	helper := filepath.Join(repo, "scripts/private-variable-v2-host-handoff.py")
	c, path, _ := variableAutomaticConfig(t, 1)
	base := filepath.Dir(path)
	control := filepath.Join(base, "control")
	if e = os.Mkdir(control, 0700); e != nil {
		t.Fatal(e)
	}
	worker := filepath.Join(base, "owned-worker")
	script := []byte("#!/bin/sh\nexec /bin/sleep 30\n")
	if e = os.WriteFile(worker, script, 0700); e != nil {
		t.Fatal(e)
	}
	raw, _ := os.ReadFile(path)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	out, e := exec.CommandContext(ctx, python, helper, "launch", "--config", path, "--config-sha256", pilot.Digest(raw), "--worker", worker, "--worker-sha256", pilot.Digest(script), "--control", control).CombinedOutput()
	if e != nil {
		t.Fatalf("launch: %v %s", e, out)
	}
	waitFile := func(name string) []byte {
		t.Helper()
		for {
			raw, e := os.ReadFile(filepath.Join(control, name))
			if e == nil {
				return raw
			}
			select {
			case <-ctx.Done():
				t.Fatalf("supervisor did not produce %s", name)
			case <-time.After(10 * time.Millisecond):
			}
		}
	}
	waitFile("worker.json")
	out, e = exec.CommandContext(ctx, python, helper, "stop", "--control", control).CombinedOutput()
	if e != nil {
		t.Fatalf("stop: %v %s", e, out)
	}
	var status struct {
		ExitCode        int
		Stopped, Joined bool
	}
	if e = json.Unmarshal(waitFile("exit.json"), &status); e != nil || !status.Stopped || !status.Joined || status.ExitCode == 0 {
		t.Fatal("HOST_STOP_JOINED: FAIL", e, status)
	}
	if _, e = os.Stat(c.AttemptMarkerPath); !os.IsNotExist(e) {
		t.Fatal("test stand-in unexpectedly acquired")
	}
	t.Log("HOST_STOP_JOINED: PASS background supervisor terminated and joined its owned child")
}
