package schema

import (
	"bytes"
	"embed"
	"fmt"
	"strings"

	jsonschema "github.com/santhosh-tekuri/jsonschema/v6"

	"lsp-trace/internal/graph"
)

//go:embed schemas/*.json
var files embed.FS

const (
	FamilyGraph   = "graph"
	FamilyInspect = "inspect"
	FamilyFilter  = "filter"
)

var familyVersions = map[string]map[string]string{
	FamilyGraph:   {"v1": graph.SchemaVersionV1, "v2": graph.SchemaVersionV2, "v3": graph.SchemaVersionV3},
	FamilyInspect: {"v1": "lsp-trace.inspect.v1"},
	FamilyFilter:  {"v1": "lsp-trace.filter.v1"},
}

var versionFields = map[string]string{
	FamilyGraph: "schema_version", FamilyInspect: "inspection_schema_version", FamilyFilter: "filter_schema_version",
}

func normalizeFamily(family, version string) (string, string, error) {
	family, version = strings.TrimSpace(family), strings.TrimSpace(version)
	versions, ok := familyVersions[family]
	if !ok {
		return "", "", fmt.Errorf("unsupported schema family %q", family)
	}
	if full, ok := versions[version]; ok {
		return version, full, nil
	}
	for short, full := range versions {
		if version == full {
			return short, full, nil
		}
	}
	return "", "", fmt.Errorf("unsupported schema version %q for family %q", version, family)
}

// BytesFor returns the exact committed Draft 2020-12 schema bytes for a family and version.
func BytesFor(family, version string) ([]byte, error) {
	_, full, err := normalizeFamily(family, version)
	if err != nil {
		return nil, err
	}
	return files.ReadFile("schemas/" + full + ".schema.json")
}

// Bytes preserves the graph-family schema lookup compatibility alias.
func Bytes(version string) ([]byte, error) { return BytesFor(FamilyGraph, version) }

// ValidateFor runs structural validation for the requested schema family.
func ValidateFor(data []byte, family, requested string) (string, error) {
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("invalid JSON: %w", err)
	}
	object, ok := doc.(map[string]any)
	if !ok {
		return "", fmt.Errorf("schema_version: document must be an object")
	}
	field, familyOK := versionFields[family]
	if !familyOK {
		return "", fmt.Errorf("unsupported schema family %q", family)
	}
	detected, ok := object[field].(string)
	if !ok || detected == "" {
		return "", fmt.Errorf("%s: missing or not a string", field)
	}
	short, full, err := normalizeFamily(family, detected)
	if err != nil {
		return "", err
	}
	if requested != "" {
		_, want, err := normalizeFamily(family, requested)
		if err != nil {
			return "", err
		}
		if want != full {
			return "", fmt.Errorf("schema version mismatch: document=%s requested=%s", full, want)
		}
	}
	raw, err := BytesFor(family, short)
	if err != nil {
		return "", err
	}
	schemaDoc, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	if err != nil {
		return "", fmt.Errorf("embedded schema %s: %w", full, err)
	}
	compiler := jsonschema.NewCompiler()
	compiler.DefaultDraft(jsonschema.Draft2020)
	resource := "https://jaresty.github.io/lsp-trace/schemas/" + full + ".schema.json"
	if err := compiler.AddResource(resource, schemaDoc); err != nil {
		return "", fmt.Errorf("embedded schema %s: %w", full, err)
	}
	compiled, err := compiler.Compile(resource)
	if err != nil {
		return "", fmt.Errorf("embedded schema %s: %w", full, err)
	}
	if err := compiled.Validate(doc); err != nil {
		return "", fmt.Errorf("schema validation %s: %w", full, err)
	}
	if family == FamilyGraph && full == graph.SchemaVersionV3 {
		if err := graph.ValidateSemanticBundle(bytes.TrimSpace(data)); err != nil {
			return "", fmt.Errorf("semantic validation %s: %w", full, err)
		}
	}
	if family == FamilyInspect && object["projection_kind"] == "ALL_SEEDS" {
		if err := ValidateAllSeedInspection(bytes.TrimSpace(data)); err != nil {
			return "", fmt.Errorf("semantic validation %s: %w", full, err)
		}
	}
	if family == FamilyFilter {
		if err := ValidateFilter(bytes.TrimSpace(data)); err != nil {
			return "", fmt.Errorf("semantic validation %s: %w", full, err)
		}
	}
	return full, nil
}

// Validate preserves the graph-family validation compatibility alias.
func Validate(data []byte, requested string) (string, error) {
	return ValidateFor(data, FamilyGraph, requested)
}
