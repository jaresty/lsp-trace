package mcp

import (
	"bytes"
	"reflect"
	"testing"

	"lsp-trace/internal/mcpcontract"
	"lsp-trace/internal/schema"
)

func TestPublicAnalyticsV2PresentationSchema(t *testing.T) {
	canonicalBefore, err := mcpcontract.SchemaJSON(mcpcontract.PublicAnalyticsV2InputID)
	if err != nil {
		t.Fatal(err)
	}
	r := NewRegistry(false)
	operations := map[string]string{
		"lsp_trace_v2_bounded_retained_analysis": "ANALYSIS",
		"lsp_trace_v2_bounded_retained_metrics":  "METRICS",
		"lsp_trace_v2_bounded_retained_ranking":  "RANKING",
	}
	for name, operation := range operations {
		tool, ok := r.Resolve(name)
		if !ok {
			t.Fatalf("ASSERT_ANALYTICS_PRESENTATION_TOOL: missing %s", name)
		}
		variants, ok := tool.PresentationInputSchema["oneOf"].([]any)
		if !ok || len(variants) != 2 {
			t.Fatalf("ASSERT_ANALYTICS_PRESENTATION_COMPLETE_UNION: %s %#v", name, tool.PresentationInputSchema)
		}
		carriers := map[string]int{}
		for _, raw := range variants {
			variant, _ := raw.(map[string]any)
			properties, _ := variant["properties"].(map[string]any)
			operationSchema, _ := properties["operation"].(map[string]any)
			if variant["type"] != "object" || variant["additionalProperties"] != false || len(properties) < 5 || operationSchema["const"] != operation {
				t.Fatalf("ASSERT_ANALYTICS_PRESENTATION_RENDERABLE_OBJECT: %s %#v", name, variant)
			}
			for _, carrier := range []string{"input", "publication_selector"} {
				if _, exists := properties[carrier]; exists {
					carriers[carrier]++
				}
			}
		}
		if carriers["input"] != 1 || carriers["publication_selector"] != 1 {
			t.Fatalf("ASSERT_ANALYTICS_PRESENTATION_EXACTLY_ONE_CARRIER: %s %v", name, carriers)
		}
	}
	canonicalAfter, err := mcpcontract.SchemaJSON(mcpcontract.PublicAnalyticsV2InputID)
	if err != nil || !bytes.Equal(canonicalBefore, canonicalAfter) {
		t.Fatalf("ASSERT_ANALYTICS_CANONICAL_SCHEMA_IMMUTABLE: err=%v", err)
	}
}

func TestPublicAnalyticsV2WiringRED(t *testing.T) {
	const assertion = "ASSERT_PUBLIC_ANALYTICS_V2_WIRING"
	r := NewRegistry(false)
	tools := r.Tools()
	if len(tools) != 28 {
		t.Fatalf("%s: tool count=%d want=28", assertion, len(tools))
	}
	want := []string{
		"lsp_trace_v2_bounded_retained_analysis",
		"lsp_trace_v2_bounded_retained_metrics",
		"lsp_trace_v2_bounded_retained_ranking",
	}
	got := []string{}
	for _, tool := range tools {
		for _, name := range want {
			if tool.Name == name {
				got = append(got, name)
			}
		}
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("%s: tools=%v want=%v", assertion, got, want)
	}
	for _, family := range []string{
		"bounded-retained-graph-v2",
		"bounded-retained-analysis-v2",
		"bounded-retained-metrics-v2",
		"bounded-retained-ranking-v2",
	} {
		if _, err := schema.BytesFor(family, "v2"); err != nil {
			t.Fatalf("%s: family=%s: %v", assertion, family, err)
		}
	}
	wantSchemaIDs := []string{
		mcpcontract.PublicAnalyticsV2GraphArtifactID,
		mcpcontract.PublicAnalyticsV2AnalysisArtifactID,
		mcpcontract.PublicAnalyticsV2MetricsArtifactID,
		mcpcontract.PublicAnalyticsV2RankingArtifactID,
	}
	for _, name := range []string{"lsp_trace_v1_schema_get", "lsp_trace_v1_validate"} {
		tool, ok := r.Resolve(name)
		if !ok {
			t.Fatalf("%s: missing tool=%s", assertion, name)
		}
		for _, id := range wantSchemaIDs {
			if !containsString(tool.ArtifactSchemaIDs, id) {
				t.Fatalf("ASSERT_PUBLIC_ANALYTICS_V2_EXACT_SCHEMA_PERMISSION: tool=%s missing=%s schemas=%v", name, id, tool.ArtifactSchemaIDs)
			}
		}
	}
}
