package adr0007sourcetextsearchv3private

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"math"
	"sort"
	"strings"
	"unicode/utf8"
)

var ErrTerminal = errors.New("terminal")

func digest(b []byte) string            { h := sha256.Sum256(b); return "sha256:" + hex.EncodeToString(h[:]) }
func Evaluate(a Attempt) TerminalResult { r, _, _ := eval(a); return r }
func fail(a Attempt, acc Accounting, code string, detail map[string]any) *TerminalResult {
	if acc.FailureCounters == nil {
		acc.FailureCounters = fc()
	}
	acc.FailureCounters[code] = 1
	tr := base(a, acc)
	tr.Outcome = "FAILED"
	tr.Failure = &Failure{Code: code, Detail: detail}
	seal(&tr, a, nil)
	return &tr
}
func base(a Attempt, acc Accounting) TerminalResult {
	acc.SchemaVersion = "lsp-trace.adr0007.source-text-search.accounting.private.v3"
	return TerminalResult{SchemaVersion: "lsp-trace.adr0007.source-text-search.terminal-result.private.v3", AttemptID: a.AttemptID, TerminalSequence: 0, Outcome: "COMPLETE", Authority: 0, Accepted: false, Completeness: "UNKNOWN", FeatureIdentity: "UNRESOLVED", Matches: []Match{}, Accounting: acc, Custody: Custody{SchemaVersion: "lsp-trace.adr0007.source-text-search.custody.private.v3", AttemptID: a.AttemptID, TerminalSequence: 0, TerminalCount: 1}, Replay: Replay{SchemaVersion: "lsp-trace.adr0007.source-text-search.replay.private.v3"}}
}
func fc() map[string]uint64 {
	return map[string]uint64{"INVALID_INPUT": 0, "CANCELLED": 0, "DEADLINE_EXCEEDED": 0, "ASSOCIATION_FAILED": 0, "ADMISSION_FAILED": 0, "RESOURCE_EXHAUSTED": 0, "OVERFLOW": 0, "INVARIANT_FAILED": 0}
}
func eval(a Attempt) (TerminalResult, []byte, error) {
	acc := Accounting{FailureCounters: fc()}
	if a.SchemaVersion != "lsp-trace.adr0007.source-text-search.attempt.private.v3" || !strings.HasPrefix(a.AttemptID, "attempt-") || a.Request.Query == "" || !utf8.ValidString(a.Request.Query) {
		tr := fail(a, acc, "INVALID_INPUT", map[string]any{"field": "attempt"})
		return *tr, Canon(tr), ErrTerminal
	}
	obs := map[uint64]Observation{}
	var last uint64
	for i, o := range a.ExecutionControl.Observations {
		if i > 0 && o.PollIndex <= last {
			tr := fail(a, acc, "INVALID_INPUT", map[string]any{"field": "execution_control"})
			return *tr, Canon(tr), ErrTerminal
		}
		last = o.PollIndex
		obs[o.PollIndex] = o
	}
	poll := uint64(0)
	check := func() (string, bool) {
		if o, ok := obs[poll]; ok {
			poll++
			if o.DeadlineExpired {
				return "DEADLINE_EXCEEDED", true
			}
			if o.Cancelled {
				return "CANCELLED", true
			}
			return "", false
		}
		poll++
		return "", false
	}
	if c, yes := check(); yes {
		tr := fail(a, acc, c, map[string]any{"poll": 0})
		return *tr, Canon(tr), ErrTerminal
	}
	if len(a.Request.Sources) == 0 || uint64(len(a.Request.Sources)) > a.Request.Limits.MaxFiles {
		tr := fail(a, acc, "INVALID_INPUT", map[string]any{"field": "sources"})
		return *tr, Canon(tr), ErrTerminal
	}
	if c, yes := check(); yes {
		tr := fail(a, acc, c, map[string]any{"poll": 1})
		return *tr, Canon(tr), ErrTerminal
	}
	byOrd := map[uint64]SourceInput{}
	for _, in := range a.SourceInputs {
		if _, ok := byOrd[in.Ordinal]; ok {
			tr := fail(a, acc, "ASSOCIATION_FAILED", map[string]any{"reason": "duplicate ordinal"})
			return *tr, Canon(tr), ErrTerminal
		}
		byOrd[in.Ordinal] = in
	}
	type sf struct {
		t SourceTuple
		b []byte
	}
	files := []sf{}
	seenPath := map[string]bool{}
	for _, s := range a.Request.Sources {
		in, ok := byOrd[s.Ordinal]
		if !ok || in.Path != s.Path || in.Revision != s.Revision || in.FileDigest != s.FileDigest || in.ObjectDigest != s.ObjectDigest {
			tr := fail(a, acc, "ASSOCIATION_FAILED", map[string]any{"ordinal": s.Ordinal})
			return *tr, Canon(tr), ErrTerminal
		}
		if seenPath[s.Path] {
			tr := fail(a, acc, "ASSOCIATION_FAILED", map[string]any{"path": s.Path})
			return *tr, Canon(tr), ErrTerminal
		}
		seenPath[s.Path] = true
		b, e := base64.StdEncoding.DecodeString(in.BytesBase64)
		if e != nil || len(b) == 0 || !utf8.Valid(b) || strings.Contains(s.Path, "..") || strings.Contains(s.Path, "\\") || strings.HasPrefix(s.Path, "/") || !utf8.ValidString(s.Path) {
			tr := fail(a, acc, "ADMISSION_FAILED", map[string]any{"path": s.Path})
			return *tr, Canon(tr), ErrTerminal
		}
		if digest(b) != s.FileDigest || digest([]byte("object:"+string(b))) != s.ObjectDigest {
			tr := fail(a, acc, "ASSOCIATION_FAILED", map[string]any{"reason": "digest"})
			return *tr, Canon(tr), ErrTerminal
		}
		files = append(files, sf{SourceTuple{s.Ordinal, s.Path, s.Revision, s.FileDigest, s.ObjectDigest, uint64(len(b))}, b})
	}
	if len(files) != len(byOrd) {
		tr := fail(a, acc, "ASSOCIATION_FAILED", map[string]any{"reason": "extra input"})
		return *tr, Canon(tr), ErrTerminal
	}
	if c, yes := check(); yes {
		tr := fail(a, acc, c, map[string]any{"poll": 2})
		return *tr, Canon(tr), ErrTerminal
	}
	if c, yes := check(); yes {
		tr := fail(a, acc, c, map[string]any{"poll": 3})
		return *tr, Canon(tr), ErrTerminal
	}
	sort.Slice(files, func(i, j int) bool { return bytes.Compare([]byte(files[i].t.Path), []byte(files[j].t.Path)) < 0 })
	adm := AdmissionRecord{SchemaVersion: "lsp-trace.adr0007.source-text-search.admission-record.private.v3", AdmissionSchema: "lsp-trace.adr0007.source-admission.private.v2"}
	for _, f := range files {
		adm.OrderedSources = append(adm.OrderedSources, f.t)
		acc.JFiles++
		acc.PPathBytes += uint64(len([]byte(f.t.Path)))
		acc.SSourceBytes += uint64(len(f.b))
	}
	adm.AdmissionDigest = digest(Canon(adm))
	if c, yes := check(); yes {
		tr := fail(a, acc, c, map[string]any{"poll": 4})
		return *tr, Canon(tr), ErrTerminal
	}
	q := []byte(a.Request.Query)
	acc.QQueryBytes = uint64(len(q))
	matches := []Match{}
	for _, f := range files {
		if c, yes := check(); yes {
			tr := fail(a, acc, c, map[string]any{"poll": "file"})
			tr.Admission = &adm
			return *tr, Canon(tr), ErrTerminal
		}
		for i := 0; i+len(q) <= len(f.b); i++ {
			if c, yes := check(); yes {
				tr := fail(a, acc, c, map[string]any{"poll": "byte"})
				tr.Admission = &adm
				return *tr, Canon(tr), ErrTerminal
			}
			acc.TScannedTuples++
			if bytes.Equal(f.b[i:i+len(q)], q) {
				u := utf16Units(f.b[i : i+len(q)])
				if acc.MMatches+1 > a.Request.Limits.MaxMatches {
					tr := fail(a, acc, "RESOURCE_EXHAUSTED", map[string]any{"limit": "max_matches"})
					tr.Admission = &adm
					return *tr, Canon(tr), ErrTerminal
				}
				ps := position(f.b, i)
				pe := position(f.b, i+len(q))
				m := Match{SchemaVersion: "lsp-trace.adr0007.source-text-search.match.private.v3", Source: f.t, ByteRange: ByteRange{uint64(i), uint64(i + len(q))}, LSPUTF16Range: LSPRange{ps, pe}, Literal: a.Request.Query}
				matches = append(matches, m)
				acc.MMatches++
				acc.RRanges++
				acc.UUTF16Units += u
				if c, yes := check(); yes {
					tr := fail(a, acc, c, map[string]any{"poll": "match"})
					tr.Admission = &adm
					return *tr, Canon(tr), ErrTerminal
				}
			}
		}
	}
	tr := base(a, acc)
	tr.Admission = &adm
	tr.Matches = matches
	cand := RangeUnionCandidate{SchemaVersion: "lsp-trace.adr0007.source-text-search.range-union-candidate.private.v3", Operation: "RANGE_UNION", ExecutedLocation: false, CandidateOnly: true, DesignCommit: "c943a484060462121c6f0929d4182b053ff95ab5", DesignRootSha256: "sha256:1195a420cc2ae215ff1627dbf23b606caaa243aa9fc0ae234242acceddafb48d", ExecutionCommit: "f0f8b49aa368bea9b3e6d105eef5cb2614221067", SealCommit: "16f40dcb03a234b00db80059a7eef400495e9d97", FinalSealSha256: "sha256:f4981045d3489f5ef0633eb4ce6b4a17ab6de1ddc73c4b729f9dd524106b1fd6", AdmissionDigest: adm.AdmissionDigest, Members: matches}
	cand.CandidateDigest = digest(Canon(cand))
	tr.RangeUnionCandidate = &cand
	if c, yes := check(); yes {
		return *fail(a, acc, c, map[string]any{"poll": "candidate"}), Canon(tr), ErrTerminal
	}
	seal(&tr, a, &adm)
	return tr, Canon(tr), nil
}
func seal(tr *TerminalResult, a Attempt, adm *AdmissionRecord) {
	for i := 0; i < 64; i++ {
		tr.Accounting.BOutputBytes = uint64(len(Canon(tr)))
		w, ok := work(tr.Accounting)
		if !ok {
			tr.Outcome = "FAILED"
			tr.Failure = &Failure{Code: "OVERFLOW", Detail: map[string]any{}}
		}
		tr.Accounting.WWork = w
		n := uint64(len(Canon(tr)))
		if n == tr.Accounting.BOutputBytes {
			break
		}
		tr.Accounting.BOutputBytes = n
	}
	tr.Replay.CanonicalAttemptSHA256 = digest(Canon(a))
	if adm != nil {
		tr.Replay.AdmittedBindingSHA256 = digest(Canon(adm))
	}
	tr.Replay.ToolingIdentitySHA256 = digest([]byte("adr0007-v3-private"))
	tr.Replay.FreezeRootSha256 = "sha256:pending-freeze-root"
	pre := *tr
	pre.Custody.TerminalResultSHA256 = "sha256:" + strings.Repeat("0", 64)
	tr.Replay.TerminalPreimageSHA256 = digest(Canon(pre))
	tr.Custody.TerminalResultSHA256 = digest(Canon(pre))
}
func work(a Accounting) (uint64, bool) {
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
