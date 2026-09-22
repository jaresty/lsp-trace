package mcpcontract

import (
	"bytes"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"lsp-trace/internal/strictjson"

	jsonschema "github.com/santhosh-tekuri/jsonschema/v6"
)

const (
	FutureCensusV2InputID                    = "https://jaresty.github.io/lsp-trace/mcp/schemas/input-census.v2.schema.json"
	FutureCensusCompositeResultID            = "https://jaresty.github.io/lsp-trace/schemas/lsp-trace.census-feature-catalog-result.v2.schema.json"
	FutureCensusCompositeSuccessID           = "https://jaresty.github.io/lsp-trace/mcp/schemas/envelope-census-feature-catalog-result.v2.schema.json"
	CensusContinuationDiagnosticID           = "https://jaresty.github.io/lsp-trace/schemas/lsp-trace.census-continuation-diagnostic.v1.schema.json"
	CensusContinuationDiagnosticEnvelopeID   = "https://jaresty.github.io/lsp-trace/mcp/schemas/envelope-census-continuation-diagnostic.v1.schema.json"
	CensusContinuationDiagnosticV2ID         = "https://jaresty.github.io/lsp-trace/schemas/lsp-trace.census-continuation-diagnostic.v2.schema.json"
	CensusContinuationDiagnosticEnvelopeV2ID = "https://jaresty.github.io/lsp-trace/mcp/schemas/envelope-census-continuation-diagnostic.v2.schema.json"
	CensusRequestReceiptID                   = "https://jaresty.github.io/lsp-trace/schemas/lsp-trace.census-request-receipt.v1.schema.json"
	CensusDiscoveryDiagnosticV2ID            = "https://jaresty.github.io/lsp-trace/schemas/lsp-trace.census-discovery-diagnostic.v2.schema.json"
	CensusDiscoveryDiagnosticEnvelopeV2ID    = "https://jaresty.github.io/lsp-trace/mcp/schemas/envelope-census-discovery-diagnostic.v2.schema.json"
)

//go:embed testdata/schemas/input-census.v2.schema.json testdata/schemas/lsp-trace.census-feature-catalog-result.v2.schema.json testdata/schemas/envelope-census-feature-catalog-result.v2.schema.json testdata/schemas/lsp-trace.census-continuation-diagnostic.v1.schema.json testdata/schemas/envelope-census-continuation-diagnostic.v1.schema.json testdata/schemas/lsp-trace.census-continuation-diagnostic.v2.schema.json testdata/schemas/envelope-census-continuation-diagnostic.v2.schema.json testdata/schemas/lsp-trace.census-request-receipt.v1.schema.json testdata/schemas/lsp-trace.census-discovery-diagnostic.v2.schema.json testdata/schemas/envelope-census-discovery-diagnostic.v2.schema.json
var futureCensusV2Files embed.FS

var futureCensusResumeSelector = regexp.MustCompile(`^g-[0-9a-f]{64}\.selector\.json$`)

func futureCensusV2SchemaJSON(schemaID string) ([]byte, bool, error) {
	paths := map[string]string{
		FutureCensusV2InputID:                    "testdata/schemas/input-census.v2.schema.json",
		FutureCensusCompositeResultID:            "testdata/schemas/lsp-trace.census-feature-catalog-result.v2.schema.json",
		FutureCensusCompositeSuccessID:           "testdata/schemas/envelope-census-feature-catalog-result.v2.schema.json",
		CensusContinuationDiagnosticID:           "testdata/schemas/lsp-trace.census-continuation-diagnostic.v1.schema.json",
		CensusContinuationDiagnosticEnvelopeID:   "testdata/schemas/envelope-census-continuation-diagnostic.v1.schema.json",
		CensusContinuationDiagnosticV2ID:         "testdata/schemas/lsp-trace.census-continuation-diagnostic.v2.schema.json",
		CensusContinuationDiagnosticEnvelopeV2ID: "testdata/schemas/envelope-census-continuation-diagnostic.v2.schema.json",
		CensusRequestReceiptID:                   "testdata/schemas/lsp-trace.census-request-receipt.v1.schema.json",
		CensusDiscoveryDiagnosticV2ID:            "testdata/schemas/lsp-trace.census-discovery-diagnostic.v2.schema.json",
		CensusDiscoveryDiagnosticEnvelopeV2ID:    "testdata/schemas/envelope-census-discovery-diagnostic.v2.schema.json",
	}
	name, ok := paths[schemaID]
	if !ok {
		return nil, false, nil
	}
	raw, err := futureCensusV2Files.ReadFile(name)
	return append([]byte(nil), raw...), true, err
}

func DecodeFutureCensusRequestV2(raw []byte) (map[string]any, error) {
	if err := strictjson.RejectDuplicates(raw); err != nil {
		if strings.Contains(err.Error(), "duplicate JSON member") {
			return nil, errFutureCensusDuplicate
		}
		return nil, errFutureCensusShape
	}
	var value map[string]any
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	if err := dec.Decode(&value); err != nil || value == nil {
		return nil, errFutureCensusShape
	}
	continuation, err := objectField(value, "continuation")
	if err != nil || !allowed(continuation, "kind", "resume_selector", "stop_after") || continuation["kind"] != "ADR_0007_FEATURE_CATALOG" {
		return nil, errFutureCensusShape
	}
	if stop, present := continuation["stop_after"]; present {
		if value, ok := stop.(string); !ok || value != "DESCRIBE_REQUESTS" {
			return nil, errFutureCensusValue
		}
	}
	resume, hasResume := continuation["resume_selector"]
	if hasResume {
		if !allowed(value, "continuation") || !required(continuation, "kind", "resume_selector") {
			return nil, errFutureCensusShape
		}
		s, ok := resume.(string)
		if !ok || len(s) != 80 || !futureCensusResumeSelector.MatchString(s) || !safeFutureSelector(s) {
			return nil, errFutureCensusValue
		}
		return value, nil
	}
	if !allowed(value, "session_id", "generation", "sources", "includes", "excludes", "down_depth", "up_depth", "max_nodes", "batch_targets", "timeout_ms", "request_timeout_ms", "continuation") {
		return nil, errFutureCensusShape
	}
	fresh := make(map[string]any, len(value)-1)
	for k, v := range value {
		if k != "continuation" && k != "batch_targets" {
			fresh[k] = v
		}
	}
	if err := ValidateFutureCensusRequestV1(fresh); err != nil {
		return nil, err
	}
	batchTargets, present := value["batch_targets"]
	if !present {
		value["batch_targets"] = json.Number("16")
	} else if number, ok := batchTargets.(json.Number); !ok {
		return nil, errFutureCensusValue
	} else if n, err := number.Int64(); err != nil || n < 1 || n > 63 {
		return nil, errFutureCensusValue
	}
	return value, nil
}

func ValidateFutureCensusCompositeEnvelopeV2(raw []byte) error {
	if err := strictjson.RejectDuplicates(raw); err != nil {
		return errFutureCensusShape
	}
	var value map[string]any
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	if err := dec.Decode(&value); err != nil || value == nil {
		return errFutureCensusShape
	}
	compiler := jsonschema.NewCompiler()
	compiler.DefaultDraft(jsonschema.Draft2020)
	for _, id := range []string{CensusResultID, FutureCensusCompositeResultID, FutureCensusCompositeSuccessID} {
		schemaRaw, err := FutureCensusSchemaJSON(id)
		if err != nil {
			return err
		}
		doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(schemaRaw))
		if err != nil {
			return err
		}
		if err := compiler.AddResource(id, doc); err != nil {
			return err
		}
	}
	compiled, err := compiler.Compile(FutureCensusCompositeSuccessID)
	if err != nil || compiled.Validate(value) != nil {
		return errFutureCensusShape
	}
	resultRaw, err := json.Marshal(value["result"])
	if err != nil {
		return errFutureCensusShape
	}
	return ValidateFutureCensusCompositeResultV2(resultRaw)
}

// ClassifyFutureCensusCompositeResultV2 returns only a closed-schema field and
// invariant classification. It never exposes the underlying validation error.
func ClassifyFutureCensusCompositeResultV2(raw []byte) (field, invariant string) {
	if err := validateFutureCensusCompositeSchema(raw); err != nil {
		var ve *jsonschema.ValidationError
		if errors.As(err, &ve) {
			pending := []*jsonschema.ValidationError{ve}
			for len(pending) > 0 {
				current := pending[0]
				pending = pending[1:]
				field, invariant = classifyFutureCensusValidationError(current)
				if field != "" {
					return field, invariant
				}
				pending = append(pending, current.Causes...)
			}
		}
		if field, invariant = classifyFutureCensusCompositeSemantics(raw); field != "" {
			return field, invariant
		}
	}
	return "UNKNOWN", "SCHEMA"
}

func classifyFutureCensusCompositeSemantics(raw []byte) (string, string) {
	var value map[string]any
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	if dec.Decode(&value) != nil {
		return "ROOT", "DECODE"
	}
	catalog, ok := value["catalog"].(map[string]any)
	if !ok {
		return "CATALOG", "TYPE"
	}
	checkpoint, _ := catalog["checkpoint_selector"].(string)
	if catalog["composite_selector"] != checkpoint || catalog["catalog_selector"] != checkpoint {
		return "CATALOG_SELECTOR", "EQUALITY"
	}
	if identity, ok := value["census_identity"].(map[string]any); ok && identity["selector"] != checkpoint {
		return "CENSUS_IDENTITY_SELECTOR", "EQUALITY"
	}
	_, hasRequests := catalog["request_count"]
	_, hasPreparations := catalog["preparation_count"]
	guidance, hasGuidance := catalog["resume_guidance"]
	if catalog["status"] != "PAUSED" {
		if hasRequests || hasPreparations || hasGuidance {
			return "CATALOG_PAUSED_FIELDS", "FORBIDDEN"
		}
		return "", ""
	}
	if !hasRequests || !hasPreparations || !hasGuidance {
		return "CATALOG_PAUSED_FIELDS", "REQUIRED"
	}
	if guidance != "Resume with this selector and omit stop_after to continue exactly the remaining work once." {
		return "CATALOG_GUIDANCE", "CONST"
	}
	requestCount, requestOK := catalog["request_count"].(json.Number)
	preparationCount, preparationOK := catalog["preparation_count"].(json.Number)
	requestValue, requestErr := requestCount.Int64()
	preparationValue, preparationErr := preparationCount.Int64()
	if !requestOK || requestErr != nil || requestValue < 1 {
		return "CATALOG_REQUEST_COUNT", "MINIMUM"
	}
	if !preparationOK || preparationErr != nil || preparationValue < 0 {
		return "CATALOG_PREPARATION_COUNT", "MINIMUM"
	}
	if preparationValue > requestValue {
		return "CATALOG_PREPARATION_COUNT", "LTE_REQUEST_COUNT"
	}
	return "", ""
}

func classifyFutureCensusValidationError(err *jsonschema.ValidationError) (string, string) {
	if err == nil {
		return "", ""
	}
	field := strings.Join(err.InstanceLocation, "_")
	if field == "" {
		field = "root"
	}
	known := map[string]bool{
		"root": true, "census": true, "catalog": true,
		"census_schema_version": true, "census_status": true,
		"census_census_id": true, "census_capture_set_id": true, "census_session_id": true,
		"census_generation": true, "census_target_count": true, "census_batch_count": true,
		"census_authority": true, "census_source_graph_complete": true,
		"census_native_aggregate_custody": true, "census_cross_capture_calls": true,
		"census_leiden_admissible":           true,
		"census_file_accounting_denominator": true, "census_file_accounting_processed": true,
		"census_file_accounting_excluded": true, "census_file_accounting_forbidden": true,
		"census_file_accounting_unreadable": true, "census_file_accounting_unsupported": true,
		"census_file_accounting_document_symbol_failed": true, "census_file_accounting_omitted": true,
		"census_file_accounting_incomplete":    true,
		"census_symbol_accounting_denominator": true, "census_symbol_accounting_prepared": true,
		"census_symbol_accounting_unsupported": true, "census_symbol_accounting_preparation_failed": true,
		"census_symbol_accounting_prepare_missing": true, "census_symbol_accounting_non_callable": true,
		"census_symbol_accounting_omitted": true, "census_symbol_accounting_incomplete": true,
		"census_publication_selector": true, "census_publication_digest": true,
		"census_publication_byte_length": true, "census_publication_verification_status": true,
		"census_publication_directory_sync_status": true, "census_publication_close_status": true,
		"catalog_checkpoint_selector": true, "catalog_composite_selector": true,
		"catalog_catalog_selector": true, "catalog_status": true, "catalog_resume_guidance": true,
	}
	keywords := err.ErrorKind.KeywordPath()
	keyword := "OTHER"
	if len(keywords) > 0 {
		switch value := keywords[len(keywords)-1]; value {
		case "pattern", "maxLength", "required", "type", "minimum", "maximum", "enum", "const", "minItems", "maxItems", "additionalProperties", "oneOf", "allOf", "not", "$ref":
			keyword = strings.ToUpper(strings.ReplaceAll(value, "Length", "_LENGTH"))
		}
	}
	if known[field] {
		return strings.ToUpper(field), keyword
	}
	branch := "ROOT"
	if len(err.InstanceLocation) > 0 {
		switch err.InstanceLocation[0] {
		case "census":
			branch = "CENSUS"
		case "catalog":
			branch = "CATALOG"
		}
	}
	depth := len(err.InstanceLocation)
	depthToken := "3_PLUS"
	if depth < 3 {
		depthToken = fmt.Sprintf("%d", depth)
	}
	return branch + "_DEPTH_" + depthToken, keyword
}

func validateFutureCensusCompositeSchema(raw []byte) error {
	if err := strictjson.RejectDuplicates(raw); err != nil {
		return errFutureCensusShape
	}
	var value map[string]any
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	if err := dec.Decode(&value); err != nil || value == nil {
		return errFutureCensusShape
	}
	compiler := jsonschema.NewCompiler()
	compiler.DefaultDraft(jsonschema.Draft2020)
	for _, id := range []string{CensusResultID, FutureCensusCompositeResultID} {
		schemaRaw, err := FutureCensusSchemaJSON(id)
		if err != nil {
			return err
		}
		doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(schemaRaw))
		if err != nil {
			return err
		}
		if err := compiler.AddResource(id, doc); err != nil {
			return err
		}
	}
	compiled, err := compiler.Compile(FutureCensusCompositeResultID)
	if err != nil {
		return err
	}
	if err := compiled.Validate(value); err != nil {
		return err
	}
	catalog := value["catalog"].(map[string]any)
	checkpoint := catalog["checkpoint_selector"].(string)
	if catalog["composite_selector"] != checkpoint || catalog["catalog_selector"] != checkpoint {
		return errFutureCensusValue
	}
	if identity, ok := value["census_identity"].(map[string]any); ok && identity["selector"] != checkpoint {
		return errFutureCensusValue
	}
	_, hasRequests := catalog["request_count"]
	_, hasPreparations := catalog["preparation_count"]
	guidance, hasGuidance := catalog["resume_guidance"]
	if catalog["status"] == "PAUSED" {
		if !hasRequests || !hasPreparations || !hasGuidance || guidance != "Resume with this selector and omit stop_after to continue exactly the remaining work once." {
			return errFutureCensusValue
		}
		requestCount, requestOK := catalog["request_count"].(json.Number)
		preparationCount, preparationOK := catalog["preparation_count"].(json.Number)
		requestValue, requestErr := requestCount.Int64()
		preparationValue, preparationErr := preparationCount.Int64()
		if !requestOK || !preparationOK || requestErr != nil || preparationErr != nil || requestValue < 1 || preparationValue < 0 || preparationValue > requestValue {
			return errFutureCensusValue
		}
	} else if hasRequests || hasPreparations || hasGuidance {
		return errFutureCensusValue
	}
	return nil
}

func ValidateFutureCensusCompositeResultV2(raw []byte) error {
	if err := validateFutureCensusCompositeSchema(raw); err != nil {
		if _, ok := err.(*jsonschema.ValidationError); ok {
			return errFutureCensusShape
		}
		return err
	}
	value := map[string]any{}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	if err := dec.Decode(&value); err != nil {
		return errFutureCensusShape
	}
	catalog := value["catalog"].(map[string]any)
	checkpoint := catalog["checkpoint_selector"].(string)
	if catalog["composite_selector"] != checkpoint || catalog["catalog_selector"] != checkpoint {
		return errFutureCensusValue
	}
	if identity, ok := value["census_identity"].(map[string]any); ok && identity["selector"] != checkpoint {
		return errFutureCensusValue
	}
	_, hasRequests := catalog["request_count"]
	_, hasPreparations := catalog["preparation_count"]
	guidance, hasGuidance := catalog["resume_guidance"]
	if catalog["status"] == "PAUSED" {
		if !hasRequests || !hasPreparations || !hasGuidance || guidance != "Resume with this selector and omit stop_after to continue exactly the remaining work once." {
			return errFutureCensusValue
		}
		requestCount, requestOK := catalog["request_count"].(json.Number)
		preparationCount, preparationOK := catalog["preparation_count"].(json.Number)
		requestValue, requestErr := requestCount.Int64()
		preparationValue, preparationErr := preparationCount.Int64()
		if !requestOK || !preparationOK || requestErr != nil || preparationErr != nil || requestValue < 1 || preparationValue < 0 || preparationValue > requestValue {
			return errFutureCensusValue
		}
	} else if hasRequests || hasPreparations || hasGuidance {
		return errFutureCensusValue
	}
	return nil
}
