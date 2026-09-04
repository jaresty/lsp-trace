package graph

import (
	"bytes"
	_ "embed"
	"fmt"

	jsonschema "github.com/santhosh-tekuri/jsonschema/v6"
)

const NormalizedRelationsSchemaVersion = "lsp-trace.normalized-relations.v1"
const NormalizedRelationsArtifactKind = "NORMALIZED_RELATIONS"
const normalizedRelationsSchemaID = "https://lsp-trace.dev/schemas/normalized-relations/v1/schema.json"

//go:embed schemas/normalized-relations.v1.schema.json
var normalizedRelationsSchema []byte

// NormalizedRelations is a standalone projection of canonical call relations.
// It is deliberately separate from Result so graph v2/v3 serialization remains unchanged.
type NormalizedRelations struct {
	SchemaVersion string `json:"schema_version"`
	ArtifactKind  string `json:"artifact_kind"`
	Relations     []Edge `json:"relations"`
}

// NormalizeRelations returns a detached relations artifact without mutating result.
func NormalizeRelations(result Result) NormalizedRelations {
	relations := make([]Edge, len(result.Edges))
	for i, edge := range result.Edges {
		relations[i] = edge
		relations[i].CallSites = append([]Range(nil), edge.CallSites...)
	}
	return NormalizedRelations{
		SchemaVersion: NormalizedRelationsSchemaVersion,
		ArtifactKind:  NormalizedRelationsArtifactKind,
		Relations:     relations,
	}
}

// NormalizedRelationsSchemaJSON returns a defensive copy of the embedded schema.
func NormalizedRelationsSchemaJSON() []byte {
	return append([]byte(nil), normalizedRelationsSchema...)
}

// ValidateNormalizedRelationsJSON validates a standalone artifact against its embedded schema.
func ValidateNormalizedRelationsJSON(data []byte) error {
	compiler := jsonschema.NewCompiler()
	compiler.DefaultDraft(jsonschema.Draft2020)
	document, err := jsonschema.UnmarshalJSON(bytes.NewReader(normalizedRelationsSchema))
	if err != nil {
		return fmt.Errorf("decode normalized relations schema: %w", err)
	}
	if err := compiler.AddResource(normalizedRelationsSchemaID, document); err != nil {
		return fmt.Errorf("register normalized relations schema: %w", err)
	}
	compiled, err := compiler.Compile(normalizedRelationsSchemaID)
	if err != nil {
		return fmt.Errorf("compile normalized relations schema: %w", err)
	}
	value, err := jsonschema.UnmarshalJSON(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("invalid normalized relations JSON: %w", err)
	}
	if err := compiled.Validate(value); err != nil {
		return fmt.Errorf("normalized relations schema validation: %w", err)
	}
	return nil
}
