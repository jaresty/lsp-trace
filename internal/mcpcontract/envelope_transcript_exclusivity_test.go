package mcpcontract

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	jsonschema "github.com/santhosh-tekuri/jsonschema/v6"
)

const (
	assertExclusiveEnvelope = "ASSERT_EXACTLY_ONE_PERMITTED_ENVELOPE"
	assertNamedEnvelope     = "ASSERT_NAMED_ENVELOPE_IDENTITY"
	assertCanonicalTool     = "ASSERT_CANONICAL_ENVELOPE_TOOL"
	assertValidationFirst   = "ASSERT_VALIDATION_BEFORE_AVAILABILITY"
	assertCanonicalJSONL    = "ASSERT_CANONICAL_JSONL_BYTES"
	assertTranscriptShape   = "ASSERT_TRANSCRIPT_SCHEMA_CONFORMANCE"
	assertStageBoundaries   = "ASSERT_STAGE_BOUNDARIES_PRESERVED"
)

type transcriptDocument map[string]any

func TestEnvelopeTranscriptExclusivityConformance(t *testing.T) {
	for _, assertion := range []string{
		assertExclusiveEnvelope,
		assertNamedEnvelope,
		assertCanonicalTool,
		assertValidationFirst,
		assertCanonicalJSONL,
		assertTranscriptShape,
		assertStageBoundaries,
	} {
		t.Log("ASSERTION: " + assertion)
	}

	stage1, err := LoadManifest()
	if err != nil {
		t.Fatal(err)
	}
	stage2 := loadStage2LifecycleContract(t)
	compiler := compileStage2ContractSchemas(t, stage2)

	assertRetainedTranscriptBytes(t, stage1.Transcripts)
	assertRetainedTranscriptBytes(t, stage2.Transcripts)
	if err := ValidateTranscripts(stage1); err != nil {
		t.Fatalf("%s: Stage 1: %v", assertTranscriptShape, err)
	}
	stage1Tools := make(map[string]ToolContract, len(stage1.Tools))
	for _, tool := range stage1.Tools {
		stage1Tools[tool.Name] = tool
	}
	for _, name := range stage1.Transcripts {
		for _, doc := range readCanonicalTranscript(t, name) {
			envelope := structuredEnvelope(doc)
			if envelope == nil {
				continue
			}
			canonical, _ := envelope["tool"].(string)
			tool, ok := stage1Tools[canonical]
			if !ok {
				t.Fatalf("%s[%s]: envelope tool %q is not canonical", assertCanonicalTool, name, canonical)
			}
			assertExactlyOneEnvelope(t, nil, name, tool, envelope)
		}
	}

	tools := make(map[string]ToolContract, len(stage2.Tools)*2)
	for _, tool := range stage2.Tools {
		tools[tool.Name] = tool
		for _, alias := range tool.Aliases {
			tools[alias] = tool
		}
		if tool.Advertised || tool.Availability != "NOT_IMPLEMENTED" {
			t.Fatalf("%s: lifecycle tool %q is advertised or available", assertStageBoundaries, tool.Name)
		}
	}

	for _, name := range stage2.Transcripts {
		docs := readCanonicalTranscript(t, name)
		if len(docs) != 2 {
			t.Fatalf("%s[%s]: got %d records, want request/response pair", assertTranscriptShape, name, len(docs))
		}
		request, response := docs[0], docs[1]
		if request["id"] != response["id"] {
			t.Fatalf("%s[%s]: request id %v != response id %v", assertTranscriptShape, name, request["id"], response["id"])
		}
		requestName, arguments := transcriptCall(t, name, request)
		envelope := structuredEnvelope(response)
		tool, ok := tools[requestName]
		if !ok {
			if envelope == nil && (strings.Contains(name, "unknown-tool") || strings.Contains(name, "unsupported-version")) {
				continue
			}
			t.Fatalf("%s[%s]: unrecognized request tool %q", assertCanonicalTool, name, requestName)
		}

		inputValid := validateCompiledJSON(t, compiler, tool.InputSchemaID, arguments) == nil
		if !inputValid {
			if envelope != nil {
				t.Fatalf("%s[%s]: structurally invalid input reached operation envelope", assertValidationFirst, name)
			}
			if _, ok := response["error"]; !ok {
				t.Fatalf("%s[%s]: structurally invalid input lacks pre-envelope error", assertValidationFirst, name)
			}
			continue
		}
		if envelope == nil {
			t.Fatalf("%s[%s]: valid input lacks operation envelope", assertTranscriptShape, name)
		}

		expectedCanonical := tool.Name
		if got, _ := envelope["tool"].(string); got != expectedCanonical {
			t.Fatalf("%s[%s]: got %q want canonical %q", assertCanonicalTool, name, got, expectedCanonical)
		}
		for marker, availability := range map[string]string{"containment_unavailable": "CONTAINMENT_UNAVAILABLE", "runtime_disabled": "RUNTIME_DISABLED", "enabled": "ENABLED"} {
			if strings.Contains(name, marker) {
				tool.EnvelopeSchemaIDs = stage2.Availability[availability].EnvelopeSchemaIDs[tool.Name]
			}
		}
		assertExactlyOneEnvelope(t, compiler, name, tool, envelope)
	}

	advertised := 0
	for _, tool := range stage1.Tools {
		if tool.Advertised {
			advertised++
		}
		if tool.Name == "lsp_trace_v1_incoming" || tool.Name == "lsp_trace_v1_slice" {
			if tool.Advertised || tool.Availability != "NOT_IMPLEMENTED" {
				t.Fatalf("%s: Stage 3 tool changed: %+v", assertStageBoundaries, tool)
			}
		}
	}
	if advertised != 6 {
		t.Fatalf("%s: Stage 1 advertised=%d want 6", assertStageBoundaries, advertised)
	}
}

func assertRetainedTranscriptBytes(t *testing.T, names []string) {
	t.Helper()
	for _, name := range names {
		raw, err := os.ReadFile(filepath.Join("testdata", name))
		if err != nil {
			t.Fatalf("%s[%s]: %v", assertCanonicalJSONL, name, err)
		}
		if len(raw) == 0 || bytes.HasPrefix(raw, []byte{0xef, 0xbb, 0xbf}) || raw[len(raw)-1] != '\n' || bytes.HasSuffix(raw, []byte("\n\n")) {
			t.Fatalf("%s[%s]: BOM, empty input, or non-single terminal newline", assertCanonicalJSONL, name)
		}
		scanner := bufio.NewScanner(bytes.NewReader(raw))
		for line := 1; scanner.Scan(); line++ {
			if len(scanner.Bytes()) == 0 {
				t.Fatalf("%s[%s:%d]: blank line", assertCanonicalJSONL, name, line)
			}
			var value any
			if err := json.Unmarshal(scanner.Bytes(), &value); err != nil {
				t.Fatalf("%s[%s:%d]: %v", assertCanonicalJSONL, name, line, err)
			}
			canonical, err := json.Marshal(value)
			if err != nil || !bytes.Equal(scanner.Bytes(), canonical) {
				t.Fatalf("%s[%s:%d]: line is not canonical JSON", assertCanonicalJSONL, name, line)
			}
		}
		if err := scanner.Err(); err != nil {
			t.Fatalf("%s[%s]: %v", assertCanonicalJSONL, name, err)
		}
	}
}

func readCanonicalTranscript(t *testing.T, name string) []transcriptDocument {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	var docs []transcriptDocument
	scanner := bufio.NewScanner(bytes.NewReader(raw))
	for scanner.Scan() {
		var doc transcriptDocument
		if err := json.Unmarshal(scanner.Bytes(), &doc); err != nil {
			t.Fatalf("%s[%s]: %v", assertTranscriptShape, name, err)
		}
		docs = append(docs, doc)
	}
	return docs
}

func transcriptCall(t *testing.T, name string, request transcriptDocument) (string, any) {
	t.Helper()
	if request["jsonrpc"] != "2.0" || request["method"] != "tools/call" {
		t.Fatalf("%s[%s]: malformed tools/call request", assertTranscriptShape, name)
	}
	params, ok := request["params"].(map[string]any)
	if !ok {
		t.Fatalf("%s[%s]: missing params", assertTranscriptShape, name)
	}
	requestName, ok := params["name"].(string)
	if !ok || requestName == "" {
		t.Fatalf("%s[%s]: missing tool name", assertTranscriptShape, name)
	}
	arguments, ok := params["arguments"]
	if !ok {
		t.Fatalf("%s[%s]: missing arguments", assertTranscriptShape, name)
	}
	return requestName, arguments
}

func structuredEnvelope(response transcriptDocument) map[string]any {
	result, _ := response["result"].(map[string]any)
	envelope, _ := result["structuredContent"].(map[string]any)
	return envelope
}

func assertExactlyOneEnvelope(t *testing.T, compiler *jsonschema.Compiler, name string, tool ToolContract, envelope map[string]any) {
	t.Helper()
	raw, err := json.Marshal(envelope)
	if err != nil {
		t.Fatal(err)
	}
	var matches []string
	for _, schemaID := range tool.EnvelopeSchemaIDs {
		var validationErr error
		if compiler == nil {
			validationErr = ValidateJSON(schemaID, raw)
		} else {
			validationErr = validateCompiledJSON(t, compiler, schemaID, raw)
		}
		if validationErr == nil {
			matches = append(matches, schemaID)
		}
	}
	if len(matches) != 1 {
		t.Fatalf("%s[%s]: matches=%v", assertExclusiveEnvelope, name, matches)
	}
	named, _ := envelope["envelope_schema_id"].(string)
	if named != matches[0] {
		t.Fatalf("%s[%s]: named=%q match=%q", assertNamedEnvelope, name, named, matches[0])
	}
}

func compileStage2ContractSchemas(t *testing.T, contract stage2LifecycleContract) *jsonschema.Compiler {
	t.Helper()
	compiler := jsonschema.NewCompiler()
	compiler.DefaultDraft(jsonschema.Draft2020)
	for _, registration := range contract.Schemas {
		raw, err := os.ReadFile(filepath.Join("testdata", registration.Path))
		if err != nil {
			t.Fatalf("%s: %v", assertTranscriptShape, err)
		}
		doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
		if err != nil {
			t.Fatalf("%s: schema %s: %v", assertTranscriptShape, registration.ID, err)
		}
		if err := compiler.AddResource(registration.ID, doc); err != nil {
			t.Fatalf("%s: schema %s: %v", assertTranscriptShape, registration.ID, err)
		}
	}
	return compiler
}

func validateCompiledJSON(t *testing.T, compiler *jsonschema.Compiler, schemaID string, value any) error {
	t.Helper()
	compiled, err := compiler.Compile(schemaID)
	if err != nil {
		t.Fatalf("%s: compile %s: %v", assertTranscriptShape, schemaID, err)
	}
	var doc any
	switch typed := value.(type) {
	case []byte:
		doc, err = jsonschema.UnmarshalJSON(bytes.NewReader(typed))
	default:
		raw, marshalErr := json.Marshal(typed)
		if marshalErr != nil {
			return marshalErr
		}
		doc, err = jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	}
	if err != nil {
		return fmt.Errorf("decode JSON: %w", err)
	}
	if err := compiled.Validate(doc); err != nil {
		return fmt.Errorf("%s: %w", strings.TrimSpace(schemaID), err)
	}
	return nil
}
