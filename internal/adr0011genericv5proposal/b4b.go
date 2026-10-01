package adr0011genericv5proposal

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"reflect"
	"strconv"
	"strings"
)

// B4bInput contains original frames held independently of the claimant envelope.
// No descriptor or claimant summary is a source of bytes or chronology.
type B4bInput struct {
	Write                                  B4aInput
	CapabilityEnvelope, CapabilityOriginal []byte
	Frames                                 []B4Frame
	HeldClientSelector                     []byte // separately held; nil means unavailable
	ClaimedOutcome                         string
}
type B4bResult struct {
	Outcome, Cause string
	ClaimMatches   bool
}

func b4bResult(in B4bInput, outcome, cause string) B4bResult {
	return B4bResult{outcome, cause, in.ClaimedOutcome == "" || in.ClaimedOutcome == outcome}
}

// CheckB4b is private, offline classification. It neither admits evidence nor issues selectors.
func CheckB4b(in B4bInput) B4bResult {
	fail := func(outcome, cause string) B4bResult { return b4bResult(in, outcome, cause) }
	if err := CheckB4a(in.Write); err != nil {
		return fail("INVALID_CHRONOLOGY", "B4a held WRITE: "+err.Error())
	}
	if err := A4Validate("CAPABILITY_EVENTS", in.CapabilityEnvelope); err != nil {
		return fail("INVALID_CHRONOLOGY", "capability claimant shape")
	}
	var envelope b4aEnvelope
	if json.Unmarshal(in.CapabilityEnvelope, &envelope) != nil || envelope.Role != "CAPABILITY_EVENTS" || envelope.Identity.Session != in.Write.Session || envelope.Identity.Generation != in.Write.Generation || envelope.Identity.Transaction != in.Write.Transaction || !b4aDescriptor(envelope.Original, in.CapabilityOriginal) {
		return fail("INVALID_CHRONOLOGY", "capability original identity")
	}
	if len(in.Frames) == 0 {
		return fail("INVALID_CHRONOLOGY", "no held frames")
	}
	if !b4bCorresponds(in, envelope) {
		return fail("INVALID_CHRONOLOGY", "held capability artifact or claimant stream correspondence")
	}
	stage := 0
	var initial ID
	static := false
	active := map[string]json.RawMessage{}
	type pendingChange struct {
		id     ID
		method string
		params json.RawMessage
	}
	pending := []pendingChange{}
	seenDynamic := false
	var prev uint64
	first := true
	// Account for every response occurrence, including those too late to change
	// the capability state at WRITE. The state fold below remains time-bounded.
	var outstanding []struct {
		id        ID
		direction string
	}
	for _, frame := range in.Frames {
		if (!first && frame.Ordinal <= prev) || frame.Direction == "" {
			return fail("INVALID_CHRONOLOGY", "held frame ordering")
		}
		first = false
		prev = frame.Ordinal
		object, err := b4aFrame(frame.Bytes)
		if err != nil || string(object["jsonrpc"]) != `"2.0"` {
			return fail("INVALID_CHRONOLOGY", "held frame syntax")
		}
		if method, isRequest := object["method"]; isRequest {
			if string(method) == `"initialize"` || string(method) == `"client/registerCapability"` || string(method) == `"client/unregisterCapability"` {
				id, e := parseID(object["id"])
				if e != nil {
					return fail("INVALID_CHRONOLOGY", "request ID accounting")
				}
				outstanding = append(outstanding, struct {
					id        ID
					direction string
				}{id, frame.Direction})
			}
		} else if len(object["id"]) != 0 {
			id, e := parseID(object["id"])
			if e != nil {
				return fail("INVALID_CHRONOLOGY", "response ID accounting")
			}
			match := -1
			for i, request := range outstanding {
				if request.id.SameValue(id) {
					match = i
					break
				}
			}
			if match < 0 || frame.Direction == outstanding[match].direction {
				return fail("INVALID_CHRONOLOGY", "unmatched or duplicate response occurrence")
			}
			outstanding = append(outstanding[:match], outstanding[match+1:]...)
		}
		if frame.Ordinal >= in.Write.CompletedOrdinal {
			continue // accounted and retained, but ineffective for this WRITE
		}
		if method, present := object["method"]; present {
			switch string(method) {
			case `"initialize"`:
				if stage != 0 || frame.Direction != "CLIENT_TO_SERVER" || len(object["params"]) == 0 {
					return fail("INVALID_CHRONOLOGY", "initialize request")
				}
				initial, err = parseID(object["id"])
				if err != nil {
					return fail("INVALID_CHRONOLOGY", "initialize ID")
				}
				stage = 1
			case `"initialized"`:
				if stage != 2 || frame.Direction != "CLIENT_TO_SERVER" || len(object["id"]) != 0 || string(object["params"]) != "{}" {
					return fail("INVALID_CHRONOLOGY", "initialized notification")
				}
				stage = 3
			case `"client/registerCapability"`, `"client/unregisterCapability"`:
				if stage != 3 || frame.Direction != "SERVER_TO_CLIENT" {
					return fail("INVALID_CHRONOLOGY", "early capability exchange")
				}
				id, e := parseID(object["id"])
				if e != nil {
					return fail("INVALID_CHRONOLOGY", "capability request ID")
				}
				params, e := b4aFields(object["params"])
				if e != nil {
					return fail("INVALID_CHRONOLOGY", "capability request params")
				}
				key := "registrations"
				if string(method) == `"client/unregisterCapability"` {
					key = "unregisterations"
				}
				items, ok := b4List(params[key])
				if !ok || len(items) == 0 {
					return fail("MALFORMED", "capability entries")
				}
				for _, p := range pending {
					if p.id.SameValue(id) {
						return fail("INVALID_CHRONOLOGY", "duplicate pending ID")
					}
				}
				pending = append(pending, pendingChange{id, string(method), object["params"]})
			default:
				if stage != 3 {
					return fail("INVALID_CHRONOLOGY", "early request")
				}
			}
		} else if stage == 1 {
			id, e := parseID(object["id"])
			if e != nil || frame.Direction != "SERVER_TO_CLIENT" || !id.SameValue(initial) || len(object["error"]) != 0 {
				return fail("INVALID_CHRONOLOGY", "initialize response")
			}
			result, e := b4aFields(object["result"])
			if e != nil {
				return fail("INVALID_CHRONOLOGY", "initialize result")
			}
			capabilities, e := b4aFields(result["capabilities"])
			if e != nil {
				return fail("INVALID_CHRONOLOGY", "initialize capabilities")
			}
			provider := capabilities[strings.TrimPrefix(in.Write.Method, "textDocument/")+"Provider"]
			switch {
			case len(provider) == 0, bytes.Equal(provider, []byte("false")), bytes.Equal(provider, []byte("null")):
				static = false
			case bytes.Equal(provider, []byte("true")):
				static = true
			case provider[0] == '{':
				options, e := b4aFields(provider)
				if e != nil {
					return fail("MALFORMED", "static provider options")
				}
				if raw, present := options["workDoneProgress"]; present && string(raw) != "true" && string(raw) != "false" {
					return fail("MALFORMED", "static workDoneProgress type")
				}
				static = true
			default:
				return fail("MALFORMED", "static provider type")
			}
			stage = 2
		} else {
			if stage != 3 || frame.Direction != "CLIENT_TO_SERVER" {
				return fail("INVALID_CHRONOLOGY", "unexpected capability response")
			}
			id, e := parseID(object["id"])
			if e != nil {
				return fail("INVALID_CHRONOLOGY", "response ID")
			}
			index := -1
			for j, p := range pending {
				if p.id.SameValue(id) {
					index = j
					break
				}
			}
			if index < 0 {
				return fail("INVALID_CHRONOLOGY", "unmatched capability response")
			}
			p := pending[index]
			pending = append(pending[:index], pending[index+1:]...)
			_, success := object["result"]
			_, failed := object["error"]
			if success == failed {
				return fail("INVALID_CHRONOLOGY", "response result/error")
			}
			if failed {
				continue
			}
			params, _ := b4aFields(p.params)
			key := "registrations"
			if p.method == `"client/unregisterCapability"` {
				key = "unregisterations"
			}
			items, _ := b4List(params[key])
			for _, item := range items {
				entry, e := b4aFields(item)
				if e != nil {
					return fail("MALFORMED", "registration entry")
				}
				rid, ok := b4String(entry["id"])
				method, valid := b4String(entry["method"])
				if !ok || !valid || rid == "" || method == "" {
					return fail("MALFORMED", "registration identity")
				}
				pair := rid + "\x00" + method
				if p.method == `"client/registerCapability"` {
					if _, exists := active[pair]; exists {
						return fail("INVALID_CHRONOLOGY", "duplicate registration")
					}
					active[pair] = entry["registerOptions"]
					if method == in.Write.Method {
						seenDynamic = true
					}
				} else {
					if _, exists := active[pair]; !exists {
						return fail("INVALID_CHRONOLOGY", "unregister absent pair")
					}
					delete(active, pair)
				}
			}
		}
	}
	if stage != 3 {
		return fail("INVALID_CHRONOLOGY", "initialization incomplete")
	}
	payload, e := b4aFields(envelope.Payload)
	if e != nil {
		return fail("INVALID_CHRONOLOGY", "capability payload")
	}
	init, e := b4aFields(payload["initialize"])
	if e != nil {
		return fail("INVALID_CHRONOLOGY", "initialize claim")
	}
	notification, e := b4aFields(payload["initialized_notification"])
	if e != nil {
		return fail("INVALID_CHRONOLOGY", "notification claim")
	}
	ordinal, err := b4aUint(payload, "target_write_frame_ordinal")
	if err != nil || ordinal != in.Write.CompletedOrdinal {
		return fail("INVALID_CHRONOLOGY", "held target WRITE ordinal")
	}
	for _, item := range []struct {
		claim map[string]json.RawMessage
		key   string
		frame B4Frame
	}{
		{init, "request", in.Frames[0]}, {init, "response", in.Frames[1]}, {notification, "", in.Frames[2]},
	} {
		prefix := item.key
		ordKey, frameKey, dirKey := "frame_ordinal", "frame", "direction"
		if prefix != "" {
			ordKey, frameKey, dirKey = prefix+"_frame_ordinal", prefix+"_frame", prefix+"_direction"
		}
		ord, e := b4aUint(item.claim, ordKey)
		if e != nil || ord != item.frame.Ordinal || b4aText(item.claim, dirKey) != item.frame.Direction || !b4aByteField(item.claim, frameKey, item.frame.Bytes) {
			return fail("INVALID_CHRONOLOGY", "claimant/held capability frame correspondence")
		}
	}
	outcome := "UNSUPPORTED"
	for _, p := range pending {
		if p.method != `"client/registerCapability"` {
			continue
		}
		params, _ := b4aFields(p.params)
		items, _ := b4List(params["registrations"])
		for _, item := range items {
			entry, err := b4aFields(item)
			if err != nil {
				return fail("MALFORMED", "registration entry")
			}
			method, ok := b4String(entry["method"])
			if !ok {
				return fail("MALFORMED", "registration method")
			}
			if method == in.Write.Method {
				outcome = "UNKNOWN"
			}
		}
	}
	// First validate every known field; only then evaluate applicability.
	for pass := 0; pass < 2; pass++ {
		for pair, opts := range active {
			if !strings.HasSuffix(pair, "\x00"+in.Write.Method) {
				continue
			}
			trimmed := bytes.TrimSpace(opts)
			if len(trimmed) != 0 && trimmed[0] != '{' {
				return fail("MALFORMED", "non-object registerOptions")
			}
			options, e := b4aFields(opts)
			if e != nil {
				if len(trimmed) != 0 {
					return fail("MALFORMED", "registerOptions object")
				}
				if pass == 0 {
					continue
				}
				if outcome != "SUPPORTED" {
					outcome = "UNKNOWN"
				}
				continue
			}
			selector := options["documentSelector"]
			if bytes.Equal(selector, []byte("null")) {
				selector = in.HeldClientSelector
			}
			filters, ok := b4List(selector)
			if !ok {
				if pass == 0 {
					continue
				}
				if outcome != "SUPPORTED" {
					outcome = "UNKNOWN"
				}
				continue
			}
			if len(filters) == 0 {
				return fail("MALFORMED", "empty documentSelector")
			}
			for _, raw := range filters {
				filter, e := b4aFields(raw)
				if e != nil {
					return fail("MALFORMED", "selector filter")
				}
				for _, field := range []string{"scheme", "pattern", "language"} {
					if value, present := filter[field]; present {
						if _, ok := b4String(value); !ok {
							return fail("MALFORMED", "selector "+field+" type")
						}
					}
				}
				notebook, hasNotebook := filter["notebook"]
				notebookType, stringNotebook := b4String(notebook)
				if hasNotebook && stringNotebook && notebookType == "" {
					return fail("MALFORMED", "selector notebook type")
				}
				var notebookObject map[string]json.RawMessage
				if hasNotebook && !stringNotebook {
					var err error
					notebookObject, err = b4aFields(notebook)
					if err != nil || len(notebookObject) == 0 {
						return fail("MALFORMED", "selector notebook type")
					}
					if raw, present := notebookObject["notebookType"]; present && string(raw) == `""` {
						return fail("MALFORMED", "selector notebookType empty")
					}
					for _, key := range []string{"notebookType", "scheme", "pattern"} {
						if value, present := notebookObject[key]; present {
							if _, ok := b4String(value); !ok {
								return fail("MALFORMED", "selector notebook "+key+" type")
							}
						}
					}
					if value, present := notebookObject["scheme"]; present {
						s, _ := b4String(value)
						if !b4bValidScheme(s) {
							return fail("MALFORMED", "selector notebook scheme grammar")
						}
					}
				}
				scheme, hasScheme := b4String(filter["scheme"])
				if hasScheme && !b4bValidScheme(scheme) {
					return fail("MALFORMED", "selector scheme grammar")
				}
				if pass == 0 {
					continue
				}
				language, hasLanguage := b4String(filter["language"])
				if hasLanguage && in.Write.Language != nil && language != *in.Write.Language && !(hasNotebook && language == "*") {
					continue
				}
				uriScheme, hasURIScheme := b4bInitialScheme(in.Write.URI)
				if hasScheme && hasURIScheme && !strings.EqualFold(scheme, uriScheme) {
					continue
				}
				if hasNotebook {
					if in.Write.NotebookURI == nil && in.Write.NotebookType == nil {
						continue // ordinary document cannot satisfy notebook-only conditions
					}
					if !stringNotebook {
						notebookType, _ = b4String(notebookObject["notebookType"])
						if len(notebookObject["notebookType"]) == 0 && len(notebookObject["scheme"]) == 0 && len(notebookObject["pattern"]) == 0 {
							if outcome != "SUPPORTED" {
								outcome = "UNKNOWN"
							}
							continue
						}
					}
					if in.Write.NotebookType != nil && notebookType != "" && notebookType != "*" && notebookType != *in.Write.NotebookType {
						continue
					}
					if raw, present := notebookObject["scheme"]; present && in.Write.NotebookURI != nil {
						wanted, _ := b4String(raw)
						parentScheme, known := b4bInitialScheme(*in.Write.NotebookURI)
						if known && !strings.EqualFold(wanted, parentScheme) {
							continue
						}
					}
					if in.Write.NotebookType == nil || in.Write.NotebookURI == nil {
						if outcome != "SUPPORTED" {
							outcome = "UNKNOWN"
						}
						continue
					}
					if _, present := notebookObject["scheme"]; present {
						if _, known := b4bInitialScheme(*in.Write.NotebookURI); !known {
							if outcome != "SUPPORTED" {
								outcome = "UNKNOWN"
							}
							continue
						}
					}
				}
				// No held path/glob contract proves a pattern match.
				if (hasScheme && !hasURIScheme) || (!hasScheme && !hasLanguage && !hasNotebook) || (hasLanguage && in.Write.Language == nil) || len(filter["pattern"]) != 0 || len(notebookObject["pattern"]) != 0 {
					if outcome != "SUPPORTED" {
						outcome = "UNKNOWN"
					}
					continue
				}
				outcome = "SUPPORTED"
			}
		}
	}
	// Validate active same-method options before resolving the static/dynamic
	// conflict. A known-invalid pre-WRITE option wins; valid coexistence does not.
	if static && seenDynamic {
		return fail("INVALID_CHRONOLOGY", "static/dynamic coexistence")
	}
	if static {
		return fail("SUPPORTED", "")
	}
	return fail(outcome, "")
}

// b4bValidScheme accepts the ASCII RFC3986 scheme grammar.
func b4bValidScheme(s string) bool {
	if len(s) == 0 || !b4bASCIIAlpha(s[0]) {
		return false
	}
	for i := 1; i < len(s); i++ {
		c := s[i]
		if !b4bASCIIAlpha(c) && !(c >= '0' && c <= '9') && c != '+' && c != '-' && c != '.' {
			return false
		}
	}
	return true
}

func b4bASCIIAlpha(c byte) bool {
	return c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z'
}

func b4bInitialScheme(uri string) (string, bool) {
	colon := strings.IndexByte(uri, ':')
	if colon < 0 || !b4bValidScheme(uri[:colon]) {
		return "", false
	}
	return uri[:colon], true
}

// b4bCorresponds constructs the /2 preimage solely from independent held bytes.
// Claimants can describe occurrences, but cannot supply expected frames or fields.
func b4bCorresponds(in B4bInput, envelope b4aEnvelope) bool {
	payload, err := b4aFields(envelope.Payload)
	if err != nil {
		return false
	}
	var observed []json.RawMessage
	if json.Unmarshal(payload["observed_frames"], &observed) != nil || len(observed) != len(in.Frames) {
		return false
	}
	fields := [][]byte{[]byte("ADR0011-GENERIC-CAPABILITY-EXCHANGE-ARTIFACT/2"), []byte(in.Write.Transaction), []byte(strconv.FormatUint(in.Write.CompletedOrdinal, 10))}
	var requests []int
	var notification int = -1
	for i, f := range in.Frames {
		claim, e := b4aFields(observed[i])
		if e != nil || !b4bOccurrence(claim, "", f) {
			return false
		}
		obj, e := b4aFrame(f.Bytes)
		if e != nil || string(obj["jsonrpc"]) != `"2.0"` {
			return false
		}
		switch string(obj["method"]) {
		case `"initialize"`, `"client/registerCapability"`, `"client/unregisterCapability"`:
			requests = append(requests, i)
		case `"initialized"`:
			if notification >= 0 {
				return false
			}
			notification = i
		}
	}
	if notification < 0 || len(requests) == 0 || len(requests) > 8193 {
		return false
	}
	fields = append(fields, []byte(strconv.Itoa(len(requests))))
	var events []json.RawMessage
	if json.Unmarshal(payload["events"], &events) != nil || len(events) != len(requests)-1 {
		return false
	}
	for n, i := range requests {
		f := in.Frames[i]
		obj, e := b4aFrame(f.Bytes)
		if e != nil {
			return false
		}
		id, e := parseID(obj["id"])
		if e != nil {
			return false
		}
		method := b4aText(obj, "method")
		if (method == "initialize" && f.Direction != "CLIENT_TO_SERVER") || (method != "initialize" && f.Direction != "SERVER_TO_CLIENT") {
			return false
		}
		var raw json.RawMessage
		if n == 0 {
			raw = payload["initialize"]
			if method != "initialize" {
				return false
			}
		} else {
			raw = events[n-1]
			if method == "initialize" {
				return false
			}
		}
		claim, e := b4aFields(raw)
		if e != nil || !b4bOccurrence(claim, "request_", f) || b4aText(claim, "request_method") != method || !b4bID(claim["request_id"], id) || !b4bJSON(claim["request_params"], obj["params"]) {
			return false
		}
		ordinal, e := b4aUint(claim, "target_write_frame_ordinal")
		if e != nil || ordinal != in.Write.CompletedOrdinal {
			return false
		}
		fields = append(fields, []byte(f.Direction), f.Bytes, []byte(b4bTypedID(id)), []byte(method), []byte(strconv.FormatUint(f.Ordinal, 10)))
		response := -1
		for j := i + 1; j < len(in.Frames); j++ {
			other, e := b4aFrame(in.Frames[j].Bytes)
			if e != nil {
				return false
			}
			if len(other["method"]) != 0 {
				continue
			}
			rid, e := parseID(other["id"])
			if e == nil && id.SameValue(rid) {
				response = j
				break
			}
		}
		if response < 0 {
			if string(claim["response_direction"]) != "null" || string(claim["response_frame"]) != "null" || string(claim["response_id"]) != "null" || string(claim["response_frame_ordinal"]) != "null" || b4aText(claim, "response_status") != "PENDING" || string(claim["response_result"]) != "null" || string(claim["response_error"]) != "null" {
				return false
			}
			fields = append(fields, []byte("ABSENT"), nil, []byte("ABSENT"), []byte("-1"), []byte("PENDING"))
			continue
		}
		r := in.Frames[response]
		if (method == "initialize" && r.Direction != "SERVER_TO_CLIENT") || (method != "initialize" && r.Direction != "CLIENT_TO_SERVER") {
			return false
		}
		ro, e := b4aFrame(r.Bytes)
		if e != nil || !b4bOccurrence(claim, "response_", r) || !b4bID(claim["response_id"], id) {
			return false
		}
		status := "SUCCESS"
		result, success := ro["result"]
		failure, failed := ro["error"]
		if success == failed {
			return false
		}
		if failed {
			status = "ERROR"
			result = []byte("null")
		} else {
			failure = []byte("null")
		}
		if b4aText(claim, "response_status") != status || !b4bJSON(claim["response_result"], result) || !b4bJSON(claim["response_error"], failure) {
			return false
		}
		fields = append(fields, []byte(r.Direction), r.Bytes, []byte(b4bTypedID(id)), []byte(strconv.FormatUint(r.Ordinal, 10)), []byte(status))
	}
	note := in.Frames[notification]
	if note.Direction != "CLIENT_TO_SERVER" {
		return false
	}
	notify, e := b4aFields(payload["initialized_notification"])
	if e != nil || !b4bOccurrence(notify, "", note) {
		return false
	}
	fields = append(fields, []byte(note.Direction), []byte(strconv.FormatUint(note.Ordinal, 10)), note.Bytes, []byte(strconv.Itoa(len(in.Frames))))
	for _, f := range in.Frames {
		fields = append(fields, []byte(f.Direction), []byte(strconv.FormatUint(f.Ordinal, 10)), f.Bytes)
	}
	var artifact []byte
	for _, field := range fields {
		var length [8]byte
		binary.BigEndian.PutUint64(length[:], uint64(len(field)))
		artifact = append(artifact, length[:]...)
		artifact = append(artifact, field...)
	}
	return bytes.Equal(in.CapabilityOriginal, artifact)
}

func b4bTypedID(id ID) string {
	if id.Kind == "STRING" {
		return "string:" + id.String
	}
	return "number:" + string(id.RawToken)
}

func b4bID(raw json.RawMessage, expected ID) bool {
	id, err := parseID(raw)
	return err == nil && id.SameValue(expected)
}

func b4bJSON(a, b json.RawMessage) bool {
	if b4Strict(a) != nil || b4Strict(b) != nil {
		return false
	}
	var x, y any
	return json.Unmarshal(a, &x) == nil && json.Unmarshal(b, &y) == nil && reflect.DeepEqual(x, y)
}

func b4bOccurrence(claim map[string]json.RawMessage, prefix string, frame B4Frame) bool {
	ordinal, err := b4aUint(claim, prefix+"frame_ordinal")
	return err == nil && ordinal == frame.Ordinal && b4aText(claim, prefix+"direction") == frame.Direction && b4aByteField(claim, prefix+"frame", frame.Bytes)
}
