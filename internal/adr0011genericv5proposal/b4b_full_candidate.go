package adr0011genericv5proposal

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"strconv"
	"strings"
)

// B4bFullCandidateInput carries independently held synthetic originals.
// Descriptors and claimed outcomes never supply original bytes or replay state.
type B4bFullCandidateInput struct {
	Write                                  B4aInput
	CapabilityEnvelope, CapabilityOriginal []byte
	Frames                                 []B4Frame
	HeldClientSelector                     []byte
}

type b4bFullOccurrence struct {
	Direction string   `json:"direction"`
	Ordinal   uint64   `json:"frame_ordinal"`
	Frame     b4aBytes `json:"frame"`
}
type b4bFullExchange struct {
	RequestDirection     string          `json:"request_direction"`
	RequestFrame         b4aBytes        `json:"request_frame"`
	RequestFrameOrdinal  uint64          `json:"request_frame_ordinal"`
	RequestID            json.RawMessage `json:"request_id"`
	RequestMethod        string          `json:"request_method"`
	RequestParams        json.RawMessage `json:"request_params"`
	ResponseDirection    json.RawMessage `json:"response_direction"`
	ResponseFrame        json.RawMessage `json:"response_frame"`
	ResponseFrameOrdinal json.RawMessage `json:"response_frame_ordinal"`
	ResponseID           json.RawMessage `json:"response_id"`
	ResponseStatus       string          `json:"response_status"`
	ResponseResult       json.RawMessage `json:"response_result"`
	ResponseError        json.RawMessage `json:"response_error"`
	Target               uint64          `json:"target_write_frame_ordinal"`
}
type b4bFullEvent struct {
	request, response int
	method, status    string
	id                ID
	params            json.RawMessage
}
type b4bFullParsed struct {
	messages     []map[string]json.RawMessage
	events       []b4bFullEvent
	notification int
	observed     []int
}

// CheckB4bFullCandidate is a private, provisional, time-bounded classification.
// It neither authenticates synthetic originals nor admits, retains, or issues evidence.
func CheckB4bFullCandidate(in B4bFullCandidateInput) string {
	if CheckB4a(in.Write) != nil {
		return "UNKNOWN"
	}
	if A4Validate("CAPABILITY_EVENTS", in.CapabilityEnvelope) != nil {
		return "MALFORMED"
	}
	var e b4aEnvelope
	if json.Unmarshal(in.CapabilityEnvelope, &e) != nil || e.Role != "CAPABILITY_EVENTS" || e.Identity.Session != in.Write.Session || e.Identity.Generation != in.Write.Generation || e.Identity.Transaction != in.Write.Transaction || !b4aDescriptor(e.Original, in.CapabilityOriginal) {
		return "UNKNOWN"
	}
	capSelector, err := A4Selector(strings.TrimPrefix(in.Write.Method, "textDocument/"), "CAPABILITY_EVENTS", in.Write.Session, strconv.FormatUint(in.Write.Generation, 10), in.CapabilityOriginal, b4aTransaction(in.Write))
	if err != nil {
		return "UNKNOWN"
	}
	bound := false
	for _, claim := range in.Write.Claims {
		if claim.Role != "REQUEST_WRITE" {
			continue
		}
		var w b4aEnvelope
		if json.Unmarshal(claim.Envelope, &w) != nil {
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
	if !ok || !b4bFullCorresponds(in, e, parsed) {
		return "UNKNOWN"
	}
	return b4bFullFold(in, parsed)
}

func b4bFullParse(in B4bFullCandidateInput) (b4bFullParsed, bool) {
	var out b4bFullParsed
	out.notification = -1
	if len(in.Frames) == 0 {
		return out, false
	}
	out.messages = make([]map[string]json.RawMessage, len(in.Frames))
	var pending []int
	var prev uint64
	for i, f := range in.Frames {
		if i > 0 && f.Ordinal <= prev || (f.Direction != "CLIENT_TO_SERVER" && f.Direction != "SERVER_TO_CLIENT") {
			return out, false
		}
		prev = f.Ordinal
		m, err := b4aFrame(f.Bytes)
		if err != nil || string(m["jsonrpc"]) != `"2.0"` {
			return out, false
		}
		out.messages[i] = m
		if raw, hasMethod := m["method"]; hasMethod {
			var method string
			if json.Unmarshal(raw, &method) != nil || len(m["result"]) > 0 || len(m["error"]) > 0 {
				return out, false
			}
			switch method {
			case "initialize", "client/registerCapability", "client/unregisterCapability":
				id, err := parseID(m["id"])
				if err != nil || (method == "initialize") != (f.Direction == "CLIENT_TO_SERVER") {
					return out, false
				}
				for _, j := range pending {
					if out.events[j].id.SameValue(id) {
						return out, false
					}
				}
				out.events = append(out.events, b4bFullEvent{request: i, response: -1, method: method, id: id, params: m["params"]})
				pending = append(pending, len(out.events)-1)
				out.observed = append(out.observed, i)
			case "initialized":
				if out.notification >= 0 || f.Direction != "CLIENT_TO_SERVER" || len(m["id"]) != 0 {
					return out, false
				}
				out.notification = i
				out.observed = append(out.observed, i)
			default:
				if f.Direction != "CLIENT_TO_SERVER" || method != in.Write.Method {
					return out, false
				}
				if _, err := parseID(m["id"]); err != nil {
					return out, false
				}
				if f.Ordinal == in.Write.CompletedOrdinal && (!bytes.Equal(f.Bytes, in.Write.RequestFrame) || !bytes.Equal(m["id"], in.Write.RequestID) || !bytes.Equal(m["params"], in.Write.RequestParams)) {
					return out, false
				}
			}
		} else {
			id, err := parseID(m["id"])
			if err != nil {
				return out, false
			}
			found := -1
			for k, j := range pending {
				if out.events[j].id.SameValue(id) {
					found = k
					break
				}
			}
			if found < 0 {
				return out, false
			}
			j := pending[found]
			wantDirection := "CLIENT_TO_SERVER"
			if out.events[j].method == "initialize" {
				wantDirection = "SERVER_TO_CLIENT"
			}
			if f.Direction != wantDirection {
				return out, false
			}
			pending = append(pending[:found], pending[found+1:]...)
			_, success := m["result"]
			_, failed := m["error"]
			if success == failed || failed && (len(m["error"]) == 0 || string(m["error"]) == "null") {
				return out, false
			}
			status := "SUCCESS"
			if failed {
				status = "ERROR"
			}
			out.events[j].response = i
			out.events[j].status = status
			out.observed = append(out.observed, i)
		}
	}
	if out.notification < 0 || len(out.events) == 0 || out.events[0].method != "initialize" {
		return out, false
	}
	foundWrite := false
	for i, f := range in.Frames {
		if f.Ordinal == in.Write.CompletedOrdinal {
			foundWrite = true
			if !bytes.Equal(f.Bytes, in.Write.RequestFrame) || out.messages[i]["method"] == nil {
				return out, false
			}
		}
	}
	return out, foundWrite
}

func b4bFullCorresponds(in B4bFullCandidateInput, envelope b4aEnvelope, parsed b4bFullParsed) bool {
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
		canonicalID, err := b4bHeldSuccessorTypedID(m["id"])
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
		responseID, err := b4bHeldSuccessorTypedID(rm["id"])
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

func b4bFullID(raw json.RawMessage, id ID) bool {
	got, err := parseID(raw)
	return err == nil && id.SameValue(got)
}

func b4bFullFold(in B4bFullCandidateInput, parsed b4bFullParsed) string {
	stage := 0
	static := false
	type registration struct {
		method  string
		options json.RawMessage
	}
	active := map[string]registration{}
	pending := map[int]map[string]registration{}
	for i, frame := range in.Frames {
		if frame.Ordinal >= in.Write.CompletedOrdinal {
			break
		}
		m := parsed.messages[i]
		method := b4aText(m, "method")
		switch method {
		case "initialize":
			if stage != 0 || frame.Direction != "CLIENT_TO_SERVER" {
				return "INVALID_CHRONOLOGY"
			}
			stage = 1
		case "initialized":
			if stage != 2 || frame.Direction != "CLIENT_TO_SERVER" {
				return "INVALID_CHRONOLOGY"
			}
			stage = 3
		case "client/registerCapability", "client/unregisterCapability":
			if stage != 3 || frame.Direction != "SERVER_TO_CLIENT" {
				return "INVALID_CHRONOLOGY"
			}
			var entries struct {
				Registrations []struct {
					ID, Method      string
					RegisterOptions json.RawMessage
				} `json:"registrations"`
				Unregisterations []struct{ ID, Method string } `json:"unregisterations"`
			}
			if b4aFrameFields(m["params"], &entries) != nil {
				return "MALFORMED"
			}
			items := map[string]registration{}
			if method == "client/registerCapability" {
				if len(entries.Registrations) == 0 {
					return "MALFORMED"
				}
				for _, x := range entries.Registrations {
					if x.ID == "" || x.Method == "" {
						return "MALFORMED"
					}
					key := x.ID + "\x00" + x.Method
					if _, exists := items[key]; exists {
						return "INVALID_CHRONOLOGY"
					}
					items[key] = registration{method: x.Method, options: x.RegisterOptions}
				}
			} else {
				if len(entries.Unregisterations) == 0 {
					return "MALFORMED"
				}
				for _, x := range entries.Unregisterations {
					if x.ID == "" || x.Method == "" {
						return "MALFORMED"
					}
					key := x.ID + "\x00" + x.Method
					if _, exists := items[key]; exists {
						return "INVALID_CHRONOLOGY"
					}
					items[key] = registration{method: x.Method}
				}
			}
			pending[i] = items
		case "":
			request := -1
			for j, ev := range parsed.events {
				if ev.response == i {
					request = j
					break
				}
			}
			if request < 0 {
				return "INVALID_CHRONOLOGY"
			}
			ev := parsed.events[request]
			if ev.method == "initialize" {
				if stage != 1 || ev.status != "SUCCESS" {
					return "INVALID_CHRONOLOGY"
				}
				var result struct {
					Capabilities map[string]json.RawMessage `json:"capabilities"`
				}
				if b4aFrameFields(m["result"], &result) != nil || result.Capabilities == nil {
					return "MALFORMED"
				}
				provider := result.Capabilities[strings.TrimPrefix(in.Write.Method, "textDocument/")+"Provider"]
				switch string(provider) {
				case "", "false", "null":
					static = false
				case "true":
					static = true
				default:
					var options map[string]json.RawMessage
					if b4aFrameFields(provider, &options) != nil {
						return "MALFORMED"
					}
					if raw, ok := options["workDoneProgress"]; ok && string(raw) != "true" && string(raw) != "false" {
						return "MALFORMED"
					}
					static = true
				}
				stage = 2
				continue
			}
			if stage != 3 {
				return "INVALID_CHRONOLOGY"
			}
			if ev.status == "ERROR" {
				delete(pending, ev.request)
				continue
			}
			items, ok := pending[ev.request]
			if !ok {
				return "INVALID_CHRONOLOGY"
			}
			for key, x := range items {
				if ev.method == "client/registerCapability" {
					if _, exists := active[key]; exists {
						return "INVALID_CHRONOLOGY"
					}
					active[key] = x
				} else {
					if _, exists := active[key]; !exists {
						return "INVALID_CHRONOLOGY"
					}
					delete(active, key)
				}
			}
			delete(pending, ev.request)
		default:
			if stage != 3 || frame.Direction != "CLIENT_TO_SERVER" {
				return "INVALID_CHRONOLOGY"
			}
		}
	}
	if stage != 3 {
		return "INVALID_CHRONOLOGY"
	}
	outcome := "UNSUPPORTED"
	for _, ev := range parsed.events {
		if ev.request >= len(in.Frames) || in.Frames[ev.request].Ordinal >= in.Write.CompletedOrdinal || ev.response >= 0 && in.Frames[ev.response].Ordinal < in.Write.CompletedOrdinal {
			continue
		}
		if ev.method != "client/registerCapability" {
			continue
		}
		for _, x := range pending[ev.request] {
			if x.method == in.Write.Method {
				outcome = "UNKNOWN"
			}
		}
	}
	// Validate all active same-method known fields before any selector matches.
	activeSameMethod := false
	for _, x := range active {
		if x.method != in.Write.Method {
			continue
		}
		activeSameMethod = true
		supported, uncertain, malformed := b4bFullSelector(x.options, in)
		if malformed {
			return "MALFORMED"
		}
		if supported {
			outcome = "SUPPORTED"
		} else if uncertain && outcome != "SUPPORTED" {
			outcome = "UNKNOWN"
		}
	}
	if static && activeSameMethod {
		return "INVALID_CHRONOLOGY"
	}
	if static {
		return "SUPPORTED"
	}
	return outcome
}

// Unmarshal into a typed value only after recursively rejecting duplicate keys.
func b4aFrameFields(b json.RawMessage, v any) error {
	if err := b4Strict(b); err != nil {
		return err
	}
	return json.Unmarshal(b, v)
}

func b4bFullSelector(raw json.RawMessage, in B4bFullCandidateInput) (supported, uncertain, malformed bool) {
	if len(raw) == 0 {
		return false, true, false
	} // missing options are unresolved, not malformed
	var options map[string]json.RawMessage
	if b4aFrameFields(raw, &options) != nil || options == nil {
		return false, false, true
	}
	if v, ok := options["workDoneProgress"]; ok && string(v) != "true" && string(v) != "false" {
		return false, false, true
	}
	selector, ok := options["documentSelector"]
	if !ok {
		return false, true, false
	}
	if string(selector) == "null" {
		selector = in.HeldClientSelector
		if len(selector) == 0 {
			return false, true, false
		}
	}
	var filters []json.RawMessage
	if b4aFrameFields(selector, &filters) != nil || len(filters) == 0 {
		return false, false, true
	}
	type parsedFilter struct {
		values          map[string]string
		notebook        map[string]string
		notebookPresent bool
		notebookString  bool
	}
	valid := make([]parsedFilter, 0, len(filters))
	// First validate every known field, including filters after a proven match.
	for _, rawFilter := range filters {
		var fields map[string]json.RawMessage
		if b4aFrameFields(rawFilter, &fields) != nil || fields == nil {
			return false, false, true
		}
		f := parsedFilter{values: map[string]string{}}
		for _, name := range []string{"scheme", "language", "pattern"} {
			if v, exists := fields[name]; exists {
				var s string
				if len(v) == 0 || v[0] != '"' || json.Unmarshal(v, &s) != nil {
					return false, false, true
				}
				if name == "scheme" && !b4bHeldSuccessorScheme(s) {
					return false, false, true
				}
				f.values[name] = s
			}
		}
		if v, exists := fields["notebook"]; exists {
			f.notebookPresent = true
			if len(v) == 0 {
				return false, false, true
			}
			if v[0] == '"' {
				var name string
				if json.Unmarshal(v, &name) != nil || name == "" {
					return false, false, true
				}
				f.notebookString = true
				f.notebook = map[string]string{"notebookType": name}
			} else if v[0] == '{' {
				var obj map[string]json.RawMessage
				if b4aFrameFields(v, &obj) != nil || len(obj) == 0 {
					return false, false, true
				}
				f.notebook = map[string]string{}
				for _, name := range []string{"notebookType", "scheme", "pattern"} {
					if nested, exists := obj[name]; exists {
						var s string
						if len(nested) == 0 || nested[0] != '"' || json.Unmarshal(nested, &s) != nil {
							return false, false, true
						}
						if name == "notebookType" && s == "" || name == "scheme" && !b4bHeldSuccessorScheme(s) {
							return false, false, true
						}
						f.notebook[name] = s
					}
				}
			} else {
				return false, false, true
			}
		}
		valid = append(valid, f)
	}
	for _, f := range valid {
		if want, ok := f.values["scheme"]; ok {
			got, known := b4bInitialScheme(in.Write.URI)
			if !known {
				uncertain = true
				continue
			}
			if !strings.EqualFold(want, got) {
				continue
			}
		}
		if want, ok := f.values["language"]; ok {
			if in.Write.Language == nil {
				uncertain = true
				continue
			}
			if want != *in.Write.Language && !(f.notebookPresent && want == "*") {
				continue
			}
		}
		if f.notebookPresent {
			if in.Write.NotebookURI == nil && in.Write.NotebookType == nil {
				continue
			} // ordinary source cannot match a notebook-only filter
			if in.Write.NotebookURI == nil || in.Write.NotebookType == nil {
				uncertain = true
				continue
			}
			if want, ok := f.notebook["notebookType"]; ok && want != "*" && want != *in.Write.NotebookType {
				continue
			}
			if want, ok := f.notebook["scheme"]; ok {
				got, known := b4bInitialScheme(*in.Write.NotebookURI)
				if !known {
					uncertain = true
					continue
				}
				if !strings.EqualFold(want, got) {
					continue
				}
			}
			if _, ok := f.notebook["pattern"]; ok {
				uncertain = true
				continue
			}
			if len(f.notebook) == 0 {
				uncertain = true
				continue
			}
		}
		if _, ok := f.values["pattern"]; ok {
			uncertain = true
			continue
		}
		if len(f.values) == 0 && !f.notebookPresent {
			uncertain = true
			continue
		}
		supported = true
	}
	return supported, uncertain, false
}
