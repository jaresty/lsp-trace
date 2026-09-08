package graphprovenance

import (
	"encoding/json"
	"sort"
	"strconv"
)

// BindingV2 separates URI attribution from coordinate usability. A readable
// receipt never upgrades an invalid coordinate anchor. V1 Binding stays frozen.
type BindingV2 struct {
	Pointer      string   `json:"pointer"`
	URI          string   `json:"uri"`
	Attribution  string   `json:"attribution"`
	AnchorStatus string   `json:"anchor_status"`
	ReceiptIDs   []string `json:"receipt_ids"`
}

func sourceURIsV2(bindings []BindingV2) []string {
	seen := map[string]bool{}
	for _, b := range bindings {
		if b.Attribution == "SOURCE" {
			seen[b.URI] = true
		}
	}
	out := make([]string, 0, len(seen))
	for uri := range seen {
		out = append(out, uri)
	}
	sort.Strings(out)
	return out
}

type coordinateV2 struct{ line, character uint32 }

func positionV2(v any) (coordinateV2, bool) {
	m, ok := v.(map[string]any)
	if !ok {
		return coordinateV2{}, false
	}
	number := func(key string) (uint32, bool) {
		n, ok := m[key].(json.Number)
		if !ok {
			return 0, false
		}
		x, err := strconv.ParseUint(n.String(), 10, 32)
		return uint32(x), err == nil
	}
	l, lok := number("line")
	c, cok := number("character")
	return coordinateV2{l, c}, lok && cok
}
func beforeOrEqualV2(a, b coordinateV2) bool {
	return a.line < b.line || a.line == b.line && a.character <= b.character
}
func rangeV2(v any) (coordinateV2, coordinateV2, bool) {
	m, ok := v.(map[string]any)
	if !ok {
		return coordinateV2{}, coordinateV2{}, false
	}
	a, aok := positionV2(m["start"])
	b, bok := positionV2(m["end"])
	// Empty ranges are valid under the established LSP coordinate policy.
	return a, b, aok && bok && beforeOrEqualV2(a, b)
}
func validRangeV2(v, enclosing any) bool {
	a, b, ok := rangeV2(v)
	if !ok {
		return false
	}
	if enclosing != nil {
		lo, hi, valid := rangeV2(enclosing)
		return valid && beforeOrEqualV2(lo, a) && beforeOrEqualV2(b, hi)
	}
	return true
}
