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

func TestProjectDepthTwoImpactSelectsDiscoveryWitnessEdges(t *testing.T) {
	q := transientstructural.Request{Generation: 1, UpDepth: 0, DownDepth: 2, MaxNodes: 10, TimeoutMS: 1000, RequestTimeoutMS: 500, MaxMessages: 8, MaxBytes: 8192, Analysis: transientstructural.AnalysisRequest{Kind: transientstructural.AnalysisImpact, Direction: transientstructural.DirectionOutgoing, MaxDepth: 2}}
	in := transientstructural.Result{TargetID: "root", Qualification: transientstructural.Qualification{Generation: 1, PositionEncoding: "utf-16"}, Accounting: transientstructural.Accounting{Requests: transientstructural.RequestAccounting{Attempted: 1, Succeeded: 1}, Preparation: transientstructural.PreparationAccounting{Attempted: 1, Returned: 1}, Nodes: transientstructural.AdmissionAccounting{Observed: 4, Admitted: 4}, Occurrences: transientstructural.AdmissionAccounting{Observed: 5, Admitted: 5}, Frontier: transientstructural.FrontierAccounting{Observed: 3, Expanded: 3}}, Analysis: transientstructural.AnalysisResult{Kind: transientstructural.AnalysisImpact, Direction: transientstructural.DirectionOutgoing, MaxDepth: 2, Nodes: []transientstructural.NodeFact{{ID: "a"}, {ID: "b"}, {ID: "c"}}, Occurrences: []transientstructural.OccurrenceFact{{ID: "root-a", CallerID: "root", CalleeID: "a"}, {ID: "root-b", CallerID: "root", CalleeID: "b"}, {ID: "a-c", CallerID: "a", CalleeID: "c"}, {ID: "b-c", CallerID: "b", CalleeID: "c"}, {ID: "a-b", CallerID: "a", CalleeID: "b"}}}}
	out, err := Project(in, q, "ts_0123456789abcdef0123456789abcdef")
	if err != nil {
		t.Fatalf("ASSERT_DEPTH_TWO_IMPACT_PROJECTS_VALID_GRAPH: %v", err)
	}
	impact := out.Analysis.(ImpactResult)
	if len(impact.Edges) != 5 || len(impact.WitnessEdgeIDs) != 4 {
		t.Fatalf("ASSERT_DEPTH_TWO_IMPACT_SEPARATES_GRAPH_AND_DISCOVERY_EDGES: edges=%d witnesses=%d", len(impact.Edges), len(impact.WitnessEdgeIDs))
	}
}
