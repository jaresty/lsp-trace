package schema

import (
	"bytes"
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
