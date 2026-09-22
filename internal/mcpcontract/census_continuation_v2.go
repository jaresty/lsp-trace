package mcpcontract

import (
	"bytes"
	"embed"
	"encoding/json"
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

func ValidateFutureCensusCompositeResultV2(raw []byte) error {
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
	if err != nil || compiled.Validate(value) != nil {
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
	} else if hasRequests || hasPreparations || hasGuidance {
		return errFutureCensusValue
	}
	return nil
}
