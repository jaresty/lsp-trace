package adr0007v4contractvalidator

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"regexp"
	"sort"
	"strings"
	"unicode/utf16"
	"unicode/utf8"
)

const TerminalSchemaVersion = "lsp-trace.adr0007.source-text-search.terminal.private.v4"
const AccountingSchemaVersion = "lsp-trace.adr0007.source-text-search.accounting.private.v4"
const CustodySchemaVersion = "lsp-trace.adr0007.source-text-search.custody.private.v4"
const ReplaySchemaVersion = "lsp-trace.adr0007.source-text-search.replay.private.v4"
const CandidateOperation = "RANGE_UNION"
const zeroSHA = "sha256:0000000000000000000000000000000000000000000000000000000000000000"
const MaxIterations = 64

type Bundle struct {
	SchemaBytes              json.RawMessage   `json:"schema_bytes"`
	RawAttemptBytes          string            `json:"raw_attempt_bytes"`
	TerminalBytes            string            `json:"terminal_bytes"`
	AdmittedSourceBytes      map[string]string `json:"admitted_source_bytes"`
	AdmittedBindingBytes     string            `json:"admitted_binding_bytes"`
	ToolingManifestBytes     string            `json:"tooling_manifest_bytes"`
	PredecessorManifestBytes string            `json:"predecessor_manifest_bytes"`
	PayloadFreezeBytes       string            `json:"payload_freeze_binding_bytes"`
	WantErrorCode            string            `json:"want_error_code,omitempty"`
	WantErrorPath            string            `json:"want_error_path,omitempty"`
}

type VError struct{ Code, Path, Msg string }

func (e *VError) Error() string         { return e.Code + " " + e.Path + ": " + e.Msg }
func verr(code, path, msg string) error { return &VError{code, path, msg} }

type Terminal struct {
	SchemaVersion       string     `json:"schema_version"`
	Terminal            string     `json:"terminal"`
	Attempt             Attempt    `json:"attempt"`
	Request             Request    `json:"request"`
	Admission           Admission  `json:"admission"`
	Sources             []Source   `json:"sources"`
	Matches             []Match    `json:"matches"`
	Positions           []Position `json:"positions"`
	RangeUnionCandidate *Candidate `json:"range_union_candidate"`
	Accounting          Accounting `json:"accounting"`
	Failure             *Failure   `json:"failure"`
	Custody             Custody    `json:"custody"`
	Replay              Replay     `json:"replay"`
	Payload             Payload    `json:"payload"`
}
type Attempt struct {
	AttemptID    string `json:"attempt_id"`
	MalformedRaw bool   `json:"malformed_raw"`
}
type Request struct {
	Query string `json:"query"`
}
type Admission struct {
	Completed         bool     `json:"completed"`
	AdmissionDigest   string   `json:"admission_digest"`
	AdmittedSourceIDs []string `json:"admitted_source_ids"`
}
type Source struct {
	SourceID      string `json:"source_id"`
	LogicalURI    string `json:"logical_uri"`
	PathBytes     uint64 `json:"path_bytes"`
	ByteLength    uint64 `json:"byte_length"`
	ContentSHA256 string `json:"content_sha256"`
	Ordinal       uint64 `json:"ordinal"`
}
type Match struct {
	MatchID   string `json:"match_id"`
	SourceID  string `json:"source_id"`
	PathBytes uint64 `json:"path_bytes"`
	StartByte uint64 `json:"start_byte"`
	EndByte   uint64 `json:"end_byte"`
	Ordinal   uint64 `json:"ordinal"`
	Literal   string `json:"literal"`
}
type Position struct {
	MatchID             string `json:"match_id"`
	StartLine           uint64 `json:"start_line"`
	StartCharacterUTF16 uint64 `json:"start_character_utf16"`
	EndLine             uint64 `json:"end_line"`
	EndCharacterUTF16   uint64 `json:"end_character_utf16"`
}
type Candidate struct {
	Operation             string        `json:"operation"`
	ExecutedLocation      bool          `json:"executedLocation"`
	CandidateOnly         bool          `json:"candidate_only"`
	AdmissionDigest       string        `json:"admission_digest"`
	MemberMatchIDs        []string      `json:"member_match_ids"`
	QualifiedLocationPins []LocationPin `json:"qualified_location_pins"`
	CandidateDigest       string        `json:"candidate_digest"`
}
type LocationPin struct {
	SourceID            string `json:"source_id"`
	StartByte           uint64 `json:"start_byte"`
	EndByte             uint64 `json:"end_byte"`
	StartLine           uint64 `json:"start_line"`
	StartCharacterUTF16 uint64 `json:"start_character_utf16"`
	EndLine             uint64 `json:"end_line"`
	EndCharacterUTF16   uint64 `json:"end_character_utf16"`
}
type Accounting struct {
	SchemaVersion                                                                                                      string `json:"schema_version"`
	JFiles, QQueryBytes, PPathBytes, SSourceBytes, TScannedTuples, MMatches, RRanges, UUTF16Units, BOutputBytes, WWork uint64
	FailureCounters                                                                                                    map[string]uint64 `json:"failure_counters"`
}
type Failure struct {
	Code   string         `json:"code"`
	Stage  string         `json:"stage"`
	Detail map[string]any `json:"detail"`
}
type Custody struct {
	SchemaVersion        string `json:"schema_version"`
	AttemptID            string `json:"attempt_id"`
	TerminalResultSHA256 string `json:"terminal_result_sha256"`
	TerminalSequence0    uint64 `json:"terminal_sequence0"`
	TerminalCount1       uint64 `json:"terminal_count1"`
}
type Replay struct {
	SchemaVersion          string `json:"schema_version"`
	CanonicalAttemptSHA256 string `json:"canonical_attempt_sha256"`
	AdmittedBindingSHA256  string `json:"admitted_binding_sha256"`
	TerminalPreimageSHA256 string `json:"terminal_preimage_sha256"`
	ToolingIdentitySHA256  string `json:"tooling_identity_sha256"`
	FreezeBindingSHA256    string `json:"freeze_binding_sha256"`
	PredecessorLockSHA256  string `json:"predecessor_lock_sha256"`
}
type Payload struct {
	PayloadDigest       string `json:"payload_digest"`
	FreezeBindingSHA256 string `json:"freeze_binding_sha256"`
}

var failureNames = []string{"INVALID_INPUT", "CANCELLED", "DEADLINE_EXCEEDED", "ASSOCIATION_FAILED", "ADMISSION_FAILED", "RESOURCE_EXHAUSTED", "OVERFLOW", "INVARIANT_FAILED"}

func (a Accounting) MarshalJSON() ([]byte, error) {
	type raw struct {
		SchemaVersion   string            `json:"schema_version"`
		JFiles          uint64            `json:"J_files"`
		QQueryBytes     uint64            `json:"Q_query_bytes"`
		PPathBytes      uint64            `json:"P_path_bytes"`
		SSourceBytes    uint64            `json:"S_source_bytes"`
		TScannedTuples  uint64            `json:"T_scanned_tuples"`
		MMatches        uint64            `json:"M_matches"`
		RRanges         uint64            `json:"R_ranges"`
		UUTF16Units     uint64            `json:"U_utf16_units"`
		BOutputBytes    uint64            `json:"B_output_bytes"`
		WWork           uint64            `json:"W_work"`
		FailureCounters map[string]uint64 `json:"failure_counters"`
	}
	return json.Marshal(raw{a.SchemaVersion, a.JFiles, a.QQueryBytes, a.PPathBytes, a.SSourceBytes, a.TScannedTuples, a.MMatches, a.RRanges, a.UUTF16Units, a.BOutputBytes, a.WWork, a.FailureCounters})
}
func (a *Accounting) UnmarshalJSON(b []byte) error {
	type raw struct {
		SchemaVersion   string            `json:"schema_version"`
		JFiles          json.Number       `json:"J_files"`
		QQueryBytes     json.Number       `json:"Q_query_bytes"`
		PPathBytes      json.Number       `json:"P_path_bytes"`
		SSourceBytes    json.Number       `json:"S_source_bytes"`
		TScannedTuples  json.Number       `json:"T_scanned_tuples"`
		MMatches        json.Number       `json:"M_matches"`
		RRanges         json.Number       `json:"R_ranges"`
		UUTF16Units     json.Number       `json:"U_utf16_units"`
		BOutputBytes    json.Number       `json:"B_output_bytes"`
		WWork           json.Number       `json:"W_work"`
		FailureCounters map[string]uint64 `json:"failure_counters"`
	}
	var r raw
	if err := decodeExact(b, &r, []string{"schema_version", "J_files", "Q_query_bytes", "P_path_bytes", "S_source_bytes", "T_scanned_tuples", "M_matches", "R_ranges", "U_utf16_units", "B_output_bytes", "W_work", "failure_counters"}); err != nil {
		return err
	}
	vals := []*uint64{&a.JFiles, &a.QQueryBytes, &a.PPathBytes, &a.SSourceBytes, &a.TScannedTuples, &a.MMatches, &a.RRanges, &a.UUTF16Units, &a.BOutputBytes, &a.WWork}
	nums := []json.Number{r.JFiles, r.QQueryBytes, r.PPathBytes, r.SSourceBytes, r.TScannedTuples, r.MMatches, r.RRanges, r.UUTF16Units, r.BOutputBytes, r.WWork}
	for i, n := range nums {
		u, err := parseU64(n.String())
		if err != nil {
			return err
		}
		*vals[i] = u
	}
	a.SchemaVersion = r.SchemaVersion
	a.FailureCounters = r.FailureCounters
	return nil
}

func ValidateBundleFile(path string) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return ValidateBundleBytes(b)
}
func ValidateBundleBytes(b []byte) error {
	var bun Bundle
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	dec.DisallowUnknownFields()
	if err := dec.Decode(&bun); err != nil {
		return err
	}
	if err := ValidateBundle(bun); err != nil {
		return err
	}
	return nil
}
func ValidateFile(path string) error { return ValidateBundleFile(path) }

func ValidateBundle(b Bundle) error {
	if err := walkSchema(b.SchemaBytes); err != nil {
		return err
	}
	if !utf8.ValidString(b.TerminalBytes) {
		return verr("INVALID_INPUT", "/terminal_bytes", "invalid utf8")
	}
	var rawTerminal any
	rdec := json.NewDecoder(strings.NewReader(b.TerminalBytes))
	rdec.UseNumber()
	if err := rdec.Decode(&rawTerminal); err != nil {
		return verr("INVALID_INPUT", "/terminal_bytes", err.Error())
	}
	if err := validateSchemaValue(b.SchemaBytes, rawTerminal); err != nil {
		return err
	}
	var t Terminal
	dec := json.NewDecoder(strings.NewReader(b.TerminalBytes))
	dec.UseNumber()
	dec.DisallowUnknownFields()
	if err := dec.Decode(&t); err != nil {
		return verr("INVALID_INPUT", "/terminal_bytes", err.Error())
	}
	if dec.Decode(&struct{}{}) != io.EOF {
		return verr("INVALID_INPUT", "/terminal_bytes", "trailing data")
	}
	canon, err := CanonicalJSON(t)
	if err != nil {
		return err
	}
	if canon != b.TerminalBytes {
		return verr("INVALID_INPUT", "/terminal_bytes", "non-canonical")
	}
	if t.SchemaVersion != TerminalSchemaVersion {
		return verr("INVARIANT_FAILED", "/schema_version", "bad schema")
	}
	if t.Accounting.SchemaVersion != AccountingSchemaVersion {
		return verr("INVARIANT_FAILED", "/accounting/schema_version", "bad accounting schema")
	}
	if t.Custody.SchemaVersion != CustodySchemaVersion {
		return verr("INVARIANT_FAILED", "/custody/schema_version", "bad custody schema")
	}
	if t.Replay.SchemaVersion != ReplaySchemaVersion {
		return verr("INVARIANT_FAILED", "/replay/schema_version", "bad replay schema")
	}
	if got := sha(b.RawAttemptBytes); t.Replay.CanonicalAttemptSHA256 != got {
		return verr("INVARIANT_FAILED", "/replay/canonical_attempt_sha256", "raw attempt digest mismatch")
	}
	if t.Attempt.MalformedRaw && t.Attempt.AttemptID != "attempt-raw-sha256-"+strings.TrimPrefix(t.Replay.CanonicalAttemptSHA256, "sha256:") {
		return verr("INVARIANT_FAILED", "/attempt/attempt_id", "malformed raw digest identity")
	}
	if t.Custody.AttemptID != t.Attempt.AttemptID || t.Custody.TerminalSequence0 != 0 || t.Custody.TerminalCount1 != 1 {
		return verr("INVARIANT_FAILED", "/custody", "bad custody constants")
	}
	admissionDigest := digestOrEmpty(b.AdmittedBindingBytes)
	if t.Replay.AdmittedBindingSHA256 != admissionDigest {
		return verr("INVARIANT_FAILED", "/replay/admitted_binding_sha256", "admission digest mismatch")
	}
	if t.Admission.AdmissionDigest != admissionDigest {
		return verr("INVARIANT_FAILED", "/admission/admission_digest", "admission digest mismatch")
	}
	if t.Replay.ToolingIdentitySHA256 != sha(b.ToolingManifestBytes) || t.Replay.PredecessorLockSHA256 != sha(b.PredecessorManifestBytes) || t.Replay.FreezeBindingSHA256 != sha(b.PayloadFreezeBytes) || t.Payload.FreezeBindingSHA256 != sha(b.PayloadFreezeBytes) || t.Payload.PayloadDigest != sha(b.PayloadFreezeBytes) {
		return verr("INVARIANT_FAILED", "/replay", "manifest pin mismatch")
	}
	if err := validateFailure(t); err != nil {
		return err
	}
	if err := validateSourcesMatches(t, b); err != nil {
		return err
	}
	if err := validateCandidate(t); err != nil {
		return err
	}
	if err := validateAccounting(t, b.TerminalBytes); err != nil {
		return err
	}
	pre := normalizedTerminalPreimage(t)
	if t.Replay.TerminalPreimageSHA256 != sha(pre) || t.Custody.TerminalResultSHA256 != t.Replay.TerminalPreimageSHA256 {
		return verr("INVARIANT_FAILED", "/custody/terminal_result_sha256", "preimage digest mismatch")
	}
	return nil
}

func validateSchemaValue(schemaBytes []byte, value any) error {
	var schema any
	dec := json.NewDecoder(bytes.NewReader(schemaBytes))
	dec.UseNumber()
	if err := dec.Decode(&schema); err != nil {
		return verr("INVALID_INPUT", "/schema_bytes", err.Error())
	}
	return evalSchema(schema, value, "")
}

func evalSchema(schema any, value any, path string) error {
	m, ok := schema.(map[string]any)
	if !ok {
		return nil
	}
	if c, ok := m["const"]; ok && !jsonEqual(c, value) {
		return verr("INVALID_INPUT", pathOrRoot(path), "const mismatch")
	}
	if e, ok := m["enum"].([]any); ok {
		found := false
		for _, x := range e {
			if jsonEqual(x, value) {
				found = true
			}
		}
		if !found {
			return verr("INVALID_INPUT", pathOrRoot(path), "enum mismatch")
		}
	}
	if typ, ok := m["type"]; ok && !typeOK(typ, value) {
		return verr("INVALID_INPUT", pathOrRoot(path), "type mismatch")
	}
	if one, ok := m["oneOf"].([]any); ok {
		n := 0
		for _, sub := range one {
			if err := evalSchema(sub, value, path); err == nil {
				n++
			}
		}
		if n != 1 {
			return verr("INVALID_INPUT", pathOrRoot(path), "oneOf mismatch")
		}
	}
	if ifs, ok := m["if"]; ok {
		if evalSchema(ifs, value, path) == nil {
			if th, ok := m["then"]; ok {
				if err := evalSchema(th, value, path); err != nil {
					return err
				}
			}
		}
	}
	if props, ok := m["properties"].(map[string]any); ok {
		obj, ok := value.(map[string]any)
		if !ok {
			return verr("INVALID_INPUT", pathOrRoot(path), "object required")
		}
		if req, ok := m["required"].([]any); ok {
			for _, r := range req {
				k := r.(string)
				if _, exists := obj[k]; !exists {
					return verr("INVALID_INPUT", pathJoin(path, k), "required missing")
				}
			}
		}
		if add, ok := m["additionalProperties"].(bool); ok && !add {
			for k := range obj {
				if _, known := props[k]; !known {
					return verr("INVALID_INPUT", pathJoin(path, k), "additional property")
				}
			}
		}
		for k, sub := range props {
			if v, exists := obj[k]; exists {
				if err := evalSchema(sub, v, pathJoin(path, k)); err != nil {
					return err
				}
			}
		}
	}
	if items, ok := m["items"]; ok {
		arr, ok := value.([]any)
		if !ok {
			return verr("INVALID_INPUT", pathOrRoot(path), "array required")
		}
		for i, v := range arr {
			if err := evalSchema(items, v, fmt.Sprintf("%s/%d", pathOrRoot(path), i)); err != nil {
				return err
			}
		}
	}
	if min, ok := num(m["minItems"]); ok {
		if arr, ok := value.([]any); ok && uint64(len(arr)) < min {
			return verr("INVALID_INPUT", pathOrRoot(path), "minItems")
		}
	}
	if max, ok := num(m["maxItems"]); ok {
		if arr, ok := value.([]any); ok && uint64(len(arr)) > max {
			return verr("INVALID_INPUT", pathOrRoot(path), "maxItems")
		}
	}
	if min, ok := num(m["minimum"]); ok {
		if n, ok := asU(value); ok && n < min {
			return verr("INVALID_INPUT", pathOrRoot(path), "minimum")
		}
	}
	if max, ok := num(m["maximum"]); ok {
		if n, ok := asU(value); ok && n > max {
			return verr("INVALID_INPUT", pathOrRoot(path), "maximum")
		}
	}
	if ml, ok := num(m["minLength"]); ok {
		if st, ok := value.(string); ok && uint64(len(st)) < ml {
			return verr("INVALID_INPUT", pathOrRoot(path), "minLength")
		}
	}
	if pat, ok := m["pattern"].(string); ok {
		if st, ok := value.(string); ok {
			matched, _ := regexp.MatchString(pat, st)
			if !matched {
				return verr("INVALID_INPUT", pathOrRoot(path), "pattern")
			}
		}
	}
	return nil
}
func pathOrRoot(p string) string {
	if p == "" {
		return "/"
	}
	return p
}
func pathJoin(p, k string) string {
	if p == "" {
		return "/" + k
	}
	return p + "/" + k
}
func jsonEqual(a, b any) bool {
	ab, _ := json.Marshal(a)
	bb, _ := json.Marshal(b)
	return bytes.Equal(ab, bb)
}
func num(v any) (uint64, bool) {
	switch x := v.(type) {
	case json.Number:
		u, err := parseU64(x.String())
		return u, err == nil
	case float64:
		return uint64(x), true
	}
	return 0, false
}
func asU(v any) (uint64, bool) {
	if n, ok := v.(json.Number); ok {
		u, err := parseU64(n.String())
		return u, err == nil
	}
	return 0, false
}
func typeOK(t any, v any) bool {
	if arr, ok := t.([]any); ok {
		for _, x := range arr {
			if typeOK(x, v) {
				return true
			}
		}
		return false
	}
	s, ok := t.(string)
	if !ok {
		return true
	}
	switch s {
	case "object":
		_, ok := v.(map[string]any)
		return ok
	case "array":
		_, ok := v.([]any)
		return ok
	case "string":
		_, ok := v.(string)
		return ok
	case "boolean":
		_, ok := v.(bool)
		return ok
	case "null":
		return v == nil
	case "integer":
		_, ok := v.(json.Number)
		return ok
	}
	return true
}

func validateFailure(t Terminal) error {
	sum := uint64(0)
	for _, k := range failureNames {
		v, ok := t.Accounting.FailureCounters[k]
		if !ok {
			return verr("INVARIANT_FAILED", "/accounting/failure_counters/"+k, "missing")
		}
		sum += v
	}
	if len(t.Accounting.FailureCounters) != len(failureNames) {
		return verr("INVARIANT_FAILED", "/accounting/failure_counters", "extra counter")
	}
	if t.Terminal == "COMPLETE" {
		if t.Failure != nil || sum != 0 {
			return verr("INVARIANT_FAILED", "/failure", "complete failure state")
		}
		return nil
	}
	if t.Terminal != "FAILED" {
		return verr("INVARIANT_FAILED", "/terminal", "bad terminal")
	}
	if t.Failure == nil || sum != 1 || t.Accounting.FailureCounters[t.Failure.Code] != 1 {
		return verr("INVARIANT_FAILED", "/failure", "failed counter/code mismatch")
	}
	stage := map[string]string{"INVALID_INPUT": "raw", "CANCELLED": "control", "DEADLINE_EXCEEDED": "control", "ASSOCIATION_FAILED": "association", "ADMISSION_FAILED": "admission", "RESOURCE_EXHAUSTED": "scan", "OVERFLOW": "accounting", "INVARIANT_FAILED": "invariant"}[t.Failure.Code]
	if stage == "" || t.Failure.Stage != stage {
		return verr("INVARIANT_FAILED", "/failure/stage", "bad stage")
	}
	req := map[string][]string{"INVALID_INPUT": {"reason"}, "CANCELLED": {"control"}, "DEADLINE_EXCEEDED": {"deadline"}, "ASSOCIATION_FAILED": {"source_id"}, "ADMISSION_FAILED": {"source_id"}, "RESOURCE_EXHAUSTED": {"limit"}, "OVERFLOW": {"counter"}, "INVARIANT_FAILED": {"invariant"}}[t.Failure.Code]
	if len(t.Failure.Detail) != len(req) {
		return verr("INVARIANT_FAILED", "/failure/detail", "detail shape")
	}
	for _, k := range req {
		if _, ok := t.Failure.Detail[k]; !ok {
			return verr("INVARIANT_FAILED", "/failure/detail/"+k, "missing")
		}
	}
	early := map[string]bool{"INVALID_INPUT": true, "CANCELLED": true, "DEADLINE_EXCEEDED": true, "ASSOCIATION_FAILED": true}
	if early[t.Failure.Code] && t.Admission.Completed {
		return verr("INVARIANT_FAILED", "/admission", "early failure must not have admission")
	}
	if !early[t.Failure.Code] && !t.Admission.Completed {
		return verr("INVARIANT_FAILED", "/admission", "late failure requires admission")
	}
	if len(t.Matches) != 0 || t.RangeUnionCandidate != nil {
		return verr("INVARIANT_FAILED", "/matches", "failed terminal must not retain matches")
	}
	return nil
}

func validateSourcesMatches(t Terminal, b Bundle) error {
	src := map[string]Source{}
	if t.Admission.Completed {
		if len(t.Admission.AdmittedSourceIDs) != len(t.Sources) {
			return verr("ADMISSION_FAILED", "/admission/admitted_source_ids", "membership count")
		}
		for i, s := range t.Sources {
			if t.Admission.AdmittedSourceIDs[i] != s.SourceID {
				return verr("ADMISSION_FAILED", "/admission/admitted_source_ids", "membership order")
			}
		}
	}
	for _, s := range t.Sources {
		data, ok := b.AdmittedSourceBytes[s.SourceID]
		if !ok {
			return verr("ASSOCIATION_FAILED", "/sources/"+s.SourceID, "missing source bytes")
		}
		if uint64(len([]byte(data))) != s.ByteLength || sha(data) != s.ContentSHA256 {
			return verr("ASSOCIATION_FAILED", "/sources/"+s.SourceID, "source metadata mismatch")
		}
		src[s.SourceID] = s
	}
	prevPath, prevStart, prevEnd, prevOrd := uint64(0), uint64(0), uint64(0), uint64(0)
	for i, m := range t.Matches {
		s, ok := src[m.SourceID]
		if !ok {
			return verr("ASSOCIATION_FAILED", "/matches", "unknown source")
		}
		if m.StartByte >= m.EndByte {
			return verr("INVARIANT_FAILED", "/matches/start_byte", "start must be < end")
		}
		data := []byte(b.AdmittedSourceBytes[m.SourceID])
		if m.EndByte > uint64(len(data)) {
			return verr("ASSOCIATION_FAILED", "/matches/end_byte", "out of source")
		}
		if string(data[m.StartByte:m.EndByte]) != m.Literal || m.Literal != t.Request.Query {
			return verr("ASSOCIATION_FAILED", "/matches/literal", "literal mismatch")
		}
		if m.PathBytes != s.PathBytes {
			return verr("ASSOCIATION_FAILED", "/matches/path_bytes", "path mismatch")
		}
		if i > 0 && (m.PathBytes < prevPath || (m.PathBytes == prevPath && (m.StartByte < prevStart || (m.StartByte == prevStart && (m.EndByte < prevEnd || (m.EndByte == prevEnd && m.Ordinal <= prevOrd)))))) {
			return verr("INVARIANT_FAILED", "/matches", "deterministic order")
		}
		prevPath, prevStart, prevEnd, prevOrd = m.PathBytes, m.StartByte, m.EndByte, m.Ordinal
	}
	if len(t.Positions) != len(t.Matches) {
		return verr("ASSOCIATION_FAILED", "/positions", "position count")
	}
	posSeen := map[string]bool{}
	for _, p := range t.Positions {
		if posSeen[p.MatchID] {
			return verr("ASSOCIATION_FAILED", "/positions/"+p.MatchID, "duplicate position")
		}
		posSeen[p.MatchID] = true
		var mm *Match
		for i := range t.Matches {
			if t.Matches[i].MatchID == p.MatchID {
				mm = &t.Matches[i]
			}
		}
		if mm == nil {
			return verr("ASSOCIATION_FAILED", "/positions", "no match")
		}
		got := positionFor([]byte(b.AdmittedSourceBytes[mm.SourceID]), mm.StartByte, mm.EndByte)
		got.MatchID = p.MatchID
		if *got != p {
			return verr("ASSOCIATION_FAILED", "/positions/"+p.MatchID, "utf16 position mismatch")
		}
	}
	return nil
}

func validateCandidate(t Terminal) error {
	if t.Terminal == "COMPLETE" {
		c := t.RangeUnionCandidate
		if c == nil {
			return verr("INVARIANT_FAILED", "/range_union_candidate", "required")
		}
		if c.Operation != CandidateOperation || c.ExecutedLocation || !c.CandidateOnly || c.AdmissionDigest != t.Admission.AdmissionDigest {
			return verr("INVARIANT_FAILED", "/range_union_candidate", "bad candidate constants")
		}
		if len(c.MemberMatchIDs) != len(t.Matches) || len(c.QualifiedLocationPins) != len(t.Matches) {
			return verr("INVARIANT_FAILED", "/range_union_candidate/member_match_ids", "member count")
		}
		h := sha256.New()
		for i, m := range t.Matches {
			if c.MemberMatchIDs[i] != m.MatchID {
				return verr("INVARIANT_FAILED", "/range_union_candidate/member_match_ids", "not ordered matches")
			}
			pin := c.QualifiedLocationPins[i]
			pos := t.Positions[i]
			if pin.SourceID != m.SourceID || pin.StartByte != m.StartByte || pin.EndByte != m.EndByte || pin.StartLine != pos.StartLine || pin.StartCharacterUTF16 != pos.StartCharacterUTF16 || pin.EndLine != pos.EndLine || pin.EndCharacterUTF16 != pos.EndCharacterUTF16 {
				return verr("INVARIANT_FAILED", "/range_union_candidate/qualified_location_pins", "pin mismatch")
			}
			fmt.Fprintf(h, "%s:%s:%d:%d:%d\n", m.MatchID, m.SourceID, m.PathBytes, m.StartByte, m.EndByte)
		}
		if c.CandidateDigest != "sha256:"+hex.EncodeToString(h.Sum(nil)) {
			return verr("INVARIANT_FAILED", "/range_union_candidate/candidate_digest", "bad digest")
		}
	}
	return nil
}
func validateAccounting(t Terminal, term string) error {
	a := t.Accounting
	w, ok := weightedW(a)
	if !ok {
		return verr("OVERFLOW", "/accounting/W_work", "overflow")
	}
	if a.WWork != w {
		return verr("INVARIANT_FAILED", "/accounting/W_work", "bad weighted formula")
	}
	if uint64(len([]byte(term))) != a.BOutputBytes {
		return verr("INVARIANT_FAILED", "/accounting/B_output_bytes", "encoded length mismatch")
	}
	if got, ok := fixedPointB(t); !ok {
		return verr("INVARIANT_FAILED", "/accounting/B_output_bytes", "fixed point not reached")
	} else if got != a.BOutputBytes {
		return verr("INVARIANT_FAILED", "/accounting/B_output_bytes", "fixed point mismatch")
	}
	return nil
}
func weightedW(a Accounting) (uint64, bool) {
	vals := []struct{ v, m uint64 }{{a.JFiles, 3}, {a.QQueryBytes, 5}, {a.PPathBytes, 7}, {a.SSourceBytes, 1}, {a.TScannedTuples, 11}, {a.MMatches, 13}, {a.RRanges, 17}, {a.UUTF16Units, 19}, {a.BOutputBytes, 31}}
	total := uint64(50)
	for _, x := range vals {
		if x.v != 0 && x.v > math.MaxUint64/x.m {
			return 0, false
		}
		p := x.v * x.m
		if total > math.MaxUint64-p {
			return 0, false
		}
		total += p
	}
	return total, true
}
func fixedPointB(t Terminal) (uint64, bool) {
	x := t
	x.Custody.TerminalResultSHA256 = zeroSHA
	x.Replay.TerminalPreimageSHA256 = zeroSHA
	last := uint64(0)
	for i := 0; i < MaxIterations; i++ {
		b, _ := CanonicalJSON(x)
		n := uint64(len([]byte(b)))
		if n < last {
			return 0, false
		}
		x.Accounting.BOutputBytes = n
		w, ok := weightedW(x.Accounting)
		if !ok {
			return 0, false
		}
		x.Accounting.WWork = w
		if n == last {
			return n, true
		}
		last = n
	}
	return 0, false
}
func normalizedTerminalPreimage(t Terminal) string {
	x := t
	x.Custody.TerminalResultSHA256 = zeroSHA
	x.Replay.TerminalPreimageSHA256 = zeroSHA
	s, _ := CanonicalJSON(x)
	return s
}

func positionFor(data []byte, start, end uint64) *Position {
	sl, sc := lineChar(data, start)
	el, ec := lineChar(data, end)
	return &Position{StartLine: sl, StartCharacterUTF16: sc, EndLine: el, EndCharacterUTF16: ec}
}
func lineChar(data []byte, off uint64) (uint64, uint64) {
	line, ch := uint64(0), uint64(0)
	for i := uint64(0); i < off; {
		r, n := utf8.DecodeRune(data[i:])
		if r == '\n' {
			line++
			ch = 0
		} else {
			ch += uint64(len(utf16.Encode([]rune{r})))
		}
		i += uint64(n)
	}
	return line, ch
}
func digestOrEmpty(s string) string {
	if s == "" {
		return zeroSHA
	}
	if !strings.HasSuffix(s, "\n") {
		return ""
	}
	return sha(s)
}
func sha(s string) string { h := sha256.Sum256([]byte(s)); return "sha256:" + hex.EncodeToString(h[:]) }
func parseU64(s string) (uint64, error) {
	if strings.ContainsAny(s, ".-+eE") {
		return 0, errors.New("not uint64")
	}
	var n uint64
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, errors.New("not uint64")
		}
		d := uint64(c - '0')
		if n > (math.MaxUint64-d)/10 {
			return 0, errors.New("uint64 overflow")
		}
		n = n*10 + d
	}
	return n, nil
}

func CanonicalJSON(v any) (string, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	var x any
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	if err := dec.Decode(&x); err != nil {
		return "", err
	}
	var buf bytes.Buffer
	writeCanon(&buf, x)
	buf.WriteByte('\n')
	return buf.String(), nil
}
func writeCanon(buf *bytes.Buffer, v any) {
	switch x := v.(type) {
	case nil:
		buf.WriteString("null")
	case bool:
		if x {
			buf.WriteString("true")
		} else {
			buf.WriteString("false")
		}
	case string:
		b, _ := json.Marshal(x)
		buf.Write(b)
	case json.Number:
		buf.WriteString(x.String())
	case float64:
		buf.WriteString(fmt.Sprintf("%.0f", x))
	case []any:
		buf.WriteByte('[')
		for i, e := range x {
			if i > 0 {
				buf.WriteByte(',')
			}
			writeCanon(buf, e)
		}
		buf.WriteByte(']')
	case map[string]any:
		keys := make([]string, 0, len(x))
		for k := range x {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		buf.WriteByte('{')
		for i, k := range keys {
			if i > 0 {
				buf.WriteByte(',')
			}
			kb, _ := json.Marshal(k)
			buf.Write(kb)
			buf.WriteByte(':')
			writeCanon(buf, x[k])
		}
		buf.WriteByte('}')
	}
}
func decodeExact(b []byte, dst any, required []string) error {
	var raw map[string]json.RawMessage
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	dec.DisallowUnknownFields()
	if err := dec.Decode(&raw); err != nil {
		return err
	}
	allow := map[string]bool{}
	for _, k := range required {
		allow[k] = true
		if _, ok := raw[k]; !ok {
			return fmt.Errorf("missing %s", k)
		}
	}
	for k := range raw {
		if !allow[k] {
			return fmt.Errorf("unknown %s", k)
		}
	}
	dec = json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	dec.DisallowUnknownFields()
	return dec.Decode(dst)
}
func walkSchema(b []byte) error {
	var s map[string]any
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	if err := dec.Decode(&s); err != nil {
		return verr("INVALID_INPUT", "/schema_bytes", err.Error())
	}
	if s["$schema"] != "https://json-schema.org/draft/2020-12/schema" {
		return verr("INVARIANT_FAILED", "/schema_bytes/$schema", "not draft2020-12")
	}
	if s["$id"] != TerminalSchemaVersion {
		return verr("INVARIANT_FAILED", "/schema_bytes/$id", "wrong id")
	}
	req, ok := s["required"].([]any)
	if !ok || len(req) < 10 {
		return verr("INVARIANT_FAILED", "/schema_bytes/required", "schema drift")
	}
	props, ok := s["properties"].(map[string]any)
	if !ok {
		return verr("INVARIANT_FAILED", "/schema_bytes/properties", "schema drift")
	}
	for _, k := range []string{"accounting", "custody", "replay", "range_union_candidate"} {
		if _, ok := props[k]; !ok {
			return verr("INVARIANT_FAILED", "/schema_bytes/properties/"+k, "missing")
		}
	}
	return nil
}
