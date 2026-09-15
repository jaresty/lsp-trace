package transientstructural

import "testing"

func TestDiagnoseTruncationIsDeterministicAndBounded(t *testing.T) {
	failure := &DomainFailure{Phase: PhaseTraversal, State: StateTruncated, Accounting: Accounting{Nodes: AdmissionAccounting{Observed: 137, Admitted: 100}, Frontier: FrontierAccounting{Unexpanded: 37}, Omissions: []OmissionCount{{Reason: OmissionNodeBound, Count: 37}}}}
	got := DiagnoseTruncation(Request{MaxNodes: 100}, failure)
	if got == nil || got.Reason != string(OmissionNodeBound) || got.NodeLimit != 100 || got.NodesObserved != 137 || got.NodesAdmitted != 100 || got.FrontierUnexpanded != 37 || len(got.Suggestions) != 4 {
		t.Fatalf("ASSERT_TRUNCATION_DIAGNOSTIC_ACTIONABLE: %+v", got)
	}
	if DiagnoseTruncation(Request{}, &DomainFailure{State: StatePartial}) != nil {
		t.Fatal("ASSERT_NONTRUNCATED_DIAGNOSTIC_ABSENT")
	}
}
