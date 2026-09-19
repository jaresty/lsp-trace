package censusresult

import (
	"encoding/json"
	"errors"
)

const DiagnosticSchemaVersion = "lsp-trace.census-diagnostic.v1"

type Stage string
type Code string

const (
	StageSyntax      Stage = "syntax"
	StageConfig      Stage = "config"
	StageDiscovery   Stage = "discovery"
	StageAcquisition Stage = "acquisition"
	StageAssembly    Stage = "assembly"
	StagePublication Stage = "publication"
	StageCommitted   Stage = "committed-degradation"

	CodeInvalidSyntax     Code = "INVALID_SYNTAX"
	CodeInvalidConfig     Code = "INVALID_CONFIG"
	CodeDiscoveryFailed   Code = "DISCOVERY_FAILED"
	CodeAcquisitionFailed Code = "ACQUISITION_FAILED"
	CodeAssemblyFailed    Code = "ASSEMBLY_FAILED"
	CodePublicationFailed Code = "PUBLICATION_FAILED"
	CodeCommittedDegraded Code = "COMMITTED_DEGRADED"
)

type Diagnostic struct {
	SchemaVersion string `json:"schema_version"`
	Status        string `json:"status"`
	Stage         Stage  `json:"stage"`
	Code          Code   `json:"code"`
	BatchOrdinal  *int   `json:"batch_ordinal,omitempty"`
	Retry         bool   `json:"retry"`
	// Detail is an optional, human-readable, non-authoritative explanation of
	// which specific check produced the failure. It never changes the stage or
	// code contract; it exists so an opaque stage code (for example
	// INVALID_CONFIG) can name its actionable cause. Omitted when empty.
	Detail string `json:"detail,omitempty"`
}

// WithDetail returns a copy of d annotated with an actionable failure reason.
func (d Diagnostic) WithDetail(detail string) Diagnostic {
	d.Detail = detail
	return d
}

func NewDiagnostic(stage Stage, batchOrdinal *int) (Diagnostic, error) {
	codes := map[Stage]Code{StageSyntax: CodeInvalidSyntax, StageConfig: CodeInvalidConfig, StageDiscovery: CodeDiscoveryFailed, StageAcquisition: CodeAcquisitionFailed, StageAssembly: CodeAssemblyFailed, StagePublication: CodePublicationFailed, StageCommitted: CodeCommittedDegraded}
	code, ok := codes[stage]
	if !ok || batchOrdinal != nil && (*batchOrdinal < 0 || stage != StageAcquisition) {
		return Diagnostic{}, errors.New("invalid census diagnostic")
	}
	d := Diagnostic{SchemaVersion: DiagnosticSchemaVersion, Status: "FAILED", Stage: stage, Code: code, BatchOrdinal: batchOrdinal, Retry: stage != StageCommitted}
	if stage == StageCommitted {
		d.Status = "SUCCEEDED_DEGRADED"
	}
	return d, nil
}

func MarshalDiagnostic(d Diagnostic) ([]byte, error) {
	expected, err := NewDiagnostic(d.Stage, d.BatchOrdinal)
	if err != nil || d.SchemaVersion != expected.SchemaVersion || d.Status != expected.Status || d.Stage != expected.Stage || d.Code != expected.Code || d.Retry != expected.Retry {
		if err == nil {
			err = errors.New("invalid census diagnostic")
		}
		return nil, err
	}
	b, err := json.Marshal(d)
	if err != nil {
		return nil, err
	}
	return append(b, '\n'), nil
}
