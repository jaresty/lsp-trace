package transientstructural

const (
	MaxDiagnosticNodes    = 10000
	MaxRegexDocumentBytes = 16777216
	MaxRegexPatternBytes  = 4096
	MaxRegexWork          = 536870912
	MaxRegexMatches       = 1000
)

type TruncationDiagnostic struct {
	Reason             string   `json:"reason"`
	NodeLimit          int      `json:"node_limit"`
	NodesObserved      int      `json:"nodes_observed"`
	NodesAdmitted      int      `json:"nodes_admitted"`
	FrontierUnexpanded int      `json:"frontier_unexpanded"`
	Suggestions        []string `json:"suggestions"`
}

type NodeBudgetDiagnostic struct {
	Reason               string   `json:"reason"`
	Resource             string   `json:"resource"`
	Allowed              int      `json:"allowed"`
	Observed             int      `json:"observed"`
	Admitted             int      `json:"admitted"`
	FrontierUnexpanded   int      `json:"frontier_unexpanded"`
	MaximumAllowed       int      `json:"maximum_allowed"`
	SuggestedLimit       int      `json:"suggested_limit"`
	EvidenceAvailability string   `json:"evidence_availability"`
	Suggestions          []string `json:"suggestions"`
}

func DiagnoseResourceLimit(request Request, failure *DomainFailure) *ResourceDiagnostic {
	if failure == nil || (failure.State != StateResourceLimit && failure.State != StateTruncated) {
		return nil
	}
	if failure.ResourceDiagnostic != nil {
		return failure.ResourceDiagnostic
	}
	if request.MaxNodes <= 0 || !hasOmission(failure.Accounting, OmissionNodeBound) {
		return nil
	}
	observed := failure.Accounting.Nodes.Observed
	diagnostic := &ResourceDiagnostic{
		Reason: ResourceReasonNodeBound, Field: ResourceFieldMaxNodes, Allowed: request.MaxNodes,
		Observed: &observed, MaximumAllowed: MaxDiagnosticNodes,
	}
	if suggested := cappedSuggestion(request.MaxNodes, observed, MaxDiagnosticNodes); suggested != nil {
		diagnostic.SuggestedLimit = suggested
	}
	return diagnostic
}

func cappedSuggestion(allowed, observed, maximum int) *int {
	if allowed >= maximum || observed <= allowed {
		return nil
	}
	suggested := observed
	if suggested > maximum {
		suggested = maximum
	}
	return &suggested
}

func DiagnoseNodeBudget(request Request, failure *DomainFailure) *NodeBudgetDiagnostic {
	if failure == nil || (failure.State != StateTruncated && failure.State != StateResourceLimit) || request.MaxNodes <= 0 {
		return nil
	}
	nodeBound := false
	for _, omission := range failure.Accounting.Omissions {
		if omission.Reason == OmissionNodeBound && omission.Count > 0 {
			nodeBound = true
			break
		}
	}
	if !nodeBound {
		return nil
	}
	suggested := failure.Accounting.Nodes.Observed
	if suggested <= request.MaxNodes {
		suggested = request.MaxNodes + 1
	}
	if suggested > MaxDiagnosticNodes {
		suggested = MaxDiagnosticNodes
	}
	suggestions := []string{"NARROW_OUTGOING_IMPACT", "NARROW_INCOMING_IMPACT", "RESOLVE_TARGET_ONLY"}
	if request.MaxNodes < MaxDiagnosticNodes {
		suggestions = append(suggestions, "RAISE_MAX_NODES")
	}
	return &NodeBudgetDiagnostic{
		Reason:               string(OmissionNodeBound),
		Resource:             "MAX_NODES",
		Allowed:              request.MaxNodes,
		Observed:             failure.Accounting.Nodes.Observed,
		Admitted:             failure.Accounting.Nodes.Admitted,
		FrontierUnexpanded:   failure.Accounting.Frontier.Unexpanded,
		MaximumAllowed:       MaxDiagnosticNodes,
		SuggestedLimit:       suggested,
		EvidenceAvailability: "NONE",
		Suggestions:          suggestions,
	}
}

func DiagnoseTruncation(request Request, failure *DomainFailure) *TruncationDiagnostic {
	if failure == nil || failure.State != StateTruncated {
		return nil
	}
	reason := "UNKNOWN_BOUND"
	for _, omission := range failure.Accounting.Omissions {
		if omission.Count > 0 {
			reason = string(omission.Reason)
			break
		}
	}
	suggestions := []string{"NARROW_OUTGOING_IMPACT", "NARROW_INCOMING_IMPACT", "RESOLVE_TARGET_ONLY"}
	if request.MaxNodes > 0 && request.MaxNodes < MaxDiagnosticNodes {
		suggestions = append(suggestions, "RAISE_MAX_NODES")
	}
	return &TruncationDiagnostic{Reason: reason, NodeLimit: request.MaxNodes, NodesObserved: failure.Accounting.Nodes.Observed, NodesAdmitted: failure.Accounting.Nodes.Admitted, FrontierUnexpanded: failure.Accounting.Frontier.Unexpanded, Suggestions: suggestions}
}
