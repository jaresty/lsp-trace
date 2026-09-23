package adr0011methodresult

// CandidateBinding pairs an externally declared query occurrence with one
// retained, still-unadmitted canonical candidate. No candidate may declare its
// own occurrence identity or reference target.
type CandidateBinding struct {
	OccurrenceID string
	Raw          []byte
}

// CheckCompleteCandidateTransactionLedger checks a fully supplied private
// candidate bundle. The declarations, terminal flags and runtime-reported keys
// remain untrusted: success is consistency, not an observed completion, method
// receipt, reference identity, occurrence admission or Leiden input.
func CheckCompleteCandidateTransactionLedger(bindings []CandidateBinding, ledger CandidateLedger) error {
	if err := CheckCandidateLedger(ledger); err != nil || len(bindings) != len(ledger.Queries) || len(ledger.Members) != len(ledger.Queries) {
		return ErrCandidateJoin
	}
	queries := make(map[string]CandidateQuery, len(ledger.Queries))
	members := make(map[string]CandidateQueryMember, len(ledger.Members))
	for _, q := range ledger.Queries {
		queries[q.OccurrenceID] = q
	}
	for _, m := range ledger.Members {
		if m.Terminal == "" {
			return ErrCandidateJoin
		}
		members[m.OccurrenceID] = m
	}
	seen := make(map[string]bool, len(bindings))
	ids := make(map[string]bool, len(bindings))
	keys := make(map[struct {
		session         string
		generation, key uint64
	}]bool, len(bindings))
	for _, binding := range bindings {
		q, declared := queries[binding.OccurrenceID]
		member, terminal := members[binding.OccurrenceID]
		if !declared || !terminal || seen[binding.OccurrenceID] {
			return ErrCandidateJoin
		}
		seen[binding.OccurrenceID] = true
		c, err := VerifyCanonicalCandidate(binding.Raw)
		if err != nil || ids[c.ID] {
			return ErrCandidateJoin
		}
		key := struct {
			session         string
			generation, key uint64
		}{c.SessionID, c.Generation, c.KeyID}
		if keys[key] {
			return ErrCandidateJoin
		}
		ids[c.ID], keys[key] = true, true
		if CheckCandidateTransactionLedger(binding.Raw, CandidateLedger{Queries: []CandidateQuery{q}, Members: []CandidateQueryMember{member}}, binding.OccurrenceID) != nil {
			return ErrCandidateJoin
		}
	}
	return nil
}
