package mcpcontract

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestStructuralContextPagingSuccessorsRegisteredButInactive(t *testing.T) {
	for _, id := range []string{SourceProjectionRequestV3ID, StructuralContextPagingInputID, SourceProjectionResultV3ID, UnifiedStructuralContextResultV3ID, StructuralContextPagingSuccessID} {
		raw, err := SchemaJSON(id)
		if err != nil || len(raw) == 0 {
			t.Fatalf("ASSERT_PAGING_SUCCESSOR_REGISTERED[%s]: bytes=%d err=%v", id, len(raw), err)
		}
	}
	manifest, err := LoadManifest()
	if err != nil {
		t.Fatal(err)
	}
	manifest = WithStructuralContextV2(manifest)
	for _, tool := range manifest.Tools {
		if tool.Name != StructuralContextV2Tool {
			continue
		}
		if tool.InputSchemaID != StructuralContextRegexLocatorInputID || !reflect.DeepEqual(tool.ArtifactSchemaIDs, []string{UnifiedStructuralContextResultV2ID}) || !reflect.DeepEqual(tool.EnvelopeSchemaIDs, []string{StructuralContextProjectionSuccessID, StructuralContextTraversalDomainErrorID}) {
			t.Fatalf("ASSERT_PAGING_SUCCESSOR_INACTIVE: %+v", tool)
		}
		return
	}
	t.Fatal("ASSERT_PAGING_SUCCESSOR_INACTIVE: operation 36 absent")
}

func TestSourceProjectionRequestV3RequiresExplicitPaging(t *testing.T) {
	base := map[string]any{
		"mode": "TARGET", "body": "OMIT", "include_relation_occurrences": false, "include_ancillary": false,
		"display_range_policy": "FULL_DEFINITION", "privacy_policy_id": "public",
		"limits": map[string]any{
			"max_objects": 1, "max_ranges": 1, "max_source_bytes": 0, "max_work": 1, "max_response_bytes": 4096,
			"max_additional_documents": 0, "max_document_requests": 1, "max_document_bytes": 1024,
			"max_total_document_bytes": 1024, "max_document_messages": 1, "max_document_acquisition_work": 1, "max_display_resolution_work": 1,
		},
	}
	raw, _ := json.Marshal(base)
	if err := ValidateJSON(SourceProjectionRequestV3ID, raw); err == nil {
		t.Fatal("ASSERT_PAGING_V3_REQUIRES_PAGING")
	}
	base["paging"] = map[string]any{"max_page_bytes": 1024, "max_pages": 4, "max_response_bytes": 8192}
	raw, _ = json.Marshal(base)
	if err := ValidateJSON(SourceProjectionRequestV3ID, raw); err != nil {
		t.Fatalf("ASSERT_PAGING_V3_VALID: %v", err)
	}
	base["paging"].(map[string]any)["max_pages"] = 0
	raw, _ = json.Marshal(base)
	if err := ValidateJSON(SourceProjectionRequestV3ID, raw); err == nil {
		t.Fatal("ASSERT_PAGING_V3_ZERO_PAGE_BOUND_REJECTED")
	}
}

func TestSourceProjectionResultV3ClosedPageContract(t *testing.T) {
	valid := map[string]any{
		"schema_version": "lsp-trace.source-projection.v3", "authority": 0, "source_graph_complete": "UNKNOWN", "graph_facts_added": 0,
		"custody_mode": "LIVE", "physical_projection_id": "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		"request_policy_id": "public", "status": "COMPLETE", "records": []any{},
		"accounting": map[string]any{"pages": 1, "response_bytes": 512, "objects": 0, "ranges": 0, "source_bytes": 0, "work": 0},
		"complete":   false, "next_cursor": "cursor",
	}
	raw, _ := json.Marshal(valid)
	if err := ValidateJSON(SourceProjectionResultV3ID, raw); err != nil {
		t.Fatalf("ASSERT_PAGING_V3_RESULT_VALID: %v", err)
	}
	delete(valid, "next_cursor")
	raw, _ = json.Marshal(valid)
	if err := ValidateJSON(SourceProjectionResultV3ID, raw); err == nil {
		t.Fatal("ASSERT_PAGING_V3_INCOMPLETE_REQUIRES_CURSOR")
	}
	valid["complete"] = true
	valid["next_cursor"] = "cursor"
	raw, _ = json.Marshal(valid)
	if err := ValidateJSON(SourceProjectionResultV3ID, raw); err == nil {
		t.Fatal("ASSERT_PAGING_V3_COMPLETE_FORBIDS_CURSOR")
	}
}
