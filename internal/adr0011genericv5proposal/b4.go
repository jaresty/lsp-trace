package adr0011genericv5proposal

// Private, offline B4 candidate. Frames and context must be separately held by
// the verifier; this function cannot establish their provenance or issue selectors.
import (
	"bytes"
	"encoding/json"
	"lsp-trace/internal/strictjson"
	"strings"
)

type B4Frame struct {
	Ordinal   uint64
	Direction string
	Bytes     []byte
}

// B4RoleBinding is separately held context, never a claimant envelope.
type B4RoleBinding struct {
	Session                     string
	Generation                  uint64
	Transaction, Workspace, URI string
	Source                      []byte
	Language                    *string
	NotebookCell                bool
	NotebookURI, NotebookType   *string
}
type B4Input struct {
	Query, SourceRole, Applicability B4RoleBinding

	Session                                        string
	Generation                                     uint64
	Transaction, Workspace, Method, URI, SourceURI string
	Source, QuerySource                            []byte
	Language                                       *string
	NotebookCell                                   bool
	NotebookURI, NotebookType                      *string
	ClientSelector                                 []byte
	WriteOrdinal                                   uint64
	WriteFrame                                     []byte // separately held completed target WRITE
	WriteCompleted                                 bool   // independently held transport completion, not claimant summary
	Frames                                         []B4Frame
	// Comparison only. Never enters state derivation.
	ClaimedOutcome string
}
type B4Result struct {
	Outcome, Cause string
	ClaimMatches   bool
}

func b4Result(outcome, cause string, in B4Input) B4Result {
	return B4Result{outcome, cause, in.ClaimedOutcome == "" || in.ClaimedOutcome == outcome}
}
func b4String(raw json.RawMessage) (string, bool) {
	var s string
	err := json.Unmarshal(raw, &s)
	return s, err == nil && len(raw) > 0 && raw[0] == '"'
}
func b4Object(raw json.RawMessage) (map[string]json.RawMessage, bool) {
	if len(raw) == 0 {
		return nil, false
	}
	if err := b4Strict(raw); err != nil {
		return nil, false
	}
	var m map[string]json.RawMessage
	err := json.Unmarshal(raw, &m)
	return m, err == nil && m != nil
}
func b4Strict(raw []byte) error { return strictjson.RejectDuplicates(raw) }
func b4List(raw json.RawMessage) ([]json.RawMessage, bool) {
	var a []json.RawMessage
	if b4Strict(raw) != nil || json.Unmarshal(raw, &a) != nil || a == nil {
		return nil, false
	}
	return a, true
}
func b4SameID(a, b ID) bool { return a.SameValue(b) }
func b4OptionalEqual(a, b *string) bool {
	return a == nil && b == nil || a != nil && b != nil && *a == *b
}

type b4Pending struct {
	id     ID
	method string
	params json.RawMessage
}
type b4Registration struct {
	method   string
	selector json.RawMessage
	present  bool
}

func ReplayB4(in B4Input) B4Result {
	fail := func(o, c string) B4Result { return b4Result(o, c, in) }
	if in.Session == "" || in.Generation == 0 || in.Transaction == "" || in.Workspace == "" || in.WriteOrdinal == 0 || (in.Method != "textDocument/references" && in.Method != "textDocument/definition") || in.URI == "" || in.SourceURI != in.URI || !bytes.Equal(in.Source, in.QuerySource) || in.Source == nil {
		return fail("INVALID_CHRONOLOGY", "held query/source/transaction binding")
	}
	for _, role := range []B4RoleBinding{in.Query, in.SourceRole, in.Applicability} {
		if role.Session != in.Session || role.Generation != in.Generation || role.Transaction != in.Transaction || role.Workspace != in.Workspace || role.URI != in.URI || !bytes.Equal(role.Source, in.Source) || role.NotebookCell != in.NotebookCell || !b4OptionalEqual(role.Language, in.Language) || !b4OptionalEqual(role.NotebookURI, in.NotebookURI) || !b4OptionalEqual(role.NotebookType, in.NotebookType) {
			return fail("INVALID_CHRONOLOGY", "held QUERY/SOURCE/QUERY_APPLICABILITY role binding")
		}
	}
	if in.NotebookCell && (in.NotebookURI == nil || in.NotebookType == nil) {
		return fail("UNKNOWN", "missing held notebook parent")
	}
	if !in.NotebookCell && (in.NotebookURI != nil || in.NotebookType != nil) {
		return fail("INVALID_CHRONOLOGY", "ordinary document has notebook parent")
	}
	if !in.WriteCompleted {
		return fail("PARTIAL", "target write completion not held")
	}
	write, writeErr := framedObject(in.WriteFrame)
	if writeErr != nil || string(write["jsonrpc"]) != `"2.0"` || string(write["method"]) != `"`+in.Method+`"` {
		return fail("PARTIAL", "completed target WRITE bytes not independently verified")
	}
	if _, err := parseID(write["id"]); err != nil {
		return fail("INVALID_CHRONOLOGY", "target WRITE ID")
	}
	if len(in.Frames) > 16387 {
		return fail("PARTIAL", "frame bound")
	}
	pending := []b4Pending{}
	active := map[string]b4Registration{}
	static := false
	seenDynamic := false
	initialized := false
	stage := 0
	var initializeID ID
	var prev uint64
	for _, f := range in.Frames {
		if f.Ordinal >= in.WriteOrdinal {
			continue
		}
		if f.Ordinal == 0 || f.Ordinal <= prev || len(f.Bytes) > 2097280 {
			return fail("INVALID_CHRONOLOGY", "frame ordering or bound")
		}
		prev = f.Ordinal
		obj, err := framedObject(f.Bytes)
		if err != nil {
			return fail("INVALID_CHRONOLOGY", "invalid complete frame: "+err.Error())
		}
		if string(obj["jsonrpc"]) != `"2.0"` {
			return fail("INVALID_CHRONOLOGY", "JSON-RPC version")
		}
		if raw, ok := obj["method"]; ok {
			method, ok := b4String(raw)
			if !ok {
				return fail("INVALID_CHRONOLOGY", "method type")
			}
			switch method {
			case "initialize":
				if stage != 0 || f.Direction != "CLIENT_TO_SERVER" || len(obj["params"]) == 0 {
					return fail("INVALID_CHRONOLOGY", "initialize request order")
				}
				var e error
				initializeID, e = parseID(obj["id"])
				if e != nil {
					return fail("INVALID_CHRONOLOGY", "initialize ID")
				}
				stage = 1
			case "initialized":
				if stage != 2 || f.Direction != "CLIENT_TO_SERVER" || len(obj["id"]) != 0 || string(obj["params"]) != "{}" {
					return fail("INVALID_CHRONOLOGY", "initialized notification order")
				}
				stage = 3
				initialized = true
			case "client/registerCapability", "client/unregisterCapability":
				if !initialized || f.Direction != "SERVER_TO_CLIENT" {
					return fail("INVALID_CHRONOLOGY", "premature capability request")
				}
				id, e := parseID(obj["id"])
				if e != nil {
					return fail("INVALID_CHRONOLOGY", "capability ID")
				}
				if _, ok := b4Object(obj["params"]); !ok {
					return fail("MALFORMED", "capability params")
				}
				for _, p := range pending {
					if b4SameID(p.id, id) {
						return fail("INVALID_CHRONOLOGY", "duplicate pending ID")
					}
				}
				pending = append(pending, b4Pending{id, method, obj["params"]})
			default: // other traffic cannot authorize capability state
			}
			continue
		}
		if len(obj["id"]) == 0 || len(obj["params"]) != 0 {
			return fail("INVALID_CHRONOLOGY", "unmatched response shape")
		}
		id, e := parseID(obj["id"])
		if e != nil {
			return fail("INVALID_CHRONOLOGY", "response ID")
		}
		_, hasResult := obj["result"]
		errorRaw, hasError := obj["error"]
		if hasResult == hasError || hasError && string(errorRaw) == "null" {
			return fail("INVALID_CHRONOLOGY", "response result/error")
		}
		if stage == 1 {
			if f.Direction != "SERVER_TO_CLIENT" || !id.SameValue(initializeID) || hasError {
				return fail("INVALID_CHRONOLOGY", "initialize reply")
			}
			result, ok := b4Object(obj["result"])
			if !ok {
				return fail("MALFORMED", "initialize result")
			}
			cap, ok := b4Object(result["capabilities"])
			if !ok {
				return fail("MALFORMED", "initialize capabilities")
			}
			key := strings.TrimPrefix(in.Method, "textDocument/") + "Provider"
			raw := cap[key]
			if len(raw) > 0 && string(raw) != "false" && string(raw) != "null" {
				if string(raw) == "true" {
					static = true
				} else {
					options, ok := b4Object(raw)
					if !ok {
						return fail("MALFORMED", "static provider")
					}
					if v, exists := options["workDoneProgress"]; exists && string(v) != "true" && string(v) != "false" {
						return fail("MALFORMED", "workDoneProgress")
					}
					static = true
				}
			}
			stage = 2
			continue
		}
		if stage != 3 || f.Direction != "CLIENT_TO_SERVER" {
			return fail("INVALID_CHRONOLOGY", "premature or reversed reply")
		}
		index := -1
		for j, p := range pending {
			if id.SameValue(p.id) {
				index = j
				break
			}
		}
		if index < 0 {
			return fail("INVALID_CHRONOLOGY", "unmatched or duplicate reply")
		}
		p := pending[index]
		pending = append(pending[:index], pending[index+1:]...)
		if hasError {
			if _, ok := b4Object(errorRaw); !ok {
				return fail("MALFORMED", "error object")
			}
			continue
		}
		if string(obj["result"]) != "null" {
			if _, ok := b4Object(obj["result"]); !ok {
				return fail("MALFORMED", "response result")
			}
		}
		params, _ := b4Object(p.params)
		key := "registrations"
		if p.method == "client/unregisterCapability" {
			key = "unregisterations"
		}
		items, ok := b4List(params[key])
		if !ok || len(items) == 0 {
			return fail("MALFORMED", "registration entries")
		}
		for _, raw := range items {
			entry, ok := b4Object(raw)
			if !ok {
				return fail("MALFORMED", "registration entry")
			}
			rid, okID := b4String(entry["id"])
			method, okMethod := b4String(entry["method"])
			if !okID || !okMethod || rid == "" || method == "" {
				return fail("MALFORMED", "registration identity")
			}
			pair := rid + "\x00" + method
			if p.method == "client/registerCapability" {
				if method == in.Method {
					seenDynamic = true
				}
				if _, exists := active[pair]; exists {
					return fail("INVALID_CHRONOLOGY", "duplicate active registration")
				}
				active[pair] = b4Registration{method, entry["registerOptions"], len(entry["registerOptions"]) > 0}
			} else {
				if _, exists := active[pair]; !exists {
					return fail("INVALID_CHRONOLOGY", "unregister without active pair")
				}
				delete(active, pair)
			}
		}
	}
	if stage != 3 {
		return fail("INVALID_CHRONOLOGY", "initialization incomplete")
	}
	outcome := "UNSUPPORTED"
	for _, reg := range active {
		if reg.method != in.Method {
			continue
		}
		v := b4Selector(reg, in)
		if v == "MALFORMED" {
			return fail(v, "selector")
		}
		if v == "SUPPORTED" {
			outcome = v
		} else if v == "UNKNOWN" && outcome != "SUPPORTED" {
			outcome = v
		}
	}
	if static && seenDynamic {
		return fail("INVALID_CHRONOLOGY", "static and dynamic same method")
	}
	if static {
		return fail("SUPPORTED", "")
	}
	if outcome == "UNSUPPORTED" {
		for _, p := range pending {
			if p.method == "client/registerCapability" {
				outcome = "UNKNOWN"
				break
			}
		}
	}
	return fail(outcome, "")
}
func b4Selector(reg b4Registration, in B4Input) string {
	if !reg.present {
		return "UNKNOWN"
	}
	o, ok := b4Object(reg.selector)
	if !ok {
		return "MALFORMED"
	}
	raw, ok := o["documentSelector"]
	if !ok {
		return "UNKNOWN"
	}
	if string(raw) == "null" {
		raw = in.ClientSelector
		if len(raw) == 0 {
			return "UNKNOWN"
		}
	}
	filters, ok := b4List(raw)
	if !ok || len(filters) == 0 {
		return "MALFORMED"
	}
	result := "UNSUPPORTED"
	for _, f := range filters {
		obj, ok := b4Object(f)
		if !ok {
			return "MALFORMED"
		}
		v := b4Filter(obj, in)
		if v == "MALFORMED" {
			return v
		}
		if v == "SUPPORTED" {
			result = v
		} else if v == "UNKNOWN" && result != "SUPPORTED" {
			result = v
		}
	}
	return result
}
func b4And(a, b string) string {
	if a == "UNSUPPORTED" || b == "UNSUPPORTED" {
		return "UNSUPPORTED"
	}
	if a == "UNKNOWN" || b == "UNKNOWN" {
		return "UNKNOWN"
	}
	return "SUPPORTED"
}
func b4Scheme(uri, want string) string {
	if want == "" || !((want[0] >= 'a' && want[0] <= 'z') || (want[0] >= 'A' && want[0] <= 'Z')) {
		return "MALFORMED"
	}
	for _, c := range want[1:] {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '+' || c == '-' || c == '.') {
			return "MALFORMED"
		}
	}
	i := strings.IndexByte(uri, ':')
	if i < 1 {
		return "UNKNOWN"
	}
	if strings.EqualFold(uri[:i], want) {
		return "SUPPORTED"
	}
	return "UNSUPPORTED"
}
func b4Match(raw json.RawMessage, held *string, star bool) string {
	s, ok := b4String(raw)
	if !ok {
		return "MALFORMED"
	}
	if held == nil {
		return "UNKNOWN"
	}
	if star && s == "*" || s == *held {
		return "SUPPORTED"
	}
	return "UNSUPPORTED"
}
func b4Filter(o map[string]json.RawMessage, in B4Input) string {
	result := "SUPPORTED"
	known := false
	for _, key := range []string{"language", "scheme", "pattern", "notebook"} {
		raw, ok := o[key]
		if !ok {
			continue
		}
		known = true
		v := "UNKNOWN"
		switch key {
		case "language":
			v = b4Match(raw, in.Language, in.NotebookCell)
		case "scheme":
			s, ok := b4String(raw)
			if !ok {
				return "MALFORMED"
			}
			v = b4Scheme(in.URI, s)
		case "pattern":
			if _, ok := b4String(raw); !ok {
				return "MALFORMED"
			}
		case "notebook":
			v = b4Notebook(raw, in)
		}
		if v == "MALFORMED" {
			return v
		}
		result = b4And(result, v)
	}
	if !known {
		return "UNKNOWN"
	}
	return result
}
func b4Notebook(raw json.RawMessage, in B4Input) string {
	if len(raw) > 0 && raw[0] == '"' {
		if !in.NotebookCell || in.NotebookURI == nil {
			return "UNKNOWN"
		}
		return b4Match(raw, in.NotebookType, true)
	}
	o, ok := b4Object(raw)
	if !ok {
		return "MALFORMED"
	}
	result := "SUPPORTED"
	known := false
	for _, key := range []string{"notebookType", "scheme", "pattern"} {
		v, ok := o[key]
		if !ok {
			continue
		}
		known = true
		part := "UNKNOWN"
		switch key {
		case "notebookType":
			part = b4Match(v, in.NotebookType, false)
		case "scheme":
			s, valid := b4String(v)
			if !valid {
				return "MALFORMED"
			}
			if in.NotebookURI != nil {
				part = b4Scheme(*in.NotebookURI, s)
			}
		case "pattern":
			if _, valid := b4String(v); !valid {
				return "MALFORMED"
			}
		}
		if part == "MALFORMED" {
			return part
		}
		result = b4And(result, part)
	}
	if !known || !in.NotebookCell || in.NotebookURI == nil {
		return "UNKNOWN"
	}
	return result
}
