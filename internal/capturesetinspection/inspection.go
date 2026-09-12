// Package capturesetinspection projects verified private capture-set manifests
// into a bounded, non-disclosing inspection result.
package capturesetinspection

import (
	"errors"
	"fmt"
	"sort"

	"lsp-trace/internal/captureset"
)

const Version = "lsp-trace.capture-set-inspection.v1"

type DispositionCount struct {
	Disposition string `json:"disposition"`
	Count       int    `json:"count"`
}

type LedgerSummary struct {
	Denominator  int                `json:"denominator"`
	Dispositions []DispositionCount `json:"dispositions"`
}

type Result struct {
	InspectionVersion          string        `json:"inspection_version"`
	CaptureSetIdentity         string        `json:"capture_set_identity"`
	Disclosure                 string        `json:"disclosure"`
	TargetCount                int           `json:"target_count"`
	BatchCount                 int           `json:"batch_count"`
	ConstituentCount           int           `json:"constituent_count"`
	Files                      LedgerSummary `json:"files"`
	Symbols                    LedgerSummary `json:"symbols"`
	Authority                  int           `json:"authority"`
	SourceGraphComplete        string        `json:"source_graph_complete"`
	NativeSingleCaptureCustody bool          `json:"native_single_capture_custody"`
	CrossCaptureCalls          bool          `json:"cross_capture_calls"`
	LeidenAdmissible           bool          `json:"leiden_admissible"`
}

func Project(m captureset.Manifest) Result {
	return Result{
		InspectionVersion:  Version,
		CaptureSetIdentity: m.ImmutableSelector,
		Disclosure:         m.Disclosure,
		TargetCount:        len(m.Targets), BatchCount: len(m.Batches), ConstituentCount: len(m.Constituents),
		Files: summarize(m.FileLedger), Symbols: summarize(m.SymbolLedger),
		Authority: m.Authority, SourceGraphComplete: m.SourceGraphComplete,
		NativeSingleCaptureCustody: m.NativeSingleCaptureCustody,
		CrossCaptureCalls:          len(m.CrossCaptureCalls) != 0,
		LeidenAdmissible:           m.LeidenAdmissible,
	}
}

func summarize(l captureset.Ledger) LedgerSummary {
	counts := make(map[string]int)
	for _, entry := range l.Entries {
		counts[entry.Disposition]++
	}
	keys := make([]string, 0, len(counts))
	for key := range counts {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := LedgerSummary{Denominator: l.Denominator, Dispositions: make([]DispositionCount, 0, len(keys))}
	for _, key := range keys {
		out.Dispositions = append(out.Dispositions, DispositionCount{Disposition: key, Count: counts[key]})
	}
	return out
}

func Validate(r Result) error {
	if r.InspectionVersion != Version || r.CaptureSetIdentity == "" || (r.Disclosure != "PRIVATE" && r.Disclosure != "REDACTED") {
		return errors.New("invalid capture-set inspection identity")
	}
	if r.TargetCount < 1 || r.TargetCount > captureset.MaxTargets || r.BatchCount < 1 || r.ConstituentCount != r.BatchCount || r.BatchCount != (r.TargetCount+captureset.MaxBatchTargets-1)/captureset.MaxBatchTargets {
		return errors.New("invalid capture-set inspection counts")
	}
	if err := validateLedger("files", r.Files); err != nil {
		return err
	}
	if err := validateLedger("symbols", r.Symbols); err != nil {
		return err
	}
	if r.Authority != 0 || r.SourceGraphComplete != "UNKNOWN" || r.NativeSingleCaptureCustody || r.CrossCaptureCalls || r.LeidenAdmissible {
		return errors.New("capture-set inspection ceiling violated")
	}
	return nil
}

func validateLedger(name string, l LedgerSummary) error {
	if l.Denominator < 0 || l.Denominator > captureset.MaxResources {
		return fmt.Errorf("invalid %s denominator", name)
	}
	total, prior := 0, ""
	for i, d := range l.Dispositions {
		if d.Disposition == "" || d.Count < 1 || (i > 0 && prior >= d.Disposition) {
			return fmt.Errorf("invalid %s dispositions", name)
		}
		total += d.Count
		prior = d.Disposition
	}
	if total != l.Denominator {
		return fmt.Errorf("%s dispositions are not closed", name)
	}
	return nil
}
