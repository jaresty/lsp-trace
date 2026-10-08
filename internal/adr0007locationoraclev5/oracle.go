// Package adr0007locationoraclev5 is an independent clean-room oracle for the
// prospective ADR0007 Location v5 contract. It depends only on the published
// prospective specification and the pinned source-admission v2 implementation.
package adr0007locationoraclev5

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"sort"
	"strings"
	"unicode/utf16"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
	"lsp-trace/internal/sourceadmissionv2"
)

const (
	RequestSchema  = "lsp-trace.adr0007.location-intersection.request.private.v5"
	ResultSchema   = "lsp-trace.adr0007.location-intersection.result.private.v5"
	EnvelopeSchema = "lsp-trace.adr0007.source-admission-envelope.private.v5"
	PolicyDigest   = "sha256:ba2dd40c552f9492be66b67b760f366d5c80af292278a33510f97dd2a1b7972e"
	LimitsDigest   = "sha256:55553e121b51ad8f3d4c491ffb5f2635cbc608131d721afcdb0423046625a8d3"
)

type Limits struct {
	MaxRequestBytes, MaxMembers, MaxSelectorPaths, MaxRangesPerPath, MaxTotalRanges, MaxFrozenPaths, MaxWitnesses, MaxTopK, MaxSources, MaxSourceBytes, MaxTotalSourceBytes, MaxOutputBytes int
	MaxWork                                                                                                                                                                                 uint64
}

func PublishedLimits() Limits {
	return Limits{1048576, 10000, 1000, 10000, 100000, 1000, 10000, 10000, 1000, 1048576, 8388608, 8388608, 50000000}
}

type Condition struct {
	Schema          string `json:"schema"`
	Cancel          bool   `json:"cancel"`
	DeadlineExpired bool   `json:"deadlineExpired"`
	LimitsProfile   string `json:"limitsProfile"`
}
type Position struct {
	Line      int64 `json:"line"`
	Character int64 `json:"character"`
}
type Range struct {
	Start Position `json:"start"`
	End   Position `json:"end"`
}
type UnionEntry struct {
	Path   string  `json:"path"`
	Ranges []Range `json:"ranges"`
}
type Selector struct {
	Kind        string       `json:"kind"`
	Path        string       `json:"path,omitempty"`
	FrozenPaths []string     `json:"frozenPaths,omitempty"`
	Union       []UnionEntry `json:"union,omitempty"`
}
type Member struct {
	ID            string  `json:"id"`
	Path          string  `json:"path"`
	Revision      string  `json:"revision"`
	FileDigest    string  `json:"fileDigest"`
	ObjectDigest  string  `json:"objectDigest"`
	Ranges        []Range `json:"ranges"`
	Available     bool    `json:"available"`
	PolicyAllowed bool    `json:"policyAllowed"`
	Score         int64   `json:"score"`
}
type Request struct {
	Schema          string   `json:"schema"`
	ID              string   `json:"id"`
	Relation        string   `json:"relation"`
	Selector        Selector `json:"selector"`
	AdmissionDigest string   `json:"admissionDigest"`
	PolicyDigest    string   `json:"policyDigest"`
	LimitsDigest    string   `json:"limitsDigest"`
	TopK            int64    `json:"topK"`
	Members         []Member `json:"members"`
}
type sourceWire struct {
	Path         string `json:"path"`
	Revision     string `json:"revision"`
	FileDigest   string `json:"fileDigest"`
	ObjectDigest string `json:"objectDigest"`
	Bytes        string `json:"bytes"`
}
type bindingWire struct {
	Schema          string       `json:"schema"`
	AdmissionDigest string       `json:"admissionDigest"`
	Sources         []sourceWire `json:"sources"`
}
type envelopeWire struct {
	Schema        string       `json:"schema"`
	Outcome       string       `json:"outcome"`
	Binding       *bindingWire `json:"binding,omitempty"`
	Detail        string       `json:"detail,omitempty"`
	DuplicatePath string       `json:"duplicatePath,omitempty"`
	Input         []sourceWire `json:"input,omitempty"`
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
	Ordinal   int       `json:"ordinal"`
	MemberID  string    `json:"memberId"`
	Outcome   string    `json:"outcome"`
	Witnesses []Witness `json:"witnesses"`
}
type Ranked struct {
	Ordinal  int    `json:"ordinal"`
	MemberID string `json:"memberId"`
	Score    int64  `json:"score"`
}
type Counters struct {
	Input               int    `json:"input"`
	Eligible            int    `json:"eligible"`
	Ineligible          int    `json:"ineligible"`
	UnavailableLocation int    `json:"unavailableLocation"`
	InvalidLocation     int    `json:"invalidLocation"`
	DuplicateMember     int    `json:"duplicateMember"`
	FilteredByPolicy    int    `json:"filteredByPolicy"`
	Ranked              int    `json:"ranked"`
	Witnesses           int    `json:"witnesses"`
	Work                uint64 `json:"work"`
	SourceBytes         int    `json:"sourceBytes"`
	OutputBytes         int    `json:"outputBytes"`
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
type populations struct{ J, P, R, M, S, Q, X, C uint64 }

type workStageObserver func(stage, event string, before, charge, after, limit uint64)

var observeWorkStage workStageObserver

func failure(id, outcome, detail string) Result {
	return Result{ResultSchema, id, outcome, []MemberRow{}, []Ranked{}, Counters{}, detail}
}
func Canonical(r Result) []byte { b, _ := json.Marshal(r); return append(b, '\n') }
func choose2(n int) uint64 {
	if n < 2 {
		return 0
	}
	return uint64(n * (n - 1) / 2)
}

type workMeter struct {
	limits Limits
	p      *populations
	work   uint64
}

func newWorkMeter(l Limits, p *populations) *workMeter { return &workMeter{limits: l, p: p, work: 50} }
func (w *workMeter) inc(k string, n uint64) (string, string) {
	coeff := uint64(0)
	slot := (*uint64)(nil)
	switch k {
	case "J":
		coeff, slot = 3, &w.p.J
	case "P":
		coeff, slot = 7, &w.p.P
	case "R":
		coeff, slot = 11, &w.p.R
	case "M":
		coeff, slot = 13, &w.p.M
	case "S":
		coeff, slot = 1, &w.p.S
	case "Q":
		coeff, slot = 19, &w.p.Q
	case "X":
		coeff, slot = 23, &w.p.X
	case "C":
		coeff, slot = 29, &w.p.C
	case "B":
		coeff = 31
	default:
		panic("unknown population " + k)
	}
	if coeff != 0 && n > math.MaxUint64/coeff {
		if observeWorkStage != nil {
			observeWorkStage(k, "fail", w.work, math.MaxUint64, w.work, w.limits.MaxWork)
		}
		return "RESOURCE_LIMIT", "WORK"
	}
	charge := n * coeff
	before := w.work
	if observeWorkStage != nil {
		observeWorkStage(k, "before", before, charge, before, w.limits.MaxWork)
	}
	if w.work > math.MaxUint64-charge || w.work+charge > w.limits.MaxWork {
		if observeWorkStage != nil {
			observeWorkStage(k, "fail", before, charge, before, w.limits.MaxWork)
		}
		return "RESOURCE_LIMIT", "WORK"
	}
	if observeWorkStage != nil {
		observeWorkStage(k, "increment", before, charge, before+charge, w.limits.MaxWork)
	}
	w.work += charge
	if slot != nil {
		if *slot > math.MaxUint64-n {
			if observeWorkStage != nil {
				observeWorkStage(k, "fail", before, charge, w.work, w.limits.MaxWork)
			}
			return "RESOURCE_LIMIT", "WORK"
		}
		*slot += n
	}
	if observeWorkStage != nil {
		observeWorkStage(k, "after", before, charge, w.work, w.limits.MaxWork)
	}
	return "", ""
}
func (w *workMeter) addChoose2(n int) (string, string) { return w.inc("C", choose2(n)) }

func failForPoll(id string, err error) Result {
	if err != nil && err.Error() == "cancel" {
		return failure(id, "CANCELLED", "CANCEL_SIGNAL")
	}
	return failure(id, "TIMEOUT", "DEADLINE")
}

func Evaluate(raw, bindingRaw []byte, bindingPresent bool, cond Condition, limits Limits) Result {
	poll := func() error {
		if cond.Cancel {
			return errors.New("cancel")
		}
		if cond.DeadlineExpired {
			return errors.New("deadline")
		}
		return nil
	}
	if e := poll(); e != nil {
		return failForPoll("", e)
	}
	if len(raw) > limits.MaxRequestBytes {
		return failure("", "INVALID_REQUEST", "REQUEST_FIELD")
	}
	p := populations{}
	meter := newWorkMeter(limits, &p)
	if out, detail := meter.inc("J", uint64(len(raw))); detail != "" {
		return failure("", out, detail)
	}
	var req Request
	id, detail := decodeRequest(raw, &req)
	if detail != "" {
		return failure(id, "INVALID_REQUEST", detail)
	}
	if e := poll(); e != nil {
		return failForPoll(id, e)
	}
	if len(req.Members) > limits.MaxMembers {
		return failure(id, "RESOURCE_LIMIT", "MEMBERS")
	}
	if req.TopK > int64(limits.MaxTopK) {
		return failure(id, "RESOURCE_LIMIT", "TOP_K")
	}
	for _, m := range req.Members {
		if e := poll(); e != nil {
			return failForPoll(id, e)
		}
		if out, detail := meter.inc("M", 1); detail != "" {
			return failure(id, out, detail)
		}
		if m.ID == "" || !norm.NFC.IsNormalString(m.ID) {
			return failure(id, "INVALID_REQUEST", "REQUEST_FIELD")
		}
	}
	if !bindingPresent {
		return failure(id, "SOURCE_ADMISSION_UNAVAILABLE", "BINDING_UNAVAILABLE")
	}
	env, sources, detail, outcome := decodeEnvelope(bindingRaw, limits, meter)
	if detail != "" {
		return failure(id, outcome, detail)
	}
	if env.Outcome != "COMPLETE" {
		return failure(id, mapTypedOutcome(env.Outcome, env.Detail), mapTypedDetail(env.Outcome, env.Detail))
	}
	admitted := make([]sourceadmissionv2.SelectedSource, len(sources))
	total := 0
	for i, s := range sources {
		if len(s.Bytes) > limits.MaxSourceBytes || total > limits.MaxTotalSourceBytes-len(s.Bytes) {
			return failure(id, "RESOURCE_LIMIT", "SOURCE_BYTES")
		}
		total += len(s.Bytes)
		if out, detail := meter.inc("S", uint64(len(s.Bytes))); detail != "" {
			return failure(id, out, detail)
		}
		admitted[i] = s
	}
	if len(admitted) > limits.MaxSources {
		return failure(id, "RESOURCE_LIMIT", "SOURCES")
	}
	for i := 1; i < len(admitted); i++ {
		if admitted[i-1].Path >= admitted[i].Path {
			return failure(id, "SOURCE_ADMISSION_MISMATCH", "BINDING_INVALID_SOURCE")
		}
	}
	ar := sourceadmissionv2.Admit(admitted, sourceadmissionv2.Limits{MaxSources: limits.MaxSources, MaxSourceBytes: limits.MaxSourceBytes, MaxTotalBytes: limits.MaxTotalSourceBytes})
	if out, detail := meter.addChoose2(len(admitted)); detail != "" {
		return failure(id, out, detail)
	}
	if ar.Outcome != sourceadmissionv2.Complete {
		return failure(id, "SOURCE_ADMISSION_MISMATCH", mapAdmission(ar.Outcome))
	}
	if ar.Binding.AdmissionDigest != env.Binding.AdmissionDigest || req.AdmissionDigest != ar.Binding.AdmissionDigest {
		return failure(id, "SOURCE_ADMISSION_MISMATCH", "BINDING_DIGEST")
	}
	if req.PolicyDigest != PolicyDigest {
		return failure(id, "POLICY_MISMATCH", "POLICY_DIGEST")
	}
	if req.LimitsDigest != LimitsDigest {
		return failure(id, "POLICY_MISMATCH", "LIMITS_DIGEST")
	}
	byPath := map[string]sourceadmissionv2.SelectedSource{}
	for _, s := range admitted {
		byPath[s.Path] = s
	}
	selectors, failOut, failDetail := selectorRanges(req.Selector, byPath, limits, meter)
	if failDetail != "" {
		return failure(id, failOut, failDetail)
	}
	needed := map[string]bool{}
	for path := range selectors {
		needed[path] = true
	}
	seenOwner := map[string]bool{}
	for _, m := range req.Members {
		if !seenOwner[m.ID] {
			seenOwner[m.ID] = true
			if m.Available {
				if _, ok := byPath[m.Path]; ok {
					needed[m.Path] = true
				}
			}
		}
	}
	sourceBytes := 0
	for path := range needed {
		sourceBytes += len(byPath[path].Bytes)
	}
	if sourceBytes > limits.MaxTotalSourceBytes {
		return failure(id, "RESOURCE_LIMIT", "SOURCE_BYTES")
	}
	rows := make([]MemberRow, 0, len(req.Members))
	eligibleRanks := []Ranked{}
	counts := Counters{Input: len(req.Members), SourceBytes: sourceBytes}
	seen := map[string]bool{}
	for i, m := range req.Members {
		if e := poll(); e != nil {
			return failForPoll(id, e)
		}
		row := MemberRow{i, m.ID, "", []Witness{}}
		if seen[m.ID] {
			row.Outcome = "DUPLICATE_MEMBER"
			counts.DuplicateMember++
		} else {
			seen[m.ID] = true
			if !m.Available {
				row.Outcome = "UNAVAILABLE_LOCATION"
				counts.UnavailableLocation++
			} else {
				if out, detail := meter.inc("P", 1); detail != "" {
					return failure(id, out, detail)
				}
				s, ok := byPath[m.Path]
				if !ok || s.Revision != m.Revision || s.FileDigest != m.FileDigest || s.ObjectDigest != m.ObjectDigest {
					row.Outcome = "INVALID_LOCATION"
					counts.InvalidLocation++
				} else if !validRanges(m.Ranges, s.Bytes, meter) {
					row.Outcome = "INVALID_LOCATION"
					counts.InvalidLocation++
				} else if !m.PolicyAllowed {
					row.Outcome = "FILTERED_BY_POLICY"
					counts.FilteredByPolicy++
				} else {
					for _, sr := range selectors[m.Path] {
						for _, cr := range m.Ranges {
							if e := poll(); e != nil {
								return failForPoll(id, e)
							}
							if out, detail := meter.inc("Q", 1); detail != "" {
								return failure(id, out, detail)
							}
							if relation(req.Relation, sr, cr) {
								row.Witnesses = append(row.Witnesses, Witness{m.Path, sr, cr, intersection(sr, cr), s.Revision, s.FileDigest, s.ObjectDigest})
							}
						}
					}
					if out, detail := meter.addChoose2(len(row.Witnesses)); detail != "" {
						return failure(id, out, detail)
					}
					row.Witnesses = dedupWitnesses(row.Witnesses)
					if len(row.Witnesses) > 0 {
						row.Outcome = "ELIGIBLE"
						counts.Eligible++
						eligibleRanks = append(eligibleRanks, Ranked{i, m.ID, m.Score})
						counts.Witnesses += len(row.Witnesses)
					} else {
						row.Outcome = "INELIGIBLE"
						counts.Ineligible++
					}
				}
			}
		}
		rows = append(rows, row)
	}
	if counts.Witnesses > limits.MaxWitnesses {
		return failure(id, "RESOURCE_LIMIT", "WITNESSES")
	}
	for range counts.Witnesses {
		if e := poll(); e != nil {
			return failForPoll(id, e)
		}
		if out, detail := meter.inc("X", 1); detail != "" {
			return failure(id, out, detail)
		}
	}
	if out, detail := meter.addChoose2(len(eligibleRanks)); detail != "" {
		return failure(id, out, detail)
	}
	sort.SliceStable(eligibleRanks, func(i, j int) bool {
		if eligibleRanks[i].Score != eligibleRanks[j].Score {
			return eligibleRanks[i].Score > eligibleRanks[j].Score
		}
		if eligibleRanks[i].Ordinal != eligibleRanks[j].Ordinal {
			return eligibleRanks[i].Ordinal < eligibleRanks[j].Ordinal
		}
		return eligibleRanks[i].MemberID < eligibleRanks[j].MemberID
	})
	if int64(len(eligibleRanks)) > req.TopK {
		eligibleRanks = eligibleRanks[:req.TopK]
	}
	counts.Ranked = len(eligibleRanks)
	result := Result{ResultSchema, id, "COMPLETE", rows, eligibleRanks, counts, "NONE"}
	image := Canonical(result)
	B := uint64(len(image))
	for range B {
		if e := poll(); e != nil {
			return failForPoll(id, e)
		}
	}
	if out, detail := meter.inc("B", B); detail != "" {
		return failure(id, out, detail)
	}
	W := meter.work
	if int(B) > limits.MaxOutputBytes {
		return failure(id, "RESOURCE_LIMIT", "OUTPUT_BYTES")
	}
	result.Counters.Work = W
	result.Counters.OutputBytes = int(B)
	return result
}

func uniqueJSON(r io.Reader) error {
	dec := json.NewDecoder(r)
	if err := uniqueJSONValue(dec); err != nil {
		return err
	}
	if tok, err := dec.Token(); err == nil {
		return fmt.Errorf("trailing JSON after %v", tok)
	} else if err != io.EOF {
		return err
	}
	return nil
}

func uniqueJSONValue(dec *json.Decoder) error {
	tok, err := dec.Token()
	if err != nil {
		return err
	}
	delim, ok := tok.(json.Delim)
	if !ok {
		return nil
	}
	switch delim {
	case '{':
		seen := map[string]bool{}
		for dec.More() {
			key, err := dec.Token()
			if err != nil {
				return err
			}
			s, ok := key.(string)
			if !ok || seen[s] {
				return fmt.Errorf("duplicate/nonstring key")
			}
			seen[s] = true
			if err := uniqueJSONValue(dec); err != nil {
				return err
			}
		}
	case '[':
		for dec.More() {
			if err := uniqueJSONValue(dec); err != nil {
				return err
			}
		}
	default:
		return fmt.Errorf("unexpected delimiter")
	}
	_, err = dec.Token()
	return err
}

func strictRequestObject(raw []byte) (map[string]json.RawMessage, string, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	if tok, err := dec.Token(); err != nil || tok != json.Delim('{') {
		return nil, "", fmt.Errorf("request object")
	}
	fields := map[string]json.RawMessage{}
	id := ""
	for dec.More() {
		tok, err := dec.Token()
		if err != nil {
			return nil, id, err
		}
		key, ok := tok.(string)
		if !ok {
			return nil, id, fmt.Errorf("request key")
		}
		var raw json.RawMessage
		if err := dec.Decode(&raw); err != nil {
			return nil, id, err
		}
		if _, exists := fields[key]; exists {
			return nil, id, fmt.Errorf("duplicate request key")
		}
		if key == "id" {
			_ = json.Unmarshal(raw, &id)
		}
		fields[key] = raw
	}
	if tok, err := dec.Token(); err != nil || tok != json.Delim('}') {
		return nil, id, fmt.Errorf("request close")
	}
	if tok, err := dec.Token(); err == nil {
		return nil, id, fmt.Errorf("trailing request token %v", tok)
	} else if err != io.EOF {
		return nil, id, err
	}
	return fields, id, nil
}

func decodeRequest(raw []byte, out *Request) (string, string) {
	if !utf8.Valid(raw) || bytes.HasPrefix(raw, []byte{0xef, 0xbb, 0xbf}) {
		return "", "JSON_ENCODING"
	}
	generic, id, err := strictRequestObject(raw)
	if err != nil {
		return id, "JSON_SYNTAX"
	}
	allowed := map[string]bool{"schema": true, "id": true, "relation": true, "selector": true, "admissionDigest": true, "policyDigest": true, "limitsDigest": true, "topK": true, "members": true}
	ordered := []string{"schema", "id", "relation", "selector", "admissionDigest", "policyDigest", "limitsDigest", "topK", "members"}
	for _, k := range ordered {
		if _, ok := generic[k]; !ok {
			return id, "REQUEST_FIELD"
		}
	}
	for k := range generic {
		if !allowed[k] {
			return id, "UNKNOWN_FIELD"
		}
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(out); err != nil {
		return id, "REQUEST_FIELD"
	}
	if out.Schema != RequestSchema {
		return id, "SCHEMA_ID"
	}
	if out.ID == "" || (out.Relation != "INTERSECTS" && out.Relation != "CONTAINED_BY" && out.Relation != "CONTAINS") || out.TopK < 0 {
		return id, "REQUEST_FIELD"
	}
	return id, ""
}
func decodeEnvelope(raw []byte, l Limits, meter *workMeter) (envelopeWire, []sourceadmissionv2.SelectedSource, string, string) {
	var e envelopeWire
	if err := uniqueJSON(bytes.NewReader(raw)); err != nil {
		return e, nil, "BINDING_SCHEMA", "SOURCE_ADMISSION_MISMATCH"
	}
	var top map[string]json.RawMessage
	if json.Unmarshal(raw, &top) != nil {
		return e, nil, "BINDING_SCHEMA", "SOURCE_ADMISSION_MISMATCH"
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(&e); err != nil {
		return e, nil, "BINDING_SCHEMA", "SOURCE_ADMISSION_MISMATCH"
	}
	if e.Schema != EnvelopeSchema {
		return e, nil, "BINDING_SCHEMA", "SOURCE_ADMISSION_MISMATCH"
	}
	wire := e.Input
	var rawWire []json.RawMessage
	if e.Outcome == "COMPLETE" {
		if len(top) != 3 || e.Binding == nil || e.Binding.Schema != sourceadmissionv2.Schema || e.Detail != "" || e.DuplicatePath != "" || e.Input != nil {
			return e, nil, "BINDING_SCHEMA", "SOURCE_ADMISSION_MISMATCH"
		}
		var binding struct {
			Sources []json.RawMessage `json:"sources"`
		}
		if json.Unmarshal(top["binding"], &binding) != nil {
			return e, nil, "BINDING_SCHEMA", "SOURCE_ADMISSION_MISMATCH"
		}
		rawWire = binding.Sources
		wire = e.Binding.Sources
	} else {
		if _, ok := top["binding"]; ok || e.Binding != nil {
			return e, nil, "BINDING_SCHEMA", "SOURCE_ADMISSION_MISMATCH"
		}
		expected := map[string]bool{"schema": true, "outcome": true}
		switch e.Outcome {
		case "INVALID_REQUEST":
			expected["detail"], expected["input"] = true, true
			if e.Detail == "" || e.DuplicatePath != "" || e.Input == nil {
				return e, nil, "BINDING_SCHEMA", "SOURCE_ADMISSION_MISMATCH"
			}
		case "INVALID_SOURCE", "RESOURCE_LIMIT":
			expected["detail"], expected["input"] = true, true
			if e.Detail == "" || e.DuplicatePath != "" {
				return e, nil, "BINDING_SCHEMA", "SOURCE_ADMISSION_MISMATCH"
			}
		case "DUPLICATE_SOURCE":
			expected["detail"], expected["duplicatePath"], expected["input"] = true, true, true
			if e.Detail == "" || e.DuplicatePath == "" {
				return e, nil, "BINDING_SCHEMA", "SOURCE_ADMISSION_MISMATCH"
			}
		default:
			return e, nil, "BINDING_SCHEMA", "SOURCE_ADMISSION_MISMATCH"
		}
		if len(top) != len(expected) {
			return e, nil, "BINDING_SCHEMA", "SOURCE_ADMISSION_MISMATCH"
		}
		for k := range top {
			if !expected[k] {
				return e, nil, "BINDING_SCHEMA", "SOURCE_ADMISSION_MISMATCH"
			}
		}
		if expected["input"] {
			raw, ok := top["input"]
			if !ok || json.Unmarshal(raw, &rawWire) != nil || e.Input == nil {
				return e, nil, "BINDING_SCHEMA", "SOURCE_ADMISSION_MISMATCH"
			}
		}
	}
	out := make([]sourceadmissionv2.SelectedSource, len(wire))
	for i, s := range wire {
		if i >= len(rawWire) || !validSourceWire(rawWire[i], s) {
			return e, nil, "BINDING_INVALID_SOURCE", "SOURCE_ADMISSION_MISMATCH"
		}
		b, err := base64.StdEncoding.Strict().DecodeString(s.Bytes)
		if err != nil || base64.StdEncoding.EncodeToString(b) != s.Bytes || len(b) == 0 || !utf8.Valid(b) {
			return e, nil, "BINDING_INVALID_SOURCE", "SOURCE_ADMISSION_MISMATCH"
		}
		if outc, detail := meter.inc("P", 1); detail != "" {
			return e, nil, detail, outc
		}
		out[i] = sourceadmissionv2.SelectedSource{Path: s.Path, Revision: s.Revision, FileDigest: s.FileDigest, ObjectDigest: s.ObjectDigest, Bytes: b}
	}
	if len(wire) > l.MaxSources {
		return e, nil, "SOURCES", "RESOURCE_LIMIT"
	}
	return e, out, "", ""
}

func validSourceWire(raw json.RawMessage, s sourceWire) bool {
	var fields map[string]json.RawMessage
	if json.Unmarshal(raw, &fields) != nil || len(fields) != 5 {
		return false
	}
	for _, key := range []string{"path", "revision", "fileDigest", "objectDigest", "bytes"} {
		rawValue, ok := fields[key]
		if !ok {
			return false
		}
		var value string
		if json.Unmarshal(rawValue, &value) != nil || value == "" {
			return false
		}
	}
	return s.Path != "" && s.Revision != "" && s.FileDigest != "" && s.ObjectDigest != "" && s.Bytes != ""
}
func mapTypedOutcome(o, d string) string {
	if o == "RESOURCE_LIMIT" {
		return "RESOURCE_LIMIT"
	}
	return "SOURCE_ADMISSION_MISMATCH"
}
func mapTypedDetail(o, d string) string {
	switch o {
	case "INVALID_REQUEST":
		return "BINDING_INVALID_REQUEST"
	case "INVALID_SOURCE":
		return "BINDING_INVALID_SOURCE"
	case "DUPLICATE_SOURCE":
		return "BINDING_DUPLICATE_SOURCE"
	case "RESOURCE_LIMIT":
		if d == "SOURCES" {
			return "SOURCES"
		}
		return "SOURCE_BYTES"
	}
	return "BINDING_INVALID_REQUEST"
}
func mapAdmission(o sourceadmissionv2.Outcome) string {
	switch o {
	case sourceadmissionv2.InvalidRequest:
		return "BINDING_INVALID_REQUEST"
	case sourceadmissionv2.InvalidSource:
		return "BINDING_INVALID_SOURCE"
	case sourceadmissionv2.DuplicateSource:
		return "BINDING_DUPLICATE_SOURCE"
	}
	return "BINDING_INVALID_REQUEST"
}
func selectorRanges(s Selector, src map[string]sourceadmissionv2.SelectedSource, l Limits, meter *workMeter) (map[string][]Range, string, string) {
	out := map[string][]Range{}
	switch s.Kind {
	case "EXACT_FILE":
		if outc, detail := meter.inc("P", 1); detail != "" {
			return nil, outc, detail
		}
		x, ok := src[s.Path]
		if !ok {
			return nil, "INVALID_SELECTOR", "PATH"
		}
		out[s.Path] = []Range{whole(x.Bytes)}
	case "PATH_PREFIX":
		if outc, detail := meter.inc("P", 1); detail != "" {
			return nil, outc, detail
		}
		for range s.FrozenPaths {
			if outc, detail := meter.inc("P", 1); detail != "" {
				return nil, outc, detail
			}
		}
		if len(s.FrozenPaths) > l.MaxFrozenPaths {
			return nil, "RESOURCE_LIMIT", "FROZEN_PATHS"
		}
		var matches []string
		for x := range src {
			if x == s.Path || strings.HasPrefix(x, s.Path+"/") {
				matches = append(matches, x)
			}
		}
		sort.Strings(matches)
		if !equalStrings(matches, s.FrozenPaths) {
			return nil, "INVALID_SELECTOR", "FROZEN_EXPANSION"
		}
		if len(matches) > l.MaxSelectorPaths {
			return nil, "RESOURCE_LIMIT", "SELECTOR_PATHS"
		}
		for _, x := range matches {
			out[x] = []Range{whole(src[x].Bytes)}
		}
		if outc, detail := meter.addChoose2(len(s.FrozenPaths)); detail != "" {
			return nil, outc, detail
		}
	case "RANGE_UNION":
		if len(s.Union) > l.MaxSelectorPaths {
			return nil, "RESOURCE_LIMIT", "SELECTOR_PATHS"
		}
		total := 0
		for _, u := range s.Union {
			if outc, detail := meter.inc("P", 1); detail != "" {
				return nil, outc, detail
			}
			x, ok := src[u.Path]
			if !ok {
				return nil, "INVALID_SELECTOR", "PATH"
			}
			if len(u.Ranges) > l.MaxRangesPerPath {
				return nil, "RESOURCE_LIMIT", "RANGES_PER_PATH"
			}
			total += len(u.Ranges)
			if total > l.MaxTotalRanges {
				return nil, "RESOURCE_LIMIT", "TOTAL_RANGES"
			}
			if !validRanges(u.Ranges, x.Bytes, meter) {
				return nil, "INVALID_RANGE", "SELECTOR_RANGE"
			}
			out[u.Path] = append(out[u.Path], u.Ranges...)
			if outc, detail := meter.addChoose2(len(u.Ranges)); detail != "" {
				return nil, outc, detail
			}
		}
		if outc, detail := meter.addChoose2(len(s.Union)); detail != "" {
			return nil, outc, detail
		}
		for x := range out {
			sortRanges(out[x])
			out[x] = dedupRanges(out[x])
		}
	default:
		return nil, "INVALID_REQUEST", "REQUEST_FIELD"
	}
	return out, "", ""
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
func whole(b []byte) Range {
	lines := lineLengths(b)
	return Range{Position{0, 0}, Position{int64(len(lines) - 1), int64(lines[len(lines)-1])}}
}
func lineLengths(b []byte) []int {
	parts := bytes.Split(b, []byte{'\n'})
	out := make([]int, len(parts))
	for i, x := range parts {
		if i < len(parts)-1 && len(x) > 0 && x[len(x)-1] == '\r' {
			x = x[:len(x)-1]
		}
		out[i] = len(utf16.Encode([]rune(string(x))))
	}
	return out
}
func validRanges(rs []Range, b []byte, meter *workMeter) bool {
	if len(rs) == 0 {
		return false
	}
	ls := lineLengths(b)
	for _, r := range rs {
		if _, detail := meter.inc("R", 1); detail != "" {
			return false
		}
		if !validPos(r.Start, ls) || !validPos(r.End, ls) || cmp(r.Start, r.End) >= 0 {
			return false
		}
		if midSurrogate(r.Start, b) || midSurrogate(r.End, b) {
			return false
		}
	}
	return true
}
func validPos(x Position, ls []int) bool {
	return x.Line >= 0 && int(x.Line) < len(ls) && x.Character >= 0 && x.Character <= int64(ls[x.Line])
}
func midSurrogate(x Position, b []byte) bool {
	parts := bytes.Split(b, []byte{'\n'})
	line := parts[x.Line]
	if len(line) > 0 && line[len(line)-1] == '\r' {
		line = line[:len(line)-1]
	}
	n := 0
	for _, r := range string(line) {
		w := 1
		if r > 0xffff {
			w = 2
		}
		if int64(n) < x.Character && x.Character < int64(n+w) {
			return true
		}
		n += w
	}
	return false
}
func cmp(a, b Position) int {
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
func maxp(a, b Position) Position {
	if cmp(a, b) >= 0 {
		return a
	}
	return b
}
func minp(a, b Position) Position {
	if cmp(a, b) <= 0 {
		return a
	}
	return b
}
func relation(k string, s, c Range) bool {
	switch k {
	case "INTERSECTS":
		return cmp(maxp(s.Start, c.Start), minp(s.End, c.End)) < 0
	case "CONTAINED_BY":
		return cmp(s.Start, c.Start) <= 0 && cmp(c.End, s.End) <= 0
	case "CONTAINS":
		return cmp(c.Start, s.Start) <= 0 && cmp(s.End, c.End) <= 0
	}
	return false
}
func intersection(a, b Range) Range { return Range{maxp(a.Start, b.Start), minp(a.End, b.End)} }
func sortRanges(x []Range) {
	sort.Slice(x, func(i, j int) bool {
		if c := cmp(x[i].Start, x[j].Start); c != 0 {
			return c < 0
		}
		return cmp(x[i].End, x[j].End) < 0
	})
}
func dedupRanges(x []Range) []Range {
	if len(x) == 0 {
		return x
	}
	o := x[:1]
	for _, r := range x[1:] {
		if r != o[len(o)-1] {
			o = append(o, r)
		}
	}
	return o
}
func dedupWitnesses(x []Witness) []Witness {
	sort.Slice(x, func(i, j int) bool { return fmt.Sprint(x[i]) < fmt.Sprint(x[j]) })
	if len(x) == 0 {
		return x
	}
	o := x[:1]
	for _, w := range x[1:] {
		if w != o[len(o)-1] {
			o = append(o, w)
		}
	}
	return o
}
