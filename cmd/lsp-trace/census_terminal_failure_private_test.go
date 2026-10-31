//go:build darwin || linux

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestCensusTerminalPostGapFailedCLIPrivate(t *testing.T) {
	base, _, path, args := terminalApplicationFixturePrivate(t)
	runner := newTerminalProcessRunnerPrivate(t, base, args)
	for i := 0; i < 2; i++ {
		r, code, pid := runner.run()
		if code != 0 || r.Status != censusTerminalSegmentStatusPrivate {
			t.Fatal("terminal setup")
		}
		applicationHostObservationPrivate(t, r, code, pid, true, false)
	}
	t.Setenv("LSP_TRACE_OWNED_TERMINAL_FAIL", "1")
	failed, code, pid := runner.run()
	assertInventoryPrivate(t, "ASSERT_TERMINAL_ACTUAL_FAILURE_WAIT_ONE", code == 1 && failed.ApplicationBinding.Segment == 2 && failed.ApplicationBinding.ClaimSHA256 != "" && failed.StopCalls == 1 && failed.ShutdownJoined && failed.CleanupVerified && failed.Outcome == nil)
	applicationHostObservationPrivate(t, failed, code, pid, true, false)
	stopped, code, _ := runner.run()
	assertInventoryPrivate(t, "ASSERT_TERMINAL_ACTUAL_FAILED_SPENT_NO_ADVANCE", code == 1 && stopped.StopCalls == 0 && stopped.ApplicationBinding.Segment == 2)
	t.Logf("TERMINAL_FAILED_CLI_EVIDENCE base=%s failed=%s stopped=%s config=%s", base, failed.AttemptDirectory, stopped.AttemptDirectory, path)
}
func TestCensusTerminalMixedSegmentPrivate(t *testing.T) {
	base, src, path, args := terminalApplicationFixturePrivate(t)
	// First normal origin and second capped origin share one segment. This pins
	// the disposition of valid computation rather than silently discarding it.
	// Put a normal fixture file first lexically and retain the cap as second origin.
	if e := os.Rename(filepath.Join(src, "c.go"), filepath.Join(src, "0-normal.go")); e != nil {
		t.Fatal(e)
	}
	c, _ := loadCensusInventoryConfigPrivate(path)
	c.MaxOrigins = 2
	saveInventoryConfigPrivate(t, path, c)
	t.Setenv("LSP_TRACE_OWNED_TERMINAL_SERVER", "1")
	deps, _, _ := inventoryTwoDepsPrivate(t)
	r, code := runInventoryCLIPrivate(t, args, deps)
	assertInventoryPrivate(t, "ASSERT_TERMINAL_MIXED_COMPUTED_DISPOSITION", code == 0 && len(r.Origins) == 2 && r.Origins[0].Status == censusComputedSegmentGapPrivate && r.Origins[0].ProducerCopies[0].Receipt != nil && r.Origins[1].Terminal != nil && r.Outcome == nil && r.Inventory == nil)
	applicationHostObservationPrivate(t, r, code, r.ApplicationBinding.ProcessID, true, false)
	later, laterCode := runInventoryCLIPrivate(t, args, deps)
	assertInventoryPrivate(t, "ASSERT_TERMINAL_MIXED_VERIFIED_ADVANCE", laterCode == 0 && later.ApplicationBinding.Segment == 1)
	raw, e := os.ReadFile(filepath.Join(later.AttemptDirectory, "application-accounting.json"))
	if e != nil {
		t.Fatal(e)
	}
	var view censusApplicationAccountingPrivate
	_ = json.Unmarshal(raw, &view)
	assertInventoryPrivate(t, "ASSERT_TERMINAL_MIXED_DISJOINT_CREDIT", view.CompletedFiles == 0 && view.TerminalAccountedFiles == 2 && view.TerminalAccountedTargets == 85 && view.NativePreparedTargets == 0)
	_ = base
}
