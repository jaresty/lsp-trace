package mcpcontract

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"

	jsonschema "github.com/santhosh-tekuri/jsonschema/v6"
)

const selectorSchemaID = "https://jaresty.github.io/lsp-trace/mcp/schemas/input-session-selector.v1.schema.json"

var selectorTestLimits = SelectorLimits{
	MaxSessionIDBytes:    8,
	MaxTrustDomainBytes:  8,
	MaxWorkspaceBytes:    8,
	MaxProfileBytes:      8,
	MaxOptionNameBytes:   8,
	MaxValueBytes:        4,
	MaxTotalDecodedBytes: 64,
	MaxDepth:             3,
	MaxCollectionSize:    2,
}

func validSelectorDocument() map[string]any {
	return map[string]any{
		"selector": map[string]any{
			"session_key": map[string]any{
				"version": "1", "trust_domain": "trust", "workspace": "/work", "profile": "prof",
				"server_affecting_options": []any{
					map[string]any{"name": "a", "value": map[string]any{"string": "ok"}},
					map[string]any{"name": "b", "value": map[string]any{"bytes_base64": "AQI="}},
				},
			},
		},
	}
}

func encodeSelector(t *testing.T, doc map[string]any) []byte {
	t.Helper()
	raw, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func selectorOptions(doc map[string]any) []any {
	return doc["selector"].(map[string]any)["session_key"].(map[string]any)["server_affecting_options"].([]any)
}

func assertSelectorResult(t *testing.T, assertion, wantCode string, doc map[string]any) {
	t.Helper()
	err := ValidateSessionSelectorSemantics(encodeSelector(t, doc), selectorTestLimits)
	if wantCode == "" {
		if err != nil {
			t.Fatalf("%s: got %v want pass", assertion, err)
		}
		t.Logf("ASSERTION %s PASS", assertion)
		return
	}
	if err == nil || !strings.Contains(err.Error(), wantCode) {
		t.Fatalf("%s: got %v want code %s", assertion, err, wantCode)
	}
	t.Logf("ASSERTION %s FAIL[%s]", assertion, wantCode)
}

func TestSelectorSemanticBoundaryVectors(t *testing.T) {
	t.Run("ASSERT_SELECTOR_NFC", func(t *testing.T) {
		good := validSelectorDocument()
		assertSelectorResult(t, "ASSERT_SELECTOR_NFC", "", good)
		bad := validSelectorDocument()
		bad["selector"].(map[string]any)["session_key"].(map[string]any)["profile"] = "e\u0301"
		assertSelectorResult(t, "ASSERT_SELECTOR_NFC", "SELECTOR_NON_NFC", bad)
	})

	t.Run("ASSERT_SELECTOR_UTF8_BYTES", func(t *testing.T) {
		good := validSelectorDocument()
		good["selector"].(map[string]any)["session_key"].(map[string]any)["profile"] = "éééé"
		assertSelectorResult(t, "ASSERT_SELECTOR_UTF8_BYTES", "", good)
		bad := validSelectorDocument()
		bad["selector"].(map[string]any)["session_key"].(map[string]any)["profile"] = "ééééé"
		assertSelectorResult(t, "ASSERT_SELECTOR_UTF8_BYTES", "SELECTOR_STRING_BYTES", bad)
	})

	t.Run("ASSERT_SELECTOR_DEPTH", func(t *testing.T) {
		good := validSelectorDocument()
		selectorOptions(good)[0].(map[string]any)["value"] = map[string]any{"list": []any{map[string]any{"list": []any{map[string]any{"string": "x"}}}}}
		assertSelectorResult(t, "ASSERT_SELECTOR_DEPTH", "", good)
		bad := validSelectorDocument()
		selectorOptions(bad)[0].(map[string]any)["value"] = map[string]any{"list": []any{map[string]any{"list": []any{map[string]any{"list": []any{map[string]any{"string": "x"}}}}}}}
		assertSelectorResult(t, "ASSERT_SELECTOR_DEPTH", "SELECTOR_DEPTH", bad)
	})

	t.Run("ASSERT_SELECTOR_DECODED_BYTES", func(t *testing.T) {
		good := validSelectorDocument()
		selectorOptions(good)[1].(map[string]any)["value"] = map[string]any{"bytes_base64": "AQIDBA=="}
		assertSelectorResult(t, "ASSERT_SELECTOR_DECODED_BYTES", "", good)
		bad := validSelectorDocument()
		selectorOptions(bad)[1].(map[string]any)["value"] = map[string]any{"bytes_base64": "AQIDBAU="}
		assertSelectorResult(t, "ASSERT_SELECTOR_DECODED_BYTES", "SELECTOR_DECODED_BYTES", bad)
	})

	t.Run("ASSERT_SELECTOR_COLLECTION_BOUND", func(t *testing.T) {
		good := validSelectorDocument()
		assertSelectorResult(t, "ASSERT_SELECTOR_COLLECTION_BOUND", "", good)
		bad := validSelectorDocument()
		selectorOptions(bad)[0].(map[string]any)["value"] = map[string]any{"list": []any{map[string]any{"null": true}, map[string]any{"null": true}, map[string]any{"null": true}}}
		assertSelectorResult(t, "ASSERT_SELECTOR_COLLECTION_BOUND", "SELECTOR_COLLECTION_SIZE", bad)
	})

	t.Run("ASSERT_SELECTOR_OPTIONS_CANONICAL", func(t *testing.T) {
		good := validSelectorDocument()
		assertSelectorResult(t, "ASSERT_SELECTOR_OPTIONS_CANONICAL", "", good)
		for _, mutate := range []func([]any){
			func(options []any) { options[0], options[1] = options[1], options[0] },
			func(options []any) { options[1].(map[string]any)["name"] = "a" },
		} {
			bad := validSelectorDocument()
			mutate(selectorOptions(bad))
			assertSelectorResult(t, "ASSERT_SELECTOR_OPTIONS_CANONICAL", "SELECTOR_OPTIONS_ORDER", bad)
		}
	})
}

func TestSelectorSemanticClosure(t *testing.T) {
	t.Run("ASSERT_SELECTOR_TRAILING_JSON", func(t *testing.T) {
		raw := append(encodeSelector(t, validSelectorDocument()), []byte(` {}`)...)
		if err := ValidateSessionSelectorSemantics(raw, selectorTestLimits); err == nil || !strings.Contains(err.Error(), "SELECTOR_TRAILING_JSON") {
			t.Fatalf("ASSERT_SELECTOR_TRAILING_JSON: got %v", err)
		}
	})
	t.Run("ASSERT_SELECTOR_BASE64_CANONICAL", func(t *testing.T) {
		doc := validSelectorDocument()
		selectorOptions(doc)[1].(map[string]any)["value"] = map[string]any{"bytes_base64": "AQI=\n"}
		if err := ValidateSessionSelectorSemantics(encodeSelector(t, doc), selectorTestLimits); err == nil || !strings.Contains(err.Error(), "SELECTOR_BASE64_CANONICAL") {
			t.Fatalf("ASSERT_SELECTOR_BASE64_CANONICAL: got %v", err)
		}
	})
	t.Run("ASSERT_SELECTOR_EXACT_UNION", func(t *testing.T) {
		doc := validSelectorDocument()
		doc["selector"].(map[string]any)["session_id"] = "session-1"
		if err := ValidateSessionSelectorSemantics(encodeSelector(t, doc), selectorTestLimits); err == nil || !strings.Contains(err.Error(), "SELECTOR_UNION") {
			t.Fatalf("ASSERT_SELECTOR_EXACT_UNION: got %v", err)
		}
	})
}

func TestSelectorNormativeLimitsAndPaths(t *testing.T) {
	limits := SessionSelectorV1Limits()
	if limits.MaxTrustDomainBytes != 256 || limits.MaxWorkspaceBytes != 4096 || limits.MaxProfileBytes != 256 || limits.MaxOptionNameBytes != 128 || limits.MaxValueBytes != 4096 || limits.MaxTotalDecodedBytes != 65536 || limits.MaxDepth != 8 || limits.MaxCollectionSize != 64 {
		t.Fatalf("ASSERT_SELECTOR_NORMATIVE_LIMITS: %+v", limits)
	}
	for _, workspace := range []string{"relative", "C:relative", `\\?\C:\x`, `\\.\pipe\x`, "C:/a/../../b", "//Srv/Share/../../x", "C:/file:stream"} {
		doc := validSelectorDocument()
		key := doc["selector"].(map[string]any)["session_key"].(map[string]any)
		key["workspace"] = workspace
		if err := ValidateSessionSelectorSemantics(encodeSelector(t, doc), limits); err == nil || !strings.Contains(err.Error(), "SELECTOR_WORKSPACE_PATH") {
			t.Fatalf("ASSERT_SELECTOR_VERSION_AND_PATH_CLASSES: %q: %v", workspace, err)
		}
	}
	doc := validSelectorDocument()
	doc["selector"].(map[string]any)["session_key"].(map[string]any)["version"] = "2"
	if err := ValidateSessionSelectorSemantics(encodeSelector(t, doc), limits); err == nil || !strings.Contains(err.Error(), "SELECTOR_VERSION") {
		t.Fatalf("ASSERT_SELECTOR_VERSION_AND_PATH_CLASSES: %v", err)
	}

	doc = validSelectorDocument()
	options := make([]any, 16)
	for i := range options {
		options[i] = map[string]any{"name": fmt.Sprintf("%02d", i), "value": map[string]any{"string": strings.Repeat("x", 4096)}}
	}
	doc["selector"].(map[string]any)["session_key"].(map[string]any)["server_affecting_options"] = options
	if err := ValidateSessionSelectorSemantics(encodeSelector(t, doc), limits); err == nil || !strings.Contains(err.Error(), "SELECTOR_TOTAL_DECODED_BYTES") {
		t.Fatalf("ASSERT_SELECTOR_TOTAL_DECODED_BYTES: %v", err)
	}
}

func TestSelectorSemanticsPrecedeAvailability(t *testing.T) {
	availabilityReached := false
	doc := validSelectorDocument()
	doc["selector"].(map[string]any)["session_key"].(map[string]any)["profile"] = "ééééé"
	raw := encodeSelector(t, doc)
	if err := ValidateSessionSelectorSemantics(raw, selectorTestLimits); err != nil {
		t.Log("ASSERTION ASSERT_SELECTOR_BEFORE_AVAILABILITY FAIL[SELECTOR_STRING_BYTES]")
	} else {
		availabilityReached = true
	}
	if availabilityReached {
		t.Fatal("ASSERT_SELECTOR_BEFORE_AVAILABILITY: availability reached for semantically excessive selector")
	}
}

func TestSelectorStructurePrecedesSemantics(t *testing.T) {
	availabilityReached := false
	doc := validSelectorDocument()
	doc["selector"].(map[string]any)["session_id"] = "session-1"
	raw := encodeSelector(t, doc)

	contract := loadStage2LifecycleContract(t)
	compiler := compileStage2ContractSchemas(t, contract)
	if err := validateCompiledJSON(t, compiler, selectorSchemaID, raw); err != nil {
		t.Log("ASSERTION ASSERT_SELECTOR_STRUCTURAL_BEFORE_SEMANTIC FAIL[STRUCTURE]")
	} else if err := ValidateSessionSelectorSemantics(raw, selectorTestLimits); err != nil {
		t.Logf("ASSERTION ASSERT_SELECTOR_STRUCTURAL_BEFORE_SEMANTIC FAIL[SEMANTICS]: %v", err)
	} else {
		availabilityReached = true
	}
	if availabilityReached {
		t.Fatal("ASSERT_SELECTOR_STRUCTURAL_BEFORE_SEMANTIC: structurally ambiguous selector reached availability")
	}
}

func TestSelectorVectorsAreStructurallyValid(t *testing.T) {
	raw, err := os.ReadFile("testdata/schemas/input-session-selector.v1.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	compiler := jsonschema.NewCompiler()
	compiler.DefaultDraft(jsonschema.Draft2020)
	if err := compiler.AddResource(selectorSchemaID, doc); err != nil {
		t.Fatal(err)
	}
	compiled, err := compiler.Compile(selectorSchemaID)
	if err != nil {
		t.Fatal(err)
	}
	value, err := jsonschema.UnmarshalJSON(bytes.NewReader(encodeSelector(t, validSelectorDocument())))
	if err != nil {
		t.Fatal(err)
	}
	if err := compiled.Validate(value); err != nil {
		t.Fatalf("ASSERT_SELECTOR_STRUCTURAL_CONTROL: %v", err)
	}
}
