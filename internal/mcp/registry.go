package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"lsp-trace/internal/mcpcontract"
	"lsp-trace/internal/provider"
)

const (
	publicationEnvelopeSchemaID      = "https://jaresty.github.io/lsp-trace/mcp/schemas/envelope-publication.v1.schema.json"
	publicationErrorEnvelopeSchemaID = "https://jaresty.github.io/lsp-trace/mcp/schemas/envelope-publication-error.v1.schema.json"
	inlineByteLimit                  = 1048576
	graphV4ArtifactSchemaID          = "https://jaresty.github.io/lsp-trace/schemas/lsp-trace.graph.v4.schema.json"
	incomingCompositionSchemaID      = "https://jaresty.github.io/lsp-trace/schemas/lsp-trace.incoming-composition.v1.schema.json"
	sliceCompositionSchemaID         = "https://jaresty.github.io/lsp-trace/schemas/lsp-trace.slice-composition.v1.schema.json"
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
	OfflineExecutorFamily       ExecutorFamily = "offline"
	LifecycleExecutorFamily     ExecutorFamily = "lifecycle"
	IncomingExecutorFamily      ExecutorFamily = "incoming"
	SliceExecutorFamily         ExecutorFamily = "slice"
	AcquisitionV2ExecutorFamily ExecutorFamily = "acquisition-v2"
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
	providerInventory    provider.ConfiguredInventory
}

func NewRegistry(enableLiveLSP bool) *Registry {
	return NewRegistryWithPublication(enableLiveLSP, false)
}

func NewRegistryWithPublication(_ bool, publicationSupported bool) *Registry {
	return NewRegistryWithRouting(publicationSupported, Routing{})
}

func NewRegistryWithProviderInventory(_ bool, publicationSupported bool, inventory provider.ConfiguredInventory) *Registry {
	registry := NewRegistryWithRouting(publicationSupported, Routing{})
	registry.providerInventory = inventory
	return registry
}

func NewRegistryWithRouting(publicationSupported bool, routing Routing) *Registry {
	manifest, err := mcpcontract.LoadManifest()
	if err != nil {
		panic("embedded MCP contract is invalid: " + err.Error())
	}
	manifest = mcpcontract.WithAcquisitionV3(mcpcontract.WithRetainedCallsV2Verifier(mcpcontract.WithRetainedCallsV2Export(mcpcontract.WithHydratedInspection(mcpcontract.WithRetainedCalls(manifest)))))
	descriptions := map[string]string{
		mcpcontract.HydratedTool:                 "Inspect exact retained node/relation context offline with explicit focus dispositions and body opt-in; no source acquisition or publication",
		"lsp_trace_v2_verify":                    "Verify exact immutable selected-publication bytes under explicit graph-provenance/v2 admission; consistency is not producer authentication",
		"lsp_trace_v2_verify_retained_calls":     "Verify immutable selected-publication custody before explicit retained-calls/v2 admission; consistency is not producer authentication",
		"lsp_trace_v1_bounded_retained_ranking":  "Bounded PageRank or exact-seed PPR over admitted historical retained unit CALLS groups; not source completeness or authentication",
		"lsp_trace_v1_bounded_retained_metrics":  "Compute structural group degrees, histograms and exact directed density offline over admitted historical retained CALLS; not source-complete or authenticated",
		"lsp_trace_v1_bounded_retained_analysis": "Project retained CALLS, find bounded directed shortest paths, or explicit WEAK/STRONG components offline; unverified historical scope, not normative Program B",
		"lsp_trace_v1_export_retained_calls":     "Export distinct retained CALLS callsites offline with historical group and source provenance; not acquisition events",
		"lsp_trace_v2_export_retained_calls":     "Export admitted graph-provenance/v2 as retained-calls/v2 offline; source implemented, not deployed qualification",
		"lsp_trace_v1_inspect":                   "Inspect retained evidence for one seed or all retained seeds without changing authority",
		"lsp_trace_v1_filter":                    "Compare exactly two retained seed evidence sets with a mechanical filter",
		"lsp_trace_v1_validate":                  "Validate retained evidence against its schema contract",
		"lsp_trace_v1_verify":                    "Verify immutable publication custody, byte length, and digest",
		"lsp_trace_v1_schema_get":                "Retrieve the exact schema contract for an evidence family and version",
		"lsp_trace_v1_capabilities":              "Discover canonical LSP Trace tools, schemas, publication support, and limits",
		"lsp_trace_v1_execute":                   "Execute one canonical request through the shared transport-neutral operation",
		"lsp_trace_v2_slice":                     "Acquire ordered required targets with shared limits and retained directed witnesses; source implementation, not deployed qualification or analyzed-source authentication",
		"lsp_trace_v2_incoming":                  "Acquire ordered required callers with shared limits and required-to-root witnesses; source implementation, not deployed qualification or analyzed-source authentication",
		"lsp_trace_v3_slice":                     "Acquire graph-provenance/v3 with exact embedded V2 bytes and bounded managed diagnostics for one exact session generation",
		"lsp_trace_v3_incoming":                  "Acquire incoming graph-provenance/v3 with exact embedded V2 bytes and bounded managed diagnostics for one exact session generation",
		"lsp_trace_v1_incoming":                  "Answer who calls this exact callee by tracing bounded incoming calls in a managed local language-server session",
		"lsp_trace_v1_slice":                     "Explore a bounded outgoing call frontier, then trace incoming callers from its exact frontier and leaves",
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
		} else if contract.Name == "lsp_trace_v1_incoming" {
			executorFamily = IncomingExecutorFamily
		} else if contract.Name == "lsp_trace_v1_slice" {
			executorFamily = SliceExecutorFamily
		} else if contract.Name == "lsp_trace_v2_slice" || contract.Name == "lsp_trace_v2_incoming" || contract.Name == "lsp_trace_v3_slice" || contract.Name == "lsp_trace_v3_incoming" {
			executorFamily = AcquisitionV2ExecutorFamily
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
		// Always-local lifecycle and traversal operations are enabled by default.
		if tools[i].ExecutorFamily == LifecycleExecutorFamily {
			tools[i].Availability = Enabled
			tools[i].Description = lifecycleDescription(tools[i].Name)
			tools[i].InputSchema = lifecycleInputSchema(tools[i].Name)
			tools[i].EnvelopeSchemaIDs = []string{resultEnvelopeSchemaID, domainEnvelopeSchemaID}
		}
		if tools[i].ExecutorFamily == IncomingExecutorFamily {
			tools[i].Availability = Enabled
			tools[i].InputSchema = incomingInputSchema()
			tools[i].EnvelopeSchemaIDs = traversalEnvelopeSchemaIDs(publicationSupported)
			tools[i].ArtifactSchemaIDs = []string{"https://jaresty.github.io/lsp-trace/schemas/lsp-trace.graph.v3.schema.json", graphV4ArtifactSchemaID, incomingCompositionSchemaID}
		}
		if tools[i].ExecutorFamily == SliceExecutorFamily {
			tools[i].EnvelopeSchemaIDs = traversalEnvelopeSchemaIDs(publicationSupported)
			tools[i].ArtifactSchemaIDs = appendUnique(tools[i].ArtifactSchemaIDs, graphV4ArtifactSchemaID)
			tools[i].ArtifactSchemaIDs = appendUnique(tools[i].ArtifactSchemaIDs, sliceCompositionSchemaID)
			tools[i].ArtifactSchemaIDs = appendUnique(tools[i].ArtifactSchemaIDs, "https://jaresty.github.io/lsp-trace/schemas/lsp-trace.graph-provenance.v1.schema.json")
		}
		if tools[i].ExecutorFamily == IncomingExecutorFamily || tools[i].ExecutorFamily == SliceExecutorFamily {
			addNormalizedProviderInputProperties(tools[i].InputSchema)
		}
		if tools[i].Name == "lsp_trace_v1_schema_get" || tools[i].Name == "lsp_trace_v1_validate" {
			tools[i].ArtifactSchemaIDs = appendUnique(tools[i].ArtifactSchemaIDs, mcpcontract.GraphProvenanceV2ArtifactID)
			tools[i].ArtifactSchemaIDs = appendUnique(tools[i].ArtifactSchemaIDs, mcpcontract.GraphProvenanceV3ArtifactID)
			tools[i].ArtifactSchemaIDs = appendUnique(tools[i].ArtifactSchemaIDs, mcpcontract.RetainedCallsArtifactID)
			tools[i].ArtifactSchemaIDs = appendUnique(tools[i].ArtifactSchemaIDs, mcpcontract.RetainedCallsV2ArtifactID)
			tools[i].ArtifactSchemaIDs = appendUnique(tools[i].ArtifactSchemaIDs, mcpcontract.BoundedAnalysisArtifactID)
			tools[i].ArtifactSchemaIDs = appendUnique(tools[i].ArtifactSchemaIDs, mcpcontract.BoundedMetricsArtifactID)
			tools[i].ArtifactSchemaIDs = appendUnique(tools[i].ArtifactSchemaIDs, mcpcontract.BoundedRankingArtifactID)
			tools[i].ArtifactSchemaIDs = appendUnique(tools[i].ArtifactSchemaIDs, graphV4ArtifactSchemaID)
			tools[i].ArtifactSchemaIDs = appendUnique(tools[i].ArtifactSchemaIDs, "https://jaresty.github.io/lsp-trace/schemas/lsp-trace.operational-custody.v1.schema.json")
			tools[i].ArtifactSchemaIDs = appendUnique(tools[i].ArtifactSchemaIDs, "https://jaresty.github.io/lsp-trace/schemas/lsp-trace.graph-provenance.v1.schema.json")
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

func appendUnique(ids []string, id string) []string {
	for _, candidate := range ids {
		if candidate == id {
			return ids
		}
	}
	return append(ids, id)
}

func withoutPublicationEnvelopes(ids []string) []string {
	out := ids[:0]
	for _, id := range ids {
		if id == mcpcontract.AcquisitionV2EnvelopeID(publicationEnvelopeSchemaID) || id == mcpcontract.AcquisitionV2EnvelopeID(compactEnvelopeSchemaID) || id == mcpcontract.AcquisitionV2EnvelopeID(publicationErrorEnvelopeSchemaID) {
			continue
		}
		if id == mcpcontract.BoundedRankingEnvelopeID(publicationEnvelopeSchemaID) || id == mcpcontract.BoundedRankingEnvelopeID(compactEnvelopeSchemaID) || id == mcpcontract.BoundedRankingEnvelopeID(publicationErrorEnvelopeSchemaID) {
			continue
		}
		if id == mcpcontract.BoundedMetricsEnvelopeID(publicationEnvelopeSchemaID) || id == mcpcontract.BoundedMetricsEnvelopeID(compactEnvelopeSchemaID) || id == mcpcontract.BoundedMetricsEnvelopeID(publicationErrorEnvelopeSchemaID) {
			continue
		}
		if id == mcpcontract.BoundedAnalysisEnvelopeID(publicationEnvelopeSchemaID) || id == mcpcontract.BoundedAnalysisEnvelopeID(compactEnvelopeSchemaID) || id == mcpcontract.BoundedAnalysisEnvelopeID(publicationErrorEnvelopeSchemaID) {
			continue
		}
		if id != publicationEnvelopeSchemaID && id != compactEnvelopeSchemaID && id != publicationErrorEnvelopeSchemaID && id != mcpcontract.RetainedCallsEnvelopeID(publicationEnvelopeSchemaID) && id != mcpcontract.RetainedCallsEnvelopeID(compactEnvelopeSchemaID) && id != mcpcontract.RetainedCallsEnvelopeID(publicationErrorEnvelopeSchemaID) && id != mcpcontract.RetainedCallsV2EnvelopeID(publicationEnvelopeSchemaID) && id != mcpcontract.RetainedCallsV2EnvelopeID(compactEnvelopeSchemaID) && id != mcpcontract.RetainedCallsV2EnvelopeID(publicationErrorEnvelopeSchemaID) {
			out = append(out, id)
		}
	}
	return out
}

func lifecycleDescription(name string) string {
	switch name {
	case "lsp_session_v1_list":
		return "Discover host-provisioned local language-server sessions and exact generations"
	case "lsp_session_v1_status":
		return "Read the current observed state of one local language-server session generation"
	case "lsp_session_v1_stop":
		return "Request the host runtime to stop one exact local language-server session generation"
	case "lsp_session_v1_restart":
		return "Request the host runtime to restart one exact local language-server session generation"
	default:
		return ""
	}
}

func traversalEnvelopeSchemaIDs(publicationSupported bool) []string {
	ids := []string{artifactEnvelopeSchemaID, publicationEnvelopeSchemaID, compactEnvelopeSchemaID, publicationErrorEnvelopeSchemaID, domainEnvelopeSchemaID}
	if !publicationSupported {
		ids = withoutPublicationEnvelopes(ids)
	}
	return ids
}

func incomingInputSchema() map[string]any {
	return map[string]any{
		"$schema": "https://json-schema.org/draft/2020-12/schema",
		"type":    "object", "additionalProperties": false,
		"properties": map[string]any{
			"session_id": map[string]any{"type": "string", "minLength": 1}, "generation": map[string]any{"type": "integer", "minimum": 1},
			"uri": map[string]any{"type": "string", "minLength": 1, "format": "uri"}, "line": map[string]any{"type": "integer", "minimum": 0},
			"character": map[string]any{"type": "integer", "minimum": 0}, "symbol": map[string]any{"type": "string", "minLength": 1},
			"detail": map[string]any{"type": "string", "enum": []any{"compact", "full"}}, "output_selector": map[string]any{"type": "string", "minLength": 1},
			"max_depth": map[string]any{"type": "integer", "minimum": 1, "maximum": 64}, "max_nodes": map[string]any{"type": "integer", "minimum": 1, "maximum": 10000},
			"timeout_ms": map[string]any{"type": "integer", "minimum": 1, "maximum": 60000}, "request_timeout_ms": map[string]any{"type": "integer", "minimum": 1, "maximum": 60000},
		},
		"required": []any{"session_id", "uri"},
		"oneOf": []any{
			map[string]any{"required": []any{"line", "character"}, "not": map[string]any{"required": []any{"symbol"}}},
			map[string]any{"required": []any{"symbol"}, "not": map[string]any{"anyOf": []any{map[string]any{"required": []any{"line"}}, map[string]any{"required": []any{"character"}}}}},
		},
	}
}

func addNormalizedProviderInputProperties(input map[string]any) {
	properties, ok := input["properties"].(map[string]any)
	if !ok {
		panic("MCP traversal input schema has no properties object")
	}
	properties["relations"] = map[string]any{
		"type": "array", "uniqueItems": true, "minItems": 1,
		"items": map[string]any{"type": "string", "enum": []any{"CALLS", "BINDS_ARGUMENT", "PASSES_CALLBACK", "INVOKES_TASK", "TRIGGERS_RELOAD", "UPDATES_STATE", "RENDERS_FROM"}},
	}
	properties["adapters"] = map[string]any{"oneOf": []any{
		map[string]any{"const": "auto"},
		map[string]any{"type": "array", "uniqueItems": true, "items": map[string]any{"type": "string", "minLength": 1}},
	}}
	properties["providers"] = map[string]any{"type": "array", "uniqueItems": true, "items": map[string]any{"type": "string", "minLength": 1}}
	properties["workspace_revision"] = map[string]any{
		"type": "object", "additionalProperties": false,
		"properties": map[string]any{
			"kind":    map[string]any{"type": "string", "minLength": 1},
			"commit":  map[string]any{"type": "string", "minLength": 1},
			"custody": map[string]any{"type": "string", "enum": []any{"CALLER_ASSERTED", "PROVIDER_VERIFIED", "UNKNOWN"}},
		},
		"required": []any{"kind", "commit", "custody"},
	}
	properties["fail_on_unknown_revision"] = map[string]any{"type": "boolean"}
}

func lifecycleInputSchema(name string) map[string]any {
	properties := map[string]any{}
	required := []any{}
	if name != "lsp_session_v1_list" {
		properties = map[string]any{
			"session_id": map[string]any{"type": "string", "minLength": 1},
			"generation": map[string]any{"type": "integer", "minimum": 1},
		}
		required = []any{"session_id"}
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
		"configured_providers": r.providerInventory.Entries(),
		"acquisition_v2": map[string]any{
			"contract_version": "lsp-trace.public-acquisition.v2", "default_acquisition_version": "v1",
			"source_implementation": "EXPERIMENTAL", "deployed_availability": "UNKNOWN",
			"producers":       []string{"lsp_trace_v2_slice", "lsp_trace_v2_incoming"},
			"cli_producers":   []string{"slice --acquisition-version v2", "incoming --acquisition-version v2"},
			"input_schema_id": mcpcontract.AcquisitionV2InputID, "output_schema_id": mcpcontract.GraphProvenanceV2ArtifactID,
			"public_consumers": []string{"validate --family graph-provenance --version v2", "verify --family graph-provenance --version v2", "lsp_trace_v1_validate (explicit graph-provenance/v2)", "lsp_trace_v2_verify"},
			"public_export": map[string]any{
				"source_implementation": "IMPLEMENTED", "deployed_availability": "UNKNOWN",
				"mcp_tool": "lsp_trace_v2_export_retained_calls", "cli": "export-retained-calls --version v2",
			},
			"public_analysis":       "NOT_IMPLEMENTED",
			"coordinate_convention": "zero-based-session", "max_targets": 64, "max_input_bytes": 262144,
			"authority": "EXACT_HOST_SESSION_GENERATION_WORKSPACE", "analyzed_source": "UNVERIFIED",
		},
		"inline_byte_limit": uint64(inlineByteLimit), "list_page_max": uint32(100),
		"normalized_relations": map[string]any{
			"kinds":                []string{"CALLS", "BINDS_ARGUMENT", "PASSES_CALLBACK", "INVOKES_TASK", "TRIGGERS_RELOAD", "UPDATES_STATE", "RENDERS_FROM"},
			"default_when_omitted": "CALLS_ONLY", "provider_authority": "HOST_PROVISIONED_ONLY",
		},
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
