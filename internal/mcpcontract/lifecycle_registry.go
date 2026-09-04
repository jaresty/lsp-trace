package mcpcontract

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"path"
	"sort"

	"lsp-trace/internal/operation"

	jsonschema "github.com/santhosh-tekuri/jsonschema/v6"
)

//go:embed testdata/stage2-lifecycle-contract.v1.json testdata/schemas/*.json
var lifecycleContractFiles embed.FS

const lifecycleContractPath = "testdata/stage2-lifecycle-contract.v1.json"

type AvailabilityState string

const (
	NotImplementedState         AvailabilityState = "NOT_IMPLEMENTED"
	ContainmentUnavailableState AvailabilityState = "CONTAINMENT_UNAVAILABLE"
	RuntimeDisabledState        AvailabilityState = "RUNTIME_DISABLED"
	EnabledState                AvailabilityState = "ENABLED"
)

var lifecycleStates = [...]AvailabilityState{
	NotImplementedState,
	ContainmentUnavailableState,
	RuntimeDisabledState,
	EnabledState,
}

const (
	lifecycleListOperation    operation.Name = "session_list"
	lifecycleRestartOperation operation.Name = "session_restart"
	lifecycleStatusOperation  operation.Name = "session_status"
	lifecycleStopOperation    operation.Name = "session_stop"
)

type AvailabilityProjection struct {
	Advertised        bool                `json:"advertised"`
	EnvelopeSchemaIDs map[string][]string `json:"envelope_schema_ids_by_tool"`
}

// LifecycleContract is the closed Stage 2 projection and operation registry.
// Loading it does not activate or advertise any runtime operation.
type LifecycleContract struct {
	CandidateStatus          string                                       `json:"candidate_status"`
	ContractVersion          string                                       `json:"contract_version"`
	Extends                  string                                       `json:"extends"`
	ValidationOrder          []string                                     `json:"validation_order"`
	AvailabilityProjections  map[AvailabilityState]AvailabilityProjection `json:"availability_projections"`
	Schemas                  []SchemaRegistration                         `json:"schemas"`
	Tools                    []ToolContract                               `json:"tools"`
	Transcripts              []string                                     `json:"transcripts"`
	TranscriptEvidence       map[string]string                            `json:"transcript_evidence"`
	Unresolved               []json.RawMessage                            `json:"unresolved_contract_assertions"`
	CallerAttachmentOverflow json.RawMessage                              `json:"caller_attachment_overflow"`
}

func LoadLifecycleContract() (*LifecycleContract, error) {
	raw, err := lifecycleContractFiles.ReadFile(lifecycleContractPath)
	if err != nil {
		return nil, err
	}
	var contract LifecycleContract
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&contract); err != nil {
		return nil, fmt.Errorf("decode lifecycle contract: %w", err)
	}
	if err := validateLifecycleContract(&contract); err != nil {
		return nil, err
	}
	return &contract, nil
}

func validateLifecycleContract(contract *LifecycleContract) error {
	if contract.CandidateStatus != "UNACCEPTED_NOT_RUNTIME" || contract.ContractVersion != "1" || contract.Extends != "stage1-manifest.v1" {
		return fmt.Errorf("invalid lifecycle contract identity")
	}
	registered := make(map[string]SchemaRegistration, len(contract.Schemas))
	for _, schema := range contract.Schemas {
		if err := ValidateSchemaIdentity(schema); err != nil {
			return err
		}
		if _, exists := registered[schema.ID]; exists {
			return fmt.Errorf("lifecycle schema identity rebound: %s", schema.ID)
		}
		registered[schema.ID] = schema
	}
	tools := make(map[string]ToolContract, len(contract.Tools))
	operations := make(map[operation.Name]bool, len(contract.Tools))
	for _, tool := range contract.Tools {
		name, ok := lifecycleOperationName(tool.Name)
		if !ok || operations[name] || len(tool.Aliases) != 1 || tool.Aliases[0] == "" {
			return fmt.Errorf("invalid lifecycle operation registry entry %q", tool.Name)
		}
		operations[name] = true
		tools[tool.Name] = tool
		if schema, ok := registered[tool.InputSchemaID]; !ok || schema.Layer != "input" {
			return fmt.Errorf("lifecycle tool %q input schema is not registered as input", tool.Name)
		}
	}
	if len(tools) != 4 || len(contract.AvailabilityProjections) != len(lifecycleStates) {
		return fmt.Errorf("lifecycle registry must contain four tools and four availability projections")
	}
	for _, state := range lifecycleStates {
		projection, ok := contract.AvailabilityProjections[state]
		if !ok || projection.Advertised != (state == EnabledState) || len(projection.EnvelopeSchemaIDs) != len(tools) {
			return fmt.Errorf("invalid lifecycle availability projection %q", state)
		}
		for toolName, ids := range projection.EnvelopeSchemaIDs {
			if _, ok := tools[toolName]; !ok || len(ids) == 0 {
				return fmt.Errorf("projection %q contains invalid tool %q", state, toolName)
			}
			for _, id := range ids {
				schema, ok := registered[id]
				if !ok || schema.Layer != "envelope" {
					return fmt.Errorf("projection %q schema %q is not a registered envelope", state, id)
				}
			}
		}
	}
	return nil
}

// Project returns a defensive, canonical-name-sorted operation registry for state.
func (c *LifecycleContract) Project(state AvailabilityState) ([]ToolContract, error) {
	if c == nil {
		return nil, fmt.Errorf("lifecycle contract is nil")
	}
	projection, ok := c.AvailabilityProjections[state]
	if !ok {
		return nil, fmt.Errorf("unknown lifecycle availability %q", state)
	}
	tools := make([]ToolContract, len(c.Tools))
	for i, tool := range c.Tools {
		tool.Aliases = append([]string(nil), tool.Aliases...)
		tool.EnvelopeSchemaIDs = append([]string(nil), projection.EnvelopeSchemaIDs[tool.Name]...)
		tool.ArtifactSchemaIDs = append([]string(nil), tool.ArtifactSchemaIDs...)
		tool.Advertised = projection.Advertised
		tool.Availability = string(state)
		tools[i] = tool
	}
	sort.Slice(tools, func(i, j int) bool { return tools[i].Name < tools[j].Name })
	return tools, nil
}

type LifecycleOperationInputValidator struct {
	contract   *LifecycleContract
	schemaIDs  map[operation.Name]string
	operations []operation.Name
}

func NewLifecycleOperationInputValidator(state AvailabilityState) (*LifecycleOperationInputValidator, error) {
	contract, err := LoadLifecycleContract()
	if err != nil {
		return nil, err
	}
	projected, err := contract.Project(state)
	if err != nil {
		return nil, err
	}
	validator := &LifecycleOperationInputValidator{contract: contract, schemaIDs: make(map[operation.Name]string, len(projected))}
	for _, tool := range projected {
		name, _ := lifecycleOperationName(tool.Name)
		validator.schemaIDs[name] = tool.InputSchemaID
		validator.operations = append(validator.operations, name)
	}
	sort.Slice(validator.operations, func(i, j int) bool { return validator.operations[i] < validator.operations[j] })
	return validator, nil
}

func lifecycleOperationName(tool string) (operation.Name, bool) {
	switch tool {
	case "lsp_session_v1_list":
		return lifecycleListOperation, true
	case "lsp_session_v1_restart":
		return lifecycleRestartOperation, true
	case "lsp_session_v1_status":
		return lifecycleStatusOperation, true
	case "lsp_session_v1_stop":
		return lifecycleStopOperation, true
	default:
		return "", false
	}
}

func (v *LifecycleOperationInputValidator) ValidateOperationInput(name operation.Name, input json.RawMessage) error {
	if v == nil {
		return fmt.Errorf("lifecycle operation input validator is nil")
	}
	schemaID := v.schemaIDs[name]
	if schemaID == "" {
		return fmt.Errorf("unknown lifecycle operation %q", name)
	}
	return validateLifecycleJSON(v.contract, schemaID, input)
}

func (v *LifecycleOperationInputValidator) Operations() []operation.Name {
	if v == nil {
		return nil
	}
	return append([]operation.Name(nil), v.operations...)
}

func validateLifecycleJSON(contract *LifecycleContract, schemaID string, data []byte) error {
	compiler := jsonschema.NewCompiler()
	compiler.DefaultDraft(jsonschema.Draft2020)
	found := false
	for _, registration := range contract.Schemas {
		raw, err := lifecycleContractFiles.ReadFile(path.Join("testdata", registration.Path))
		if err != nil {
			return err
		}
		doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
		if err != nil {
			return err
		}
		if err := compiler.AddResource(registration.ID, doc); err != nil {
			return err
		}
		found = found || registration.ID == schemaID
	}
	if !found {
		return fmt.Errorf("unknown lifecycle schema %q", schemaID)
	}
	compiled, err := compiler.Compile(schemaID)
	if err != nil {
		return err
	}
	value, err := jsonschema.UnmarshalJSON(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}
	if err := compiled.Validate(value); err != nil {
		return fmt.Errorf("schema validation %s: %w", schemaID, err)
	}
	return nil
}
