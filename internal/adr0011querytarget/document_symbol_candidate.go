// Package adr0011querytarget tests an unadmitted document-symbol derivation.
// Candidate identities do not establish a method receipt or references target.
package adr0011querytarget

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"

	"lsp-trace/internal/strictjson"
)

const CandidateVersion = "lsp-trace.private.adr0011-document-symbol-query-target.v0"
const CandidateVersionV1 = "lsp-trace.private.adr0011-document-symbol-query-target.v1"
const maxResultBytes = 1 << 20
const maxSymbols = 10000
const maxDepth = 64
const maxJSONDepth = 512
const maxJSONTokens = 100000

var ErrUnresolved = errors.New("unadmitted document-symbol query target unresolved")

type Position struct{ Line, Character uint32 }
type Range struct{ Start, End Position }

// Query must be declared separately from any references result. Source digest,
// version and session coordinates are caller assertions until a separately
// admitted document-symbol transaction and source supply bind them.
type Query struct {
	OccurrenceID, URI, Encoding, DocumentVersion, SourceDigest string
	SessionID                                                  string
	Generation                                                 uint64
	Line, Character                                            uint32
}

type Candidate struct {
	Version, ID, SymbolID, QueryOccurrenceID, SymbolName string
	SymbolKind                                           int
	QueryURI, SessionID, Encoding, DocumentVersion       string
	SourceDigest, DocumentSymbolResultSHA256             string
	Generation                                           uint64
	QueryLine, QueryCharacter                            uint32
	DisplayRange, SelectionRange                         Range
}

type point struct{ Line, Character *uint32 }
type span struct{ Start, End *point }
type symbol struct {
	Name           string   `json:"name"`
	Detail         string   `json:"detail,omitempty"`
	Kind           int      `json:"kind"`
	Tags           []int    `json:"tags,omitempty"`
	Deprecated     bool     `json:"deprecated,omitempty"`
	Range          *span    `json:"range"`
	SelectionRange *span    `json:"selectionRange"`
	Children       []symbol `json:"children,omitempty"`
}

func compare(a, b Position) int {
	if a.Line < b.Line || a.Line == b.Line && a.Character < b.Character {
		return -1
	}
	if a != b {
		return 1
	}
	return 0
}
func within(r Range, p Position) bool { return compare(r.Start, p) <= 0 && compare(p, r.End) < 0 }
func encloses(outer, inner Range) bool {
	return compare(outer.Start, inner.Start) <= 0 && compare(inner.End, outer.End) <= 0
}
func unpack(r *span) (Range, bool) {
	if r == nil || r.Start == nil || r.End == nil || r.Start.Line == nil || r.Start.Character == nil || r.End.Line == nil || r.End.Character == nil {
		return Range{}, false
	}
	out := Range{Start: Position{*r.Start.Line, *r.Start.Character}, End: Position{*r.End.Line, *r.End.Character}}
	return out, compare(out.Start, out.End) < 0
}
func boundedJSONTokens(raw []byte) bool {
	dec := json.NewDecoder(bytes.NewReader(raw))
	depth, tokens := 0, 0
	for {
		token, err := dec.Token()
		if err == io.EOF {
			return depth == 0
		}
		if err != nil {
			return false
		}
		tokens++
		if tokens > maxJSONTokens {
			return false
		}
		if delim, ok := token.(json.Delim); ok {
			switch delim {
			case '{', '[':
				depth++
				if depth > maxJSONDepth {
					return false
				}
			case '}', ']':
				depth--
			}
		}
	}
}
func validSHA256(s string) bool {
	if len(s) != len("sha256:")+64 || s[:7] != "sha256:" {
		return false
	}
	b, err := hex.DecodeString(s[7:])
	return err == nil && len(b) == 32 && "sha256:"+hex.EncodeToString(b) == s
}
func hash(domain string, v any) string {
	raw, _ := json.Marshal(v)
	sum := sha256.Sum256(append([]byte(domain+"\x00"), raw...))
	return "sha256:" + hex.EncodeToString(sum[:])
}

// SelectDocumentSymbolCandidate accepts only a complete bounded hierarchical
// documentSymbol array, with exactly ONE selectionRange containing the query
// position. Parent display containment or references-returned locations cannot
// break ties, select a symbol, or invent its pre-existing identity. No I/O,
// LSP invocation, query admission, or source/document verification occurs here.
func SelectDocumentSymbolCandidate(q Query, raw []byte) (Candidate, error) {
	if q.OccurrenceID == "" || q.URI == "" || (q.Encoding != "utf-8" && q.Encoding != "utf-16" && q.Encoding != "utf-32") || q.DocumentVersion == "" || !validSHA256(q.SourceDigest) || q.SessionID == "" || q.Generation == 0 ||
		len(raw) == 0 || len(raw) > maxResultBytes || !boundedJSONTokens(raw) || strictjson.RejectDuplicates(raw) != nil || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return Candidate{}, ErrUnresolved
	}
	var roots []symbol
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if dec.Decode(&roots) != nil || roots == nil || dec.Decode(&struct{}{}) != io.EOF {
		return Candidate{}, ErrUnresolved
	}
	// The pre-decode token bound limits JSON nesting/work; this separate
	// post-decode bound limits hierarchical symbol depth and item cardinality.
	type entry struct {
		s      symbol
		depth  int
		parent *Range
	}
	stack := make([]entry, 0, len(roots))
	for _, root := range roots {
		stack = append(stack, entry{s: root, depth: 1})
	}
	var chosen *Candidate
	count := 0
	for len(stack) > 0 {
		e := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		count++
		if count > maxSymbols || e.depth > maxDepth || e.s.Name == "" || e.s.Kind < 1 || e.s.Kind > 26 {
			return Candidate{}, ErrUnresolved
		}
		display, ok := unpack(e.s.Range)
		if !ok {
			return Candidate{}, ErrUnresolved
		}
		selection, ok := unpack(e.s.SelectionRange)
		if !ok || !encloses(display, selection) || e.parent != nil && !encloses(*e.parent, display) {
			return Candidate{}, ErrUnresolved
		}
		for _, child := range e.s.Children {
			parent := display
			stack = append(stack, entry{s: child, depth: e.depth + 1, parent: &parent})
		}
		if within(selection, Position{q.Line, q.Character}) {
			if chosen != nil {
				return Candidate{}, ErrUnresolved // Even duplicate provider items are ambiguous.
			}
			c := Candidate{Version: CandidateVersion, QueryOccurrenceID: q.OccurrenceID, QueryURI: q.URI, QueryLine: q.Line, QueryCharacter: q.Character,
				SessionID: q.SessionID, Generation: q.Generation, Encoding: q.Encoding, DocumentVersion: q.DocumentVersion, SourceDigest: q.SourceDigest,
				SymbolName: e.s.Name, SymbolKind: e.s.Kind, DisplayRange: display, SelectionRange: selection,
				DocumentSymbolResultSHA256: hash("lsp-trace:adr0011:unadmitted-document-symbol-result:v0", raw)}
			symbolIdentity := struct {
				URI, SessionID, Encoding, DocumentVersion, SourceDigest, ResultSHA256 string
				Generation                                                            uint64
				Name                                                                  string
				Kind                                                                  int
				Display, Selection                                                    Range
			}{q.URI, q.SessionID, q.Encoding, q.DocumentVersion, q.SourceDigest, c.DocumentSymbolResultSHA256, q.Generation, e.s.Name, e.s.Kind, display, selection}
			c.SymbolID = hash("lsp-trace:adr0011:unadmitted-document-symbol-identity:v0", symbolIdentity)
			c.ID = hash("lsp-trace:adr0011:unadmitted-query-target-candidate:v0", c)
			chosen = &c
		}
	}
	if chosen == nil {
		return Candidate{}, ErrUnresolved
	}
	return *chosen, nil
}

// SelectDocumentSymbolCandidateV1 is a private, unadmitted selector. It
// validates the entire hierarchy before returning a most-specific match.
func SelectDocumentSymbolCandidateV1(q Query, raw []byte) (Candidate, error) {
	return selectDocumentSymbolCandidateV1(q, raw, nil)
}

// observe is a package-private test seam for actual finalization operations.
func selectDocumentSymbolCandidateV1(q Query, raw []byte, observe func(string)) (Candidate, error) {
	if q.OccurrenceID == "" || q.URI == "" || (q.Encoding != "utf-8" && q.Encoding != "utf-16" && q.Encoding != "utf-32") || q.DocumentVersion == "" || !validSHA256(q.SourceDigest) || q.SessionID == "" || q.Generation == 0 ||
		len(raw) == 0 || len(raw) > maxResultBytes || !boundedJSONTokens(raw) || strictjson.RejectDuplicates(raw) != nil || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return Candidate{}, ErrUnresolved
	}
	var roots []symbol
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if dec.Decode(&roots) != nil || roots == nil || dec.Decode(&struct{}{}) != io.EOF {
		return Candidate{}, ErrUnresolved
	}
	type entry struct {
		s      symbol
		depth  int
		parent *Range
	}
	stack := make([]entry, 0, len(roots))
	for _, root := range roots {
		stack = append(stack, entry{s: root, depth: 1})
	}
	var matches []Candidate
	count := 0
	for len(stack) > 0 {
		e := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		count++
		if count > maxSymbols || e.depth > maxDepth || e.s.Name == "" || e.s.Kind < 1 || e.s.Kind > 26 {
			return Candidate{}, ErrUnresolved
		}
		display, ok := unpack(e.s.Range)
		if !ok {
			return Candidate{}, ErrUnresolved
		}
		selection, ok := unpack(e.s.SelectionRange)
		if !ok || !encloses(display, selection) || e.parent != nil && !encloses(*e.parent, display) {
			return Candidate{}, ErrUnresolved
		}
		for _, child := range e.s.Children {
			parent := display
			stack = append(stack, entry{s: child, depth: e.depth + 1, parent: &parent})
		}
		if !within(selection, Position{q.Line, q.Character}) {
			continue
		}
		matches = append(matches, Candidate{SymbolName: e.s.Name, SymbolKind: e.s.Kind, DisplayRange: display, SelectionRange: selection})
	}
	index, _ := mostSpecificV1(matches)
	if index < 0 {
		return Candidate{}, ErrUnresolved
	}
	selected := matches[index]
	c := Candidate{Version: CandidateVersionV1, QueryOccurrenceID: q.OccurrenceID, QueryURI: q.URI, QueryLine: q.Line, QueryCharacter: q.Character,
		SessionID: q.SessionID, Generation: q.Generation, Encoding: q.Encoding, DocumentVersion: q.DocumentVersion, SourceDigest: q.SourceDigest,
		SymbolName: selected.SymbolName, SymbolKind: selected.SymbolKind, DisplayRange: selected.DisplayRange, SelectionRange: selected.SelectionRange,
		DocumentSymbolResultSHA256: hash("lsp-trace:adr0011:unadmitted-document-symbol-result:v1", raw)}
	if observe != nil {
		observe("raw")
		observe("identity")
	}
	identity := struct {
		URI, SessionID, Encoding, DocumentVersion, SourceDigest, ResultSHA256 string
		Generation                                                            uint64
		Name                                                                  string
		Kind                                                                  int
		Display, Selection                                                    Range
	}{q.URI, q.SessionID, q.Encoding, q.DocumentVersion, q.SourceDigest, c.DocumentSymbolResultSHA256, q.Generation, selected.SymbolName, selected.SymbolKind, selected.DisplayRange, selected.SelectionRange}
	c.SymbolID = hash("lsp-trace:adr0011:unadmitted-document-symbol-identity:v1", identity)
	c.ID = hash("lsp-trace:adr0011:unadmitted-query-target-candidate:v1", c)
	return c, nil
}

// mostSpecificV1 returns the unique strict minimum and the number of range
// comparisons performed. The count is observable only to package tests.
func mostSpecificV1(matches []Candidate) (int, int) {
	if len(matches) == 0 {
		return -1, 0
	}
	chosen, comparisons := 0, 0
	// A strict subset of every other range must have the greatest start;
	// among equal starts it must have the smallest end. This only proposes
	// a winner: the verification pass rejects ties and overlapping ranges.
	for i := 1; i < len(matches); i++ {
		comparisons++
		current, best := matches[i].SelectionRange, matches[chosen].SelectionRange
		if compare(current.Start, best.Start) > 0 || current.Start == best.Start && compare(current.End, best.End) < 0 {
			chosen = i
		}
	}
	for i := range matches {
		if i == chosen {
			continue
		}
		comparisons++
		candidate, other := matches[chosen].SelectionRange, matches[i].SelectionRange
		if !encloses(other, candidate) || other == candidate {
			return -1, comparisons
		}
	}
	return chosen, comparisons
}
