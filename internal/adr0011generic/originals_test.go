package adr0011generic

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestEmbeddedFinalOriginalsRejectSubstitution(t *testing.T) {
	for role, pin := range originalPins {
		original, err := loadOriginal(role)
		if err != nil {
			t.Fatalf("%s: %v", role, err)
		}
		if len(original) != pin.length || digest(original) != pin.hash {
			t.Fatalf("%s pin mismatch", role)
		}
		if err := verifyOriginal(role, original); err != nil {
			t.Fatalf("%s original rejected: %v", role, err)
		}
		altered := bytes.Clone(original)
		altered[0] ^= 1
		if err := verifyOriginal(role, altered); err == nil {
			t.Fatalf("%s substituted original accepted", role)
		}
		if len(altered) > 1 && digest(altered) == pin.hash {
			t.Fatal("test mutation did not change digest")
		}
		original[0] ^= 1
		if again, e := loadOriginal(role); e != nil || digest(again) != pin.hash {
			t.Fatalf("%s embedded expectation mutated", role)
		}
	}
	if err := verifyOriginal("UNKNOWN", []byte("x")); err == nil {
		t.Fatal("unknown role accepted")
	}
}
func TestCanonicalMetadataAndDecodedDuplicate(t *testing.T) {
	v := map[string]any{"\U0001f600": "s", "\ue000": "p"}
	raw, err := canonicalMetadata(v)
	if err != nil {
		t.Fatal(err)
	}
	if want := []byte("{\"\ue000\":\"p\",\"\U0001f600\":\"s\"}"); !bytes.Equal(raw, want) {
		t.Fatalf("Unicode-scalar order got %x want %x", raw, want)
	}
	if _, err := readCanonicalMetadata(raw); err != nil {
		t.Fatal(err)
	}
	if _, err := parseMetadata([]byte(`{"uri":1,"\u0075ri":2}`)); err == nil {
		t.Fatal("escaped duplicate decoded key accepted before canonicalization")
	}
	for _, bad := range [][]byte{[]byte(`{"uri":1,"\u0075ri":2}`), []byte(`{"outer":{"uri":1,"\u0075ri":2}}`), []byte(`{"a":1 }`), []byte(`{"n":1e0}`), []byte(`{"n":-0}`), []byte("{\"x\":\"\xff\"}")} {
		if _, err := readCanonicalMetadata(bad); err == nil {
			t.Fatalf("bad metadata accepted: %q", bad)
		}
	}
	for _, x := range []string{"<", "&", "\u2028", "\n"} {
		canonical, err := canonicalMetadata(map[string]any{"x": x})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := readCanonicalMetadata(canonical); err != nil {
			t.Fatalf("canonical string %q: %v", x, err)
		}
	}
}
func TestCanonicalMetadataRequiresObjectRoot(t *testing.T) {
	for _, raw := range [][]byte{[]byte(`null`), []byte(`[]`), []byte(`"x"`)} {
		if _, err := readCanonicalMetadata(raw); err == nil {
			t.Fatalf("non-object metadata root accepted: %q", raw)
		}
	}
	if _, err := canonicalMetadata([]any{}); err == nil {
		t.Fatal("non-object metadata root encoded")
	}
}

func checkFrozenVectorFixture(b []byte) error {
	if len(b) != 1429 || digest(b) != "d511db2c3c1454eadbb197aef747ab5441c7bcfc96dd2d960b067e12badec9f0" {
		return errors.New("frozen selector fixture identity mismatch")
	}
	return nil
}

// This test-only calculation can substitute hypothetical policy bytes; the
// production selector deliberately cannot accept a caller-selected policy.
func hypotheticalDefinitionSelector(role string, artifact, schema, transport, policy []byte) string {
	parts := []string{"ADR0011-GENERIC-EXACT/1", "GENERIC_LSP_DEFINITION_EXACT_V1", "textDocument/definition", role,
		digest(schema), digest(transport), digest(policy), "s", "1", digest(artifact)}
	if role == "QUERY" {
		parts = append(parts, "file:///a", "buffer:v1", digest([]byte("a")), "utf-16", "0", "0")
	} else {
		parts = append(parts, "observed-key-1", digest([]byte("[]")), "0")
	}
	var preimage []byte
	for _, part := range parts {
		var length [8]byte
		binary.BigEndian.PutUint64(length[:], uint64(len([]byte(part))))
		preimage = append(preimage, length[:]...)
		preimage = append(preimage, []byte(part)...)
	}
	return "sha256:" + digest(preimage)
}

// These definition expectations were calculated independently with Python
// hashlib/struct over the final originals. They supplement (not alter) the
// seven accepted references-only fixture vectors.
func TestDefinitionSelectorAndPolicySubstitution(t *testing.T) {
	_, file, _, _ := runtime.Caller(0)
	fixture, err := os.ReadFile(filepath.Join(filepath.Dir(file), "..", "..", "docs", "qualification", "originals", "generic-lsp-final-selector-vectors.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := checkFrozenVectorFixture(fixture); err != nil {
		t.Fatal(err)
	}
	var v struct {
		QueryHex string `json:"query_artifact_utf8_hex"`
		EventHex string `json:"event_artifact_utf8_hex"`
	}
	if err := json.Unmarshal(fixture, &v); err != nil {
		t.Fatal(err)
	}
	query, err := hex.DecodeString(v.QueryHex)
	if err != nil {
		t.Fatal(err)
	}
	event, err := hex.DecodeString(v.EventHex)
	if err != nil {
		t.Fatal(err)
	}
	in := selectorInput{Method: "textDocument/definition", Role: "QUERY", SessionID: "s", Generation: 1, Artifact: query, URI: "file:///a", Version: "buffer:v1", SourceDigest: digest([]byte("a")), Encoding: "utf-16"}
	const definitionQuery = "sha256:829ec3c29f715529e48fbe01527c5f944d6c57d828880636f957392f60a10144"
	const substitutedQuery = "sha256:3da7f56c76854b891db65bd82f66aef0a26716c6477002b2183d6690078b01a8"
	const definitionEvent = "sha256:61ee825b6226149e28df605ee37c101a99be23796a7a67c15bb3de378f1c29d6"
	const substitutedEvent = "sha256:1807a41451ea874828be2a12a3f4341cca6601c99d9d23b2e4a38b6270bb97b7"
	if got := historicalSelector(t, in); got != definitionQuery {
		t.Fatalf("historical definition QUERY selector got %q want %q", got, definitionQuery)
	}
	in.Role, in.Artifact, in.ActualWriteKey, in.ResultDigest = "TARGET_EVENTS", event, "observed-key-1", digest([]byte("[]"))
	if got := historicalSelector(t, in); got != definitionEvent {
		t.Fatalf("historical definition TARGET_EVENTS selector got %q want %q", got, definitionEvent)
	}
	original := historicalOriginal(t, "DEFINITION")
	if !historicalVerify(t, "DEFINITION", original) {
		t.Fatal("historical definition original rejected")
	}
	substituted := append(bytes.Clone(original), '\n')
	if historicalVerify(t, "DEFINITION", substituted) {
		t.Fatal("historical definition-policy LF substitution accepted")
	}
	schema := historicalOriginal(t, "SCHEMA")
	transport := historicalOriginal(t, "TRANSPORT")
	for _, tc := range []struct {
		role, positive, negative string
		artifact                 []byte
	}{
		{"QUERY", definitionQuery, substitutedQuery, query},
		{"TARGET_EVENTS", definitionEvent, substitutedEvent, event},
	} {
		if got := hypotheticalDefinitionSelector(tc.role, tc.artifact, schema, transport, original); got != tc.positive {
			t.Fatalf("definition %s independently calculated positive selector got %q want %q", tc.role, got, tc.positive)
		}
		if got := hypotheticalDefinitionSelector(tc.role, tc.artifact, schema, transport, substituted); got != tc.negative {
			t.Fatalf("definition %s LF-substitution vector got %q want %q", tc.role, got, tc.negative)
		}
	}
	if definitionQuery == substitutedQuery || definitionEvent == substitutedEvent {
		t.Fatal("definition-policy LF substitution did not separate selectors")
	}
}

func TestFrozenSelectorVectors(t *testing.T) {
	_, file, _, _ := runtime.Caller(0)
	b, err := os.ReadFile(filepath.Join(filepath.Dir(file), "..", "..", "docs", "qualification", "originals", "generic-lsp-final-selector-vectors.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := checkFrozenVectorFixture(b); err != nil {
		t.Fatal(err)
	}
	mutated := bytes.Clone(b)
	mutated[0] ^= 1
	if err := checkFrozenVectorFixture(mutated); err == nil {
		t.Fatal("substituted frozen selector fixture accepted")
	}
	var v struct {
		QueryHex  string            `json:"query_artifact_utf8_hex"`
		EventHex  string            `json:"event_artifact_utf8_hex"`
		Selectors map[string]string `json:"selectors"`
	}
	if err := json.Unmarshal(b, &v); err != nil {
		t.Fatal(err)
	}
	query, err := hex.DecodeString(v.QueryHex)
	if err != nil {
		t.Fatal(err)
	}
	event, err := hex.DecodeString(v.EventHex)
	if err != nil {
		t.Fatal(err)
	}
	base := selectorInput{Method: "textDocument/references", Role: "QUERY", SessionID: "s", Generation: 1, Artifact: query, URI: "file:///a", Version: "buffer:v1", SourceDigest: digest([]byte("a")), Encoding: "utf-16"}
	check := func(name string, in selectorInput) {
		t.Helper()
		got := historicalSelector(t, in)
		if got != v.Selectors[name] {
			t.Fatalf("historical %s selector got %s want %s", name, got, v.Selectors[name])
		}
	}
	check("query_version1_line0", base)
	changed := base
	changed.Version = "buffer:v2"
	check("query_version2_line0", changed)
	changed = base
	changed.Line = 1
	check("query_version1_line1", changed)
	occ := selectorInput{Method: base.Method, Role: "TARGET_EVENTS", SessionID: "s", Generation: 1, Artifact: event, ActualWriteKey: "observed-key-1", ResultDigest: digest([]byte("[]"))}
	check("event_ordinal0", occ)
	occ.Ordinal = 1
	check("event_ordinal1", occ)
	if v.Selectors["query_schema_substitution"] == v.Selectors["query_version1_line0"] || v.Selectors["query_policy_substitution"] == v.Selectors["query_version1_line0"] {
		t.Fatal("original substitution did not change selector")
	}
	if _, err := calculateSelector(selectorInput{Method: base.Method, Role: base.Role, SessionID: "s", Generation: 1, Artifact: query, URI: base.URI, Version: base.Version, SourceDigest: strings.Repeat("0", 64), Encoding: "utf-16"}); err != nil {
		t.Fatal(err)
	} // Different source bytes identify a different transaction, not an allowlist refusal.
	for _, bad := range []selectorInput{{Method: "unknown", Role: base.Role, SessionID: "s", Generation: 1, Artifact: query}, {Method: base.Method, Role: base.Role, SessionID: "s", Generation: 1, Artifact: query, URI: base.URI, Version: "1", SourceDigest: base.SourceDigest, Encoding: "utf-16"}} {
		if _, err := calculateSelector(bad); err == nil {
			t.Fatal("invalid selector identity accepted")
		}
	}
}
