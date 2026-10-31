//go:build darwin || linux

package main

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// Runtime assertions deliberately decode the existing report, so the baseline
// reaches the assertion rather than failing to compile for a missing new type.
func TestCensusPrivateApplicationPlanDenominator(t *testing.T) {
	base, src, _, args := inventoryTwoFixturePrivate(t)
	for i := 0; i < 62; i++ {
		if err := os.WriteFile(filepath.Join(src, fmt.Sprintf("eligible-%03d.go", i)), []byte("package owned\n\nfunc A0() {}\n"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	deps, _, _ := inventoryTwoDepsPrivate(t)
	r, code := runInventoryCLIPrivate(t, args, deps)
	raw, err := os.ReadFile(filepath.Join(r.AttemptDirectory, "application-plan.json"))
	var plan struct {
		Schema, SourceBinding, Progression                                                                    string
		Authority, CompletedFiles, FileDenominator, EligibleFiles, IneligibleFiles, OriginCount, SegmentCount int
		Pages                                                                                                 []censusInventoryPagePrivate
	}
	valid := err == nil && json.Unmarshal(raw, &plan) == nil && r.ApplicationPlan != nil
	if r.ApplicationPlan != nil {
		valid = valid && r.ApplicationPlan.Selector == "application-plan.json" && r.ApplicationPlan.ByteLength == len(raw) && r.ApplicationPlan.SHA256 == fmt.Sprintf("sha256:%x", sha256.Sum256(raw)) && r.ApplicationPlan.Entries == plan.SegmentCount
	}
	seen := map[int]bool{}
	ordered := true
	last := -1
	origins, segments := 0, 0
	for _, page := range plan.Pages {
		b, e := os.ReadFile(filepath.Join(r.AttemptDirectory, page.Selector))
		var rows []struct {
			Ordinal int
			Origins []struct {
				Ordinal int
				Files   []int
			}
		}
		if e != nil || json.Unmarshal(b, &rows) != nil {
			valid = false
			continue
		}
		if page.SHA256 != fmt.Sprintf("sha256:%x", sha256.Sum256(b)) || len(b) != page.ByteLength || len(rows) != page.Entries {
			valid = false
		}
		for _, segment := range rows {
			if segment.Ordinal != segments || len(segment.Origins) > 2 {
				valid = false
			}
			segments++
			for _, origin := range segment.Origins {
				if origin.Ordinal != origins || len(origin.Files) < 1 || len(origin.Files) > 8 {
					valid = false
				}
				origins++
				for _, i := range origin.Files {
					if seen[i] || i <= last {
						ordered = false
					}
					seen[i] = true
					last = i
				}
			}
		}
	}
	assertInventoryPrivate(t, "ASSERT_APPLICATION_PLAN_COMPLETE_PARTITIONS", code == 0 && valid && ordered && plan.FileDenominator == 72 && plan.EligibleFiles == 71 && plan.IneligibleFiles == 1 && len(seen) == 71 && origins == 9 && segments == 5 && plan.OriginCount == origins && plan.SegmentCount == segments)
	// A separate assertion owns the boundary, rather than treating a partition
	// count as evidence that these files have been captured or interpreted.
	assertInventoryPrivate(t, "ASSERT_APPLICATION_PLAN_NO_ADVANCEMENT_AUTHORITY", valid && plan.Authority == 0 && plan.CompletedFiles == 0 && plan.SourceBinding == "METADATA_ONLY" && plan.Progression == "BLOCKED_FULL_SOURCE_PINS_AND_VERIFIED_SEGMENT_RECEIPTS_REQUIRED")
	t.Logf("application metadata-only plan fixture=%s attempt=%s; no continuation or application completeness claim", base, r.AttemptDirectory)
}

func TestCensusPrivateApplicationPlanPagedMetadata(t *testing.T) {
	rows := make([]censusInventoryFilePrivate, 2049)
	for i := range rows {
		rows[i].File.Path = fmt.Sprintf("file-%06d.go", i)
		rows[i].Disposition = "UNPROCESSED"
		rows[i].Origin = -1
	}
	dir := t.TempDir()
	denominator, err := retainCensusInventoryPagesPrivate(dir, "planned-files", rows)
	if err != nil {
		t.Fatal(err)
	}
	c := inventoryConfigPrivate()
	c.MaxOrigins, c.Policy.MaxNominationFiles = 2, 8
	ref, err := retainCensusApplicationPlanPrivate(context.Background(), dir, censusCLIOptions{}, c, rows, denominator)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(dir, ref.Selector))
	if err != nil {
		t.Fatal(err)
	}
	var plan censusApplicationPlanPrivate
	if err := json.Unmarshal(raw, &plan); err != nil {
		t.Fatal(err)
	}
	if len(plan.Pages) != 2 || plan.Pages[0].Entries != 128 || plan.Pages[1].Entries != 1 || plan.OriginCount != 257 || plan.SegmentCount != 129 || plan.EligibleFiles != 2049 {
		t.Fatal("metadata page boundary accounting")
	}
	// An existing artifact is not a cache or an overwrite target.
	if _, err := retainCensusApplicationPlanPrivate(context.Background(), dir, censusCLIOptions{}, c, rows, denominator); err == nil {
		t.Fatal("retention replay overwrote existing metadata")
	}
	after, err := os.ReadFile(filepath.Join(dir, ref.Selector))
	if err != nil || string(after) != string(raw) {
		t.Fatal("retained plan changed on replay")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	cancelDir := t.TempDir()
	if _, err := retainCensusApplicationPlanPrivate(ctx, cancelDir, censusCLIOptions{}, c, rows, denominator); err == nil {
		t.Fatal("canceled plan retained")
	}
	entries, err := os.ReadDir(cancelDir)
	if err != nil || len(entries) != 0 {
		t.Fatal("canceled metadata plan wrote artifacts")
	}
}
