// Package adr0007locationv5 implements the private prospective-v5 location evaluator.
package adr0007locationv5

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math"
	"sort"
	"strings"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
	src "lsp-trace/internal/sourceadmissionv2"
)

const (
	RequestSchema  = "lsp-trace.adr0007.location-intersection.request.private.v5"
	ResultSchema   = "lsp-trace.adr0007.location-intersection.result.private.v5"
	EnvelopeSchema = "lsp-trace.adr0007.source-admission-envelope.private.v5"
	PolicyDigest   = "sha256:ba2dd40c552f9492be66b67b760f366d5c80af292278a33510f97dd2a1b7972e"
	LimitsDigest   = "sha256:55553e121b51ad8f3d4c491ffb5f2635cbc608131d721afcdb0423046625a8d3"
)

type Limits struct{ MaxRequestBytes, MaxMembers, MaxSelectorPaths, MaxRangesPerPath, MaxTotalRanges, MaxFrozenPaths, MaxWitnesses, MaxTopK, MaxSources, MaxSourceBytes, MaxTotalSourceBytes, MaxOutputBytes, MaxWork uint64 }

func PublishedLimits() Limits {
	return Limits{1048576, 10000, 1000, 10000, 100000, 1000, 10000, 10000, 1000, 1048576, 8388608, 8388608, 50000000}
}

type Control interface {
	Cancelled() bool
	DeadlineExceeded() bool
}
type StaticControl struct{ Cancel, Deadline bool }

func (s StaticControl) Cancelled() bool        { return s.Cancel }
func (s StaticControl) DeadlineExceeded() bool { return s.Deadline }

type ContextControl struct{ Context context.Context }

func (c ContextControl) Cancelled() bool {
	return c.Context != nil && errors.Is(c.Context.Err(), context.Canceled)
}
func (c ContextControl) DeadlineExceeded() bool {
	return c.Context != nil && errors.Is(c.Context.Err(), context.DeadlineExceeded)
}

type Position struct {
	Line      uint64 `json:"line"`
	Character uint64 `json:"character"`
}
type Range struct {
	Start Position `json:"start"`
	End   Position `json:"end"`
}
type PathRanges struct {
	Path   string  `json:"path"`
	Ranges []Range `json:"ranges"`
}
type Selector struct {
	Kind        string       `json:"kind"`
	Path        string       `json:"path,omitempty"`
	FrozenPaths []string     `json:"frozenPaths,omitempty"`
	Union       []PathRanges `json:"union,omitempty"`
}
type Member struct {
	ID, Path, Revision, FileDigest, ObjectDigest string
	Ranges                                       []Range
	Available, PolicyAllowed                     bool
	Score                                        uint64
}

func (m *Member) UnmarshalJSON(b []byte) error {
	type wire struct {
		ID            string  `json:"id"`
		Path          string  `json:"path"`
		Revision      string  `json:"revision"`
		FileDigest    string  `json:"fileDigest"`
		ObjectDigest  string  `json:"objectDigest"`
		Ranges        []Range `json:"ranges"`
		Available     bool    `json:"available"`
		PolicyAllowed bool    `json:"policyAllowed"`
		Score         uint64  `json:"score"`
	}
	var w wire
	if err := json.Unmarshal(b, &w); err != nil {
		return err
	}
	*m = Member(w)
	return nil
}

type Request struct {
	Schema, ID, Relation                        string
	Selector                                    Selector
	AdmissionDigest, PolicyDigest, LimitsDigest string
	TopK                                        uint64
	Members                                     []Member
}

func (q *Request) UnmarshalJSON(b []byte) error {
	type wire struct {
		Schema          string   `json:"schema"`
		ID              string   `json:"id"`
		Relation        string   `json:"relation"`
		Selector        Selector `json:"selector"`
		AdmissionDigest string   `json:"admissionDigest"`
		PolicyDigest    string   `json:"policyDigest"`
		LimitsDigest    string   `json:"limitsDigest"`
		TopK            uint64   `json:"topK"`
		Members         []Member `json:"members"`
	}
	var w wire
	if err := json.Unmarshal(b, &w); err != nil {
		return err
	}
	*q = Request(w)
	return nil
}

type Witness struct {
	Path           string `json:"path"`
	SelectorRange  Range  `json:"selectorRange"`
	CandidateRange Range  `json:"candidateRange"`
	Intersection   Range  `json:"intersection"`
	Revision       string `json:"revision"`
	FileDigest     string `json:"fileDigest"`
	ObjectDigest   string `json:"objectDigest"`
}
type MemberRow struct {
	Ordinal   uint64    `json:"ordinal"`
	MemberID  string    `json:"memberId"`
	Outcome   string    `json:"outcome"`
	Witnesses []Witness `json:"witnesses"`
}
type Ranked struct {
	Ordinal  uint64 `json:"ordinal"`
	MemberID string `json:"memberId"`
	Score    uint64 `json:"score"`
}
type Counters struct{ Input, Eligible, Ineligible, UnavailableLocation, InvalidLocation, DuplicateMember, FilteredByPolicy, Ranked, Witnesses, Work, SourceBytes, OutputBytes uint64 }

func (c Counters) MarshalJSON() ([]byte, error) {
	type w struct {
		Input               uint64 `json:"input"`
		Eligible            uint64 `json:"eligible"`
		Ineligible          uint64 `json:"ineligible"`
		UnavailableLocation uint64 `json:"unavailableLocation"`
		InvalidLocation     uint64 `json:"invalidLocation"`
		DuplicateMember     uint64 `json:"duplicateMember"`
		FilteredByPolicy    uint64 `json:"filteredByPolicy"`
		Ranked              uint64 `json:"ranked"`
		Witnesses           uint64 `json:"witnesses"`
		Work                uint64 `json:"work"`
		SourceBytes         uint64 `json:"sourceBytes"`
		OutputBytes         uint64 `json:"outputBytes"`
	}
	return json.Marshal(w(c))
}

type Result struct {
	Schema    string      `json:"schema"`
	RequestID string      `json:"requestId"`
	Outcome   string      `json:"outcome"`
	Members   []MemberRow `json:"members"`
	Ranked    []Ranked    `json:"ranked"`
	Counters  Counters    `json:"counters"`
	Detail    string      `json:"detail"`
}

func failure(id, out, detail string) Result {
	return Result{ResultSchema, id, out, []MemberRow{}, []Ranked{}, Counters{}, detail}
}
func cloneResult(r Result) Result {
	members := make([]MemberRow, len(r.Members))
	copy(members, r.Members)
	r.Members = members
	for i := range r.Members {
		witnesses := make([]Witness, len(r.Members[i].Witnesses))
		copy(witnesses, r.Members[i].Witnesses)
		r.Members[i].Witnesses = witnesses
	}
	ranked := make([]Ranked, len(r.Ranked))
	copy(ranked, r.Ranked)
	r.Ranked = ranked
	return r
}
func Canonical(r Result) ([]byte, error) {
	b, e := json.Marshal(r)
	if e != nil {
		return nil, e
	}
	return append(b, '\n'), nil
}

type sourceWire struct{ Path, Revision, FileDigest, ObjectDigest, Bytes string }
type envelope struct {
	Outcome, Detail, DuplicatePath string
	Sources                        []sourceWire
	AdmissionDigest                string
}
type work struct{ v, max uint64 }

func (w *work) add(k, n uint64) bool {
	if n != 0 && k > math.MaxUint64/n {
		return false
	}
	x := k * n
	if w.v > math.MaxUint64-x || w.v+x > w.max {
		return false
	}
	w.v += x
	return true
}
func choose2(n uint64) (uint64, bool) {
	if n < 2 {
		return 0, true
	}
	a, b := n, n-1
	if a%2 == 0 {
		a /= 2
	} else {
		b /= 2
	}
	if b != 0 && a > math.MaxUint64/b {
		return 0, false
	}
	return a * b, true
}
func poll(c Control) (string, string, bool) {
	if c != nil && c.Cancelled() {
		return "CANCELLED", "CANCEL_SIGNAL", true
	}
	if c != nil && c.DeadlineExceeded() {
		return "TIMEOUT", "DEADLINE", true
	}
	return "", "", false
}

func Evaluate(raw, bindingRaw []byte, c Control, limits Limits) (Result, error) {
	id := ""
	if o, d, x := poll(c); x {
		return failure(id, o, d), nil
	}
	if uint64(len(raw)) > limits.MaxRequestBytes {
		return failure(id, "INVALID_REQUEST", "REQUEST_FIELD"), nil
	}
	wk := work{max: limits.MaxWork}
	if !wk.add(3, uint64(len(raw))) {
		return failure(id, "RESOURCE_LIMIT", "WORK"), nil
	}
	q, detail := strictRequest(raw)
	if q != nil {
		id = q.ID
	}
	if detail != "" {
		return failure(id, "INVALID_REQUEST", detail), nil
	}
	if o, d, x := poll(c); x {
		return failure(id, o, d), nil
	}
	if uint64(len(q.Members)) > limits.MaxMembers {
		return failure(id, "RESOURCE_LIMIT", "MEMBERS"), nil
	}
	if q.TopK > limits.MaxTopK {
		return failure(id, "RESOURCE_LIMIT", "TOP_K"), nil
	}
	for _, m := range q.Members {
		if o, d, x := poll(c); x {
			return failure(id, o, d), nil
		}
		if !wk.add(13, 1) {
			return failure(id, "RESOURCE_LIMIT", "WORK"), nil
		}
		if m.ID == "" || !norm.NFC.IsNormalString(m.ID) {
			return failure(id, "INVALID_REQUEST", "REQUEST_FIELD"), nil
		}
	}
	if o, d, x := poll(c); x {
		return failure(id, o, d), nil
	}
	env, eo, ed := parseEnvelope(bindingRaw, &wk, limits)
	if eo != "" {
		return failure(id, eo, ed), nil
	}
	if o, d, x := poll(c); x {
		return failure(id, o, d), nil
	}
	if env.AdmissionDigest != q.AdmissionDigest {
		return failure(id, "SOURCE_ADMISSION_MISMATCH", "BINDING_DIGEST"), nil
	}
	if q.PolicyDigest != PolicyDigest {
		return failure(id, "POLICY_MISMATCH", "POLICY_DIGEST"), nil
	}
	if q.LimitsDigest != LimitsDigest {
		return failure(id, "POLICY_MISMATCH", "LIMITS_DIGEST"), nil
	}
	sm := map[string]src.SelectedSource{}
	for _, s := range env.Sources {
		bb, _ := base64.StdEncoding.DecodeString(s.Bytes)
		sm[s.Path] = src.SelectedSource{Path: s.Path, Revision: s.Revision, FileDigest: s.FileDigest, ObjectDigest: s.ObjectDigest, Bytes: bb}
	}
	sel, so, sd := selectorRanges(q.Selector, sm, &wk, limits)
	if so != "" {
		return failure(id, so, sd), nil
	}
	needed := map[string]bool{}
	for p := range sel {
		needed[p] = true
	}
	seen := map[string]bool{}
	for _, m := range q.Members {
		if !seen[m.ID] {
			seen[m.ID] = true
			if m.Available {
				if _, ok := sm[m.Path]; ok {
					needed[m.Path] = true
				}
			}
		}
	}
	var sourceBytes uint64
	for p := range needed {
		sourceBytes += uint64(len(sm[p].Bytes))
	}
	if sourceBytes > limits.MaxTotalSourceBytes {
		return failure(id, "RESOURCE_LIMIT", "SOURCE_BYTES"), nil
	}
	rows := make([]MemberRow, 0, len(q.Members))
	seen = map[string]bool{}
	var counts Counters
	counts.Input = uint64(len(q.Members))
	ranking := []Ranked{}
	for i, m := range q.Members {
		if o, d, x := poll(c); x {
			return failure(id, o, d), nil
		}
		row := MemberRow{uint64(i), m.ID, "", []Witness{}}
		if seen[m.ID] {
			row.Outcome = "DUPLICATE_MEMBER"
			counts.DuplicateMember++
			rows = append(rows, row)
			continue
		}
		seen[m.ID] = true
		if !m.Available {
			row.Outcome = "UNAVAILABLE_LOCATION"
			counts.UnavailableLocation++
			rows = append(rows, row)
			continue
		}
		if !wk.add(7, 1) || !src.CanonicalPath(m.Path) {
			if wk.v > wk.max {
				return failure(id, "RESOURCE_LIMIT", "WORK"), nil
			}
			row.Outcome = "INVALID_LOCATION"
			counts.InvalidLocation++
			rows = append(rows, row)
			continue
		}
		s, ok := sm[m.Path]
		if !ok || s.Revision != m.Revision || s.FileDigest != m.FileDigest || s.ObjectDigest != m.ObjectDigest {
			row.Outcome = "INVALID_LOCATION"
			counts.InvalidLocation++
			rows = append(rows, row)
			continue
		}
		valid := true
		for _, r := range m.Ranges {
			if !wk.add(11, 1) {
				return failure(id, "RESOURCE_LIMIT", "WORK"), nil
			}
			if rangeStatus(r, s.Bytes) != "" {
				valid = false
			}
		}
		if !valid {
			row.Outcome = "INVALID_LOCATION"
			counts.InvalidLocation++
			rows = append(rows, row)
			continue
		}
		if !m.PolicyAllowed {
			row.Outcome = "FILTERED_BY_POLICY"
			counts.FilteredByPolicy++
			rows = append(rows, row)
			continue
		}
		var ws []Witness
		for _, a := range sel[m.Path] {
			for _, b := range m.Ranges {
				if o, d, x := poll(c); x {
					return failure(id, o, d), nil
				}
				if !wk.add(19, 1) {
					return failure(id, "RESOURCE_LIMIT", "WORK"), nil
				}
				if relation(q.Relation, a, b) {
					ws = append(ws, Witness{m.Path, a, b, Range{maxPos(a.Start, b.Start), minPos(a.End, b.End)}, m.Revision, m.FileDigest, m.ObjectDigest})
				}
			}
		}
		cp, _ := choose2(uint64(len(ws)))
		if !wk.add(29, cp) {
			return failure(id, "RESOURCE_LIMIT", "WORK"), nil
		}
		sortWitnesses(ws)
		ws = dedupWitnesses(ws)
		row.Witnesses = ws
		if len(ws) > 0 {
			row.Outcome = "ELIGIBLE"
			counts.Eligible++
			ranking = append(ranking, Ranked{uint64(i), m.ID, m.Score})
		} else {
			row.Outcome = "INELIGIBLE"
			counts.Ineligible++
		}
		rows = append(rows, row)
	}
	var nw uint64
	for _, r := range rows {
		nw += uint64(len(r.Witnesses))
	}
	if nw > limits.MaxWitnesses {
		return failure(id, "RESOURCE_LIMIT", "WITNESSES"), nil
	}
	if !wk.add(23, nw) {
		return failure(id, "RESOURCE_LIMIT", "WORK"), nil
	}
	cp, _ := choose2(uint64(len(ranking)))
	if !wk.add(29, cp) {
		return failure(id, "RESOURCE_LIMIT", "WORK"), nil
	}
	sort.SliceStable(ranking, func(i, j int) bool {
		if ranking[i].Score != ranking[j].Score {
			return ranking[i].Score > ranking[j].Score
		}
		if ranking[i].Ordinal != ranking[j].Ordinal {
			return ranking[i].Ordinal < ranking[j].Ordinal
		}
		return ranking[i].MemberID < ranking[j].MemberID
	})
	if uint64(len(ranking)) > q.TopK {
		ranking = ranking[:q.TopK]
	}
	counts.Ranked = uint64(len(ranking))
	counts.Witnesses = nw
	counts.SourceBytes = sourceBytes
	r := Result{ResultSchema, id, "COMPLETE", rows, ranking, counts, "NONE"}
	image, _ := Canonical(r)
	B := uint64(len(image))
	if !wk.add(31, B) {
		return failure(id, "RESOURCE_LIMIT", "WORK"), nil
	}
	if B > limits.MaxOutputBytes {
		return failure(id, "RESOURCE_LIMIT", "OUTPUT_BYTES"), nil
	}
	r.Counters.Work = wk.v
	r.Counters.OutputBytes = B
	if o, d, x := poll(c); x {
		return failure(id, o, d), nil
	}
	return cloneResult(r), nil
}

func strictRequest(raw []byte) (*Request, string) {
	if !utf8.Valid(raw) || bytes.HasPrefix(raw, []byte{0xef, 0xbb, 0xbf}) {
		return nil, "JSON_ENCODING"
	}
	body := raw
	if bytes.HasSuffix(body, []byte{'\n'}) {
		body = body[:len(body)-1]
	}
	if len(body) == 0 || bytes.IndexAny(body, " \t\r\n") >= 0 {
		return nil, "JSON_SYNTAX"
	}
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.UseNumber()
	v, err := parseValue(dec, nil)
	if err != nil {
		return nil, err.Error()
	}
	if dec.More() {
		return nil, "TRAILING_DATA"
	}
	obj, ok := v.(map[string]any)
	if !ok {
		return nil, "JSON_SYNTAX"
	}
	id, _ := obj["id"].(string)
	if hasUnknownRequestField(obj) {
		return &Request{ID: id}, "UNKNOWN_FIELD"
	}
	b, _ := json.Marshal(v)
	var q Request
	if json.Unmarshal(b, &q) != nil {
		return &Request{ID: id}, "REQUEST_FIELD"
	}
	q.ID = id
	if !validRequest(obj, &q) {
		if q.Schema != RequestSchema {
			return &q, "SCHEMA_ID"
		}
		return &q, "REQUEST_FIELD"
	}
	return &q, ""
}
func parseValue(d *json.Decoder, allowed map[string]bool) (any, error) {
	t, e := d.Token()
	if e != nil {
		return nil, errors.New("JSON_SYNTAX")
	}
	switch x := t.(type) {
	case json.Delim:
		if x == '{' {
			m := map[string]any{}
			for d.More() {
				kt, e := d.Token()
				if e != nil {
					return nil, errors.New("JSON_SYNTAX")
				}
				k, ok := kt.(string)
				if !ok {
					return nil, errors.New("JSON_SYNTAX")
				}
				if _, exists := m[k]; exists {
					return nil, errors.New("DUPLICATE_FIELD")
				}
				v, e := parseValue(d, nil)
				if e != nil {
					return nil, e
				}
				m[k] = v
			}
			if _, e = d.Token(); e != nil {
				return nil, errors.New("JSON_SYNTAX")
			}
			return m, nil
		}
		if x == '[' {
			var a []any
			for d.More() {
				v, e := parseValue(d, nil)
				if e != nil {
					return nil, e
				}
				a = append(a, v)
			}
			if _, e = d.Token(); e != nil {
				return nil, errors.New("JSON_SYNTAX")
			}
			return a, nil
		}
		return nil, errors.New("JSON_SYNTAX")
	case json.Number:
		s := string(x)
		if strings.ContainsAny(s, ".eE") || s == "-0" || strings.HasPrefix(s, "-") || (len(s) > 1 && s[0] == '0') {
			return nil, errors.New("JSON_SYNTAX")
		}
		var n uint64
		if _, e := fmtSscan(s, &n); e != nil || n > math.MaxInt64 {
			return nil, errors.New("JSON_SYNTAX")
		}
		return n, nil
	default:
		return x, nil
	}
}
func fmtSscan(s string, n *uint64) (int, error) {
	var v uint64
	if s == "" {
		return 0, errors.New("bad")
	}
	for _, c := range s {
		if c < '0' || c > '9' || v > (math.MaxUint64-uint64(c-'0'))/10 {
			return 0, errors.New("bad")
		}
		v = v*10 + uint64(c-'0')
	}
	*n = v
	return 1, nil
}
func hasUnknownRequestField(o map[string]any) bool {
	allowed := func(m map[string]any, keys ...string) bool {
		a := map[string]bool{}
		for _, k := range keys {
			a[k] = true
		}
		for k := range m {
			if !a[k] {
				return true
			}
		}
		return false
	}
	if allowed(o, "schema", "id", "relation", "selector", "admissionDigest", "policyDigest", "limitsDigest", "topK", "members") {
		return true
	}
	if s, ok := o["selector"].(map[string]any); ok {
		kind, _ := s["kind"].(string)
		if kind == "RANGE_UNION" {
			if allowed(s, "kind", "union") {
				return true
			}
		} else if kind == "PATH_PREFIX" {
			if allowed(s, "kind", "path", "frozenPaths") {
				return true
			}
		} else if allowed(s, "kind", "path") {
			return true
		}
	}
	if ms, ok := o["members"].([]any); ok {
		for _, x := range ms {
			if m, ok := x.(map[string]any); ok && allowed(m, "id", "path", "revision", "fileDigest", "objectDigest", "ranges", "available", "policyAllowed", "score") {
				return true
			}
		}
	}
	return false
}
func validRequest(o map[string]any, q *Request) bool {
	req := []string{"schema", "id", "relation", "selector", "admissionDigest", "policyDigest", "limitsDigest", "topK", "members"}
	if len(o) != len(req) {
		return false
	}
	for _, k := range req {
		if _, ok := o[k]; !ok {
			return false
		}
	}
	if q.Schema != RequestSchema || q.ID == "" || (q.Relation != "INTERSECTS" && q.Relation != "CONTAINED_BY" && q.Relation != "CONTAINS") || !digest(q.AdmissionDigest) || !digest(q.PolicyDigest) || !digest(q.LimitsDigest) {
		return false
	}
	return true
}
func digest(s string) bool {
	if len(s) != 71 || !strings.HasPrefix(s, "sha256:") {
		return false
	}
	_, e := hex.DecodeString(s[7:])
	return e == nil
}
func parseEnvelope(raw []byte, w *work, l Limits) (envelope, string, string) {
	if len(raw) == 0 {
		return envelope{}, "SOURCE_ADMISSION_UNAVAILABLE", "BINDING_UNAVAILABLE"
	}
	var top map[string]json.RawMessage
	if json.Unmarshal(raw, &top) != nil {
		return envelope{}, "SOURCE_ADMISSION_MISMATCH", "BINDING_SCHEMA"
	}
	var schema, out string
	if json.Unmarshal(top["schema"], &schema) != nil || schema != EnvelopeSchema || json.Unmarshal(top["outcome"], &out) != nil {
		return envelope{}, "SOURCE_ADMISSION_MISMATCH", "BINDING_SCHEMA"
	}
	e := envelope{Outcome: out}
	var arr []json.RawMessage
	if out == "COMPLETE" {
		if len(top) != 3 {
			return e, "SOURCE_ADMISSION_MISMATCH", "BINDING_SCHEMA"
		}
		var b map[string]json.RawMessage
		if json.Unmarshal(top["binding"], &b) != nil || len(b) != 3 {
			return e, "SOURCE_ADMISSION_MISMATCH", "BINDING_SCHEMA"
		}
		var bs string
		if json.Unmarshal(b["schema"], &bs) != nil || bs != src.Schema || json.Unmarshal(b["admissionDigest"], &e.AdmissionDigest) != nil || json.Unmarshal(b["sources"], &arr) != nil {
			return e, "SOURCE_ADMISSION_MISMATCH", "BINDING_SCHEMA"
		}
	} else {
		if json.Unmarshal(top["detail"], &e.Detail) != nil || json.Unmarshal(top["input"], &arr) != nil {
			return e, "SOURCE_ADMISSION_MISMATCH", "BINDING_SCHEMA"
		}
	}
	for _, rr := range arr {
		var m map[string]json.RawMessage
		if json.Unmarshal(rr, &m) != nil || len(m) != 5 {
			return e, "SOURCE_ADMISSION_MISMATCH", "BINDING_INVALID_SOURCE"
		}
		var s sourceWire
		for k, p := range map[string]*string{"path": &s.Path, "revision": &s.Revision, "fileDigest": &s.FileDigest, "objectDigest": &s.ObjectDigest, "bytes": &s.Bytes} {
			if json.Unmarshal(m[k], p) != nil {
				return e, "SOURCE_ADMISSION_MISMATCH", "BINDING_INVALID_SOURCE"
			}
		}
		bb, er := base64.StdEncoding.Strict().DecodeString(s.Bytes)
		if er != nil || base64.StdEncoding.EncodeToString(bb) != s.Bytes || len(bb) == 0 || !utf8.Valid(bb) || !src.CanonicalPath(s.Path) || s.Revision == "" || !digest(s.FileDigest) || !digest(s.ObjectDigest) || src.Digest(bb) != s.FileDigest || src.Digest(bb) != s.ObjectDigest {
			return e, "SOURCE_ADMISSION_MISMATCH", "BINDING_INVALID_SOURCE"
		}
		if !w.add(7, 1) {
			return e, "RESOURCE_LIMIT", "WORK"
		}
		e.Sources = append(e.Sources, s)
	}
	if uint64(len(e.Sources)) > l.MaxSources {
		return e, "RESOURCE_LIMIT", "SOURCES"
	}
	var total uint64
	ins := make([]src.SelectedSource, len(e.Sources))
	for i, s := range e.Sources {
		bb, _ := base64.StdEncoding.DecodeString(s.Bytes)
		n := uint64(len(bb))
		if n > l.MaxSourceBytes || total > math.MaxUint64-n || total+n > l.MaxTotalSourceBytes {
			return e, "RESOURCE_LIMIT", "SOURCE_BYTES"
		}
		if !w.add(1, n) {
			return e, "RESOURCE_LIMIT", "WORK"
		}
		total += n
		ins[i] = src.SelectedSource{Path: s.Path, Revision: s.Revision, FileDigest: s.FileDigest, ObjectDigest: s.ObjectDigest, Bytes: bb}
		if out == "COMPLETE" && i > 0 && e.Sources[i-1].Path >= s.Path {
			return e, "SOURCE_ADMISSION_MISMATCH", "BINDING_INVALID_SOURCE"
		}
	}
	if out != "COMPLETE" {
		a := src.Admit(ins, src.Limits{MaxSources: int(l.MaxSources), MaxSourceBytes: int(l.MaxSourceBytes), MaxTotalBytes: int(l.MaxTotalSourceBytes)})
		if out == "DUPLICATE_SOURCE" && a.Outcome == src.DuplicateSource {
			return e, "SOURCE_ADMISSION_MISMATCH", "BINDING_DUPLICATE_SOURCE"
		}
		if out == "INVALID_SOURCE" && a.Outcome == src.InvalidSource {
			return e, "SOURCE_ADMISSION_MISMATCH", "BINDING_INVALID_SOURCE"
		}
		if out == "INVALID_REQUEST" && a.Outcome == src.InvalidRequest {
			return e, "SOURCE_ADMISSION_MISMATCH", "BINDING_INVALID_REQUEST"
		}
		if out == "RESOURCE_LIMIT" && a.Outcome == src.ResourceLimit {
			if e.Detail == "SOURCES" {
				return e, "RESOURCE_LIMIT", "SOURCES"
			}
			return e, "RESOURCE_LIMIT", "SOURCE_BYTES"
		}
		return e, "SOURCE_ADMISSION_MISMATCH", "BINDING_INVALID_REQUEST"
	}
	a := src.Admit(ins, src.Limits{MaxSources: int(l.MaxSources), MaxSourceBytes: int(l.MaxSourceBytes), MaxTotalBytes: int(l.MaxTotalSourceBytes)})
	if a.Outcome != src.Complete || a.Binding.AdmissionDigest != e.AdmissionDigest {
		return e, "SOURCE_ADMISSION_MISMATCH", "BINDING_DIGEST"
	}
	cp, _ := choose2(uint64(len(ins)))
	if !w.add(29, cp) {
		return e, "RESOURCE_LIMIT", "WORK"
	}
	return e, "", ""
}
func mapFailure(o, d string) string {
	if o == "DUPLICATE_SOURCE" {
		return "BINDING_DUPLICATE_SOURCE"
	}
	if o == "INVALID_SOURCE" {
		return "BINDING_INVALID_SOURCE"
	}
	return "BINDING_INVALID_REQUEST"
}
func selectorRanges(s Selector, sm map[string]src.SelectedSource, w *work, l Limits) (map[string][]Range, string, string) {
	out := map[string][]Range{}
	switch s.Kind {
	case "EXACT_FILE":
		if !w.add(7, 1) {
			return nil, "RESOURCE_LIMIT", "WORK"
		}
		x, ok := sm[s.Path]
		if !src.CanonicalPath(s.Path) || !ok {
			return nil, "INVALID_SELECTOR", "PATH"
		}
		out[s.Path] = []Range{whole(x.Bytes)}
	case "RANGE_UNION":
		if uint64(len(s.Union)) > l.MaxSelectorPaths {
			return nil, "RESOURCE_LIMIT", "SELECTOR_PATHS"
		}
		var total uint64
		for _, u := range s.Union {
			if !w.add(7, 1) {
				return nil, "RESOURCE_LIMIT", "WORK"
			}
			x, ok := sm[u.Path]
			if !src.CanonicalPath(u.Path) || !ok {
				return nil, "INVALID_SELECTOR", "PATH"
			}
			if uint64(len(u.Ranges)) > l.MaxRangesPerPath {
				return nil, "RESOURCE_LIMIT", "RANGES_PER_PATH"
			}
			total += uint64(len(u.Ranges))
			if total > l.MaxTotalRanges {
				return nil, "RESOURCE_LIMIT", "TOTAL_RANGES"
			}
			for _, r := range u.Ranges {
				if !w.add(11, 1) {
					return nil, "RESOURCE_LIMIT", "WORK"
				}
				if z := rangeStatus(r, x.Bytes); z != "" {
					return nil, "INVALID_RANGE", z
				}
				out[u.Path] = append(out[u.Path], r)
			}
		}
		cp, _ := choose2(uint64(len(s.Union)))
		w.add(29, cp)
		for _, u := range s.Union {
			cp, _ = choose2(uint64(len(u.Ranges)))
			w.add(29, cp)
		}
		for p := range out {
			sortRanges(out[p])
			out[p] = dedupRanges(out[p])
		}
	case "PATH_PREFIX":
		if !w.add(7, 1) || !src.CanonicalPath(s.Path) {
			return nil, "INVALID_SELECTOR", "PATH"
		}
		for _, p := range s.FrozenPaths {
			if !w.add(7, 1) || !src.CanonicalPath(p) {
				return nil, "INVALID_SELECTOR", "FROZEN_EXPANSION"
			}
		}
		if uint64(len(s.FrozenPaths)) > l.MaxFrozenPaths {
			return nil, "RESOURCE_LIMIT", "FROZEN_PATHS"
		}
		var got []string
		for p := range sm {
			if p == s.Path || strings.HasPrefix(p, s.Path+"/") {
				got = append(got, p)
			}
		}
		sort.Strings(got)
		if uint64(len(got)) > l.MaxSelectorPaths {
			return nil, "RESOURCE_LIMIT", "SELECTOR_PATHS"
		}
		if !equalStrings(got, s.FrozenPaths) {
			return nil, "INVALID_SELECTOR", "FROZEN_EXPANSION"
		}
		for _, p := range got {
			out[p] = []Range{whole(sm[p].Bytes)}
		}
		cp, _ := choose2(uint64(len(s.FrozenPaths)))
		w.add(29, cp)
	default:
		return nil, "INVALID_REQUEST", "REQUEST_FIELD"
	}
	return out, "", ""
}
func whole(b []byte) Range {
	ls := lines(b)
	return Range{Position{0, 0}, Position{uint64(len(ls) - 1), utf16Units(ls[len(ls)-1])}}
}
func utf16Units(rs []rune) uint64 {
	var n uint64
	for _, r := range rs {
		n++
		if r > 0xffff {
			n++
		}
	}
	return n
}
func lines(b []byte) [][]rune {
	s := string(b)
	parts := strings.Split(s, "\n")
	out := make([][]rune, len(parts))
	for i, p := range parts {
		p = strings.TrimSuffix(p, "\r")
		out[i] = []rune(p)
	}
	return out
}
func rangeStatus(r Range, b []byte) string {
	ls := lines(b)
	valid := func(p Position) bool {
		if p.Line >= uint64(len(ls)) {
			return false
		}
		units := uint64(0)
		for _, x := range ls[p.Line] {
			n := uint64(1)
			if x > 0xffff {
				n = 2
			}
			if p.Character > units && p.Character < units+n {
				return false
			}
			units += n
		}
		return p.Character <= units
	}
	if !valid(r.Start) || !valid(r.End) {
		return "POSITION"
	}
	if cmpPos(r.Start, r.End) >= 0 {
		return "MEMBER_RANGE"
	}
	return ""
}
func relation(k string, a, b Range) bool {
	switch k {
	case "INTERSECTS":
		return cmpPos(maxPos(a.Start, b.Start), minPos(a.End, b.End)) < 0
	case "CONTAINED_BY":
		return cmpPos(a.Start, b.Start) <= 0 && cmpPos(b.End, a.End) <= 0
	case "CONTAINS":
		return cmpPos(b.Start, a.Start) <= 0 && cmpPos(a.End, b.End) <= 0
	}
	return false
}
func cmpPos(a, b Position) int {
	if a.Line < b.Line {
		return -1
	}
	if a.Line > b.Line {
		return 1
	}
	if a.Character < b.Character {
		return -1
	}
	if a.Character > b.Character {
		return 1
	}
	return 0
}
func maxPos(a, b Position) Position {
	if cmpPos(a, b) >= 0 {
		return a
	}
	return b
}
func minPos(a, b Position) Position {
	if cmpPos(a, b) <= 0 {
		return a
	}
	return b
}
func sortRanges(a []Range) {
	sort.Slice(a, func(i, j int) bool {
		if c := cmpPos(a[i].Start, a[j].Start); c != 0 {
			return c < 0
		}
		return cmpPos(a[i].End, a[j].End) < 0
	})
}
func dedupRanges(a []Range) []Range {
	n := 0
	for _, x := range a {
		if n == 0 || x != a[n-1] {
			a[n] = x
			n++
		}
	}
	return a[:n]
}
func sortWitnesses(a []Witness) {
	sort.Slice(a, func(i, j int) bool {
		bi, _ := json.Marshal(a[i])
		bj, _ := json.Marshal(a[j])
		return bytes.Compare(bi, bj) < 0
	})
}
func dedupWitnesses(a []Witness) []Witness {
	n := 0
	for _, x := range a {
		if n == 0 || x != a[n-1] {
			a[n] = x
			n++
		}
	}
	return a[:n]
}
func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
func SHA256(b []byte) string { s := sha256.Sum256(b); return "sha256:" + hex.EncodeToString(s[:]) }
