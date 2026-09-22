package mcpcontract

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	jsonschema "github.com/santhosh-tekuri/jsonschema/v6"
)

const (
	goodContinuationSelector = "g-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa.selector.json"
	goodDigest               = "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
)

func continuationSchemas(t *testing.T) map[string]*jsonschema.Schema {
	t.Helper()
	compiler := jsonschema.NewCompiler()
	compiler.DefaultDraft(jsonschema.Draft2020)
	ids := []string{CensusResultID, CensusSuccessID, FutureCensusV2InputID, FutureCensusCompositeResultID, FutureCensusCompositeSuccessID, CensusDomainErrorID}
	for _, id := range ids {
		raw, err := FutureCensusSchemaJSON(id)
		if err != nil {
			t.Fatalf("ASSERT_CENSUS_V2_SCHEMA_REGISTERED[%s]: %v", id, err)
		}
		doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
		if err != nil {
			t.Fatal(err)
		}
		if err := compiler.AddResource(id, doc); err != nil {
			t.Fatal(err)
		}
	}
	out := map[string]*jsonschema.Schema{}
	for _, id := range ids {
		compiled, err := compiler.Compile(id)
		if err != nil {
			t.Fatalf("ASSERT_CENSUS_V2_SCHEMA_COMPILES[%s]: %v", id, err)
		}
		out[id] = compiled
	}
	return out
}

func TestFutureCensusV1RejectsContinuationAndBytesRemainMirrored(t *testing.T) {
	if _, err := DecodeFutureCensusRequestV1([]byte(`{"session_id":"s","generation":1,"sources":["."],"continuation":{"kind":"ADR_0007_FEATURE_CATALOG"}}`)); err == nil {
		t.Fatal("ASSERT_CENSUS_V1_REJECTS_CONTINUATION: accepted")
	}
	for id, name := range map[string]string{
		CensusInputID:       "input-census.v1.schema.json",
		CensusResultID:      "lsp-trace.census-result.v1.schema.json",
		CensusSuccessID:     "envelope-census-result.v1.schema.json",
		CensusDomainErrorID: "envelope-census-domain-error.v1.schema.json",
	} {
		got, err := FutureCensusSchemaJSON(id)
		if err != nil {
			t.Fatal(err)
		}
		want, err := censusSchemaFixture(name)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, want) {
			t.Fatalf("ASSERT_CENSUS_V1_BYTES_UNCHANGED[%s]", id)
		}
	}
}

func censusSchemaFixture(name string) ([]byte, error) {
	return futureCensusFiles.ReadFile("testdata/schemas/" + name)
}

func TestFutureCensusRequestV2BatchTargetsFreshOnlyWithServerDefault(t *testing.T) {
	for _, tc := range []struct {
		name string
		raw  string
		want json.Number
		ok   bool
	}{
		{name: "fresh_omitted_defaults_16", raw: `{"session_id":"s","generation":1,"sources":["."],"continuation":{"kind":"ADR_0007_FEATURE_CATALOG"}}`, want: json.Number("16"), ok: true},
		{name: "fresh_explicit_1", raw: `{"session_id":"s","generation":1,"sources":["."],"batch_targets":1,"continuation":{"kind":"ADR_0007_FEATURE_CATALOG"}}`, want: json.Number("1"), ok: true},
		{name: "fresh_explicit_63", raw: `{"session_id":"s","generation":1,"sources":["."],"batch_targets":63,"continuation":{"kind":"ADR_0007_FEATURE_CATALOG"}}`, want: json.Number("63"), ok: true},
		{name: "fresh_zero", raw: `{"session_id":"s","generation":1,"sources":["."],"batch_targets":0,"continuation":{"kind":"ADR_0007_FEATURE_CATALOG"}}`},
		{name: "fresh_64", raw: `{"session_id":"s","generation":1,"sources":["."],"batch_targets":64,"continuation":{"kind":"ADR_0007_FEATURE_CATALOG"}}`},
		{name: "historical_forbidden", raw: `{"session_id":"s","generation":1,"sources":["."],"batch_targets":16}`},
		{name: "resume_forbidden", raw: `{"batch_targets":16,"continuation":{"kind":"ADR_0007_FEATURE_CATALOG","resume_selector":"` + goodContinuationSelector + `"}}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			decoded, err := DecodeFutureCensusRequestV2([]byte(tc.raw))
			if (err == nil) != tc.ok {
				t.Fatalf("ASSERT_BATCH_TARGETS_MCP_V2_BRANCH_AND_BOUNDS: err=%v decoded=%v", err, decoded)
			}
			if tc.ok && decoded["batch_targets"] != tc.want {
				t.Fatalf("ASSERT_BATCH_TARGETS_MCP_V2_SERVER_DEFAULT_TRANSFER: got=%v want=%v", decoded["batch_targets"], tc.want)
			}
		})
	}
}

func TestFutureCensusRequestV2StopAfterDescribeRequests(t *testing.T) {
	schema := continuationSchemas(t)[FutureCensusV2InputID]
	for _, tc := range []struct {
		name string
		raw  string
		ok   bool
	}{
		{name: "fresh", raw: `{"session_id":"s","generation":1,"sources":["."],"continuation":{"kind":"ADR_0007_FEATURE_CATALOG","stop_after":"DESCRIBE_REQUESTS"}}`, ok: true},
		{name: "same_stop_replay", raw: `{"continuation":{"kind":"ADR_0007_FEATURE_CATALOG","resume_selector":"` + goodContinuationSelector + `","stop_after":"DESCRIBE_REQUESTS"}}`, ok: true},
		{name: "unknown_enum", raw: `{"session_id":"s","generation":1,"sources":["."],"continuation":{"kind":"ADR_0007_FEATURE_CATALOG","stop_after":"OTHER"}}`},
		{name: "wrong_type", raw: `{"session_id":"s","generation":1,"sources":["."],"continuation":{"kind":"ADR_0007_FEATURE_CATALOG","stop_after":1}}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := DecodeFutureCensusRequestV2([]byte(tc.raw))
			if (err == nil) != tc.ok {
				t.Fatalf("ASSERT_STOP_AFTER_DESCRIBE_REQUESTS_MCP_CONTRACT: ok=%t err=%v got=%v", tc.ok, err, got)
			}
			schemaAccepts(t, schema, []byte(tc.raw), tc.ok)
		})
	}
}

func TestFutureCensusRequestV2ExclusiveBranches(t *testing.T) {
	schemas := continuationSchemas(t)
	valid := map[string]string{
		"historical": `{"session_id":"s","generation":1,"sources":["."]}`,
		"fresh":      `{"session_id":"s","generation":1,"sources":["."],"continuation":{"kind":"ADR_0007_FEATURE_CATALOG"}}`,
		"resume":     `{"continuation":{"kind":"ADR_0007_FEATURE_CATALOG","resume_selector":"` + goodContinuationSelector + `"}}`,
	}
	for name, raw := range valid {
		t.Run(name, func(t *testing.T) {
			var err error
			if name == "historical" {
				_, err = DecodeFutureCensusRequestV1([]byte(raw))
			} else {
				_, err = DecodeFutureCensusRequestV2([]byte(raw))
			}
			if err != nil {
				t.Fatalf("ASSERT_CENSUS_V2_%s_ACCEPTED: %v", strings.ToUpper(name), err)
			}
			schemaAccepts(t, schemas[FutureCensusV2InputID], []byte(raw), true)
		})
	}

	invalid := map[string]string{
		"fresh_resume_selector": `{"session_id":"s","generation":1,"sources":["."],"continuation":{"kind":"ADR_0007_FEATURE_CATALOG","resume_selector":"` + goodContinuationSelector + `"}}`,
		"resume_session":        `{"session_id":"s","continuation":{"kind":"ADR_0007_FEATURE_CATALOG","resume_selector":"` + goodContinuationSelector + `"}}`,
		"resume_generation":     `{"generation":1,"continuation":{"kind":"ADR_0007_FEATURE_CATALOG","resume_selector":"` + goodContinuationSelector + `"}}`,
		"resume_sources":        `{"sources":["."],"continuation":{"kind":"ADR_0007_FEATURE_CATALOG","resume_selector":"` + goodContinuationSelector + `"}}`,
		"resume_option":         `{"max_nodes":1,"continuation":{"kind":"ADR_0007_FEATURE_CATALOG","resume_selector":"` + goodContinuationSelector + `"}}`,
		"wrong_kind":            `{"session_id":"s","generation":1,"sources":["."],"continuation":{"kind":"OTHER"}}`,
		"historical_unknown":    `{"session_id":"s","generation":1,"sources":["."],"unknown":true}`,
	}
	for name, raw := range invalid {
		t.Run(name, func(t *testing.T) {
			if _, err := DecodeFutureCensusRequestV2([]byte(raw)); err == nil {
				t.Fatal("ASSERT_CENSUS_V2_CROSS_BRANCH_REJECTED: accepted")
			}
			schemaAccepts(t, schemas[FutureCensusV2InputID], []byte(raw), false)
		})
	}
}

func TestFutureCensusV2HistoricalAcceptedCorpusParity(t *testing.T) {
	schema := continuationSchemas(t)[FutureCensusV2InputID]
	fixtures := map[string]string{
		"minimal":           `{"session_id":"s","generation":1,"sources":["."]}`,
		"explicit_defaults": `{"session_id":"s","generation":1,"sources":["."],"down_depth":1,"up_depth":0,"timeout_ms":60000,"request_timeout_ms":30000}`,
		"all_fields":        `{"session_id":"project","generation":9223372036854775807,"sources":[".","a.go"],"includes":["**/*.go"],"excludes":["generated/**"],"down_depth":64,"up_depth":64,"max_nodes":10000,"timeout_ms":60000,"request_timeout_ms":30000}`,
		"zero_depths":       `{"session_id":"s","generation":1,"sources":["a.go"],"down_depth":0,"up_depth":0,"max_nodes":1,"timeout_ms":1,"request_timeout_ms":1}`,
	}
	for name, raw := range fixtures {
		t.Run(name, func(t *testing.T) {
			decoded, err := DecodeFutureCensusRequestV1([]byte(raw))
			if err != nil {
				t.Fatalf("ASSERT_CENSUS_HISTORICAL_CORPUS_V1_ACCEPTED: %v", err)
			}
			if decoded["timeout_ms"] == nil || decoded["request_timeout_ms"] == nil {
				t.Fatalf("ASSERT_CENSUS_HISTORICAL_CORPUS_SERVER_DEFAULTS: %v", decoded)
			}
			schemaAccepts(t, schema, []byte(raw), true)
		})
	}
}

func TestFutureCensusRequestV2StrictAndPathFree(t *testing.T) {
	base := `{"session_id":"s","generation":1,"sources":["."],"continuation":{"kind":"ADR_0007_FEATURE_CATALOG"}}`
	for _, field := range []string{"workspace", "workspace_uri", "model", "model_id", "executable", "library", "sandbox", "publication", "publication_root", "store", "store_path", "path"} {
		t.Run("forbidden_"+field, func(t *testing.T) {
			raw := strings.TrimSuffix(base, "}") + `,"` + field + `":"x"}`
			if _, err := DecodeFutureCensusRequestV2([]byte(raw)); err == nil {
				t.Fatal("ASSERT_CENSUS_V2_FORBIDDEN_FIELD_REJECTED: accepted")
			}
		})
	}
	for name, raw := range map[string]string{
		"unknown":               strings.TrimSuffix(base, "}") + `,"unknown":true}`,
		"duplicate":             `{"session_id":"s","session_id":"t","generation":1,"sources":["."],"continuation":{"kind":"ADR_0007_FEATURE_CATALOG"}}`,
		"nested_duplicate":      `{"continuation":{"kind":"ADR_0007_FEATURE_CATALOG","kind":"ADR_0007_FEATURE_CATALOG","resume_selector":"` + goodContinuationSelector + `"}}`,
		"trailing":              base + ` {}`,
		"noncanonical_kind":     `{"session_id":"s","generation":1,"sources":["."],"continuation":{"kind":"adr_0007_feature_catalog"}}`,
		"selector_short":        `{"continuation":{"kind":"ADR_0007_FEATURE_CATALOG","resume_selector":"g-a.selector.json"}}`,
		"selector_upper":        `{"continuation":{"kind":"ADR_0007_FEATURE_CATALOG","resume_selector":"g-AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA.selector.json"}}`,
		"selector_traversal":    `{"continuation":{"kind":"ADR_0007_FEATURE_CATALOG","resume_selector":"../` + goodContinuationSelector + `"}}`,
		"selector_absolute":     `{"continuation":{"kind":"ADR_0007_FEATURE_CATALOG","resume_selector":"/` + goodContinuationSelector + `"}}`,
		"selector_noncanonical": `{"continuation":{"kind":"ADR_0007_FEATURE_CATALOG","resume_selector":"x/../` + goodContinuationSelector + `"}}`,
		"continuation_extra":    `{"session_id":"s","generation":1,"sources":["."],"continuation":{"kind":"ADR_0007_FEATURE_CATALOG","path":"x"}}`,
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := DecodeFutureCensusRequestV2([]byte(raw)); err == nil {
				t.Fatal("ASSERT_CENSUS_V2_STRICT_INPUT_REJECTED: accepted")
			}
		})
	}
}

func TestFutureCensusPausedCompositeAccepted(t *testing.T) {
	catalog := `{"kind":"ADR_0007_FEATURE_CATALOG","checkpoint_selector":"` + goodContinuationSelector + `","composite_selector":"` + goodContinuationSelector + `","catalog_selector":"` + goodContinuationSelector + `","status":"PAUSED","request_count":3,"preparation_count":2,"resume_guidance":"Resume with this selector and omit stop_after to continue exactly the remaining work once.","authority":0,"accepted":false,"completeness":"UNKNOWN"}`
	raw := []byte(`{"schema_version":"lsp-trace.census-feature-catalog-result.v2","census":` + validFutureCensusResultJSON + `,"catalog":` + catalog + `}`)
	schemaAccepts(t, continuationSchemas(t)[FutureCensusCompositeResultID], raw, true)
	if err := ValidateFutureCensusCompositeResultV2(raw); err != nil {
		t.Fatalf("ASSERT_STOP_AFTER_PAUSED_COMPOSITE_ACCEPTED: %v raw=%s", err, raw)
	}
}

func TestFutureCensusCompositeResultAndEnvelopeExclusivity(t *testing.T) {
	schemas := continuationSchemas(t)
	census := validFutureCensusResultJSON
	catalog := `{"kind":"ADR_0007_FEATURE_CATALOG","checkpoint_selector":"` + goodContinuationSelector + `","composite_selector":"` + goodContinuationSelector + `","catalog_selector":"` + goodContinuationSelector + `","status":"COMPLETE","authority":0,"accepted":false,"completeness":"UNKNOWN"}`
	fresh := `{"schema_version":"lsp-trace.census-feature-catalog-result.v2","census":` + census + `,"catalog":` + catalog + `}`
	identity := `{"selector":"` + goodContinuationSelector + `","digest":"` + goodDigest + `","byte_length":1}`
	resume := `{"schema_version":"lsp-trace.census-feature-catalog-result.v2","census_identity":` + identity + `,"catalog":` + catalog + `}`
	for name, raw := range map[string]string{"fresh": fresh, "resume": resume} {
		t.Run(name, func(t *testing.T) {
			schemaAccepts(t, schemas[FutureCensusCompositeResultID], []byte(raw), true)
			if err := ValidateFutureCensusCompositeResultV2([]byte(raw)); err != nil {
				t.Fatalf("ASSERT_CENSUS_COMPOSITE_%s_ACCEPTED: %v", strings.ToUpper(name), err)
			}
		})
	}

	mutations := map[string]func(map[string]any){
		"authority":    func(v map[string]any) { v["catalog"].(map[string]any)["authority"] = json.Number("1") },
		"accepted":     func(v map[string]any) { v["catalog"].(map[string]any)["accepted"] = true },
		"completeness": func(v map[string]any) { v["catalog"].(map[string]any)["completeness"] = "COMPLETE" },
		"status":       func(v map[string]any) { v["catalog"].(map[string]any)["status"] = "PARTIAL" },
		"substitution": func(v map[string]any) {
			v["catalog"].(map[string]any)["checkpoint_selector"] = "g-cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc.selector.json"
		},
		"both_branches": func(v map[string]any) {
			v["census"] = decodeMap(t, validFutureCensusResultJSON)
		},
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			v := decodeMap(t, resume)
			mutate(v)
			raw, _ := json.Marshal(v)
			if name != "substitution" {
				schemaAccepts(t, schemas[FutureCensusCompositeResultID], raw, false)
			}
			if err := ValidateFutureCensusCompositeResultV2(raw); err == nil {
				t.Fatal("ASSERT_CENSUS_COMPOSITE_MUTATION_REJECTED: accepted")
			}
		})
	}

	success := `{"envelope_version":"1","envelope_schema_id":"` + FutureCensusCompositeSuccessID + `","tool":"` + CensusTool + `","request_id":"r","outcome":"COMPLETE","operation_status":"SUCCEEDED","isError":false,"result":` + fresh + `}`
	schemaAccepts(t, schemas[FutureCensusCompositeSuccessID], []byte(success), true)
	schemaAccepts(t, schemas[CensusSuccessID], []byte(success), false)
	if err := ValidateFutureCensusEnvelopeExclusive([]byte(success)); err != nil {
		t.Fatalf("ASSERT_CENSUS_SUCCESS_EXCLUSIVE: %v", err)
	}
	domain := `{"envelope_version":"1","envelope_schema_id":"` + CensusDomainErrorID + `","tool":"` + CensusTool + `","request_id":"r","outcome":"DOMAIN_ERROR","operation_status":"FAILED","isError":true,"error":{"schema_version":"lsp-trace.census-diagnostic.v1","status":"FAILED","stage":"acquisition","code":"ACQUISITION_FAILED","batch_ordinal":1,"retry":true}}`
	if err := ValidateFutureCensusEnvelopeExclusive([]byte(domain)); err != nil {
		t.Fatalf("ASSERT_CENSUS_DOMAIN_ERROR_RETAINED: %v", err)
	}
}

func TestFutureCensusV2SchemasRegisteredAdditively(t *testing.T) {
	manifest, err := LoadManifest()
	if err != nil {
		t.Fatal(err)
	}
	extended := WithCensus(manifest)
	found := map[string]bool{}
	for _, registration := range extended.Schemas {
		found[registration.ID] = true
	}
	for _, id := range []string{FutureCensusV2InputID, FutureCensusCompositeResultID, FutureCensusCompositeSuccessID} {
		if !found[id] {
			t.Fatalf("ASSERT_CENSUS_V2_SCHEMA_REGISTERED_ADDITIVELY[%s]", id)
		}
	}
	var censusTools int
	for _, tool := range extended.Tools {
		if tool.Name == CensusTool {
			censusTools++
			if len(tool.Aliases) != 0 {
				t.Fatalf("ASSERT_CENSUS_OPERATION_HAS_NO_ALIAS: %v", tool.Aliases)
			}
		}
	}
	if censusTools != 1 {
		t.Fatalf("ASSERT_CENSUS_CANONICAL_OPERATION_SINGLETON: %d", censusTools)
	}
}
