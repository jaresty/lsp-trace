package retainedprojection

import (
	"crypto/sha256"
	"encoding/hex"
	"reflect"
	"testing"

	"lsp-trace/internal/graphprovenance"
)

func adr0007BindingFixture(t *testing.T) (Admitted, Plan) {
	t.Helper()
	a := fixture()
	a.raw = []byte("exact admitted source snapshot bytes")
	a.parent.GraphV5Bytes = []byte("synthetic graph bytes")
	graphSum := sha256.Sum256(a.parent.GraphV5Bytes)
	a.parent.GraphV5Digest = "sha256:" + hex.EncodeToString(graphSum[:])
	plan, err := Select(a, Request{Target: Key{"target", "file:///target.go"}, Selections: []Key{{"a", "file:///a.go"}, {"target", "file:///target.go"}}})
	if err != nil {
		t.Fatalf("fixture select: %v", err)
	}
	return a, plan
}

func TestADR0007LegacyAndGraphProvenanceCustodyBindings(t *testing.T) {
	a, plan := adr0007BindingFixture(t)
	legacy, err := a.CustodyBinding(plan)
	if err != nil {
		t.Fatalf("legacy binding: %v", err)
	}
	outer, err := a.GraphProvenanceCustodyBinding(plan)
	if err != nil {
		t.Fatalf("outer binding: %v", err)
	}

	if legacy.GraphSchemaID != graphprovenance.GraphV5SchemaID {
		t.Fatalf("ASSERT_ADR0007_LEGACY_BINDING_USES_INNER_GRAPH_SCHEMA: got=%q want=%q", legacy.GraphSchemaID, graphprovenance.GraphV5SchemaID)
	}
	if outer.GraphSchemaID != GraphProvenanceV5SchemaID {
		t.Fatalf("ASSERT_ADR0007_OUTER_BINDING_USES_GRAPH_PROVENANCE_SCHEMA: got=%q want=%q", outer.GraphSchemaID, GraphProvenanceV5SchemaID)
	}
	legacy.GraphSchemaID = ""
	outer.GraphSchemaID = ""
	if !reflect.DeepEqual(legacy, outer) {
		t.Fatalf("ASSERT_ADR0007_LEGACY_AND_OUTER_BINDINGS_SHARE_NON_SCHEMA_TUPLE: legacy=%+v outer=%+v", legacy, outer)
	}
}

func TestADR0007GraphProvenanceCustodyBindingPropagatesInvalidInputAtomically(t *testing.T) {
	got, err := (Admitted{}).GraphProvenanceCustodyBinding(Plan{})
	if err == nil {
		t.Fatal("ASSERT_ADR0007_OUTER_BINDING_INVALID_INPUT_RETURNS_ERROR: accepted")
	}
	if got != (RetainedCustodyBinding{}) {
		t.Fatalf("ASSERT_ADR0007_OUTER_BINDING_INVALID_INPUT_IS_ATOMIC: got=%+v", got)
	}
}
