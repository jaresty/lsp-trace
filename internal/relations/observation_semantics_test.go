package relations

import (
	"strings"
	"testing"
)

func validObservation() SemanticObservation {
	return SemanticObservation{
		SchemaVersion:  ObservationSchemaV1,
		Kind:           RelationBindsArgument,
		Authority:      AuthoritySourceDerivedAdapter,
		Provider:       ProducerIdentity{Name: "ember-template-relations", Version: "1.0.0"},
		From:           Endpoint{NodeID: "source", Role: RoleSourceExpression},
		To:             Endpoint{NodeID: "argument", Role: RoleBoundArgument},
		Anchors:        []SourceAnchor{{URI: "file:///workspace/app.gjs", Revision: "abc", Blob: "def", StartLine: 7, StartCharacter: 4, EndLine: 7, EndCharacter: 73}},
		Supports:       []Claim{ClaimSourceDependencyRelation},
		DoesNotSupport: append([]Claim(nil), ProhibitedClaims...),
	}
}

func requireInvalid(t *testing.T, assertion string, observation SemanticObservation) {
	t.Helper()
	if err := observation.Validate(); err == nil {
		t.Fatalf("%s: accepted invalid observation", assertion)
	}
}

func TestObservationSemanticContract(t *testing.T) {
	t.Run("ASSERT_CLOSED_RELATION_VOCABULARY", func(t *testing.T) {
		kinds := []RelationKind{RelationCalls, RelationBindsArgument, RelationPassesCallback, RelationInvokesTask, RelationTriggersReload, RelationUpdatesState, RelationRendersFrom}
		for _, kind := range kinds {
			if !kind.Valid() {
				t.Fatalf("ASSERT_CLOSED_RELATION_VOCABULARY: rejected required kind %s", kind)
			}
		}
		o := validObservation()
		o.Kind = RelationKind("UNKNOWN")
		requireInvalid(t, "ASSERT_CLOSED_RELATION_VOCABULARY", o)
	})
	t.Run("ASSERT_AUTHORITY_AND_PRODUCER", func(t *testing.T) {
		o := validObservation()
		o.Kind, o.Authority = RelationCalls, AuthorityCallerAsserted
		requireInvalid(t, "ASSERT_AUTHORITY_AND_PRODUCER", o)
		o = validObservation()
		o.Provider = ProducerIdentity{}
		requireInvalid(t, "ASSERT_AUTHORITY_AND_PRODUCER", o)
		o = validObservation()
		o.Anchors[0].Revision = ""
		requireInvalid(t, "ASSERT_AUTHORITY_AND_PRODUCER", o)
	})
	t.Run("ASSERT_ENDPOINTS_AND_NON_ENTAILMENTS", func(t *testing.T) {
		o := validObservation()
		o.Kind = RelationPassesCallback
		o.From.Role, o.To.Role = RoleCallableReference, RoleCallbackParameter
		o.Supports = append(o.Supports, ClaimCallbackInvocation)
		requireInvalid(t, "ASSERT_ENDPOINTS_AND_NON_ENTAILMENTS", o)
	})
	t.Run("ASSERT_COVERAGE_DENOMINATOR", func(t *testing.T) {
		coverage := Coverage{Status: CoverageCompleteWithinBounds, Denominator: []string{"a"}, Covered: []string{"a", "b"}, CoveredCount: 2}
		if err := coverage.Validate(); err == nil {
			t.Fatal("ASSERT_COVERAGE_DENOMINATOR: accepted coverage outside denominator")
		}
	})
	t.Run("ASSERT_DISTINCT_FAILURE_TAXONOMY", func(t *testing.T) {
		for _, outcome := range []FailureKind{FailurePrepareReturnedNoItem, FailureRelationNotSupported, FailureAdapterNotAvailable, FailureNoRelationsWithinBounds, FailureTransportFailed, FailureTruncated} {
			if !outcome.Valid() {
				t.Fatalf("ASSERT_DISTINCT_FAILURE_TAXONOMY: rejected %s", outcome)
			}
		}
		if FailureKind("EMPTY").Valid() {
			t.Fatal("ASSERT_DISTINCT_FAILURE_TAXONOMY: accepted conflated EMPTY outcome")
		}
	})
	t.Run("ASSERT_DETERMINISTIC_IDENTITY", func(t *testing.T) {
		a := validObservation()
		b := validObservation()
		b.Anchors = append([]SourceAnchor{{URI: "file:///workspace/z.gjs", Revision: "abc", Blob: "ghi", StartLine: 1, EndLine: 1}}, b.Anchors...)
		a.Anchors = append(a.Anchors, b.Anchors[0])
		a.RequestID, b.RequestID = "request-a", "request-b"
		a.Timestamp, b.Timestamp = "now", "later"
		ida, err := a.CanonicalID()
		if err != nil {
			t.Fatal(err)
		}
		idb, err := b.CanonicalID()
		if err != nil {
			t.Fatal(err)
		}
		if ida != idb || !strings.HasPrefix(ida, "sha256:") {
			t.Fatalf("ASSERT_DETERMINISTIC_IDENTITY: ids differ %q %q", ida, idb)
		}
	})
	t.Run("ASSERT_SCHEMA_EVOLUTION_FAILS_CLOSED", func(t *testing.T) {
		o := validObservation()
		o.SchemaVersion = "lsp-trace.observation.v2"
		requireInvalid(t, "ASSERT_SCHEMA_EVOLUTION_FAILS_CLOSED", o)
	})
	t.Run("ASSERT_MULTI_OBSERVATION_PROVENANCE", func(t *testing.T) {
		ids, err := CanonicalContributingObservationIDs([]string{"obs-b", "obs-a"})
		if err != nil {
			t.Fatal(err)
		}
		if strings.Join(ids, ",") != "obs-a,obs-b" {
			t.Fatalf("ASSERT_MULTI_OBSERVATION_PROVENANCE: got %v", ids)
		}
		if _, err := CanonicalContributingObservationIDs([]string{"obs-a", "obs-a"}); err == nil {
			t.Fatal("ASSERT_MULTI_OBSERVATION_PROVENANCE: accepted duplicate provenance")
		}
		if _, err := CanonicalContributingObservationIDs(nil); err == nil {
			t.Fatal("ASSERT_MULTI_OBSERVATION_PROVENANCE: accepted empty provenance")
		}
	})
}
