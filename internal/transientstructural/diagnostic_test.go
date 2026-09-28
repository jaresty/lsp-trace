package transientstructural

import (
	"testing"

	"lsp-trace/incomingops"
	"lsp-trace/internal/lsp"
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
	matchObserved := 2
	match := regexResourceDiagnostic(&regexlocator.ResourceLimit{Reason: regexlocator.ResourceLimitMatches, Allowed: 1, Observed: &matchObserved})
	if match == nil || match.Field != ResourceFieldRegexMaxMatches || match.Allowed != 1 || match.Observed == nil || *match.Observed != 2 || match.MaximumAllowed != MaxRegexMatches || match.SuggestedLimit == nil || *match.SuggestedLimit != 2 || len(match.SuggestedLimits) != 1 || match.SuggestedLimits[0] != (SuggestedLimit{Field: ResourceFieldRegexMaxMatches, Current: 1, Observed: 2, Suggested: 2}) || match.CallerAction != "REQUIRED" || match.RequestFragment == nil || match.RequestFragment.RegexLocator == nil || match.RequestFragment.RegexLocator.Limits.MaxMatches != 2 {
		t.Fatalf("ASSERT_REGEX_MAX_MATCHES_COMPLETE_RESOURCE_DIAGNOSTIC: %+v", match)
	}

	workObserved := MaxRegexWork + 1
	work := regexResourceDiagnostic(&regexlocator.ResourceLimit{Reason: regexlocator.ResourceLimitWork, Allowed: 64, Observed: &workObserved})
	if work == nil || len(work.SuggestedLimits) != 1 || work.SuggestedLimits[0].Field != ResourceFieldRegexMaxWork || work.SuggestedLimits[0].Current != 64 || work.SuggestedLimits[0].Observed != workObserved || work.SuggestedLimits[0].Suggested != MaxRegexWork || work.CallerAction != "REQUIRED" || work.RequestFragment == nil || work.RequestFragment.RegexLocator == nil || work.RequestFragment.RegexLocator.Limits.MaxWork != MaxRegexWork {
		t.Fatalf("ASSERT_REGEX_MAX_WORK_SUGGESTED_LIMITS_FRAGMENT: %+v", work)
	}
}

func TestPreparedTargetDiagnosticReportsBoundedContainmentCandidates(t *testing.T) {
	prepared := incomingops.PreparedTarget{
		TotalSymbols: 14, OmittedSymbols: 6, Action: "ENUMERATION_TRUNCATED",
		ProviderMethod: "textDocument/documentSymbol", NormalizationStage: "EXACT_MATCH",
		FailedField: "range", FailedInvariant: "POSITION_CONTAINMENT_PRESENT",
		LocatorScope: "URI_POSITION", Guidance: "no callable document symbol contains the requested position; candidate kinds 6, 9, and 12 are callable",
		Candidates: []incomingops.PreparedTargetCandidate{{
			URI: "file:///workspace/FeatureAuthorizeAttribute.cs", Name: "FeatureAuthorizeAttribute", Kind: 5,
			Range: lsp.Range{Start: lsp.Position{Line: 8, Character: 1}, End: lsp.Position{Line: 20, Character: 2}},
		}},
	}
	got := preparedTargetDiagnostic(prepared)
	if got == nil || len(got.Candidates) != 1 || got.Candidates[0].Name != "FeatureAuthorizeAttribute" || got.Candidates[0].Kind != 5 || got.Candidates[0].Range.Start.Line != 8 || got.CandidateAccounting == nil || got.CandidateAccounting.Observed == nil || *got.CandidateAccounting.Observed != 14 || got.CandidateAccounting.Returned != 1 || got.CandidateAccounting.Excluded != 13 || got.CandidateAccounting.Truncated != 6 || got.LocatorScope != "URI_POSITION" || got.Guidance == "" || got.Completeness != "UNKNOWN" {
		t.Fatalf("ASSERT_POSITION_CONTAINMENT_DIAGNOSTIC_CANDIDATES: %+v", got)
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
