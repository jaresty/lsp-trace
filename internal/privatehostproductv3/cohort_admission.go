//go:build darwin || linux

package privatehostproductv3

import (
	"bytes"
	"io"
	pilot "lsp-trace/internal/censuslineagepilot"
)

type cohortFrontierRecord struct {
	Generation uint64
	Tip        string
	Cost       pilot.PageCostPrivateV3
	Cleanup    Lifecycle
}

func verifyCohortSnapshot(s *store, c CohortConfig, r CohortReport) bool {
	if r.SourceValidation != "UNKNOWN" || r.LedgerValidation != "UNKNOWN" || r.HostReturns != "UNKNOWN" || r.UnadmittedPublications && r.PriorPublicationValidation != "UNKNOWN" {
		return false
	}
	if r.LedgerGeneration > 0 {
		raw, e := s.read("host/state-00000000.json", 16384)
		var f cohortFrontierRecord
		if e != nil || strict(raw, &f) != nil || f.Generation != r.LedgerGeneration || f.Tip != r.LedgerTip || f.Cost != r.PolicyReservation || f.Cleanup != r.OriginalCleanup || !digestValue(f.Tip) {
			return false
		}
	} else if r.LedgerTip != "" || r.PolicyReservation != (pilot.PageCostPrivateV3{}) {
		return false
	}
	d, e := s.root.Open("pages")
	if e != nil {
		return false
	}
	defer d.Close()
	names, e := d.Readdirnames(stateLimit/128 + 1)
	if e != nil && e != io.EOF || len(names) > stateLimit/128 {
		return false
	}
	for _, name := range names {
		raw, e := s.read("pages/"+name, stateLimit)
		var p cohortProgress
		if e == nil && strict(raw, &p) == nil && admitCohortGraph(p) && bytes.Equal(mustJSON(p.Config), mustJSON(c)) && p.Inflight == r.Inflight && p.Calls == r.Calls && p.RequestBytes == r.RequestBytes && p.ResponseBytes == r.ResponseBytes && bytes.Equal(mustJSON(p.Outcomes), mustJSON(r.Outcomes)) && bytes.Equal(mustJSON(p.NavigationHistory), mustJSON(r.NavigationHistory)) && p.NavigationStatus == r.NavigationStatus {
			return true
		}
	}
	return false
}
