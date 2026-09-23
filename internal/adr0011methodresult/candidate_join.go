package adr0011methodresult

import (
	"bytes"
	"encoding/base64"
	"errors"

	transport "lsp-trace/internal/adr0011methodtransport"
)

var ErrCandidateJoin = errors.New("untrusted query/result candidate and ledger do not balance")

// CheckCandidateTransactionLedger joins exactly one untrusted, predeclared query
// member to one replay-checked query/result candidate. It refuses a declaration
// manufactured from returned locations and returns no admitted occurrence,
// referenced target identity, completeness verdict, or Leiden input. Neither
// the candidate nor the ledger authenticates its producer or declaration.
func CheckCandidateTransactionLedger(raw []byte, ledger CandidateLedger, occurrenceID string) error {
	if occurrenceID == "" || len(ledger.Queries) != 1 || len(ledger.Members) != 1 || CheckCandidateLedger(ledger) != nil {
		return ErrCandidateJoin
	}
	c, err := VerifyCanonicalCandidate(raw)
	if err != nil {
		return ErrCandidateJoin
	}
	q, member := ledger.Queries[0], ledger.Members[0]
	if q.OccurrenceID != occurrenceID || member.OccurrenceID != occurrenceID ||
		q.SessionID != c.SessionID || q.Generation != c.Generation || q.Method != c.Method || q.ParamsSHA256 != c.ParamsSHA256 ||
		!member.Began || !member.Envelope.Known || member.Envelope.Reason != "" || member.Envelope.Count != c.ItemCount {
		return ErrCandidateJoin
	}
	result, err := base64.StdEncoding.DecodeString(c.ResultBase64)
	if err != nil { // VerifyCanonicalCandidate already validated this encoding.
		return ErrCandidateJoin
	}
	trimmed := bytes.TrimSpace(result)
	form := EnvelopeArray
	if c.Null {
		form = EnvelopeNull
	} else if len(trimmed) != 0 && trimmed[0] == '{' && c.Method == transport.MethodDefinition {
		form = EnvelopeDefinitionScalar
	}
	if member.Envelope.Form != form {
		return ErrCandidateJoin
	}
	if c.ItemCount == 0 {
		if member.Terminal != CandidateEmpty || len(member.Elements) != 0 {
			return ErrCandidateJoin
		}
		return nil
	}
	if member.Terminal != CandidateItems || len(member.Elements) != c.ItemCount {
		return ErrCandidateJoin
	}
	for ordinal, element := range member.Elements {
		if !element.Began || element.Ordinal != ordinal || element.Terminal != CandidateValidElement {
			return ErrCandidateJoin
		}
	}
	return nil
}
