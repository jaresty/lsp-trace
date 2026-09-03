package mcpcontract

import (
	"encoding/json"
	"os"
	"sort"
	"strings"
	"testing"
)

const (
	assertAcceptedEmbedIsolation = "ASSERT_ACCEPTED_EMBED_EXCLUDES_CANDIDATE_SCHEMAS"
	assertClosedSupportRegistry  = "ASSERT_STAGE2_REFERENCED_SUPPORT_SCHEMAS_REGISTERED"
	assertCaller129Decision      = "ASSERT_CALLER_129_RESOURCE_EXHAUSTED_NO_MUTATION"
	assertEvidenceClassification = "ASSERT_ENABLED_VECTORS_CANDIDATE_REFERENCE_EVIDENCE"
	assertExactToolErrors        = "ASSERT_STAGE2_PER_TOOL_EXACT_ERRORS"
	assertNoLooseCandidateSchema = "ASSERT_STAGE2_NO_SILENT_LOOSE_CANDIDATE_SCHEMA"
	assertCandidateSupportLayer  = "ASSERT_STAGE2_CANDIDATE_INTERNAL_SUPPORT_LAYER"
	assertTerminalCombinations   = "ASSERT_STAGE2_TERMINAL_HISTORY_COMBINATIONS"
	assertRestartSuccessor       = "ASSERT_STAGE2_RESTART_SUCCESSOR_PROVENANCE_AND_HISTORY"
)

func TestStage2AcceptanceRepairs(t *testing.T) {
	for _, assertion := range []string{
		assertAcceptedEmbedIsolation,
		assertClosedSupportRegistry,
		assertCaller129Decision,
		assertEvidenceClassification,
		assertExactToolErrors,
		assertNoLooseCandidateSchema,
		assertCandidateSupportLayer,
		assertTerminalCombinations,
		assertRestartSuccessor,
	} {
		t.Log("ASSERTION: " + assertion)
	}

	contract := loadStage2LifecycleContract(t)

	t.Run(assertAcceptedEmbedIsolation, func(t *testing.T) {
		entries, err := contractFiles.ReadDir("testdata/schemas")
		if err != nil {
			t.Fatal(err)
		}
		for _, entry := range entries {
			if strings.Contains(entry.Name(), "session-") || strings.HasPrefix(entry.Name(), "provenance-session") || strings.HasPrefix(entry.Name(), "result-session") {
				t.Fatalf("candidate schema embedded in accepted runtime: %s", entry.Name())
			}
		}
	})

	t.Run(assertClosedSupportRegistry, func(t *testing.T) {
		registered := map[string]bool{}
		for _, schema := range contract.Schemas {
			registered[schema.ID] = true
		}
		for _, id := range []string{
			"https://jaresty.github.io/lsp-trace/mcp/schemas/provenance-session.v1.schema.json",
			"https://jaresty.github.io/lsp-trace/mcp/schemas/result-session-list.v1.schema.json",
			"https://jaresty.github.io/lsp-trace/mcp/schemas/result-session-status.v1.schema.json",
			"https://jaresty.github.io/lsp-trace/mcp/schemas/result-session-stop.v1.schema.json",
			"https://jaresty.github.io/lsp-trace/mcp/schemas/result-session-restart.v1.schema.json",
		} {
			if !registered[id] {
				t.Errorf("unregistered referenced support schema: %s", id)
			}
		}
	})

	t.Run(assertCaller129Decision, func(t *testing.T) {
		if len(contract.Unresolved) != 0 {
			t.Fatalf("stale unresolved assertions: %+v", contract.Unresolved)
		}
		if contract.CallerAttachmentOverflow != (candidateCallerOverflow{
			Caller: 129, Outcome: "DOMAIN_ERROR", OperationStatus: "FAILED", Code: "RESOURCE_EXHAUSTED",
			Provenance: "SELECTED", Attach: false, Mutate: false,
		}) {
			t.Fatalf("caller 129 decision mismatch: %+v", contract.CallerAttachmentOverflow)
		}
	})

	t.Run(assertEvidenceClassification, func(t *testing.T) {
		for _, name := range contract.Transcripts {
			if !strings.Contains(name, "-enabled.jsonl") {
				continue
			}
			if got := contract.TranscriptEvidence[name]; got != "CANDIDATE_REFERENCE_NOT_RUNTIME_EVIDENCE" {
				t.Errorf("%s: classification=%q", name, got)
			}
		}
	})

	t.Run(assertExactToolErrors, func(t *testing.T) {
		wantByTool := map[string][]string{
			"lsp_session_v1_list":    {"INPUT_FAMILY_MISMATCH", "INPUT_INVALID", "LIST_CURSOR_INVALID"},
			"lsp_session_v1_status":  {"STALE_GENERATION"},
			"lsp_session_v1_stop":    {"LIFECYCLE_CONFLICT", "RESOURCE_EXHAUSTED", "SESSION_POISONED", "SESSION_REAP_INCOMPLETE", "STALE_GENERATION"},
			"lsp_session_v1_restart": {"INITIALIZATION_FAILED", "INITIALIZATION_TIMEOUT", "LIFECYCLE_CONFLICT", "PIPE_SETUP_FAILED", "RESOURCE_EXHAUSTED", "SESSION_CRASHED", "SESSION_POISONED", "SESSION_REAP_INCOMPLETE", "SPAWN_FAILED", "STALE_GENERATION"},
		}
		fragmentByTool := map[string]string{
			"lsp_session_v1_list":    "envelope-session-list-error.v1.schema.json",
			"lsp_session_v1_status":  "envelope-session-status-exact-error.v1.schema.json",
			"lsp_session_v1_stop":    "envelope-session-stop-exact-error.v1.schema.json",
			"lsp_session_v1_restart": "envelope-session-restart-exact-error.v1.schema.json",
		}
		for tool, want := range wantByTool {
			var schemaPath string
			for _, id := range contract.Availability["ENABLED"].EnvelopeSchemaIDs[tool] {
				if strings.HasSuffix(id, fragmentByTool[tool]) {
					schemaPath = "testdata/schemas/" + fragmentByTool[tool]
				}
				if strings.HasSuffix(id, "envelope-session-exact-error.v1.schema.json") {
					t.Errorf("%s retains shared exact-error schema", tool)
				}
			}
			if schemaPath == "" {
				t.Fatalf("%s missing %s", tool, fragmentByTool[tool])
			}
			got := schemaCodeSet(t, schemaPath)
			if !equalStrings(got, want) {
				t.Errorf("%s: got exact codes %v want %v", tool, got, want)
			}
		}
	})

	t.Run(assertNoLooseCandidateSchema, func(t *testing.T) {
		registeredPaths := map[string]bool{}
		for _, schema := range contract.Schemas {
			registeredPaths[schema.Path] = true
		}
		entries, err := os.ReadDir("testdata/schemas")
		if err != nil {
			t.Fatal(err)
		}
		for _, entry := range entries {
			name := entry.Name()
			if isCandidateSchemaPath(name) && !registeredPaths["schemas/"+name] {
				t.Errorf("unregistered loose candidate schema: %s", name)
			}
		}
	})

	t.Run(assertCandidateSupportLayer, func(t *testing.T) {
		for _, schema := range contract.Schemas {
			isSupport := strings.HasPrefix(schema.Family, "provenance-session.") || strings.HasPrefix(schema.Family, "result-session-")
			if isSupport && schema.Layer != "candidate_support" {
				t.Errorf("candidate-internal support schema %s has layer %q", schema.ID, schema.Layer)
			}
			if schema.Layer == "candidate_support" && schemaIDReferencedByTools(schema.ID, contract.Tools) {
				t.Errorf("candidate-internal support schema entered tool envelope/artifact IDs: %s", schema.ID)
			}
		}
	})

	t.Run(assertTerminalCombinations, func(t *testing.T) {
		compiler := compileStage2ContractSchemas(t, contract)
		invalid := map[string]string{
			"https://jaresty.github.io/lsp-trace/mcp/schemas/result-session-status.v1.schema.json":  `{"session_id":"s","state":"STOPPED","generation":1,"reap_status":"REAPED","created_manager_event_seq":1,"last_use_manager_event_seq":1,"restart_count":0,"active_request_count":0,"current_intent":null,"terminal_history":[{"intent_id":"i","kind":"STOP","target_generation":1,"outcome":"COMPLETE","operation_status":"SUCCEEDED","code":"SESSION_POISONED","terminal_event_seq":2}],"health":{"diagnosis":null}}`,
			"https://jaresty.github.io/lsp-trace/mcp/schemas/result-session-stop.v1.schema.json":    `{"state":"STOPPED","generation":1,"reap_status":"REAPED","current_intent":null,"terminal_history":[{"intent_id":"i","kind":"STOP","target_generation":1,"outcome":"COMPLETE","operation_status":"SUCCEEDED","code":"ALREADY_STOPPED","terminal_event_seq":2}]}`,
			"https://jaresty.github.io/lsp-trace/mcp/schemas/result-session-restart.v1.schema.json": `{"state":"READY","generation":2,"predecessor_generation":1,"reap_status":"REAPED","current_intent":null,"terminal_history":[]}`,
		}
		for id, raw := range invalid {
			if err := validateCompiledJSON(t, compiler, id, []byte(raw)); err == nil {
				t.Errorf("%s accepted impossible terminal history", id)
			}
		}
	})

	t.Run(assertRestartSuccessor, func(t *testing.T) {
		for _, name := range []string{"transcripts/stage2-candidate/restart-canonical-enabled.jsonl", "transcripts/stage2-candidate/restart-alias-enabled.jsonl"} {
			raw, err := os.ReadFile("testdata/" + name)
			if err != nil {
				t.Fatal(err)
			}
			var response map[string]any
			lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
			if err := json.Unmarshal([]byte(lines[1]), &response); err != nil {
				t.Fatal(err)
			}
			envelope := structuredEnvelope(response)
			provenance := envelope["provenance"].(map[string]any)
			result := envelope["result"].(map[string]any)
			if provenance["mcp_session_generation"] != result["generation"] {
				t.Errorf("%s: provenance=%v successor=%v", name, provenance["mcp_session_generation"], result["generation"])
			}
			if history, _ := result["terminal_history"].([]any); len(history) == 0 {
				t.Errorf("%s: completed restart omitted retained terminal history", name)
			}
		}
	})
}

func TestStage2FinalFixPerturbations(t *testing.T) {
	t.Run("exact code set rejects extra and missing codes", func(t *testing.T) {
		want := []string{"LIFECYCLE_CONFLICT", "RESOURCE_EXHAUSTED", "SESSION_POISONED", "SESSION_REAP_INCOMPLETE", "STALE_GENERATION"}
		if equalStrings(append(append([]string{}, want...), "SESSION_STOPPING"), want) {
			t.Fatal("extra stop code perturbation was not rejected")
		}
		if equalStrings(want[:len(want)-1], want) {
			t.Fatal("missing stop code perturbation was not rejected")
		}
	})

	t.Run("unregistered candidate residue is detected", func(t *testing.T) {
		registered := map[string]bool{"schemas/envelope-session-stop-exact-error.v1.schema.json": true}
		perturbed := "envelope-session-unregistered.v1.schema.json"
		if !isCandidateSchemaPath(perturbed) || registered["schemas/"+perturbed] {
			t.Fatal("loose candidate schema perturbation was not rejected")
		}
	})

	t.Run("candidate support cannot enter envelope or artifact IDs", func(t *testing.T) {
		const supportID = "https://jaresty.github.io/lsp-trace/mcp/schemas/result-session-stop.v1.schema.json"
		perturbed := []ToolContract{{EnvelopeSchemaIDs: []string{supportID}}}
		if !schemaIDReferencedByTools(supportID, perturbed) {
			t.Fatal("candidate support ID leakage perturbation was not detected")
		}
	})
}

func isCandidateSchemaPath(name string) bool {
	return strings.Contains(name, "session-")
}

func schemaIDReferencedByTools(schemaID string, tools []ToolContract) bool {
	for _, tool := range tools {
		for _, id := range append(append([]string{}, tool.EnvelopeSchemaIDs...), tool.ArtifactSchemaIDs...) {
			if id == schemaID {
				return true
			}
		}
	}
	return false
}

func schemaCodeSet(t *testing.T, schemaPath string) []string {
	t.Helper()
	doc := readCandidateJSON(t, schemaPath)
	properties := doc["properties"].(map[string]any)
	code := properties["code"].(map[string]any)
	if value, ok := code["const"].(string); ok {
		return []string{value}
	}
	values := code["enum"].([]any)
	got := make([]string, 0, len(values))
	for _, value := range values {
		got = append(got, value.(string))
	}
	sort.Strings(got)
	return got
}

func equalStrings(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}
