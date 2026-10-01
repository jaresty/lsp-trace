package adr0011genericproposal

// Proposal-only cross-language selector verification. No publisher or admission path imports this package.

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

func finalSHA(b []byte) string { sum := sha256.Sum256(b); return hex.EncodeToString(sum[:]) }
func finalField(s string) []byte {
	b := []byte(s)
	out := make([]byte, 8+len(b))
	binary.BigEndian.PutUint64(out[:8], uint64(len(b)))
	copy(out[8:], b)
	return out
}
func finalSelector(role string, artifact []byte, schema, transport, policy []byte, version string, line int, ordinal *int) string {
	parts := []string{"ADR0011-GENERIC-EXACT/1", "GENERIC_LSP_REFERENCES_EXACT_V1", "textDocument/references", role,
		finalSHA(schema), finalSHA(transport), finalSHA(policy), "s", "1", finalSHA(artifact)}
	if role == "QUERY" {
		parts = append(parts, "file:///a", version, finalSHA([]byte("a")), "utf-16", strconv.Itoa(line), "0")
	}
	if role == "TARGET_EVENTS" {
		parts = append(parts, "observed-key-1", finalSHA([]byte("[]")), strconv.Itoa(*ordinal))
	}
	var preimage []byte
	for _, p := range parts {
		preimage = append(preimage, finalField(p)...)
	}
	return "sha256:" + finalSHA(preimage)
}
func finalUniqueJSON(raw []byte) error {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var visit func() error
	visit = func() error {
		token, err := dec.Token()
		if err != nil {
			return err
		}
		delim, ok := token.(json.Delim)
		if !ok {
			return nil
		}
		if delim == '{' {
			seen := make(map[string]bool)
			for dec.More() {
				k, err := dec.Token()
				if err != nil {
					return err
				}
				key, ok := k.(string)
				if !ok {
					return errors.New("invalid key")
				}
				if seen[key] {
					return errors.New("duplicate decoded key")
				}
				seen[key] = true
				if err := visit(); err != nil {
					return err
				}
			}
			_, err := dec.Token()
			return err
		}
		if delim == '[' {
			for dec.More() {
				if err := visit(); err != nil {
					return err
				}
			}
			_, err := dec.Token()
			return err
		}
		return errors.New("unexpected delimiter")
	}
	if err := visit(); err != nil {
		return err
	}
	if _, err := dec.Token(); err != io.EOF {
		return errors.New("trailing JSON")
	}
	return nil
}
func TestFinalGenericPythonGoVectors(t *testing.T) {
	root := originals(t)
	read := func(name string) []byte {
		t.Helper()
		b, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			t.Fatal(err)
		}
		if bytes.Contains(b, []byte(".proposed")) {
			t.Fatalf("final original %s contains proposed identity", name)
		}
		return b
	}
	schema := read("adr0011-generic-envelope-v1.schema.json")
	transport := read("generic-lsp-exact-transport-v1.json")
	policy := read("generic-lsp-references-exact-v1.json")
	definition := read("generic-lsp-definition-exact-v1.json")
	var s map[string]any
	if err := json.Unmarshal(schema, &s); err != nil {
		t.Fatal(err)
	}
	schemaURI := "https://jaresty.github.io/lsp-trace/schemas/adr0011-generic-envelope-v1.schema.json"
	if s["$id"] != schemaURI {
		t.Fatal("final schema URI")
	}
	for name, b := range map[string][]byte{"generic-lsp-exact-transport-v1.json": transport, "generic-lsp-references-exact-v1.json": policy, "generic-lsp-definition-exact-v1.json": definition} {
		var p map[string]any
		if err := json.Unmarshal(b, &p); err != nil {
			t.Fatal(err)
		}
		uri := "https://jaresty.github.io/lsp-trace/policies/" + name
		if p["policyId"] != uri || p["version"] != float64(1) {
			t.Fatalf("final policy ID %s", name)
		}
		if name == "generic-lsp-exact-transport-v1.json" && p["schema"] != schemaURI {
			t.Fatal("transport schema URI")
		}
	}
	b := read("generic-lsp-final-selector-vectors.json")
	var v struct {
		Domain, SchemaSHA, TransportSHA, ReferencesSHA, QueryHex, EventHex, UnicodeHex, Duplicate string
		Selectors                                                                                 map[string]string
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatal(err)
	}
	for key, dest := range map[string]*string{"domain": &v.Domain, "schema_sha256": &v.SchemaSHA, "transport_sha256": &v.TransportSHA, "references_sha256": &v.ReferencesSHA, "query_artifact_utf8_hex": &v.QueryHex, "event_artifact_utf8_hex": &v.EventHex, "unicode_canonical_utf8_hex": &v.UnicodeHex, "escaped_duplicate_json": &v.Duplicate} {
		if err := json.Unmarshal(raw[key], dest); err != nil {
			t.Fatal(key, err)
		}
	}
	if err := json.Unmarshal(raw["selectors"], &v.Selectors); err != nil {
		t.Fatal(err)
	}
	if v.Domain != "ADR0011-GENERIC-EXACT/1" || v.SchemaSHA != finalSHA(schema) || v.TransportSHA != finalSHA(transport) || v.ReferencesSHA != finalSHA(policy) {
		t.Fatal("vector original identity")
	}
	query, _ := hex.DecodeString(v.QueryHex)
	event, _ := hex.DecodeString(v.EventHex)
	zero, one := 0, 1
	expected := map[string]string{
		"query_version1_line0":      finalSelector("QUERY", query, schema, transport, policy, "buffer:v1", 0, nil),
		"query_version2_line0":      finalSelector("QUERY", query, schema, transport, policy, "buffer:v2", 0, nil),
		"query_version1_line1":      finalSelector("QUERY", query, schema, transport, policy, "buffer:v1", 1, nil),
		"event_ordinal0":            finalSelector("TARGET_EVENTS", event, schema, transport, policy, "", 0, &zero),
		"event_ordinal1":            finalSelector("TARGET_EVENTS", event, schema, transport, policy, "", 0, &one),
		"query_schema_substitution": finalSelector("QUERY", query, append(bytes.Clone(schema), '\n'), transport, policy, "buffer:v1", 0, nil),
		"query_policy_substitution": finalSelector("QUERY", query, schema, transport, append(bytes.Clone(policy), '\n'), "buffer:v1", 0, nil),
	}
	if len(v.Selectors) != len(expected) {
		t.Fatal("vector cardinality")
	}
	distinct := map[string]bool{}
	for k, want := range expected {
		if got := v.Selectors[k]; got != want {
			t.Fatalf("%s: Python=%s Go=%s", k, got, want)
		}
		distinct[want] = true
	}
	if len(distinct) != len(expected) {
		t.Fatal("identity collision")
	}
	unicode, err := hex.DecodeString(v.UnicodeHex)
	if err != nil {
		t.Fatal(err)
	}
	// Go sorts valid UTF-8 map keys by code point, unlike UTF-16 surrogate ordering.
	names := []string{"\ue000", "\U0001f600"}
	sort.Strings(names)
	var ordered bytes.Buffer
	ordered.WriteByte('{')
	for i, k := range names {
		if i > 0 {
			ordered.WriteByte(',')
		}
		key, _ := json.Marshal(k)
		value, _ := json.Marshal(map[string]string{"\ue000": "p", "\U0001f600": "s"}[k])
		ordered.Write(key)
		ordered.WriteByte(':')
		ordered.Write(value)
	}
	ordered.WriteByte('}')
	if !bytes.Equal(unicode, ordered.Bytes()) {
		t.Fatalf("Unicode canonical mismatch: %x != %x", unicode, ordered.Bytes())
	}
	if err := finalUniqueJSON([]byte(v.Duplicate)); err == nil || !strings.Contains(err.Error(), "duplicate decoded key") {
		t.Fatalf("escaped duplicate accepted: %v", err)
	}
	// Corrected proposed RESULT_READ branch must survive identity-only finalization.
	c := jsonschema.NewCompiler()
	c.AssertFormat()
	if err := c.AddResource(schemaURI, s); err != nil {
		t.Fatal(err)
	}
	compiled, err := c.Compile(schemaURI)
	if err != nil {
		t.Fatal(err)
	}
	digest := "sha256:" + strings.Repeat("0", 64)
	original := map[string]any{"length": 1, "sha256": digest, "private_ref": "private:one"}
	result := map[string]any{"role": "RESULT_READ", "identity": map[string]any{"session": "s", "generation": 1, "transaction": "t"}, "original": original, "predecessors": []any{map[string]any{"role": "INBOUND_FRAMES", "selector": "f", "digest": digest}, map[string]any{"role": "REQUEST_WRITE", "selector": "w", "digest": digest}}, "payload": map[string]any{"actual_key": "k", "matched_frame_index": 0, "result_present": true, "result_token": nil, "parse": "WHOLE_PARSE_SUCCESS"}}
	marshaled, _ := json.Marshal(result)
	var decoded any
	if err := json.Unmarshal(marshaled, &decoded); err != nil {
		t.Fatal(err)
	}
	if compiled.Validate(decoded) == nil {
		t.Fatal("RESULT_READ true/null accepted")
	}
}
