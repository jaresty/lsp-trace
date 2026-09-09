package schema

import (
	"bytes"
	"fmt"

	jsonschema "github.com/santhosh-tekuri/jsonschema/v6"

	"lsp-trace/internal/graph"
)

// StructuralResult identifies a structurally validated schema family document.
// Its parsed representation is retained privately so semantic validation cannot
// accidentally precede structural admission.
type StructuralResult struct {
	Family    string
	Version   string
	document  map[string]any
	validated bool
}

// ValidateStructure validates JSON, resolves the family/version contract, and
// applies only the selected embedded Draft 2020-12 schema.
func ValidateStructure(data []byte, family, requested string) (StructuralResult, error) {
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(data))
	if err != nil {
		return StructuralResult{}, fmt.Errorf("invalid JSON: %w", err)
	}
	object, ok := doc.(map[string]any)
	if !ok {
		return StructuralResult{}, fmt.Errorf("schema_version: document must be an object")
	}
	field, familyOK := versionFields[family]
	if !familyOK {
		return StructuralResult{}, fmt.Errorf("unsupported schema family %q", family)
	}
	detected, ok := object[field].(string)
	if !ok || detected == "" {
		return StructuralResult{}, fmt.Errorf("%s: missing or not a string", field)
	}
	short, full, err := normalizeFamily(family, detected)
	if err != nil {
		return StructuralResult{}, err
	}
	if requested != "" {
		_, want, err := normalizeFamily(family, requested)
		if err != nil {
			return StructuralResult{}, err
		}
		if want != full {
			return StructuralResult{}, fmt.Errorf("schema version mismatch: document=%s requested=%s", full, want)
		}
	}
	raw, err := BytesFor(family, short)
	if err != nil {
		return StructuralResult{}, err
	}
	schemaDoc, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	if err != nil {
		return StructuralResult{}, fmt.Errorf("embedded schema %s: %w", full, err)
	}
	compiler := jsonschema.NewCompiler()
	compiler.DefaultDraft(jsonschema.Draft2020)
	if family == FamilyBoundedMetricsV2 || family == FamilyBoundedRankingV2 {
		baseRaw, baseErr := BytesFor(FamilyBoundedAnalysisV2, "v2")
		if baseErr != nil {
			return StructuralResult{}, baseErr
		}
		baseDoc, baseErr := jsonschema.UnmarshalJSON(bytes.NewReader(baseRaw))
		if baseErr != nil {
			return StructuralResult{}, fmt.Errorf("embedded analysis base schema: %w", baseErr)
		}
		baseResource := "https://jaresty.github.io/lsp-trace/schemas/lsp-trace.local-normative-analysis.v1.schema.json"
		if baseErr = compiler.AddResource(baseResource, baseDoc); baseErr != nil {
			return StructuralResult{}, fmt.Errorf("embedded analysis base schema: %w", baseErr)
		}
	}
	resource := "https://jaresty.github.io/lsp-trace/schemas/" + full + ".schema.json"
	if err := compiler.AddResource(resource, schemaDoc); err != nil {
		return StructuralResult{}, fmt.Errorf("embedded schema %s: %w", full, err)
	}
	compiled, err := compiler.Compile(resource)
	if err != nil {
		return StructuralResult{}, fmt.Errorf("embedded schema %s: %w", full, err)
	}
	if err := compiled.Validate(doc); err != nil {
		return StructuralResult{}, fmt.Errorf("schema validation %s: %w", full, err)
	}
	return StructuralResult{Family: family, Version: full, document: object, validated: true}, nil
}

// ValidateSemantics applies only the semantic rules selected by an admitted
// StructuralResult. Historical graph schemas intentionally have no semantic pass.
func ValidateSemantics(data []byte, structural StructuralResult) error {
	if !structural.validated {
		return fmt.Errorf("structural validation required before semantic validation")
	}
	trimmed := bytes.TrimSpace(data)
	var err error
	switch {
	case structural.Family == FamilyBoundedRanking:
		return fmt.Errorf("bounded ranking requires composed boundedranking.ValidateFor semantic validation")
	case structural.Family == FamilyBoundedMetrics:
		return fmt.Errorf("bounded metrics requires composed boundedmetrics.ValidateFor semantic validation")
	case structural.Family == FamilyBoundedAnalysis:
		return fmt.Errorf("bounded analysis requires composed boundedanalysis.ValidateFor semantic validation")
	case structural.Family == FamilyRetainedCalls:
		return fmt.Errorf("retained calls requires composed retainedcalls.ValidateFor semantic validation")
	case structural.Family == FamilyGraphProvenance:
		return fmt.Errorf("graph provenance requires composed graphprovenance.ValidateFor semantic validation")
	case structural.Family == FamilyOperationalCustody:
		return fmt.Errorf("operational custody requires composed custodyevidence.ValidateFor semantic validation")
	case structural.Family == FamilyGraph && structural.Version == graph.SchemaVersionV3:
		err = graph.ValidateSemanticBundle(trimmed)
	case structural.Family == FamilyInspect && structural.Version == InspectionVersionV1 && structural.document["projection_kind"] == "ALL_SEEDS":
		err = ValidateAllSeedInspection(trimmed)
	case structural.Family == FamilyFilter && structural.Version == "lsp-trace.filter.v1":
		err = ValidateFilter(trimmed)
	case structural.Family == FamilySourceDenominator && structural.Version == SourceDenominatorVersionV1:
		err = ValidateSourceDenominator(trimmed)
	}
	if err != nil {
		return fmt.Errorf("semantic validation %s: %w", structural.Version, err)
	}
	return nil
}
