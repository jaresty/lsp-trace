package mcpcontract

import (
	"encoding/json"
	"fmt"

	"lsp-trace/internal/operation"
)

// OperationInputValidator binds transport-neutral operation names to the exact
// input schemas selected by the accepted MCP manifest.
type OperationInputValidator struct {
	schemaIDs map[operation.Name]string
}

// NewOperationInputValidator constructs an immutable manifest-backed validator
// for the six Stage 1 offline operations and activated incoming traversal.
func NewOperationInputValidator() (*OperationInputValidator, error) {
	manifest, err := LoadManifest()
	if err != nil {
		return nil, err
	}
	canonical := map[operation.Name]string{
		operation.Capabilities: "lsp_trace_v1_capabilities",
		operation.SchemaGet:    "lsp_trace_v1_schema_get",
		operation.Validate:     "lsp_trace_v1_validate",
		operation.Verify:       "lsp_trace_v1_verify",
		operation.Inspect:      "lsp_trace_v1_inspect",
		operation.Filter:       "lsp_trace_v1_filter",
	}
	schemaIDs := make(map[operation.Name]string, len(canonical))
	for name, toolName := range canonical {
		for _, tool := range manifest.Tools {
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
	schemaID := v.schemaIDs[name]
	if schemaID == "" {
		return fmt.Errorf("unknown offline operation %q", name)
	}
	return ValidateJSON(schemaID, input)
}
