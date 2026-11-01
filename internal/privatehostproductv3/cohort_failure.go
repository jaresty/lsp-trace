//go:build darwin || linux

package privatehostproductv3

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
)

func InitializeCohort(c CohortConfig) error {
	if ValidateCohortConfig(c) != nil {
		return ErrInvalid
	}
	s, e := openCohortStore(c)
	if e != nil {
		return e
	}
	defer s.close()
	stored, readErr := s.read("plan/config-00000001.json", stateLimit)
	if readErr == nil {
		if !bytes.Equal(stored, mustJSON(c)) {
			return ErrInvalid
		}
		return nil
	}
	if !os.IsNotExist(readErr) {
		return readErr
	}
	for _, dir := range []string{"pages", "claim", "host", "result"} {
		d, e := s.root.Open(dir)
		if e != nil {
			return e
		}
		names, e := d.Readdirnames(1)
		d.Close()
		if len(names) > 0 || e != nil && e != io.EOF {
			return ErrInvalid
		}
	}
	return nil
}

// Qualified previous reports survive fresh source/ledger refusal. The returned
// view explicitly says UNKNOWN; no decoded DTO becomes an admitted source owner.
func DeliverCohortFailure(c CohortConfig, requested int, class string) (CohortReport, error) {
	if ValidateCohortConfig(c) != nil || !filepath.IsAbs(c.Store) || filepath.Clean(c.Store) != c.Store || requested != c.Requested || requested < 1 || requested > 64 {
		return CohortReport{}, ErrInvalid
	}
	// Failure classes are safe details, not a channel for source/environment text.
	if len(class) == 0 || len(class) > 64 {
		class = "RECOVERY_REFUSED"
	}
	for _, ch := range class {
		if ch != '_' && (ch < 'A' || ch > 'Z') && (ch < '0' || ch > '9') {
			class = "RECOVERY_REFUSED"
			break
		}
	}
	s, e := openCohortStore(c)
	if e != nil {
		return CohortReport{}, e
	}
	defer s.close()
	if r, e := latestCohort(s, c); e == nil {
		// A valid earlier report is not evidence that no later physical work
		// occurred. Keep its outcomes, but only fast-path unchanged spending.
		p := cohortProgress{Config: c, Outcomes: r.Outcomes, NavigationHistory: r.NavigationHistory, NavigationStatus: r.NavigationStatus, AccountingValidation: r.AccountingValidation, Calls: r.Calls, RequestBytes: r.RequestBytes, ResponseBytes: r.ResponseBytes, Inflight: r.Inflight}
		if e = recoverCohortSpend(s, c, &p); e != nil {
			return qualifiedCohortFailure(s, c, p, "FAILED_"+class), ErrInvalid
		}
		if p.Calls == r.Calls && p.RequestBytes == r.RequestBytes && p.ResponseBytes == r.ResponseBytes {
			r.PriorPublicationValidation = "LOCAL_CONSISTENCY_ONLY"
			if s.rejectedPrior {
				r.PriorPublicationValidation = "UNKNOWN"
				r.UnadmittedPublications = true
			}
			r.Status = "FAILED_" + class
			r.SourceValidation = "UNKNOWN"
			r.LedgerValidation = "UNKNOWN"
			e = cohortDeliver(s, &r)
			return r, e
		}
	}
	// A failed/ambiguous exchange may have no report yet. Recover ONLY the last
	// bounded reservation snapshot, never invoke the backend or refund its call.
	var p cohortProgress
	d, e := s.root.Open("pages")
	if e != nil {
		return CohortReport{}, e
	}
	names, e := d.Readdirnames(stateLimit/128 + 1)
	d.Close()
	if e != nil && e != io.EOF {
		return CohortReport{}, e
	}
	if len(names) > stateLimit/128 {
		return CohortReport{}, ErrInvalid
	}
	sort.Sort(sort.Reverse(sort.StringSlice(names)))
	found := false
	for _, name := range names {
		raw, e := s.read("pages/"+name, stateLimit)
		if e == nil && strict(raw, &p) == nil && admitCohortGraph(p) && bytes.Equal(mustJSON(p.Config), mustJSON(c)) {
			found = true
			break
		}
	}
	if !found {
		p = cohortProgress{Config: c, Outcomes: []CohortOutcome{}}
		for n := 0; n < requested; n++ {
			p.Outcomes = append(p.Outcomes, CohortOutcome{Item: CohortItem{ItemID: fmt.Sprintf("unprepared-%08d", n)}, Status: "PENDING", History: []CohortDecision{}})
		}
	}
	if raw, e := s.read("plan/config-00000001.json", stateLimit); os.IsNotExist(e) {
		if e = s.write("plan/config-00000001.json", mustJSON(c)); e != nil {
			return CohortReport{}, e
		}
	} else if e != nil || !bytes.Equal(raw, mustJSON(c)) {
		return CohortReport{}, ErrInvalid
	}
	if found {
		for _, name := range names {
			var seq int
			if _, parseErr := fmt.Sscanf(name, "state-%08d.json", &seq); parseErr != nil {
				return CohortReport{}, ErrInvalid
			}
			if seq > s.stateSeq {
				s.stateSeq = seq
			}
		}
	}
	if e = recoverCohortSpend(s, c, &p); e != nil {
		return qualifiedCohortFailure(s, c, p, "FAILED_"+class), ErrInvalid
	}
	if !found && p.Calls > 0 || p.AccountingValidation != "" && p.AccountingValidation != "EXACT" {
		return qualifiedCohortFailure(s, c, p, "FAILED_"+class), ErrInvalid
	}
	{
		if e = cohortSave(s, p); e != nil {
			return qualifiedCohortFailure(s, c, p, "FAILED_"+class), ErrInvalid
		}
	}
	r := cohortReport(c, p, "FAILED_"+class)
	// Replay physical exchanges before promoting recovered progress to a report.
	if found && verifyCohortHistory(s, c, r) != nil {
		return qualifiedCohortFailure(s, c, p, "FAILED_"+class), ErrInvalid
	}
	e = cohortDeliver(s, &r)
	return r, e
}
