package censusresult

import (
	"encoding/json"
	"strings"
	"testing"

	"lsp-trace/internal/censusrequest"
)

func TestDiscoveryDiagnosticIsClosedActionableAndPrivacySafe(t *testing.T) {
	d, err := NewDiscoveryDiagnostic([]DiscoveryObservation{
		{Dimension: DimensionNodeCeiling, State: ObservationKnown, Observed: uint64ptr(10000), Limit: uint64ptr(10000), Omitted: uint64ptr(37)},
		{Dimension: DimensionDepthBoundary, State: ObservationKnown, Observed: uint64ptr(1), Limit: uint64ptr(1)},
		{Dimension: DimensionTargetSourceOmissions, State: ObservationKnown, Omitted: uint64ptr(37)},
		{Dimension: DimensionProviderCompleteness, State: ObservationUnknown},
		{Dimension: DimensionTimeoutRequestBudget, State: ObservationKnown, Observed: uint64ptr(30000), Limit: uint64ptr(60000)},
		{Dimension: DimensionMessageByteBudget, State: ObservationUnknown},
	})
	if err != nil {
		t.Fatal(err)
	}
	if d.CallerAction != "REQUIRED" || !d.Retry || d.Invariant == "" || d.CorrectionFragment == "" {
		t.Fatalf("not actionable: %+v", d)
	}
	raw, err := MarshalDiscoveryDiagnostic(d)
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	for _, forbidden := range []string{"/Users/", "pipeline.go", "capture.go"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("privacy leak %q", forbidden)
		}
	}
	var value map[string]any
	if err := json.Unmarshal(raw, &value); err != nil {
		t.Fatal(err)
	}
	value["unknown"] = true
	bad, _ := json.Marshal(value)
	if _, err := DecodeDiscoveryDiagnostic(bad); err == nil {
		t.Fatal("accepted unknown field")
	}
}

func TestTargetSourceOmissionsDoNotSuggestTraversalMaxNodes(t *testing.T) {
	for _, test := range []struct {
		name     string
		maxNodes uint64
	}{{"below_200", 199}, {"equal_200", 200}, {"above_200", 201}} {
		t.Run(test.name, func(t *testing.T) {
			maxNodes := test.maxNodes
			d, err := NewDiscoveryDiagnosticForRequest([]DiscoveryObservation{{Dimension: DimensionTargetSourceOmissions, State: ObservationKnown, Omitted: uint64ptr(1)}}, censusrequest.SemanticFields{
				MaxNodes: maxNodes, TimeoutMS: censusrequest.DefaultTimeoutMS, RequestTimeoutMS: censusrequest.DefaultRequestTimeoutMS,
			})
			if err != nil {
				t.Fatal(err)
			}
			if d.CorrectionFragment != "{}" {
				t.Fatalf("ASSERT_TARGET_CAP_HAS_NO_MAX_NODES_CORRECTION: max_nodes=%d fragment=%s", maxNodes, d.CorrectionFragment)
			}
			if len(d.Observations) != 1 || d.Observations[0].Dimension != DimensionTargetSourceOmissions || d.Observations[0].State != ObservationKnown || d.Observations[0].Omitted == nil || *d.Observations[0].Omitted != 1 {
				t.Fatalf("ASSERT_TARGET_CAP_OMISSION_PRESERVED: max_nodes=%d observations=%+v", maxNodes, d.Observations)
			}
		})
	}
}

func TestTraversalNodeCeilingStillSuggestsMaxNodes(t *testing.T) {
	d, err := NewDiscoveryDiagnosticForRequest([]DiscoveryObservation{{Dimension: DimensionNodeCeiling, State: ObservationKnown, Limit: uint64ptr(199), Omitted: uint64ptr(1)}}, censusrequest.SemanticFields{
		MaxNodes: 199, TimeoutMS: censusrequest.DefaultTimeoutMS, RequestTimeoutMS: censusrequest.DefaultRequestTimeoutMS,
	})
	if err != nil {
		t.Fatal(err)
	}
	if d.CorrectionFragment != `{"max_nodes":398}` {
		t.Fatalf("ASSERT_TRAVERSAL_NODE_CEILING_MAX_NODES_CORRECTION: %s", d.CorrectionFragment)
	}
}

func uint64ptr(v uint64) *uint64 { return &v }
