package censusresult

import (
	"encoding/json"
	"strings"
	"testing"
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

func uint64ptr(v uint64) *uint64 { return &v }
