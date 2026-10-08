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
	"strings"
	"unicode/utf8"

	admit "lsp-trace/docs/pilot/adr0007/source-text-search-v3/pinned/sourceadmissionv2"
)

var ErrTerminal = errors.New("terminal")

func Digest(b []byte) string { h := sha256.Sum256(b); return "sha256:" + hex.EncodeToString(h[:]) }
func FailureCounters() map[string]uint64 {
	return map[string]uint64{"INVALID_INPUT": 0, "CANCELLED": 0, "DEADLINE_EXCEEDED": 0, "ASSOCIATION_FAILED": 0, "ADMISSION_FAILED": 0, "RESOURCE_EXHAUSTED": 0, "OVERFLOW": 0, "INVARIANT_FAILED": 0}
}

func StrictParseAttempt(raw []byte) (Attempt, string) {
	if err := validateJSON(bytes.NewReader(raw)); err != nil {
		msg := err.Error()
		if strings.Contains(msg, "duplicate field") {
			return Attempt{}, "DUPLICATE_FIELD"
		}
		if strings.Contains(msg, "trailing") {
			return Attempt{}, "TRAILING_DATA"
		}
		return Attempt{}, "JSON_SYNTAX"
	}
	if code := validateKnownJSON(raw); code != "" {
		return Attempt{}, code
	}
	var a Attempt
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	if err := dec.Decode(&a); err != nil {
		return Attempt{}, "JSON_SYNTAX"
	}
	if dec.More() {
		return Attempt{}, "TRAILING_DATA"
	}
	return a, ""
}

func validateKnownJSON(raw []byte) string {
	var v any
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	if err := dec.Decode(&v); err != nil {
		return "JSON_SYNTAX"
	}
	if _, err := dec.Token(); err != io.EOF {
		return "TRAILING_DATA"
	}
	if code := checkKnown(v, "attempt"); code != "" {
		return code
	}
	return ""
}

func checkKnown(v any, ctx string) string {
	m, ok := v.(map[string]any)
	if !ok {
		if a, ok := v.([]any); ok {
			for _, e := range a {
				if code := checkKnown(e, ctx); code != "" {
					return code
				}
			}
		}
		return ""
	}
	allowed := map[string]bool{}
	for _, k := range knownFields(ctx) {
		allowed[k] = true
	}
	for k, val := range m {
		if !allowed[k] {
			return "UNKNOWN_FIELD"
		}
		next := ctx
		switch k {
		case "request":
			next = "request"
		case "policy":
			next = "policy"
		case "limits":
			next = "limits"
		case "location_pin":
			next = "location_pin"
		case "source_admission_pin":
			next = "source_admission_pin"
		case "execution_control":
			next = "execution_control"
		case "observations":
			next = "observation"
		case "sources":
			next = "source_ref"
		case "source_inputs":
			next = "source_input"
		case "external_freeze_binding":
			next = "external_freeze_binding"
		}
		if code := checkKnown(val, next); code != "" {
			return code
		}
	}
	return ""
}

func knownFields(ctx string) []string {
	switch ctx {
	case "attempt":
		return []string{"schema_version", "attempt_id", "request", "execution_control", "source_inputs", "external_freeze_binding"}
	case "request":
		return []string{"schema_version", "query", "sources", "policy", "limits", "location_pin"}
	case "policy":
		return []string{"schema_version", "literal_mode", "allow_regex", "allow_fuzzy", "allow_token", "allow_rank", "allow_model", "allow_backend_semantics"}
	case "limits":
		return []string{"schema_version", "max_files", "max_matches", "max_work", "max_output_bytes", "max_source_bytes", "max_total_bytes", "max_path_bytes"}
	case "location_pin":
		return []string{"schema_version", "operation", "executedLocation", "design_commit", "design_root_sha256", "execution_commit", "seal_commit", "final_seal_sha256", "source_admission_pin", "complete_location_pins"}
	case "source_admission_pin":
		return []string{"repository_commit", "path", "bytes", "sha256", "git_blob_sha1", "symbols"}
	case "execution_control":
		return []string{"schema_version", "observations"}
	case "observation":
		return []string{"poll_index", "cancelled", "deadline_expired"}
	case "source_ref":
		return []string{"ordinal", "path", "revision", "file_digest", "object_digest"}
	case "source_input":
		return []string{"schema_version", "ordinal", "path", "revision", "file_digest", "object_digest", "bytes_base64"}
	case "external_freeze_binding":
		return []string{"schema_version", "mode", "freeze_root_sha256", "design_identity_sha256"}
	default:
		return nil
	}
}

func validateJSON(r io.Reader) error {
	dec := json.NewDecoder(r)
	dec.UseNumber()
	if err := walk(dec); err != nil {
		return err
	}
	if tok, err := dec.Token(); err != io.EOF {
		if err != nil {
			return err
		}
		return fmt.Errorf("trailing data after %v", tok)
	}
	return nil
}

func walk(dec *json.Decoder) error {
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
				return fmt.Errorf("object key not string")
			}
			if seen[k] {
				return fmt.Errorf("duplicate field %s", k)
			}
			seen[k] = true
			if err := walk(dec); err != nil {
				return err
			}
		}
		_, err := dec.Token()
		return err
	case '[':
		for dec.More() {
			if err := walk(dec); err != nil {
				return err
			}
		}
		_, err := dec.Token()
		return err
	default:
		return fmt.Errorf("unexpected delimiter")
	}
}

func EvaluateRaw(raw []byte) TerminalResult {
	a, perr := StrictParseAttempt(raw)
	if perr != "" {
		return sealFailure(Attempt{AttemptID: "parse-failure"}, Accounting{FailureCounters: FailureCounters()}, raw, nil, "INVALID_INPUT", map[string]any{"parse": perr})
	}
	return Evaluate(a, raw)
}
func Evaluate(a Attempt, raw []byte) TerminalResult { r, _, _ := eval(a, raw); return r }

func base(a Attempt, acc Accounting) TerminalResult {
	acc.SchemaVersion = AccountingSchema
	return TerminalResult{SchemaVersion: TerminalSchema, AttemptID: a.AttemptID, TerminalSequence: 0, Outcome: "COMPLETE", Authority: 0, Accepted: false, Completeness: "UNKNOWN", FeatureIdentity: "UNRESOLVED", Matches: []Match{}, Accounting: acc, Custody: Custody{SchemaVersion: CustodySchema, AttemptID: a.AttemptID, TerminalSequence: 0, TerminalCount: 1}, Replay: Replay{SchemaVersion: ReplaySchema}}
}
func sealFailure(a Attempt, acc Accounting, raw []byte, adm *AdmissionRecord, code string, detail map[string]any) TerminalResult {
	if acc.FailureCounters == nil {
		acc.FailureCounters = FailureCounters()
	}
	acc.FailureCounters[code] = 1
	tr := base(a, acc)
	tr.Outcome = "FAILED"
	tr.Failure = &Failure{Code: code, Detail: detail}
	tr.Admission = adm
	seal(&tr, a, raw, adm)
	return tr
}

func eval(a Attempt, raw []byte) (TerminalResult, []byte, error) {
	acc := Accounting{FailureCounters: FailureCounters()}
	if err := validateAttempt(a); err != "" {
		tr := sealFailure(a, acc, raw, nil, "INVALID_INPUT", map[string]any{"field": err})
		return tr, Canon(tr), ErrTerminal
	}
	obs := map[uint64]Observation{}
	var last uint64
	for i, o := range a.ExecutionControl.Observations {
		if i > 0 && o.PollIndex <= last {
			tr := sealFailure(a, acc, raw, nil, "INVALID_INPUT", map[string]any{"field": "execution_control.observations"})
			return tr, Canon(tr), ErrTerminal
		}
		if obs[o.PollIndex].Cancelled || obs[o.PollIndex].DeadlineExpired {
			tr := sealFailure(a, acc, raw, nil, "INVALID_INPUT", map[string]any{"field": "duplicate poll"})
			return tr, Canon(tr), ErrTerminal
		}
		last = o.PollIndex
		obs[o.PollIndex] = o
	}
	poll := uint64(0)
	check := func() (string, bool) {
		o, ok := obs[poll]
		poll++
		if !ok {
			return "", false
		}
		if o.DeadlineExpired {
			return "DEADLINE_EXCEEDED", true
		}
		if o.Cancelled {
			return "CANCELLED", true
		}
		return "", false
	}
	if c, yes := check(); yes {
		tr := sealFailure(a, acc, raw, nil, c, map[string]any{"poll": 0})
		return tr, Canon(tr), ErrTerminal
	}
	if uint64(len(a.Request.Sources)) > a.Request.Limits.MaxFiles {
		tr := sealFailure(a, acc, raw, nil, "RESOURCE_EXHAUSTED", map[string]any{"limit": "max_files"})
		return tr, Canon(tr), ErrTerminal
	}
	byOrd := map[uint64]SourceInput{}
	for _, in := range a.SourceInputs {
		if _, ok := byOrd[in.Ordinal]; ok {
			tr := sealFailure(a, acc, raw, nil, "ASSOCIATION_FAILED", map[string]any{"reason": "duplicate ordinal"})
			return tr, Canon(tr), ErrTerminal
		}
		byOrd[in.Ordinal] = in
	}
	type sf struct {
		ref SourceRef
		in  SourceInput
		b   []byte
	}
	files := []sf{}
	pathBytes := uint64(0)
	totalBytes := uint64(0)
	for _, s := range a.Request.Sources {
		in, ok := byOrd[s.Ordinal]
		if !ok || in.Path != s.Path || in.Revision != s.Revision || in.FileDigest != s.FileDigest || in.ObjectDigest != s.ObjectDigest {
			tr := sealFailure(a, acc, raw, nil, "ASSOCIATION_FAILED", map[string]any{"ordinal": s.Ordinal})
			return tr, Canon(tr), ErrTerminal
		}
		b, e := base64.StdEncoding.DecodeString(in.BytesBase64)
		if e != nil {
			tr := sealFailure(a, acc, raw, nil, "ADMISSION_FAILED", map[string]any{"path": s.Path, "reason": "base64"})
			return tr, Canon(tr), ErrTerminal
		}
		pathBytes += uint64(len([]byte(s.Path)))
		if pathBytes > a.Request.Limits.MaxPathBytes {
			tr := sealFailure(a, acc, raw, nil, "RESOURCE_EXHAUSTED", map[string]any{"limit": "max_path_bytes"})
			return tr, Canon(tr), ErrTerminal
		}
		if uint64(len(b)) > a.Request.Limits.MaxSourceBytes {
			tr := sealFailure(a, acc, raw, nil, "RESOURCE_EXHAUSTED", map[string]any{"limit": "max_source_bytes"})
			return tr, Canon(tr), ErrTerminal
		}
		totalBytes += uint64(len(b))
		if totalBytes > a.Request.Limits.MaxTotalBytes {
			tr := sealFailure(a, acc, raw, nil, "RESOURCE_EXHAUSTED", map[string]any{"limit": "max_total_bytes"})
			return tr, Canon(tr), ErrTerminal
		}
		files = append(files, sf{s, in, b})
	}
	if len(files) != len(byOrd) {
		tr := sealFailure(a, acc, raw, nil, "ASSOCIATION_FAILED", map[string]any{"reason": "extra source input"})
		return tr, Canon(tr), ErrTerminal
	}
	if c, yes := check(); yes {
		tr := sealFailure(a, acc, raw, nil, c, map[string]any{"poll": "pre-admission"})
		return tr, Canon(tr), ErrTerminal
	}
	selected := make([]admit.SelectedSource, 0, len(files))
	for _, f := range files {
		selected = append(selected, admit.SelectedSource{Path: f.ref.Path, Revision: f.ref.Revision, FileDigest: f.ref.FileDigest, ObjectDigest: f.ref.ObjectDigest, Bytes: f.b})
	}
	ar := admit.Admit(selected, admit.Limits{MaxSources: int(a.Request.Limits.MaxFiles), MaxSourceBytes: int(a.Request.Limits.MaxSourceBytes), MaxTotalBytes: int(a.Request.Limits.MaxTotalBytes)})
	if ar.Outcome != admit.Complete {
		code := "ADMISSION_FAILED"
		if ar.Outcome == admit.ResourceLimit {
			code = "RESOURCE_EXHAUSTED"
		}
		tr := sealFailure(a, acc, raw, nil, code, map[string]any{"admission_outcome": string(ar.Outcome), "detail": ar.Detail})
		return tr, Canon(tr), ErrTerminal
	}
	adm := AdmissionRecord{SchemaVersion: AdmissionRecordSchema, AdmissionSchema: admit.Schema, AdmissionDigest: ar.Binding.AdmissionDigest}
	orderedBytes := map[string][]byte{}
	for _, s := range ar.Binding.Sources {
		t := SourceTuple{Ordinal: ordinalFor(a.Request.Sources, s.Path), Path: s.Path, Revision: s.Revision, FileDigest: s.FileDigest, ObjectDigest: s.ObjectDigest, SourceByteLength: uint64(len(s.Bytes))}
		adm.OrderedSources = append(adm.OrderedSources, t)
		orderedBytes[s.Path] = s.Bytes
		acc.JFiles++
		acc.PPathBytes += uint64(len([]byte(s.Path)))
		acc.SSourceBytes += uint64(len(s.Bytes))
	}
	if c, yes := check(); yes {
		tr := sealFailure(a, acc, raw, &adm, c, map[string]any{"poll": "admission"})
		return tr, Canon(tr), ErrTerminal
	}
	q := []byte(a.Request.Query)
	acc.QQueryBytes = uint64(len(q))
	matches := []Match{}
	for _, src := range adm.OrderedSources {
		if c, yes := check(); yes {
			tr := sealFailure(a, acc, raw, &adm, c, map[string]any{"poll": "file"})
			return tr, Canon(tr), ErrTerminal
		}
		b := orderedBytes[src.Path]
		for i := 0; i+len(q) <= len(b); i++ {
			if c, yes := check(); yes {
				tr := sealFailure(a, acc, raw, &adm, c, map[string]any{"poll": "byte"})
				return tr, Canon(tr), ErrTerminal
			}
			acc.TScannedTuples++
			if bytes.Equal(b[i:i+len(q)], q) {
				if acc.MMatches+1 > a.Request.Limits.MaxMatches {
					tr := sealFailure(a, acc, raw, &adm, "RESOURCE_EXHAUSTED", map[string]any{"limit": "max_matches"})
					return tr, Canon(tr), ErrTerminal
				}
				ps := position(b, i)
				pe := position(b, i+len(q))
				m := Match{SchemaVersion: MatchSchema, Source: src, ByteRange: ByteRange{uint64(i), uint64(i + len(q))}, LSPUTF16Range: LSPRange{ps, pe}, Literal: a.Request.Query}
				matches = append(matches, m)
				acc.MMatches++
				acc.RRanges++
				acc.UUTF16Units += utf16Units(b[i : i+len(q)])
				if c, yes := check(); yes {
					tr := sealFailure(a, acc, raw, &adm, c, map[string]any{"poll": "match"})
					return tr, Canon(tr), ErrTerminal
				}
			}
		}
	}
	tr := base(a, acc)
	tr.Admission = &adm
	tr.Matches = matches
	tr.RangeUnionCandidate = &RangeUnionCandidate{SchemaVersion: RangeUnionSchema, Operation: "RANGE_UNION", ExecutedLocation: false, CandidateOnly: true, DesignCommit: a.Request.LocationPin.DesignCommit, DesignRootSha256: a.Request.LocationPin.DesignRootSha256, ExecutionCommit: a.Request.LocationPin.ExecutionCommit, SealCommit: a.Request.LocationPin.SealCommit, FinalSealSha256: a.Request.LocationPin.FinalSealSha256, AdmissionDigest: adm.AdmissionDigest, Members: matches}
	tr.RangeUnionCandidate.CandidateDigest = Digest(Canon(*tr.RangeUnionCandidate))
	seal(&tr, a, raw, &adm)
	return tr, Canon(tr), nil
}

func validateAttempt(a Attempt) string {
	if a.SchemaVersion != AttemptSchema {
		return "schema_version"
	}
	if !strings.HasPrefix(a.AttemptID, "attempt-") {
		return "attempt_id"
	}
	r := a.Request
	if r.SchemaVersion != RequestSchema || r.Query == "" || !utf8.ValidString(r.Query) || len(r.Sources) == 0 {
		return "request"
	}
	if r.Policy.SchemaVersion != PolicySchema || r.Policy.LiteralMode != "byte-literal" || r.Policy.AllowRegex || r.Policy.AllowFuzzy || r.Policy.AllowToken || r.Policy.AllowRank || r.Policy.AllowModel || r.Policy.AllowBackendSemantics {
		return "policy"
	}
	if r.Limits.SchemaVersion != LimitsSchema || r.Limits.MaxFiles == 0 || r.Limits.MaxMatches == 0 || r.Limits.MaxWork == 0 || r.Limits.MaxOutputBytes == 0 || r.Limits.MaxSourceBytes == 0 || r.Limits.MaxTotalBytes == 0 || r.Limits.MaxPathBytes == 0 {
		return "limits"
	}
	lp := r.LocationPin
	if lp.SchemaVersion != LocationPinSchema || lp.Operation != "RANGE_UNION" || lp.ExecutedLocation {
		return "location_pin"
	}
	pin := lp.SourceAdmissionPin
	if pin.RepositoryCommit != "af2ce89321afc94c937636f841bb98b8977b6496" || pin.Path != "internal/sourceadmissionv2/admission.go" || pin.Bytes != 3519 || pin.SHA256 != "sha256:da74770d5b36f63e6f1265ba78e2404e13f2d1f1451a4e7aa405f448e47fe7da" || pin.GitBlobSHA1 != "4953fab89d911e2352fa6c2253497c07c8e9a777" || strings.Join(pin.Symbols, ",") != "Admit,CanonicalPath,Digest" {
		return "source_admission_pin"
	}
	if a.ExecutionControl.SchemaVersion != ExecutionControlSchema {
		return "execution_control"
	}
	for _, in := range a.SourceInputs {
		if in.SchemaVersion != SourceInputSchema {
			return "source_inputs.schema_version"
		}
	}
	if a.ExternalFreezeBinding != nil {
		if a.ExternalFreezeBinding.SchemaVersion != FreezeBindingSchema || a.ExternalFreezeBinding.Mode == "" || strings.Contains(a.ExternalFreezeBinding.FreezeRootSha256, "pending") || strings.Contains(a.ExternalFreezeBinding.DesignIdentitySHA256, "pending") {
			return "external_freeze_binding"
		}
	}
	return ""
}
func ordinalFor(s []SourceRef, path string) uint64 {
	for _, x := range s {
		if x.Path == path {
			return x.Ordinal
		}
	}
	return 0
}
func seal(tr *TerminalResult, a Attempt, raw []byte, adm *AdmissionRecord) {
	if raw == nil {
		raw = Canon(a)
	}
	for i := 0; i < 64; i++ {
		tr.Accounting.BOutputBytes = uint64(len(Canon(tr)))
		w, ok := Work(tr.Accounting)
		if !ok {
			tr.Outcome = "FAILED"
			tr.Failure = &Failure{Code: "OVERFLOW", Detail: map[string]any{}}
			tr.Accounting.FailureCounters["OVERFLOW"] = 1
		}
		tr.Accounting.WWork = w
		n := uint64(len(Canon(tr)))
		if n == tr.Accounting.BOutputBytes {
			break
		}
		tr.Accounting.BOutputBytes = n
	}
	if tr.Accounting.WWork > a.Request.Limits.MaxWork && a.Request.Limits.MaxWork > 0 {
		tr.Outcome = "FAILED"
		tr.Failure = &Failure{Code: "RESOURCE_EXHAUSTED", Detail: map[string]any{"limit": "max_work"}}
		tr.Accounting.FailureCounters["RESOURCE_EXHAUSTED"] = 1
	}
	if tr.Accounting.BOutputBytes > a.Request.Limits.MaxOutputBytes && a.Request.Limits.MaxOutputBytes > 0 {
		tr.Outcome = "FAILED"
		tr.Failure = &Failure{Code: "RESOURCE_EXHAUSTED", Detail: map[string]any{"limit": "max_output_bytes"}}
		tr.Accounting.FailureCounters["RESOURCE_EXHAUSTED"] = 1
	}
	tr.Replay.CanonicalAttemptSHA256 = Digest(raw)
	if adm != nil {
		tr.Replay.AdmittedBindingSHA256 = Digest(Canon(adm))
	}
	tr.Replay.ToolingIdentitySHA256 = ToolDigest()
	tr.Replay.PredecessorLockSHA256 = PredecessorDigest()
	if a.ExternalFreezeBinding != nil {
		tr.Replay.FreezeBindingSHA256 = Digest(Canon(*a.ExternalFreezeBinding))
	} else {
		tr.Replay.FreezeBindingSHA256 = Digest([]byte("UNFROZEN-PROSPECTIVE-V4-DESIGN-IDENTITY"))
	}
	pre := *tr
	pre.Custody.TerminalResultSHA256 = "sha256:" + strings.Repeat("0", 64)
	tr.Replay.TerminalPreimageSHA256 = Digest(Canon(pre))
	tr.Custody.TerminalResultSHA256 = Digest(Canon(pre))
}
func Work(a Accounting) (uint64, bool) {
	w := uint64(50)
	for _, tv := range []struct{ c, v uint64 }{{3, a.JFiles}, {5, a.QQueryBytes}, {7, a.PPathBytes}, {1, a.SSourceBytes}, {11, a.TScannedTuples}, {13, a.MMatches}, {17, a.RRanges}, {19, a.UUTF16Units}, {31, a.BOutputBytes}} {
		if tv.v != 0 && tv.c > math.MaxUint64/tv.v {
			return 0, false
		}
		add := tv.c * tv.v
		if math.MaxUint64-w < add {
			return 0, false
		}
		w += add
	}
	return w, true
}
func ToolDigest() string {
	return Digest([]byte("adr0007-v4-private-production:types.go+search.go+cmd/evaluate"))
}
func PredecessorDigest() string {
	return Digest([]byte("adr0007-v3-private-predecessor:source-text-search-v3-private"))
}
func utf16Units(b []byte) uint64 {
	var u uint64
	for len(b) > 0 {
		r, s := utf8.DecodeRune(b)
		if r > 0xffff {
			u += 2
		} else {
			u++
		}
		b = b[s:]
	}
	return u
}
func position(b []byte, off int) Pos {
	var line, ch uint64
	for i := 0; i < off; {
		if b[i] == '\r' {
			line++
			ch = 0
			if i+1 < off && b[i+1] == '\n' {
				i += 2
			} else {
				i++
			}
			continue
		}
		if b[i] == '\n' {
			line++
			ch = 0
			i++
			continue
		}
		r, s := utf8.DecodeRune(b[i:])
		if r > 0xffff {
			ch += 2
		} else {
			ch++
		}
		i += s
	}
	return Pos{line, ch}
}
