package graphprovenance

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"

	"lsp-trace/internal/graph"
	"lsp-trace/internal/manageddiagnostic"
	"lsp-trace/internal/schema"
	"lsp-trace/internal/strictjson"
)

const (
	VersionV5          = "lsp-trace.graph-provenance.v5"
	GraphV5SchemaID    = "https://jaresty.github.io/lsp-trace/schemas/lsp-trace.graph.v5.schema.json"
	MaxEnvelopeBytesV5 = 192<<20 + MaxDiagnosticBytes
)

type EvidenceV5 struct {
	SchemaVersion   string        `json:"schema_version"`
	SessionID       string        `json:"session_id"`
	Generation      uint64        `json:"generation"`
	GraphV5         string        `json:"graph_v5"`
	GraphV5SHA256   string        `json:"graph_v5_sha256"`
	GraphV5SchemaID string        `json:"graph_v5_schema_id"`
	Diagnostics     DiagnosticsV3 `json:"diagnostics"`
}

// CaptureV5 retains the native graph.v5 serialization exactly. It deliberately
// does not project sibling candidates into CALLS or any support relation.
func CaptureV5(native []byte, sessionID string, generation uint64, query manageddiagnostic.QueryResult) ([]byte, error) {
	if sessionID == "" || generation == 0 {
		return nil, errors.New("V5 exact session and generation required")
	}
	if detected, err := schema.ValidateFor(native, schema.FamilyGraph, "v5"); err != nil || detected != graph.SchemaVersionV5 {
		if err == nil {
			err = errors.New("native graph is not graph.v5")
		}
		return nil, fmt.Errorf("V5 embedded graph: %w", err)
	}
	d := DiagnosticsV3{Status: query.Status, Records: []DiagnosticRecordV3{}, EvictedRecords: query.EvictedRecords, MaxRecords: MaxDiagnosticRecords, MaxBytes: MaxDiagnosticBytes}
	var last uint64
	for _, r := range query.Records {
		if err := manageddiagnostic.Validate(r); err != nil || r.SessionID != sessionID || r.Generation != generation || r.Sequence <= last {
			return nil, errors.New("V5 diagnostic identity or ordering invalid")
		}
		last = r.Sequence
		p := DiagnosticRecordV3{Sequence: r.Sequence, Phase: r.Phase, Substep: r.Substep, Terminal: r.Terminal, CallHierarchy: r.CallHierarchy, DocumentSupplyCompleted: r.DocumentSupplyCompleted, ProcessExitStatus: r.ProcessExit.Status, NumericRPCCode: r.NumericRPCCode}
		candidate, _ := json.Marshal(p)
		if len(d.Records) >= MaxDiagnosticRecords || d.RetainedBytes+len(candidate) > MaxDiagnosticBytes {
			d.OmittedRecords++
			continue
		}
		d.Records = append(d.Records, p)
		d.RetainedBytes += len(candidate)
	}
	if err := validateDiagnosticsV3(d); err != nil {
		return nil, err
	}
	sum := sha256.Sum256(native)
	e := EvidenceV5{SchemaVersion: VersionV5, SessionID: sessionID, Generation: generation, GraphV5: base64.StdEncoding.EncodeToString(native), GraphV5SHA256: fmt.Sprintf("sha256:%x", sum), GraphV5SchemaID: GraphV5SchemaID, Diagnostics: d}
	raw, err := json.Marshal(e)
	if err != nil {
		return nil, err
	}
	raw = append(raw, '\n')
	if _, err = validateForV5(raw); err != nil {
		return nil, err
	}
	return raw, nil
}

func validateForV5(raw []byte) (string, error) {
	if len(raw) > MaxEnvelopeBytesV5 {
		return "", errors.New("V5 envelope byte limit")
	}
	if _, err := schema.ValidateStructure(raw, Family, "v5"); err != nil {
		return "", err
	}
	if err := strictjson.RejectDuplicates(raw); err != nil {
		return "", err
	}
	var e EvidenceV5
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(&e); err != nil {
		return "", err
	}
	if e.SchemaVersion != VersionV5 || e.SessionID == "" || e.Generation == 0 || e.GraphV5SchemaID != GraphV5SchemaID {
		return "", errors.New("invalid V5 identity")
	}
	native, err := base64.StdEncoding.DecodeString(e.GraphV5)
	if err != nil {
		return "", err
	}
	if detected, err := schema.ValidateFor(native, schema.FamilyGraph, "v5"); err != nil || detected != graph.SchemaVersionV5 {
		if err != nil {
			return "", err
		}
		return "", errors.New("embedded graph is not graph.v5")
	}
	sum := sha256.Sum256(native)
	if e.GraphV5SHA256 != fmt.Sprintf("sha256:%x", sum) {
		return "", errors.New("V5 embedded graph digest mismatch")
	}
	if err := validateDiagnosticsV3(e.Diagnostics); err != nil {
		return "", err
	}
	return VersionV5, nil
}
