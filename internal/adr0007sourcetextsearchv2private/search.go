package adr0007sourcetextsearchv2private

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math"
	"sort"
	"strings"
	"unicode/utf8"
)

var ErrFailClosed = errors.New("adr0007 source text search v2 private fail closed")

type SourceFile struct {
	Binding SourceBinding
	Bytes   []byte
}

func CheckedWork(a Accounting) (uint64, bool) {
	w := uint64(50)
	terms := []struct{ c, v uint64 }{{3, a.JFiles}, {5, a.QQueryBytes}, {7, a.PPathBytes}, {1, a.SSourceBytes}, {11, a.TScannedTuples}, {13, a.MMatches}, {17, a.RRanges}, {19, a.UUTF16Units}, {31, a.BOutputBytes}}
	for _, tv := range terms {
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

func ValidatePolicy(p Policy) bool {
	return p.SchemaVersion == SchemaPolicyV2 && p.LiteralMode == "EXACT_NONEMPTY_CASE_SENSITIVE_UTF8" && !p.AllowRegex && !p.AllowFuzzy && !p.AllowToken && !p.AllowRank && !p.AllowModel && !p.AllowBackendSemantics
}

func ValidateRequest(r Request) error {
	if r.SchemaVersion != SchemaRequestV2 || r.Query == "" || !utf8.ValidString(r.Query) {
		return ErrFailClosed
	}
	if !ValidatePolicy(r.Policy) || r.Limits.SchemaVersion != SchemaLimitsV2 || r.Limits.MaxFiles == 0 || r.Limits.MaxWork == 0 || r.Limits.MaxOutputBytes == 0 || len(r.Sources) == 0 || uint64(len(r.Sources)) > r.Limits.MaxFiles {
		return ErrFailClosed
	}
	seen := map[string]bool{}
	for _, s := range r.Sources {
		if s.SchemaVersion != SchemaBindingV2 || s.Path == "" || !utf8.ValidString(s.Path) || strings.Contains(s.Path, "..") || strings.HasPrefix(s.Path, "/") || s.RepositoryCommit == "" || s.Revision == "" || s.FileSHA256 == "" || s.GitBlobSHA1 == "" || s.ObjectID == "" || s.AdmissionSchema != "lsp-trace.adr0007.source-admission.private.v2" || s.AdmissionID == "" {
			return ErrFailClosed
		}
		k := s.Path + "\x00" + s.Revision + "\x00" + s.FileSHA256
		if seen[k] {
			return ErrFailClosed
		}
		seen[k] = true
		if uint64(len([]byte(s.Path))) > r.Limits.MaxPathBytes {
			return ErrFailClosed
		}
	}
	return nil
}

func Search(r Request, files []SourceFile) (Result, error) {
	if err := ValidateRequest(r); err != nil {
		return Result{}, err
	}
	if len(files) != len(r.Sources) {
		return Result{}, ErrFailClosed
	}
	for _, f := range files {
		if !utf8.Valid(f.Bytes) || uint64(len(f.Bytes)) > r.Limits.MaxSourceBytes {
			return Result{}, ErrFailClosed
		}
		h := sha256.Sum256(f.Bytes)
		if f.Binding.FileSHA256 != "sha256:"+hex.EncodeToString(h[:]) {
			return Result{}, ErrFailClosed
		}
	}
	sort.Slice(files, func(i, j int) bool {
		return bytes.Compare([]byte(files[i].Binding.Path), []byte(files[j].Binding.Path)) < 0
	})
	acc := Accounting{SchemaVersion: SchemaAccounting, FailureCounters: map[string]uint64{"schema": 0, "pin": 0, "custody": 0, "replay": 0, "accounting": 0, "precedence": 0, "overflow": 0, "cancel": 0, "deadline": 0, "freeze": 0}}
	acc.JFiles = uint64(len(files))
	acc.QQueryBytes = uint64(len([]byte(r.Query)))
	var matches []Match
	q := []byte(r.Query)
	for _, f := range files {
		acc.PPathBytes += uint64(len([]byte(f.Binding.Path)))
		acc.SSourceBytes += uint64(len(f.Bytes))
		acc.TScannedTuples++
		for i := 0; i+len(q) <= len(f.Bytes); i++ {
			if bytes.Equal(f.Bytes[i:i+len(q)], q) {
				ls, cs := Position(f.Bytes, i)
				le, ce := Position(f.Bytes, i+len(q))
				m := Match{SchemaVersion: "lsp-trace.adr0007.source-text-search.match.private.v2", Path: f.Binding.Path, ByteStart: uint64(i), ByteEnd: uint64(i + len(q)), LineStart: ls, CharacterStart: cs, LineEnd: le, CharacterEnd: ce, Literal: r.Query}
				matches = append(matches, m)
				acc.MMatches++
				acc.RRanges++
				acc.UUTF16Units += ce - cs
				if acc.MMatches > r.Limits.MaxMatches {
					return Result{}, ErrFailClosed
				}
			}
		}
	}
	acc.BOutputBytes = 0
	var ok bool
	if acc.WWork, ok = CheckedWork(acc); !ok || acc.WWork > r.Limits.MaxWork {
		return Result{}, ErrFailClosed
	}
	res := Result{SchemaVersion: SchemaResultV2, Accepted: false, Authority: 0, Completeness: "UNKNOWN", FeatureIdentity: "UNRESOLVED", Matches: matches, Accounting: acc, Custody: Custody{SchemaVersion: SchemaCustodyV2, Terminal: true, AttemptID: "attempt-0000000000000000", ResultID: "result-0000000000000000", ExactlyOnce: true}, Replay: Replay{SchemaVersion: SchemaReplayV2, CanonicalRequestSHA256: zero(), CanonicalResultSHA256: zero(), ToolingIdentitySHA256: zero()}}
	b, _ := json.Marshal(res)
	res.Accounting.BOutputBytes = uint64(len(b))
	res.Accounting.WWork, ok = CheckedWork(res.Accounting)
	if !ok || res.Accounting.WWork > r.Limits.MaxWork || res.Accounting.BOutputBytes > r.Limits.MaxOutputBytes {
		return Result{}, ErrFailClosed
	}
	b2, _ := json.Marshal(res)
	if uint64(len(b2)) != res.Accounting.BOutputBytes {
		res.Accounting.BOutputBytes = uint64(len(b2))
		res.Accounting.WWork, ok = CheckedWork(res.Accounting)
		if !ok {
			return Result{}, ErrFailClosed
		}
	}
	return res, nil
}

func Position(b []byte, off int) (uint64, uint64) {
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
		r, sz := utf8.DecodeRune(b[i:])
		if r > 0xFFFF {
			ch += 2
		} else {
			ch++
		}
		i += sz
	}
	return line, ch
}
func zero() string { return "sha256:0000000000000000000000000000000000000000000000000000000000000000" }
