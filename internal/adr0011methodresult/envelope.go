package adr0011methodresult

import (
	"bytes"
	"encoding/json"

	transport "lsp-trace/internal/adr0011methodtransport"
)

// EnvelopeObservation is a provisional, syntax-only result observation. It is
// not a terminal request-member ledger, method receipt, or occurrence.
type EnvelopeObservation struct {
	Known  bool
	Form   EnvelopeForm
	Count  int
	Reason EnvelopeUnknownReason
}

type EnvelopeForm string

const (
	EnvelopeNull             EnvelopeForm = "NULL"
	EnvelopeArray            EnvelopeForm = "ARRAY"
	EnvelopeDefinitionScalar EnvelopeForm = "DEFINITION_SCALAR"
)

type EnvelopeUnknownReason string

const (
	EnvelopeNonSuccess   EnvelopeUnknownReason = "NON_SUCCESS"
	EnvelopeInvalidLimit EnvelopeUnknownReason = "INVALID_LIMIT"
	EnvelopeUnavailable  EnvelopeUnknownReason = "RESULT_UNAVAILABLE"
	EnvelopeMalformed    EnvelopeUnknownReason = "MALFORMED"
	EnvelopeOverLimit    EnvelopeUnknownReason = "OVER_LIMIT"
)

// ObserveEnvelope counts top-level elements in a complete, bounded method
// result. A count includes syntactically present malformed members; it does not
// validate a Location, evaluate query members, or establish request completion.
// Raw bytes are copied from the private bounded transport and never retained.
func ObserveEnvelope(wire transport.Result, maxElements int) EnvelopeObservation {
	if wire.Outcome() != transport.OutcomeTransportSuccess {
		return EnvelopeObservation{Reason: EnvelopeNonSuccess}
	}
	if maxElements <= 0 {
		return EnvelopeObservation{Reason: EnvelopeInvalidLimit}
	}
	raw := wire.Raw()
	if raw == nil {
		return EnvelopeObservation{Reason: EnvelopeUnavailable}
	}
	if !json.Valid(raw) {
		return EnvelopeObservation{Reason: EnvelopeMalformed}
	}
	trimmed := bytes.TrimSpace(raw)
	if bytes.Equal(trimmed, []byte("null")) {
		return EnvelopeObservation{Known: true, Form: EnvelopeNull}
	}
	if len(trimmed) == 0 {
		return EnvelopeObservation{Reason: EnvelopeMalformed}
	}
	if trimmed[0] == '{' && wire.Method() == transport.MethodDefinition {
		return EnvelopeObservation{Known: true, Form: EnvelopeDefinitionScalar, Count: 1}
	}
	if trimmed[0] != '[' || wire.Method() != transport.MethodDefinition && wire.Method() != transport.MethodReferences {
		return EnvelopeObservation{Reason: EnvelopeMalformed}
	}
	dec := json.NewDecoder(bytes.NewReader(trimmed))
	if _, err := dec.Token(); err != nil {
		return EnvelopeObservation{Reason: EnvelopeMalformed}
	}
	count := 0
	for dec.More() {
		if count >= maxElements {
			return EnvelopeObservation{Reason: EnvelopeOverLimit}
		}
		var member json.RawMessage
		if err := dec.Decode(&member); err != nil {
			return EnvelopeObservation{Reason: EnvelopeMalformed}
		}
		count++
	}
	if _, err := dec.Token(); err != nil {
		return EnvelopeObservation{Reason: EnvelopeMalformed}
	}
	return EnvelopeObservation{Known: true, Form: EnvelopeArray, Count: count}
}
