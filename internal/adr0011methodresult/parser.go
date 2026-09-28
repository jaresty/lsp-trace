// Package adr0011methodresult parses bounded, successful definition/reference
// method results. Parsed items are not admitted occurrences or method receipts.
package adr0011methodresult

import (
	"bytes"
	"encoding/json"
	"strconv"

	transport "lsp-trace/internal/adr0011methodtransport"
	"lsp-trace/internal/strictjson"
)

type Code string

const (
	FailureTransport Code = "TRANSPORT_NOT_SUCCESS"
	FailureMethod    Code = "INVALID_METHOD"
	FailureLimit     Code = "INVALID_LIMIT"
	FailureMalformed Code = "MALFORMED"
	FailureResource  Code = "RESOURCE_LIMIT"
)

type Failure struct {
	Code    Code
	Ordinal int // -1 for a whole-result failure; otherwise the zero-based item.
}

func (f *Failure) Error() string { return string(f.Code) }

type Position struct{ Line, Character uint32 }
type Range struct{ Start, End Position }
type Kind string

const (
	Location     Kind = "LOCATION"
	LocationLink Kind = "LOCATION_LINK"
)

// Item retains server-reported ranges only; no source body or relation is inferred.
type Item struct {
	Ordinal              int
	Kind                 Kind
	URI                  string
	Range                Range  // Location.range or LocationLink.targetSelectionRange.
	TargetRange          *Range // LocationLink.targetRange only.
	OriginSelectionRange *Range // Optional LocationLink hint, never a query replacement.
}

type Result struct {
	Items []Item
	Null  bool // Distinguishes raw null from an empty array.
}

// Parse uses the method recorded by the bounded transport; it cannot choose a
// different method from caller-supplied raw bytes. It does not admit evidence or
// decide request completeness. On any failure it returns no parsed items.
func Parse(wire transport.Result, maxCandidates int) (Result, *Failure) {
	if wire.Outcome() != transport.OutcomeTransportSuccess {
		return Result{}, &Failure{Code: FailureTransport, Ordinal: -1}
	}
	return parseRawUntrusted(wire.Method(), wire.Raw(), maxCandidates)
}

// ParseRawReferences replays exact retained bytes without trusting evaluation.Items.
// Parsing conveys no transport custody, occurrence admission, or issuance.
func ParseRawReferences(raw []byte, maxCandidates int) (Result, *Failure) {
	return parseRawUntrusted(transport.MethodReferences, raw, maxCandidates)
}

// parseRawUntrusted replays the method-result grammar without manufacturing a
// transport outcome or producer custody. Only a separately bound transaction
// record can use its output, and parsing alone never admits an occurrence.
func parseRawUntrusted(method string, raw []byte, maxCandidates int) (Result, *Failure) {
	if method != transport.MethodDefinition && method != transport.MethodReferences {
		return Result{}, &Failure{Code: FailureMethod, Ordinal: -1}
	}
	if maxCandidates <= 0 {
		return Result{}, &Failure{Code: FailureLimit, Ordinal: -1}
	}
	// Validate the complete result first, then check duplicate keys at each
	// member boundary so a nested duplicate retains its response ordinal.
	if !json.Valid(raw) {
		return Result{}, &Failure{Code: FailureMalformed, Ordinal: -1}
	}
	trimmed := bytes.TrimSpace(raw)
	if bytes.Equal(trimmed, []byte("null")) {
		return Result{Items: []Item{}, Null: true}, nil
	}
	if len(trimmed) == 0 {
		return Result{}, &Failure{Code: FailureMalformed, Ordinal: -1}
	}
	if trimmed[0] == '{' {
		if method != transport.MethodDefinition {
			return Result{}, &Failure{Code: FailureMalformed, Ordinal: -1}
		}
		if err := strictjson.RejectDuplicates(trimmed); err != nil {
			return Result{}, &Failure{Code: FailureMalformed, Ordinal: 0}
		}
		item, ok := parseItem(trimmed, 0)
		if !ok || item.Kind != Location {
			return Result{}, &Failure{Code: FailureMalformed, Ordinal: 0}
		}
		return Result{Items: []Item{item}}, nil
	}
	if trimmed[0] != '[' {
		return Result{}, &Failure{Code: FailureMalformed, Ordinal: -1}
	}
	dec := json.NewDecoder(bytes.NewReader(trimmed))
	if _, err := dec.Token(); err != nil {
		return Result{}, &Failure{Code: FailureMalformed, Ordinal: -1}
	}
	items := make([]Item, 0)
	var kind Kind
	for dec.More() {
		if len(items) >= maxCandidates {
			return Result{}, &Failure{Code: FailureResource, Ordinal: len(items)}
		}
		var member json.RawMessage
		if err := dec.Decode(&member); err != nil {
			return Result{}, &Failure{Code: FailureMalformed, Ordinal: len(items)}
		}
		if err := strictjson.RejectDuplicates(member); err != nil {
			return Result{}, &Failure{Code: FailureMalformed, Ordinal: len(items)}
		}
		item, ok := parseItem(member, len(items))
		if !ok || method == transport.MethodReferences && item.Kind != Location || kind != "" && kind != item.Kind {
			return Result{}, &Failure{Code: FailureMalformed, Ordinal: len(items)}
		}
		kind = item.Kind
		items = append(items, item)
	}
	if _, err := dec.Token(); err != nil {
		return Result{}, &Failure{Code: FailureMalformed, Ordinal: -1}
	}
	return Result{Items: items}, nil
}

func parseItem(raw []byte, ordinal int) (Item, bool) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil || fields == nil {
		return Item{}, false
	}
	_, uri := fields["uri"]
	_, locationRange := fields["range"]
	_, targetURI := fields["targetUri"]
	_, targetRange := fields["targetRange"]
	_, targetSelection := fields["targetSelectionRange"]
	_, origin := fields["originSelectionRange"]
	if (uri || locationRange) && (targetURI || targetRange || targetSelection || origin) {
		return Item{}, false
	}
	if uri || locationRange {
		name, ok := parseURI(fields["uri"])
		if !ok {
			return Item{}, false
		}
		r, ok := parseRange(fields["range"])
		if !ok {
			return Item{}, false
		}
		return Item{Ordinal: ordinal, Kind: Location, URI: name, Range: r}, true
	}
	if !targetURI && !targetRange && !targetSelection {
		return Item{}, false
	}
	name, ok := parseURI(fields["targetUri"])
	if !ok {
		return Item{}, false
	}
	envelope, ok := parseRange(fields["targetRange"])
	if !ok {
		return Item{}, false
	}
	selection, ok := parseRange(fields["targetSelectionRange"])
	if !ok || !contains(envelope, selection) {
		return Item{}, false
	}
	item := Item{Ordinal: ordinal, Kind: LocationLink, URI: name, Range: selection, TargetRange: &envelope}
	if origin {
		r, valid := parseRange(fields["originSelectionRange"])
		if !valid {
			return Item{}, false
		}
		item.OriginSelectionRange = &r
	}
	return item, true
}

func parseURI(raw json.RawMessage) (string, bool) {
	if len(raw) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return "", false
	}
	var uri string
	return uri, json.Unmarshal(raw, &uri) == nil && uri != ""
}

func parseRange(raw json.RawMessage) (Range, bool) {
	var fields map[string]json.RawMessage
	if len(raw) == 0 || json.Unmarshal(raw, &fields) != nil || fields == nil {
		return Range{}, false
	}
	start, ok := parsePosition(fields["start"])
	if !ok {
		return Range{}, false
	}
	end, ok := parsePosition(fields["end"])
	if !ok || compare(start, end) > 0 {
		return Range{}, false
	}
	return Range{Start: start, End: end}, true
}

func parsePosition(raw json.RawMessage) (Position, bool) {
	var fields map[string]json.RawMessage
	if len(raw) == 0 || json.Unmarshal(raw, &fields) != nil || fields == nil {
		return Position{}, false
	}
	line, ok := coordinate(fields["line"])
	if !ok {
		return Position{}, false
	}
	character, ok := coordinate(fields["character"])
	return Position{Line: line, Character: character}, ok
}

func coordinate(raw json.RawMessage) (uint32, bool) {
	// Valid whole JSON was checked first; only a decimal integer token can pass.
	n, err := strconv.ParseUint(string(raw), 10, 31)
	return uint32(n), err == nil
}

func compare(a, b Position) int {
	if a.Line < b.Line || a.Line == b.Line && a.Character < b.Character {
		return -1
	}
	if a == b {
		return 0
	}
	return 1
}

func contains(outer, inner Range) bool {
	return compare(outer.Start, inner.Start) <= 0 && compare(inner.End, outer.End) <= 0
}
