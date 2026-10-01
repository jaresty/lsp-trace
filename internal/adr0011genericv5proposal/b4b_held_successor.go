package adr0011genericv5proposal

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"reflect"
	"strconv"
	"strings"
)

// B4bHeldSuccessorFrame is a separately held, ordered framed JSON-RPC message.
type B4bHeldSuccessorFrame struct {
	Ordinal   uint64
	Direction string
	Bytes     []byte
}

// B4bHeldSuccessorInput never obtains held originals from claimant descriptors.
type B4bHeldSuccessorInput struct {
	Write                                  B4aInput
	CapabilityEnvelope, CapabilityOriginal []byte
	Frames                                 []B4bHeldSuccessorFrame
	ClientSelector                         []byte
}

// CheckB4bHeldSuccessor is a private, provisional classification, not admission.
// This increment recognizes only a held successful null-selector registration.
func CheckB4bHeldSuccessor(in B4bHeldSuccessorInput) string {
	if err := CheckB4a(in.Write); err != nil {
		return "UNKNOWN"
	}
	if err := A4Validate("CAPABILITY_EVENTS", in.CapabilityEnvelope); err != nil {
		return "MALFORMED"
	}
	var envelope b4aEnvelope
	if json.Unmarshal(in.CapabilityEnvelope, &envelope) != nil || envelope.Role != "CAPABILITY_EVENTS" || envelope.Identity.Session != in.Write.Session || envelope.Identity.Generation != in.Write.Generation || envelope.Identity.Transaction != in.Write.Transaction || !b4aDescriptor(envelope.Original, in.CapabilityOriginal) {
		return "UNKNOWN"
	}
	var payload struct {
		Initialize  b4bHeldSuccessorExchange `json:"initialize"`
		Initialized struct {
			Direction    string   `json:"direction"`
			FrameOrdinal uint64   `json:"frame_ordinal"`
			Frame        b4aBytes `json:"frame"`
		} `json:"initialized_notification"`
		Events   []b4bHeldSuccessorExchange `json:"events"`
		Target   uint64                     `json:"target_write_frame_ordinal"`
		Observed []struct {
			Direction string   `json:"direction"`
			Ordinal   uint64   `json:"frame_ordinal"`
			Frame     b4aBytes `json:"frame"`
		} `json:"observed_frames"`
	}
	if json.Unmarshal(envelope.Payload, &payload) != nil || payload.Target != in.Write.CompletedOrdinal {
		return "UNKNOWN"
	}
	// Parse every supplied known frame before evaluating whether chronology completed.
	messages := make([]map[string]json.RawMessage, len(in.Frames))
	for i, f := range in.Frames {
		if f.Ordinal != uint64(i) || (f.Direction != "CLIENT_TO_SERVER" && f.Direction != "SERVER_TO_CLIENT") {
			return "UNKNOWN"
		}
		m, err := b4aFrame(f.Bytes)
		if err != nil {
			return "MALFORMED"
		}
		messages[i] = m
	}
	if len(in.ClientSelector) > 0 && !json.Valid(in.ClientSelector) {
		return "MALFORMED"
	}
	if len(in.Frames) < 4 {
		return "UNKNOWN"
	}
	check := func(index int, direction string, desc b4aBytes) bool {
		return index < len(in.Frames) && in.Frames[index].Direction == direction && b4aDescriptor(desc, in.Frames[index].Bytes)
	}
	if len(payload.Observed) != 5 || len(payload.Events) != 1 || payload.Initialize.RequestFrameOrdinal != 0 || payload.Initialize.ResponseFrameOrdinal != 1 || payload.Initialized.FrameOrdinal != 2 || payload.Events[0].RequestFrameOrdinal != 3 || payload.Events[0].ResponseFrameOrdinal != 4 || !check(0, "CLIENT_TO_SERVER", payload.Initialize.RequestFrame) || !check(1, "SERVER_TO_CLIENT", payload.Initialize.ResponseFrame) || !check(2, "CLIENT_TO_SERVER", payload.Initialized.Frame) || !check(3, "SERVER_TO_CLIENT", payload.Events[0].RequestFrame) {
		return "UNKNOWN"
	}
	for i, record := range payload.Observed {
		if record.Ordinal != uint64(i) || !check(i, record.Direction, record.Frame) {
			if i >= len(in.Frames) {
				break
			}
			return "UNKNOWN"
		}
	}
	if !b4bHeldSuccessorID(messages[0]["id"], []byte("1")) || !b4bHeldSuccessorID(messages[1]["id"], messages[0]["id"]) || !b4bHeldSuccessorString(messages[0]["method"], "initialize") || payload.Initialize.RequestMethod != "initialize" || payload.Initialize.ResponseStatus != "SUCCESS" || !b4bHeldSuccessorJSONEqual(messages[0]["params"], payload.Initialize.RequestParams) || !b4bHeldSuccessorJSONEqual(messages[1]["result"], payload.Initialize.ResponseResult) || !b4bHeldSuccessorString(messages[2]["method"], "initialized") || !b4bHeldSuccessorString(messages[3]["method"], "client/registerCapability") || !b4bHeldSuccessorID(messages[3]["id"], []byte(`"r"`)) || !b4bHeldSuccessorJSONEqual(messages[3]["params"], payload.Events[0].RequestParams) {
		return "UNKNOWN"
	}
	var initResult struct {
		Capabilities map[string]json.RawMessage `json:"capabilities"`
	}
	if json.Unmarshal(messages[1]["result"], &initResult) != nil || initResult.Capabilities == nil {
		return "MALFORMED"
	}
	if static, exists := initResult.Capabilities["referencesProvider"]; exists && !bytes.Equal(bytes.TrimSpace(static), []byte("false")) && !bytes.Equal(bytes.TrimSpace(static), []byte("null")) {
		return "UNKNOWN"
	}
	var register struct {
		Registrations []struct {
			Method  string `json:"method"`
			Options struct {
				Selector json.RawMessage `json:"documentSelector"`
			} `json:"registerOptions"`
		} `json:"registrations"`
	}
	if json.Unmarshal(messages[3]["params"], &register) != nil || len(register.Registrations) != 1 || register.Registrations[0].Method != in.Write.Method {
		return "MALFORMED"
	}
	claimedSelector := register.Registrations[0].Options.Selector
	if len(claimedSelector) == 0 {
		return "MALFORMED"
	}
	if !bytes.Equal(bytes.TrimSpace(claimedSelector), []byte("null")) {
		return "UNKNOWN"
	}
	if len(in.Frames) < 5 {
		return "UNKNOWN"
	}
	if !b4bHeldSuccessorArtifactExact(in.CapabilityOriginal, in.Write, in.Frames[:5], messages[:5]) {
		return "UNKNOWN"
	}
	event := payload.Events[0]
	if !check(4, "CLIENT_TO_SERVER", event.ResponseFrame) || event.ResponseStatus != "SUCCESS" || !b4bHeldSuccessorID(messages[4]["id"], messages[3]["id"]) || !bytes.Equal(bytes.TrimSpace(messages[4]["result"]), []byte("null")) || !bytes.Equal(bytes.TrimSpace(event.ResponseResult), []byte("null")) || len(messages[4]["error"]) > 0 {
		return "UNKNOWN"
	}
	if len(in.Frames) != 6 || !bytes.Equal(in.Frames[5].Bytes, in.Write.RequestFrame) || in.Frames[5].Direction != "CLIENT_TO_SERVER" || !b4bHeldSuccessorID(messages[5]["id"], in.Write.RequestID) || !b4bHeldSuccessorString(messages[5]["method"], in.Write.Method) || !bytes.Equal(messages[5]["params"], in.Write.RequestParams) {
		return "UNKNOWN"
	}
	if len(in.ClientSelector) == 0 {
		return "UNKNOWN"
	}
	var filters []map[string]json.RawMessage
	if json.Unmarshal(in.ClientSelector, &filters) != nil || filters == nil || len(filters) == 0 {
		return "MALFORMED"
	}
	schemes := make([]string, len(filters))
	for i, filter := range filters {
		raw, ok := filter["scheme"]
		if !ok || json.Unmarshal(raw, &schemes[i]) != nil || !b4bHeldSuccessorScheme(schemes[i]) {
			return "MALFORMED"
		}
	}
	for _, scheme := range schemes {
		if len(in.Write.URI) > len(scheme) && strings.EqualFold(in.Write.URI[:len(scheme)], scheme) && in.Write.URI[len(scheme)] == ':' {
			return "SUPPORTED"
		}
	}
	return "UNSUPPORTED"
}

func b4bHeldSuccessorScheme(s string) bool {
	if len(s) == 0 || !((s[0] >= 'a' && s[0] <= 'z') || (s[0] >= 'A' && s[0] <= 'Z')) {
		return false
	}
	for i := 1; i < len(s); i++ {
		c := s[i]
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '+' || c == '.' || c == '-') {
			return false
		}
	}
	return true
}

// Expected /2 fields derive only from held B4a custody and framed occurrences.
func b4bHeldSuccessorArtifactExact(original []byte, write B4aInput, frames []B4bHeldSuccessorFrame, messages []map[string]json.RawMessage) bool {
	if len(frames) != 5 || len(messages) != 5 {
		return false
	}
	fields := [][]byte{
		[]byte("ADR0011-GENERIC-CAPABILITY-EXCHANGE-ARTIFACT/2"),
		[]byte(b4aTransaction(write)),
		[]byte(strconv.FormatUint(write.CompletedOrdinal, 10)),
		[]byte("2"),
	}
	add := func(s string) { fields = append(fields, []byte(s)) }
	for _, pair := range [][2]int{{0, 1}, {3, 4}} {
		req, resp := pair[0], pair[1]
		requestID, err := b4bHeldSuccessorTypedID(messages[req]["id"])
		if err != nil {
			return false
		}
		responseID, err := b4bHeldSuccessorTypedID(messages[resp]["id"])
		if err != nil || requestID != responseID {
			return false
		}
		var method string
		if json.Unmarshal(messages[req]["method"], &method) != nil || method == "" {
			return false
		}
		// Response status comes from the strictly parsed observed response.
		status := ""
		_, result := messages[resp]["result"]
		_, failure := messages[resp]["error"]
		switch {
		case result && !failure:
			status = "SUCCESS"
		case failure && !result:
			status = "ERROR"
		default:
			return false
		}
		add(frames[req].Direction)
		fields = append(fields, frames[req].Bytes)
		add(requestID)
		add(method)
		add(strconv.FormatUint(frames[req].Ordinal, 10))
		add(frames[resp].Direction)
		fields = append(fields, frames[resp].Bytes)
		add(responseID)
		add(strconv.FormatUint(frames[resp].Ordinal, 10))
		add(status)
	}
	add(frames[2].Direction)
	add(strconv.FormatUint(frames[2].Ordinal, 10))
	fields = append(fields, frames[2].Bytes)
	add(strconv.Itoa(len(frames)))
	for _, frame := range frames {
		add(frame.Direction)
		add(strconv.FormatUint(frame.Ordinal, 10))
		fields = append(fields, frame.Bytes)
	}
	for _, expected := range fields {
		if len(original) < 8 {
			return false
		}
		n := binary.BigEndian.Uint64(original[:8])
		original = original[8:]
		if n != uint64(len(expected)) || n > uint64(len(original)) || !bytes.Equal(original[:int(n)], expected) {
			return false
		}
		original = original[int(n):]
	}
	return len(original) == 0
}

func b4bHeldSuccessorTypedID(raw json.RawMessage) (string, error) {
	id, err := parseID(raw)
	if err != nil {
		return "", err
	}
	if id.Kind == "STRING" {
		return "string:" + id.String, nil
	}
	value := id.Digits
	if id.Power != 0 {
		value += "e" + strconv.FormatInt(id.Power, 10)
	}
	if id.Negative {
		value = "-" + value
	}
	return "number:" + value, nil
}

type b4bHeldSuccessorExchange struct {
	RequestFrame         b4aBytes        `json:"request_frame"`
	ResponseFrame        b4aBytes        `json:"response_frame"`
	RequestFrameOrdinal  uint64          `json:"request_frame_ordinal"`
	ResponseFrameOrdinal uint64          `json:"response_frame_ordinal"`
	RequestMethod        string          `json:"request_method"`
	RequestParams        json.RawMessage `json:"request_params"`
	ResponseResult       json.RawMessage `json:"response_result"`
	ResponseStatus       string          `json:"response_status"`
}

func b4bHeldSuccessorJSONEqual(a, b []byte) bool {
	var left, right any
	return json.Unmarshal(a, &left) == nil && json.Unmarshal(b, &right) == nil && reflect.DeepEqual(left, right)
}
func b4bHeldSuccessorID(a, b []byte) bool {
	return len(a) > 0 && bytes.Equal(bytes.TrimSpace(a), bytes.TrimSpace(b))
}
func b4bHeldSuccessorString(raw []byte, value string) bool {
	var s string
	return json.Unmarshal(raw, &s) == nil && s == value
}
