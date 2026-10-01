package adr0011genericv5proposal

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"strconv"
)

// CheckB4bDefinitionSuccessor is a private, additive definition-only replay.
// The accepted references candidate and its eight-target test remain unchanged.
func CheckB4bDefinitionSuccessor(in B4bFullCandidateInput) string {
	if in.Write.Method != "textDocument/definition" || CheckB4a(in.Write) != nil {
		return "UNKNOWN"
	}
	if A4Validate("CAPABILITY_EVENTS", in.CapabilityEnvelope) != nil {
		return "MALFORMED"
	}
	var e b4aEnvelope
	if json.Unmarshal(in.CapabilityEnvelope, &e) != nil || e.Role != "CAPABILITY_EVENTS" || e.Identity.Session != in.Write.Session || e.Identity.Generation != in.Write.Generation || e.Identity.Transaction != in.Write.Transaction || !b4aDescriptor(e.Original, in.CapabilityOriginal) {
		return "UNKNOWN"
	}
	capSelector, err := A4Selector("definition", "CAPABILITY_EVENTS", in.Write.Session, strconv.FormatUint(in.Write.Generation, 10), in.CapabilityOriginal, b4aTransaction(in.Write))
	if err != nil {
		return "UNKNOWN"
	}
	bound := false
	for _, c := range in.Write.Claims {
		if c.Role != "REQUEST_WRITE" {
			continue
		}
		var w b4aEnvelope
		if json.Unmarshal(c.Envelope, &w) != nil {
			return "UNKNOWN"
		}
		for _, edge := range w.Predecessors {
			if edge.Role == "CAPABILITY_EVENTS" && edge.Selector == capSelector && edge.Digest == "sha256:"+a4Hash(in.CapabilityOriginal) {
				bound = true
			}
		}
	}
	if !bound {
		return "UNKNOWN"
	}
	parsed, ok := b4bFullParse(in)
	if !ok || !b4bDefinitionCorresponds(in, e, parsed) {
		return "UNKNOWN"
	}
	return b4bFullFold(in, parsed)
}

// Keep the original JSON number token in the /2 field while comparing typed
// bounded-decimal IDs semantically. The captured framed bytes remain independent.
func b4bDefinitionTokenID(raw json.RawMessage) (string, error) {
	id, err := parseID(raw)
	if err != nil {
		return "", err
	}
	if id.Kind == "STRING" {
		return "string:" + id.String, nil
	}
	return "number:" + string(id.RawToken), nil
}

func b4bDefinitionCorresponds(in B4bFullCandidateInput, envelope b4aEnvelope, parsed b4bFullParsed) bool {
	var payload struct {
		Initialize  b4bFullExchange     `json:"initialize"`
		Events      []b4bFullExchange   `json:"events"`
		Initialized b4bFullOccurrence   `json:"initialized_notification"`
		Observed    []b4bFullOccurrence `json:"observed_frames"`
		Target      uint64              `json:"target_write_frame_ordinal"`
	}
	if json.Unmarshal(envelope.Payload, &payload) != nil || payload.Target != in.Write.CompletedOrdinal || len(payload.Events) != len(parsed.events)-1 || len(payload.Observed) != len(parsed.observed) || len(parsed.events) > 8193 {
		return false
	}
	fields := [][]byte{[]byte("ADR0011-GENERIC-CAPABILITY-EXCHANGE-ARTIFACT/2"), []byte(b4aTransaction(in.Write)), []byte(strconv.FormatUint(in.Write.CompletedOrdinal, 10)), []byte(strconv.Itoa(len(parsed.events)))}
	add := func(v string) { fields = append(fields, []byte(v)) }
	for i, event := range parsed.events {
		f := in.Frames[event.request]
		m := parsed.messages[event.request]
		claim := payload.Initialize
		if i > 0 {
			claim = payload.Events[i-1]
		}
		if claim.RequestDirection != f.Direction || claim.RequestFrameOrdinal != f.Ordinal || claim.Target != in.Write.CompletedOrdinal || claim.RequestMethod != event.method || !b4aDescriptor(claim.RequestFrame, f.Bytes) || !b4bFullID(claim.RequestID, event.id) || !b4bJSON(claim.RequestParams, m["params"]) {
			return false
		}
		canonicalID, err := b4bDefinitionTokenID(m["id"])
		if err != nil {
			return false
		}
		add(f.Direction)
		fields = append(fields, f.Bytes)
		add(canonicalID)
		add(event.method)
		add(strconv.FormatUint(f.Ordinal, 10))
		if event.response < 0 {
			if claim.ResponseStatus != "PENDING" || string(claim.ResponseDirection) != "null" || string(claim.ResponseFrame) != "null" || string(claim.ResponseID) != "null" || string(claim.ResponseFrameOrdinal) != "null" || string(claim.ResponseResult) != "null" || string(claim.ResponseError) != "null" {
				return false
			}
			fields = append(fields, []byte("ABSENT"), nil, []byte("ABSENT"), []byte("-1"), []byte("PENDING"))
			continue
		}
		r := in.Frames[event.response]
		rm := parsed.messages[event.response]
		var descriptor b4aBytes
		if json.Unmarshal(claim.ResponseFrame, &descriptor) != nil || !b4aDescriptor(descriptor, r.Bytes) || claim.ResponseStatus != event.status || !b4bFullID(claim.ResponseID, event.id) || string(claim.ResponseDirection) != `"`+r.Direction+`"` {
			return false
		}
		var ordinal uint64
		if json.Unmarshal(claim.ResponseFrameOrdinal, &ordinal) != nil || ordinal != r.Ordinal {
			return false
		}
		result, ok := rm["result"]
		if !ok {
			result = []byte("null")
		}
		errorValue, ok := rm["error"]
		if !ok {
			errorValue = []byte("null")
		}
		if !b4bJSON(claim.ResponseResult, result) || !b4bJSON(claim.ResponseError, errorValue) {
			return false
		}
		responseID, err := b4bDefinitionTokenID(rm["id"])
		if err != nil || responseID != canonicalID {
			return false
		}
		add(r.Direction)
		fields = append(fields, r.Bytes)
		add(responseID)
		add(strconv.FormatUint(r.Ordinal, 10))
		add(event.status)
	}
	note := in.Frames[parsed.notification]
	if payload.Initialized.Ordinal != note.Ordinal || payload.Initialized.Direction != note.Direction || !b4aDescriptor(payload.Initialized.Frame, note.Bytes) {
		return false
	}
	add(note.Direction)
	add(strconv.FormatUint(note.Ordinal, 10))
	fields = append(fields, note.Bytes)
	add(strconv.Itoa(len(parsed.observed)))
	for i, j := range parsed.observed {
		frame := in.Frames[j]
		claim := payload.Observed[i]
		if claim.Ordinal != frame.Ordinal || claim.Direction != frame.Direction || !b4aDescriptor(claim.Frame, frame.Bytes) {
			return false
		}
		add(frame.Direction)
		add(strconv.FormatUint(frame.Ordinal, 10))
		fields = append(fields, frame.Bytes)
	}
	original := in.CapabilityOriginal
	for _, field := range fields {
		if len(original) < 8 {
			return false
		}
		n := binary.BigEndian.Uint64(original[:8])
		original = original[8:]
		if n != uint64(len(field)) || n > uint64(len(original)) || !bytes.Equal(original[:int(n)], field) {
			return false
		}
		original = original[int(n):]
	}
	return len(original) == 0
}
