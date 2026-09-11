package mcpcontract

const ExecuteGatewayEnvelopeID = "https://jaresty.github.io/lsp-trace/mcp/schemas/envelope-execute-gateway.v1.schema.json"

const (
	CustodyExecuteTool  = "lsp_trace_v1_custody_execute"
	CustodyExecuteAlias = "lsp_trace_custody_execute"
)

// WithExecuteGateway splits the historical custody executor from the generic
// dispatcher. The custody contract keeps its artifact and envelope schemas
// under a new canonical identity; execute owns only the gateway envelope.
func WithExecuteGateway(m *Manifest) *Manifest {
	copy := *m
	copy.Schemas = append([]SchemaRegistration{}, m.Schemas...)
	copy.Tools = append([]ToolContract{}, m.Tools...)
	copy.Schemas = append(copy.Schemas, SchemaRegistration{
		ID: ExecuteGatewayEnvelopeID, Family: "envelope-execute-gateway.v1", Layer: "envelope", Path: "schemas/envelope-execute-gateway.v1.schema.json",
	})
	for i := range copy.Tools {
		if copy.Tools[i].Name != "lsp_trace_v1_execute" {
			continue
		}
		custody := copy.Tools[i]
		custody.Name = CustodyExecuteTool
		custody.Aliases = []string{CustodyExecuteAlias}
		custody.EnvelopeSchemaIDs = append([]string{}, custody.EnvelopeSchemaIDs...)
		custody.ArtifactSchemaIDs = append([]string{}, custody.ArtifactSchemaIDs...)

		gateway := copy.Tools[i]
		gateway.Aliases = []string{"lsp_trace_execute"}
		gateway.EnvelopeSchemaIDs = []string{ExecuteGatewayEnvelopeID}
		gateway.ArtifactSchemaIDs = []string{}
		copy.Tools[i] = gateway
		copy.Tools = append(copy.Tools, custody)
		break
	}
	return &copy
}
