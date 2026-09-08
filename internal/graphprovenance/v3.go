package graphprovenance

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"

	"lsp-trace/internal/manageddiagnostic"
	"lsp-trace/internal/strictjson"
)

const (
	VersionV3            = "lsp-trace.graph-provenance.v3"
	MaxDiagnosticRecords = 64
	MaxDiagnosticBytes   = 64 << 10
)

type DiagnosticRecordV3 struct {
	Sequence                uint64                                            `json:"sequence"`
	Phase                   manageddiagnostic.Phase                           `json:"phase"`
	Substep                 manageddiagnostic.Fact[manageddiagnostic.Substep] `json:"substep"`
	Terminal                manageddiagnostic.Terminal                        `json:"terminal"`
	CallHierarchy           manageddiagnostic.Fact[bool]                      `json:"call_hierarchy"`
	DocumentSupplyCompleted manageddiagnostic.Fact[bool]                      `json:"document_supply_completed"`
	ProcessExitStatus       manageddiagnostic.Status                          `json:"process_exit_status"`
	NumericRPCCode          manageddiagnostic.Fact[int]                       `json:"numeric_rpc_code"`
}

type DiagnosticsV3 struct {
	Status         manageddiagnostic.QueryStatus `json:"status"`
	Records        []DiagnosticRecordV3          `json:"records"`
	EvictedRecords uint64                        `json:"evicted_records"`
	OmittedRecords uint64                        `json:"omitted_records"`
	RetainedBytes  int                           `json:"retained_bytes"`
	MaxRecords     int                           `json:"max_records"`
	MaxBytes       int                           `json:"max_bytes"`
}

type EvidenceV3 struct {
	SchemaVersion           string        `json:"schema_version"`
	SessionID               string        `json:"session_id"`
	Generation              uint64        `json:"generation"`
	GraphProvenanceV2       string        `json:"graph_provenance_v2"`
	GraphProvenanceV2SHA256 string        `json:"graph_provenance_v2_sha256"`
	Diagnostics             DiagnosticsV3 `json:"diagnostics"`
}

func CaptureV3(v2 []byte, sessionID string, generation uint64, query manageddiagnostic.QueryResult) ([]byte, error) {
	if sessionID == "" || generation == 0 {
		return nil, errors.New("V3 exact session and generation required")
	}
	if _, err := validateForV2(v2); err != nil {
		return nil, fmt.Errorf("V3 embedded V2: %w", err)
	}
	d := DiagnosticsV3{Status: query.Status, Records: []DiagnosticRecordV3{}, EvictedRecords: query.EvictedRecords, MaxRecords: MaxDiagnosticRecords, MaxBytes: MaxDiagnosticBytes}
	if d.Status != manageddiagnostic.QueryAvailable && d.Status != manageddiagnostic.QueryUnavailable && d.Status != manageddiagnostic.QueryEvicted {
		return nil, errors.New("V3 diagnostic status invalid")
	}
	var last uint64
	for _, r := range query.Records {
		if err := manageddiagnostic.Validate(r); err != nil || r.SessionID != sessionID || r.Generation != generation || r.Sequence <= last {
			return nil, errors.New("V3 diagnostic identity or ordering invalid")
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
	sum := sha256.Sum256(v2)
	e := EvidenceV3{SchemaVersion: VersionV3, SessionID: sessionID, Generation: generation, GraphProvenanceV2: base64.StdEncoding.EncodeToString(v2), GraphProvenanceV2SHA256: fmt.Sprintf("sha256:%x", sum), Diagnostics: d}
	raw, err := json.Marshal(e)
	if err != nil {
		return nil, err
	}
	raw = append(raw, '\n')
	if _, err = validateForV3(raw); err != nil {
		return nil, err
	}
	return raw, nil
}

func validateForV3(raw []byte) (string, error) {
	if len(raw) > MaxEnvelopeBytes+MaxDiagnosticBytes {
		return "", errors.New("V3 envelope byte limit")
	}
	if err := strictjson.RejectDuplicates(raw); err != nil {
		return "", err
	}
	var e EvidenceV3
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(&e); err != nil {
		return "", err
	}
	if e.SchemaVersion != VersionV3 || e.SessionID == "" || e.Generation == 0 {
		return "", errors.New("invalid V3 identity")
	}
	v2, err := base64.StdEncoding.DecodeString(e.GraphProvenanceV2)
	if err != nil {
		return "", err
	}
	if _, err = validateForV2(v2); err != nil {
		return "", err
	}
	sum := sha256.Sum256(v2)
	if e.GraphProvenanceV2SHA256 != fmt.Sprintf("sha256:%x", sum) {
		return "", errors.New("V3 embedded V2 digest mismatch")
	}
	if e.Diagnostics.MaxRecords != MaxDiagnosticRecords || e.Diagnostics.MaxBytes != MaxDiagnosticBytes || e.Diagnostics.Records == nil || len(e.Diagnostics.Records) > MaxDiagnosticRecords || e.Diagnostics.RetainedBytes > MaxDiagnosticBytes {
		return "", errors.New("V3 diagnostic bounds invalid")
	}
	return VersionV3, nil
}
