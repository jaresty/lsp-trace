package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

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

type ExecutorFamily string

const (
	OfflineExecutorFamily   ExecutorFamily = "offline"
	LifecycleExecutorFamily ExecutorFamily = "lifecycle"
)

// SemanticValidator runs after structural schema validation and before dispatch.
type SemanticValidator func(context.Context, Tool, map[string]any) error

// Routing computes immutable process-lifetime metadata while the registry is built.
// Its callbacks are invoked during construction and are not retained.
type Routing struct {
	Availability      func(Tool) Availability
	Aliases           func(Tool) []string
	SemanticValidator func(Tool) SemanticValidator
	ExecutorFamily    func(Tool) ExecutorFamily
}

type Tool struct {
	Name              string         `json:"name"`
	Aliases           []string       `json:"aliases"`
	InputSchemaID     string         `json:"input_schema_id"`
	EnvelopeSchemaIDs []string       `json:"envelope_schema_ids"`
	ArtifactSchemaIDs []string       `json:"artifact_schema_ids"`
	Availability      Availability   `json:"availability"`
	Description       string         `json:"-"`
	InputSchema       map[string]any `json:"-"`
	ExecutorFamily    ExecutorFamily `json:"-"`
	semanticValidator SemanticValidator
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
	return NewRegistryWithRouting(publicationSupported, Routing{})
}

func NewRegistryWithRouting(publicationSupported bool, routing Routing) *Registry {
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
		executorFamily := OfflineExecutorFamily
		if strings.HasPrefix(contract.Name, "lsp_session_v1_") {
			executorFamily = LifecycleExecutorFamily
		}
		tools = append(tools, Tool{
			Name: contract.Name, Aliases: append([]string{}, contract.Aliases...), InputSchemaID: contract.InputSchemaID,
			EnvelopeSchemaIDs: envelopeSchemaIDs, ArtifactSchemaIDs: append([]string{}, contract.ArtifactSchemaIDs...),
			Availability: Availability(contract.Availability), Description: descriptions[contract.Name], InputSchema: inputSchema,
			ExecutorFamily: executorFamily,
		})
	}
	for i := range tools {
		base := cloneTool(tools[i])
		// Stage 2 is an always-local development contract. Lifecycle tools are
		// enabled by default; the two live traversal tools remain reserved.
		if tools[i].ExecutorFamily == LifecycleExecutorFamily {
			tools[i].Availability = Enabled
			tools[i].InputSchema = lifecycleInputSchema(tools[i].Name)
			tools[i].EnvelopeSchemaIDs = []string{resultEnvelopeSchemaID, domainEnvelopeSchemaID}
		}
		if routing.Availability != nil {
			tools[i].Availability = routing.Availability(base)
		}
		if routing.Aliases != nil {
			tools[i].Aliases = append([]string(nil), routing.Aliases(base)...)
		}
		if routing.SemanticValidator != nil {
			tools[i].semanticValidator = routing.SemanticValidator(base)
		}
		if routing.ExecutorFamily != nil {
			tools[i].ExecutorFamily = routing.ExecutorFamily(base)
			if tools[i].ExecutorFamily == "" {
				tools[i].ExecutorFamily = OfflineExecutorFamily
			}
		}
	}
	sort.Slice(tools, func(i, j int) bool { return tools[i].Name < tools[j].Name })
	r := &Registry{tools: tools, byName: make(map[string]int, len(tools)*2), publicationSupported: publicationSupported}
	for i := range tools {
		names := append([]string{tools[i].Name}, tools[i].Aliases...)
		for _, name := range names {
			if name == "" {
				panic("MCP registry contains an empty canonical name or alias")
			}
			if prior, exists := r.byName[name]; exists && prior != i {
				panic(fmt.Sprintf("MCP registry name %q is ambiguous", name))
			}
			r.byName[name] = i
		}
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

func lifecycleInputSchema(name string) map[string]any {
	properties := map[string]any{}
	required := []any{}
	if name != "lsp_session_v1_list" {
		properties = map[string]any{
			"session_id": map[string]any{"type": "string", "minLength": 1},
			"generation": map[string]any{"type": "integer", "minimum": 1},
		}
		required = []any{"session_id", "generation"}
		if name != "lsp_session_v1_status" {
			properties["caller_id"] = map[string]any{"type": "string", "minLength": 1}
			required = append(required, "caller_id")
		}
	}
	return map[string]any{"type": "object", "additionalProperties": false, "properties": properties, "required": required}
}

func cloneTool(tool Tool) Tool {
	tool.Aliases = append([]string{}, tool.Aliases...)
	tool.EnvelopeSchemaIDs = append([]string{}, tool.EnvelopeSchemaIDs...)
	tool.ArtifactSchemaIDs = append([]string{}, tool.ArtifactSchemaIDs...)
	tool.InputSchema = cloneMap(tool.InputSchema)
	return tool
}

func cloneMap(input map[string]any) map[string]any {
	if input == nil {
		return nil
	}
	output := make(map[string]any, len(input))
	for key, value := range input {
		output[key] = cloneValue(value)
	}
	return output
}

func cloneValue(value any) any {
	switch value := value.(type) {
	case map[string]any:
		return cloneMap(value)
	case []any:
		output := make([]any, len(value))
		for i := range value {
			output[i] = cloneValue(value[i])
		}
		return output
	default:
		return value
	}
}

func (r *Registry) Tools() []Tool {
	tools := make([]Tool, len(r.tools))
	for i := range r.tools {
		tools[i] = cloneTool(r.tools[i])
	}
	return tools
}

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
	return cloneTool(r.tools[i]), true
}
