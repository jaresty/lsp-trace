package mcpcontract

const ExecuteGatewayEnvelopeID = "https://jaresty.github.io/lsp-trace/mcp/schemas/envelope-execute-gateway.v1.schema.json"

// WithExecuteGateway registers the current-only execute gateway envelope while
// preserving the embedded historical Stage 1 manifest and execution v1 schemas.
func WithExecuteGateway(m *Manifest) *Manifest {
	copy := *m
	copy.Schemas = append([]SchemaRegistration{}, m.Schemas...)
	copy.Tools = append([]ToolContract{}, m.Tools...)
	copy.Schemas = append(copy.Schemas, SchemaRegistration{
		ID: ExecuteGatewayEnvelopeID, Family: "envelope-execute-gateway.v1", Layer: "envelope", Path: "schemas/envelope-execute-gateway.v1.schema.json",
	})
	for i := range copy.Tools {
		if copy.Tools[i].Name == "lsp_trace_v1_execute" {
			copy.Tools[i].EnvelopeSchemaIDs = append([]string{}, copy.Tools[i].EnvelopeSchemaIDs...)
			copy.Tools[i].EnvelopeSchemaIDs = append(copy.Tools[i].EnvelopeSchemaIDs, ExecuteGatewayEnvelopeID)
			break
		}
	}
	return &copy
}
