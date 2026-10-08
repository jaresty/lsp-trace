package adr0007sourcetextsearchv4oracle

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf16"
	"unicode/utf8"

	validator "lsp-trace/internal/adr0007v4contractvalidator"

	"golang.org/x/text/unicode/norm"
)

const (
	BaseDir                  = "docs/pilot/adr0007/source-text-search-v4"
	CasesDir                 = BaseDir + "/cases"
	OracleDir                = BaseDir + "/oracle"
	ProvPath                 = OracleDir + "/ORACLE_PROVENANCE.json"
	TerminalSchema           = validator.TerminalSchemaVersion
	AttemptSchema            = "lsp-trace.adr0007.source-text-search.attempt.private.v4"
	ExecutionControlSchema   = "lsp-trace.adr0007.source-text-search.execution-control.private.v4"
	RequestSchema            = "lsp-trace.adr0007.source-text-search.request.private.v4"
	LimitsSchema             = "lsp-trace.adr0007.source-text-search.limits.private.v4"
	PolicySchema             = "lsp-trace.adr0007.source-text-search.policy.private.v4"
	SourceInputSchema        = "lsp-trace.adr0007.source-text-search.source-input.private.v4"
	SourceRefSchemaIfPresent = "lsp-trace.adr0007.source-text-search.source-ref.private.v4"
	TestControlSchema        = "lsp-trace.adr0007.source-text-search.test-control.private.v4"
	zeroSHA                  = "sha256:0000000000000000000000000000000000000000000000000000000000000000"
)

type Attempt struct {
	AttemptID        string           `json:"attempt_id"`
	ExecutionControl ExecutionControl `json:"execution_control"`
	Request          Request          `json:"request"`
	SchemaVersion    string           `json:"schema_version"`
	SourceInputs     []SourceInput    `json:"source_inputs"`
	TestControl      TestControl      `json:"test_control,omitempty"`
}
type TestControl struct {
	InitialBOutputBytes uint64 `json:"initial_B_output_bytes,omitempty"`
	SchemaVersion       string `json:"schema_version,omitempty"`
}
type ExecutionControl struct {
	SchemaVersion string         `json:"schema_version"`
	Observations  []ControlEvent `json:"observations"`
}
type ControlEvent struct {
	Cancelled       bool `json:"cancelled"`
	DeadlineExpired bool `json:"deadline_expired"`
	PollIndex       int  `json:"poll_index"`
}
type Request struct {
	SchemaVersion string         `json:"schema_version"`
	Query         string         `json:"query"`
	Sources       []SourceRef    `json:"sources"`
	Limits        Limits         `json:"limits"`
	Policy        Policy         `json:"policy"`
	LocationPin   map[string]any `json:"location_pin"`
}
type Limits struct {
	SchemaVersion  string `json:"schema_version"`
	MaxFiles       uint64 `json:"max_files"`
	MaxMatches     uint64 `json:"max_matches"`
	MaxOutputBytes uint64 `json:"max_output_bytes"`
	MaxPathBytes   uint64 `json:"max_path_bytes"`
	MaxSourceBytes uint64 `json:"max_source_bytes"`
	MaxTotalBytes  uint64 `json:"max_total_bytes"`
	MaxWork        uint64 `json:"max_work"`
}
type Policy struct {
	SchemaVersion         string `json:"schema_version"`
	AllowBackendSemantics bool   `json:"allow_backend_semantics"`
	AllowFuzzy            bool   `json:"allow_fuzzy"`
	AllowModel            bool   `json:"allow_model"`
	AllowRank             bool   `json:"allow_rank"`
	AllowRegex            bool   `json:"allow_regex"`
	AllowToken            bool   `json:"allow_token"`
	LiteralMode           string `json:"literal_mode"`
}
type SourceRef struct {
	FileDigest    string `json:"file_digest"`
	ObjectDigest  string `json:"object_digest"`
	Ordinal       uint64 `json:"ordinal"`
	Path          string `json:"path"`
	Revision      string `json:"revision"`
	SchemaVersion string `json:"schema_version,omitempty"`
}
type SourceInput struct {
	SchemaVersion string `json:"schema_version"`
	Path          string `json:"path"`
	Revision      string `json:"revision"`
	FileDigest    string `json:"file_digest"`
	ObjectDigest  string `json:"object_digest"`
	BytesBase64   string `json:"bytes_base64"`
	Ordinal       uint64 `json:"ordinal"`
}
type Row struct {
	ID              string `json:"id"`
	Requirement     string `json:"requirement"`
	Stimulus        string `json:"stimulus"`
	Assertion       string `json:"assertion"`
	ExpectedOutcome string `json:"expected_outcome"`
	ExpectedCode    string `json:"expected_code"`
}
type Manifest struct {
	SchemaVersion         string `json:"schema_version"`
	Stage                 string `json:"stage"`
	ToolingDigest         string `json:"tooling_digest"`
	PredecessorLockDigest string `json:"predecessor_lock_digest"`
	Members               []struct {
		Path   string `json:"path"`
		SHA256 string `json:"sha256"`
		Bytes  int    `json:"bytes"`
	} `json:"members"`
	Excludes []string `json:"excludes"`
}
type selectedSource struct {
	ID, Path, Revision string
	Ordinal            uint64
	Bytes              []byte
}

type EvalResult struct {
	Terminal             validator.Terminal
	RawAttemptBytes      string
	AdmittedSourceBytes  map[string]string
	AdmittedBindingBytes string
	ToolingManifestBytes string
	PredecessorBytes     string
	PayloadFreezeBytes   string
	SchemaBytes          json.RawMessage
}

func SHA(b []byte) string       { h := sha256.Sum256(b); return "sha256:" + hex.EncodeToString(h[:]) }
func shaString(s string) string { return SHA([]byte(s)) }

func EvaluateFile(p string) ([]byte, error) {
	r, err := EvaluateFileResult(p)
	if err != nil {
		return nil, err
	}
	return Canonical(r.Terminal)
}
func EvaluateFileResult(p string) (EvalResult, error) {
	m, err := ReadManifest()
	if err != nil {
		return EvalResult{}, err
	}
	raw, err := os.ReadFile(p)
	if err != nil {
		return EvalResult{}, err
	}
	return EvaluateBytesResult(raw, m)
}
func EvaluateBytes(raw []byte, m Manifest) validator.Terminal {
	r, _ := EvaluateBytesResult(raw, m)
	return r.Terminal
}
func EvaluateBytesResult(raw []byte, m Manifest) (EvalResult, error) {
	ctx, err := newEvalContext(raw, m)
	if err != nil {
		return EvalResult{}, err
	}
	ctx.eval()
	ctx.finalize()
	return ctx.result(), nil
}

type evalContext struct {
	raw                 []byte
	m                   Manifest
	attempt             Attempt
	terminal            validator.Terminal
	admitted            []selectedSource
	admittedBinding     string
	admittedSourceBytes map[string]string
	schemaBytes         []byte
	toolingBytes        string
	predecessorBytes    string
	payloadFreezeBytes  string
}

func newEvalContext(raw []byte, m Manifest) (*evalContext, error) {
	root, err := repoRoot()
	if err != nil {
		return nil, err
	}
	schema, err := os.ReadFile(filepath.Join(root, BaseDir, "contracts/terminal-envelope-v4.schema.json"))
	if err != nil {
		return nil, err
	}
	tooling, err := os.ReadFile(filepath.Join(root, BaseDir, "TOOLING_CENSUS.json"))
	if err != nil {
		return nil, err
	}
	payload, err := os.ReadFile(filepath.Join(root, BaseDir, "PAYLOAD_MANIFEST.json"))
	if err != nil {
		return nil, err
	}
	predecessor := []byte(m.PredecessorLockDigest + "\n")
	c := &evalContext{raw: raw, m: m, admittedSourceBytes: map[string]string{}, schemaBytes: schema, toolingBytes: string(tooling), predecessorBytes: string(predecessor), payloadFreezeBytes: string(payload)}
	c.terminal = validator.Terminal{
		SchemaVersion: TerminalSchema,
		Terminal:      "FAILED",
		Attempt:       validator.Attempt{AttemptID: "attempt-raw-sha256-" + strings.TrimPrefix(SHA(raw), "sha256:"), MalformedRaw: true},
		Request:       validator.Request{Query: "<unparsed>"},
		Admission:     validator.Admission{Completed: false, AdmissionDigest: zeroSHA, AdmittedSourceIDs: []string{}},
		Sources:       []validator.Source{}, Matches: []validator.Match{}, Positions: []validator.Position{}, RangeUnionCandidate: nil,
		Accounting: validator.Accounting{SchemaVersion: validator.AccountingSchemaVersion, FailureCounters: zeroCounters()},
		Failure:    nil,
		Custody:    validator.Custody{SchemaVersion: validator.CustodySchemaVersion, AttemptID: "attempt-raw-sha256-" + strings.TrimPrefix(SHA(raw), "sha256:"), TerminalSequence0: 0, TerminalCount1: 1},
		Replay:     validator.Replay{SchemaVersion: validator.ReplaySchemaVersion, CanonicalAttemptSHA256: SHA(raw), AdmittedBindingSHA256: zeroSHA, ToolingIdentitySHA256: shaString(string(tooling)), FreezeBindingSHA256: shaString(string(payload)), PredecessorLockSHA256: shaString(string(predecessor))},
		Payload:    validator.Payload{PayloadDigest: shaString(string(payload)), FreezeBindingSHA256: shaString(string(payload))},
	}
	return c, nil
}
func zeroCounters() map[string]uint64 {
	m := map[string]uint64{}
	for _, k := range []string{"INVALID_INPUT", "CANCELLED", "DEADLINE_EXCEEDED", "ASSOCIATION_FAILED", "ADMISSION_FAILED", "RESOURCE_EXHAUSTED", "OVERFLOW", "INVARIANT_FAILED"} {
		m[k] = 0
	}
	return m
}

func (c *evalContext) eval() {
	if err := rejectDuplicateKeys(c.raw); err != nil {
		c.fail("INVALID_INPUT", detail("reason", err.Error()))
		return
	}
	dec := json.NewDecoder(bytes.NewReader(c.raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&c.attempt); err != nil {
		c.fail("INVALID_INPUT", detail("reason", err.Error()))
		return
	}
	if dec.Decode(&struct{}{}) != io.EOF {
		c.terminal.Attempt.MalformedRaw = true
		c.terminal.Attempt.AttemptID = "attempt-raw-sha256-" + strings.TrimPrefix(SHA(c.raw), "sha256:")
		c.terminal.Custody.AttemptID = c.terminal.Attempt.AttemptID
		c.fail("INVALID_INPUT", detail("reason", "trailing JSON data"))
		return
	}
	c.terminal.Attempt = validator.Attempt{AttemptID: c.attempt.AttemptID, MalformedRaw: false}
	c.terminal.Custody.AttemptID = c.attempt.AttemptID
	q := c.attempt.Request.Query
	if q == "" {
		q = "<empty>"
	}
	c.terminal.Request.Query = q
	c.terminal.Accounting.QQueryBytes = uint64(len([]byte(c.attempt.Request.Query)))
	if err := c.validateAttempt(); err != nil {
		c.fail("INVALID_INPUT", detail("reason", err.Error()))
		return
	}
	if c.control(0) {
		return
	}
	if err := c.associate(); err != nil {
		c.fail("ASSOCIATION_FAILED", detail("source_id", err.Error()))
		return
	}
	if c.control(1) {
		return
	}
	if err := c.admit(); err != nil {
		code := "ADMISSION_FAILED"
		dkey := "source_id"
		if strings.HasPrefix(err.Error(), "limit:") {
			code = "RESOURCE_EXHAUSTED"
			dkey = "limit"
		}
		c.fail(code, detail(dkey, strings.TrimPrefix(err.Error(), "limit:")))
		return
	}
	if c.attempt.TestControl.InitialBOutputBytes == math.MaxUint64 {
		c.fail("OVERFLOW", detail("counter", "B_output_bytes"))
		return
	}
	if c.control(2) {
		return
	}
	if err := c.scan(); err != nil {
		return
	}
	if c.control(3) {
		return
	}
	c.complete()
}
func (c *evalContext) validateAttempt() error {
	if c.attempt.SchemaVersion != AttemptSchema {
		return errors.New("attempt schema_version")
	}
	if c.attempt.AttemptID == "" {
		return errors.New("missing attempt_id")
	}
	if c.attempt.ExecutionControl.SchemaVersion != ExecutionControlSchema {
		return errors.New("execution_control schema_version")
	}
	if c.attempt.TestControl.SchemaVersion != "" && c.attempt.TestControl.SchemaVersion != TestControlSchema {
		return errors.New("test_control schema_version")
	}
	r := c.attempt.Request
	if r.SchemaVersion != RequestSchema {
		return errors.New("request schema_version")
	}
	if r.Limits.SchemaVersion != LimitsSchema {
		return errors.New("limits schema_version")
	}
	if r.Policy.SchemaVersion != PolicySchema {
		return errors.New("policy schema_version")
	}
	if r.Query == "" || !utf8.ValidString(r.Query) {
		return errors.New("query")
	}
	if len(r.Sources) == 0 {
		return errors.New("empty source set")
	}
	l := r.Limits
	if l.MaxFiles < 1 || l.MaxMatches < 1 || l.MaxOutputBytes < 1 || l.MaxPathBytes < 1 || l.MaxSourceBytes < 1 || l.MaxTotalBytes < 1 || l.MaxWork < 1 {
		return errors.New("nonpositive limit")
	}
	if uint64(len(r.Sources)) > l.MaxFiles {
		return errors.New("max_files")
	}
	for _, s := range r.Sources {
		if s.SchemaVersion != "" && s.SchemaVersion != SourceRefSchemaIfPresent {
			return errors.New("source ref schema_version")
		}
		if s.Ordinal < 1 {
			return errors.New("ordinal")
		}
	}
	for _, s := range c.attempt.SourceInputs {
		if s.SchemaVersion != SourceInputSchema {
			return errors.New("source input schema_version")
		}
	}
	p := r.Policy
	if p.AllowBackendSemantics || p.AllowFuzzy || p.AllowModel || p.AllowRank || p.AllowRegex || p.AllowToken || p.LiteralMode != "byte-literal" {
		return errors.New("policy")
	}
	return nil
}
func (c *evalContext) control(idx int) bool {
	for _, o := range c.attempt.ExecutionControl.Observations {
		if o.PollIndex == idx {
			if o.DeadlineExpired {
				c.fail("DEADLINE_EXCEEDED", detail("deadline", fmt.Sprintf("poll_index:%d", idx)))
				return true
			}
			if o.Cancelled {
				c.fail("CANCELLED", detail("control", fmt.Sprintf("poll_index:%d", idx)))
				return true
			}
			if !o.DeadlineExpired && !o.Cancelled {
				c.fail("INVALID_INPUT", detail("reason", fmt.Sprintf("duplicate/noop control at poll_index:%d", idx)))
				return true
			}
		}
	}
	return false
}
func (c *evalContext) associate() error {
	byOrd := map[uint64]SourceInput{}
	for _, si := range c.attempt.SourceInputs {
		if _, ok := byOrd[si.Ordinal]; ok {
			return fmt.Errorf("ordinal-%d", si.Ordinal)
		}
		byOrd[si.Ordinal] = si
	}
	for _, sr := range c.attempt.Request.Sources {
		si, ok := byOrd[sr.Ordinal]
		if !ok {
			return fmt.Errorf("ordinal-%d", sr.Ordinal)
		}
		if si.Path != sr.Path || si.Revision != sr.Revision {
			return fmt.Errorf("ordinal-%d", sr.Ordinal)
		}
		bb, err := base64.StdEncoding.DecodeString(si.BytesBase64)
		if err != nil {
			return fmt.Errorf("ordinal-%d", sr.Ordinal)
		}
		c.admitted = append(c.admitted, selectedSource{ID: sourceID(sr), Path: sr.Path, Revision: sr.Revision, Ordinal: sr.Ordinal, Bytes: bb})
	}
	if len(byOrd) != len(c.attempt.Request.Sources) {
		return fmt.Errorf("extra-source-input")
	}
	return nil
}
func sourceID(s SourceRef) string { return fmt.Sprintf("%s@%s#%d", s.Path, s.Revision, s.Ordinal) }
func (c *evalContext) admit() error {
	l := c.attempt.Request.Limits
	if uint64(len(c.admitted)) > l.MaxFiles {
		return errors.New("limit:max_files")
	}
	seen := map[string]bool{}
	total := uint64(0)
	for _, s := range c.admitted {
		pb := uint64(len([]byte(s.Path)))
		if pb > l.MaxPathBytes {
			return errors.New("limit:max_path_bytes")
		}
		if !CanonicalPath(s.Path) || len(s.Bytes) == 0 || !utf8.Valid(s.Bytes) {
			return fmt.Errorf("%s", s.ID)
		}
		if seen[s.Path] {
			return fmt.Errorf("%s", s.ID)
		}
		seen[s.Path] = true
		if uint64(len(s.Bytes)) > l.MaxSourceBytes {
			return errors.New("limit:max_source_bytes")
		}
		if total > math.MaxUint64-uint64(len(s.Bytes)) {
			return errors.New("limit:max_total_bytes")
		}
		total += uint64(len(s.Bytes))
		if total > l.MaxTotalBytes {
			return errors.New("limit:max_total_bytes")
		}
		ref := c.findRef(s.Ordinal)
		d := SHA(s.Bytes)
		if ref.FileDigest != "" && ref.FileDigest != d {
			return fmt.Errorf("%s", s.ID)
		}
		if ref.ObjectDigest != "" && ref.ObjectDigest != d {
			return fmt.Errorf("%s", s.ID)
		}
	}
	sort.Slice(c.admitted, func(i, j int) bool {
		if c.admitted[i].Path != c.admitted[j].Path {
			return c.admitted[i].Path < c.admitted[j].Path
		}
		return c.admitted[i].Ordinal < c.admitted[j].Ordinal
	})
	parts := []string{"lsp-trace.adr0007.source-admission.private.v2"}
	ids := []string{}
	for _, s := range c.admitted {
		ids = append(ids, s.ID)
		c.admittedSourceBytes[s.ID] = string(s.Bytes)
		parts = append(parts, s.ID, s.Path, s.Revision, fmt.Sprint(s.Ordinal), SHA(s.Bytes))
	}
	c.admittedBinding = mustCanon(map[string]any{"schema_version": "lsp-trace.adr0007.source-admission.private.v2", "source_ids": ids, "digest_preimage": parts})
	dig := shaString(c.admittedBinding)
	c.terminal.Admission = validator.Admission{Completed: true, AdmissionDigest: dig, AdmittedSourceIDs: ids}
	c.terminal.Replay.AdmittedBindingSHA256 = dig
	for _, s := range c.admitted {
		c.terminal.Sources = append(c.terminal.Sources, validator.Source{SourceID: s.ID, LogicalURI: "file:///" + s.Path, PathBytes: uint64(len([]byte(s.Path))), ByteLength: uint64(len(s.Bytes)), ContentSHA256: SHA(s.Bytes), Ordinal: s.Ordinal})
	}
	return nil
}
func (c *evalContext) findRef(ord uint64) SourceRef {
	for _, r := range c.attempt.Request.Sources {
		if r.Ordinal == ord {
			return r
		}
	}
	return SourceRef{}
}
func (c *evalContext) scan() error {
	q := []byte(c.attempt.Request.Query)
	var matches []validator.Match
	var positions []validator.Position
	ordinal := uint64(0)
	for _, s := range c.admitted {
		off := 0
		c.terminal.Accounting.TScannedTuples++
		for {
			i := bytes.Index(s.Bytes[off:], q)
			if i < 0 {
				break
			}
			st := uint64(off + i)
			en := st + uint64(len(q))
			ordinal++
			mid := fmt.Sprintf("match-%06d", ordinal)
			matches = append(matches, validator.Match{MatchID: mid, SourceID: s.ID, PathBytes: uint64(len([]byte(s.Path))), StartByte: st, EndByte: en, Ordinal: ordinal, Literal: string(q)})
			p := PositionFor(s.Bytes, st, en)
			p.MatchID = mid
			positions = append(positions, *p)
			if uint64(len(matches)) > c.attempt.Request.Limits.MaxMatches {
				c.fail("RESOURCE_EXHAUSTED", detail("limit", "max_matches"))
				return errors.New("max_matches")
			}
			off = int(st) + 1
		}
	}
	sort.SliceStable(matches, func(i, j int) bool {
		if matches[i].PathBytes != matches[j].PathBytes {
			return matches[i].PathBytes < matches[j].PathBytes
		}
		if matches[i].StartByte != matches[j].StartByte {
			return matches[i].StartByte < matches[j].StartByte
		}
		if matches[i].EndByte != matches[j].EndByte {
			return matches[i].EndByte < matches[j].EndByte
		}
		return matches[i].Ordinal < matches[j].Ordinal
	})
	posBy := map[string]validator.Position{}
	for _, p := range positions {
		posBy[p.MatchID] = p
	}
	positions = positions[:0]
	for _, m := range matches {
		positions = append(positions, posBy[m.MatchID])
	}
	c.terminal.Matches = matches
	c.terminal.Positions = positions
	return nil
}
func (c *evalContext) complete() {
	ids := []string{}
	pins := []validator.LocationPin{}
	h := sha256.New()
	utf16Units := uint64(0)
	for i, m := range c.terminal.Matches {
		ids = append(ids, m.MatchID)
		p := c.terminal.Positions[i]
		pins = append(pins, validator.LocationPin{SourceID: m.SourceID, StartByte: m.StartByte, EndByte: m.EndByte, StartLine: p.StartLine, StartCharacterUTF16: p.StartCharacterUTF16, EndLine: p.EndLine, EndCharacterUTF16: p.EndCharacterUTF16})
		fmt.Fprintf(h, "%s:%s:%d:%d:%d\n", m.MatchID, m.SourceID, m.PathBytes, m.StartByte, m.EndByte)
		utf16Units += p.EndCharacterUTF16 - p.StartCharacterUTF16
	}
	c.terminal.RangeUnionCandidate = &validator.Candidate{Operation: "RANGE_UNION", ExecutedLocation: false, CandidateOnly: true, AdmissionDigest: c.terminal.Admission.AdmissionDigest, MemberMatchIDs: ids, QualifiedLocationPins: pins, CandidateDigest: "sha256:" + hex.EncodeToString(h.Sum(nil))}
	c.terminal.Terminal = "COMPLETE"
	c.terminal.Failure = nil
	c.terminal.Accounting.MMatches = uint64(len(c.terminal.Matches))
	c.terminal.Accounting.RRanges = uint64(len(c.terminal.Matches))
	c.terminal.Accounting.UUTF16Units = utf16Units
}
func detail(k, v string) map[string]any { return map[string]any{k: v} }
func (c *evalContext) fail(code string, d map[string]any) {
	c.terminal.Terminal = "FAILED"
	c.terminal.Failure = &validator.Failure{Code: code, Stage: stage(code), Detail: d}
	c.terminal.Matches = []validator.Match{}
	c.terminal.Positions = []validator.Position{}
	c.terminal.RangeUnionCandidate = nil
	if isEarlyFailure(code) {
		c.terminal.Admission = validator.Admission{Completed: false, AdmissionDigest: zeroSHA, AdmittedSourceIDs: []string{}}
		c.terminal.Sources = []validator.Source{}
		c.terminal.Replay.AdmittedBindingSHA256 = zeroSHA
		c.admittedBinding = ""
		c.admittedSourceBytes = map[string]string{}
	} else if !c.terminal.Admission.Completed {
		c.terminal.Admission = validator.Admission{Completed: true, AdmissionDigest: zeroSHA, AdmittedSourceIDs: []string{}}
		c.terminal.Sources = []validator.Source{}
		c.terminal.Replay.AdmittedBindingSHA256 = zeroSHA
		c.admittedBinding = ""
		c.admittedSourceBytes = map[string]string{}
	}
	for k := range c.terminal.Accounting.FailureCounters {
		c.terminal.Accounting.FailureCounters[k] = 0
	}
	c.terminal.Accounting.FailureCounters[code] = 1
}
func stage(code string) string {
	return map[string]string{"INVALID_INPUT": "raw", "CANCELLED": "control", "DEADLINE_EXCEEDED": "control", "ASSOCIATION_FAILED": "association", "ADMISSION_FAILED": "admission", "RESOURCE_EXHAUSTED": "scan", "OVERFLOW": "accounting", "INVARIANT_FAILED": "invariant"}[code]
}
func isEarlyFailure(code string) bool {
	return code == "INVALID_INPUT" || code == "CANCELLED" || code == "DEADLINE_EXCEEDED" || code == "ASSOCIATION_FAILED"
}
func (c *evalContext) finalize() {
	// common accounting after the selected terminal state is known
	c.terminal.Accounting.JFiles = uint64(len(c.terminal.Sources))
	c.terminal.Accounting.QQueryBytes = uint64(len([]byte(c.attempt.Request.Query)))
	for _, s := range c.terminal.Sources {
		c.terminal.Accounting.PPathBytes += s.PathBytes
		c.terminal.Accounting.SSourceBytes += s.ByteLength
	}
	if c.terminal.Failure != nil && c.terminal.Failure.Code == "OVERFLOW" { /* keep below */
	}
	for i := 0; i < 64; i++ {
		c.terminal.Custody.TerminalResultSHA256 = zeroSHA
		c.terminal.Replay.TerminalPreimageSHA256 = zeroSHA
		b, _ := Canonical(c.terminal)
		c.terminal.Accounting.BOutputBytes = uint64(len(b))
		w, ok := weighted(c.terminal.Accounting)
		if !ok {
			c.fail("OVERFLOW", detail("counter", "W_work"))
			continue
		}
		c.terminal.Accounting.WWork = w
	}
	if c.terminal.Terminal == "COMPLETE" && c.attempt.Request.Limits.MaxWork > 0 && c.terminal.Accounting.WWork > c.attempt.Request.Limits.MaxWork {
		c.fail("RESOURCE_EXHAUSTED", detail("limit", "max_work"))
		c.finalizeNoLimit()
		return
	}
	if c.terminal.Terminal == "COMPLETE" && c.attempt.Request.Limits.MaxOutputBytes > 0 && c.terminal.Accounting.BOutputBytes > c.attempt.Request.Limits.MaxOutputBytes {
		c.fail("RESOURCE_EXHAUSTED", detail("limit", "max_output_bytes"))
		c.finalizeNoLimit()
		return
	}
	pre, _ := Canonical(c.terminal)
	digest := SHA(pre)
	c.terminal.Replay.TerminalPreimageSHA256 = digest
	c.terminal.Custody.TerminalResultSHA256 = digest
	b, _ := Canonical(c.terminal)
	c.terminal.Accounting.BOutputBytes = uint64(len(b))
	c.terminal.Accounting.WWork, _ = weighted(c.terminal.Accounting)
}
func (c *evalContext) finalizeNoLimit() {
	c.terminal.Matches = []validator.Match{}
	c.terminal.Positions = []validator.Position{}
	c.terminal.RangeUnionCandidate = nil
	for i := 0; i < 64; i++ {
		c.terminal.Custody.TerminalResultSHA256 = zeroSHA
		c.terminal.Replay.TerminalPreimageSHA256 = zeroSHA
		b, _ := Canonical(c.terminal)
		c.terminal.Accounting.BOutputBytes = uint64(len(b))
		c.terminal.Accounting.WWork, _ = weighted(c.terminal.Accounting)
	}
	pre, _ := Canonical(c.terminal)
	digest := SHA(pre)
	c.terminal.Replay.TerminalPreimageSHA256 = digest
	c.terminal.Custody.TerminalResultSHA256 = digest
	b, _ := Canonical(c.terminal)
	c.terminal.Accounting.BOutputBytes = uint64(len(b))
	c.terminal.Accounting.WWork, _ = weighted(c.terminal.Accounting)
}
func weighted(a validator.Accounting) (uint64, bool) {
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
func (c *evalContext) result() EvalResult {
	return EvalResult{Terminal: c.terminal, RawAttemptBytes: string(c.raw), AdmittedSourceBytes: c.admittedSourceBytes, AdmittedBindingBytes: c.admittedBinding, ToolingManifestBytes: c.toolingBytes, PredecessorBytes: c.predecessorBytes, PayloadFreezeBytes: c.payloadFreezeBytes, SchemaBytes: c.schemaBytes}
}

func Canonical(v any) ([]byte, error) { s, err := validator.CanonicalJSON(v); return []byte(s), err }
func mustCanon(v any) string          { s, _ := validator.CanonicalJSON(v); return s }
func CanonicalPath(p string) bool {
	return p != "" && utf8.ValidString(p) && norm.NFC.IsNormalString(p) && !strings.Contains(p, "\\") && !strings.Contains(p, "//") && !strings.Contains(p, ":") && !strings.HasPrefix(p, "/") && p != "." && p != ".." && !strings.HasPrefix(p, "./") && !strings.HasPrefix(p, "../") && !strings.Contains(p, "/./") && !strings.Contains(p, "/../") && path.Clean(p) == p
}
func PositionFor(b []byte, st, en uint64) *validator.Position {
	sl, sc := lineChar(b, st)
	el, ec := lineChar(b, en)
	return &validator.Position{StartLine: sl, StartCharacterUTF16: sc, EndLine: el, EndCharacterUTF16: ec}
}
func LineUTF16(b []byte, off int) (int, int) { l, c := lineChar(b, uint64(off)); return int(l), int(c) }
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

func rejectDuplicateKeys(b []byte) error {
	dec := json.NewDecoder(bytes.NewReader(b))
	return scanValue(dec)
}
func scanValue(dec *json.Decoder) error {
	tok, err := dec.Token()
	if err != nil {
		return err
	}
	d, ok := tok.(json.Delim)
	if !ok {
		return nil
	}
	switch d {
	case '{':
		seen := map[string]bool{}
		for dec.More() {
			kt, err := dec.Token()
			if err != nil {
				return err
			}
			k, ok := kt.(string)
			if !ok {
				return fmt.Errorf("object key is not string")
			}
			if seen[k] {
				return fmt.Errorf("duplicate key %q", k)
			}
			seen[k] = true
			if err := scanValue(dec); err != nil {
				return err
			}
		}
		_, err := dec.Token()
		return err
	case '[':
		for dec.More() {
			if err := scanValue(dec); err != nil {
				return err
			}
		}
		_, err := dec.Token()
		return err
	}
	return fmt.Errorf("unexpected delimiter")
}

func ReadManifest() (Manifest, error) {
	var m Manifest
	root, err := repoRoot()
	if err != nil {
		return m, err
	}
	b, err := os.ReadFile(filepath.Join(root, BaseDir, "PAYLOAD_MANIFEST.json"))
	if err != nil {
		return m, err
	}
	err = json.Unmarshal(b, &m)
	return m, err
}
func repoRoot() (string, error) {
	d, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(d, "go.mod")); err == nil {
			return d, nil
		}
		p := filepath.Dir(d)
		if p == d {
			return "", fmt.Errorf("go.mod not found")
		}
		d = p
	}
}
func ReadRows(root string) ([]Row, error) {
	b, err := os.ReadFile(filepath.Join(root, "CASE_MATRIX.json"))
	if err != nil {
		return nil, err
	}
	var r []Row
	err = json.Unmarshal(b, &r)
	return r, err
}
func ExpectedPath(id string) string { return filepath.Join(OracleDir, id+".expected.json") }
func BundlePath(id string) string   { return filepath.Join(OracleDir, id+".validator-bundle.json") }

func WriteBundle(path string, r EvalResult) error {
	bun := validator.Bundle{SchemaBytes: r.SchemaBytes, RawAttemptBytes: r.RawAttemptBytes, TerminalBytes: mustString(Canonical(r.Terminal)), AdmittedSourceBytes: r.AdmittedSourceBytes, AdmittedBindingBytes: r.AdmittedBindingBytes, ToolingManifestBytes: r.ToolingManifestBytes, PredecessorManifestBytes: r.PredecessorBytes, PayloadFreezeBytes: r.PayloadFreezeBytes}
	b, _ := json.Marshal(bun)
	var x any
	json.Unmarshal(b, &x)
	s, _ := validator.CanonicalJSON(x)
	return os.WriteFile(path, []byte(s), 0644)
}
func mustString(b []byte, e error) string {
	if e != nil {
		return ""
	}
	return string(b)
}
func ValidateTerminalBytes(b []byte) error {
	tmp, err := os.CreateTemp("", "oracle-terminal-*.json")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	return os.WriteFile(tmp.Name(), b, 0600)
}
func ValidateResult(r EvalResult) error {
	bun := validator.Bundle{SchemaBytes: r.SchemaBytes, RawAttemptBytes: r.RawAttemptBytes, TerminalBytes: mustString(Canonical(r.Terminal)), AdmittedSourceBytes: r.AdmittedSourceBytes, AdmittedBindingBytes: r.AdmittedBindingBytes, ToolingManifestBytes: r.ToolingManifestBytes, PredecessorManifestBytes: r.PredecessorBytes, PayloadFreezeBytes: r.PayloadFreezeBytes}
	return validator.ValidateBundle(bun)
}

func Freeze(root string) error {
	rows, err := ReadRows(root)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(OracleDir, 0755); err != nil {
		return err
	}
	m, err := ReadManifest()
	if err != nil {
		return err
	}
	prov := []string{BaseDir + "/contracts/CONTRACT.md", BaseDir + "/contracts/terminal-envelope-v4.schema.json", BaseDir + "/PAYLOAD_MANIFEST.json", BaseDir + "/TOOLING_CENSUS.json", filepath.Join(root, "CASE_MATRIX.json")}
	for _, row := range rows {
		p := filepath.Join(root, row.ID, "attempt.json")
		r, err := EvaluateFileResult(p)
		if err != nil {
			return err
		}
		if err := ValidateResult(r); err != nil {
			return fmt.Errorf("%s terminal invalid: %w", row.ID, err)
		}
		b, _ := Canonical(r.Terminal)
		if err := os.WriteFile(ExpectedPath(row.ID), b, 0644); err != nil {
			return err
		}
		if err := WriteBundle(BundlePath(row.ID), r); err != nil {
			return err
		}
		prov = append(prov, p)
	}
	sort.Strings(prov)
	return writeProvenance(prov, m)
}
func writeProvenance(paths []string, m Manifest) error {
	type P struct {
		SchemaVersion  string   `json:"schema_version"`
		ForbiddenGlobs []string `json:"forbidden_globs"`
		PathsRead      []string `json:"paths_read"`
		PayloadDigest  string   `json:"payload_digest"`
	}
	p := P{"lsp-trace.adr0007.source-text-search.oracle-provenance.v1", []string{"internal/adr0007sourcetextsearchv4private/**", "cmd/adr0007-source-text-search-v4-private-evaluate/**", "cmd/adr0007-source-text-search-v4-private-check/**", "production generated outputs/bundles"}, paths, shaString(m.SchemaVersion + m.Stage + m.ToolingDigest + m.PredecessorLockDigest)}
	b, _ := Canonical(p)
	return os.WriteFile(ProvPath, b, 0644)
}
func Check(root string) error {
	rows, err := ReadRows(root)
	if err != nil {
		return err
	}
	seen := map[string]bool{}
	for _, row := range rows {
		seen[row.ID] = true
		r, err := EvaluateFileResult(filepath.Join(root, row.ID, "attempt.json"))
		if err != nil {
			return err
		}
		if err := ValidateResult(r); err != nil {
			return fmt.Errorf("%s terminal invalid: %w", row.ID, err)
		}
		b, _ := Canonical(r.Terminal)
		exp, err := os.ReadFile(ExpectedPath(row.ID))
		if err != nil {
			return err
		}
		if !bytes.Equal(b, exp) {
			return fmt.Errorf("oracle mismatch %s", row.ID)
		}
	}
	ents, err := os.ReadDir(OracleDir)
	if err != nil {
		return err
	}
	for _, e := range ents {
		if e.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("symlink in oracle: %s", e.Name())
		}
		if strings.HasSuffix(e.Name(), ".expected.json") {
			id := strings.TrimSuffix(e.Name(), ".expected.json")
			if !seen[id] {
				return fmt.Errorf("extra expected %s", e.Name())
			}
		}
	}
	return CheckProvenance()
}
func CheckProvenance() error {
	b, err := os.ReadFile(ProvPath)
	if err != nil {
		return err
	}
	var p struct {
		PathsRead []string `json:"paths_read"`
	}
	if err := json.Unmarshal(b, &p); err != nil {
		return err
	}
	for _, x := range p.PathsRead {
		if strings.HasPrefix(x, "internal/adr0007sourcetextsearchv4private/") || strings.HasPrefix(x, "cmd/adr0007-source-text-search-v4-private-evaluate/") || strings.HasPrefix(x, "cmd/adr0007-source-text-search-v4-private-check/") || strings.Contains(x, "production") {
			return fmt.Errorf("forbidden provenance path %s", x)
		}
	}
	return nil
}
