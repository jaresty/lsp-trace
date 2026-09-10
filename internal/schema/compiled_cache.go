package schema

import (
	"bytes"
	"fmt"
	"sync"

	jsonschema "github.com/santhosh-tekuri/jsonschema/v6"
)

type compiledSchemaResult struct {
	once   sync.Once
	schema *jsonschema.Schema
	err    error
}

var compiledSchemaCache sync.Map

func compiledSchemaFor(family, short, full string) (*jsonschema.Schema, error) {
	value, _ := compiledSchemaCache.LoadOrStore(family+"\x00"+full, &compiledSchemaResult{})
	result := value.(*compiledSchemaResult)
	result.once.Do(func() {
		result.schema, result.err = compileSchema(family, short, full)
	})
	return result.schema, result.err
}

func compileSchema(family, short, full string) (*jsonschema.Schema, error) {
	raw, err := BytesFor(family, short)
	if err != nil {
		return nil, err
	}
	schemaDoc, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("embedded schema %s: %w", full, err)
	}
	compiler := jsonschema.NewCompiler()
	compiler.DefaultDraft(jsonschema.Draft2020)
	if family == FamilyBoundedMetricsV2 || family == FamilyBoundedRankingV2 {
		baseRaw, baseErr := BytesFor(FamilyBoundedAnalysisV2, "v2")
		if baseErr != nil {
			return nil, baseErr
		}
		baseDoc, baseErr := jsonschema.UnmarshalJSON(bytes.NewReader(baseRaw))
		if baseErr != nil {
			return nil, fmt.Errorf("embedded analysis base schema: %w", baseErr)
		}
		baseResource := "https://jaresty.github.io/lsp-trace/schemas/lsp-trace.local-normative-analysis.v1.schema.json"
		if baseErr = compiler.AddResource(baseResource, baseDoc); baseErr != nil {
			return nil, fmt.Errorf("embedded analysis base schema: %w", baseErr)
		}
	}
	resource := "https://jaresty.github.io/lsp-trace/schemas/" + full + ".schema.json"
	if err := compiler.AddResource(resource, schemaDoc); err != nil {
		return nil, fmt.Errorf("embedded schema %s: %w", full, err)
	}
	compiled, err := compiler.Compile(resource)
	if err != nil {
		return nil, fmt.Errorf("embedded schema %s: %w", full, err)
	}
	return compiled, nil
}
