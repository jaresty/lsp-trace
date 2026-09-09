package normativeanalytics

import (
	"math"
	"testing"
)

const InventoryStatus = "synthetic fixture-qualified, package-private, unshipped"

func TestSyntheticFixtureQualificationIsNotExecutionAuthority(t *testing.T) {
	if InventoryStatus != "synthetic fixture-qualified, package-private, unshipped" || LocalScope != "LOCAL_SYNTHETIC_FIXTURE_QUALIFIED_PACKAGE_PRIVATE_UNSHIPPED" {
		t.Fatal("ASSERT_EXACT_SYNTHETIC_INVENTORY_STATUS")
	}
	raw := graphBytes(t, []string{"n"}, []retainedEdgeJSON{})
	got, err := EvaluateLocal(LocalRequest{Operation: Analysis, BuildRevision: "rev", RetainedJSON: raw, Relations: []string{"CALLS"}, Policy: LocalPolicy{MaxWork: math.MaxInt64}})
	if err != nil || got.Status != Complete || got.Scope != LocalScope {
		t.Fatalf("ASSERT_LOCAL_ANALYTICS_NEEDS_NO_PROGRAM_B_ADMISSION: %#v %v", got, err)
	}
	if LocalDescriptors()[0].Protocol != "" {
		t.Fatal("ASSERT_LOCAL_DESCRIPTORS_UNREGISTERED")
	}
}
