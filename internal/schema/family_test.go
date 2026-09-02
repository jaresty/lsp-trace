package schema

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

const validFilterV1 = `{
  "filter_schema_version":"lsp-trace.filter.v1",
  "projection_kind":"SEED_EVIDENCE_COMPARISON",
  "authority":"TOOL_DERIVED_SET_PROJECTION",
  "support_contribution":0,
  "native_semantics_policy":"PRESERVE_WITHOUT_AUTHORITY_UPGRADE",
  "input_identity":{
    "inspection_exact_bytes_digest":"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
    "artifact_semantic_commitment_digest":"sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
    "artifact_exact_serialized_bytes_digest":"sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc",
    "execution_bundle_id":"sha256:dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd"
  },
  "operands":{"left_seed_label":"left","right_seed_label":"right"},
  "seeds":[{"label":"left","state":"SUCCESSFUL_EMPTY","failure":null},{"label":"right","state":"FAILED","failure":{"phase":"prepare","message":"failed"}}],
  "partitions":{
    "nodes":{"shared":[],"left_only":[],"right_only":[]},
    "call_relations":{"shared":[],"left_only":[],"right_only":[]},
    "dispatch_relationships":{"shared":[],"left_only":[],"right_only":[]},
    "sibling_candidates":{"shared":[],"left_only":[],"right_only":[]},
    "diagnostic_correlations":{"shared":[],"left_only":[],"right_only":[]}
  },
  "global_boundary":{"truncated":false,"traversal_complete":true,"source_graph_complete":"UNKNOWN"},
  "accounting":{
    "requested_seed_count":2,"successful_seed_count":1,"failed_seed_count":1,
    "successful_seed_with_membership_count":0,"successful_seed_without_membership_count":1,
    "nodes":{"left_reference_count":0,"right_reference_count":0,"shared_reference_count":0,"left_only_reference_count":0,"right_only_reference_count":0,"pair_universe_count":0},
    "call_relations":{"left_reference_count":0,"right_reference_count":0,"shared_reference_count":0,"left_only_reference_count":0,"right_only_reference_count":0,"pair_universe_count":0},
    "dispatch_relationships":{"left_reference_count":0,"right_reference_count":0,"shared_reference_count":0,"left_only_reference_count":0,"right_only_reference_count":0,"pair_universe_count":0},
    "sibling_candidates":{"left_reference_count":0,"right_reference_count":0,"shared_reference_count":0,"left_only_reference_count":0,"right_only_reference_count":0,"pair_universe_count":0},
    "diagnostic_correlations":{"left_reference_count":0,"right_reference_count":0,"shared_reference_count":0,"left_only_reference_count":0,"right_only_reference_count":0,"pair_universe_count":0}
  },
  "claim_ceiling":{
    "supports":["EXACT_REFERENCE_INTERSECTION","EXACT_REFERENCE_DIFFERENCE"],
    "does_not_support":["SHARED_FEATURE_PURPOSE","DISTINCT_FEATURE_PURPOSE","FEATURE_IDENTITY","WORKFLOW_IDENTITY","MERGE_OR_SPLIT_DISPOSITION","INDEPENDENT_OBSERVATION","EVIDENTIARY_SUPPORT","CONFIDENCE","COVERAGE","RUNTIME_BEHAVIOR","ACCEPTANCE"]
  }
}`

func TestFamilySchemasMatchCommittedBytesDeterministically(t *testing.T) {
	for _, tc := range []struct{ family, version, full string }{
		{FamilyGraph, "v3", "lsp-trace.graph.v3"},
		{FamilyInspect, "v1", "lsp-trace.inspect.v1"},
		{FamilyFilter, "v1", "lsp-trace.filter.v1"},
	} {
		first, err := BytesFor(tc.family, tc.version)
		if err != nil {
			t.Fatalf("ASSERT_FAMILY_SCHEMA_BYTES: %v", err)
		}
		second, err := BytesFor(tc.family, tc.full)
		if err != nil {
			t.Fatalf("ASSERT_FAMILY_SCHEMA_BYTES: %v", err)
		}
		committed, err := os.ReadFile("schemas/" + tc.full + ".schema.json")
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(first, second) || !bytes.Equal(first, committed) {
			t.Fatalf("ASSERT_FAMILY_SCHEMA_BYTES: %s bytes differ", tc.full)
		}
	}
}

func TestValidateForInspectRunsAllSeedSemanticsAfterStructure(t *testing.T) {
	mutated := strings.Replace(minimalAllSeedInspection, `"requested_seed_count":0`, `"requested_seed_count":1`, 1)
	_, err := ValidateFor([]byte(mutated), FamilyInspect, "v1")
	if err == nil || !strings.Contains(err.Error(), "semantic validation") || strings.Contains(err.Error(), "schema validation") {
		t.Fatalf("ASSERT_INSPECT_FAMILY_LAYERED_VALIDATION: %v", err)
	}
}

func TestValidateForFilterFamilyAndRejectCrossFamily(t *testing.T) {
	detected, err := ValidateFor([]byte(validFilterV1), FamilyFilter, "v1")
	if err != nil || detected != "lsp-trace.filter.v1" {
		t.Fatalf("ASSERT_FILTER_FAMILY_VALIDATION: detected=%q err=%v", detected, err)
	}
	_, err = ValidateFor([]byte(validFilterV1), FamilyGraph, "v1")
	if err == nil || !strings.Contains(err.Error(), "schema_version") {
		t.Fatalf("ASSERT_FILTER_CROSS_FAMILY_REJECTED: %v", err)
	}
}
