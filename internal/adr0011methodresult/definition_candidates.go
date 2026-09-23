package adr0011methodresult

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"

	transport "lsp-trace/internal/adr0011methodtransport"
)

var ErrDefinitionCandidate = errors.New("unadmitted definition candidate projection unavailable")

// DefinitionCandidate is a server-result-shaped proposal, not an admitted
// RESOLVES_TO_DEFINITION relation, a verified source body, or an input to Leiden.
// Repeated returned targets retain separate occurrence IDs and ordinals even
// when they share a transaction-scoped target ID. This ID is not a canonical
// cross-query symbol identity. A query occurrence ID is caller-declared here.
type DefinitionCandidate struct {
	OccurrenceID, QueryOccurrenceID, TargetID string
	Ordinal                                   int
	QueryURI                                  string
	QueryLine, QueryCharacter                 uint32
	TargetURI                                 string
	TargetKind                                Kind
	TargetSelectionRange                      Range
	TargetRange                               *Range
}

func definitionCandidateID(domain string, value any) string {
	raw, _ := json.Marshal(value)
	sum := sha256.Sum256(append([]byte(domain+"\x00"), raw...))
	return "sha256:" + hex.EncodeToString(sum[:])
}

// ProjectDefinitionCandidates requires a replay-checked untrusted transaction
// and balanced one-query candidate ledger. It does not mint a method receipt or
// terminal verdict and cannot accept a references query or infer its target.
func ProjectDefinitionCandidates(raw []byte, ledger CandidateLedger, queryOccurrenceID string) ([]DefinitionCandidate, error) {
	if len(queryOccurrenceID) == 0 || len(queryOccurrenceID) > 1024 || CheckCandidateTransactionLedger(raw, ledger, queryOccurrenceID) != nil {
		return nil, ErrDefinitionCandidate
	}
	c, err := VerifyCanonicalCandidate(raw)
	if err != nil || c.Method != transport.MethodDefinition {
		return nil, ErrDefinitionCandidate
	}
	result, err := CandidateResultBytes(raw)
	if err != nil {
		return nil, ErrDefinitionCandidate
	}
	parsed, failure := parseRawUntrusted(c.Method, result, c.MaxCandidates)
	if failure != nil || len(parsed.Items) != c.ItemCount {
		return nil, ErrDefinitionCandidate
	}
	out := make([]DefinitionCandidate, 0, len(parsed.Items))
	for _, item := range parsed.Items {
		target := struct {
			TransactionID string
			Kind          Kind
			URI           string
			Selection     Range
			TargetRange   *Range
		}{c.ID, item.Kind, item.URI, item.Range, item.TargetRange}
		targetID := definitionCandidateID("lsp-trace:adr0011:unadmitted-definition-target:v0", target)
		identity := struct {
			TransactionID, QueryOccurrenceID, TargetID string
			Ordinal                                    int
		}{c.ID, queryOccurrenceID, targetID, item.Ordinal}
		candidate := DefinitionCandidate{OccurrenceID: definitionCandidateID("lsp-trace:adr0011:unadmitted-definition-occurrence:v0", identity),
			QueryOccurrenceID: queryOccurrenceID, TargetID: targetID, Ordinal: item.Ordinal,
			QueryURI: c.QueryURI, QueryLine: c.QueryLine, QueryCharacter: c.QueryCharacter,
			TargetURI: item.URI, TargetKind: item.Kind, TargetSelectionRange: item.Range}
		if item.TargetRange != nil {
			r := *item.TargetRange
			candidate.TargetRange = &r
		}
		out = append(out, candidate)
	}
	return out, nil
}
