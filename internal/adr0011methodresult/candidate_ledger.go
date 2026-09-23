package adr0011methodresult

import (
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	transport "lsp-trace/internal/adr0011methodtransport"
)

// CandidateLedger is an untrusted, private consistency input. Neither its
// declarations nor its begun/terminal flags are verified owner observations.
// A passing check never issues a method receipt, request outcome, query-target
// identity, admitted occurrence, or COMPLETE_EMPTY verdict.
type CandidateLedger struct {
	Queries []CandidateQuery
	Members []CandidateQueryMember
}

type CandidateQuery struct {
	OccurrenceID string
	SessionID    string
	Generation   uint64
	Method       string
	ParamsSHA256 string
}

type CandidateQueryTerminal string

const (
	CandidateEmpty     CandidateQueryTerminal = "OBSERVED_EMPTY"
	CandidateItems     CandidateQueryTerminal = "OBSERVED_ITEMS"
	CandidateMalformed CandidateQueryTerminal = "OBSERVED_MALFORMED"
	CandidateLimited   CandidateQueryTerminal = "OBSERVED_LIMITED"
)

type CandidateElementTerminal string

const (
	CandidateValidElement     CandidateElementTerminal = "VALID"
	CandidateMalformedElement CandidateElementTerminal = "MALFORMED"
	CandidateLimitedElement   CandidateElementTerminal = "LIMITED"
)

type CandidateElement struct {
	Ordinal  int
	Began    bool
	Terminal CandidateElementTerminal
}

type CandidateQueryMember struct {
	OccurrenceID string
	Began        bool
	Terminal     CandidateQueryTerminal
	Envelope     EnvelopeObservation
	Elements     []CandidateElement
}

// CheckCandidateLedger checks only supplied facts against one another. The
// terminal vocabulary is intentionally private and provisional, not ADR 0011's
// request/member outcome contract. Presence of a member denotes one begun
// query; presence of an element denotes one begun element. No producer or
// observation is authenticated and no admission count is returned.
func CheckCandidateLedger(ledger CandidateLedger) error {
	if len(ledger.Queries) == 0 || len(ledger.Queries) > 16 {
		return errors.New("candidate query count outside private bound")
	}
	declared := make(map[string]CandidateQuery, len(ledger.Queries))
	for _, q := range ledger.Queries {
		if q.OccurrenceID == "" || q.SessionID == "" || q.Generation == 0 ||
			(q.Method != transport.MethodDefinition && q.Method != transport.MethodReferences) || !candidateDigest(q.ParamsSHA256) {
			return errors.New("invalid candidate query declaration")
		}
		if _, duplicate := declared[q.OccurrenceID]; duplicate {
			return errors.New("duplicate candidate query occurrence")
		}
		declared[q.OccurrenceID] = q
	}
	seen := make(map[string]bool, len(ledger.Members))
	for _, member := range ledger.Members {
		q, ok := declared[member.OccurrenceID]
		if !ok || seen[member.OccurrenceID] || !member.Began {
			return errors.New("undeclared, duplicate, or unbegun query member")
		}
		seen[member.OccurrenceID] = true
		if err := checkCandidateMember(q.Method, member); err != nil {
			return fmt.Errorf("candidate query %q: %w", member.OccurrenceID, err)
		}
	}
	return nil
}

func candidateDigest(value string) bool {
	if !strings.HasPrefix(value, "sha256:") || len(value) != len("sha256:")+64 {
		return false
	}
	_, err := hex.DecodeString(value[len("sha256:"):])
	return err == nil && strings.ToLower(value) == value
}

func checkCandidateMember(method string, member CandidateQueryMember) error {
	e := member.Envelope
	if e.Known {
		if e.Reason != "" || e.Count < 0 || e.Form != EnvelopeNull && e.Form != EnvelopeArray && e.Form != EnvelopeDefinitionScalar ||
			e.Form == EnvelopeNull && e.Count != 0 || e.Form == EnvelopeDefinitionScalar && (method != transport.MethodDefinition || e.Count != 1) {
			return errors.New("inconsistent known result shape")
		}
	} else if e.Reason == "" || e.Form != "" || e.Count != 0 || e.Reason == EnvelopeInvalidLimit {
		return errors.New("invalid unknown result shape")
	}
	// A failed transport or absent raw result does not establish that evaluation
	// began. A later owner-bound observation would need a separate contract for
	// a post-output failure; this candidate validator cannot invent one.
	if !e.Known && e.Reason != EnvelopeMalformed && e.Reason != EnvelopeOverLimit {
		return errors.New("no evaluable result for a begun member")
	}
	if !e.Known && len(member.Elements) != 0 {
		return errors.New("element count unknown")
	}
	if e.Known && len(member.Elements) > e.Count {
		return errors.New("more begun elements than returned elements")
	}
	for ordinal, element := range member.Elements {
		if !element.Began || element.Ordinal != ordinal ||
			(element.Terminal != "" && element.Terminal != CandidateValidElement && element.Terminal != CandidateMalformedElement && element.Terminal != CandidateLimitedElement) {
			return errors.New("unbegun, out-of-order, or invalid element")
		}
		if ordinal+1 < len(member.Elements) && (element.Terminal == "" || element.Terminal == CandidateMalformedElement || element.Terminal == CandidateLimitedElement) {
			return errors.New("evaluation continued after non-valid element")
		}
		if member.Terminal != "" && element.Terminal == "" {
			return errors.New("terminal query has unterminated begun element")
		}
	}
	switch member.Terminal {
	case "":
		return nil // Begun but not terminal: no request completion follows.
	case CandidateEmpty:
		if !e.Known || e.Count != 0 || len(member.Elements) != 0 {
			return errors.New("empty query lacks complete zero-element result")
		}
	case CandidateItems:
		if !e.Known || e.Count == 0 || len(member.Elements) != e.Count || member.Elements[len(member.Elements)-1].Terminal != CandidateValidElement {
			return errors.New("items query lacks fully evaluated result")
		}
	case CandidateMalformed:
		if !e.Known {
			if e.Reason != EnvelopeMalformed {
				return errors.New("malformed query lacks malformed result")
			}
		} else if len(member.Elements) == 0 || member.Elements[len(member.Elements)-1].Terminal != CandidateMalformedElement {
			return errors.New("malformed query lacks failing element")
		}
	case CandidateLimited:
		if !e.Known {
			if e.Reason != EnvelopeOverLimit {
				return errors.New("limited query lacks over-limit result")
			}
		} else if len(member.Elements) == 0 || member.Elements[len(member.Elements)-1].Terminal != CandidateLimitedElement {
			return errors.New("limited query lacks limited element")
		}
	default:
		return errors.New("unknown candidate query terminal")
	}
	return nil
}
