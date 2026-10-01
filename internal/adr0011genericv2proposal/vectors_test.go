package adr0011genericv2proposal

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func v2sum(b []byte) string { sum := sha256.Sum256(b); return hex.EncodeToString(sum[:]) }
func v2selector(method, role string, artifact, schema, transport, policy []byte, version string, line int, ordinal *int) string {
	name := "GENERIC_LSP_REFERENCES_EXACT_V2"
	if method == "textDocument/definition" {
		name = "GENERIC_LSP_DEFINITION_EXACT_V2"
	}
	parts := []string{"ADR0011-GENERIC-EXACT/2", name, method, role, v2sum(schema), v2sum(transport), v2sum(policy), "s", "1", v2sum(artifact)}
	if role == "QUERY" {
		parts = append(parts, "file:///a", version, v2sum([]byte("a")), "utf-16", strconv.Itoa(line), "0")
	}
	if role == "TARGET_EVENTS" {
		parts = append(parts, "observed-key-1", v2sum([]byte("[]")), strconv.Itoa(*ordinal))
	}
	h := sha256.New()
	for _, part := range parts {
		b := []byte(part)
		var size [8]byte
		binary.BigEndian.PutUint64(size[:], uint64(len(b)))
		h.Write(size[:])
		h.Write(b)
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil))
}

func TestV2OnlyAuthorizedOriginalDifferences(t *testing.T) {
	r := originals(t)
	read := func(name string) map[string]any {
		t.Helper()
		b, err := os.ReadFile(filepath.Join(r, name))
		if err != nil {
			t.Fatal(err)
		}
		var value map[string]any
		if err := json.Unmarshal(b, &value); err != nil {
			t.Fatal(err)
		}
		return value
	}
	for _, stem := range []string{"generic-lsp-exact-transport", "generic-lsp-references-exact", "generic-lsp-definition-exact"} {
		old := read(stem + "-v1.json")
		current := read(stem + "-v2.json")
		for _, key := range []string{"policyId", "version", "selector", "schema", "transport"} {
			delete(old, key)
			delete(current, key)
		}
		if !reflect.DeepEqual(old, current) {
			t.Fatalf("%s changed a non-identity field", stem)
		}
	}
	old := read("adr0011-generic-envelope-v1.schema.json")
	current := read("adr0011-generic-envelope-v2.schema.json")
	current["$id"] = old["$id"]
	d := current["$defs"].(map[string]any)
	delete(d, "TRANSACTION")
	base := d["base"].(map[string]any)["properties"].(map[string]any)["predecessors"].(map[string]any)
	base["maxItems"] = float64(32)
	for _, role := range []string{"TARGET_EVENTS", "READBACK"} {
		p := d[role].(map[string]any)["allOf"].([]any)[1].(map[string]any)["properties"].(map[string]any)["predecessors"].(map[string]any)
		delete(p, "maxItems")
	}
	terminal := d["TERMINAL"].(map[string]any)["allOf"].([]any)[1].(map[string]any)["properties"].(map[string]any)
	delete(terminal, "predecessors")
	if !reflect.DeepEqual(old, current) {
		t.Fatal("V2 schema changed outside selected identity, predecessor bounds and transaction cardinalities")
	}
}

func TestV2PythonGoSelectorVectorsAndV1Pins(t *testing.T) {
	r := originals(t)
	read := func(name string) []byte {
		t.Helper()
		b, err := os.ReadFile(filepath.Join(r, name))
		if err != nil {
			t.Fatal(err)
		}
		return b
	}
	for name, want := range map[string]string{
		"adr0011-generic-envelope-v1.schema.json": "efa909a074fa8b7e8949f7e70395f50312a384f0d0bd95b65168844fa6c8a9bd",
		"generic-lsp-exact-transport-v1.json":     "6fbb54cf37ec6efe86cb42e8717f9610716b8a84e432a57367135b8da9e265ae",
		"generic-lsp-references-exact-v1.json":    "5d99986050dd33ff0fffaf623bd8a940a9675053361c8d137a749413dff7311a",
		"generic-lsp-definition-exact-v1.json":    "466994a66fadd65ff80692b5d0284de221d0cd1b6907ea3b11a0652400697648",
		"generic-lsp-final-selector-vectors.json": "d511db2c3c1454eadbb197aef747ab5441c7bcfc96dd2d960b067e12badec9f0",
	} {
		if got := v2sum(read(name)); got != want {
			t.Fatalf("historical V1 %s changed: %s", name, got)
		}
	}
	schema := read("adr0011-generic-envelope-v2.schema.json")
	transport := read("generic-lsp-exact-transport-v2.json")
	var fixture struct {
		Domain       string            `json:"domain"`
		SchemaSHA    string            `json:"schema_sha256"`
		TransportSHA string            `json:"transport_sha256"`
		QueryHex     string            `json:"query_artifact_utf8_hex"`
		EventHex     string            `json:"event_artifact_utf8_hex"`
		PolicySHA    map[string]string `json:"policy_sha256"`
		Selectors    map[string]string `json:"selectors"`
	}
	if err := json.Unmarshal(read("generic-lsp-v2-selector-vectors.json"), &fixture); err != nil {
		t.Fatal(err)
	}
	if fixture.Domain != "ADR0011-GENERIC-EXACT/2" || fixture.SchemaSHA != v2sum(schema) || fixture.TransportSHA != v2sum(transport) {
		t.Fatal("V2 original digest or domain mismatch")
	}
	query, err := hex.DecodeString(fixture.QueryHex)
	if err != nil {
		t.Fatal(err)
	}
	event, err := hex.DecodeString(fixture.EventHex)
	if err != nil {
		t.Fatal(err)
	}
	for _, suffix := range []string{"references", "definition"} {
		method := "textDocument/" + suffix
		policy := read("generic-lsp-" + suffix + "-exact-v2.json")
		if fixture.PolicySHA[suffix] != v2sum(policy) {
			t.Fatalf("%s policy digest", suffix)
		}
		zero, one := 0, 1
		expected := map[string]string{
			"query_version1_line0":      v2selector(method, "QUERY", query, schema, transport, policy, "buffer:v1", 0, nil),
			"query_version2_line0":      v2selector(method, "QUERY", query, schema, transport, policy, "buffer:v2", 0, nil),
			"query_version1_line1":      v2selector(method, "QUERY", query, schema, transport, policy, "buffer:v1", 1, nil),
			"event_ordinal0":            v2selector(method, "TARGET_EVENTS", event, schema, transport, policy, "", 0, &zero),
			"event_ordinal1":            v2selector(method, "TARGET_EVENTS", event, schema, transport, policy, "", 0, &one),
			"query_schema_substitution": v2selector(method, "QUERY", query, append(append([]byte{}, schema...), '\n'), transport, policy, "buffer:v1", 0, nil),
			"query_policy_substitution": v2selector(method, "QUERY", query, schema, transport, append(append([]byte{}, policy...), '\n'), "buffer:v1", 0, nil),
		}
		for key, want := range expected {
			if got := fixture.Selectors[suffix+"_"+key]; got != want {
				t.Fatalf("%s %s: Python=%s Go=%s", suffix, key, got, want)
			}
		}
		var p map[string]any
		if err := json.Unmarshal(policy, &p); err != nil {
			t.Fatal(err)
		}
		if p["selector"] != "GENERIC_LSP_"+strings.ToUpper(suffix)+"_EXACT_V2" || p["version"] != float64(2) || p["transport"] != "GENERIC_LSP_EXACT_TRANSPORT_V2" {
			t.Fatalf("%s V2 identity", suffix)
		}
	}
	if len(fixture.Selectors) != 14 {
		t.Fatalf("V2 vector count %d", len(fixture.Selectors))
	}
}
