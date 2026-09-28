package adr0011acquisition

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"strconv"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
	"lsp-trace/internal/adr0011querytarget"
)

// targetIdentityV1 is a private calculation, not an admission or publication.
// The caller selects and verifies the three refs independently before invoking it.
func targetIdentityV1(sourceRef, revisionRef, resultRef targetResultRef, query adr0011querytarget.Query, raw []byte) (symbolID, targetID string, err error) {
	if !validTargetIdentityRef(sourceRef, sourceIdentityRole, "source") || !validTargetIdentityRef(revisionRef, revisionIdentityRole, "revision") || !validTargetIdentityRef(resultRef, targetResultRole, "target-result") ||
		!targetResultValidDigest(query.OccurrenceID) || !validTargetIdentityString(query.URI) || query.URI == "" || query.Encoding != "utf-16" || len(raw) == 0 || len(raw) > 1048576 {
		return "", "", errTargetResult
	}
	candidate, e := adr0011querytarget.SelectDocumentSymbolCandidateV1(query, raw)
	if e != nil || !validTargetIdentityString(candidate.SymbolName) || candidate.SymbolKind < 1 || candidate.SymbolKind > 26 {
		return "", "", errTargetResult
	}
	resultDigest := privateDigest(raw) // H(exact raw result), not the candidate's private domain hash.
	symbolID = targetIdentityHash([]byte("REFERENCES_SYMBOL_ID_V1"), targetIdentityRefJSON(sourceRef), targetIdentityRefJSON(revisionRef), []byte(resultDigest), targetIdentityStringJSON(query.URI), targetIdentityStringJSON(candidate.SymbolName), []byte(strconv.Itoa(candidate.SymbolKind)), targetIdentityRangeJSON(candidate.DisplayRange), targetIdentityRangeJSON(candidate.SelectionRange))
	targetID = targetIdentityHash([]byte("REFERENCES_QUERY_TARGET_V1"), []byte(query.OccurrenceID), []byte(symbolID), targetIdentityStringJSON(query.URI), []byte(strconv.FormatUint(uint64(query.Line), 10)), []byte(strconv.FormatUint(uint64(query.Character), 10)), targetIdentityStringJSON(query.Encoding), targetIdentityRefJSON(resultRef))
	return symbolID, targetID, nil
}

func validTargetIdentityString(s string) bool {
	return utf8.ValidString(s) && norm.NFC.IsNormalString(s)
}
func validTargetIdentityRef(r targetResultRef, role, selectorRole string) bool {
	return r.SchemaVersion == role && targetResultValidDigest(r.Digest) && r.Selector == sourceRecordSelector(selectorRole, r.Digest) && validTargetIdentityString(r.Selector)
}
func targetIdentityRefJSON(r targetResultRef) []byte {
	return []byte(`{"digest":` + string(targetIdentityStringJSON(r.Digest)) + `,"schema_version":` + string(targetIdentityStringJSON(r.SchemaVersion)) + `,"selector":` + string(targetIdentityStringJSON(r.Selector)) + `}`)
}
func targetIdentityStringJSON(s string) []byte {
	var b bytes.Buffer
	writeJCSString(&b, s)
	return b.Bytes()
}
func targetIdentityRangeJSON(r adr0011querytarget.Range) []byte {
	point := func(p adr0011querytarget.Position) string {
		return fmt.Sprintf(`{"character":%d,"line":%d}`, p.Character, p.Line)
	}
	return []byte(`{"end":` + point(r.End) + `,"start":` + point(r.Start) + `}`)
}
func targetIdentityHash(parts ...[]byte) string {
	h := sha256.New()
	var length [8]byte
	for _, part := range parts {
		binary.BigEndian.PutUint64(length[:], uint64(len(part)))
		_, _ = h.Write(length[:])
		_, _ = h.Write(part)
	}
	return fmt.Sprintf("sha256:%x", h.Sum(nil))
}
