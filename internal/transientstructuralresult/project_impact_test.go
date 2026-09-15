package transientstructuralresult

import (
	"lsp-trace/internal/transientstructural"
	"testing"
)

func TestProjectImpactRestoresOpaqueRoot(t *testing.T) {
	q := transientstructural.Request{Generation: 1, UpDepth: 0, DownDepth: 1, MaxNodes: 10, TimeoutMS: 1000, RequestTimeoutMS: 500, MaxMessages: 8, MaxBytes: 8192, Analysis: transientstructural.AnalysisRequest{Kind: transientstructural.AnalysisImpact, Direction: transientstructural.DirectionOutgoing, MaxDepth: 1}}
	in := transientstructural.Result{TargetID: "root", Qualification: transientstructural.Qualification{Generation: 1, PositionEncoding: "utf-16"}, Accounting: transientstructural.Accounting{Requests: transientstructural.RequestAccounting{Attempted: 1, Succeeded: 1}, Preparation: transientstructural.PreparationAccounting{Attempted: 1, Returned: 1}, Nodes: transientstructural.AdmissionAccounting{Observed: 2, Admitted: 2}, Occurrences: transientstructural.AdmissionAccounting{Observed: 1, Admitted: 1}, Frontier: transientstructural.FrontierAccounting{Observed: 1, Expanded: 1}}, Analysis: transientstructural.AnalysisResult{Kind: transientstructural.AnalysisImpact, Direction: transientstructural.DirectionOutgoing, MaxDepth: 1, Nodes: []transientstructural.NodeFact{{ID: "child", Witnesses: []transientstructural.Witness{{Direction: transientstructural.DirectionOutgoing, Depth: 1}}}}, Occurrences: []transientstructural.OccurrenceFact{{ID: "edge", CallerID: "root", CalleeID: "child", Witnesses: []transientstructural.Witness{{Direction: transientstructural.DirectionOutgoing, Depth: 1}}}}}}
	if _, err := Project(in, q, "ts_0123456789abcdef0123456789abcdef"); err != nil {
		t.Fatalf("ASSERT_IMPACT_PROJECT_RESTORES_ROOT: %v", err)
	}
}
