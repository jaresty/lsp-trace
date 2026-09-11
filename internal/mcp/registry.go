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
	inspectAncillaryArtifactSchemaID = "https://jaresty.github.io/lsp-trace/schemas/lsp-trace.inspect-ancillary.v1.schema.json"
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
	Name                    string         `json:"name"`
	Aliases                 []string       `json:"aliases"`
	InputSchemaID           string         `json:"input_schema_id"`
	EnvelopeSchemaIDs       []string       `json:"envelope_schema_ids"`
	ArtifactSchemaIDs       []string       `json:"artifact_schema_ids"`
	OutputFamiliesVersions  []string       `json:"-"`
	Availability            Availability   `json:"availability"`
	Description             string         `json:"-"`
	InputSchema             map[string]any `json:"-"`
	PresentationInputSchema map[string]any `json:"-"`
	ExecutorFamily          ExecutorFamily `json:"-"`
	semanticValidator       SemanticValidator
}

// ToolProfile is the closed, immutable process-lifetime MCP advertisement profile.
type ToolProfile string

const (
	ToolProfileFull    ToolProfile = "full"
	ToolProfileCompact ToolProfile = "compact"
)

var compactToolNames = map[string]struct{}{
	"lsp_session_v1_list": {}, "lsp_session_v1_restart": {}, "lsp_session_v1_status": {}, "lsp_session_v1_stop": {},
	"lsp_trace_v1_capabilities": {}, "lsp_trace_v1_execute": {}, "lsp_trace_v1_incoming": {},
	"lsp_trace_v1_inspect_hydrated": {}, "lsp_trace_v1_schema_get": {}, "lsp_trace_v1_slice": {},
}

type Registry struct {
	tools                []Tool
	byName               map[string]int
	publicationSupported bool
	providerInventory    provider.ConfiguredInventory
	toolProfile          ToolProfile
}

func NewRegistry(enableLiveLSP bool) *Registry {
	return NewRegistryWithPublication(enableLiveLSP, false)
}

func NewRegistryWithProfile(enableLiveLSP bool, profile ToolProfile) *Registry {
	return NewRegistryWithPublicationAndProfile(enableLiveLSP, false, profile)
}

func NewRegistryWithPublication(enableLiveLSP bool, publicationSupported bool) *Registry {
	return NewRegistryWithPublicationAndProfile(enableLiveLSP, publicationSupported, ToolProfileFull)
}

func NewRegistryWithPublicationAndProfile(_ bool, publicationSupported bool, profile ToolProfile) *Registry {
	return newRegistryWithRoutingAndProfile(publicationSupported, Routing{}, profile)
}

func NewRegistryWithProviderInventory(enableLiveLSP bool, publicationSupported bool, inventory provider.ConfiguredInventory) *Registry {
	return NewRegistryWithProviderInventoryAndProfile(enableLiveLSP, publicationSupported, inventory, ToolProfileFull)
}

func NewRegistryWithProviderInventoryAndProfile(_ bool, publicationSupported bool, inventory provider.ConfiguredInventory, profile ToolProfile) *Registry {
	registry := newRegistryWithRoutingAndProfile(publicationSupported, Routing{}, profile)
	registry.providerInventory = inventory
	return registry
}

func NewRegistryWithRouting(publicationSupported bool, routing Routing) *Registry {
	return newRegistryWithRoutingAndProfile(publicationSupported, routing, ToolProfileFull)
}

func newRegistryWithRoutingAndProfile(publicationSupported bool, routing Routing, profile ToolProfile) *Registry {
	if profile != ToolProfileFull && profile != ToolProfileCompact {
		panic(fmt.Sprintf("unknown MCP tool profile %q", profile))
	}
	manifest, err := mcpcontract.LoadManifest()
	if err != nil {
		panic("embedded MCP contract is invalid: " + err.Error())
	}
	manifest = mcpcontract.WithExecuteGateway(mcpcontract.WithPublicAnalyticsV2(mcpcontract.WithAcquisitionV3(mcpcontract.WithRetainedCallsV2Verifier(mcpcontract.WithRetainedCallsV2Export(mcpcontract.WithHydratedInspection(mcpcontract.WithRetainedRelations(mcpcontract.WithRetainedCalls(manifest))))))))
	descriptions := map[string]string{
		mcpcontract.HydratedTool:                 "Inspect exact retained node/relation context offline with explicit focus dispositions and body opt-in; no source acquisition or publication",
		"lsp_trace_v2_verify":                    "Verify exact immutable selected-publication bytes under explicit graph-provenance/v2 admission; consistency is not producer authentication",
		"lsp_trace_v2_verify_retained_calls":     "Verify immutable selected-publication custody before explicit retained-calls/v2 admission; consistency is not producer authentication",
		"lsp_trace_v1_bounded_retained_ranking":  "Bounded PageRank or exact-seed PPR over admitted historical retained unit CALLS groups; not source completeness or authentication",
		"lsp_trace_v1_bounded_retained_metrics":  "Compute structural group degrees, histograms and exact directed density offline over admitted historical retained CALLS; not source-complete or authenticated",
		"lsp_trace_v1_bounded_retained_analysis": "Project retained CALLS, find bounded directed shortest paths, or explicit WEAK/STRONG components offline; unverified historical scope, not normative Program B",
		"lsp_trace_v2_bounded_retained_analysis": "Compute bounded local synthetic root and leaf evidence from exact retained graph bytes; not permission or production authority",
		"lsp_trace_v2_bounded_retained_metrics":  "Compute bounded local synthetic multiplicity, support and density evidence from exact retained graph bytes; not permission or production authority",
		"lsp_trace_v2_bounded_retained_ranking":  "Compute bounded local synthetic support ranking from exact retained graph bytes; not permission or production authority",
		"lsp_trace_v1_export_retained_calls":     "Export distinct retained CALLS callsites offline with historical group and source provenance; not acquisition events",
		"lsp_trace_v2_export_retained_calls":     "Export admitted graph-provenance/v2 as retained-calls/v2 offline; source implemented, not deployed qualification",
		"lsp_trace_v1_inspect":                   "Inspect retained evidence for one seed or all retained seeds without changing authority",
		"lsp_trace_v1_filter":                    "Compare exactly two retained seed evidence sets with a mechanical filter",
		"lsp_trace_v1_validate":                  "Validate retained evidence against its schema contract",
		"lsp_trace_v1_verify":                    "Verify immutable publication custody, byte length, digest, and native Graph Provenance V5 semantics",
		"lsp_trace_v1_schema_get":                "Retrieve the exact schema contract for an evidence family and version",
		"lsp_trace_v1_capabilities":              "Discover canonical LSP Trace tools, schemas, publication support, and limits",
		"lsp_trace_v1_execute":                   "Execute one canonical request through the shared transport-neutral operation",
		"lsp_trace_v2_slice":                     "Graph Provenance V2 output is DEPRECATED; use lsp_trace_v3_slice with output_version=lsp-trace.graph-provenance.v5. Historical V2 dispatch remains compatible",
		"lsp_trace_v2_incoming":                  "Graph Provenance V2 output is DEPRECATED; use lsp_trace_v3_incoming with output_version=lsp-trace.graph-provenance.v5. Historical V2 dispatch remains compatible",
		"lsp_trace_v3_slice":                     "Acquisition route v3 with historical Graph V3 output is DEPRECATED; select output_version=lsp-trace.graph-provenance.v5 for source-qualified Graph Provenance V5 output. Historical v3 dispatch remains compatible",
		"lsp_trace_v3_incoming":                  "Acquisition route v3 with historical Graph V3 output is DEPRECATED; select output_version=lsp-trace.graph-provenance.v5 for source-qualified Graph Provenance V5 output. Historical v3 dispatch remains compatible",
		"lsp_trace_v1_incoming":                  "Prefer this operation to answer who calls an exact callee when a matching managed language-server session is READY; it traces bounded incoming calls",
		"lsp_trace_v1_slice":                     "Prefer this operation to explore an exact target's bounded outgoing call frontier and incoming callers when a matching managed language-server session is READY",
	}
	registeredFamilies := make(map[string]string, len(manifest.Schemas))
	for _, schema := range manifest.Schemas {
		registeredFamilies[schema.ID] = schema.Family
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
			OutputFamiliesVersions: registeredOutputFamilies(contract.ArtifactSchemaIDs, registeredFamilies),
			Availability:           Availability(contract.Availability), Description: descriptions[contract.Name], InputSchema: inputSchema,
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
		if tools[i].ExecutorFamily == SliceExecutorFamily {
			tools[i].PresentationInputSchema = slicePresentationInputSchema(tools[i].InputSchema)
		}
		if operation, ok := publicAnalyticsV2Operation(tools[i].Name); ok {
			tools[i].PresentationInputSchema = publicAnalyticsV2PresentationSchema(tools[i].InputSchema, operation)
		}
		if tools[i].Name == "lsp_trace_v1_verify" {
			tools[i].ArtifactSchemaIDs = appendUnique(tools[i].ArtifactSchemaIDs, mcpcontract.GraphProvenanceV5ArtifactID)
			tools[i].ArtifactSchemaIDs = appendUnique(tools[i].ArtifactSchemaIDs, mcpcontract.GraphV5SourceSnapshotArtifactID)
		}
		if tools[i].Name == "lsp_trace_v1_schema_get" {
			tools[i].EnvelopeSchemaIDs = appendUnique(tools[i].EnvelopeSchemaIDs, resultEnvelopeSchemaID)
		}
		if tools[i].Name == "lsp_trace_v1_schema_get" || tools[i].Name == "lsp_trace_v1_validate" {
			tools[i].ArtifactSchemaIDs = appendUnique(tools[i].ArtifactSchemaIDs, mcpcontract.GraphProvenanceV2ArtifactID)
			tools[i].ArtifactSchemaIDs = appendUnique(tools[i].ArtifactSchemaIDs, mcpcontract.GraphProvenanceV3ArtifactID)
			tools[i].ArtifactSchemaIDs = appendUnique(tools[i].ArtifactSchemaIDs, mcpcontract.GraphProvenanceV5ArtifactID)
			tools[i].ArtifactSchemaIDs = appendUnique(tools[i].ArtifactSchemaIDs, mcpcontract.GraphV5SourceSnapshotArtifactID)
			tools[i].ArtifactSchemaIDs = appendUnique(tools[i].ArtifactSchemaIDs, mcpcontract.RetainedCallsArtifactID)
			tools[i].ArtifactSchemaIDs = appendUnique(tools[i].ArtifactSchemaIDs, mcpcontract.RetainedCallsV2ArtifactID)
			tools[i].ArtifactSchemaIDs = appendUnique(tools[i].ArtifactSchemaIDs, mcpcontract.BoundedAnalysisArtifactID)
			tools[i].ArtifactSchemaIDs = appendUnique(tools[i].ArtifactSchemaIDs, mcpcontract.BoundedMetricsArtifactID)
			tools[i].ArtifactSchemaIDs = appendUnique(tools[i].ArtifactSchemaIDs, mcpcontract.BoundedRankingArtifactID)
			tools[i].ArtifactSchemaIDs = appendUnique(tools[i].ArtifactSchemaIDs, graphV4ArtifactSchemaID)
			tools[i].ArtifactSchemaIDs = appendUnique(tools[i].ArtifactSchemaIDs, inspectAncillaryArtifactSchemaID)
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
		tools[i].OutputFamiliesVersions = registeredOutputFamilies(tools[i].ArtifactSchemaIDs, registeredFamilies)
		tools[i].Description = completeToolDescription(tools[i])
	}
	sort.Slice(tools, func(i, j int) bool { return tools[i].Name < tools[j].Name })
	r := &Registry{tools: tools, byName: make(map[string]int, len(tools)*2), publicationSupported: publicationSupported, toolProfile: profile}
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

func completeToolDescription(tool Tool) string {
	mode := "offline"
	if tool.ExecutorFamily != OfflineExecutorFamily {
		mode = "live local"
	}
	resultFamily := "MCP result envelope"
	if len(tool.ArtifactSchemaIDs) != 0 {
		resultFamily = "MCP result envelope carrying " + strings.Join(tool.ArtifactSchemaIDs, ", ")
	} else if len(tool.EnvelopeSchemaIDs) != 0 {
		resultFamily = "MCP result envelope from " + strings.Join(tool.EnvelopeSchemaIDs, ", ")
	}
	return fmt.Sprintf("%s. This is a %s operation requiring input matching %s and returning a %s. Results are evidence bounded by the named schemas and do not establish source completeness, runtime execution, producer authentication, permission, or production authority. If this direct tool is hidden by the active advertisement profile, call lsp_trace_v1_execute with its canonical request instead", strings.TrimSuffix(tool.Description, "."), mode, tool.InputSchemaID, resultFamily)
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
	if name == "lsp_session_v1_list" {
		properties["uri"] = map[string]any{"type": "string", "minLength": 1, "format": "uri"}
		properties["detail"] = map[string]any{"type": "string", "enum": []any{"compact", "full"}}
	} else {
		properties = map[string]any{
			"session_id": map[string]any{"type": "string", "minLength": 1, "description": "Exact session ID or unique host-configured alias"},
			"generation": map[string]any{"type": "integer", "minimum": 1},
		}
		required = []any{"session_id"}
		if name == "lsp_session_v1_status" {
			properties["detail"] = map[string]any{"type": "string", "enum": []any{"compact", "full"}}
		}
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
	tool.OutputFamiliesVersions = append([]string{}, tool.OutputFamiliesVersions...)
	tool.InputSchema = cloneMap(tool.InputSchema)
	tool.PresentationInputSchema = cloneMap(tool.PresentationInputSchema)
	return tool
}

func publicAnalyticsV2Operation(name string) (string, bool) {
	switch name {
	case "lsp_trace_v2_bounded_retained_analysis":
		return "ANALYSIS", true
	case "lsp_trace_v2_bounded_retained_metrics":
		return "METRICS", true
	case "lsp_trace_v2_bounded_retained_ranking":
		return "RANKING", true
	default:
		return "", false
	}
}

func slicePresentationInputSchema(canonical map[string]any) map[string]any {
	properties, ok := canonical["properties"].(map[string]any)
	if !ok {
		panic("MCP slice input schema has no properties object")
	}
	variant := func(selectorFields ...string) map[string]any {
		selected := map[string]bool{}
		for _, field := range selectorFields {
			selected[field] = true
		}
		variantProperties := make(map[string]any, len(properties)-3+len(selectorFields))
		for name, property := range properties {
			if name == "line" || name == "character" || name == "symbol" {
				if !selected[name] {
					continue
				}
			}
			variantProperties[name] = cloneValue(property)
		}
		required := []any{"session_id", "start_mode", "uri"}
		for _, field := range selectorFields {
			required = append(required, field)
		}
		return map[string]any{"type": "object", "additionalProperties": false, "properties": variantProperties, "required": required}
	}
	return map[string]any{
		"$schema": "https://json-schema.org/draft/2020-12/schema",
		"type":    "object",
		"oneOf":   []any{variant("line", "character"), variant("symbol")},
	}
}

func publicAnalyticsV2PresentationSchema(canonical map[string]any, operation string) map[string]any {
	properties, _ := canonical["properties"].(map[string]any)
	variant := func(carrier string) map[string]any {
		variantProperties := make(map[string]any, 5)
		for _, name := range []string{"filter", "max_work", "output_selector", carrier} {
			if property, ok := properties[name]; ok {
				variantProperties[name] = cloneValue(property)
			}
		}
		variantProperties["operation"] = map[string]any{"const": operation}
		return map[string]any{
			"type": "object", "additionalProperties": false,
			"properties": variantProperties,
			"required":   []any{"operation", "filter", "max_work", carrier},
		}
	}
	return map[string]any{"type": "object", "oneOf": []any{variant("input"), variant("publication_selector")}}
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
	advertised := r.Advertised()
	advertisedNames := make([]string, len(advertised))
	for i := range advertised {
		advertisedNames[i] = advertised[i].Name
	}
	dispatchableNames := make([]string, len(r.tools))
	for i := range r.tools {
		dispatchableNames[i] = r.tools[i].Name
	}
	return map[string]any{
		"capabilities_version": "1", "selected_envelope_version": "1", "supported_envelope_versions": []string{"1"},
		"active_tool_profile": string(r.toolProfile), "advertised_tool_names": advertisedNames, "dispatchable_tool_names": dispatchableNames,
		"tools": advertised, "selector_publication_supported": r.publicationSupported,
		"operation_discovery": map[string]any{
			"describe_with": "lsp_trace_v1_capabilities", "operation_argument": "operation",
			"compact_tools_are_advertised_only": true, "hidden_operations_remain_dispatchable": true,
			"schema_identity_authority": "describe the operation and use its registered output schema IDs; never infer schema identity from a selector",
		},
		"configured_providers": r.providerInventory.Entries(),
		"managed_session_provisioning": map[string]any{
			"authority":                      "HOST_PROVISIONED_ONLY",
			"bootstrap_option":               "--bootstrap-config",
			"public_template_command":        "lsp-trace-mcp --print-bootstrap-example",
			"caller_create_start_exposed":    false,
			"full_profile_adds_create_start": false,
			"discovery_sequence":             []string{"host configures bootstrap", "server starts trusted processes", "caller lists READY sessions", "caller binds exact session_id and generation", "caller invokes traversal"},
			"environment_policy":             "omit execution.environment unless entries are independently required; private diagnostics are not configuration authority",
		},
		"acquisition_v2": map[string]any{
			"contract_version": "lsp-trace.public-acquisition.v2", "default_acquisition_version": "v1",
			"producers":       []string{"lsp_trace_v2_slice", "lsp_trace_v2_incoming"},
			"cli_producers":   []string{"slice --acquisition-version v2", "incoming --acquisition-version v2"},
			"input_schema_id": mcpcontract.AcquisitionV2InputID, "output_schema_id": mcpcontract.GraphProvenanceV2ArtifactID,
			"public_consumers": []string{"validate --family graph-provenance --version v2", "verify --family graph-provenance --version v2", "lsp_trace_v1_validate (explicit graph-provenance/v2)", "lsp_trace_v2_verify"},
			"public_export": map[string]any{
				"mcp_tool": "lsp_trace_v2_export_retained_calls", "cli": "export-retained-calls --version v2",
			},
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

// DescribeOperation returns static registry-owned metadata for one canonical
// operation or alias. It does not inspect sessions, bootstrap configuration, or
// private publication state.
func (r *Registry) DescribeOperation(name string) (map[string]any, bool) {
	tool, ok := r.Resolve(name)
	if !ok {
		for i := range r.tools {
			candidate := r.tools[i]
			if operationShortName(candidate.Name) != name {
				continue
			}
			if ok {
				return nil, false
			}
			tool, ok = candidate, true
		}
		if !ok {
			return nil, false
		}
	}
	advertised := false
	for _, candidate := range r.Advertised() {
		if candidate.Name == tool.Name {
			advertised = true
			break
		}
	}
	invocationRoute := "lsp_trace_v1_execute"
	if advertised {
		invocationRoute = "direct_or_lsp_trace_v1_execute"
	}
	if tool.Name == "lsp_trace_v1_execute" {
		invocationRoute = "direct"
	}
	description := map[string]any{
		"name":                tool.Name,
		"aliases":             append([]string(nil), tool.Aliases...),
		"description":         tool.Description,
		"input_schema_id":     tool.InputSchemaID,
		"input_schema":        cloneMap(tool.InputSchema),
		"envelope_schema_ids": append([]string(nil), tool.EnvelopeSchemaIDs...),
		"artifact_schema_ids": append([]string(nil), tool.ArtifactSchemaIDs...),
		"advertised":          advertised,
		"dispatchable":        true,
		"availability":        tool.Availability,
		"invocation_route":    invocationRoute,
		"guidance":            operationGuidance(tool, advertised),
	}
	if tool.Name == "lsp_trace_v3_slice" || tool.Name == "lsp_trace_v3_incoming" {
		description["graph_v5_production"] = map[string]any{
			"output_version":                  "lsp-trace.graph-provenance.v5",
			"requires_topmost_siblings":       true,
			"sibling_support_contribution":    0,
			"sibling_relations_are_calls":     false,
			"delegated_output_selector_field": "request.arguments.output_selector",
			"request_template": map[string]any{
				"tool": tool.Name,
				"arguments": map[string]any{
					"session_id": "<ready-session-id>", "generation": 1,
					"seed_manifest": map[string]any{
						"schema_version": "lsp-trace.seed-manifest.v2", "coordinate_convention": "zero-based-session",
						"root":             map[string]any{"id": "root", "locator": map[string]any{"uri": "<file-uri>", "line": 0, "character": 0, "language_id": "<language-id>"}, "down_depth": 1, "up_depth": 1},
						"required_targets": []any{map[string]any{"id": "seed", "locator": map[string]any{"uri": "<file-uri>", "line": 0, "character": 0, "language_id": "<language-id>"}, "down_depth": 1, "up_depth": 1}},
						"expansion":        map[string]any{"topmost_siblings": true},
						"limits":           map[string]any{"max_nodes": 200, "max_requests": 100, "max_evidence_bytes": 67108864, "max_path_work": 1000000, "timeout_ms": 60000, "request_timeout_ms": 30000, "max_response_bytes": 16777216, "max_messages": 4096},
					},
					"output_version": "lsp-trace.graph-provenance.v5", "detail": "full", "output_selector": "graphs/result.json",
				},
			},
		}
		description["graph_v5_source_snapshot"] = map[string]any{
			"output_version":                "lsp-trace.graph-v5-source-snapshot.v1",
			"requires_topmost_siblings":     true,
			"native_bounded_receipts":       true,
			"retained_bytes_only":           true,
			"authenticated_source_identity": false,
			"sibling_support_contribution":  0,
			"sibling_relations_are_calls":   false,
			"hydrated_selector":             "sibling_relation_ids",
		}
	}
	return description, true
}

func operationGuidance(tool Tool, advertised bool) map[string]any {
	visibility := "hidden"
	if advertised {
		visibility = "advertised"
	}
	guidance := map[string]any{
		"canonical_operation":      tool.Name,
		"direct_tool":              tool.Name,
		"compact_visibility":       visibility,
		"required_arguments":       schemaStringArray(tool.InputSchema, "required"),
		"required_argument_sets":   schemaRequiredAlternatives(tool.InputSchema),
		"selector_alternatives":    schemaSelectorProperties(tool.InputSchema),
		"selector_role":            "artifact publication destination supplied by the caller; not an artifact or schema identity",
		"output_schema_ids":        append([]string(nil), tool.ArtifactSchemaIDs...),
		"output_families_versions": append([]string(nil), tool.OutputFamiliesVersions...),
		"output_schema_source":     "registered artifact schema IDs and versioned families; no schema identity is inferred from a selector",
	}
	if tool.ExecutorFamily == IncomingExecutorFamily || tool.ExecutorFamily == SliceExecutorFamily || tool.ExecutorFamily == AcquisitionV2ExecutorFamily {
		guidance["position_convention"] = "MCP line and character values are zero-based; CLI --at PATH:LINE:COLUMN values are one-based"
	}
	if tool.Name == "lsp_trace_v3_slice" || tool.Name == "lsp_trace_v3_incoming" {
		guidance["output_version_guidance"] = "acquisition route v3; select Graph Provenance V5 output with output_version=lsp-trace.graph-provenance.v5"
	}
	return guidance
}

func registeredOutputFamilies(ids []string, registered map[string]string) []string {
	families := make([]string, 0, len(ids))
	for _, id := range ids {
		if family, ok := registered[id]; ok {
			families = append(families, family)
		}
	}
	return families
}

func schemaStringArray(schema map[string]any, key string) []string {
	raw, _ := schema[key].([]any)
	values := make([]string, 0, len(raw))
	for _, item := range raw {
		if value, ok := item.(string); ok {
			values = append(values, value)
		}
	}
	return values
}

func schemaRequiredAlternatives(schema map[string]any) [][]string {
	variants, _ := schema["oneOf"].([]any)
	alternatives := make([][]string, 0, len(variants))
	for _, raw := range variants {
		variant, _ := raw.(map[string]any)
		if required := schemaStringArray(variant, "required"); len(required) > 0 {
			alternatives = append(alternatives, required)
		}
	}
	return alternatives
}

func schemaSelectorProperties(schema map[string]any) []string {
	set := map[string]struct{}{}
	var collect func(map[string]any)
	collect = func(candidate map[string]any) {
		properties, _ := candidate["properties"].(map[string]any)
		for name := range properties {
			if name == "selector" || strings.HasSuffix(name, "_selector") {
				set[name] = struct{}{}
			}
		}
		for _, keyword := range []string{"oneOf", "anyOf"} {
			variants, _ := candidate[keyword].([]any)
			for _, raw := range variants {
				if variant, ok := raw.(map[string]any); ok {
					collect(variant)
				}
			}
		}
	}
	collect(schema)
	selectors := make([]string, 0, len(set))
	for name := range set {
		selectors = append(selectors, name)
	}
	sort.Strings(selectors)
	return selectors
}

func operationShortName(name string) string {
	parts := strings.Split(name, "_")
	for i := 0; i+1 < len(parts); i++ {
		if len(parts[i]) >= 2 && parts[i][0] == 'v' {
			version := true
			for _, digit := range parts[i][1:] {
				if digit < '0' || digit > '9' {
					version = false
					break
				}
			}
			if version {
				return strings.Join(parts[i+1:], "_")
			}
		}
	}
	return name
}

func (r *Registry) Advertised() []Tool {
	out := make([]Tool, 0, len(r.tools))
	for _, tool := range r.tools {
		if tool.Availability != Enabled {
			continue
		}
		if r.toolProfile == ToolProfileCompact {
			if _, ok := compactToolNames[tool.Name]; !ok {
				continue
			}
		}
		out = append(out, cloneTool(tool))
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

// ResolveCanonical resolves only exact canonical names, including enabled tools
// that a presentation layer may choose not to advertise.
func (r *Registry) ResolveCanonical(name string) (Tool, bool) {
	for i := range r.tools {
		if r.tools[i].Name == name {
			return cloneTool(r.tools[i]), true
		}
	}
	return Tool{}, false
}
