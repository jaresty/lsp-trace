package adr0011methodresult

import (
	"bytes"
	"encoding/json"
	"lsp-trace/internal/lspwire"
	"lsp-trace/internal/strictjson"
)

// ReferenceEvaluation is a keyed, raw-result-local evaluation observation.
// It is not a manager transport observation or an issued terminal.
type ReferenceEvaluation struct {
	Key                      lspwire.RequestKey
	RawDigest                string
	Events                   []ReferenceEvaluationEvent
	N, B, T, E, EB, ET, P, A int
	Outcome, Disposition     string
	FailureOrdinal           int
	Items                    []Item
}
type ReferenceEvaluationEvent struct {
	Ordinal    int
	Transition string
}

func evaluateCompleteReferences(key lspwire.RequestKey, raw json.RawMessage) ReferenceEvaluation {
	observed := ReferenceEvaluation{Key: key, RawDigest: chainDigest(raw), N: 1, B: 1, FailureOrdinal: -1}
	trimmed := bytes.TrimSpace(raw)
	if !json.Valid(trimmed) {
		observed.Outcome = "MALFORMED"
		observed.Disposition = "MALFORMED"
		return observed
	}
	if bytes.Equal(trimmed, []byte("null")) {
		observed.Outcome = "COMPLETE_EMPTY"
		observed.Disposition = "EMPTY"
		return observed
	}
	var members []json.RawMessage
	if len(trimmed) == 0 || trimmed[0] != '[' || json.Unmarshal(trimmed, &members) != nil || members == nil {
		observed.Outcome = "MALFORMED"
		observed.Disposition = "MALFORMED"
		return observed
	}
	observed.E = len(members) // Counted from the complete raw array envelope, never parsed Items.
	if observed.E > 1000 {
		observed.Outcome = "RESOURCE_LIMIT"
		observed.Disposition = "LIMITED"
		return observed
	}
	observed.Items = make([]Item, 0, len(members))
	for ordinal, member := range members {
		observed.Events = append(observed.Events, ReferenceEvaluationEvent{Ordinal: ordinal, Transition: "BEGIN"})
		observed.EB++
		valid := strictjson.RejectDuplicates(member) == nil
		item, ok := parseItem(member, ordinal)
		observed.Events = append(observed.Events, ReferenceEvaluationEvent{Ordinal: ordinal, Transition: "TERMINAL"})
		observed.ET++
		if !valid || !ok || item.Kind != Location {
			observed.Outcome = "MALFORMED"
			observed.Disposition = "MALFORMED"
			observed.FailureOrdinal = ordinal
			observed.Items = nil
			return observed
		}
		observed.Items = append(observed.Items, item)
	}
	observed.P = len(observed.Items)
	// Parsing produces candidates, not independently verified issued occurrences.
	// T and A remain zero until a separate final-readback issuance path exists.
	if observed.E == 0 {
		observed.Outcome = "COMPLETE_EMPTY"
		observed.Disposition = "EMPTY"
	} else {
		observed.Outcome = "COMPLETE"
		observed.Disposition = "ITEMS"
	}
	return observed
}
