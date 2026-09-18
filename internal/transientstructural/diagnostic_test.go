package transientstructural

import (
	"testing"

	"lsp-trace/internal/regexlocator"
)

func TestDiagnoseResourceLimitReportsExactRegexBoundAndCapsSuggestion(t *testing.T) {
	observed, suggested := 125, 120
	failure := &DomainFailure{Phase: PhasePreflight, State: StateResourceLimit, ResourceDiagnostic: &ResourceDiagnostic{Reason: ResourceReasonRegexMaxDocumentBytes, Field: ResourceFieldRegexMaxDocumentBytes, Allowed: 100, Observed: &observed, MaximumAllowed: 120, SuggestedLimit: &suggested}}
	got := DiagnoseResourceLimit(Request{}, failure)
	if got == nil || got.Reason != ResourceReasonRegexMaxDocumentBytes || got.Field != ResourceFieldRegexMaxDocumentBytes || got.Allowed != 100 || got.Observed == nil || *got.Observed != 125 || got.MaximumAllowed != 120 || got.SuggestedLimit == nil || *got.SuggestedLimit != 120 {
		t.Fatalf("ASSERT_RESOURCE_DIAGNOSTIC_EXACT_CAPPED: %+v", got)
	}
	if got := cappedSuggestion(100, 125, 120); got == nil || *got != 120 {
		t.Fatalf("ASSERT_RESOURCE_SUGGESTION_CAP: %v", got)
	}
	if DiagnoseResourceLimit(Request{}, &DomainFailure{Phase: PhasePreflight, State: StateResourceLimit}) != nil {
		t.Fatal("ASSERT_GENERIC_RESOURCE_LIMIT_REMAINS_GENERIC")
	}
}

func TestRegexResourceDiagnosticMapsExactPreflightBound(t *testing.T) {
	observed := 125
	got := regexResourceDiagnostic(&regexlocator.ResourceLimit{Reason: regexlocator.ResourceLimitDocumentBytes, Allowed: 100, Observed: &observed})
	if got == nil || got.Reason != ResourceReasonRegexMaxDocumentBytes || got.Field != ResourceFieldRegexMaxDocumentBytes || got.Allowed != 100 || got.Observed == nil || *got.Observed != 125 || got.MaximumAllowed != MaxRegexDocumentBytes || got.SuggestedLimit == nil || *got.SuggestedLimit != 125 {
		t.Fatalf("ASSERT_REGEX_RESOURCE_DIAGNOSTIC: %+v", got)
	}
	if got := regexResourceDiagnostic(&regexlocator.ResourceLimit{Reason: regexlocator.ResourceLimitMatches, Allowed: 1}); got == nil || got.Observed != nil || got.SuggestedLimit != nil {
		t.Fatalf("ASSERT_REGEX_MATCHES_NO_INVENTED_CONSUMPTION: %+v", got)
	}
}

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
