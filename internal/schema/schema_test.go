package schema

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"lsp-trace/internal/graph"
)

func TestEmbeddedSchemasMatchCommittedBytes(t *testing.T) {
	for _, version := range []string{"v1", "v2", "v3"} {
		got, err := Bytes(version)
		if err != nil {
			t.Fatalf("ASSERT_SCHEMA_BYTES_%s: %v", strings.ToUpper(version), err)
		}
		full := "lsp-trace.graph." + version
		want, err := os.ReadFile("schemas/" + full + ".schema.json")
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, want) {
			t.Fatalf("ASSERT_SCHEMA_BYTES_%s: embedded bytes differ from committed schema", strings.ToUpper(version))
		}
	}
}

func TestProducerGeneratedV3PassesLayeredValidation(t *testing.T) {
	result := graph.Result{
		SchemaVersion: graph.SchemaVersionV3,
		Invocation: graph.Invocation{
			Server: graph.ServerInvocation{Command: "server", Arguments: []string{"--stdio"}},
			Limits: graph.Limits{MaxDepth: 2, MaxNodes: 3, TimeoutMS: 4000},
		},
		Summary: graph.Summary{Complete: true},
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	if detected, err := Validate(encoded, "v3"); err != nil || detected != graph.SchemaVersionV3 {
		t.Fatalf("ASSERT_PRODUCER_V3_SCHEMA_VALID: detected=%q err=%v", detected, err)
	}
}

func TestV3SliceValidatesOptionalTerminalAndUpwardStartEvidenceFieldTypes(t *testing.T) {
	n := graph.NewNode(graph.Item{Name: "start", URI: "file:///w/a.go"})
	result := graph.Result{SchemaVersion: graph.SchemaVersionV3, Invocation: graph.Invocation{Seeds: []graph.InvocationSeed{{Label: "seed", At: "a.go:1:1"}}}, Nodes: []graph.Node{n}, Seeds: []graph.SeedResult{{Label: "seed", PreparedTargetIDs: []string{n.ID}, ReachedNodeIDs: []string{n.ID}}}, Slice: &graph.SliceEvidence{
		SourceURI: "file:///w/a.go", DownDepth: 0, UpDepth: 1,
		StartingNodeIDs: []string{n.ID}, Layers: []graph.SliceLayer{{Depth: 0, NodeIDs: []string{n.ID}}}, FrontierNodeIDs: []string{n.ID}, UpwardStartNodeIDs: []string{n.ID},
	}}
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"outgoing_terminal_node_ids", "upward_start_node_ids"} {
		var document map[string]any
		if err := json.Unmarshal(encoded, &document); err != nil {
			t.Fatal(err)
		}
		document["slice"].(map[string]any)[field] = "not-an-array"
		mutated, _ := json.Marshal(document)
		if _, err := Validate(mutated, "v3"); err == nil || !strings.Contains(err.Error(), field) {
			t.Fatalf("ASSERT_V3_SLICE_FIELD_TYPE_%s: %v", field, err)
		}
	}
	legacyResult := graph.Result{SchemaVersion: graph.SchemaVersionV3, Nodes: []graph.Node{n}, Slice: &graph.SliceEvidence{
		SourceURI: "file:///w/a.go", DownDepth: 0, UpDepth: 1,
		StartingNodeIDs: []string{n.ID}, Layers: []graph.SliceLayer{{Depth: 0, NodeIDs: []string{n.ID}}}, FrontierNodeIDs: []string{n.ID},
	}}
	legacyEncoded, err := json.Marshal(legacyResult)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(legacyEncoded, []byte("outgoing_terminal_node_ids")) || bytes.Contains(legacyEncoded, []byte("upward_start_node_ids")) {
		t.Fatalf("ASSERT_V3_HISTORICAL_SLICE_OMITS_ADDITIVE_FIELDS: %s", legacyEncoded)
	}
	if _, err := Validate(legacyEncoded, "v3"); err != nil {
		t.Fatalf("ASSERT_V3_HISTORICAL_SLICE_WITHOUT_ADDITIVE_FIELDS: %v", err)
	}
}

func TestProducerGeneratedV3SlicePassesLayeredValidation(t *testing.T) {
	n := graph.NewNode(graph.Item{Name: "start", URI: "file:///w/a.go"})
	result := graph.Result{SchemaVersion: graph.SchemaVersionV3, Invocation: graph.Invocation{Seeds: []graph.InvocationSeed{{Label: "seed", At: "a.go:1:1"}}}, Nodes: []graph.Node{n}, Seeds: []graph.SeedResult{{Label: "seed", PreparedTargetIDs: []string{n.ID}, ReachedNodeIDs: []string{n.ID}}}, Slice: &graph.SliceEvidence{
		SourceURI: "file:///w/a.go", DownDepth: 0, UpDepth: 1,
		StartingNodeIDs: []string{n.ID}, Layers: []graph.SliceLayer{{Depth: 0, NodeIDs: []string{n.ID}}}, FrontierNodeIDs: []string{n.ID}, UpwardStartNodeIDs: []string{n.ID},
	}}
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Validate(encoded, "v3"); err != nil {
		t.Fatalf("ASSERT_V3_SLICE_SCHEMA_ACCEPTED: %v", err)
	}
}

func TestStructuralValidationPrecedesV3Semantics(t *testing.T) {
	_, err := Validate([]byte(`{"schema_version":"lsp-trace.graph.v3"}`), "")
	if err == nil || !strings.Contains(err.Error(), "schema validation") || strings.Contains(err.Error(), "semantic validation") {
		t.Fatalf("ASSERT_SCHEMA_BEFORE_V3_SEMANTICS: %v", err)
	}
}
