package mcp

import (
	"reflect"
	"testing"

	"lsp-trace/internal/schema"
)

func TestPublicAnalyticsV2WiringRED(t *testing.T) {
	const assertion = "ASSERT_PUBLIC_ANALYTICS_V2_WIRING"
	r := NewRegistry(false)
	tools := r.Tools()
	if len(tools) != 28 {
		t.Fatalf("%s: tool count=%d want=28", assertion, len(tools))
	}
	wantTail := []string{
		"lsp_trace_v2_bounded_retained_analysis",
		"lsp_trace_v2_bounded_retained_metrics",
		"lsp_trace_v2_bounded_retained_ranking",
	}
	gotTail := []string{tools[25].Name, tools[26].Name, tools[27].Name}
	if !reflect.DeepEqual(gotTail, wantTail) {
		t.Fatalf("%s: tail=%v want=%v", assertion, gotTail, wantTail)
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
}
