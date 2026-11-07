package describeworker

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func suppliedSemanticFixture() string {
	return `{"verdict":"ABSTAINED","target_role":"controlled source only","consumer_need":{"status":"UNRESOLVED","value":""},"provided_behavior":{"value":"controlled fixture","consumer_relative":false},"boundary_contribution":"unknown","limitations":["supplied fixture, not HOST output"]}`
}
func TestSuppliedTerminalResponseParsing(t *testing.T) {
	limits := SuppliedResponseLimits{512 * 1024, 64 * 1024}
	semantic := suppliedSemanticFixture()
	line := EncodeAdapterResultV2(semantic)
	c, err := ParseSuppliedTerminalResponse(line, limits)
	if err != nil || c.RawAdapterDigest() == "" || c.CanonicalSemanticDigest() == "" {
		t.Fatal("ASSERT_SUPPLIED_PARSE", err)
	}
	t.Log("ASSERT_SUPPLIED_PARSE: PASS")
	spaced, err := ParseSuppliedTerminalResponse(EncodeAdapterResultV2(" \n"+semantic+"\t"), limits)
	if err != nil || spaced.CanonicalSemanticDigest() != c.CanonicalSemanticDigest() || spaced.RawAdapterDigest() == c.RawAdapterDigest() {
		t.Fatal("ASSERT_SUPPLIED_NORMALIZATION", err)
	}
	t.Log("ASSERT_SUPPLIED_NORMALIZATION: PASS")
	cases := map[string][]byte{
		"outer-space": append([]byte(" "), line...), "repeated-line": append(append([]byte(nil), line...), line...),
		"missing-newline": line[:len(line)-1], "double-newline": append(append([]byte(nil), line...), '\n'),
		"duplicate":          bytes.Replace(line, []byte(`"status":"COMPLETE"`), []byte(`"status":"COMPLETE","status":"COMPLETE"`), 1),
		"unknown":            bytes.Replace(line, []byte(`"status":`), []byte(`"unknown":1,"status":`), 1),
		"failed":             bytes.Replace(line, []byte(`"status":"COMPLETE"`), []byte(`"status":"FAILED"`), 1),
		"error":              append([]byte(nil), EncodeAdapterResultV2(semantic)...),
		"nonfinite":          bytes.Replace(line, []byte(`"load_ms":0`), []byte(`"load_ms":1e999`), 1),
		"semantic-schema":    EncodeAdapterResultV2(`{"verdict":"COMPLETE"}`),
		"semantic-duplicate": EncodeAdapterResultV2(strings.Replace(semantic, `"verdict":"ABSTAINED"`, `"verdict":"ABSTAINED","verdict":"ABSTAINED"`, 1)),
		"semantic-value":     EncodeAdapterResultV2(strings.Replace(semantic, `"ABSTAINED"`, `"INVALID"`, 1)),
		"semantic-too-large": EncodeAdapterResultV2(strings.Repeat(" ", 64*1024+1)),
	}
	var outer workerResultV2
	_ = json.Unmarshal(line[:len(line)-1], &outer)
	outer.Error = "terminal failure"
	cases["error"], _ = json.Marshal(outer)
	cases["error"] = append(cases["error"], '\n')
	for name, raw := range cases {
		t.Run(name, func(t *testing.T) {
			if _, e := ParseSuppliedTerminalResponse(raw, limits); e == nil {
				t.Fatal("ASSERT_SUPPLIED_STRICT_REJECT", name)
			}
			t.Log("ASSERT_SUPPLIED_STRICT_REJECT: PASS")
		})
	}
	for _, l := range []SuppliedResponseLimits{{}, {len(line) - 1, 64 * 1024}, {512 * 1024, len(semantic) - 1}, {512*1024 + 1, 64 * 1024}, {512 * 1024, 64*1024 + 1}} {
		if _, e := ParseSuppliedTerminalResponse(line, l); e == nil {
			t.Fatal("ASSERT_SUPPLIED_BOUNDS")
		}
	}
	if _, e := ParseSuppliedTerminalResponse(line, SuppliedResponseLimits{len(line), len(semantic)}); e != nil {
		t.Fatal("ASSERT_SUPPLIED_EQUAL_BOUND", e)
	}
	t.Log("ASSERT_SUPPLIED_BOUNDS: PASS")
	c.CanonicalSemantic()[0] = '!'
	c.Semantic().Limitations[0] = "changed"
	if c.Semantic().Limitations[0] == "changed" || !json.Valid(c.CanonicalSemantic()) {
		t.Fatal("ASSERT_SUPPLIED_DEFENSIVE")
	}
	raw, _ := json.Marshal(c)
	var roundtrip SuppliedTerminalCandidate
	if json.Unmarshal(raw, &roundtrip) != nil || roundtrip.RawAdapterDigest() != "" {
		t.Fatal("ASSERT_SUPPLIED_NONADMITTING")
	}
	t.Log("ASSERT_SUPPLIED_DEFENSIVE: PASS; ASSERT_SUPPLIED_NONADMITTING: PASS")
}
