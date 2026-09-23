package schemas

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

// Draft-only shape checks. They do not attest canonical bytes, predecessor custody or issuance.
func TestReferencesIssuanceProposedShapes(t *testing.T) {
	b, err := os.ReadFile("adr0011-references-issuance-records.proposed.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	var doc any
	if err := json.Unmarshal(b, &doc); err != nil {
		t.Fatal(err)
	}
	id := doc.(map[string]any)["$id"].(string)
	c := jsonschema.NewCompiler()
	c.AssertFormat()
	if err := c.AddResource(id, doc); err != nil {
		t.Fatal(err)
	}
	s, err := c.Compile(id)
	if err != nil {
		t.Fatal(err)
	}
	d := "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	ref := func(role string) map[string]any {
		return map[string]any{"selector": "adr0011-" + role + ".json", "digest": d, "schema_version": role}
	}
	ctx := map[string]any{"transaction_id": d, "query_occurrence_id": d, "method_ref": ref("REFERENCES_METHOD_V1"), "target_ref": ref("REFERENCES_TARGET_V1"), "source_identity_ref": ref("REFERENCES_SOURCE_IDENTITY_V1"), "revision_identity_ref": ref("REFERENCES_REVISION_IDENTITY_V1"), "method_policy_ref": ref("REFERENCES_POLICY_V1"), "admission_policy_ref": ref("REFERENCES_POLICY_V1"), "privacy_policy_ref": ref("REFERENCES_POLICY_V1"), "retention_policy_ref": ref("REFERENCES_POLICY_V1"), "implementation_digest": d, "schema_digests": []any{map[string]any{"role": "proposal", "digest": d}, map[string]any{"role": "candidate", "digest": d}, map[string]any{"role": "final", "digest": d}, map[string]any{"role": "event", "digest": d}}, "session_id": "s", "generation": 1, "request_key": "k", "invocation_id": "i"}
	proposal := map[string]any{"schema_version": "REFERENCES_EVALUATION_PROPOSAL_V1", "context": ctx, "raw_result_ref": ref("REFERENCES_RAW_RESULT_V1"), "raw_result_presence": "PRESENT", "raw_result_digest": d, "raw_result_byte_length": 2, "scanner_observation_ref": ref("REFERENCES_SCANNER_OBSERVATION_V1"), "top_level_form": "ARRAY", "declared_n": 1, "evaluator_event_ref": ref("REFERENCES_EVALUATOR_EVENTS_V1"), "observed_b": 1, "known_e": 0, "observed_e_b": 0, "observed_e_t": 0, "whole_result_p": 0, "proposed_outcome": "COMPLETE_EMPTY", "proposed_disposition": "EMPTY"}
	candidate := map[string]any{"schema_version": "REFERENCES_OCCURRENCE_CANDIDATE_V1", "context": ctx, "proposal_ref": ref("REFERENCES_EVALUATION_PROPOSAL_V1"), "occurrences": []any{}, "proposed_p": 0}
	final := map[string]any{"schema_version": "REFERENCES_FINAL_ISSUANCE_V1", "context": ctx, "proposal_ref": ref("REFERENCES_EVALUATION_PROPOSAL_V1"), "candidate_ref": ref("REFERENCES_OCCURRENCE_CANDIDATE_V1"), "issued_outcome": "COMPLETE_EMPTY", "issued_disposition": "EMPTY", "n": 1, "b": 1, "t": 1, "e": 0, "e_b": 0, "e_t": 0, "p": 0, "a": 0, "dependency_refs": []any{ref("REFERENCES_TARGET_V1"), ref("REFERENCES_METHOD_V1"), ref("REFERENCES_RAW_RESULT_V1"), ref("REFERENCES_EVALUATOR_EVENTS_V1"), ref("REFERENCES_EVALUATION_PROPOSAL_V1"), ref("REFERENCES_OCCURRENCE_CANDIDATE_V1"), ref("REFERENCES_SOURCE_IDENTITY_V1"), ref("REFERENCES_REVISION_IDENTITY_V1")}}
	events := map[string]any{"schema_version": "REFERENCES_EVALUATOR_EVENTS_V1", "transaction_id": d, "request_key": "k", "invocation_id": "i", "response_read_ref": ref("REFERENCES_RAW_RESULT_V1"), "events": []any{map[string]any{"sequence": 0, "kind": "QUERY_BEGIN", "ordinal": nil, "terminal_disposition": "NONE"}, map[string]any{"sequence": 1, "kind": "QUERY_TERMINAL", "ordinal": nil, "terminal_disposition": "EMPTY"}}}
	check := func(label string, x map[string]any, valid bool) {
		t.Helper()
		e := s.Validate(x)
		if (e == nil) != valid {
			t.Errorf("%s: valid=%v want=%v: %v", label, e == nil, valid, e)
		}
	}
	for name, x := range map[string]map[string]any{"proposal": proposal, "candidate": candidate, "final": final, "events": events} {
		check(name, x, true)
	}
	mutate := func(base map[string]any, field string, value any) map[string]any {
		b, _ := json.Marshal(base)
		var x map[string]any
		_ = json.Unmarshal(b, &x)
		x[field] = value
		return x
	}
	check("unknown version", mutate(proposal, "schema_version", "REFERENCES_EVALUATION_PROPOSAL_V2"), false)
	check("raw limit accepted", mutate(proposal, "raw_result_byte_length", 1048576), true)
	check("ASSERT_ISSUANCE_RAW_LIMIT_REJECTED", mutate(proposal, "raw_result_byte_length", 1048577), false)
	for _, tc := range []struct {
		name  string
		base  map[string]any
		field string
	}{
		{"proposal raw", proposal, "raw_result_ref"},
		{"proposal scanner", proposal, "scanner_observation_ref"},
		{"proposal evaluator", proposal, "evaluator_event_ref"},
		{"candidate predecessor", candidate, "proposal_ref"},
		{"final proposal", final, "proposal_ref"},
		{"final candidate", final, "candidate_ref"},
		{"events response", events, "response_read_ref"},
	} {
		x := mutate(tc.base, tc.field, ref("REFERENCES_TARGET_V1"))
		check("ASSERT_ISSUANCE_ROLE_REJECTED "+tc.name, x, false)
	}
	for _, field := range []string{"method_ref", "target_ref", "source_identity_ref", "revision_identity_ref", "method_policy_ref", "admission_policy_ref", "privacy_policy_ref", "retention_policy_ref"} {
		b, _ := json.Marshal(ctx)
		var wrong map[string]any
		_ = json.Unmarshal(b, &wrong)
		wrong[field] = ref("REFERENCES_RAW_RESULT_V1")
		check("ASSERT_CONTEXT_ROLE_REJECTED "+field, mutate(proposal, "context", wrong), false)
	}
	check("proposal admitted A", mutate(proposal, "a", 0), false)
	check("candidate terminal", mutate(candidate, "terminal_ref", ref("REFERENCES_FINAL_ISSUANCE_V1")), false)
	check("candidate admitted", mutate(candidate, "admitted", true), false)
	check("final missing terminal", mutate(final, "t", 0), false)
	check("final empty with A", mutate(final, "a", 1), false)
	check("bad outcome pair", mutate(proposal, "proposed_disposition", "ITEMS"), false)
	check("event wrong begin ordinal", mutate(events, "events", []any{map[string]any{"sequence": 0, "kind": "QUERY_BEGIN", "ordinal": 0, "terminal_disposition": "NONE"}, map[string]any{"sequence": 1, "kind": "QUERY_TERMINAL", "ordinal": nil, "terminal_disposition": "EMPTY"}}), false)
	check("extra context", mutate(proposal, "context", func() map[string]any {
		b, _ := json.Marshal(ctx)
		var x map[string]any
		_ = json.Unmarshal(b, &x)
		x["source_identity"] = "opaque"
		return x
	}()), false)
}
