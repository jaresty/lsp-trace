//go:build darwin || linux

package privatehostproductv3

import (
	"bytes"
	"fmt"
	"os"
)

// Recover costs, NEVER semantics, across the reply/receipt/snapshot crash windows.
// Whole ordered claims and raw replies are bounded physical observations. A
// receipt without durable reply bytes can supply only an observed spend, not a
// supported description. No backend is invoked by recovery.
func recoverCohortSpend(s *store, c CohortConfig, p *cohortProgress) error {
	calls, requests, responses := 0, 0, 0
	gap := false
	for seq := 1; seq <= 16; seq++ {
		raw, e := s.read(fmt.Sprintf("claim/claim-%08d.json", seq), c.MaxRequestBytes)
		if os.IsNotExist(e) {
			gap = true
			continue
		}
		if e != nil || gap || seq > c.MaxCalls {
			return ErrInvalid
		}
		calls++
		requests += len(raw)
		reply, e := observedCohortReply(s, c, seq, raw)
		if e == nil {
			responses += len(reply)
			continue
		}
		if !os.IsNotExist(e) {
			return e
		}
		receiptRaw, re := s.read(fmt.Sprintf("host/claim-%08d.json", seq), 4096)
		if os.IsNotExist(re) {
			continue
		}
		var receipt cohortReceipt
		if re != nil || strict(receiptRaw, &receipt) != nil || receipt.Sequence != seq || receipt.Bytes < 0 || receipt.Bytes > (64<<10)+1 || !digestValue(receipt.ResponseSHA256) {
			return ErrInvalid
		}
		responses += receipt.Bytes
	}
	if requests > c.MaxTotalBytes || responses > c.MaxOutputBytes || requests > c.MaxTotalBytes-responses {
		return ErrInvalid
	}
	if calls < p.Calls || requests < p.RequestBytes || responses < p.ResponseBytes {
		return ErrInvalid
	}
	if calls > p.Calls || responses > p.ResponseBytes {
		p.Inflight = true
	}
	p.Calls = calls
	p.RequestBytes = requests
	p.ResponseBytes = responses
	return nil
}

// qualifiedCohortFailure preserves local observations, not semantic admission.
// It never opens the mailbox: a current READY/pin refusal cannot erase bytes
// already retained by this store. Each counted return requires its own bounded
// claim, config/grant/role/sequence binding, receipt and byte-exact host copy.
// Missing/inconsistent evidence is explicitly a previous local observation;
// neither kind of UNKNOWN is eligible for pair publication or exact history.
func qualifiedCohortFailure(s *store, c CohortConfig, p cohortProgress, status string) CohortReport {
	// These are observation bounds, NEVER allowances for another exchange. A
	// transport can return one physical frame beyond the reserved response/output
	// budget; that is past spend even though exact history must refuse it.
	knownOutputLimit := c.MaxOutputBytes + (64 << 10) + 1
	knownWireLimit := c.MaxTotalBytes + (64 << 10) + 1
	validProgress := admitCohortGraph(p) && bytes.Equal(mustJSON(p.Config), mustJSON(c)) && len(p.Outcomes) == c.Requested && p.Calls >= 0 && p.Calls <= c.MaxCalls && p.RequestBytes >= 0 && p.RequestBytes <= c.MaxTotalBytes && p.ResponseBytes >= 0 && p.ResponseBytes <= knownOutputLimit && p.RequestBytes <= knownWireLimit-p.ResponseBytes
	if !validProgress {
		p = cohortProgress{Config: c, Outcomes: []CohortOutcome{}}
		for n := 0; n < c.Requested; n++ {
			p.Outcomes = append(p.Outcomes, CohortOutcome{Item: CohortItem{ItemID: fmt.Sprintf("unprepared-%08d", n)}, Status: "PENDING", History: []CohortDecision{}})
		}
	}
	calls, requests, responses := 0, 0, 0
	complete, gap := validProgress, false
	for seq := 1; seq <= 16; seq++ {
		raw, e := s.read(fmt.Sprintf("claim/claim-%08d.json", seq), c.MaxRequestBytes)
		if os.IsNotExist(e) {
			gap = true
			continue
		}
		h, he := parseCohortFrame(raw)
		role := "interpreter"
		if seq%2 == 0 {
			role = "reviewer"
		}
		if e != nil || he != nil || gap || seq > c.MaxCalls || h.Sequence != seq || h.Role != role || h.ConfigSHA256 != CohortConfigDigest(c) || h.Grant != c.Grant || h.Protocol == CohortNavigationProtocol && !c.Navigation || requests > c.MaxTotalBytes-len(raw) {
			complete = false
			break
		}
		calls++
		requests += len(raw)
		receiptRaw, re := s.read(fmt.Sprintf("host/claim-%08d.json", seq), 4096)
		var receipt cohortReceipt
		if re != nil || strict(receiptRaw, &receipt) != nil || receipt.Sequence != seq || receipt.Bytes < 0 || receipt.Bytes > (64<<10)+1 || !digestValue(receipt.ResponseSHA256) {
			complete = false
			continue
		}
		reply, re := s.read(fmt.Sprintf("host/response-%08d.json", seq), (64<<10)+1)
		if receipt.Bytes == 0 && os.IsNotExist(re) {
			reply, re = []byte{}, nil
		}
		if re != nil || len(reply) != receipt.Bytes || hash(reply) != receipt.ResponseSHA256 || responses > knownOutputLimit-len(reply) || requests > knownWireLimit-responses-len(reply) {
			complete = false
			continue
		}
		responses += len(reply)
	}
	// Snapshot counters are never certified by deserialization. Agreement with
	// independently admitted raw local observations is required for known bytes.
	if calls < p.Calls || requests < p.RequestBytes || responses < p.ResponseBytes {
		complete = false
	}
	previousCalls, previousRequests, previousResponses := p.Calls, p.RequestBytes, p.ResponseBytes
	if calls > p.Calls {
		p.Calls = calls
	}
	if requests > p.RequestBytes {
		p.RequestBytes = requests
	}
	if responses > p.ResponseBytes {
		p.ResponseBytes = responses
	}
	// Mutually inconsistent lower-bound observations cannot be combined into a
	// report outside the configured wire bound. Keep the bounded snapshot only.
	if p.RequestBytes > knownWireLimit-p.ResponseBytes {
		p.Calls, p.RequestBytes, p.ResponseBytes = previousCalls, previousRequests, previousResponses
		complete = false
	}
	p.AccountingValidation = "UNKNOWN_LOCAL_PREVIOUS_OBSERVATION"
	if complete {
		p.AccountingValidation = cohortAccountingUnknown
	}
	r := cohortReport(c, p, status)
	r.PriorPublicationValidation = "UNKNOWN"
	r.UnadmittedPublications = true
	return r
}

func ambiguousMissingReplySpend(s *store, seq int) (int, error) {
	raw, e := s.read(fmt.Sprintf("host/claim-%08d.json", seq), 4096)
	if os.IsNotExist(e) {
		return 0, nil
	}
	var receipt cohortReceipt
	if e != nil || strict(raw, &receipt) != nil || receipt.Sequence != seq || receipt.Bytes < 0 || receipt.Bytes > (64<<10)+1 || !digestValue(receipt.ResponseSHA256) {
		return 0, ErrInvalid
	}
	return receipt.Bytes, nil
}
