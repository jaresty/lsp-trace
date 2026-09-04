package graph

import (
	"bytes"
	"encoding/json"
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
	if artifact.SchemaVersion != "lsp-trace.normalized-relations.v1" {
		t.Fatalf("ASSERT_NORMALIZED_RELATIONS_SCHEMA_VERSION: got %q", artifact.SchemaVersion)
	}
}

func TestNormalizedRelationsPreserveCanonicalRecords(t *testing.T) {
	want := normalizedRelationsFixture().Edges
	got := NormalizeRelations(normalizedRelationsFixture()).Relations
	if len(got) != 1 || !bytes.Equal(mustJSON(t, got), mustJSON(t, want)) {
		t.Fatalf("ASSERT_NORMALIZED_RELATIONS_CANONICAL_RECORDS: got %#v want %#v", got, want)
	}
}

func TestNormalizedRelationsSchemaBoundary(t *testing.T) {
	schema := NormalizedRelationsSchemaJSON()
	if !bytes.Contains(schema, []byte(`"$schema":"https://json-schema.org/draft/2020-12/schema"`)) ||
		!bytes.Contains(schema, []byte(`"const":"lsp-trace.normalized-relations.v1"`)) {
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
