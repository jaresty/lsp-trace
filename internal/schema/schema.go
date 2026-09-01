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

var versions = map[string]string{"v1": graph.SchemaVersionV1, "v2": graph.SchemaVersionV2, "v3": graph.SchemaVersionV3}

func normalize(version string) (string, string, error) {
	version = strings.TrimSpace(version)
	if full, ok := versions[version]; ok {
		return version, full, nil
	}
	for short, full := range versions {
		if version == full {
			return short, full, nil
		}
	}
	return "", "", fmt.Errorf("unsupported schema version %q", version)
}

// Bytes returns the exact committed Draft 2020-12 schema bytes.
func Bytes(version string) ([]byte, error) {
	_, full, err := normalize(version)
	if err != nil {
		return nil, err
	}
	return files.ReadFile("schemas/" + full + ".schema.json")
}

// Validate runs structural validation and then the existing deeper v3 semantic validation.
func Validate(data []byte, requested string) (string, error) {
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("invalid JSON: %w", err)
	}
	object, ok := doc.(map[string]any)
	if !ok {
		return "", fmt.Errorf("schema_version: document must be an object")
	}
	detected, ok := object["schema_version"].(string)
	if !ok || detected == "" {
		return "", fmt.Errorf("schema_version: missing or not a string")
	}
	short, full, err := normalize(detected)
	if err != nil {
		return "", err
	}
	if requested != "" {
		_, want, err := normalize(requested)
		if err != nil {
			return "", err
		}
		if want != full {
			return "", fmt.Errorf("schema version mismatch: document=%s requested=%s", full, want)
		}
	}
	raw, err := Bytes(short)
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
	if full == graph.SchemaVersionV3 {
		if err := graph.ValidateSemanticBundle(bytes.TrimSpace(data)); err != nil {
			return "", fmt.Errorf("semantic validation %s: %w", full, err)
		}
	}
	return full, nil
}
