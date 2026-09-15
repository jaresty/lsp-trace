package transientstructural

const MaxDiagnosticNodes = 10000

type TruncationDiagnostic struct {
	Reason             string   `json:"reason"`
	NodeLimit          int      `json:"node_limit"`
	NodesObserved      int      `json:"nodes_observed"`
	NodesAdmitted      int      `json:"nodes_admitted"`
	FrontierUnexpanded int      `json:"frontier_unexpanded"`
	Suggestions        []string `json:"suggestions"`
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
