package schema

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

const minimalInspection = `{
  "inspection_schema_version":"lsp-trace.inspect.v1",
  "projection_kind":"SEED_INSPECTION",
  "authority":"NON_AUTHORITATIVE_DERIVED_VIEW",
  "artifact_identity":{"semantic_commitment_digest":"sha256:semantic","exact_serialized_bytes_digest":"sha256:exact"},
  "preparation_status":"SUCCEEDED",
  "seed":{"label":"entry","requested_position":{"uri":"file:///w/entry.go","line":1,"column":1},"prepared_target_ids":[],"reached_node_ids":[],"reached_relation_ids":[]},
  "seed_memberships":[],
  "nodes":[],
  "relations":[],
  "global":{"summary":{"node_count":0,"edge_count":0,"terminal_count":0,"cycle_count":0,"traversal_complete":true,"source_graph_complete":"COMPLETE","completeness_scope":"SERVER_REPORTED_BOUNDED","truncated":false},"terminals":[],"frontier":[],"diagnostics":[]},
  "diagnostics_on_reached_nodes":{"authority":"TOOL_DERIVED_NODE_CORRELATION","diagnostics":[]}
}`

const minimalAllSeedInspection = `{
  "inspection_schema_version":"lsp-trace.inspect.v1",
  "projection_kind":"ALL_SEEDS",
  "authority":"NON_AUTHORITATIVE_DERIVED_VIEW",
  "artifact_identity":{"semantic_commitment_digest":"sha256:semantic","exact_serialized_bytes_digest":"sha256:exact"},
  "records":{"nodes":[],"call_relations":[],"dispatch_relationships":[],"sibling_candidates":[],"diagnostics":[],"terminals":[],"frontier":[]},
  "seeds":[],
  "accounting":{"requested_seed_count":0,"successful_seed_count":0,"failed_seed_count":0,"successful_seed_with_membership_count":0,"successful_seed_without_membership_count":0,"global_node_record_count":0,"global_call_relation_record_count":0,"global_dispatch_relationship_record_count":0,"global_sibling_candidate_record_count":0,"global_diagnostic_record_count":0,"global_terminal_record_count":0,"global_frontier_record_count":0,"seed_membership_record_count":0,"seed_node_reference_count":0,"seed_call_relation_reference_count":0,"seed_discovery_nomination_reference_count":0,"seed_correlated_diagnostic_reference_count":0,"truncated":false,"traversal_complete":true,"source_graph_complete":"COMPLETE"}
}`

func TestInspectionSchemaBytesMatchCommittedAsset(t *testing.T) {
	got, err := InspectionBytes(InspectionVersionV1)
	if err != nil {
		t.Fatalf("ASSERT_INSPECTION_SCHEMA_BYTES_MATCH: %v", err)
	}
	want, err := os.ReadFile("schemas/lsp-trace.inspect.v1.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatal("ASSERT_INSPECTION_SCHEMA_BYTES_MATCH: embedded bytes differ from committed asset")
	}
}

func TestValidateInspectionAcceptsValidProjection(t *testing.T) {
	if err := ValidateInspection([]byte(minimalInspection)); err != nil {
		t.Fatalf("ASSERT_INSPECTION_SCHEMA_ACCEPTS_VALID_PROJECTION: %v", err)
	}
}

const referencedAllSeedInspection = `{
  "inspection_schema_version":"lsp-trace.inspect.v1",
  "projection_kind":"ALL_SEEDS",
  "authority":"NON_AUTHORITATIVE_DERIVED_VIEW",
  "artifact_identity":{"semantic_commitment_digest":"sha256:semantic","exact_serialized_bytes_digest":"sha256:exact"},
  "records":{"nodes":[{"id":"node-1"}],"call_relations":[{"relation_id":"call-1"}],"dispatch_relationships":[{"relation_id":"dispatch-1"}],"sibling_candidates":[{"relation_id":"sibling-1"}],"diagnostics":[{}],"terminals":[],"frontier":[]},
  "seeds":[{"preparation_status":"SUCCEEDED","seed":{"label":"entry","requested_position":{"uri":"file:///w/entry.go","line":1,"column":1},"prepared_target_ids":[],"reached_node_ids":[],"reached_relation_ids":[]},"seed_memberships":[{"seed_label":"entry","evidence_kind":"DISPATCH_ASSOCIATION","endpoint_id":"dispatch-1"},{"seed_label":"entry","evidence_kind":"SIBLING_CANDIDATE","endpoint_id":"sibling-1"}],"native_node_ids":["node-1"],"native_call_relation_ids":["call-1"],"discovery_nomination_ids":["dispatch-1","sibling-1"],"correlated_diagnostic_indexes":[0]}],
  "accounting":{"requested_seed_count":1,"successful_seed_count":1,"failed_seed_count":0,"successful_seed_with_membership_count":1,"successful_seed_without_membership_count":0,"global_node_record_count":1,"global_call_relation_record_count":1,"global_dispatch_relationship_record_count":1,"global_sibling_candidate_record_count":1,"global_diagnostic_record_count":1,"global_terminal_record_count":0,"global_frontier_record_count":0,"seed_membership_record_count":2,"seed_node_reference_count":1,"seed_call_relation_reference_count":1,"seed_discovery_nomination_reference_count":2,"seed_correlated_diagnostic_reference_count":1,"truncated":false,"traversal_complete":true,"source_graph_complete":"COMPLETE"}
}`

func TestValidateAllSeedInspectionAcceptsSemanticallyValidDocument(t *testing.T) {
	for _, document := range []string{minimalAllSeedInspection, referencedAllSeedInspection} {
		if err := ValidateAllSeedInspection([]byte(document)); err != nil {
			t.Fatalf("ASSERT_ALL_SEEDS_ADMISSION_ACCEPTS_VALID_DOCUMENT: %v", err)
		}
	}
}

func TestValidateAllSeedInspectionKeepsReferenceNamespacesDistinct(t *testing.T) {
	var document map[string]any
	if err := json.Unmarshal([]byte(referencedAllSeedInspection), &document); err != nil {
		t.Fatal(err)
	}
	records := document["records"].(map[string]any)
	records["dispatch_relationships"].([]any)[0].(map[string]any)["relation_id"] = "shared"
	records["sibling_candidates"].([]any)[0].(map[string]any)["relation_id"] = "shared"
	memberships := seed(document)["seed_memberships"].([]any)
	memberships[0].(map[string]any)["endpoint_id"] = "shared"
	memberships[1].(map[string]any)["endpoint_id"] = "shared"
	seed(document)["discovery_nomination_ids"] = []any{"shared", "shared"}
	encoded, err := json.Marshal(document)
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateAllSeedInspection(encoded); err != nil {
		t.Fatalf("ASSERT_ALL_SEEDS_ADMISSION_DOMAIN_SEPARATES_REFERENCE_KEYS: %v", err)
	}
}

func TestValidateAllSeedInspectionRejectsControlledPerturbations(t *testing.T) {
	tests := []struct {
		name   string
		want   string
		mutate func(map[string]any)
	}{
		{"duplicate_label", "duplicate seed label", func(d map[string]any) { seeds := d["seeds"].([]any); d["seeds"] = append(seeds, seeds[0]) }},
		{"failed_seed_references", "failed seed", func(d map[string]any) { seed(d)["preparation_status"] = "FAILED" }},
		{"duplicate_global_identity", "duplicate NODE record identity", func(d map[string]any) {
			records := d["records"].(map[string]any)
			nodes := records["nodes"].([]any)
			records["nodes"] = append(nodes, nodes[0])
		}},
		{"unresolved_reference", "unresolved NODE reference", func(d map[string]any) { seed(d)["native_node_ids"] = []any{"missing"} }},
		{"duplicate_reference", "duplicate NODE reference", func(d map[string]any) { seed(d)["native_node_ids"] = []any{"node-1", "node-1"} }},
		{"duplicate_dispatch_reference", "duplicate DISPATCH_RELATIONSHIP reference", func(d map[string]any) {
			memberships := seed(d)["seed_memberships"].([]any)
			memberships = append(memberships, memberships[0])
			seed(d)["seed_memberships"] = memberships
			seed(d)["discovery_nomination_ids"] = []any{"dispatch-1", "sibling-1", "dispatch-1"}
		}},
		{"diagnostic_out_of_range", "unresolved DIAGNOSTIC_CORRELATION", func(d map[string]any) { seed(d)["correlated_diagnostic_indexes"] = []any{1.0} }},
		{"membership_label_mismatch", "does not match seed", func(d map[string]any) {
			memberships := seed(d)["seed_memberships"].([]any)
			memberships[0].(map[string]any)["seed_label"] = "other"
		}},
		{"nomination_mismatch", "do not match seed memberships", func(d map[string]any) { seed(d)["discovery_nomination_ids"] = []any{"sibling-1", "dispatch-1"} }},
		{"accounting_mismatch", "invalid inspection accounting", func(d map[string]any) { d["accounting"].(map[string]any)["requested_seed_count"] = 2.0 }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var document map[string]any
			if err := json.Unmarshal([]byte(referencedAllSeedInspection), &document); err != nil {
				t.Fatal(err)
			}
			test.mutate(document)
			encoded, err := json.Marshal(document)
			if err != nil {
				t.Fatal(err)
			}
			if err := ValidateAllSeedInspection(encoded); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("ASSERT_ALL_SEEDS_ADMISSION_REJECTS_%s: got=%v want substring %q", test.name, err, test.want)
			}
		})
	}
}

func TestValidateAllSeedInspectionRejectsOtherInspectionFamily(t *testing.T) {
	if err := ValidateAllSeedInspection([]byte(minimalInspection)); err == nil || !strings.Contains(err.Error(), "ALL_SEEDS") {
		t.Fatalf("ASSERT_ALL_SEEDS_ADMISSION_DISCRIMINATES_FAMILY: %v", err)
	}
}

func seed(document map[string]any) map[string]any {
	return document["seeds"].([]any)[0].(map[string]any)
}

func TestValidateInspectionRejectsMissingAllSeedAccountingField(t *testing.T) {
	invalid := strings.Replace(minimalAllSeedInspection, `"requested_seed_count":0,`, "", 1)
	if err := ValidateInspection([]byte(invalid)); err == nil || !strings.Contains(err.Error(), "requested_seed_count") {
		t.Fatalf("ASSERT_INSPECTION_SCHEMA_REQUIRES_ACCOUNTING_FIELDS: %v", err)
	}
}

func TestValidateInspectionRejectsMissingAuthority(t *testing.T) {
	invalid := strings.Replace(minimalInspection, `"authority":"NON_AUTHORITATIVE_DERIVED_VIEW",`, "", 1)
	if err := ValidateInspection([]byte(invalid)); err == nil || !strings.Contains(err.Error(), "authority") {
		t.Fatalf("ASSERT_INSPECTION_SCHEMA_REJECTS_MISSING_AUTHORITY: %v", err)
	}
}

func TestValidateInspectionRejectsGraphVersionInPlaceOfInspectionVersion(t *testing.T) {
	invalid := strings.Replace(minimalInspection, "lsp-trace.inspect.v1", "lsp-trace.graph.v3", 1)
	if err := ValidateInspection([]byte(invalid)); err == nil || !strings.Contains(err.Error(), "inspection_schema_version") {
		t.Fatalf("ASSERT_INSPECTION_API_IS_INDEPENDENT_OF_GRAPH_VERSION: %v", err)
	}
}
