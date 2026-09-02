package schema

import (
	"bytes"
	"fmt"

	jsonschema "github.com/santhosh-tekuri/jsonschema/v6"
)

const InspectionVersionV1 = "lsp-trace.inspect.v1"

// InspectionBytes returns the exact committed Draft 2020-12 inspection schema bytes.
func InspectionBytes(version string) ([]byte, error) {
	if version != InspectionVersionV1 {
		return nil, fmt.Errorf("unsupported inspection schema version %q", version)
	}
	return files.ReadFile("schemas/" + InspectionVersionV1 + ".schema.json")
}

// ValidateInspection validates an inspection projection independently of graph schemas.
func ValidateInspection(data []byte) error {
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("invalid inspection JSON: %w", err)
	}
	raw, err := InspectionBytes(InspectionVersionV1)
	if err != nil {
		return err
	}
	schemaDoc, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	if err != nil {
		return fmt.Errorf("embedded inspection schema %s: %w", InspectionVersionV1, err)
	}
	compiler := jsonschema.NewCompiler()
	compiler.DefaultDraft(jsonschema.Draft2020)
	resource := "https://jaresty.github.io/lsp-trace/schemas/" + InspectionVersionV1 + ".schema.json"
	if err := compiler.AddResource(resource, schemaDoc); err != nil {
		return fmt.Errorf("embedded inspection schema %s: %w", InspectionVersionV1, err)
	}
	compiled, err := compiler.Compile(resource)
	if err != nil {
		return fmt.Errorf("embedded inspection schema %s: %w", InspectionVersionV1, err)
	}
	if err := compiled.Validate(doc); err != nil {
		return fmt.Errorf("inspection schema validation %s: %w", InspectionVersionV1, err)
	}
	return nil
}
