package adr0011methodresult

import (
	"bytes"
	"encoding/json"
	"lsp-trace/internal/adr0011requestkey"
	"lsp-trace/internal/lspwire"
	"lsp-trace/internal/strictjson"
)

// evaluateCompleteReferencesAttached is a test-owned, successful-whole-result-only
// entry. The supplied sink is independent of parser items; neither result nor
// receipt is returned when any transition or readback fails. This is not issuance.
func evaluateCompleteReferencesAttached(key lspwire.RequestKey, raw json.RawMessage, owner referenceTransitionOwner, sink *referenceTransitionSink) (*ReferenceEvaluation, *referenceTransitionReceipt, error) {
	if sink == nil || sink.owner != owner || owner.RequestKey != adr0011requestkey.Encode(key) || owner.RawDigest != chainDigest(raw) {
		return nil, nil, errReferenceTransition
	}
	trimmed := bytes.TrimSpace(raw)
	var members []json.RawMessage
	if bytes.Equal(trimmed, []byte("null")) {
		members = []json.RawMessage{}
	} else if len(trimmed) == 0 || trimmed[0] != '[' || json.Unmarshal(trimmed, &members) != nil || members == nil || len(members) > 1000 {
		return nil, nil, errReferenceTransition
	}
	// No evaluator state or counters are initialized until this owner-keyed callback succeeds.
	if err := sink.transition(owner, "QUERY_BEGIN", -1); err != nil {
		return nil, nil, err
	}
	observed := &ReferenceEvaluation{Key: key, RawDigest: chainDigest(raw), N: 1, B: 1, E: len(members), FailureOrdinal: -1, Items: make([]Item, 0, len(members))}
	for ordinal, member := range members {
		if err := sink.transition(owner, "ELEMENT_BEGIN", ordinal); err != nil {
			return nil, nil, err
		}
		observed.Events = append(observed.Events, ReferenceEvaluationEvent{Ordinal: ordinal, Transition: "BEGIN"})
		observed.EB++
		valid := strictjson.RejectDuplicates(member) == nil
		item, ok := parseItem(member, ordinal)
		if !valid || !ok || item.Kind != Location {
			return nil, nil, errReferenceTransition
		}
		if err := sink.transition(owner, "ELEMENT_TERMINAL_VALID_PENDING_ADMISSION", ordinal); err != nil {
			return nil, nil, err
		}
		observed.Events = append(observed.Events, ReferenceEvaluationEvent{Ordinal: ordinal, Transition: "TERMINAL"})
		observed.ET++
		observed.Items = append(observed.Items, item)
	}
	terminal := "QUERY_TERMINAL_ITEMS"
	if len(members) == 0 {
		terminal = "QUERY_TERMINAL_EMPTY"
	}
	if err := sink.transition(owner, terminal, -1); err != nil {
		return nil, nil, err
	}
	observed.P = len(observed.Items)
	if len(members) == 0 {
		observed.Outcome = "COMPLETE_EMPTY"
		observed.Disposition = "EMPTY"
	} else {
		observed.Outcome = "COMPLETE"
		observed.Disposition = "ITEMS"
	}
	receipt, err := sink.seal(owner)
	if err != nil {
		return nil, nil, err
	}
	if err = verifyReferenceTransitionReceipt(sink.store, receipt); err != nil {
		return nil, nil, err
	}
	return observed, receipt, nil
}
