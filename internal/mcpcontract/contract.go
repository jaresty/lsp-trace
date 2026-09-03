package mcpcontract

import (
	"bufio"
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"path"
	"regexp"
	"sort"
	"strings"

	jsonschema "github.com/santhosh-tekuri/jsonschema/v6"
)

//go:embed testdata/stage1-manifest.v1.json testdata/schemas/*.json testdata/transcripts/*.jsonl
var contractFiles embed.FS

const manifestPath = "testdata/stage1-manifest.v1.json"

var versionedID = regexp.MustCompile(`(?:^|[./])v[0-9]+(?:[./]|$)`)

type Manifest struct {
	ManifestVersion string               `json:"manifest_version"`
	MCPProtocol     ProtocolPin          `json:"mcp_protocol"`
	Schemas         []SchemaRegistration `json:"schemas"`
	Tools           []ToolContract       `json:"tools"`
	Transcripts     []string             `json:"transcripts"`
}

type ProtocolPin struct {
	Revision        string `json:"revision"`
	PublicationDate string `json:"publication_date"`
}

type SchemaRegistration struct {
	ID     string `json:"id"`
	Family string `json:"family"`
	Layer  string `json:"layer"`
	Path   string `json:"path"`
}

type ToolContract struct {
	Name              string   `json:"name"`
	Aliases           []string `json:"aliases"`
	InputSchemaID     string   `json:"input_schema_id"`
	EnvelopeSchemaIDs []string `json:"envelope_schema_ids"`
	ArtifactSchemaIDs []string `json:"artifact_schema_ids"`
	Advertised        bool     `json:"advertised"`
	Availability      string   `json:"availability"`
}

func LoadManifest() (*Manifest, error) {
	raw, err := contractFiles.ReadFile(manifestPath)
	if err != nil {
		return nil, err
	}
	var manifest Manifest
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&manifest); err != nil {
		return nil, fmt.Errorf("decode manifest: %w", err)
	}
	if err := ValidateManifest(&manifest); err != nil {
		return nil, err
	}
	return &manifest, nil
}

func ValidateManifest(manifest *Manifest) error {
	if manifest == nil {
		return fmt.Errorf("manifest is nil")
	}
	if manifest.ManifestVersion != "1" || manifest.MCPProtocol.Revision == "" || manifest.MCPProtocol.PublicationDate == "" {
		return fmt.Errorf("manifest version and MCP protocol revision/date must be pinned")
	}
	if len(manifest.Tools) != 12 {
		return fmt.Errorf("canonical tool count: got %d want 12", len(manifest.Tools))
	}
	ids, families, names := map[string]bool{}, map[string]bool{}, map[string]bool{}
	for _, s := range manifest.Schemas {
		if err := ValidateSchemaIdentity(s); err != nil {
			return err
		}
		if ids[s.ID] || families[s.Family] {
			return fmt.Errorf("schema identity/family rebound: %s", s.ID)
		}
		ids[s.ID], families[s.Family] = true, true
	}
	for _, tool := range manifest.Tools {
		if len(tool.Aliases) != 1 || tool.Name == "" || tool.Aliases[0] == "" {
			return fmt.Errorf("tool %q must have one alias", tool.Name)
		}
		for _, name := range []string{tool.Name, tool.Aliases[0]} {
			if names[name] {
				return fmt.Errorf("name or alias %q is not globally unique", name)
			}
			names[name] = true
		}
		switch tool.Availability {
		case "ENABLED":
			if !tool.Advertised {
				return fmt.Errorf("enabled tool %q must be advertised", tool.Name)
			}
		case "NOT_IMPLEMENTED":
			if tool.Advertised {
				return fmt.Errorf("reserved tool %q must not be advertised", tool.Name)
			}
		default:
			return fmt.Errorf("tool %q has invalid availability %q", tool.Name, tool.Availability)
		}
		if !ids[tool.InputSchemaID] {
			return fmt.Errorf("tool %q input schema is unregistered", tool.Name)
		}
		for _, id := range append(append([]string{}, tool.EnvelopeSchemaIDs...), tool.ArtifactSchemaIDs...) {
			if !ids[id] {
				return fmt.Errorf("tool %q schema is unregistered: %s", tool.Name, id)
			}
		}
	}
	return nil
}

func ValidateSchemaIdentity(s SchemaRegistration) error {
	if !strings.HasPrefix(s.ID, "https://") || !versionedID.MatchString(s.ID) || !versionedID.MatchString(s.Family) {
		return fmt.Errorf("schema identity must be immutable and versioned: %q", s.ID)
	}
	switch s.Layer {
	case "input", "envelope", "artifact", "capability":
	default:
		return fmt.Errorf("schema %q has invalid layer %q", s.ID, s.Layer)
	}
	if s.Path == "" {
		return fmt.Errorf("schema %q has no path", s.ID)
	}
	return nil
}

// SchemaJSON returns a defensive copy of the schema registered under schemaID.
func SchemaJSON(schemaID string) ([]byte, error) {
	manifest, err := LoadManifest()
	if err != nil {
		return nil, err
	}
	for _, registration := range manifest.Schemas {
		if registration.ID != schemaID {
			continue
		}
		if strings.HasPrefix(registration.Path, "../") {
			return nil, fmt.Errorf("external schema %q has no embedded contract document", schemaID)
		}
		raw, err := contractFiles.ReadFile(path.Join("testdata", registration.Path))
		if err != nil {
			return nil, err
		}
		return append([]byte(nil), raw...), nil
	}
	return nil, fmt.Errorf("unknown schema %q", schemaID)
}

func ValidateJSON(schemaID string, data []byte) error {
	manifest, err := LoadManifest()
	if err != nil {
		return err
	}
	compiled, err := compileSchema(manifest, schemaID)
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

func ValidateEnvelopeExclusive(data []byte) error {
	manifest, err := LoadManifest()
	if err != nil {
		return err
	}
	var value map[string]any
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	named, _ := value["envelope_schema_id"].(string)
	matches := []string{}
	for _, registration := range manifest.Schemas {
		if registration.Layer != "envelope" {
			continue
		}
		compiled, err := compileSchema(manifest, registration.ID)
		if err != nil {
			return err
		}
		doc, _ := jsonschema.UnmarshalJSON(bytes.NewReader(data))
		if compiled.Validate(doc) == nil {
			matches = append(matches, registration.ID)
		}
	}
	if len(matches) != 1 || matches[0] != named {
		return fmt.Errorf("envelope must match exactly named schema %q; matches=%v", named, matches)
	}
	return nil
}

func ValidateTranscripts(manifest *Manifest) error {
	for _, name := range manifest.Transcripts {
		raw, err := contractFiles.ReadFile("testdata/" + name)
		if err != nil {
			return err
		}
		preEnvelope := strings.Contains(name, "error") || strings.Contains(name, "unknown-tool")
		if err := ValidateTranscript(raw, preEnvelope); err != nil {
			return fmt.Errorf("transcript %s: %w", name, err)
		}
	}
	return nil
}

func ValidateTranscript(raw []byte, preEnvelope bool) error {
	if len(raw) == 0 || raw[len(raw)-1] != '\n' || bytes.HasPrefix(raw, []byte{0xef, 0xbb, 0xbf}) {
		return fmt.Errorf("not canonical JSONL")
	}
	scanner := bufio.NewScanner(bytes.NewReader(raw))
	for scanner.Scan() {
		line := scanner.Bytes()
		var value any
		if err := json.Unmarshal(line, &value); err != nil {
			return err
		}
		canonical, _ := json.Marshal(value)
		if !bytes.Equal(line, canonical) {
			return fmt.Errorf("line is not canonical JSON")
		}
		if preEnvelope && (bytes.Contains(line, []byte("envelope_schema_id")) || bytes.Contains(line, []byte("structuredContent"))) {
			return fmt.Errorf("pre-envelope error contains operation envelope")
		}
	}
	return scanner.Err()
}

func compileSchema(manifest *Manifest, id string) (*jsonschema.Schema, error) {
	compiler := jsonschema.NewCompiler()
	compiler.DefaultDraft(jsonschema.Draft2020)
	found := false
	registrations := append([]SchemaRegistration(nil), manifest.Schemas...)
	sort.Slice(registrations, func(i, j int) bool { return registrations[i].ID < registrations[j].ID })
	for _, registration := range registrations {
		if registration.Path[:min(len(registration.Path), 3)] == "../" {
			continue
		}
		raw, err := contractFiles.ReadFile(path.Join("testdata", registration.Path))
		if err != nil {
			return nil, err
		}
		doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
		if err != nil {
			return nil, err
		}
		if err := compiler.AddResource(registration.ID, doc); err != nil {
			return nil, err
		}
		if registration.ID == id {
			found = true
		}
	}
	if !found {
		return nil, fmt.Errorf("unknown or external schema %q", id)
	}
	return compiler.Compile(id)
}
