package adr0007v4contractvalidator

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"unicode/utf8"
)

const SchemaVersion = "lsp-trace.adr0007.source-text-search.terminal.private.v4"
const CanonicalProfile = "UTF8_SORTED_KEYS_ONE_LF"
const MaxUint64String = "18446744073709551615"

type Terminal struct {
	SchemaVersion       string      `json:"schema_version"`
	Terminal            string      `json:"terminal"`
	Request             Request     `json:"request"`
	Attempt             Attempt     `json:"attempt"`
	Control             Control     `json:"control"`
	Sources             []Source    `json:"sources"`
	Admission           Admission   `json:"admission"`
	Matches             []Match     `json:"matches"`
	Positions           []Position  `json:"positions"`
	RangeUnionCandidate *Candidate  `json:"range_union_candidate"`
	Policy              Policy      `json:"policy"`
	Limits              Limits      `json:"limits"`
	Accounting          Accounting  `json:"accounting"`
	Failure             *Failure    `json:"failure"`
	Custody             Custody     `json:"custody"`
	Replay              Replay      `json:"replay"`
	Tooling             Tooling     `json:"tooling"`
	Predecessor         Predecessor `json:"predecessor"`
	Payload             Payload     `json:"payload"`
}

type Request struct {
	RequestID       string `json:"request_id"`
	RawSHA256       string `json:"raw_sha256"`
	QueryUTF8       string `json:"query_utf8"`
	SourceSetDigest string `json:"source_set_digest"`
}
type Attempt struct {
	AttemptID      string `json:"attempt_id"`
	AttemptOrdinal uint64 `json:"attempt_ordinal"`
	MalformedRaw   bool   `json:"malformed_raw"`
}
type Control struct {
	CancelRequested   bool   `json:"cancel_requested"`
	DeadlineUnixNanos uint64 `json:"deadline_unix_nanos"`
	ControlDigest     string `json:"control_digest"`
}
type Source struct {
	SourceID         string `json:"source_id"`
	LogicalURI       string `json:"logical_uri"`
	Revision         string `json:"revision"`
	ContentSHA256    string `json:"content_sha256"`
	ByteLength       uint64 `json:"byte_length"`
	AdmissionOrdinal uint64 `json:"admission_ordinal"`
}
type Admission struct {
	Completed         bool     `json:"completed"`
	AdmittedSourceIDs []string `json:"admitted_source_ids"`
	AdmissionDigest   string   `json:"admission_digest"`
}
type Match struct {
	MatchID       string `json:"match_id"`
	SourceID      string `json:"source_id"`
	StartByte     uint64 `json:"start_byte"`
	EndByte       uint64 `json:"end_byte"`
	LiteralSHA256 string `json:"literal_sha256"`
}
type Position struct {
	MatchID            string `json:"match_id"`
	StartLine          uint64 `json:"start_line"`
	StartCharacterUTF8 uint64 `json:"start_character_utf8"`
	EndLine            uint64 `json:"end_line"`
	EndCharacterUTF8   uint64 `json:"end_character_utf8"`
}
type Candidate struct {
	CandidateID    string   `json:"candidate_id"`
	MemberMatchIDs []string `json:"member_match_ids"`
	RangeDigest    string   `json:"range_digest"`
}
type Policy struct {
	LiteralMode   string `json:"literal_mode"`
	Normalization string `json:"normalization"`
	Authority     string `json:"authority"`
	Accepted      bool   `json:"accepted"`
}
type Limits struct {
	MaxSources          uint64 `json:"max_sources"`
	MaxSourceBytes      uint64 `json:"max_source_bytes"`
	MaxTotalSourceBytes uint64 `json:"max_total_source_bytes"`
	MaxMatches          uint64 `json:"max_matches"`
	MaxWork             uint64 `json:"max_work"`
	MaxOutputBytes      uint64 `json:"max_output_bytes"`
}
type Accounting struct {
	J             uint64 `json:"J"`
	Q             uint64 `json:"Q"`
	P             uint64 `json:"P"`
	S             uint64 `json:"S"`
	T             uint64 `json:"T"`
	M             uint64 `json:"M"`
	R             uint64 `json:"R"`
	U             uint64 `json:"U"`
	B             uint64 `json:"B"`
	W             uint64 `json:"W"`
	FailParse     uint64 `json:"fail_parse"`
	FailAdmission uint64 `json:"fail_admission"`
	FailResource  uint64 `json:"fail_resource"`
	FailCancelled uint64 `json:"fail_cancelled"`
	FailDeadline  uint64 `json:"fail_deadline"`
	FailOverflow  uint64 `json:"fail_overflow"`
	FailInternal  uint64 `json:"fail_internal"`
	FailPolicy    uint64 `json:"fail_policy"`
}
type Failure struct {
	Code   string         `json:"code"`
	Detail map[string]any `json:"detail"`
}
type Custody struct {
	Producer      string `json:"producer"`
	Authority     string `json:"authority"`
	Accepted      bool   `json:"accepted"`
	State         string `json:"state"`
	Resolution    string `json:"resolution"`
	CustodyDigest string `json:"custody_digest"`
}
type Replay struct {
	CanonicalJSONProfile string `json:"canonical_json_profile"`
	RawInputSHA256       string `json:"raw_input_sha256"`
	TerminalSHA256       string `json:"terminal_sha256"`
	ReplayDigest         string `json:"replay_digest"`
}
type Tooling struct {
	Validator      string `json:"validator"`
	ContractCommit string `json:"contract_commit"`
	ManifestSHA256 string `json:"manifest_sha256"`
}
type Predecessor struct {
	WireVersionsPreserved []string `json:"wire_versions_preserved"`
}
type Payload struct {
	PayloadDigest string   `json:"payload_digest"`
	Frozen        bool     `json:"frozen"`
	Members       []string `json:"members"`
}

func (r *Request) UnmarshalJSON(b []byte) error {
	type x struct {
		RequestID       string `json:"request_id"`
		RawSHA256       string `json:"raw_sha256"`
		QueryUTF8       string `json:"query_utf8"`
		SourceSetDigest string `json:"source_set_digest"`
	}
	var v x
	if err := strictObj(b, []string{"request_id", "raw_sha256", "query_utf8", "source_set_digest"}, &v); err != nil {
		return err
	}
	*r = Request(v)
	return nil
}
func (s *Source) UnmarshalJSON(b []byte) error {
	type x struct {
		SourceID         string `json:"source_id"`
		LogicalURI       string `json:"logical_uri"`
		Revision         string `json:"revision"`
		ContentSHA256    string `json:"content_sha256"`
		ByteLength       uint64 `json:"byte_length"`
		AdmissionOrdinal uint64 `json:"admission_ordinal"`
	}
	var v x
	if err := strictObj(b, []string{"source_id", "logical_uri", "revision", "content_sha256", "byte_length", "admission_ordinal"}, &v); err != nil {
		return err
	}
	*s = Source(v)
	return nil
}
func (m *Match) UnmarshalJSON(b []byte) error {
	type x struct {
		MatchID       string `json:"match_id"`
		SourceID      string `json:"source_id"`
		StartByte     uint64 `json:"start_byte"`
		EndByte       uint64 `json:"end_byte"`
		LiteralSHA256 string `json:"literal_sha256"`
	}
	var v x
	if err := strictObj(b, []string{"match_id", "source_id", "start_byte", "end_byte", "literal_sha256"}, &v); err != nil {
		return err
	}
	*m = Match(v)
	return nil
}
func (p *Position) UnmarshalJSON(b []byte) error {
	type x struct {
		MatchID            string `json:"match_id"`
		StartLine          uint64 `json:"start_line"`
		StartCharacterUTF8 uint64 `json:"start_character_utf8"`
		EndLine            uint64 `json:"end_line"`
		EndCharacterUTF8   uint64 `json:"end_character_utf8"`
	}
	var v x
	if err := strictObj(b, []string{"match_id", "start_line", "start_character_utf8", "end_line", "end_character_utf8"}, &v); err != nil {
		return err
	}
	*p = Position(v)
	return nil
}
func (p *Policy) UnmarshalJSON(b []byte) error {
	type x struct {
		LiteralMode   string `json:"literal_mode"`
		Normalization string `json:"normalization"`
		Authority     string `json:"authority"`
		Accepted      bool   `json:"accepted"`
	}
	var v x
	if err := strictObj(b, []string{"literal_mode", "normalization", "authority", "accepted"}, &v); err != nil {
		return err
	}
	*p = Policy(v)
	return nil
}
func (l *Limits) UnmarshalJSON(b []byte) error {
	type x struct {
		MaxSources          uint64 `json:"max_sources"`
		MaxSourceBytes      uint64 `json:"max_source_bytes"`
		MaxTotalSourceBytes uint64 `json:"max_total_source_bytes"`
		MaxMatches          uint64 `json:"max_matches"`
		MaxWork             uint64 `json:"max_work"`
		MaxOutputBytes      uint64 `json:"max_output_bytes"`
	}
	var v x
	if err := strictObj(b, []string{"max_sources", "max_source_bytes", "max_total_source_bytes", "max_matches", "max_work", "max_output_bytes"}, &v); err != nil {
		return err
	}
	*l = Limits(v)
	return nil
}
func (a *Accounting) UnmarshalJSON(b []byte) error {
	type x Accounting
	var v x
	if err := strictObj(b, []string{"J", "Q", "P", "S", "T", "M", "R", "U", "B", "W", "fail_parse", "fail_admission", "fail_resource", "fail_cancelled", "fail_deadline", "fail_overflow", "fail_internal", "fail_policy"}, &v); err != nil {
		return err
	}
	*a = Accounting(v)
	return nil
}
func (c *Custody) UnmarshalJSON(b []byte) error {
	type x struct {
		Producer      string `json:"producer"`
		Authority     string `json:"authority"`
		Accepted      bool   `json:"accepted"`
		State         string `json:"state"`
		Resolution    string `json:"resolution"`
		CustodyDigest string `json:"custody_digest"`
	}
	var v x
	if err := strictObj(b, []string{"producer", "authority", "accepted", "state", "resolution", "custody_digest"}, &v); err != nil {
		return err
	}
	*c = Custody(v)
	return nil
}
func (r *Replay) UnmarshalJSON(b []byte) error {
	type x struct {
		CanonicalJSONProfile string `json:"canonical_json_profile"`
		RawInputSHA256       string `json:"raw_input_sha256"`
		TerminalSHA256       string `json:"terminal_sha256"`
		ReplayDigest         string `json:"replay_digest"`
	}
	var v x
	if err := strictObj(b, []string{"canonical_json_profile", "raw_input_sha256", "terminal_sha256", "replay_digest"}, &v); err != nil {
		return err
	}
	*r = Replay(v)
	return nil
}
func (t *Tooling) UnmarshalJSON(b []byte) error {
	type x struct {
		Validator      string `json:"validator"`
		ContractCommit string `json:"contract_commit"`
		ManifestSHA256 string `json:"manifest_sha256"`
	}
	var v x
	if err := strictObj(b, []string{"validator", "contract_commit", "manifest_sha256"}, &v); err != nil {
		return err
	}
	*t = Tooling(v)
	return nil
}
func (p *Payload) UnmarshalJSON(b []byte) error {
	type x struct {
		PayloadDigest string   `json:"payload_digest"`
		Frozen        bool     `json:"frozen"`
		Members       []string `json:"members"`
	}
	var v x
	if err := strictObj(b, []string{"payload_digest", "frozen", "members"}, &v); err != nil {
		return err
	}
	*p = Payload(v)
	return nil
}

func strictObj(b []byte, allowed []string, dst any) error {
	var raw map[string]json.RawMessage
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&raw); err != nil {
		return err
	}
	if dec.More() {
		return errors.New("trailing data")
	}
	allow := map[string]bool{}
	for _, k := range allowed {
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
	dec.DisallowUnknownFields()
	return dec.Decode(dst)
}

func ValidateFile(path string) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return ValidateBytes(b)
}
func ValidateBytes(b []byte) error {
	if !utf8.Valid(b) {
		return errors.New("invalid utf8")
	}
	if !bytes.HasSuffix(b, []byte("\n")) {
		return errors.New("canonical json must end with one LF")
	}
	if bytes.HasSuffix(bytes.TrimSuffix(b, []byte("\n")), []byte("\n")) {
		return errors.New("canonical json must have exactly one final LF")
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	var t Terminal
	if err := dec.Decode(&t); err != nil {
		return err
	}
	if dec.Decode(&struct{}{}) != io.EOF {
		return errors.New("trailing data")
	}
	canon, err := CanonicalJSON(t)
	if err != nil {
		return err
	}
	if !bytes.Equal(canon, b) {
		return errors.New("non-canonical json")
	}
	return validate(t)
}

func validate(t Terminal) error {
	if t.SchemaVersion != SchemaVersion {
		return errors.New("bad schema_version")
	}
	if t.Policy.LiteralMode != "EXACT_NONEMPTY_CASE_SENSITIVE_UTF8" || t.Policy.Normalization != "NONE" || t.Policy.Authority != "authority0" || t.Policy.Accepted {
		return errors.New("bad policy constants")
	}
	if t.Custody.Authority != "authority0" || t.Custody.Accepted || t.Custody.State != "UNKNOWN" || t.Custody.Resolution != "UNRESOLVED" {
		return errors.New("bad custody constants")
	}
	if t.Replay.CanonicalJSONProfile != CanonicalProfile {
		return errors.New("bad replay profile")
	}
	if t.Tooling.Validator != "adr0007-source-text-search-v4-contract-validate" {
		return errors.New("bad tooling validator")
	}
	if strings.Join(t.Predecessor.WireVersionsPreserved, ",") != "v1,v2,v3" {
		return errors.New("predecessor pins must preserve v1,v2,v3")
	}
	if t.Attempt.MalformedRaw {
		want := "attempt-raw-sha256-" + strings.TrimPrefix(t.Request.RawSHA256, "sha256:")
		if t.Attempt.AttemptID != want {
			return errors.New("malformed raw attempt identity mismatch")
		}
	}
	sources := map[string]Source{}
	var prevOrd uint64
	for i, s := range t.Sources {
		if _, ok := sources[s.SourceID]; ok {
			return errors.New("duplicate source_id")
		}
		if i > 0 && s.AdmissionOrdinal <= prevOrd {
			return errors.New("source admission ordering")
		}
		prevOrd = s.AdmissionOrdinal
		sources[s.SourceID] = s
	}
	seenMatches := map[string]bool{}
	prevSource := ""
	var prevStart, prevEnd uint64
	for i, m := range t.Matches {
		if m.StartByte > m.EndByte {
			return errors.New("match start>end")
		}
		if _, ok := sources[m.SourceID]; !ok {
			return errors.New("match source unknown")
		}
		if seenMatches[m.MatchID] {
			return errors.New("duplicate match_id")
		}
		seenMatches[m.MatchID] = true
		if i > 0 && (m.SourceID < prevSource || (m.SourceID == prevSource && (m.StartByte < prevStart || (m.StartByte == prevStart && m.EndByte < prevEnd)))) {
			return errors.New("match ordering")
		}
		prevSource, prevStart, prevEnd = m.SourceID, m.StartByte, m.EndByte
	}
	for _, p := range t.Positions {
		if !seenMatches[p.MatchID] {
			return errors.New("position without match")
		}
		if p.StartLine > p.EndLine || (p.StartLine == p.EndLine && p.StartCharacterUTF8 > p.EndCharacterUTF8) {
			return errors.New("position ordering")
		}
	}
	if t.Accounting.W != t.Accounting.J+t.Accounting.Q+t.Accounting.P+t.Accounting.S+t.Accounting.T+t.Accounting.M+t.Accounting.R+t.Accounting.U+t.Accounting.B {
		return errors.New("W formula mismatch")
	}
	fails := failureCounters(t.Accounting)
	switch t.Terminal {
	case "COMPLETE":
		if t.Failure != nil {
			return errors.New("complete failure nonnull")
		}
		if t.RangeUnionCandidate == nil {
			return errors.New("complete candidate null")
		}
		if fails != 0 {
			return errors.New("complete failure counters nonzero")
		}
		if err := validateCandidate(*t.RangeUnionCandidate, t.Matches); err != nil {
			return err
		}
	case "FAILED":
		if t.Failure == nil {
			return errors.New("failed failure null")
		}
		if len(t.Matches) != 0 {
			return errors.New("failed matches nonempty")
		}
		if t.RangeUnionCandidate != nil {
			return errors.New("failed candidate nonnull")
		}
		if fails != 1 {
			return errors.New("failed failure counter not exactly one")
		}
		if !counterMatches(t.Failure.Code, t.Accounting) {
			return errors.New("failure counter/code mismatch")
		}
		if err := validateFailureDetail(*t.Failure); err != nil {
			return err
		}
	default:
		return errors.New("bad terminal")
	}
	if digestStable("custody", t.Custody.CustodyDigest) == "" || digestStable("replay", t.Replay.ReplayDigest) == "" {
		return errors.New("digest internal")
	}
	return nil
}
func validateCandidate(c Candidate, ms []Match) error {
	if len(c.MemberMatchIDs) != len(ms) {
		return errors.New("candidate member count")
	}
	h := sha256.New()
	for i, m := range ms {
		if c.MemberMatchIDs[i] != m.MatchID {
			return errors.New("candidate members != matches")
		}
		fmt.Fprintf(h, "%s:%s:%d:%d\n", m.MatchID, m.SourceID, m.StartByte, m.EndByte)
	}
	if c.RangeDigest != "sha256:"+hex.EncodeToString(h.Sum(nil)) {
		return errors.New("candidate digest mismatch")
	}
	return nil
}
func validateFailureDetail(f Failure) error {
	req := map[string][]string{"PARSE": {"raw_sha256", "reason"}, "ADMISSION": {"source_id", "reason"}, "RESOURCE": {"resource", "limit"}, "CANCELLED": {"control_digest"}, "DEADLINE": {"deadline_unix_nanos"}, "OVERFLOW": {"counter", "limit"}, "INTERNAL": {"reason"}, "POLICY": {"policy_key", "reason"}}
	keys, ok := req[f.Code]
	if !ok {
		return errors.New("bad failure code")
	}
	if len(f.Detail) != len(keys) {
		return errors.New("failure detail key count")
	}
	for _, k := range keys {
		if _, ok := f.Detail[k]; !ok {
			return fmt.Errorf("failure detail missing %s", k)
		}
	}
	return nil
}
func failureCounters(a Accounting) uint64 {
	return a.FailParse + a.FailAdmission + a.FailResource + a.FailCancelled + a.FailDeadline + a.FailOverflow + a.FailInternal + a.FailPolicy
}
func counterMatches(code string, a Accounting) bool {
	return map[string]uint64{"PARSE": a.FailParse, "ADMISSION": a.FailAdmission, "RESOURCE": a.FailResource, "CANCELLED": a.FailCancelled, "DEADLINE": a.FailDeadline, "OVERFLOW": a.FailOverflow, "INTERNAL": a.FailInternal, "POLICY": a.FailPolicy}[code] == 1
}
func digestStable(domain, current string) string {
	if !strings.HasPrefix(current, "sha256:") {
		return ""
	}
	h := sha256.Sum256([]byte(domain))
	return "sha256:" + hex.EncodeToString(h[:])
}

func CanonicalJSON(v any) ([]byte, error) {
	var x any
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(b, &x); err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	writeCanon(&buf, x)
	buf.WriteByte('\n')
	return buf.Bytes(), nil
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
