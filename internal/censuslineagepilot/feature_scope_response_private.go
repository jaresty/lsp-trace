package censuslineagepilot

import (
	"bytes"
	"encoding/json"
	"io"

	"lsp-trace/internal/lsp"
	"lsp-trace/internal/strictjson"
)

// Private predecode contract, intentionally stricter than historical WireClient:
// no custom lsp.DocumentSymbol unmarshal is used. Unsupported symbol tags and
// deprecated fields refuse, including zero values, rather than silently vanish.
// All protocol DTO keys are exact; only item.data is opaque JSON.
func scopeResponseObject(raw json.RawMessage, allowed ...string) (map[string]json.RawMessage, error) {
	var m map[string]json.RawMessage
	if json.Unmarshal(raw, &m) != nil || m == nil {
		return nil, ErrInvalid
	}
	for key := range m {
		found := false
		for _, a := range allowed {
			if key == a {
				found = true
				break
			}
		}
		if !found {
			return nil, ErrInvalid
		}
	}
	return m, nil
}
func scopeResponseRequired(m map[string]json.RawMessage, key string, out any) error {
	raw := m[key]
	if len(raw) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return ErrInvalid
	}
	if json.Unmarshal(raw, out) != nil {
		return ErrInvalid
	}
	return nil
}
func scopeResponseRange(raw json.RawMessage) (lsp.Range, error) {
	var r lsp.Range
	m, err := scopeResponseObject(raw, "start", "end")
	if err != nil {
		return r, err
	}
	for _, p := range []struct {
		key string
		out *lsp.Position
	}{{"start", &r.Start}, {"end", &r.End}} {
		v, e := scopeResponseObject(m[p.key], "line", "character")
		if e != nil {
			return r, e
		}
		if scopeResponseRequired(v, "line", &p.out.Line) != nil || scopeResponseRequired(v, "character", &p.out.Character) != nil {
			return r, ErrInvalid
		}
	}
	if !scopePositionBefore(r.Start, r.End) {
		return r, ErrInvalid
	}
	return r, nil
}
func scopePositionBefore(a, b lsp.Position) bool {
	return a.Line < b.Line || (a.Line == b.Line && a.Character <= b.Character)
}
func scopeResponseRanges(m map[string]json.RawMessage) error {
	r, err := scopeResponseRange(m["range"])
	if err != nil {
		return err
	}
	s, err := scopeResponseRange(m["selectionRange"])
	if err != nil {
		return err
	}
	if !scopePositionBefore(r.Start, s.Start) || !scopePositionBefore(s.End, r.End) {
		return ErrInvalid
	}
	return nil
}
func scopeResponseItem(raw json.RawMessage) error {
	m, err := scopeResponseObject(raw, "name", "kind", "tags", "detail", "uri", "range", "selectionRange", "data")
	if err != nil {
		return err
	}
	var name, uri string
	var kind int
	if scopeResponseRequired(m, "name", &name) != nil || scopeResponseRequired(m, "uri", &uri) != nil || scopeResponseRequired(m, "kind", &kind) != nil {
		return ErrInvalid
	}
	if _, ok := m["tags"]; ok {
		var tags []int
		if scopeResponseRequired(m, "tags", &tags) != nil {
			return ErrInvalid
		}
	}
	if _, ok := m["detail"]; ok {
		var detail string
		if scopeResponseRequired(m, "detail", &detail) != nil {
			return ErrInvalid
		}
	}
	return scopeResponseRanges(m)
}
func scopeResponseSymbol(raw json.RawMessage, uri string, flat bool) (lsp.DocumentSymbol, bool, error) {
	var s lsp.DocumentSymbol
	keys := []string{"name", "kind", "detail", "range", "selectionRange", "children"}
	if flat {
		keys = []string{"name", "kind", "location", "containerName"}
	}
	m, err := scopeResponseObject(raw, keys...)
	if err != nil {
		return s, false, err
	}
	if scopeResponseRequired(m, "name", &s.Name) != nil || scopeResponseRequired(m, "kind", &s.Kind) != nil {
		return s, false, ErrInvalid
	}
	if flat {
		location, e := scopeResponseObject(m["location"], "uri", "range")
		if e != nil {
			return s, false, e
		}
		var observedURI string
		if scopeResponseRequired(location, "uri", &observedURI) != nil {
			return s, false, ErrInvalid
		}
		s.Range, e = scopeResponseRange(location["range"])
		if e != nil {
			return s, false, e
		}
		s.SelectionRange = s.Range
		s.Flat = true
		s.LocationURI = observedURI
		if _, ok := m["containerName"]; ok {
			if scopeResponseRequired(m, "containerName", &s.ContainerName) != nil {
				return s, false, ErrInvalid
			}
		}
		return s, observedURI != uri, nil
	}
	if err = scopeResponseRanges(m); err != nil {
		return s, false, err
	}
	s.Range, _ = scopeResponseRange(m["range"])
	s.SelectionRange, _ = scopeResponseRange(m["selectionRange"])
	if _, ok := m["detail"]; ok {
		if scopeResponseRequired(m, "detail", &s.Detail) != nil {
			return s, false, ErrInvalid
		}
	}
	if raw, ok := m["children"]; ok {
		var children []json.RawMessage
		if len(bytes.TrimSpace(raw)) == 0 || bytes.TrimSpace(raw)[0] != '[' || json.Unmarshal(raw, &children) != nil {
			return s, false, ErrInvalid
		}
		s.Children = make([]lsp.DocumentSymbol, 0, len(children))
		for _, child := range children {
			c, _, e := scopeResponseSymbol(child, uri, false)
			if e != nil {
				return s, false, e
			}
			s.Children = append(s.Children, c)
		}
	}
	return s, false, nil
}

// Performs the same required-coordinate and non-null-array checks as the
// acquisition wire validator, plus exact-key/range checks before typed decode.
// It returns symbol normalization only for the two legal unmixed symbol forms.
func validateScopeResponse(method string, raw json.RawMessage, uri string) ([]lsp.DocumentSymbol, int, error) {
	if len(raw) > scopeAdapterResponse || strictjson.RejectDuplicates(raw) != nil {
		return nil, 0, ErrInvalid
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	depth := 0
	for {
		tok, err := d.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, 0, err
		}
		if delim, ok := tok.(json.Delim); ok {
			switch delim {
			case '[', '{':
				depth++
				if depth > 128 {
					return nil, 0, ErrInvalid
				}
			case ']', '}':
				depth--
			}
		}
	}
	trimmed := bytes.TrimSpace(raw)
	if bytes.Equal(trimmed, []byte("null")) {
		return nil, 0, nil
	}
	if len(trimmed) == 0 || trimmed[0] != '[' {
		return nil, 0, ErrInvalid
	}
	var rows []json.RawMessage
	if json.Unmarshal(raw, &rows) != nil {
		return nil, 0, ErrInvalid
	}
	symbols := make([]lsp.DocumentSymbol, 0, len(rows))
	omitted := 0
	flat := false
	for i, row := range rows {
		switch method {
		case "textDocument/documentSymbol":
			var m map[string]json.RawMessage
			if json.Unmarshal(row, &m) != nil || m == nil {
				return nil, 0, ErrInvalid
			}
			_, isFlat := m["location"]
			if i == 0 {
				flat = isFlat
			} else if flat != isFlat {
				return nil, 0, ErrInvalid
			}
			symbol, external, err := scopeResponseSymbol(row, uri, flat)
			if err != nil {
				return nil, 0, err
			}
			if external {
				omitted++
			} else {
				symbols = append(symbols, symbol)
			}
		case "textDocument/prepareCallHierarchy":
			if err := scopeResponseItem(row); err != nil {
				return nil, 0, err
			}
		case "callHierarchy/incomingCalls", "callHierarchy/outgoingCalls":
			key := "from"
			if method == "callHierarchy/outgoingCalls" {
				key = "to"
			}
			m, err := scopeResponseObject(row, key, "fromRanges")
			if err != nil {
				return nil, 0, err
			}
			if err = scopeResponseItem(m[key]); err != nil {
				return nil, 0, err
			}
			ranges := bytes.TrimSpace(m["fromRanges"])
			var rs []json.RawMessage
			if len(ranges) == 0 || ranges[0] != '[' || json.Unmarshal(ranges, &rs) != nil {
				return nil, 0, ErrInvalid
			}
			for _, r := range rs {
				if _, err = scopeResponseRange(r); err != nil {
					return nil, 0, err
				}
			}
		default:
			return nil, 0, ErrInvalid
		}
	}
	return symbols, omitted, nil
}
