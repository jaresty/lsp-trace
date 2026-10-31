//go:build darwin || linux

package main

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestCensusTerminalNormalNeedsApprovalPrivate(t *testing.T) {
	_, src, path, args := terminalApplicationFixturePrivate(t)
	for _, name := range []string{"a.go", "b.go"} {
		if err := os.Remove(filepath.Join(src, name)); err != nil {
			t.Fatal(err)
		}
	}
	deps, _, _ := inventoryTwoDepsPrivate(t)
	r, code := runInventoryCLIPrivate(t, args, deps)
	assertInventoryPrivate(t, "ASSERT_TERMINAL_NORMAL_UNAPPROVED_SPENT", code == 1 && r.ApplicationBinding != nil && r.ApplicationBinding.ClaimSHA256 != "" && r.Outcome == nil && r.Inventory == nil && r.CleanupVerified)
	deps, starts, _ := inventoryTwoDepsPrivate(t)
	_, code = runInventoryCLIPrivate(t, args, deps)
	assertInventoryPrivate(t, "ASSERT_TERMINAL_NORMAL_UNAPPROVED_NO_RETRY", code == 1 && starts.Load() == 0)
	_ = path
}

func TestCensusTerminalDeadlineClaimBeforeStartupPrivate(t *testing.T) {
	_, _, path, args := terminalApplicationFixturePrivate(t)
	c, err := loadCensusInventoryConfigPrivate(path)
	if err != nil {
		t.Fatal(err)
	}
	c.Policy.TimeoutMS = 500
	c.Policy.RequestTimeoutMS = 500
	c.OperationTimeoutMS = 500
	saveInventoryConfigPrivate(t, path, c)
	for i, a := range args {
		if a == "--timeout" || a == "--request-timeout" {
			args[i+1] = "500ms"
		}
	}
	deps, _, _ := inventoryTwoDepsPrivate(t)
	calls := 0
	deps.runPrivateInventory = func(o censusCLIOptions, cfg censusCoreConfig, out, errout io.Writer) int {
		d := productionCensusInventoryDependenciesPrivate()
		d.runSession = func(rc initializedAcquisitionRunnerConfig, _ func(context.Context, *initializedAcquisitionRuntime) int) int {
			calls++
			<-rc.parent.Done()
			return 1
		}
		return runCensusInventoryWithDependenciesPrivate(o, cfg, out, errout, d)
	}
	r, code := runInventoryCLIPrivate(t, args, deps)
	_, claimErr := os.Stat(filepath.Join(c.Application.Directory, censusApplicationLeafPrivate(0, "claim")))
	assertInventoryPrivate(t, "ASSERT_TERMINAL_DEADLINE_CLAIM_EXISTS", code == 1 && calls == 1 && claimErr == nil && r.ApplicationBinding.ClaimSHA256 != "")
	calls = 0
	_, code = runInventoryCLIPrivate(t, args, deps)
	assertInventoryPrivate(t, "ASSERT_TERMINAL_DEADLINE_SPENT_NO_STARTUP", code == 1 && calls == 0)
}
