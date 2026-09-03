package stage2scenario

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	jsonschema "github.com/santhosh-tekuri/jsonschema/v6"
)

func compileCandidateSchema(t *testing.T, name string) *jsonschema.Schema {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "schemas", name))
	if err != nil {
		t.Fatalf("ASSERT_CANDIDATE_SCHEMA_PRESENT[%s]: %v", name, err)
	}
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("ASSERT_CANDIDATE_SCHEMA_JSON[%s]: %v", name, err)
	}
	compiler := jsonschema.NewCompiler()
	compiler.DefaultDraft(jsonschema.Draft2020)
	if err := compiler.AddResource(name, doc); err != nil {
		t.Fatal(err)
	}
	schema, err := compiler.Compile(name)
	if err != nil {
		t.Fatalf("ASSERT_CANDIDATE_SCHEMA_COMPILES[%s]: %v", name, err)
	}
	return schema
}

func schemaValidate(t *testing.T, schema *jsonschema.Schema, raw []byte) error {
	t.Helper()
	value, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	if err != nil {
		return err
	}
	return schema.Validate(value)
}

func TestScenarioSchemaDraftIDAndClosedOperationBranches(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("testdata", "schemas", "stage2-scenario-v1.schema.json"))
	if err != nil {
		t.Fatalf("ASSERT_SCENARIO_SCHEMA_DRAFT_ID_AND_CLOSED_OPERATION_BRANCHES: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	if doc["$schema"] != "https://json-schema.org/draft/2020-12/schema" || doc["$id"] != ScenarioVersion {
		t.Fatalf("ASSERT_SCENARIO_SCHEMA_DRAFT_ID_AND_CLOSED_OPERATION_BRANCHES: schema=%v id=%v", doc["$schema"], doc["$id"])
	}
	schema := compileCandidateSchema(t, "stage2-scenario-v1.schema.json")
	bad := []byte(`{"version":"stage2-scenario-v1","steps":[{"op":"request","session":"s","generation":1,"request":"r","outcome":"forbidden"}]}`)
	if schemaValidate(t, schema, bad) == nil {
		t.Fatal("ASSERT_SCENARIO_SCHEMA_DRAFT_ID_AND_CLOSED_OPERATION_BRANCHES: forbidden field accepted")
	}
}

func TestStructuralValidationPrecedesSemanticPrerequisites(t *testing.T) {
	_, err := Parse([]byte(`{"version":"stage2-scenario-v1","steps":[{"op":"child","session":"s","generation":1,"request":"missing","lsp_request":"l","child":"c","ordinal":1,"outcome":"forbidden"}]}`))
	if err == nil || !strings.Contains(err.Error(), `structural stage2-scenario-v1`) {
		t.Fatalf("ASSERT_STRUCTURAL_VALIDATION_PRECEDES_SEMANTIC_PREREQUISITES: %v", err)
	}
}

func TestLedgerSchemaExactBranchesAndVocabulary(t *testing.T) {
	schema := compileCandidateSchema(t, "stage2-ledger-v1.schema.json")
	golden, err := os.ReadFile(filepath.Join("testdata", "goldens", "reference-lifecycle-ownership.ledger.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	branches := map[string]bool{}
	for _, line := range bytes.Split(bytes.TrimSuffix(golden, []byte("\n")), []byte("\n")) {
		if err := schemaValidate(t, schema, line); err != nil {
			t.Fatalf("ASSERT_LEDGER_SCHEMA_CANONICAL_VOCABULARY_REFERENCE_AND_CENSUS: %v\n%s", err, line)
		}
		var rec map[string]any
		_ = json.Unmarshal(line, &rec)
		branches[rec["record_type"].(string)] = true
		payloads := 0
		for _, name := range ledgerRecordTypesForTest() {
			if _, ok := rec[name]; ok {
				payloads++
			}
		}
		if payloads != 1 {
			t.Fatalf("ASSERT_LEDGER_SCHEMA_EXACTLY_ONE_CLOSED_PAYLOAD_BRANCH: %s payloads=%d", line, payloads)
		}
	}
	for _, name := range ledgerRecordTypesForTest() {
		if !branches[name] {
			t.Fatalf("ASSERT_LEDGER_ENCODER_SCHEMA_BIDIRECTIONAL_PARITY: branch %q has no golden vector", name)
		}
	}
	bad := []byte(`{"ledger_version":"stage2-ledger-v1","record_type":"event","event":{"event_seq":1,"kind":"request_admitted"},"final_resource_census":{"live_sessions":0}}`)
	if schemaValidate(t, schema, bad) == nil {
		t.Fatal("ASSERT_LEDGER_SCHEMA_EXACTLY_ONE_CLOSED_PAYLOAD_BRANCH: multiple payloads accepted")
	}
}

func ledgerRecordTypesForTest() []string {
	return []string{"event", "mcp_reference_result", "child_ownership", "lifecycle_intent_history", "session_generation_history", "final_resource_census"}
}

func TestCorpusStructuralThenSemanticAndNoSilentFiles(t *testing.T) {
	if err := ValidateCorpus("testdata"); err != nil {
		t.Fatalf("ASSERT_CORPUS_STRUCTURAL_THEN_SEMANTIC_NO_SILENT_FILES: %v", err)
	}
	entries, err := filepath.Glob(filepath.Join("testdata", "goldens", "*.ledger.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(entries)
	if len(entries) == 0 {
		t.Fatal("ASSERT_CORPUS_STRUCTURAL_THEN_SEMANTIC_NO_SILENT_FILES: no goldens")
	}
}

func TestReversibleSchemaAndEncoderPerturbations(t *testing.T) {
	schemaRaw, err := os.ReadFile(filepath.Join("testdata", "schemas", "stage2-ledger-v1.schema.json"))
	if err != nil {
		t.Fatal(err)
	}
	perturbedSchema := bytes.ReplaceAll(schemaRaw, []byte(`"const":"stage2-ledger-v1"`), []byte(`"const":"stage2-ledger-v1-perturbed"`))
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(perturbedSchema))
	if err != nil {
		t.Fatal(err)
	}
	compiler := jsonschema.NewCompiler()
	compiler.DefaultDraft(jsonschema.Draft2020)
	if err := compiler.AddResource("perturbed-ledger", doc); err != nil {
		t.Fatal(err)
	}
	schema, err := compiler.Compile("perturbed-ledger")
	if err != nil {
		t.Fatal(err)
	}
	golden, err := os.ReadFile(filepath.Join("testdata", "goldens", "reference-lifecycle-ownership.ledger.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	line := bytes.Split(golden, []byte("\n"))[0]
	if schemaValidate(t, schema, line) == nil {
		t.Fatal("ASSERT_REVERSIBLE_SCHEMA_PERTURBATION_DETECTED")
	}
	encoderPerturbation := bytes.Replace(line, []byte(`"record_type":"mcp_reference_result"`), []byte(`"record_type":"event"`), 1)
	if schemaValidate(t, compileCandidateSchema(t, "stage2-ledger-v1.schema.json"), encoderPerturbation) == nil {
		t.Fatal("ASSERT_REVERSIBLE_ENCODER_PERTURBATION_DETECTED")
	}
}

func TestCandidateIDsAreNotRuntimeArtifactIdentities(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "mcpcontract", "testdata", "stage2-lifecycle-contract.v1.json"))
	if err != nil {
		t.Fatal(err)
	}
	var contract struct {
		CandidateStatus string `json:"candidate_status"`
		Tools           []struct {
			ArtifactSchemaIDs []string `json:"artifact_schema_ids"`
		} `json:"tools"`
	}
	if err := json.Unmarshal(raw, &contract); err != nil {
		t.Fatal(err)
	}
	if contract.CandidateStatus != "UNACCEPTED_NOT_RUNTIME" {
		t.Fatalf("ASSERT_CANDIDATE_NON_AUTHORITATIVE: %q", contract.CandidateStatus)
	}
	for _, tool := range contract.Tools {
		for _, id := range tool.ArtifactSchemaIDs {
			if id == ScenarioSchemaID || id == LedgerSchemaID {
				t.Fatalf("ASSERT_CANDIDATE_SCHEMA_NOT_RUNTIME_ARTIFACT: %s", id)
			}
		}
	}
}

func TestIntegratedLedgersCompatibilityAliases(t *testing.T) {
	s, err := Parse([]byte(referenceScenario))
	if err != nil {
		t.Fatal(err)
	}
	got, err := ReplayIntegrated(s)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got.Ownership, got.Intent) || !bytes.Equal(got.Intent, got.TerminalHistory) || !bytes.Equal(got.TerminalHistory, got.ResourceCensus) {
		t.Fatal("ASSERT_INTEGRATED_LEDGER_FIELDS_ARE_DOCUMENTED_COMPATIBILITY_ALIASES: legacy fields diverged")
	}
	spec, err := os.ReadFile("SPEC.md")
	if err != nil || !strings.Contains(string(spec), "aliases of one combined ledger") {
		t.Fatalf("ASSERT_INTEGRATED_LEDGER_FIELDS_ARE_DOCUMENTED_COMPATIBILITY_ALIASES: compatibility contract missing: %v", err)
	}
}
