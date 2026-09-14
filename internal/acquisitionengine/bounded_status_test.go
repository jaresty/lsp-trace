package acquisitionengine

import (
	"testing"

	"lsp-trace/internal/acquisition"
	"lsp-trace/internal/graph"
)

func completeBoundedFixture() acquisition.Result {
	return acquisition.Result{
		Graph:    graph.Result{Summary: graph.Summary{Truncated: false}},
		Requests: []acquisition.RequestRecord{{Outcome: "SUCCESS"}},
		Targets: []acquisition.TargetResult{{
			Resolution: acquisition.Resolution{Status: acquisition.Resolved},
			Admission:  acquisition.Admitted,
			Outgoing:   acquisition.DirectionResult{Status: acquisition.Partial, Expansions: []acquisition.Expansion{{Status: acquisition.SuccessNonempty}, {Status: acquisition.Frontier}}},
			Incoming:   acquisition.DirectionResult{Status: acquisition.Frontier, Expansions: []acquisition.Expansion{{Status: acquisition.Frontier}}},
		}},
	}
}

func TestBoundedTraversalCompleteAcceptsRequestedFrontierOnly(t *testing.T) {
	if !boundedTraversalComplete(completeBoundedFixture()) {
		t.Fatal("ASSERT_EXPECTED_BOUNDED_FRONTIER_COMPLETE: FAIL")
	}
	t.Log("ASSERT_EXPECTED_BOUNDED_FRONTIER_COMPLETE: PASS")
}

func TestBoundedTraversalCompleteFailsClosed(t *testing.T) {
	cases := map[string]func(*acquisition.Result){
		"request":   func(r *acquisition.Result) { r.Requests[0].Outcome = "FAILED" },
		"expansion": func(r *acquisition.Result) { r.Targets[0].Outgoing.Expansions[0].Status = acquisition.Partial },
		"direction": func(r *acquisition.Result) { r.Targets[0].Incoming.Status = acquisition.BudgetBlocked },
		"bare-partial": func(r *acquisition.Result) {
			r.Targets[0].Incoming.Status = acquisition.Partial
			r.Targets[0].Incoming.Expansions = nil
		},
		"resolution": func(r *acquisition.Result) { r.Targets[0].Resolution.Status = acquisition.Missing },
		"admission":  func(r *acquisition.Result) { r.Targets[0].Admission = acquisition.AdmissionBlocked },
		"truncation": func(r *acquisition.Result) { r.Graph.Summary.Truncated = true },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			result := completeBoundedFixture()
			mutate(&result)
			if boundedTraversalComplete(result) {
				t.Fatal("ASSERT_BOUNDED_FAILURE_REJECTED: FAIL")
			}
			t.Log("ASSERT_BOUNDED_FAILURE_REJECTED: PASS")
		})
	}
}
