package adr0011generic

// Private, held-only selector replay. No request, occurrence, or runtime authority.
import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"
)

type capabilityOutcomeV3 string

const (
	capabilitySupportedV3         capabilityOutcomeV3 = "SUPPORTED"
	capabilityUnsupportedV3       capabilityOutcomeV3 = "UNSUPPORTED"
	capabilityUnknownV3           capabilityOutcomeV3 = "UNKNOWN"
	capabilityMalformedV3         capabilityOutcomeV3 = "MALFORMED"
	capabilityInvalidChronologyV3 capabilityOutcomeV3 = "INVALID_CHRONOLOGY"
)

// capabilityOriginalV3 retains an entire register/unregister JSON object, not a summary.
// Ordinals are strictly increasing within the held transaction; WRITE is an exclusive
// ordinal. Events with ordinal >= WRITE are not evaluated for that WRITE.
type capabilityOriginalV3 struct {
	Session     string
	Generation  uint64
	Transaction string
	Ordinal     uint64
	Raw         []byte
}
type capabilityQueryV3 struct {
	Session      string
	Generation   uint64
	Transaction  string
	WriteOrdinal uint64
	Method       string
	URI          string
	Language     *string
	Cell         bool
	NotebookURI  *string
	NotebookType *string
}

// The independently held client selector is bound to the same transaction;
// it is not inferred from a claimant's query or a registration event.
type capabilityClientSelectorOriginalV3 struct {
	Session     string
	Generation  uint64
	Transaction string
	Raw         []byte
}

func equalClientSelectorOriginalV3(a, b *capabilityClientSelectorOriginalV3) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return a.Session == b.Session && a.Generation == b.Generation && a.Transaction == b.Transaction && bytes.Equal(a.Raw, b.Raw)
}

type capabilityReplayInputV3 struct {
	HeldClientSelector     *capabilityClientSelectorOriginalV3
	ClaimantClientSelector *capabilityClientSelectorOriginalV3 // comparison only
	HeldInitialize         capabilityClientSelectorOriginalV3  // complete initialize result bytes, not a capabilities summary
	ClaimantInitialize     capabilityClientSelectorOriginalV3  // comparison only
	Held                   []capabilityOriginalV3
	Claimant               []capabilityOriginalV3 // comparison only; never a selector expectation
	Query                  capabilityQueryV3
}
type capabilityReplayResultV3 struct {
	Outcome        capabilityOutcomeV3
	OriginalsEqual bool
	Cause          string
	OwnError       string // nonempty blocks any exact replay receipt, independent of applicability
}

// Private decision surface; applicability alone is not an exact-replay guard.
func (r capabilityReplayResultV3) capabilityGuardPassed() bool {
	return r.Outcome == capabilitySupportedV3 && r.OriginalsEqual && r.OwnError == ""
}

// strictObjectV3 rejects duplicate decoded keys at every nesting level, trailing JSON,
// invalid UTF-8 (which encoding/json would otherwise replace), and non-JSON values.
func strictObjectV3(raw []byte) (map[string]json.RawMessage, error) {
	if !json.Valid(raw) || !utf8ValidV3(raw) {
		return nil, errors.New("invalid JSON")
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	v, e := strictValueV3(d)
	if e != nil {
		return nil, e
	}
	if _, e = d.Token(); e != io.EOF {
		return nil, errors.New("trailing JSON")
	}
	m, ok := v.(map[string]json.RawMessage)
	if !ok {
		return nil, errors.New("expected object")
	}
	return m, nil
}
func utf8ValidV3(b []byte) bool { return utf8.Valid(b) }
func strictValueV3(d *json.Decoder) (any, error) {
	start, e := d.Token()
	if e != nil {
		return nil, e
	}
	delim, ok := start.(json.Delim)
	if !ok {
		return start, nil
	}
	switch delim {
	case '{':
		m := map[string]json.RawMessage{}
		for d.More() {
			k, e := d.Token()
			if e != nil {
				return nil, e
			}
			key, ok := k.(string)
			if !ok {
				return nil, errors.New("key")
			}
			if _, exists := m[key]; exists {
				return nil, fmt.Errorf("duplicate decoded key %q", key)
			}
			var raw json.RawMessage
			if e = d.Decode(&raw); e != nil {
				return nil, e
			}
			if e = validateValueV3(raw); e != nil {
				return nil, e
			}
			m[key] = raw
		}
		_, e = d.Token()
		return m, e
	case '[':
		for d.More() {
			var raw json.RawMessage
			if e = d.Decode(&raw); e != nil {
				return nil, e
			}
			if e = validateValueV3(raw); e != nil {
				return nil, e
			}
		}
		_, e = d.Token()
		return true, e
	default:
		return nil, errors.New("invalid delimiter")
	}
}
func validateValueV3(b []byte) error {
	d := json.NewDecoder(bytes.NewReader(b))
	_, e := strictValueV3(d)
	return e
}
func equalOriginalsV3(a, b []capabilityOriginalV3) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Session != b[i].Session || a[i].Generation != b[i].Generation || a[i].Transaction != b[i].Transaction || a[i].Ordinal != b[i].Ordinal || !bytes.Equal(a[i].Raw, b[i].Raw) {
			return false
		}
	}
	return true
}
func replayCapabilityV3(in capabilityReplayInputV3) capabilityReplayResultV3 {
	q := in.Query
	out := capabilityReplayResultV3{OriginalsEqual: equalClientSelectorOriginalV3(&in.HeldInitialize, &in.ClaimantInitialize) && equalOriginalsV3(in.Held, in.Claimant) && equalClientSelectorOriginalV3(in.HeldClientSelector, in.ClaimantClientSelector)}
	if !out.OriginalsEqual {
		out.OwnError = "claimant originals differ from held bytes/order"
	}
	if len(in.HeldInitialize.Raw) > 1<<20 || len(in.ClaimantInitialize.Raw) > 1<<20 || (in.HeldClientSelector != nil && len(in.HeldClientSelector.Raw) > 1<<20) || (in.ClaimantClientSelector != nil && len(in.ClaimantClientSelector.Raw) > 1<<20) || len(in.Held) > 4096 || len(in.Claimant) > 4096 {
		out.OwnError = "replay bound exceeded"
		out.Outcome = capabilityMalformedV3
		out.Cause = out.OwnError
		return out
	}
	for _, ev := range in.Held {
		if len(ev.Raw) > 1<<20 {
			out.OwnError = "event bound exceeded"
			out.Outcome = capabilityMalformedV3
			out.Cause = out.OwnError
			return out
		}
	}
	fail := func(s capabilityOutcomeV3, c string) capabilityReplayResultV3 {
		out.Outcome = s
		out.Cause = c
		return out
	}
	if q.Session == "" || q.Generation == 0 || q.Transaction == "" || q.WriteOrdinal == 0 || (q.Method != "textDocument/references" && q.Method != "textDocument/definition") {
		return fail(capabilityMalformedV3, "invalid held query")
	}
	if in.HeldInitialize.Session != q.Session || in.HeldInitialize.Generation != q.Generation || in.HeldInitialize.Transaction != q.Transaction {
		out.OwnError = "cross-transaction initialize original"
		return fail(capabilityInvalidChronologyV3, out.OwnError)
	}
	cap, e := strictObjectV3(in.HeldInitialize.Raw)
	if e != nil {
		return fail(capabilityMalformedV3, "capability: "+e.Error())
	}
	r, ok := cap["capabilities"]
	if !ok {
		return fail(capabilityMalformedV3, "missing capabilities")
	}
	cap, e = strictObjectV3(r)
	if e != nil {
		return fail(capabilityMalformedV3, "capabilities: "+e.Error())
	}
	key := strings.TrimPrefix(q.Method, "textDocument/") + "Provider"
	static := cap[key]
	staticPresent := static != nil && string(static) != "null" && string(static) != "false"
	staticSupported := false
	if staticPresent {
		if string(static) == "true" {
			staticSupported = true
		} else {
			options, e := strictObjectV3(static)
			if e != nil {
				return fail(capabilityMalformedV3, "static options: "+e.Error())
			}
			if x, ok := options["workDoneProgress"]; ok && string(x) != "true" && string(x) != "false" {
				return fail(capabilityMalformedV3, "workDoneProgress type")
			}
			staticSupported = true
		}
	}
	active := map[string]json.RawMessage{}
	seenRegister := false
	var previous uint64
	for _, ev := range in.Held {
		// The selected chronology is the pre-WRITE prefix. A later event
		// cannot authorize or invalidate this completed request.
		if ev.Ordinal >= q.WriteOrdinal {
			continue
		}
		if ev.Session != q.Session || ev.Generation != q.Generation || ev.Transaction != q.Transaction {
			out.OwnError = "cross transaction original"
			return fail(capabilityInvalidChronologyV3, "cross transaction original")
		}
		if ev.Ordinal <= previous || ev.Ordinal == 0 {
			return fail(capabilityInvalidChronologyV3, "non-increasing ordinal")
		}
		previous = ev.Ordinal
		body, e := strictObjectV3(ev.Raw)
		if e != nil {
			return fail(capabilityMalformedV3, "event: "+e.Error())
		}
		registrations, register := body["registrations"]
		unregistrations, unregister := body["unregistrations"]
		if register == unregister {
			return fail(capabilityMalformedV3, "event shape")
		}
		raw := registrations
		if unregister {
			raw = unregistrations
		}
		var entries []json.RawMessage
		if e = json.Unmarshal(raw, &entries); e != nil || entries == nil {
			return fail(capabilityMalformedV3, "event entries")
		}
		for _, entry := range entries {
			fields, e := strictObjectV3(entry)
			if e != nil {
				return fail(capabilityMalformedV3, "entry: "+e.Error())
			}
			var id, method string
			if e = json.Unmarshal(fields["id"], &id); e != nil || id == "" {
				return fail(capabilityMalformedV3, "id")
			}
			if e = json.Unmarshal(fields["method"], &method); e != nil || method == "" {
				return fail(capabilityMalformedV3, "method")
			}
			pair := id + "\x00" + method
			if register {
				if _, exists := active[pair]; exists {
					return fail(capabilityInvalidChronologyV3, "duplicate active registration")
				}
				active[pair] = fields["registerOptions"]
				if method == q.Method {
					seenRegister = true
				}
			} else {
				if _, exists := active[pair]; !exists {
					return fail(capabilityInvalidChronologyV3, "unregister without exact active pair")
				}
				delete(active, pair)
			}
		}
	}
	if staticSupported && seenRegister {
		return fail(capabilityInvalidChronologyV3, "static and dynamic same method")
	}
	if staticSupported {
		return fail(capabilitySupportedV3, "")
	}
	found := false
	aggregate := capabilityUnsupportedV3
	for pair, options := range active {
		if !strings.HasSuffix(pair, "\x00"+q.Method) {
			continue
		}
		found = true
		v := capabilityUnknownV3
		if options != nil {
			obj, e := strictObjectV3(options)
			if e != nil {
				return fail(capabilityMalformedV3, "registerOptions: "+e.Error())
			}
			sel, ok := obj["documentSelector"]
			if ok {
				if string(sel) == "null" {
					if original := in.HeldClientSelector; original != nil {
						if original.Session != q.Session || original.Generation != q.Generation || original.Transaction != q.Transaction {
							out.OwnError = "cross-transaction client selector original"
							return fail(capabilityUnknownV3, out.OwnError)
						}
						sel = original.Raw
					} else {
						sel = nil
					}
				}
				if sel != nil && string(sel) != "null" {
					v = selectorV3(sel, q)
				}
			}
		}
		if v == capabilityMalformedV3 {
			return fail(v, "selector")
		}
		aggregate = orV3(aggregate, v)
	}
	if !found {
		return fail(capabilityUnsupportedV3, "")
	}
	return fail(aggregate, "")
}
func orV3(a, b capabilityOutcomeV3) capabilityOutcomeV3 {
	if a == capabilitySupportedV3 || b == capabilitySupportedV3 {
		return capabilitySupportedV3
	}
	if a == capabilityUnknownV3 || b == capabilityUnknownV3 {
		return capabilityUnknownV3
	}
	return capabilityUnsupportedV3
}
func andV3(a, b capabilityOutcomeV3) capabilityOutcomeV3 {
	if a == capabilityUnsupportedV3 || b == capabilityUnsupportedV3 {
		return capabilityUnsupportedV3
	}
	if a == capabilityUnknownV3 || b == capabilityUnknownV3 {
		return capabilityUnknownV3
	}
	return capabilitySupportedV3
}
func selectorV3(raw []byte, q capabilityQueryV3) capabilityOutcomeV3 {
	if len(raw) == 0 {
		return capabilityUnknownV3
	}
	if e := validateValueV3(raw); e != nil {
		return capabilityMalformedV3
	}
	var filters []json.RawMessage
	if e := json.Unmarshal(raw, &filters); e != nil || len(filters) == 0 {
		return capabilityMalformedV3
	}
	result := capabilityUnsupportedV3
	for _, f := range filters {
		obj, e := strictObjectV3(f)
		if e != nil {
			return capabilityMalformedV3
		}
		v := filterV3(obj, q)
		if v == capabilityMalformedV3 {
			return v
		}
		result = orV3(result, v)
	}
	return result
}
func filterV3(obj map[string]json.RawMessage, q capabilityQueryV3) capabilityOutcomeV3 {
	result := capabilitySupportedV3
	known := false
	compare := func(raw json.RawMessage, held *string, star bool) capabilityOutcomeV3 {
		var s string
		if json.Unmarshal(raw, &s) != nil || string(raw) == "null" {
			return capabilityMalformedV3
		}
		if held == nil {
			return capabilityUnknownV3
		}
		if star && s == "*" {
			return capabilitySupportedV3
		}
		if s == *held {
			return capabilitySupportedV3
		}
		return capabilityUnsupportedV3
	}
	for _, key := range []string{"language", "scheme", "pattern", "notebook"} {
		raw, ok := obj[key]
		if !ok {
			continue
		}
		known = true
		v := capabilitySupportedV3
		switch key {
		case "language":
			v = compare(raw, q.Language, q.Cell)
		case "scheme":
			var s string
			if json.Unmarshal(raw, &s) != nil || string(raw) == "null" {
				v = capabilityMalformedV3
			} else {
				v = schemeV3(q.URI, s)
			}
		case "pattern":
			var s string
			if json.Unmarshal(raw, &s) != nil || string(raw) == "null" {
				v = capabilityMalformedV3
			} else {
				v = capabilityUnknownV3
			}
		case "notebook":
			v = notebookV3(raw, q, compare)
		}
		if v == capabilityMalformedV3 {
			return v
		}
		result = andV3(result, v)
	}
	if !known {
		return capabilityUnknownV3
	}
	return result
}
func notebookV3(raw json.RawMessage, q capabilityQueryV3, compare func(json.RawMessage, *string, bool) capabilityOutcomeV3) capabilityOutcomeV3 {
	if len(raw) == 0 {
		return capabilityMalformedV3
	}
	if raw[0] == '"' {
		var s string
		if json.Unmarshal(raw, &s) != nil {
			return capabilityMalformedV3
		}
		if !q.Cell || q.NotebookURI == nil {
			return capabilityUnknownV3
		}
		if s == "*" {
			return capabilitySupportedV3
		}
		return compare(raw, q.NotebookType, false)
	}
	obj, e := strictObjectV3(raw)
	if e != nil {
		return capabilityMalformedV3
	}
	result := capabilitySupportedV3
	known := false
	for _, key := range []string{"notebookType", "scheme", "pattern"} {
		v, ok := obj[key]
		if !ok {
			continue
		}
		known = true
		part := capabilitySupportedV3
		switch key {
		case "notebookType":
			part = compare(v, q.NotebookType, false)
		case "scheme":
			var s string
			if json.Unmarshal(v, &s) != nil || string(v) == "null" {
				part = capabilityMalformedV3
			} else if q.NotebookURI == nil {
				part = capabilityUnknownV3
			} else {
				part = schemeV3(*q.NotebookURI, s)
			}
		case "pattern":
			var s string
			if json.Unmarshal(v, &s) != nil || string(v) == "null" {
				part = capabilityMalformedV3
			} else {
				part = capabilityUnknownV3
			}
		}
		if part == capabilityMalformedV3 {
			return part
		}
		result = andV3(result, part)
	}
	if !known {
		return capabilityUnknownV3
	}
	if !q.Cell || q.NotebookURI == nil {
		return capabilityUnknownV3
	}
	return result
}
func schemeV3(uri, expected string) capabilityOutcomeV3 {
	if !validSchemeTokenV3(expected) {
		return capabilityMalformedV3
	}
	n := 0
	for i := 0; i < len(uri); i++ {
		c := uri[i]
		valid := c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z'
		if i > 0 {
			valid = valid || c >= '0' && c <= '9' || c == '+' || c == '-' || c == '.'
		}
		if !valid {
			break
		}
		n++
	}
	if n == 0 || len(uri) <= n || uri[n] != ':' {
		return capabilityUnknownV3
	}
	if strings.EqualFold(uri[:n], expected) {
		return capabilitySupportedV3
	}
	return capabilityUnsupportedV3
}
func validSchemeTokenV3(s string) bool {
	if len(s) == 0 {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		letter := c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z'
		if !letter && (i == 0 || !(c >= '0' && c <= '9' || c == '+' || c == '-' || c == '.')) {
			return false
		}
	}
	return true
}
