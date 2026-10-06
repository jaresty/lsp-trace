// Package adr0011methodresult parses bounded, successful definition/reference
// method results. Parsed items are not admitted occurrences or method receipts.
package adr0011methodresult

import (
	"bytes"
	"encoding/json"
	"strconv"

	transport "lsp-trace/internal/adr0011methodtransport"
	"lsp-trace/internal/objectadmission"
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
	return parseRawUntrustedWithAdmission(method, raw, maxCandidates, nil)
}

// parseRawUntrustedWithAdmission is the private C16 parser seam. It validates
// the complete result before admitting any target and constructs each Item only
// inside the admission callback. A nil admitter preserves ordinary parsing.
func parseRawUntrustedWithAdmission(method string, raw []byte, maxCandidates int, admitter objectadmission.ObjectAdmitter) (Result, *Failure) {
	if method != transport.MethodDefinition && method != transport.MethodReferences {
		return Result{}, &Failure{Code: FailureMethod, Ordinal: -1}
	}
	if maxCandidates <= 0 {
		return Result{}, &Failure{Code: FailureLimit, Ordinal: -1}
	}
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
		plan, ok := parseItemPlan(trimmed, 0)
		if !ok || plan.kind != Location {
			return Result{}, &Failure{Code: FailureMalformed, Ordinal: 0}
		}
		var item Item
		if err := withObjectAdmission(admitter, func() error {
			item = plan.materialize()
			return nil
		}); err != nil {
			return Result{}, &Failure{Code: FailureResource, Ordinal: 0}
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
	plans := make([]privateItemPlan, 0)
	var kind Kind
	for dec.More() {
		if len(plans) >= maxCandidates {
			return Result{}, &Failure{Code: FailureResource, Ordinal: len(plans)}
		}
		var member json.RawMessage
		if err := dec.Decode(&member); err != nil {
			return Result{}, &Failure{Code: FailureMalformed, Ordinal: len(plans)}
		}
		if err := strictjson.RejectDuplicates(member); err != nil {
			return Result{}, &Failure{Code: FailureMalformed, Ordinal: len(plans)}
		}
		plan, ok := parseItemPlan(member, len(plans))
		if !ok || method == transport.MethodReferences && plan.kind != Location || kind != "" && kind != plan.kind {
			return Result{}, &Failure{Code: FailureMalformed, Ordinal: len(plans)}
		}
		kind = plan.kind
		plans = append(plans, plan)
	}
	if _, err := dec.Token(); err != nil {
		return Result{}, &Failure{Code: FailureMalformed, Ordinal: -1}
	}
	items := make([]Item, 0, len(plans))
	for i := range plans {
		if err := withObjectAdmission(admitter, func() error {
			items = append(items, plans[i].materialize())
			return nil
		}); err != nil {
			return Result{}, &Failure{Code: FailureResource, Ordinal: i}
		}
	}
	return Result{Items: items}, nil
}

func withObjectAdmission(admitter objectadmission.ObjectAdmitter, fn func() error) error {
	if admitter == nil {
		return fn()
	}
	return admitter.WithObjectAdmission(fn)
}

type privateItemPlan struct {
	ordinal                   int
	kind                      Kind
	uri                       string
	rangeValue                Range
	targetRange               Range
	originSelectionRange      Range
	hasTargetRange, hasOrigin bool
}

func (p privateItemPlan) materialize() Item {
	item := Item{Ordinal: p.ordinal, Kind: p.kind, URI: p.uri, Range: p.rangeValue}
	if p.hasTargetRange {
		r := p.targetRange
		item.TargetRange = &r
	}
	if p.hasOrigin {
		r := p.originSelectionRange
		item.OriginSelectionRange = &r
	}
	return item
}

func parseItem(raw []byte, ordinal int) (Item, bool) {
	plan, ok := parseItemPlan(raw, ordinal)
	if !ok {
		return Item{}, false
	}
	return plan.materialize(), true
}

func parseItemPlan(raw []byte, ordinal int) (privateItemPlan, bool) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil || fields == nil {
		return privateItemPlan{}, false
	}
	_, uri := fields["uri"]
	_, locationRange := fields["range"]
	_, targetURI := fields["targetUri"]
	_, targetRange := fields["targetRange"]
	_, targetSelection := fields["targetSelectionRange"]
	_, origin := fields["originSelectionRange"]
	if (uri || locationRange) && (targetURI || targetRange || targetSelection || origin) {
		return privateItemPlan{}, false
	}
	if uri || locationRange {
		name, ok := parseURI(fields["uri"])
		if !ok {
			return privateItemPlan{}, false
		}
		r, ok := parseRange(fields["range"])
		if !ok {
			return privateItemPlan{}, false
		}
		return privateItemPlan{ordinal: ordinal, kind: Location, uri: name, rangeValue: r}, true
	}
	if !targetURI && !targetRange && !targetSelection {
		return privateItemPlan{}, false
	}
	name, ok := parseURI(fields["targetUri"])
	if !ok {
		return privateItemPlan{}, false
	}
	envelope, ok := parseRange(fields["targetRange"])
	if !ok {
		return privateItemPlan{}, false
	}
	selection, ok := parseRange(fields["targetSelectionRange"])
	if !ok || !contains(envelope, selection) {
		return privateItemPlan{}, false
	}
	plan := privateItemPlan{ordinal: ordinal, kind: LocationLink, uri: name, rangeValue: selection, targetRange: envelope, hasTargetRange: true}
	if origin {
		r, valid := parseRange(fields["originSelectionRange"])
		if !valid {
			return privateItemPlan{}, false
		}
		plan.originSelectionRange, plan.hasOrigin = r, true
	}
	return plan, true
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
