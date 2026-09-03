package mcpcontract

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

const (
	assertAvailabilitySets = "ASSERT_STAGE2_AVAILABILITY_EVERY_AND_ONLY"
	assertResultSchemas    = "ASSERT_STAGE2_OPERATION_RESULT_SCHEMAS_CLOSED"
	assertOutcomeMatrix    = "ASSERT_STAGE2_EXACT_OUTCOME_STATUS_CODE_MATRIX"
	assertProvenancePhases = "ASSERT_STAGE2_PROVENANCE_SELECTION_PHASES"
	assertDispatchOrder    = "ASSERT_STAGE2_STRUCTURAL_FIRST_PRECEDENCE"
	assertTranscriptMatrix = "ASSERT_STAGE2_CANONICAL_ALIAS_TRANSCRIPT_MATRIX"
	assertOuterCallResult  = "ASSERT_STAGE2_OUTER_CALL_TOOL_RESULT_INVARIANTS"
	assertCallerOverflow   = "ASSERT_STAGE2_CALLER_129_RESOURCE_EXHAUSTED_NO_MUTATION"
)

type candidateLifecycleSpec struct {
	CandidateStatus          string                         `json:"candidate_status"`
	Availability             map[string]candidateProjection `json:"availability_projections"`
	ValidationOrder          []string                       `json:"validation_order"`
	Unresolved               []candidateUnresolved          `json:"unresolved_contract_assertions"`
	CallerAttachmentOverflow candidateCallerOverflow        `json:"caller_attachment_overflow"`
}

type candidateProjection struct {
	Advertised        bool                `json:"advertised"`
	EnvelopeSchemaIDs map[string][]string `json:"envelope_schema_ids_by_tool"`
}

type candidateUnresolved struct {
	ID       string `json:"id"`
	Question string `json:"question"`
	Rule     string `json:"rule"`
}

func TestStage2LifecycleRemediationContract(t *testing.T) {
	raw, err := os.ReadFile(stage2ContractPath)
	if err != nil {
		t.Fatal(err)
	}
	var spec candidateLifecycleSpec
	if err := json.Unmarshal(raw, &spec); err != nil {
		t.Fatal(err)
	}

	t.Run(assertAvailabilitySets, func(t *testing.T) {
		want := []string{"CONTAINMENT_UNAVAILABLE", "ENABLED", "NOT_IMPLEMENTED", "RUNTIME_DISABLED"}
		got := make([]string, 0, len(spec.Availability))
		for state := range spec.Availability {
			got = append(got, state)
		}
		sort.Strings(got)
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("%s: got=%v want=%v", assertAvailabilitySets, got, want)
		}
		for state, projection := range spec.Availability {
			if projection.Advertised != (state == "ENABLED") {
				t.Errorf("%s: state=%s advertised=%v", assertAvailabilitySets, state, projection.Advertised)
			}
			if len(projection.EnvelopeSchemaIDs) != 4 {
				t.Errorf("%s: state=%s tools=%d", assertAvailabilitySets, state, len(projection.EnvelopeSchemaIDs))
			}
		}
	})

	t.Run(assertDispatchOrder, func(t *testing.T) {
		want := []string{"canonicalize_recognize", "structural_schema", "semantic_bounds", "implementation", "containment", "runtime", "handler"}
		if !reflect.DeepEqual(spec.ValidationOrder, want) {
			t.Fatalf("%s: got=%v want=%v", assertDispatchOrder, spec.ValidationOrder, want)
		}
	})

	t.Run(assertCallerOverflow, func(t *testing.T) {
		want := candidateCallerOverflow{Caller: 129, Outcome: "DOMAIN_ERROR", OperationStatus: "FAILED", Code: "RESOURCE_EXHAUSTED", Provenance: "SELECTED", Attach: false, Mutate: false}
		if len(spec.Unresolved) != 0 || spec.CallerAttachmentOverflow != want {
			t.Fatalf("%s: unresolved=%v got=%+v want=%+v", assertCallerOverflow, spec.Unresolved, spec.CallerAttachmentOverflow, want)
		}
	})

	for _, schema := range []string{"result-session-list.v1.schema.json", "result-session-status.v1.schema.json", "result-session-stop.v1.schema.json", "result-session-restart.v1.schema.json"} {
		t.Run(assertResultSchemas+"/"+schema, func(t *testing.T) {
			doc := readCandidateJSON(t, filepath.Join("testdata", "schemas", schema))
			if doc["additionalProperties"] != false {
				t.Fatalf("%s[%s]: result must be closed", assertResultSchemas, schema)
			}
		})
	}

	for schema, want := range map[string][3]string{
		"envelope-session-cancelled.v1.schema.json":       {"CANCELLED", "CANCELLED", "REQUEST_CANCELLED"},
		"envelope-session-timed-out.v1.schema.json":       {"TIMED_OUT", "TIMED_OUT", "REQUEST_TIMEOUT"},
		"envelope-session-not-implemented.v1.schema.json": {"DOMAIN_ERROR", "FAILED", "TOOL_NOT_IMPLEMENTED"},
	} {
		t.Run(assertOutcomeMatrix+"/"+schema, func(t *testing.T) {
			doc := readCandidateJSON(t, filepath.Join("testdata", "schemas", schema))
			properties, _ := doc["properties"].(map[string]any)
			got := [3]string{schemaConst(properties, "outcome"), schemaConst(properties, "operation_status"), schemaConst(properties, "code")}
			if doc["additionalProperties"] != false || got != want {
				t.Fatalf("%s[%s]: got=%v want=%v closed=%v", assertOutcomeMatrix, schema, got, want, doc["additionalProperties"])
			}
		})
	}

	t.Run(assertProvenancePhases, func(t *testing.T) {
		for _, schema := range []string{"provenance-session.v1.schema.json", "envelope-session-status-result.v1.schema.json", "envelope-session-not-found.v1.schema.json"} {
			if _, err := os.Stat(filepath.Join("testdata", "schemas", schema)); err != nil {
				t.Errorf("%s: missing %s", assertProvenancePhases, schema)
			}
		}
	})

	t.Run(assertTranscriptMatrix, func(t *testing.T) {
		entries, err := os.ReadDir(filepath.Join("testdata", "transcripts", "stage2-candidate"))
		if err != nil {
			t.Fatalf("%s: %v", assertTranscriptMatrix, err)
		}
		if len(entries) != 42 {
			t.Fatalf("%s: got=%d want=42", assertTranscriptMatrix, len(entries))
		}
		for _, name := range []string{"unknown-tool.jsonl", "unsupported-version.jsonl"} {
			raw, err := os.ReadFile(filepath.Join("testdata", "transcripts", "stage2-candidate", name))
			if err != nil || strings.Contains(string(raw), `"structuredContent"`) {
				t.Errorf("%s[%s]: unknown/versioned behavior must be pre-envelope MCP error", assertTranscriptMatrix, name)
			}
		}
	})

	t.Run(assertOuterCallResult, func(t *testing.T) {
		entries, err := os.ReadDir(filepath.Join("testdata", "transcripts", "stage2-candidate"))
		if err != nil {
			t.Fatalf("%s: %v", assertOuterCallResult, err)
		}
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			raw, err := os.ReadFile(filepath.Join("testdata", "transcripts", "stage2-candidate", entry.Name()))
			if err != nil {
				t.Fatal(err)
			}
			text := string(raw)
			if strings.Contains(text, `"structuredContent"`) && !strings.Contains(text, `"content":[]`) {
				t.Errorf("%s[%s]: handled response outer content is not exactly empty", assertOuterCallResult, entry.Name())
			}
		}
	})
}

func schemaConst(properties map[string]any, name string) string {
	property, _ := properties[name].(map[string]any)
	value, _ := property["const"].(string)
	return value
}

func readCandidateJSON(t *testing.T, path string) map[string]any {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	return doc
}
