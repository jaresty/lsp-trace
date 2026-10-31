//go:build darwin || linux

package main

// This is an APPLICATION_MANIFEST metadata view, not a Native envelope, a
// continuation token, or a source owner. In particular, the overflow files have
// NOT been source-frozen by the current bounded coordinator. Nothing in this
// file may be used to select a later acquisition or to claim completed coverage.
import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	pilot "lsp-trace/internal/censuslineagepilot"
)

type censusApplicationPlanOriginPrivate struct {
	Ordinal int
	Files   []int // Exact indices in the complete planned-files denominator pages.
}
type censusApplicationPlanSegmentPrivate struct {
	Ordinal int
	Origins []censusApplicationPlanOriginPrivate
}
type censusApplicationPlanDispositionPrivate struct {
	Disposition string
	Files       int
}
type censusApplicationPlanPrivate struct {
	Schema, InputFamily, SourceBinding, Progression string
	Authority, CompletedFiles                       int
	FeatureIdentity, CrossSegmentNavigation         string
	Workspace, RevisionAssertion                    string
	Sources, Includes, Excludes, PrunedDirectories  []string
	Configuration                                   censusInventoryConfigPrivate
	FileDenominator, EligibleFiles, IneligibleFiles int
	OriginCount, SegmentCount                       int
	IneligibleByDisposition                         []censusApplicationPlanDispositionPrivate
	DenominatorPages, Pages                         []censusInventoryPagePrivate
}

// Derive only from the exact pre-freeze denominator produced by the existing
// planner. Its resource-gap rows are eligible FUTURE work, not exclusions. The
// original bounded planner, scope hashes, freezes and limits remain unchanged.
func censusApplicationSegmentsPrivate(ctx context.Context, rows []censusInventoryFilePrivate, maxOrigins, maxFiles int) (segments []censusApplicationPlanSegmentPrivate, eligible int, dispositions []censusApplicationPlanDispositionPrivate, err error) {
	if maxOrigins < 1 || maxOrigins > 2 || maxFiles < 1 || maxFiles > 8 {
		return nil, 0, nil, errors.New("private application metadata partition bounds")
	}
	counts := map[string]int{}
	originOrdinal := 0
	for i, row := range rows {
		if err := ctx.Err(); err != nil {
			return nil, 0, nil, err
		}
		switch row.Disposition {
		case "UNPROCESSED", "RESOURCE_GAP_ORIGIN_LIMIT":
			// Both statuses are exact outputs of the current pre-freeze planner.
		default:
			counts[row.Disposition]++
			continue
		}
		eligible++
		if len(segments) == 0 {
			segments = append(segments, censusApplicationPlanSegmentPrivate{Ordinal: 0})
		}
		s := &segments[len(segments)-1]
		if len(s.Origins) == 0 || len(s.Origins[len(s.Origins)-1].Files) == maxFiles {
			if len(s.Origins) == maxOrigins {
				segments = append(segments, censusApplicationPlanSegmentPrivate{Ordinal: len(segments)})
				s = &segments[len(segments)-1]
			}
			s.Origins = append(s.Origins, censusApplicationPlanOriginPrivate{Ordinal: originOrdinal})
			originOrdinal++
		}
		origin := &s.Origins[len(s.Origins)-1]
		origin.Files = append(origin.Files, i)
	}
	keys := make([]string, 0, len(counts))
	for key := range counts {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		dispositions = append(dispositions, censusApplicationPlanDispositionPrivate{key, counts[key]})
	}
	return segments, eligible, dispositions, ctx.Err()
}

func retainCensusApplicationPlanPrivate(ctx context.Context, directory string, o censusCLIOptions, c censusInventoryConfigPrivate, rows []censusInventoryFilePrivate, denominator []censusInventoryPagePrivate) (*censusInventoryPagePrivate, error) {
	segments, eligible, dispositions, err := censusApplicationSegmentsPrivate(ctx, rows, c.MaxOrigins, c.Policy.MaxNominationFiles)
	if err != nil {
		return nil, err
	}
	config := c
	config.Automation = nil // Disclosure approval is not accounting identity.
	config.CohortAutomation = nil
	plan := censusApplicationPlanPrivate{
		Schema: "lsp-trace.census-application-metadata-plan.private.v1", InputFamily: "APPLICATION_MANIFEST",
		SourceBinding: "METADATA_ONLY", Progression: "BLOCKED_FULL_SOURCE_PINS_AND_VERIFIED_SEGMENT_RECEIPTS_REQUIRED",
		FeatureIdentity: "UNRESOLVED", CrossSegmentNavigation: "UNREVIEWED",
		Workspace: o.Workspace, RevisionAssertion: c.RevisionAssertion,
		Sources: o.Sources, Includes: o.Includes, Excludes: o.Excludes, PrunedDirectories: o.PrunedDirectories,
		Configuration: config, FileDenominator: len(rows), EligibleFiles: eligible, IneligibleFiles: len(rows) - eligible,
		SegmentCount: len(segments), IneligibleByDisposition: dispositions, DenominatorPages: denominator,
	}
	for _, s := range segments {
		plan.OriginCount += len(s.Origins)
	}
	// At most 128 complete segments per page. Metadata pages do not widen the
	// 1MiB response budget and never contain source bodies, graphs or candidates.
	for start := 0; start < len(segments); start += 128 {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		end := min(start+128, len(segments))
		raw, err := json.Marshal(segments[start:end])
		if err != nil || len(raw) > 1<<20 {
			return nil, errors.New("private application metadata page bound")
		}
		name := fmt.Sprintf("application-segments-%06d.json", len(plan.Pages))
		if err = writeCensusInventoryFilePrivate(filepath.Join(directory, name), json.RawMessage(raw)); err != nil {
			return nil, err
		}
		observed, err := os.ReadFile(filepath.Join(directory, name))
		if err != nil || !bytes.Equal(observed, raw) {
			return nil, errors.Join(err, errors.New("private application metadata page readback mismatch"))
		}
		plan.Pages = append(plan.Pages, censusInventoryPagePrivate{Selector: name, SHA256: pilot.Digest(observed), ByteLength: len(observed), Entries: end - start})
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	raw, err := json.Marshal(plan)
	if err != nil {
		return nil, err
	}
	const name = "application-plan.json"
	if err = writeCensusInventoryFilePrivate(filepath.Join(directory, name), json.RawMessage(raw)); err != nil {
		return nil, err
	}
	observed, err := os.ReadFile(filepath.Join(directory, name))
	if err != nil || !bytes.Equal(observed, raw) {
		return nil, errors.Join(err, errors.New("private application metadata plan readback mismatch"))
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return &censusInventoryPagePrivate{Selector: name, SHA256: pilot.Digest(observed), ByteLength: len(observed), Entries: len(segments)}, nil
}
