package mcpcontract

import (
	"encoding/json"
	"fmt"

	"lsp-trace/internal/boundedanalysis"
	"lsp-trace/internal/hydratedinspection"
	"lsp-trace/internal/operation"
)

// OperationInputValidator binds transport-neutral operation names to the exact
// input schemas selected by the accepted MCP manifest.
type OperationInputValidator struct {
	schemaIDs map[operation.Name]string
}

// NewOperationInputValidator constructs an immutable manifest-backed validator
// for the Stage 1 offline operations.
func NewOperationInputValidator() (*OperationInputValidator, error) {
	manifest, err := LoadManifest()
	if err != nil {
		return nil, err
	}
	canonical := map[operation.Name]string{
		operation.Capabilities:              "lsp_trace_v1_capabilities",
		operation.SchemaGet:                 "lsp_trace_v1_schema_get",
		operation.Validate:                  "lsp_trace_v1_validate",
		operation.Verify:                    "lsp_trace_v1_verify",
		operation.VerifyV2:                  "lsp_trace_v2_verify",
		operation.VerifyRetainedCallsV2:     "lsp_trace_v2_verify_retained_calls",
		operation.Inspect:                   "lsp_trace_v1_inspect",
		operation.InspectHydrated:           HydratedTool,
		operation.Filter:                    "lsp_trace_v1_filter",
		operation.CustodyExecute:            "lsp_trace_v1_execute",
		operation.ExportRetainedCalls:       "lsp_trace_v1_export_retained_calls",
		operation.ExportRetainedCallsV2:     "lsp_trace_v2_export_retained_calls",
		operation.BoundedRetainedAnalysis:   "lsp_trace_v1_bounded_retained_analysis",
		operation.BoundedRetainedMetrics:    "lsp_trace_v1_bounded_retained_metrics",
		operation.BoundedRetainedRanking:    "lsp_trace_v1_bounded_retained_ranking",
		operation.BoundedRetainedAnalysisV2: "lsp_trace_v2_bounded_retained_analysis",
		operation.BoundedRetainedMetricsV2:  "lsp_trace_v2_bounded_retained_metrics",
		operation.BoundedRetainedRankingV2:  "lsp_trace_v2_bounded_retained_ranking",
	}
	schemaIDs := make(map[operation.Name]string, len(canonical))
	for name, toolName := range canonical {
		for _, tool := range WithPublicAnalyticsV2(WithAcquisitionV3(WithRetainedCallsV2Verifier(WithRetainedCallsV2Export(WithHydratedInspection(WithRetainedCalls(manifest)))))).Tools {
			if tool.Name == toolName {
				schemaIDs[name] = tool.InputSchemaID
				break
			}
		}
		if schemaIDs[name] == "" {
			return nil, fmt.Errorf("manifest tool %q is missing", toolName)
		}
	}
	return &OperationInputValidator{schemaIDs: schemaIDs}, nil
}

// ValidateOperationInput validates a complete operation input against the
// manifest-selected schema for that operation.
func (v *OperationInputValidator) ValidateOperationInput(name operation.Name, input json.RawMessage) error {
	if v == nil {
		return fmt.Errorf("operation input validator is nil")
	}
	if name == operation.InspectHydrated {
		return hydratedinspection.ValidateInputJSON(input)
	}
	bounded := name == operation.BoundedRetainedAnalysis || name == operation.BoundedRetainedMetrics || name == operation.BoundedRetainedRanking
	if name == operation.Validate {
		var h struct {
			Schema struct {
				Family string `json:"family"`
			} `json:"schema"`
		}
		if json.Unmarshal(input, &h) == nil && (h.Schema.Family == "bounded-retained-metrics" || h.Schema.Family == "bounded-retained-ranking") {
			bounded = true
		}
	}
	if bounded {
		if err := boundedanalysis.Preflight(input, boundedanalysis.MaxBytes); err != nil {
			return err
		}
	}
	schemaID := v.schemaIDs[name]
	if schemaID == "" {
		return fmt.Errorf("unknown offline operation %q", name)
	}
	return ValidateJSON(schemaID, input)
}
