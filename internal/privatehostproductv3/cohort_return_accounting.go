//go:build darwin || linux

package privatehostproductv3

import (
	"bytes"
	"fmt"
	"os"
)

const cohortAccountingUnknown = "UNKNOWN_HISTORY_KNOWN_RETURNED_BYTES"

// A nonempty transport return is already spent, independently of whether a
// second copy is available, consistent or semantically valid. Persist its
// receipt first. Reconciliation never replaces those bytes with a cheaper copy.
// UNKNOWN counters describe the known prefix/returns, not exact whole history.
func retainCohortReturn(s *store, c CohortConfig, p *cohortProgress, seq int, claim, reply []byte, backendOK bool) ([]byte, error) {
	retain := func() error {
		p.ResponseBytes += len(reply)
		if e := s.write(fmt.Sprintf("host/claim-%08d.json", seq), mustJSON(cohortReceipt{seq, hash(reply), len(reply), backendOK})); e != nil {
			return e
		}
		if len(reply) > 0 {
			if e := s.write(fmt.Sprintf("host/response-%08d.json", seq), reply); e != nil {
				return e
			}
		}
		return cohortSave(s, *p)
	}
	refuse := func(e error) ([]byte, error) { p.AccountingValidation = cohortAccountingUnknown; return nil, e }
	retained := len(reply) > 0
	if retained {
		if e := retain(); e != nil {
			return refuse(e)
		}
	}
	observed, e := observedCohortReply(s, c, seq, claim)
	if e == nil {
		if retained && !bytes.Equal(reply, observed) {
			return refuse(ErrInvalid)
		}
		if !retained {
			reply = observed
		}
	} else if !os.IsNotExist(e) {
		return refuse(e)
	}
	if !retained {
		if e := retain(); e != nil {
			return refuse(e)
		}
	}
	return reply, nil
}
