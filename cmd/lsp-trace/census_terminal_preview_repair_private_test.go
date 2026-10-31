//go:build darwin || linux

package main

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	pilot "lsp-trace/internal/censuslineagepilot"
)

// Real ordinary CLI metadata output, never a private Go scope/hash constructor.
// It returns a path to exact canonical scope bytes; it does not author approval.
func terminalScopePreviewCLIPrivate(t *testing.T, x terminalProcessRunnerPrivate) (string, string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	args := append(append([]string{"census"}, x.args...), "--private-inventory-scope-preview")
	cmd := exec.CommandContext(ctx, x.binary, args...)
	cmd.Env = append(os.Environ(), "LSP_TRACE_OWNED_TWO_ORIGIN_SERVER=1", "LSP_TRACE_OWNED_TERMINAL_SERVER=1")
	out, e := cmd.Output()
	code := -1
	if cmd.ProcessState != nil {
		code = cmd.ProcessState.ExitCode()
	}
	var meta struct{ Status, AttemptDirectory, ScopeSHA256 string }
	decode := json.Unmarshal(out, &meta)
	wait := struct {
		ExitCode, ProcessID int
		Joined, Stopped     bool
		Operation           string
	}{code, cmd.Process.Pid, cmd.ProcessState != nil, ctx.Err() != nil, "NONDELIVERY_SCOPE_PREVIEW"}
	path := filepath.Join(x.base, "preview-wait-"+time.Now().UTC().Format("150405.000000000")+".json")
	if we := writeCensusInventoryFilePrivate(path, wait); we != nil {
		t.Fatal(we)
	}
	t.Logf("ACTUAL_PREVIEW_WAIT exit=%d pid=%d joined=%t stopped=%t result=%s error=%v", code, wait.ProcessID, wait.Joined, wait.Stopped, out, e)
	ok := e == nil && decode == nil && wait.Joined && !wait.Stopped && meta.Status == "SCOPE_PREVIEW_METADATA_ONLY_NOT_GENERATION" && meta.ScopeSHA256 != "" && meta.AttemptDirectory != ""
	assertInventoryPrivate(t, "ASSERT_SIX_PREVIEW_CLI_CANONICAL_NONSPENDING", ok)
	if !ok {
		t.FailNow()
	}
	raw, e := os.ReadFile(filepath.Join(meta.AttemptDirectory, "scope.json"))
	if e != nil {
		t.Fatal(e)
	}
	if pilot.Digest(raw) != meta.ScopeSHA256 {
		t.Fatal("preview canonical bytes digest")
	}
	var report censusInventoryReportPrivate
	b, e := os.ReadFile(filepath.Join(meta.AttemptDirectory, "report.json"))
	if e != nil || json.Unmarshal(b, &report) != nil {
		t.Fatal("preview report", e)
	}
	if report.StopCalls != 0 || len(report.Origins) != 1 || report.Origins[0].Session.SessionID != "" || (len(report.Origins[0].Discovery) != 0 && string(report.Origins[0].Discovery) != "null") || report.ApplicationBinding.ClaimSHA256 != "" || report.Outcome != nil || report.Inventory != nil || report.Configuration.Automation != nil {
		t.Fatalf("preview started or minted runtime/model/claim authority stop=%d origins=%d session=%q discovery=%d claim=%q outcome=%t inventory=%t automation=%t", report.StopCalls, len(report.Origins), report.Origins[0].Session.SessionID, len(report.Origins[0].Discovery), report.ApplicationBinding.ClaimSHA256, report.Outcome != nil, report.Inventory != nil, report.Configuration.Automation != nil)
	}
	t.Logf("PREVIEW_SCOPE digest=%s bytes=%d path=%s plan=%s segment=%d no_claim=true no_session=true no_model=true", meta.ScopeSHA256, len(raw), filepath.Join(meta.AttemptDirectory, "scope.json"), report.ApplicationBinding.FullPlanSHA256, report.ApplicationBinding.Segment)
	return meta.ScopeSHA256, report.ApplicationBinding.FullPlanSHA256
}
func TestCensusTerminalSixPreviewDriftPrivate(t *testing.T) {
	for _, kind := range []string{"source", "config", "control", "runtime", "prefix"} {
		t.Run(kind, func(t *testing.T) {
			base, src, path, args := terminalApplicationFixturePrivate(t)
			if kind != "prefix" {
				for _, name := range []string{"a.go", "b.go"} {
					if e := os.Remove(filepath.Join(src, name)); e != nil {
						t.Fatal(e)
					}
				}
			}
			x := newTerminalProcessRunnerPrivate(t, base, args)
			if kind == "prefix" {
				r, code, pid := x.run()
				if code != 0 {
					t.Fatal("prefix control")
				}
				applicationHostObservationPrivate(t, r, code, pid, true, false)
			}
			terminalScopePreviewCLIPrivate(t, x)
			switch kind {
			case "source":
				if e := os.WriteFile(filepath.Join(src, "c.go"), []byte("package owned\n\nfunc Changed() {}\n"), 0600); e != nil {
					t.Fatal(e)
				}
			case "config":
				c, e := loadCensusInventoryConfigPrivate(path)
				if e != nil {
					t.Fatal(e)
				}
				c.Seeds = []uint64{44, 45}
				saveInventoryConfigPrivate(t, path, c)
			case "control":
				x.args = append(x.args, "--exclude", "never-matches")
			case "runtime":
				x.args = append(x.args, "--server-arg", "changed-runtime-control")
			case "prefix":
				c, e := loadCensusInventoryConfigPrivate(path)
				if e != nil {
					t.Fatal(e)
				}
				p := filepath.Join(c.Application.Directory, censusApplicationLeafPrivate(0, "host"))
				if e = os.Remove(p); e != nil {
					t.Fatal(e)
				}
			}
			r, code, _ := x.run()
			assertInventoryPrivate(t, "ASSERT_SIX_PREVIEW_DRIFT_"+kind, code == 1 && r.StopCalls == 0 && r.ApplicationBinding != nil && r.ApplicationBinding.ClaimSHA256 == "" && r.Outcome == nil && r.Inventory == nil)
		})
	}
}

func TestCensusTerminalSixPreviewFreshNormalPrivate(t *testing.T) {
	base, src, path, args := terminalApplicationFixturePrivate(t)
	for _, name := range []string{"a.go", "b.go"} {
		if e := os.Remove(filepath.Join(src, name)); e != nil {
			t.Fatal(e)
		}
	}
	x := newTerminalProcessRunnerPrivate(t, base, args)
	// Supported delivery control first, in a separate state, so no generation of
	// the fresh normal scope is spent merely to discover its approval locator.
	controlBase, _, _, controlArgs := terminalApplicationFixturePrivate(t)
	control := newTerminalProcessRunnerPrivate(t, controlBase, controlArgs)
	r, code, _ := control.run()
	if code != 0 || r.Status != censusTerminalSegmentStatusPrivate {
		t.Fatal("delivery control")
	}
	t.Log("ASSERT_SIX_PREVIEW_DELIVERY_CONTROL PASS")
	hash, plan := terminalScopePreviewCLIPrivate(t, x)
	again, againPlan := terminalScopePreviewCLIPrivate(t, x)
	assertInventoryPrivate(t, "ASSERT_SIX_PREVIEW_IDEMPOTENT", hash == again && plan == againPlan)
	c, e := loadCensusInventoryConfigPrivate(path)
	if e != nil {
		t.Fatal(e)
	}
	names, e := os.ReadDir(c.Application.Directory)
	if e != nil {
		t.Fatal(e)
	}
	for _, n := range names {
		if len(n.Name()) >= 8 && n.Name()[:8] == "segment-" {
			t.Fatal("preview spent generation", n.Name())
		}
	}
	// Ordinary unapproved EXECUTION remains spent, but its scope must match the
	// exact preview. The actual approved successor is covered by the pipeline.
	normal, exit, _ := x.run()
	assertInventoryPrivate(t, "ASSERT_SIX_PREVIEW_EXECUTION_SCOPE_MATCH", exit == 1 && normal.ScopeSHA256 == hash && normal.ApplicationBinding.FullPlanSHA256 == plan && normal.ApplicationBinding.ClaimSHA256 != "")
}
