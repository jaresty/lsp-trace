package normativeanalytics

import "testing"

const InventoryStatus = "synthetic fixture-qualified, package-private, unshipped"

func TestSyntheticFixtureQualificationIsNotExecutionAuthority(t *testing.T) {
	if InventoryStatus != "synthetic fixture-qualified, package-private, unshipped" {
		t.Fatal("ASSERT_EXACT_SYNTHETIC_INVENTORY_STATUS")
	}
	got, err := Evaluate(Request{Operation: Analysis, BuildRevision: "local", Graph: Graph{Nodes: []string{"n"}}, Policy: Policy{MaxWork: 1}})
	if err != nil || got.Status != Complete {
		t.Fatalf("ASSERT_LOCAL_ANALYTICS_NEEDS_NO_PROGRAM_B_ADMISSION: %#v %v", got, err)
	}
}
