//go:build darwin || linux

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"lsp-trace/internal/censusacquisition"
	pilot "lsp-trace/internal/censuslineagepilot"
	"lsp-trace/internal/source"
	"lsp-trace/sessionruntime"
	"os"
	"path/filepath"
	"testing"
)

func TestCensusPrivateInventoryPagedDenominator(t *testing.T) {
	_, source, _, args := inventoryFixturePrivate(t)
	for i := 0; i < 130; i++ {
		if e := os.WriteFile(filepath.Join(source, fmt.Sprintf("excluded-%03d.go", i)), []byte("package excluded\n"), 0600); e != nil {
			t.Fatal(e)
		}
	}
	for i, a := range args {
		if a == "excluded.go" {
			args[i] = "excluded*.go"
		}
	}
	deps, _ := inventoryDepsPrivate(t)
	r, code := runInventoryCLIPrivate(t, args, deps)
	entries := []censusInventoryFilePrivate{}
	valid := true
	for _, page := range r.FilePages {
		raw, e := os.ReadFile(filepath.Join(r.AttemptDirectory, page.Selector))
		var rows []censusInventoryFilePrivate
		if e != nil || json.Unmarshal(raw, &rows) != nil || pilot.Digest(raw) != page.SHA256 || len(raw) != page.ByteLength || len(rows) != page.Entries || len(rows) > 128 {
			valid = false
		}
		entries = append(entries, rows...)
	}
	seen := map[string]bool{}
	for _, row := range entries {
		if seen[row.File.Path] {
			valid = false
		}
		seen[row.File.Path] = true
	}
	assertInventoryPrivate(t, "ASSERT_CENSUS_PAGED_DENOMINATOR", code == 0 && r.FileDenominator == 134 && len(r.Files) == 0 && len(r.FilePages) == 2 && len(entries) == 134 && valid)
}

func TestCensusPrivateInventoryEnumerationFailure(t *testing.T) {
	_, _, _, args := inventoryFixturePrivate(t)
	deps, starts := inventoryDepsPrivate(t)
	deps.runPrivateInventory = func(o censusCLIOptions, cfg censusCoreConfig, out, err io.Writer) int {
		d := productionCensusInventoryDependenciesPrivate()
		d.enumerate = func(context.Context, censusCLIOptions) ([]censusacquisition.SourceFile, error) {
			return nil, &source.LimitError{Category: "entries", Observed: 100001, Limit: 100000}
		}
		return runCensusInventoryWithDependenciesPrivate(o, cfg, out, err, d)
	}
	r, code := runInventoryCLIPrivate(t, args, deps)
	assertInventoryPrivate(t, "ASSERT_CENSUS_ENUMERATION_FAILURE_RETAINED", code == 1 && r.Failure == "ENUMERATION_FAILED" && r.ApplicationCoverage == "UNKNOWN" && r.EnumerationLimit != nil && r.EnumerationLimit.Observed == 100001 && r.EnumerationLimit.Limit == 100000 && starts.Load() == 0 && r.Outcome == nil)
}

func TestCensusPrivateInventoryRetainsFailure(t *testing.T) {
	_, _, _, args := inventoryFixturePrivate(t)
	deps, starts := inventoryDepsPrivate(t)
	original := deps.newStarter
	deps.newStarter = func() (sessionruntime.Starter, error) {
		s, e := original()
		if e == nil {
			s.(inventoryStarterPrivate).child.failCalls.Store(true)
		}
		return s, e
	}
	r, code := runInventoryCLIPrivate(t, args, deps)
	t.Log("procedure: owned valid JSON-RPC request error -> original acquisition failure -> retained denominator and batch disposition, no retry")
	assertInventoryPrivate(t, "ASSERT_CENSUS_FAILURE_ACCOUNTING", code == 1 && r.Status == "RETAINED_FAILURE" && len(r.Files) == 4 && len(r.Origins) == 1 && len(r.Origins[0].Discovery) > 0 && len(r.Origins[0].Batches) == 1 && r.Origins[0].Batches[0].Status == "ACQUISITION_FAILED" && r.Outcome == nil && starts.Load() == 1 && r.ShutdownCalls == 1 && r.ShutdownJoined && r.CleanupVerified)
}
