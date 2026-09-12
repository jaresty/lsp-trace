package acquisitionops

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
)

// ExplicitTraceAdmission is the sole caller-local custody authority for trace.
// Private fields prevent struct-literal admission and bind it to one exact call.
type ExplicitTraceAdmission struct {
	seedSpec    []byte
	requestHash [32]byte
	workspace   string
	sessionID   string
	generation  uint64
	requestID   string
}

type explicitSeedFile struct {
	SchemaVersion        string            `json:"schema_version"`
	CoordinateConvention string            `json:"coordinate_convention"`
	Defaults             json.RawMessage   `json:"defaults"`
	Seeds                []json.RawMessage `json:"seeds"`
}
type explicitSeed struct {
	Type      string          `json:"type"`
	Label     string          `json:"label,omitempty"`
	Path      string          `json:"path,omitempty"`
	Line      uint64          `json:"line,omitempty"`
	Column    uint64          `json:"column,omitempty"`
	Symbol    string          `json:"symbol,omitempty"`
	Target    json.RawMessage `json:"target,omitempty"`
	DownDepth *int            `json:"down_depth,omitempty"`
	UpDepth   *int            `json:"up_depth,omitempty"`
}

// NewExplicitTraceAdmission canonically decodes/re-encodes Seeds V2 and binds
// every seed to the exact resolved acquisition request and V5 output request.
func NewExplicitTraceAdmission(seedSpec []byte, workspace, requestID string, input Input) (ExplicitTraceAdmission, error) {
	if workspace == "" || input.SessionID == "" || input.Generation == 0 || input.OutputVersion != "lsp-trace.graph-provenance.v5" {
		return ExplicitTraceAdmission{}, fmt.Errorf("exact explicit trace context required")
	}
	var file explicitSeedFile
	if err := decodeExplicit(seedSpec, &file); err != nil {
		return ExplicitTraceAdmission{}, err
	}
	if file.SchemaVersion != "lsp-trace.seeds.v2" || file.CoordinateConvention != "one-based" || len(file.Seeds) != 1+len(input.SeedManifest.RequiredTargets) {
		return ExplicitTraceAdmission{}, fmt.Errorf("explicit trace seed schema or count mismatch")
	}
	canonical, err := json.Marshal(file)
	if err != nil || !bytes.Equal(seedSpec, canonical) {
		return ExplicitTraceAdmission{}, fmt.Errorf("explicit trace seeds must be canonical bytes")
	}
	targets := append([]Target{input.SeedManifest.Root}, input.SeedManifest.RequiredTargets...)
	for i, raw := range file.Seeds {
		var seed explicitSeed
		if err := decodeExplicit(raw, &seed); err != nil {
			return ExplicitTraceAdmission{}, fmt.Errorf("seed %d: %w", i, err)
		}
		t := targets[i]
		if seed.Type != "position" || seed.Label != t.ID || t.Locator.Line == nil || t.Locator.Character == nil || seed.Line != uint64(*t.Locator.Line)+1 || seed.Column != uint64(*t.Locator.Character)+1 {
			return ExplicitTraceAdmission{}, fmt.Errorf("seed %d target, position, depth, or label mismatch", i)
		}
	}
	rawInput, err := json.Marshal(input)
	if err != nil {
		return ExplicitTraceAdmission{}, err
	}
	return ExplicitTraceAdmission{seedSpec: bytes.Clone(seedSpec), requestHash: sha256.Sum256(rawInput), workspace: workspace, sessionID: input.SessionID, generation: input.Generation, requestID: requestID}, nil
}

func decodeExplicit(raw []byte, dst any) error {
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(dst); err != nil {
		return err
	}
	if err := d.Decode(&struct{}{}); err != io.EOF {
		return fmt.Errorf("one JSON object required")
	}
	return nil
}

func (a ExplicitTraceAdmission) admit(opRequestID, workspace string, input Input) ([]byte, error) {
	raw, err := json.Marshal(input)
	if err != nil || a.seedSpec == nil || a.requestID != opRequestID || a.workspace != workspace || a.sessionID != input.SessionID || a.generation != input.Generation || a.requestHash != sha256.Sum256(raw) {
		return nil, fmt.Errorf("explicit trace admission mismatch")
	}
	return bytes.Clone(a.seedSpec), nil
}
