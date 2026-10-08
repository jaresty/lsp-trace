package adr0007sourcetextsearchv1

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"unicode/utf8"
)

var ErrFailClosed = errors.New("adr0007 source text search fail closed")

func Work(files, queryBytes, pathBytes, sourceBytes, scannedTuples, matches, rangeUnits, utf16Units, outputBytes int) int {
	return 50 + 3*files + 5*queryBytes + 7*pathBytes + sourceBytes + 11*scannedTuples + 13*matches + 17*rangeUnits + 19*utf16Units + 31*outputBytes
}

func EmptyAccounting() Accounting { return Accounting{} }

func UTF16Offset(b []byte, byteOffset int) int {
	if byteOffset <= 0 {
		return 0
	}
	if byteOffset > len(b) {
		byteOffset = len(b)
	}
	n := 0
	for i := 0; i < byteOffset; {
		r, sz := utf8.DecodeRune(b[i:byteOffset])
		if r == utf8.RuneError && sz == 1 {
			n++
			i++
			continue
		}
		if r > 0xFFFF {
			n += 2
		} else {
			n++
		}
		i += sz
	}
	return n
}

func LiteralMatches(source []byte, query string) []Range {
	q := []byte(query)
	if len(q) == 0 {
		return nil
	}
	out := []Range{}
	for i := 0; i+len(q) <= len(source); i++ {
		ok := true
		for j := 0; j < len(q); j++ {
			if source[i+j] != q[j] {
				ok = false
				break
			}
		}
		if ok {
			out = append(out, Range{ByteStart: i, ByteEnd: i + len(q), UTF16Start: UTF16Offset(source, i), UTF16End: UTF16Offset(source, i+len(q))})
		}
	}
	return out
}

func Search(input Input, source []byte) (Result, *Failure) {
	if input.SchemaVersion != SchemaInputV1 || input.Query == "" || !utf8.ValidString(input.Query) {
		return Result{}, fail("INVALID_INPUT", "schema_version must match and query must be nonempty valid UTF-8")
	}
	if input.Source.Path == "" || input.Source.Revision == "" || input.Source.FileSHA256 == "" || input.Source.ObjectID == "" || input.Source.AdmissionID == "" || input.Source.Seal == "" || strings.Contains(input.Source.Path, "..") {
		return Result{}, fail("SOURCE_BINDING_MISMATCH", "path, revision, file, object, admission, and seal bindings are required")
	}
	if input.Limits.MaxMatches < 0 || input.Limits.MaxOutputBytes <= 0 || input.Limits.MaxWork <= 0 || input.Limits.MaxSourceBytes < 0 || input.Limits.MaxPathBytes < 0 {
		return Result{}, fail("INVALID_LIMITS", "limits must be explicit nonnegative with positive output/work")
	}
	if len(source) > input.Limits.MaxSourceBytes || len([]byte(input.Source.Path)) > input.Limits.MaxPathBytes {
		return Result{}, fail("LIMIT_EXCEEDED", "source or path exceeds pre-admission limits")
	}
	got := sha256.Sum256(source)
	gotHex := "sha256:" + hex.EncodeToString(got[:])
	if input.Source.FileSHA256 != gotHex {
		return Result{}, fail("SOURCE_BINDING_MISMATCH", "actual source bytes do not match bound file_sha256")
	}
	ranges := LiteralMatches(source, input.Query)
	if len(ranges) > input.Limits.MaxMatches {
		return Result{}, fail("LIMIT_EXCEEDED", "match count exceeds max_matches")
	}
	matches := make([]Match, len(ranges))
	for i, r := range ranges {
		matches[i] = Match{Path: input.Source.Path, Revision: input.Source.Revision, FileSHA256: gotHex, ObjectID: input.Source.ObjectID, AdmissionID: input.Source.AdmissionID, Seal: input.Source.Seal, Range: r, Literal: input.Query}
	}
	res := Result{SchemaVersion: SchemaResultV1, Accepted: false, Authority: 0, Completeness: "UNKNOWN", FeatureIdentity: "UNRESOLVED", Matches: matches}
	out, _ := json.Marshal(res)
	acc := Accounting{Files: 1, Bytes: len(source), QueryBytes: len([]byte(input.Query)), PathBytes: len([]byte(input.Source.Path)), Matches: len(matches), OutputBytes: len(out)}
	acc.Work = Work(acc.Files, acc.QueryBytes, acc.PathBytes, acc.Bytes, 1, acc.Matches, acc.Matches, utf16Units(ranges), acc.OutputBytes)
	if acc.Work > input.Limits.MaxWork || acc.OutputBytes > input.Limits.MaxOutputBytes {
		return Result{}, fail("LIMIT_EXCEEDED", "checked precharge or output exceeds limit")
	}
	res.Accounting = acc
	return res, nil
}

func fail(code, msg string) *Failure {
	return &Failure{SchemaVersion: SchemaFailureV1, Accepted: false, Authority: 0, Completeness: "UNKNOWN", FeatureIdentity: "UNRESOLVED", Code: code, Message: msg, Accounting: EmptyAccounting()}
}
func utf16Units(rs []Range) int {
	n := 0
	for _, r := range rs {
		n += r.UTF16End - r.UTF16Start
	}
	return n
}
