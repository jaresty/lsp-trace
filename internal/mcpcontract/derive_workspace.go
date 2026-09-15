package mcpcontract

const (
	DeriveWorkspaceTool          = "lsp_session_v1_derive_workspace"
	DeriveWorkspaceInputID       = "https://jaresty.github.io/lsp-trace/mcp/schemas/input-session-derive-workspace.v1.schema.json"
	DeriveWorkspaceSuccessID     = "https://jaresty.github.io/lsp-trace/mcp/schemas/envelope-session-derive-workspace-result.v1.schema.json"
	DeriveWorkspaceDomainErrorID = "https://jaresty.github.io/lsp-trace/mcp/schemas/envelope-session-derive-workspace-domain-error.v1.schema.json"
)

func WithDeriveWorkspace(manifest *Manifest) *Manifest {
	copy := *manifest
	copy.Schemas = append([]SchemaRegistration(nil), manifest.Schemas...)
	copy.Tools = append([]ToolContract(nil), manifest.Tools...)
	copy.Transcripts = append([]string(nil), manifest.Transcripts...)
	out := &copy
	out.Schemas = append(out.Schemas,
		SchemaRegistration{ID: DeriveWorkspaceInputID, Family: "input-session-derive-workspace.v1", Layer: "input", Path: "operation42schemas/input-session-derive-workspace.v1.schema.json"},
		SchemaRegistration{ID: DeriveWorkspaceSuccessID, Family: "envelope-session-derive-workspace-result.v1", Layer: "envelope", Path: "operation42schemas/envelope-session-derive-workspace-result.v1.schema.json"},
		SchemaRegistration{ID: DeriveWorkspaceDomainErrorID, Family: "envelope-session-derive-workspace-domain-error.v1", Layer: "envelope", Path: "operation42schemas/envelope-session-derive-workspace-domain-error.v1.schema.json"},
	)
	out.Tools = append(out.Tools, ToolContract{Name: DeriveWorkspaceTool, Aliases: []string{}, InputSchemaID: DeriveWorkspaceInputID, EnvelopeSchemaIDs: []string{DeriveWorkspaceSuccessID, DeriveWorkspaceDomainErrorID}, Advertised: true, Availability: "ENABLED"})
	return out
}
