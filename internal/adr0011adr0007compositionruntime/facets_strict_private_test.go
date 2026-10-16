package adr0011adr0007compositionruntime

import (
	"context"
	"errors"
	"strings"
	"testing"

	"lsp-trace/internal/directionalraw"
)

var strictFacetNamesPrivate = []string{"D", "R", "CALLS_INCOMING", "CALLS_OUTGOING"}
var strictFacetStatesPrivate = []string{"OBSERVED_NONEMPTY", "OBSERVED_EMPTY", "UNAVAILABLE"}

func completeFacetsPrivate(state string) []FacetObservationPrivate {
	out := make([]FacetObservationPrivate, len(strictFacetNamesPrivate))
	for i, facet := range strictFacetNamesPrivate {
		out[i] = FacetObservationPrivate{Facet: facet, ObservationState: state}
	}
	return out
}

func malformedFacetCasesPrivate() []struct {
	name string
	in   []FacetObservationPrivate
} {
	var cases []struct {
		name string
		in   []FacetObservationPrivate
	}
	for _, state := range strictFacetStatesPrivate {
		in := completeFacetsPrivate("OBSERVED_EMPTY")
		in[0] = FacetObservationPrivate{Facet: "UNKNOWN", ObservationState: state}
		cases = append(cases, struct {
			name string
			in   []FacetObservationPrivate
		}{"unknown-facet-" + state, in})
	}
	for i, facet := range strictFacetNamesPrivate {
		in := completeFacetsPrivate("OBSERVED_EMPTY")
		in[i].ObservationState = "UNKNOWN"
		cases = append(cases, struct {
			name string
			in   []FacetObservationPrivate
		}{"unknown-state-" + facet, in})
	}
	for i, facet := range strictFacetNamesPrivate {
		in := completeFacetsPrivate("OBSERVED_EMPTY")
		in[(i+1)%len(in)].Facet = facet
		cases = append(cases, struct {
			name string
			in   []FacetObservationPrivate
		}{"duplicate-" + facet, in})
	}
	for missing, facet := range strictFacetNamesPrivate {
		all := completeFacetsPrivate("OBSERVED_EMPTY")
		in := append([]FacetObservationPrivate(nil), all[:missing]...)
		in = append(in, all[missing+1:]...)
		cases = append(cases, struct {
			name string
			in   []FacetObservationPrivate
		}{"missing-" + facet, in})
	}
	return cases
}

func TestValidateFacetsPrivateRejectsMalformedExhaustively(t *testing.T) {
	cases := malformedFacetCasesPrivate()
	if len(cases) != 15 {
		t.Fatalf("ASSERT_STRICT_INVALID_MATRIX_CARDINALITY: got=%d", len(cases))
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got, err := ValidateFacetsPrivate(tc.in); err == nil || got != nil {
				t.Fatalf("ASSERT_STRICT_MALFORMED_REJECTED_BEFORE_COMPOSITION: got=%+v err=%v", got, err)
			}

			reservation := directionalraw.NewReservation("strict-invalid-" + tc.name)
			terminalCalls := 0
			if !reservation.AttachPrivate(func(string, string, int) ([]byte, error) { return nil, nil }, func(directionalraw.Observation) {}, func() bool {
				terminalCalls++
				return true
			}, func() {}) {
				t.Fatal("ASSERT_STRICT_INVALID_ATTACH")
			}
			root, err := directionalraw.NewBidirectionalRootPrivate(reservation, directionalraw.BidirectionalPlanPrivate{})
			if err != nil {
				t.Fatal(err)
			}
			child, _ := root.ChildPrivate("incoming")
			if err := child.ReservePrivate(7); err != nil {
				t.Fatal(err)
			}
			result, err := CoordinatePrivate(context.Background(), CoordinateRequestPrivate{Root: root, Facets: tc.in})
			if err == nil || result.AcceptedPrivate() || result.CandidatePrivate() != nil || result.CompositionPrivate().CompositionCandidate != nil {
				t.Fatalf("ASSERT_STRICT_MALFORMED_NEVER_ACCEPTS: result=%+v err=%v", result, err)
			}
			if terminalCalls != 1 || !result.LeaseStatePrivate().Released || result.LeaseStatePrivate().RootReleaseCount != 1 || result.LeaseStatePrivate().RootLive != 0 || result.LeaseStatePrivate().RootReleased != 7 {
				t.Fatalf("ASSERT_STRICT_INVALID_ORDINARY_CLEANUP_EXACT_ONCE_LIVE_ZERO: calls=%d lease=%+v", terminalCalls, result.LeaseStatePrivate())
			}
		})
	}
}

func TestValidateFacetsPrivateMalformedTerminalReleaseFailureTruth(t *testing.T) {
	reservation := directionalraw.NewReservation("strict-invalid-terminal-failure")
	terminalCalls := 0
	if !reservation.AttachPrivate(func(string, string, int) ([]byte, error) { return nil, nil }, func(directionalraw.Observation) {}, func() bool {
		terminalCalls++
		return false
	}, func() {}) {
		t.Fatal("ASSERT_STRICT_TERMINAL_FAILURE_ATTACH")
	}
	root, err := directionalraw.NewBidirectionalRootPrivate(reservation, directionalraw.BidirectionalPlanPrivate{})
	if err != nil {
		t.Fatal(err)
	}
	child, _ := root.ChildPrivate("incoming")
	if err := child.ReservePrivate(11); err != nil {
		t.Fatal(err)
	}
	malformed := completeFacetsPrivate("OBSERVED_EMPTY")
	malformed[0].Facet = "UNKNOWN"
	result, err := CoordinatePrivate(context.Background(), CoordinateRequestPrivate{Root: root, Facets: malformed})
	if !errors.Is(err, directionalraw.ErrTerminalReleasePrivate) || err == nil || !strings.Contains(err.Error(), "unknown facet") {
		t.Fatalf("ASSERT_STRICT_RELEASE_FAILURE_JOINS_VALIDATION_TRUTH: %v", err)
	}
	if result.AcceptedPrivate() || result.CandidatePrivate() != nil || result.CompositionPrivate().CompositionCandidate != nil || result.SuppressionPrivate() == nil || result.SuppressionPrivate().Code != "TERMINAL_RELEASE_FAILED" {
		t.Fatalf("ASSERT_STRICT_RELEASE_FAILURE_NON_ACCEPTED: %+v", result)
	}
	if terminalCalls != 1 || result.LeaseStatePrivate().Released || result.LeaseStatePrivate().RootReleaseCount != 0 || result.LeaseStatePrivate().RootLive != 11 || result.LeaseStatePrivate().RootReleased != 0 {
		t.Fatalf("ASSERT_STRICT_RELEASE_FAILURE_TRUTHFUL_LIVE_ACCOUNTING: calls=%d lease=%+v", terminalCalls, result.LeaseStatePrivate())
	}
}
