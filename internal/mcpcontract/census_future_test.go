package mcpcontract

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"

	jsonschema "github.com/santhosh-tekuri/jsonschema/v6"
)

func futureCensusSchemas(t *testing.T) map[string]*jsonschema.Schema {
	t.Helper()
	c := jsonschema.NewCompiler()
	c.DefaultDraft(jsonschema.Draft2020)
	ids := []string{FutureCensusInputID, FutureCensusResultID, FutureCensusSuccessID, FutureCensusDomainErrorID}
	for _, id := range ids {
		raw, e := FutureCensusSchemaJSON(id)
		if e != nil {
			t.Fatal(e)
		}
		doc, e := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
		if e != nil {
			t.Fatal(e)
		}
		if e = c.AddResource(id, doc); e != nil {
			t.Fatal(e)
		}
	}
	out := map[string]*jsonschema.Schema{}
	for _, id := range ids {
		s, e := c.Compile(id)
		if e != nil {
			t.Fatal(e)
		}
		out[id] = s
	}
	return out
}
func schemaAccepts(t *testing.T, s *jsonschema.Schema, raw []byte, want bool) {
	t.Helper()
	v, e := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	if e == nil {
		e = s.Validate(v)
	}
	if (e == nil) != want {
		t.Fatalf("valid=%v want=%v err=%v raw=%s", e == nil, want, e, raw)
	}
}
func decodeMap(t *testing.T, raw string) map[string]any {
	t.Helper()
	var v map[string]any
	d := json.NewDecoder(strings.NewReader(raw))
	d.UseNumber()
	if e := d.Decode(&v); e != nil {
		t.Fatal(e)
	}
	return v
}

func TestFutureCensusSchemaBytesAndIDMirrors(t *testing.T) {
	files := map[string]string{FutureCensusInputID: "input-census.v1.schema.json", FutureCensusResultID: "lsp-trace.census-result.v1.schema.json", FutureCensusSuccessID: "envelope-census-result.v1.schema.json", FutureCensusDomainErrorID: "envelope-census-domain-error.v1.schema.json"}
	futureCensusSchemas(t)
	for id, name := range files {
		got, e := FutureCensusSchemaJSON(id)
		if e != nil {
			t.Fatal(e)
		}
		want, e := os.ReadFile("testdata/schemas/" + name)
		if e != nil {
			t.Fatal(e)
		}
		if !bytes.Equal(got, want) {
			t.Fatalf("schema bytes differ: %s", id)
		}
		var doc map[string]any
		if e = json.Unmarshal(got, &doc); e != nil || doc["$id"] != id {
			t.Fatalf("schema id mirror %s: %v %v", id, doc["$id"], e)
		}
	}
}
func TestFutureCensusStrictParserPrecedenceAndRequestSemantics(t *testing.T) {
	valid := `{"session_id":"s","generation":1,"sources":["."]}`
	if _, e := DecodeFutureCensusRequestV1([]byte(valid)); e != nil {
		t.Fatal(e)
	}
	if e := ValidateFutureCensusRequestV1(map[string]any{"session_id": "s", "generation": uint64(1), "sources": []any{"."}}); e != nil {
		t.Fatalf("standalone map semantics rejected integer input: %v", e)
	}
	if _, e := DecodeFutureCensusRequestV1([]byte(`{"unknown":1,"unknown":2}`)); !errors.Is(e, errFutureCensusDuplicate) {
		t.Fatalf("complete duplicate must precede unknown/type semantics: %v", e)
	}
	for name, raw := range map[string]string{
		"top_level_incomplete": `{"session_id":"s","generation":1,"sources":["."],"x":1,"x":`,
		"nested_incomplete":    `{"session_id":"s","generation":1,"sources":["."],"nested":{"x":1,"x":2`,
	} {
		t.Run("parse_invalid_before_duplicate_"+name, func(t *testing.T) {
			_, e := DecodeFutureCensusRequestV1([]byte(raw))
			if !errors.Is(e, errFutureCensusShape) || errors.Is(e, errFutureCensusDuplicate) {
				t.Fatalf("ASSERT_CENSUS_COMPLETE_VALIDITY_BEFORE_DUPLICATE: got %v", e)
			}
		})
	}
	for name, raw := range map[string]string{"nil": "null", "type": "[]", "unknown": `{"session_id":"s","generation":1,"sources":["."],"output_selector":"x"}`, "trailing": valid + ` {}`, "duplicate": `{"session_id":"s","generation":1,"sources":["."],"sources":["a"]}`} {
		t.Run(name, func(t *testing.T) {
			if _, e := DecodeFutureCensusRequestV1([]byte(raw)); e == nil {
				t.Fatal("accepted")
			}
		})
	}
	for _, field := range []string{"output_selector", "publication_root", "path", "bytes", "artifact", "hydration", "retained", "source_supply", "server", "profile", "command", "args", "env", "workspace_revision", "custody", "providers", "adapters", "relations", "seeds", "targets", "group_by", "group_options"} {
		t.Run("ban_"+field, func(t *testing.T) {
			raw := strings.TrimSuffix(valid, "}") + `,"` + field + `":true}`
			if _, e := DecodeFutureCensusRequestV1([]byte(raw)); e == nil {
				t.Fatal("accepted forbidden field")
			}
		})
	}
	for name, raw := range map[string]string{"absolute": `{"session_id":"s","generation":1,"sources":["/x"]}`, "backslash": `{"session_id":"s","generation":1,"sources":["a\\b"]}`, "unclean": `{"session_id":"s","generation":1,"sources":["a/./b"]}`, "traversal": `{"session_id":"s","generation":1,"sources":["../a"]}`, "duplicate": `{"session_id":"s","generation":1,"sources":["a","a"]}`, "bad_pattern": `{"session_id":"s","generation":1,"sources":["."],"includes":["["]}`, "blank_session": `{"session_id":" \t ","generation":1,"sources":["."]}`, "timeout_order": `{"session_id":"s","generation":1,"sources":["."],"timeout_ms":1,"request_timeout_ms":2}`, "depth": `{"session_id":"s","generation":1,"sources":["."],"down_depth":65}`, "nodes": `{"session_id":"s","generation":1,"sources":["."],"max_nodes":0}`} {
		t.Run(name, func(t *testing.T) {
			if _, e := DecodeFutureCensusRequestV1([]byte(raw)); e == nil {
				t.Fatal("accepted")
			}
		})
	}
	for _, n := range []int{1024, 1025} {
		v := map[string]any{"session_id": strings.Repeat("s", n), "generation": uint64(1), "sources": []any{"."}}
		e := ValidateFutureCensusRequestV1(v)
		if (e == nil) != (n == 1024) {
			t.Fatalf("ASSERT_CENSUS_SESSION_ID_LIMIT_%d: %v", n, e)
		}
	}
	decoded, e := DecodeFutureCensusRequestV1([]byte(valid))
	if e != nil || decoded["timeout_ms"] != uint64(60000) || decoded["request_timeout_ms"] != uint64(30000) {
		t.Fatalf("ASSERT_CENSUS_MCP_TIMEOUT_DEFAULTS: decoded=%v err=%v", decoded, e)
	}
	for _, key := range []string{"sources", "includes", "excludes"} {
		t.Run("limits_"+key, func(t *testing.T) {
			items := make([]any, 10000)
			for i := range items {
				items[i] = "p" + strings.Repeat("a", i%1000) + string(rune(0x1000+i))
			}
			v := map[string]any{"session_id": "s", "generation": uint64(1), "sources": []any{"."}}
			v[key] = items
			if e := ValidateFutureCensusRequestV1(v); e != nil {
				t.Fatalf("ASSERT_CENSUS_ARRAY_LIMIT_10000: %v", e)
			}
			v[key] = append(items, "overflow")
			if e := ValidateFutureCensusRequestV1(v); e == nil {
				t.Fatal("ASSERT_CENSUS_ARRAY_LIMIT_10001: accepted")
			}
		})
		v := map[string]any{"session_id": "s", "generation": uint64(1), "sources": []any{"."}}
		v[key] = []any{strings.Repeat("a", 1024)}
		if e := ValidateFutureCensusRequestV1(v); e != nil {
			t.Fatalf("ASSERT_CENSUS_STRING_LIMIT_1024[%s]: %v", key, e)
		}
		v[key] = []any{strings.Repeat("a", 1025)}
		if e := ValidateFutureCensusRequestV1(v); e == nil {
			t.Fatalf("ASSERT_CENSUS_STRING_LIMIT_1025[%s]: accepted", key)
		}
	}
}

const validFutureCensusResultJSON = `{"schema_version":"lsp-trace.census-result.v1","status":"SUCCEEDED","census_id":"c","capture_set_id":"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","session_id":"s","generation":1,"target_count":1,"batch_count":1,"file_accounting":{"denominator":1,"processed":1,"excluded":0,"forbidden":0,"unreadable":0,"unsupported":0,"document_symbol_failed":0,"omitted":0,"incomplete":0},"symbol_accounting":{"denominator":1,"prepared":1,"unsupported":0,"preparation_failed":0,"prepare_missing":0,"non_callable":0,"omitted":0,"incomplete":0},"authority":0,"source_graph_complete":"UNKNOWN","native_aggregate_custody":false,"cross_capture_calls":[],"leiden_admissible":false,"publication":{"selector":"census.json","digest":"sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb","byte_length":1,"verification_status":"VERIFIED","directory_sync_status":"COMPLETE","close_status":"COMPLETE"}}`

func TestFutureCensusResultParityCeilingsAndEnvelopes(t *testing.T) {
	s := futureCensusSchemas(t)
	result := decodeMap(t, validFutureCensusResultJSON)
	if e := ValidateFutureCensusResultV1(result); e != nil {
		t.Fatal(e)
	}
	schemaAccepts(t, s[FutureCensusResultID], []byte(validFutureCensusResultJSON), true)
	want := []string{"schema_version", "status", "census_id", "capture_set_id", "session_id", "generation", "target_count", "batch_count", "file_accounting", "symbol_accounting", "authority", "source_graph_complete", "native_aggregate_custody", "cross_capture_calls", "leiden_admissible", "publication"}
	got := make([]string, 0, len(result))
	for k := range result {
		got = append(got, k)
	}
	for _, k := range want {
		if _, ok := result[k]; !ok {
			t.Fatalf("missing CLI semantic field %s", k)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("field parity got=%v", got)
	}
	for _, key := range []string{"census_id", "capture_set_id", "session_id"} {
		for _, n := range []int{1024, 1025} {
			v := decodeMap(t, validFutureCensusResultJSON)
			v[key] = strings.Repeat("x", n)
			e := ValidateFutureCensusResultV1(v)
			if (e == nil) != (n == 1024) {
				t.Fatalf("ASSERT_CENSUS_RESULT_ID_LIMIT_%s_%d: %v", key, n, e)
			}
		}
	}
	for name, mutate := range map[string]func(map[string]any){"authority": func(v map[string]any) { v["authority"] = json.Number("1") }, "calls": func(v map[string]any) { v["cross_capture_calls"] = []any{"x"} }, "leiden": func(v map[string]any) { v["leiden_admissible"] = true }, "accounting": func(v map[string]any) { v["file_accounting"].(map[string]any)["processed"] = json.Number("2") }, "ceiling": func(v map[string]any) { v["target_count"] = json.Number("10001") }, "blank_census_id": func(v map[string]any) { v["census_id"] = " \t " }, "blank_capture_set_id": func(v map[string]any) { v["capture_set_id"] = " \t " }, "blank_session_id": func(v map[string]any) { v["session_id"] = " \t " }, "private": func(v map[string]any) { v["publication"].(map[string]any)["path"] = "/private" }} {
		t.Run(name, func(t *testing.T) {
			v := decodeMap(t, validFutureCensusResultJSON)
			mutate(v)
			if e := ValidateFutureCensusResultV1(v); e == nil {
				t.Fatal("accepted")
			}
		})
	}
	success := `{"envelope_version":"1","envelope_schema_id":"` + FutureCensusSuccessID + `","tool":"lsp_trace_v1_census","request_id":"r","outcome":"COMPLETE","operation_status":"SUCCEEDED","isError":false,"result":` + validFutureCensusResultJSON + `}`
	schemaAccepts(t, s[FutureCensusSuccessID], []byte(success), true)
	degraded := `{"envelope_version":"1","envelope_schema_id":"` + FutureCensusSuccessID + `","tool":"lsp_trace_v1_census","request_id":"r","outcome":"COMMITTED_DEGRADED","operation_status":"SUCCEEDED","isError":false,"result":{"schema_version":"lsp-trace.census-diagnostic.v1","status":"SUCCEEDED_DEGRADED","stage":"committed-degradation","code":"COMMITTED_DEGRADED","retry":false}}`
	schemaAccepts(t, s[FutureCensusSuccessID], []byte(degraded), true)
	domain := `{"envelope_version":"1","envelope_schema_id":"` + FutureCensusDomainErrorID + `","tool":"lsp_trace_v1_census","request_id":"r","outcome":"DOMAIN_ERROR","operation_status":"FAILED","isError":true,"error":{"schema_version":"lsp-trace.census-diagnostic.v1","status":"FAILED","stage":"acquisition","code":"ACQUISITION_FAILED","batch_ordinal":1,"retry":true}}`
	schemaAccepts(t, s[FutureCensusDomainErrorID], []byte(domain), true)
}
func TestFutureCensusSchemasAreDirectAndHistoricalManifestUnchanged(t *testing.T) {
	futureCensusSchemas(t)
	base, e := LoadManifest()
	if e != nil {
		t.Fatal(e)
	}
	if e := ValidateManifest(base); e != nil {
		t.Fatalf("historical manifest no longer validates: %v", e)
	}
	for _, tool := range base.Tools {
		if tool.Name == FutureCensusTool || tool.Name == "lsp_trace_v1_structural_context" {
			t.Fatalf("operation 34/35 future contract registered: %s", tool.Name)
		}
	}
	if len(base.Tools) != 13 {
		t.Fatalf("historical stage-1 manifest changed: %d", len(base.Tools))
	}
	// The canonical MCP registry remains guarded at exactly 33 operations by
	// cmd/lsp-trace-mcp TestRegistrySurface; future schemas carry no authority.
}
