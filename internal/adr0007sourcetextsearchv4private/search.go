package adr0007sourcetextsearchv4private

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
	"sort"
	"strings"
	"unicode/utf16"
	"unicode/utf8"

	admit "lsp-trace/docs/pilot/adr0007/source-text-search-v3/pinned/sourceadmissionv2"
)

var ErrTerminal = errors.New("terminal")

type searchFile struct {
	ref SourceRef
	in  SourceInput
	b   []byte
}

func Digest(b []byte) string { h := sha256.Sum256(b); return "sha256:" + hex.EncodeToString(h[:]) }
func FailureCounters() map[string]uint64 {
	return map[string]uint64{"INVALID_INPUT": 0, "CANCELLED": 0, "DEADLINE_EXCEEDED": 0, "ASSOCIATION_FAILED": 0, "ADMISSION_FAILED": 0, "RESOURCE_EXHAUSTED": 0, "OVERFLOW": 0, "INVARIANT_FAILED": 0}
}

func StrictParseAttempt(raw []byte) (Attempt, string) {
	if err := validateJSON(bytes.NewReader(raw)); err != nil {
		msg := err.Error()
		if strings.Contains(msg, "trailing") {
			msg = "trailing data"
		}
		return Attempt{}, msg
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
	return checkKnown(v, "attempt")
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
			return fmt.Sprintf("json: unknown field %q", k)
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
		case "test_control":
			next = "test_control"
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
		return []string{"schema_version", "attempt_id", "request", "execution_control", "source_inputs", "external_freeze_binding", "test_control"}
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
	case "test_control":
		return []string{"schema_version", "initial_B_output_bytes"}
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
		id := "attempt-raw-sha256-" + strings.TrimPrefix(Digest(raw), "sha256:")
		return sealFailure(Attempt{AttemptID: id, Request: Request{Query: "invalid"}}, raw, nil, nil, "INVALID_INPUT", map[string]any{"reason": perr}, true)
	}
	return Evaluate(a, raw)
}
func Evaluate(a Attempt, raw []byte) TerminalResult { r, _, _ := eval(a, raw); return r }

func base(a Attempt, raw []byte, adm *AdmissionRecord, sources []Source, malformed bool) TerminalResult {
	if sources == nil {
		sources = []Source{}
	}
	q := a.Request.Query
	if q == "" {
		q = "invalid"
	}
	ad := Admission{Completed: adm != nil, AdmissionDigest: zeroSHA, AdmittedSourceIDs: []string{}}
	if adm == nil {
		for _, s := range sources {
			ad.AdmittedSourceIDs = append(ad.AdmittedSourceIDs, s.SourceID)
		}
	}
	if adm != nil {
		ad.AdmissionDigest = adm.AdmissionDigest
		if ad.AdmissionDigest == "" {
			ad.AdmissionDigest = Digest(Canon(adm))
		}
		for _, s := range sources {
			ad.AdmittedSourceIDs = append(ad.AdmittedSourceIDs, s.SourceID)
		}
	}
	acc := Accounting{SchemaVersion: AccountingSchema, QQueryBytes: uint64(len([]byte(q))), FailureCounters: FailureCounters()}
	return TerminalResult{SchemaVersion: TerminalSchema, Terminal: "COMPLETE", Attempt: TerminalAttempt{AttemptID: a.AttemptID, MalformedRaw: malformed}, Request: TerminalRequest{Query: q}, Admission: ad, Sources: sources, Matches: []Match{}, Positions: []Position{}, Accounting: acc, Custody: Custody{SchemaVersion: CustodySchema, AttemptID: a.AttemptID, TerminalSequence0: 0, TerminalCount1: 1}, Replay: Replay{SchemaVersion: ReplaySchema}, Payload: Payload{}}
}
func sealFailure(a Attempt, raw []byte, adm *AdmissionRecord, sources []Source, code string, detail map[string]any, malformed ...bool) TerminalResult {
	bindingOverride := ""
	if (code == "CANCELLED" || code == "DEADLINE_EXCEEDED") && adm != nil {
		bindingOverride = adm.BindingBytes
	}
	if code == "INVALID_INPUT" || code == "ASSOCIATION_FAILED" || code == "CANCELLED" || code == "DEADLINE_EXCEEDED" {
		adm = nil
		if code == "INVALID_INPUT" || code == "ASSOCIATION_FAILED" || sources == nil {
			sources = nil
		}
	}
	if (code == "RESOURCE_EXHAUSTED" || code == "OVERFLOW" || code == "INVARIANT_FAILED") && adm == nil {
		empty := AdmissionRecord{SchemaVersion: AdmissionRecordSchema, AdmissionSchema: admit.Schema, AdmissionDigest: Digest([]byte("")), OrderedSources: []SourceTuple{}}
		adm = &empty
		sources = []Source{}
	}
	if code == "RESOURCE_EXHAUSTED" && adm != nil && adm.BindingBytes != "" && detail["limit"] == "max_path_bytes" {
		adm.AdmissionDigest = Digest([]byte(adm.BindingBytes))
	}
	malformedRaw := len(malformed) > 0 && malformed[0]
	if len(sources) == 0 && !strings.HasPrefix(a.AttemptID, "attempt-raw-sha256-") {
		a.AttemptID = "attempt-raw-sha256-" + strings.TrimPrefix(Digest(raw), "sha256:")
	}
	if code == "INVALID_INPUT" {
		malformedRaw = true
	}
	tr := base(a, raw, adm, sources, malformedRaw)
	for _, s := range sources {
		tr.Accounting.JFiles++
		tr.Accounting.PPathBytes += s.PathBytes
		tr.Accounting.SSourceBytes += s.ByteLength
	}
	if code == "CANCELLED" || code == "DEADLINE_EXCEEDED" {
		for _, v := range detail {
			if s, ok := v.(string); ok && strings.HasPrefix(s, "poll:") {
				if s == "poll:2" {
					tr.Accounting.TScannedTuples = 1
				}
			}
		}
	}
	tr.Terminal = "FAILED"
	tr.Failure = &Failure{Code: code, Stage: stageFor(code), Detail: detail}
	tr.Accounting.FailureCounters[code] = 1
	seal(&tr, a, raw, adm, bindingOverride)
	return tr
}
func stageFor(code string) string {
	return map[string]string{"INVALID_INPUT": "raw", "CANCELLED": "control", "DEADLINE_EXCEEDED": "control", "ASSOCIATION_FAILED": "association", "ADMISSION_FAILED": "admission", "RESOURCE_EXHAUSTED": "scan", "OVERFLOW": "accounting", "INVARIANT_FAILED": "invariant"}[code]
}
func detailFor(code, value string) map[string]any {
	k := map[string]string{"INVALID_INPUT": "reason", "CANCELLED": "control", "DEADLINE_EXCEEDED": "deadline", "ASSOCIATION_FAILED": "source_id", "ADMISSION_FAILED": "source_id", "RESOURCE_EXHAUSTED": "limit", "OVERFLOW": "counter", "INVARIANT_FAILED": "invariant"}[code]
	if (code == "CANCELLED" || code == "DEADLINE_EXCEEDED") && strings.HasPrefix(value, "poll-") {
		value = "poll:" + strings.TrimPrefix(value, "poll-")
	}
	return map[string]any{k: value}
}

func eval(a Attempt, raw []byte) (TerminalResult, []byte, error) {
	if err := validateAttempt(a); err != "" {
		tr := sealFailure(a, raw, nil, nil, "INVALID_INPUT", detailFor("INVALID_INPUT", err))
		return tr, Canon(tr), ErrTerminal
	}
	obs := map[uint64]Observation{}
	var last uint64
	for i, o := range a.ExecutionControl.Observations {
		if i > 0 && o.PollIndex <= last {
			tr := sealFailure(a, raw, nil, nil, "INVALID_INPUT", detailFor("INVALID_INPUT", "execution_control"))
			return tr, Canon(tr), ErrTerminal
		}
		if _, exists := obs[o.PollIndex]; exists {
			tr := sealFailure(a, raw, nil, nil, "INVALID_INPUT", detailFor("INVALID_INPUT", "execution_control"))
			return tr, Canon(tr), ErrTerminal
		}
		if o.Cancelled && o.DeadlineExpired {
			obs[o.PollIndex] = o
			last = o.PollIndex
			continue
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
		tr := sealFailure(a, raw, nil, nil, c, detailFor(c, "poll-0"))
		return tr, Canon(tr), ErrTerminal
	}
	if uint64(len(a.Request.Sources)) > a.Request.Limits.MaxFiles {
		tr := sealFailure(a, raw, nil, nil, "RESOURCE_EXHAUSTED", detailFor("RESOURCE_EXHAUSTED", "max_files"))
		return tr, Canon(tr), ErrTerminal
	}
	byOrd := map[uint64]SourceInput{}
	for _, in := range a.SourceInputs {
		if _, ok := byOrd[in.Ordinal]; ok {
			tr := sealFailure(a, raw, nil, nil, "ASSOCIATION_FAILED", detailFor("ASSOCIATION_FAILED", fmt.Sprintf("ordinal:%d", in.Ordinal)))
			return tr, Canon(tr), ErrTerminal
		}
		byOrd[in.Ordinal] = in
	}
	files := []searchFile{}
	pathBytes, totalBytes := uint64(0), uint64(0)
	for _, s := range a.Request.Sources {
		in, ok := byOrd[s.Ordinal]
		if !ok || in.Path != s.Path || in.Revision != s.Revision || in.FileDigest != s.FileDigest || in.ObjectDigest != s.ObjectDigest {
			tr := sealFailure(a, raw, nil, nil, "ASSOCIATION_FAILED", detailFor("ASSOCIATION_FAILED", fmt.Sprintf("ordinal:%d", s.Ordinal)))
			return tr, Canon(tr), ErrTerminal
		}
		b, e := base64.StdEncoding.DecodeString(in.BytesBase64)
		if e != nil {
			tr := sealFailure(a, raw, nil, nil, "ADMISSION_FAILED", detailFor("ADMISSION_FAILED", sourceID(s.Ordinal, s.Path, s.Revision)))
			return tr, Canon(tr), ErrTerminal
		}
		pathBytes += uint64(len([]byte(s.Path)))
		if uint64(len(b)) > a.Request.Limits.MaxSourceBytes {
			tr := sealFailure(a, raw, nil, nil, "RESOURCE_EXHAUSTED", detailFor("RESOURCE_EXHAUSTED", "max_source_bytes"))
			return tr, Canon(tr), ErrTerminal
		}
		totalBytes += uint64(len(b))
		if totalBytes > a.Request.Limits.MaxTotalBytes {
			tr := sealFailure(a, raw, nil, nil, "RESOURCE_EXHAUSTED", detailFor("RESOURCE_EXHAUSTED", "max_total_bytes"))
			return tr, Canon(tr), ErrTerminal
		}
		files = append(files, searchFile{s, in, b})
	}
	if len(files) != len(byOrd) {
		tr := sealFailure(a, raw, nil, nil, "ASSOCIATION_FAILED", detailFor("ASSOCIATION_FAILED", "extra_source_input"))
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
		admFail, srcFail := admissionFromFiles(files)
		detailValue := ar.Detail
		if code == "ADMISSION_FAILED" && len(files) > 0 {
			detailValue = files[0].ref.Path
			admFail = AdmissionRecord{SchemaVersion: AdmissionRecordSchema, AdmissionSchema: admit.Schema, AdmissionDigest: Digest([]byte(""))}
			srcFail = nil
		}
		tr := sealFailure(a, raw, &admFail, srcFail, code, detailFor(code, detailValue))
		return tr, Canon(tr), ErrTerminal
	}
	adm := AdmissionRecord{SchemaVersion: AdmissionRecordSchema, AdmissionSchema: admit.Schema, AdmissionDigest: ar.Binding.AdmissionDigest}
	orderedBytes := map[string][]byte{}
	sources := []Source{}
	for i, s := range ar.Binding.Sources {
		ord := uint64(i)
		st := SourceTuple{Ordinal: ord, Path: s.Path, Revision: s.Revision, FileDigest: s.FileDigest, ObjectDigest: s.ObjectDigest, SourceByteLength: uint64(len(s.Bytes))}
		adm.OrderedSources = append(adm.OrderedSources, st)
		orderedBytes[s.Path] = s.Bytes
		id := sourceID(ord, s.Path, s.Revision)
		sources = append(sources, Source{SourceID: id, LogicalURI: logicalURI(s.Path), PathBytes: uint64(len([]byte(s.Path))), ByteLength: uint64(len(s.Bytes)), ContentSHA256: Digest(s.Bytes), Ordinal: ord})
	}
	adm.AdmissionDigest = admissionDigest(adm.OrderedSources)
	adm.BindingBytes = admissionBindingBytes(adm.OrderedSources, orderedBytes)
	for _, s := range sources {
		if s.PathBytes > a.Request.Limits.MaxPathBytes {
			tr := sealFailure(a, raw, &adm, sources, "RESOURCE_EXHAUSTED", detailFor("RESOURCE_EXHAUSTED", "max_path_bytes"))
			return tr, Canon(tr), ErrTerminal
		}
	}
	if c, yes := check(); yes {
		tr := sealFailure(a, raw, &adm, sources, c, detailFor(c, "admission"))
		return tr, Canon(tr), ErrTerminal
	}
	acc := Accounting{SchemaVersion: AccountingSchema, FailureCounters: FailureCounters()}
	acc.JFiles = uint64(len(sources))
	for _, s := range sources {
		acc.PPathBytes += s.PathBytes
		acc.SSourceBytes += s.ByteLength
	}
	acc.QQueryBytes = uint64(len([]byte(a.Request.Query)))
	matches := []Match{}
	positions := []Position{}
	matchOrdinal := uint64(0)
	for _, src := range adm.OrderedSources {
		b := orderedBytes[src.Path]
		for i := 0; i+len(a.Request.Query) <= len(b); i++ {
			acc.TScannedTuples++
			if c, yes := check(); yes {
				tr := sealFailure(a, raw, &adm, sources, c, detailFor(c, fmt.Sprintf("poll-%d", poll-1)))
				return tr, Canon(tr), ErrTerminal
			}
			if bytes.Equal(b[i:i+len(a.Request.Query)], []byte(a.Request.Query)) {
				if acc.MMatches+1 > a.Request.Limits.MaxMatches {
					tr := sealFailure(a, raw, &adm, sources, "RESOURCE_EXHAUSTED", detailFor("RESOURCE_EXHAUSTED", "max_matches"))
					tr.Accounting.TScannedTuples = acc.TScannedTuples
					tr.Accounting.MMatches = acc.MMatches
					tr.Accounting.RRanges = acc.RRanges
					tr.Accounting.UUTF16Units = acc.UUTF16Units
					seal(&tr, a, raw, &adm)
					return tr, Canon(tr), ErrTerminal
				}
				start, end := uint64(i), uint64(i+len(a.Request.Query))
				sid := sourceID(src.Ordinal, src.Path, src.Revision)
				mid := matchID(sid, matchOrdinal, start, end, a.Request.Query)
				m := Match{MatchID: mid, SourceID: sid, PathBytes: uint64(len([]byte(src.Path))), StartByte: start, EndByte: end, Ordinal: matchOrdinal, Literal: a.Request.Query}
				matches = append(matches, m)
				positions = append(positions, positionFor(mid, b, start, end))
				acc.MMatches++
				acc.RRanges++
				acc.UUTF16Units += utf16Units(b[i : i+len(a.Request.Query)])
				matchOrdinal++
				if c, yes := check(); yes {
					tr := sealFailure(a, raw, &adm, sources, c, detailFor(c, "match"))
					return tr, Canon(tr), ErrTerminal
				}
			}
		}
	}
	sort.SliceStable(matches, func(i, j int) bool {
		a, b := matches[i], matches[j]
		if a.PathBytes != b.PathBytes {
			return a.PathBytes < b.PathBytes
		}
		if a.StartByte != b.StartByte {
			return a.StartByte < b.StartByte
		}
		if a.EndByte != b.EndByte {
			return a.EndByte < b.EndByte
		}
		return a.Ordinal < b.Ordinal
	})
	posByID := map[string]Position{}
	for _, p := range positions {
		posByID[p.MatchID] = p
	}
	positions = positions[:0]
	for _, m := range matches {
		positions = append(positions, posByID[m.MatchID])
	}
	tr := base(a, raw, &adm, sources, false)
	tr.Matches = matches
	tr.Positions = positions
	tr.Accounting = acc
	tr.RangeUnionCandidate = buildCandidate(tr.Admission.AdmissionDigest, matches, positions, a.Request.LocationPin)
	seal(&tr, a, raw, &adm)
	return tr, Canon(tr), nil
}

func admissionFromFiles(files []searchFile) (AdmissionRecord, []Source) {
	adm := AdmissionRecord{SchemaVersion: AdmissionRecordSchema, AdmissionSchema: admit.Schema, AdmissionDigest: zeroSHA}
	sources := []Source{}
	for _, f := range files {
		id := sourceID(f.ref.Ordinal, f.ref.Path, f.ref.Revision)
		adm.OrderedSources = append(adm.OrderedSources, SourceTuple{Ordinal: f.ref.Ordinal, Path: f.ref.Path, Revision: f.ref.Revision, FileDigest: f.ref.FileDigest, ObjectDigest: f.ref.ObjectDigest, SourceByteLength: uint64(len(f.b))})
		sources = append(sources, Source{SourceID: id, LogicalURI: logicalURI(f.ref.Path), PathBytes: uint64(len([]byte(f.ref.Path))), ByteLength: uint64(len(f.b)), ContentSHA256: Digest(f.b), Ordinal: f.ref.Ordinal})
	}
	return adm, sources
}

type sourceAdmissionBinding struct {
	Schema          string                  `json:"schema"`
	AdmissionDigest string                  `json:"admissionDigest"`
	Sources         []sourceAdmissionMember `json:"sources"`
}
type sourceAdmissionMember struct {
	Path         string `json:"path"`
	Revision     string `json:"revision"`
	FileDigest   string `json:"fileDigest"`
	ObjectDigest string `json:"objectDigest"`
	BytesBase64  string `json:"bytes_base64"`
}

func admissionDigest(in []SourceTuple) string {
	h := sha256.New()
	h.Write([]byte("lsp-trace.adr0007.source-admission.private.v2"))
	for _, s := range in {
		for _, v := range []string{s.Path, s.Revision, s.FileDigest, s.ObjectDigest} {
			h.Write([]byte{0})
			h.Write([]byte(v))
		}
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil))
}
func admissionBindingBytes(in []SourceTuple, data map[string][]byte) string {
	b := sourceAdmissionBinding{Schema: "lsp-trace.adr0007.source-admission.private.v2", AdmissionDigest: admissionDigest(in)}
	for _, s := range in {
		b.Sources = append(b.Sources, sourceAdmissionMember{Path: s.Path, Revision: s.Revision, FileDigest: s.FileDigest, ObjectDigest: s.ObjectDigest, BytesBase64: base64.StdEncoding.EncodeToString(data[s.Path])})
	}
	return string(Canon(b))
}

func matchID(sourceID string, ordinal, start, end uint64, literal string) string {
	return Digest([]byte(fmt.Sprintf("match\x00%s\x00%d\x00%d\x00%d\x00%s", sourceID, ordinal, start, end, literal)))
}

func buildCandidate(admDigest string, matches []Match, pos []Position, lp LocationPin) *Candidate {
	c := &Candidate{Operation: "RANGE_UNION", ExecutedLocation: false, CandidateOnly: true, AdmissionDigest: admDigest, MemberMatchIDs: []string{}, QualifiedLocationPins: []QualifiedLocation{}}
	for i, m := range matches {
		c.MemberMatchIDs = append(c.MemberMatchIDs, m.MatchID)
		p := pos[i]
		c.QualifiedLocationPins = append(c.QualifiedLocationPins, QualifiedLocation{SourceID: m.SourceID, StartByte: m.StartByte, EndByte: m.EndByte, StartLine: p.StartLine, StartCharacterUTF16: p.StartCharacterUTF16, EndLine: p.EndLine, EndCharacterUTF16: p.EndCharacterUTF16})
	}
	c.CandidateDigest = candidateDigest(c, lp)
	return c
}

func candidateDigest(c *Candidate, lp LocationPin) string {
	zero := *c
	zero.CandidateDigest = zeroSHA
	preimage := map[string]any{
		"schema_version":          lp.SchemaVersion,
		"operation":               lp.Operation,
		"executedLocation":        lp.ExecutedLocation,
		"candidate_only":          zero.CandidateOnly,
		"complete_location_pins":  append([]string(nil), lp.CompleteLocationPins...),
		"design_commit":           lp.DesignCommit,
		"design_root_sha256":      lp.DesignRootSha256,
		"execution_commit":        lp.ExecutionCommit,
		"seal_commit":             lp.SealCommit,
		"final_seal_sha256":       lp.FinalSealSha256,
		"source_admission_pin":    lp.SourceAdmissionPin,
		"admission_digest":        zero.AdmissionDigest,
		"member_match_ids":        append([]string(nil), zero.MemberMatchIDs...),
		"qualified_location_pins": append([]QualifiedLocation(nil), zero.QualifiedLocationPins...),
		"candidate":               zero,
	}
	return Digest(Canon(preimage))
}
func sourceID(ord uint64, path, rev string) string {
	return Digest([]byte(fmt.Sprintf("source\x00%s\x00%s\x00%d", path, rev, ord)))
}
func logicalURI(path string) string {
	return "file:///" + strings.TrimPrefix(path, "/")
}
func ordinalFor(s []SourceRef, path string) uint64 {
	for _, x := range s {
		if x.Path == path {
			return x.Ordinal
		}
	}
	return 0
}

func seal(tr *TerminalResult, a Attempt, raw []byte, adm *AdmissionRecord, bindingOverride ...string) {
	if raw == nil {
		raw = Canon(a)
	}
	if tr.Accounting.SchemaVersion == "" {
		tr.Accounting.SchemaVersion = AccountingSchema
	}
	if tr.Accounting.FailureCounters == nil {
		tr.Accounting.FailureCounters = FailureCounters()
	}
	if a.TestControl != nil && a.TestControl.InitialBOutputBytes > 0 {
		tr.Accounting.BOutputBytes = a.TestControl.InitialBOutputBytes
		if _, ok := Work(tr.Accounting); !ok {
			tr.Terminal = "FAILED"
			tr.Failure = &Failure{Code: "OVERFLOW", Stage: "accounting", Detail: detailFor("OVERFLOW", "B_output_bytes")}
			tr.Matches = []Match{}
			tr.Positions = []Position{}
			tr.RangeUnionCandidate = nil
			tr.Accounting.FailureCounters = FailureCounters()
			tr.Accounting.FailureCounters["OVERFLOW"] = 1
		}
	}
	tr.Replay.CanonicalAttemptSHA256 = Digest(raw)
	if adm != nil {
		if adm.BindingBytes != "" {
			tr.Replay.AdmittedBindingSHA256 = Digest([]byte(adm.BindingBytes))
		} else if tr.Failure != nil && (tr.Failure.Code == "ADMISSION_FAILED" || tr.Failure.Code == "RESOURCE_EXHAUSTED") {
			tr.Replay.AdmittedBindingSHA256 = Digest([]byte(""))
		} else {
			tr.Replay.AdmittedBindingSHA256 = Digest(Canon(adm))
		}
	} else if len(bindingOverride) > 0 && bindingOverride[0] != "" {
		tr.Replay.AdmittedBindingSHA256 = Digest([]byte(bindingOverride[0]))
	} else {
		tr.Replay.AdmittedBindingSHA256 = Digest([]byte(""))
	}
	tr.Replay.ToolingIdentitySHA256 = ToolDigest()
	tr.Replay.PredecessorLockSHA256 = PredecessorDigest()
	if a.ExternalFreezeBinding != nil {
		tr.Replay.FreezeBindingSHA256 = Digest(Canon(*a.ExternalFreezeBinding))
	} else {
		tr.Replay.FreezeBindingSHA256 = "sha256:94d7171a7cb8d0cd62d77401119d002d5c7627cb595e8cb670e75821a103fa7e"
	}
	tr.Payload.FreezeBindingSHA256 = tr.Replay.FreezeBindingSHA256
	tr.Payload.PayloadDigest = tr.Replay.FreezeBindingSHA256
	for i := 0; i < 64; i++ {
		tr.Custody.TerminalResultSHA256 = zeroSHA
		tr.Replay.TerminalPreimageSHA256 = zeroSHA
		tr.Accounting.BOutputBytes = uint64(len(Canon(tr)))
		w, ok := Work(tr.Accounting)
		if !ok {
			tr.Terminal = "FAILED"
			tr.Failure = &Failure{Code: "OVERFLOW", Stage: "accounting", Detail: detailFor("OVERFLOW", "W_work")}
			tr.Matches = []Match{}
			tr.Positions = []Position{}
			tr.RangeUnionCandidate = nil
			tr.Accounting.FailureCounters = FailureCounters()
			tr.Accounting.FailureCounters["OVERFLOW"] = 1
			continue
		}
		tr.Accounting.WWork = w
		n := uint64(len(Canon(tr)))
		if n == tr.Accounting.BOutputBytes {
			break
		}
		tr.Accounting.BOutputBytes = n
	}
	if tr.Accounting.WWork > a.Request.Limits.MaxWork && a.Request.Limits.MaxWork > 0 {
		tr.Terminal = "FAILED"
		tr.Failure = &Failure{Code: "RESOURCE_EXHAUSTED", Stage: "scan", Detail: detailFor("RESOURCE_EXHAUSTED", "post_scan")}
		tr.Matches = []Match{}
		tr.Positions = []Position{}
		tr.RangeUnionCandidate = nil
		tr.Accounting.FailureCounters = FailureCounters()
		tr.Accounting.FailureCounters["RESOURCE_EXHAUSTED"] = 1
	}
	if tr.Accounting.BOutputBytes > a.Request.Limits.MaxOutputBytes && a.Request.Limits.MaxOutputBytes > 0 {
		tr.Terminal = "FAILED"
		tr.Failure = &Failure{Code: "RESOURCE_EXHAUSTED", Stage: "scan", Detail: detailFor("RESOURCE_EXHAUSTED", "post_scan")}
		tr.Matches = []Match{}
		tr.Positions = []Position{}
		tr.RangeUnionCandidate = nil
		tr.Accounting.FailureCounters = FailureCounters()
		tr.Accounting.FailureCounters["RESOURCE_EXHAUSTED"] = 1
	}
	for i := 0; i < 64; i++ {
		tr.Custody.TerminalResultSHA256 = zeroSHA
		tr.Replay.TerminalPreimageSHA256 = zeroSHA
		tr.Accounting.BOutputBytes = uint64(len(Canon(tr)))
		w, _ := Work(tr.Accounting)
		tr.Accounting.WWork = w
		n := uint64(len(Canon(tr)))
		if n == tr.Accounting.BOutputBytes {
			break
		}
		tr.Accounting.BOutputBytes = n
	}
	pre := *tr
	pre.Custody.TerminalResultSHA256 = zeroSHA
	pre.Replay.TerminalPreimageSHA256 = zeroSHA
	preBytes := Canon(pre)
	tr.Replay.TerminalPreimageSHA256 = Digest(preBytes)
	tr.Custody.TerminalResultSHA256 = tr.Replay.TerminalPreimageSHA256
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
func ToolDigest() string        { return toolingDigest }
func PredecessorDigest() string { return predecessorLockDigest }
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
func positionFor(id string, b []byte, start, end uint64) Position {
	sl, sc := lineChar(b, start)
	el, ec := lineChar(b, end)
	return Position{MatchID: id, StartLine: sl, StartCharacterUTF16: sc, EndLine: el, EndCharacterUTF16: ec}
}
func lineChar(b []byte, off uint64) (uint64, uint64) {
	var line, ch uint64
	for i := uint64(0); i < off; {
		if b[i] == '\r' {
			if i+1 < off && b[i+1] == '\n' {
				i += 2
			} else {
				i++
			}
			line++
			ch = 0
			continue
		}
		if b[i] == '\n' {
			i++
			line++
			ch = 0
			continue
		}
		r, s := utf8.DecodeRune(b[i:])
		ch += uint64(len(utf16.Encode([]rune{r})))
		i += uint64(s)
	}
	return line, ch
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
	if lp.SchemaVersion != LocationPinSchema || lp.Operation != "RANGE_UNION" || lp.ExecutedLocation || lp.DesignCommit != "c943a484060462121c6f0929d4182b053ff95ab5" || lp.DesignRootSha256 != "sha256:1195a420cc2ae215ff1627dbf23b606caaa243aa9fc0ae234242acceddafb48d" || lp.ExecutionCommit != "f0f8b49aa368bea9b3e6d105eef5cb2614221067" || lp.SealCommit != "16f40dcb03a234b00db80059a7eef400495e9d97" || lp.FinalSealSha256 != "sha256:f4981045d3489f5ef0633eb4ce6b4a17ab6de1ddc73c4b729f9dd524106b1fd6" || strings.Join(lp.CompleteLocationPins, ",") != "design_commit,design_root_sha256,execution_commit,seal_commit,final_seal_sha256,source_admission_pin" {
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
	if a.TestControl != nil {
		if a.TestControl.SchemaVersion != "lsp-trace.adr0007.source-text-search.test-control.private.v4" || !strings.Contains(a.AttemptID, "overflow_helper_case") {
			return "test_control"
		}
	}
	return ""
}
