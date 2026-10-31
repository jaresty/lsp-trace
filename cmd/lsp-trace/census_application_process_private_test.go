//go:build darwin || linux

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	pilot "lsp-trace/internal/censuslineagepilot"
	prep "lsp-trace/internal/privatedescribeprep"
)

func applicationFixturePrivate(t *testing.T) (string, string, string, []string) {
	t.Helper()
	base, src, config, args := inventoryTwoFixturePrivate(t)
	for i := 0; i < 62; i++ {
		if e := os.WriteFile(filepath.Join(src, fmt.Sprintf("eligible-%03d.go", i)), []byte("package owned\n\nfunc A0() {}\n"), 0600); e != nil {
			t.Fatal(e)
		}
	}
	directory := filepath.Join(base, "application")
	if e := os.Mkdir(directory, 0700); e != nil {
		t.Fatal(e)
	}
	c, e := loadCensusInventoryConfigPrivate(config)
	if e != nil {
		t.Fatal(e)
	}
	c.Application = &censusApplicationConfigPrivate{censusApplicationProfilePrivate, directory, 1 << 20, 2 << 20, 128 << 20, 128}
	saveInventoryConfigPrivate(t, config, c)
	return base, src, config, args
}
func applicationHostObservationPrivate(t *testing.T, r censusInventoryReportPrivate, exit, pid int, joined, stopped bool) {
	t.Helper()
	b := r.ApplicationBinding
	if b == nil {
		t.Fatal("application binding absent")
	}
	directory := r.Configuration.Application.Directory
	result, e := os.ReadFile(filepath.Join(directory, censusApplicationLeafPrivate(b.Segment, "result")))
	if e != nil {
		t.Fatal(e)
	}
	report, e := os.ReadFile(filepath.Join(r.AttemptDirectory, "report.json"))
	if e != nil {
		t.Fatal(e)
	}
	host := censusApplicationHostObservationPrivate{Schema: "lsp-trace.census-application-host-process.private.v1", Trust: censusApplicationHostTrustPrivate, FullPlanSHA256: b.FullPlanSHA256, Segment: b.Segment, ProcessID: pid, ExitCode: exit, Joined: joined, Stopped: stopped, AttemptDirectory: r.AttemptDirectory, ClaimSHA256: b.ClaimSHA256, ResultSHA256: pilot.Digest(result), ReportSHA256: pilot.Digest(report), ScopeSHA256: r.ScopeSHA256}
	if r.Outcome != nil {
		host.NativeSHA256 = r.Outcome.EnvelopeSHA256
		host.NativeBytes = r.Outcome.ByteLength
	}
	if e := writeCensusInventoryFilePrivate(filepath.Join(directory, censusApplicationLeafPrivate(b.Segment, "host")), host); e != nil {
		t.Fatal(e)
	}
}
func TestCensusApplicationAllSegmentsCLIPrivate(t *testing.T) { runCensusApplicationCLIPrivate(t, -1) }
func TestCensusApplicationFailedLaterCLIPrivate(t *testing.T) { runCensusApplicationCLIPrivate(t, 1) }
func runCensusApplicationCLIPrivate(t *testing.T, failAt int) {
	python, e := exec.LookPath("python3")
	if e != nil {
		t.Fatal(e)
	}
	base, _, config, args := applicationFixturePrivate(t)
	binary := filepath.Join(base, "lsp-trace")
	if out, e := exec.Command("go", "build", "-o", binary, ".").CombinedOutput(); e != nil {
		t.Fatalf("build setup %v %s", e, out)
	}
	for i, a := range args {
		if a == "--server" {
			args[i+1] = os.Args[0]
		}
	}
	args = append(args, "--server-arg", "-test.run=^TestInventoryOwnedLSPProcessPrivate$")
	run := func() (censusInventoryReportPrivate, int, int) {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, binary, append([]string{"census"}, args...)...)
		cmd.Env = append(os.Environ(), "LSP_TRACE_OWNED_TWO_ORIGIN_SERVER=1")
		out, e := cmd.Output()
		code := -1
		if cmd.ProcessState != nil {
			code = cmd.ProcessState.ExitCode()
		}
		if e != nil {
			if x, ok := e.(*exec.ExitError); ok {
				t.Logf("CLI nonzero=%d stderr=%s", code, x.Stderr)
			}
		}
		var meta struct{ AttemptDirectory string }
		if json.Unmarshal(out, &meta) != nil || meta.AttemptDirectory == "" {
			t.Fatalf("process report setup: %v %s", e, out)
		}
		raw, e := os.ReadFile(filepath.Join(meta.AttemptDirectory, "report.json"))
		if e != nil {
			t.Fatal(e)
		}
		var r censusInventoryReportPrivate
		if e = json.Unmarshal(raw, &r); e != nil {
			t.Fatal(e)
		}
		receipt := struct {
			ExitCode, ProcessID int
			Joined, Stopped     bool
		}{code, cmd.Process.Pid, cmd.ProcessState != nil, ctx.Err() != nil}
		if e = writeCensusInventoryFilePrivate(filepath.Join(base, "process-"+filepath.Base(r.AttemptDirectory)+".json"), receipt); e != nil {
			t.Fatal(e)
		}
		t.Logf("ACTUAL_APPLICATION_CLI_WAIT exit=%d joined=%t stopped=%t segment=%v attempt=%s failure=%s", code, receipt.Joined, receipt.Stopped, r.ApplicationBinding, r.AttemptDirectory, r.Failure)
		if !receipt.Joined || receipt.Stopped {
			t.Fatal("owned process completion bound")
		}
		return r, code, cmd.Process.Pid
	}
	c, e := loadCensusInventoryConfigPrivate(config)
	if e != nil {
		t.Fatal(e)
	}
	helper, e := filepath.Abs(filepath.Join("..", "..", "scripts", "private-variable-v2-host-handoff.py"))
	if e != nil {
		t.Fatal(e)
	}
	originalIDs := map[string]bool{}
	totalInputs := 0
	var planID string
	for segment := 0; segment < 5; segment++ {
		c.Automation = nil
		saveInventoryConfigPrivate(t, config, c)
		prepared, code, _ := run()
		if code != 0 || prepared.ApplicationBinding == nil || prepared.ApplicationBinding.Segment != segment {
			t.Fatalf("preparation segment %d: code=%d failure=%s", segment, code, prepared.Failure)
		}
		if segment == 0 {
			planID = prepared.ApplicationBinding.FullPlanSHA256
		} else if planID != prepared.ApplicationBinding.FullPlanSHA256 {
			t.Fatal("full plan changed during progression")
		}
		hostBase := filepath.Join(base, fmt.Sprintf("host-%d", segment))
		if e = os.Mkdir(hostBase, 0700); e != nil {
			t.Fatal(e)
		}
		for _, name := range []string{"mailbox", "exports", "control", "payloads"} {
			if e = os.Mkdir(filepath.Join(hostBase, name), 0700); e != nil {
				t.Fatal(e)
			}
		}
		c.Automation = &prep.AutomaticHandoffConfigPrivateV2{Enabled: true, Directory: filepath.Join(hostBase, "mailbox"), Invocation: fmt.Sprintf("owned-application-segment-%d", segment), InterpreterRole: "owned-interpreter", ReviewerRole: "owned-reviewer", ScopeSHA256: prepared.ScopeSHA256, TimeoutMS: 5000, OverallTimeoutMS: 20000, PollMS: 2, Limits: prep.AutomaticLimitsPrivateV2{MaxCalls: 64, MaxRequestBytes: 4 << 20, MaxResponseBytes: 64 << 10, MaxTotalBytes: 64 << 20, MaxOutputBytes: 8 << 20}}
		c.Automation.ApprovalSHA256 = prep.AutomaticHandoffApprovalPrivateV2(*c.Automation)
		saveInventoryConfigPrivate(t, config, c)
		host := make(chan error, 1)
		exchanges := 2*len(prepared.Index) + 2
		if segment == failAt {
			exchanges -= 2
		} // owned responder intentionally stops before navigation
		go func() {
			host <- inventoryHostEmulatePrivate(hostBase, c.Automation.Directory, helper, python, exchanges)
		}()
		r, code, pid := run()
		hostErr := <-host
		if segment == failAt {
			assertInventoryPrivate(t, "ASSERT_APPLICATION_FAILED_LATER_WAIT", code == 1 && hostErr == nil && r.ShutdownJoined && r.CleanupVerified && r.ApplicationBinding.Segment == segment)
			applicationHostObservationPrivate(t, r, code, pid, true, false)
			c.Automation = nil
			saveInventoryConfigPrivate(t, config, c)
			stopped, stoppedCode, _ := run()
			assertInventoryPrivate(t, "ASSERT_APPLICATION_FAILED_SEGMENT_SPENT_NO_ADVANCE", stoppedCode == 1 && stopped.StopCalls == 0 && stopped.ApplicationBinding.Segment == segment)
			raw, e := os.ReadFile(filepath.Join(stopped.AttemptDirectory, "application-accounting.json"))
			if e != nil {
				t.Fatal(e)
			}
			var view censusApplicationAccountingPrivate
			if json.Unmarshal(raw, &view) != nil {
				t.Fatal("failed accounting")
			}
			assertInventoryPrivate(t, "ASSERT_APPLICATION_FAILED_GAP_ACCOUNTED", len(view.Verified) == 1 && view.GapFiles == 16 && view.Blocked != nil && view.Blocked.AttemptDirectory == r.AttemptDirectory && view.Blocked.BindingStatus == "UNVERIFIED_NO_COMPLETION_CREDIT" && view.FileDenominator == view.CompletedFiles+view.ExcludedFiles+view.GapFiles+view.UnattemptedFiles)
			t.Logf("APPLICATION_FAILED_LATER_EVIDENCE base=%s failed=%s stop=%s", base, r.AttemptDirectory, stopped.AttemptDirectory)
			return
		}
		if code != 0 || hostErr != nil || r.Inventory == nil || r.Outcome == nil || r.ApplicationBinding.Segment != segment || r.ApplicationBinding.ProcessID != pid {
			t.Fatalf("approved segment %d code=%d failure=%s host=%v errors=%v", segment, code, r.Failure, hostErr, r.Errors)
		}
		for _, cov := range r.Coverage {
			if cov.PrivateV2 != nil {
				originalIDs[cov.PrivateV2.OriginID] = true
			}
		}
		totalInputs += len(r.Coverage)
		// Preparation and inner cleanup alone must not advance the next invocation.
		if segment == 0 {
			c.Automation = nil
			saveInventoryConfigPrivate(t, config, c)
			pending, pendingCode, _ := run()
			assertInventoryPrivate(t, "ASSERT_APPLICATION_PENDING_HOST_NO_ADVANCE", pendingCode == 1 && pending.StopCalls == 0 && pending.ApplicationBinding.Segment == 0)
		}
		applicationHostObservationPrivate(t, r, code, pid, true, false)
	}
	c.Automation = nil
	saveInventoryConfigPrivate(t, config, c)
	complete, code, _ := run()
	accounting, e := os.ReadFile(filepath.Join(complete.AttemptDirectory, "application-accounting.json"))
	if e != nil {
		t.Fatal(e)
	}
	var view censusApplicationAccountingPrivate
	if e = json.Unmarshal(accounting, &view); e != nil {
		t.Fatal(e)
	}
	human, e := os.ReadFile(filepath.Join(complete.AttemptDirectory, "application-accounting.md"))
	if e != nil {
		t.Fatal(e)
	}
	assertInventoryPrivate(t, "ASSERT_APPLICATION_ALL_SEGMENTS_ADVANCED", code == 0 && complete.Status == "APPLICATION_ACCOUNTING_COMPLETE_HOST_ATTESTED" && complete.StopCalls == 0 && len(view.Verified) == 5 && len(originalIDs) == 9 && totalInputs > 64)
	assertInventoryPrivate(t, "ASSERT_APPLICATION_FINAL_ACCOUNTING_EXACT", view.FileDenominator == 72 && view.EligibleFiles == 71 && view.CompletedFiles == 71 && view.ObservedSymbolEntries == 71 && view.CallableTargets == 70 && view.NonCallableSymbols == 1 && view.ExcludedFiles == 1 && view.GapFiles == 0 && view.UnattemptedFiles == 0 && len(human) > 0 && view.Authority == 0 && !view.Accepted)
	t.Logf("APPLICATION_ALL_SEGMENTS_EVIDENCE base=%s final=%s plan=%s origins=%d prep-inputs=%d", base, complete.AttemptDirectory, planID, len(originalIDs), totalInputs)
}
