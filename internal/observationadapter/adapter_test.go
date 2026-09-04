package observationadapter

import (
	"reflect"
	"strings"
	"testing"

	"lsp-trace/internal/graph"
	"lsp-trace/internal/relations"
)

func validEnvelope() Envelope {
	return Envelope{
		Provider:  Identity{Name: "ember-template-compiler", Version: "7.2.0"},
		Protocol:  Identity{Name: ProtocolName, Version: ProtocolVersion},
		Adapter:   Identity{Name: "ember-template-observations", Version: "1.0.0"},
		Authority: AuthorityProviderReported,
		Coverage:  relations.Coverage{Status: relations.CoveragePartial, Denominator: []string{"component.gts"}, Covered: []string{"component.gts"}, CoveredCount: 1},
		Documents: []graph.SourceDocumentRecord{{
			DocumentID: "original", OriginalURI: "file:///workspace/component.gts",
			VirtualURI: "file:///virtual/component.ts", CoordinateURI: "file:///virtual/component.ts",
			ContentSHA256: strings.Repeat("a", 64), Revision: graph.RevisionIdentity{Kind: "git", Value: strings.Repeat("b", 40), Blob: strings.Repeat("c", 40), Custody: graph.CustodyProviderProved},
			Mapping: &graph.VirtualDocumentMapping{MappingID: "map-1", OriginalDocumentID: "original"}, Coordinates: graph.CoordinatesGenerated,
		}},
		Observations: []ReportedObservation{{
			Kind:           relations.RelationPassesCallback,
			From:           relations.Endpoint{NodeID: "component:submit", Role: relations.RoleCallableReference},
			To:             relations.Endpoint{NodeID: "helper:on-submit", Role: relations.RoleCallbackParameter},
			OriginalAnchor: Anchor{DocumentID: "original", URI: "file:///workspace/component.gts", Revision: strings.Repeat("b", 40), Blob: strings.Repeat("c", 40), Range: graph.Range{Start: graph.Position{Line: 2, Character: 4}, End: graph.Position{Line: 2, Character: 18}}},
			VirtualAnchor:  &Anchor{DocumentID: "original", URI: "file:///virtual/component.ts", Revision: strings.Repeat("b", 40), Blob: strings.Repeat("c", 40), MappingID: "map-1", Range: graph.Range{Start: graph.Position{Line: 8, Character: 1}, End: graph.Position{Line: 8, Character: 9}}},
			Supports:       []relations.Claim{relations.ClaimSourceDependencyRelation},
			DoesNotSupport: append([]relations.Claim(nil), relations.ProhibitedClaims...),
		}},
	}
}

func TestIdentityValidationFailsClosed(t *testing.T) {
	for name, mutate := range map[string]func(*Envelope){
		"provider": func(e *Envelope) { e.Provider.Version = "" },
		"protocol": func(e *Envelope) { e.Protocol.Version = "v999" },
		"adapter":  func(e *Envelope) { e.Adapter.Name = "" },
	} {
		t.Run(name, func(t *testing.T) {
			e := validEnvelope()
			mutate(&e)
			if _, err := Adapt(e); err == nil {
				t.Fatalf("ASSERT_ADAPTER_IDENTITY_FAILS_CLOSED_%s: accepted mismatch", strings.ToUpper(name))
			}
		})
	}
}

func TestProviderReportedAuthorityAndDeterministicObservationID(t *testing.T) {
	e := validEnvelope()
	r1, err := Adapt(e)
	if err != nil {
		t.Fatal(err)
	}
	e.RequestID, e.Timestamp = "volatile", "tomorrow"
	r2, err := Adapt(e)
	if err != nil {
		t.Fatal(err)
	}
	if got := r1.Observations[0].Authority; got != AuthorityProviderReported {
		t.Fatalf("ASSERT_PROVIDER_REPORTED_AUTHORITY: got %q", got)
	}
	if r1.Observations[0].ObservationID == "" || r1.Observations[0].ObservationID != r2.Observations[0].ObservationID {
		t.Fatalf("ASSERT_DETERMINISTIC_OBSERVATION_ID: %q != %q", r1.Observations[0].ObservationID, r2.Observations[0].ObservationID)
	}
}

func TestDocumentCustodyRetainsOriginalAndVirtualAnchors(t *testing.T) {
	r, err := Adapt(validEnvelope())
	if err != nil {
		t.Fatal(err)
	}
	o := r.Observations[0]
	if o.OriginalAnchor.URI != "file:///workspace/component.gts" || o.VirtualAnchor == nil || o.VirtualAnchor.URI != "file:///virtual/component.ts" || o.VirtualAnchor.MappingID != "map-1" {
		t.Fatalf("ASSERT_ORIGINAL_VIRTUAL_ANCHOR_CUSTODY: %#v", o)
	}
	e := validEnvelope()
	e.Observations[0].VirtualAnchor.MappingID = ""
	if _, err := Adapt(e); err == nil {
		t.Fatal("ASSERT_SYNTHETIC_VIRTUAL_PROVENANCE_REJECTED: accepted missing mapping")
	}
}

func TestCoverageAndFailureRemainDistinct(t *testing.T) {
	e := validEnvelope()
	e.Failure = relations.FailureTransportFailed
	r, err := Adapt(e)
	if err != nil {
		t.Fatal(err)
	}
	if r.Coverage.Status != relations.CoveragePartial || r.Failure != relations.FailureTransportFailed {
		t.Fatalf("ASSERT_COVERAGE_FAILURE_DISTINCT: %#v", r)
	}
	e = validEnvelope()
	e.Failure = relations.FailureKind("EMPTY")
	if _, err := Adapt(e); err == nil {
		t.Fatal("ASSERT_FAILURE_VOCABULARY_CLOSED: accepted EMPTY")
	}
}

func TestManyObservationGraphV4DeterminismAndProvenance(t *testing.T) {
	e := validEnvelope()
	second := e.Observations[0]
	second.OriginalAnchor.Range.Start.Character = 5
	second.VirtualAnchor = nil
	e.Observations = append(e.Observations, second)
	r1, err := Adapt(e)
	if err != nil {
		t.Fatal(err)
	}
	e.Observations[0], e.Observations[1] = e.Observations[1], e.Observations[0]
	r2, err := Adapt(e)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(r1.GraphV4, r2.GraphV4) {
		t.Fatalf("ASSERT_GRAPH_V4_ORDER_INDEPENDENT: %#v != %#v", r1.GraphV4, r2.GraphV4)
	}
	rel := r1.GraphV4.Relations[0]
	if len(rel.ContributingObservationIDs) != 2 {
		t.Fatalf("ASSERT_GRAPH_V4_ALL_CONTRIBUTORS: got %v", rel.ContributingObservationIDs)
	}
	if rel.Adapter == nil || rel.Adapter.Name != e.Adapter.Name || rel.Adapter.Version != e.Adapter.Version {
		t.Fatalf("ASSERT_GRAPH_V4_ADAPTER_IDENTITY: %#v", rel.Adapter)
	}
}

func TestNonEntailmentsAndNoFrameworkParsing(t *testing.T) {
	e := validEnvelope()
	e.Observations[0].Kind = relations.RelationCalls
	if _, err := Adapt(e); err == nil {
		t.Fatal("ASSERT_NON_CALLS_PROVIDER_OBSERVATION_NEVER_PROMOTED_TO_CALLS: accepted CALLS")
	}
	e = validEnvelope()
	e.Observations[0].Supports = append(e.Observations[0].Supports, relations.ClaimCallbackInvocation)
	if _, err := Adapt(e); err == nil {
		t.Fatal("ASSERT_PASSAGE_NEVER_ENTAILS_INVOCATION: accepted callback invocation")
	}
	e = validEnvelope()
	e.Source = "<template>{{this.secret}}</template>"
	r, err := Adapt(e)
	if err != nil {
		t.Fatal(err)
	}
	if r.Observations[0].OriginalAnchor != e.Observations[0].OriginalAnchor {
		t.Fatal("ASSERT_EMBER_GLIMMER_SOURCE_IS_OPAQUE: source changed reported anchor")
	}
}
