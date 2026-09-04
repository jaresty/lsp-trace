package graph

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func normalizedRelationsFixture() Result {
	caller := NewNode(Item{Name: "caller", Kind: 12, URI: "file:///caller.go", Range: Range{End: Position{Line: 2}}, SelectionRange: Range{End: Position{Character: 6}}})
	callee := NewNode(Item{Name: "callee", Kind: 12, URI: "file:///callee.go", Range: Range{End: Position{Line: 2}}, SelectionRange: Range{End: Position{Character: 6}}})
	return Result{Nodes: []Node{caller, callee}, Edges: []Edge{{
		RelationID: "relation-1", CallerNodeID: caller.ID, CalleeNodeID: callee.ID,
		CallSites: []Range{{Start: Position{Line: 1, Character: 2}, End: Position{Line: 1, Character: 3}}},
	}}}
}

func TestNormalizedRelationsArtifactKind(t *testing.T) {
	artifact := NormalizeRelations(normalizedRelationsFixture())
	if artifact.ArtifactKind != "NORMALIZED_RELATIONS" {
		t.Fatalf("ASSERT_NORMALIZED_RELATIONS_ARTIFACT_KIND: got %q", artifact.ArtifactKind)
	}
}

func TestNormalizedRelationsSchemaVersion(t *testing.T) {
	artifact := NormalizeRelations(normalizedRelationsFixture())
	if artifact.SchemaVersion != "lsp-trace.graph.v4" {
		t.Fatalf("ASSERT_NORMALIZED_RELATIONS_SCHEMA_VERSION: got %q", artifact.SchemaVersion)
	}
}

func TestNormalizedRelationsProjectsCanonicalCALLS(t *testing.T) {
	got := NormalizeRelations(normalizedRelationsFixture()).Relations
	if len(got) != 1 || got[0].Kind != RelationCalls || got[0].EvidenceClass != EvidenceServerReported {
		t.Fatalf("ASSERT_NORMALIZED_RELATIONS_CANONICAL_CALLS: got %#v", got)
	}
	if !bytes.Equal(mustJSON(t, got[0]), mustJSON(t, NewRelation(got[0]))) {
		t.Fatalf("ASSERT_NORMALIZED_RELATIONS_CANONICAL_RECORDS: relation was not canonical")
	}
}

func TestNormalizedRelationsSchemaBoundary(t *testing.T) {
	schema := NormalizedRelationsSchemaJSON()
	if !bytes.Contains(schema, []byte(`"$schema":"https://json-schema.org/draft/2020-12/schema"`)) ||
		!bytes.Contains(schema, []byte(`"const":"lsp-trace.graph.v4"`)) {
		t.Fatalf("ASSERT_NORMALIZED_RELATIONS_EMBEDDED_SCHEMA: %s", schema)
	}
	valid := mustJSON(t, NormalizeRelations(normalizedRelationsFixture()))
	if err := ValidateNormalizedRelationsJSON(valid); err != nil {
		t.Fatalf("ASSERT_NORMALIZED_RELATIONS_VALID_DOCUMENT: %v", err)
	}
	var malformed map[string]any
	if err := json.Unmarshal(valid, &malformed); err != nil {
		t.Fatal(err)
	}
	delete(malformed, "relations")
	if err := ValidateNormalizedRelationsJSON(mustJSON(t, malformed)); err == nil {
		t.Fatal("ASSERT_NORMALIZED_RELATIONS_REJECTS_MISSING_RELATIONS: accepted malformed artifact")
	}
}

func TestNormalizedRelationsRejectsMissingRequiredProvenanceFields(t *testing.T) {
	valid := mustJSON(t, NormalizeRelations(normalizedRelationsFixture()))
	for _, field := range []string{"kind", "from", "to", "evidence_class", "anchors", "confidence", "supports", "does_not_support", "contributing_observation_ids"} {
		t.Run(field, func(t *testing.T) {
			var document map[string]any
			if err := json.Unmarshal(valid, &document); err != nil {
				t.Fatal(err)
			}
			relations := document["relations"].([]any)
			delete(relations[0].(map[string]any), field)
			if err := ValidateNormalizedRelationsJSON(mustJSON(t, document)); err == nil {
				t.Fatalf("ASSERT_NORMALIZED_RELATIONS_REJECTS_MISSING_%s: accepted", field)
			}
		})
	}
}

func normalizedRelationJSON(t *testing.T) map[string]any {
	t.Helper()
	var document map[string]any
	if err := json.Unmarshal(mustJSON(t, NormalizeRelations(normalizedRelationsFixture())), &document); err != nil {
		t.Fatal(err)
	}
	return document["relations"].([]any)[0].(map[string]any)
}

func TestGraphV4RelationRequiresProvenance(t *testing.T) {
	relation := normalizedRelationJSON(t)
	for _, field := range []string{"kind", "from", "to", "evidence_class", "anchors", "confidence", "supports", "does_not_support", "contributing_observation_ids"} {
		if _, ok := relation[field]; !ok {
			t.Fatalf("ASSERT_GRAPH_V4_RELATION_REQUIRES_PROVENANCE: missing %s", field)
		}
	}
}

func TestGraphV4RelationIDSHA256(t *testing.T) {
	got, _ := normalizedRelationJSON(t)["relation_id"].(string)
	if len(got) != len("sha256:")+64 || got[:len("sha256:")] != "sha256:" {
		t.Fatalf("ASSERT_GRAPH_V4_RELATION_ID_SHA256: got %q", got)
	}
}

func TestGraphV4CALLSServerReported(t *testing.T) {
	relation := normalizedRelationJSON(t)
	if relation["kind"] != "CALLS" || relation["evidence_class"] != "SERVER_REPORTED" {
		t.Fatalf("ASSERT_GRAPH_V4_CALLS_SERVER_REPORTED: kind=%v evidence_class=%v", relation["kind"], relation["evidence_class"])
	}
}

func TestGraphV4NonEntailments(t *testing.T) {
	relation := normalizedRelationJSON(t)
	values, _ := relation["does_not_support"].([]any)
	seen := map[string]bool{}
	for _, value := range values {
		seen[value.(string)] = true
	}
	for _, claim := range []string{"runtime_execution", "whole_source_completeness"} {
		if !seen[claim] {
			t.Fatalf("ASSERT_GRAPH_V4_NON_ENTAILMENTS: missing %s", claim)
		}
	}
}

func sourceRelationFixture() Relation {
	return NewRelation(Relation{
		Kind: RelationPassesCallback, From: "node-a", To: "node-b", EvidenceClass: EvidenceSourceAdapter,
		Adapter: &RelationAdapter{Name: "ember-template-relations", Version: "1.0.0"},
		Anchors: []RelationAnchor{
			{URI: "file:///workspace/b.gjs", Revision: "rev", Blob: "blob-b", Range: Range{Start: Position{Line: 2}, End: Position{Line: 2, Character: 4}}},
			{URI: "file:///workspace/a.gjs", Revision: "rev", Blob: "blob-a", Range: Range{Start: Position{Line: 1}, End: Position{Line: 1, Character: 4}}},
		},
		Confidence: "EXACT", Supports: []string{"source_dependency_relation"},
		DoesNotSupport:             []string{"whole_source_completeness", "callback_invocation", "runtime_execution"},
		ContributingObservationIDs: []string{"observation-b", "observation-a"},
	})
}

func TestGraphV4RelationIdentityIgnoresInputOrder(t *testing.T) {
	first := sourceRelationFixture()
	second := first
	second.RelationID = ""
	second.Anchors = []RelationAnchor{first.Anchors[1], first.Anchors[0]}
	second.ContributingObservationIDs = []string{first.ContributingObservationIDs[1], first.ContributingObservationIDs[0]}
	second = NewRelation(second)
	if first.RelationID != second.RelationID {
		t.Fatalf("ASSERT_GRAPH_V4_RELATION_ID_ORDER_INDEPENDENT: %q != %q", first.RelationID, second.RelationID)
	}
}

func TestGraphV4ValidationRejectsSemanticOverclaims(t *testing.T) {
	cases := map[string]func(*Relation){
		"callback_invocation":       func(r *Relation) { r.DoesNotSupport = []string{"runtime_execution", "whole_source_completeness"} },
		"adapter_identity":          func(r *Relation) { r.Adapter = nil },
		"runtime_execution":         func(r *Relation) { r.DoesNotSupport = []string{"callback_invocation", "whole_source_completeness"} },
		"whole_source_completeness": func(r *Relation) { r.DoesNotSupport = []string{"callback_invocation", "runtime_execution"} },
		"deterministic_id":          func(r *Relation) { r.RelationID = "sha256:" + strings.Repeat("0", 64) },
		"duplicate_observation": func(r *Relation) {
			r.ContributingObservationIDs = []string{"observation-a", "observation-a"}
			r.RelationID = relationIdentity(*r)
		},
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			relation := sourceRelationFixture()
			mutate(&relation)
			artifact := NormalizedRelations{SchemaVersion: NormalizedRelationsSchemaVersion, ArtifactKind: NormalizedRelationsArtifactKind, Relations: []Relation{relation}}
			if err := ValidateNormalizedRelationsJSON(mustJSON(t, artifact)); err == nil {
				t.Fatalf("ASSERT_GRAPH_V4_REJECTS_%s: accepted semantic overclaim", strings.ToUpper(name))
			}
		})
	}
}

func TestGraphV4ValidationAcceptsSourceDerivedRelation(t *testing.T) {
	artifact := NormalizedRelations{SchemaVersion: NormalizedRelationsSchemaVersion, ArtifactKind: NormalizedRelationsArtifactKind, Relations: []Relation{sourceRelationFixture()}}
	if err := ValidateNormalizedRelationsJSON(mustJSON(t, artifact)); err != nil {
		t.Fatalf("ASSERT_GRAPH_V4_ACCEPTS_PROVENANCE_PRESERVING_RELATION: %v", err)
	}
}

func TestNormalizedRelationsDoesNotMutateGraphSerialization(t *testing.T) {
	for _, version := range []string{SchemaVersionV2, SchemaVersionV3} {
		result := normalizedRelationsFixture()
		result.SchemaVersion = version
		before := mustJSON(t, result)
		_ = NormalizeRelations(result)
		after := mustJSON(t, result)
		if !bytes.Equal(before, after) {
			t.Fatalf("ASSERT_NORMALIZED_RELATIONS_GRAPH_BYTES_UNCHANGED_%s", version)
		}
	}
}

func mustJSON(t *testing.T, value any) []byte {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return data
}
