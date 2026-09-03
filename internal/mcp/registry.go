package mcp

import (
	"encoding/json"
	"sort"

	"lsp-trace/internal/mcpcontract"
)

const (
	publicationEnvelopeSchemaID      = "https://jaresty.github.io/lsp-trace/mcp/schemas/envelope-publication.v1.schema.json"
	publicationErrorEnvelopeSchemaID = "https://jaresty.github.io/lsp-trace/mcp/schemas/envelope-publication-error.v1.schema.json"
	inlineByteLimit                  = 1048576
)

// Availability is the immutable process-lifetime availability of a tool.
type Availability string

const (
	NotImplemented         Availability = "NOT_IMPLEMENTED"
	ContainmentUnavailable Availability = "CONTAINMENT_UNAVAILABLE"
	RuntimeDisabled        Availability = "RUNTIME_DISABLED"
	Enabled                Availability = "ENABLED"
)

type Tool struct {
	Name              string         `json:"name"`
	Aliases           []string       `json:"aliases"`
	InputSchemaID     string         `json:"input_schema_id"`
	EnvelopeSchemaIDs []string       `json:"envelope_schema_ids"`
	ArtifactSchemaIDs []string       `json:"artifact_schema_ids"`
	Availability      Availability   `json:"availability"`
	Description       string         `json:"-"`
	InputSchema       map[string]any `json:"-"`
}

type Registry struct {
	tools                []Tool
	byName               map[string]int
	publicationSupported bool
}

func NewRegistry(enableLiveLSP bool) *Registry {
	return NewRegistryWithPublication(enableLiveLSP, false)
}

func NewRegistryWithPublication(_ bool, publicationSupported bool) *Registry {
	manifest, err := mcpcontract.LoadManifest()
	if err != nil {
		panic("embedded MCP contract is invalid: " + err.Error())
	}
	descriptions := map[string]string{
		"lsp_trace_v1_inspect": "Inspect retained evidence", "lsp_trace_v1_filter": "Filter retained evidence",
		"lsp_trace_v1_validate": "Validate retained evidence", "lsp_trace_v1_verify": "Verify retained evidence",
		"lsp_trace_v1_schema_get": "Retrieve an evidence schema", "lsp_trace_v1_capabilities": "Discover LSP Trace MCP capabilities",
	}
	tools := make([]Tool, 0, len(manifest.Tools))
	for _, contract := range manifest.Tools {
		raw, err := mcpcontract.SchemaJSON(contract.InputSchemaID)
		if err != nil {
			panic("embedded MCP input schema is invalid: " + err.Error())
		}
		var inputSchema map[string]any
		if err := json.Unmarshal(raw, &inputSchema); err != nil {
			panic("embedded MCP input schema is invalid: " + err.Error())
		}
		envelopeSchemaIDs := append([]string{}, contract.EnvelopeSchemaIDs...)
		if !publicationSupported {
			envelopeSchemaIDs = withoutPublicationEnvelopes(envelopeSchemaIDs)
		}
		tools = append(tools, Tool{
			Name: contract.Name, Aliases: append([]string{}, contract.Aliases...), InputSchemaID: contract.InputSchemaID,
			EnvelopeSchemaIDs: envelopeSchemaIDs, ArtifactSchemaIDs: append([]string{}, contract.ArtifactSchemaIDs...),
			Availability: Availability(contract.Availability), Description: descriptions[contract.Name], InputSchema: inputSchema,
		})
	}
	sort.Slice(tools, func(i, j int) bool { return tools[i].Name < tools[j].Name })
	r := &Registry{tools: tools, byName: make(map[string]int, len(tools)*2), publicationSupported: publicationSupported}
	for i := range tools {
		r.byName[tools[i].Name] = i
		r.byName[tools[i].Aliases[0]] = i
	}
	return r
}

func withoutPublicationEnvelopes(ids []string) []string {
	out := ids[:0]
	for _, id := range ids {
		if id != publicationEnvelopeSchemaID && id != publicationErrorEnvelopeSchemaID {
			out = append(out, id)
		}
	}
	return out
}

func (r *Registry) Tools() []Tool { return append([]Tool(nil), r.tools...) }

// Capabilities returns immutable Stage 1 metadata for every canonical enabled
// or reserved tool. Selector publication reflects process-lifetime root configuration.
func (r *Registry) Capabilities() map[string]any {
	return map[string]any{
		"capabilities_version": "1", "selected_envelope_version": "1", "supported_envelope_versions": []string{"1"},
		"tools": r.Tools(), "selector_publication_supported": r.publicationSupported,
		"inline_byte_limit": uint64(inlineByteLimit), "list_page_max": uint32(100),
	}
}

func (r *Registry) Advertised() []Tool {
	out := make([]Tool, 0, len(r.tools))
	for _, tool := range r.tools {
		if tool.Availability == Enabled {
			out = append(out, tool)
		}
	}
	return out
}

func (r *Registry) Resolve(name string) (Tool, bool) {
	i, ok := r.byName[name]
	if !ok {
		return Tool{}, false
	}
	return r.tools[i], true
}
