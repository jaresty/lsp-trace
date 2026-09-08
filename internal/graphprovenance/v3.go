package graphprovenance

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"

	"lsp-trace/internal/manageddiagnostic"
	"lsp-trace/internal/schema"
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
	if err := validateDiagnosticsV3(d); err != nil {
		return nil, err
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

func validProjectedFact[T comparable](f manageddiagnostic.Fact[T]) bool {
	var zero T
	if f.Status == "" {
		return f.Value == zero
	}
	if f.Status != manageddiagnostic.Observed && f.Status != manageddiagnostic.Unavailable && f.Status != manageddiagnostic.Withheld {
		return false
	}
	return f.Status == manageddiagnostic.Observed || f.Value == zero
}

func validateDiagnosticRecordV3(r DiagnosticRecordV3) error {
	if r.Sequence == 0 || !validProjectedFact(r.Substep) || !validProjectedFact(r.CallHierarchy) || !validProjectedFact(r.DocumentSupplyCompleted) || !validProjectedFact(r.NumericRPCCode) {
		return errors.New("V3 diagnostic fact invalid or carries hidden value")
	}
	if r.ProcessExitStatus != manageddiagnostic.Observed && r.ProcessExitStatus != manageddiagnostic.Unavailable && r.ProcessExitStatus != manageddiagnostic.Withheld {
		return errors.New("V3 process exit status invalid")
	}
	terminals := map[manageddiagnostic.Phase]map[manageddiagnostic.Terminal]bool{
		manageddiagnostic.PhaseSpawn:              {manageddiagnostic.TerminalProtocolError: true, manageddiagnostic.TerminalTransportClosed: true, manageddiagnostic.TerminalCancelled: true, manageddiagnostic.TerminalDeadlineExceeded: true, manageddiagnostic.TerminalProcessExited: true, manageddiagnostic.TerminalUnknown: true},
		manageddiagnostic.PhaseInitializeWrite:    {manageddiagnostic.TerminalProtocolError: true, manageddiagnostic.TerminalTransportClosed: true, manageddiagnostic.TerminalCancelled: true, manageddiagnostic.TerminalDeadlineExceeded: true, manageddiagnostic.TerminalProcessExited: true, manageddiagnostic.TerminalUnknown: true},
		manageddiagnostic.PhaseInitializeResponse: {manageddiagnostic.TerminalResponseReceived: true, manageddiagnostic.TerminalProtocolError: true, manageddiagnostic.TerminalTransportClosed: true, manageddiagnostic.TerminalCancelled: true, manageddiagnostic.TerminalDeadlineExceeded: true, manageddiagnostic.TerminalProcessExited: true, manageddiagnostic.TerminalUnknown: true},
		manageddiagnostic.PhaseRequestDispatch:    {manageddiagnostic.TerminalResponseReceived: true, manageddiagnostic.TerminalProtocolError: true, manageddiagnostic.TerminalTransportClosed: true, manageddiagnostic.TerminalCancelled: true, manageddiagnostic.TerminalDeadlineExceeded: true, manageddiagnostic.TerminalProcessExited: true, manageddiagnostic.TerminalUnknown: true},
		manageddiagnostic.PhaseReadinessComplete:  {manageddiagnostic.TerminalResponseReceived: true},
		manageddiagnostic.PhaseDocumentSupply:     {manageddiagnostic.TerminalResponseReceived: true, manageddiagnostic.TerminalProtocolError: true},
		manageddiagnostic.PhaseCapabilityCheck:    {manageddiagnostic.TerminalResponseReceived: true, manageddiagnostic.TerminalProtocolError: true},
	}
	if !terminals[r.Phase][r.Terminal] {
		return errors.New("V3 diagnostic terminal does not belong to phase")
	}
	if r.Phase == manageddiagnostic.PhaseReadinessComplete {
		if r.Substep.Status != manageddiagnostic.Observed || r.Substep.Value != manageddiagnostic.SubstepInitializedNotification {
			return errors.New("V3 readiness diagnostic requires substep")
		}
	} else if r.Substep.Status == manageddiagnostic.Observed {
		return errors.New("V3 diagnostic substep does not belong to phase")
	}
	if (r.Terminal == manageddiagnostic.TerminalProcessExited) != (r.ProcessExitStatus == manageddiagnostic.Observed) {
		return errors.New("V3 process exit projection incoherent")
	}
	if r.Phase != manageddiagnostic.PhaseCapabilityCheck && r.CallHierarchy.Status == manageddiagnostic.Observed {
		return errors.New("V3 call hierarchy fact does not belong to phase")
	}
	if r.Phase != manageddiagnostic.PhaseDocumentSupply && r.DocumentSupplyCompleted.Status == manageddiagnostic.Observed {
		return errors.New("V3 document supply fact does not belong to phase")
	}
	return nil
}

func validateDiagnosticsV3(d DiagnosticsV3) error {
	if d.MaxRecords != MaxDiagnosticRecords || d.MaxBytes != MaxDiagnosticBytes || d.Records == nil || len(d.Records) > MaxDiagnosticRecords || d.RetainedBytes < 0 || d.RetainedBytes > MaxDiagnosticBytes {
		return errors.New("V3 diagnostic bounds invalid")
	}
	if d.Status != manageddiagnostic.QueryAvailable && d.Status != manageddiagnostic.QueryUnavailable && d.Status != manageddiagnostic.QueryEvicted {
		return errors.New("V3 diagnostic status invalid")
	}
	last, retained := uint64(0), 0
	for _, r := range d.Records {
		if r.Sequence <= last {
			return errors.New("V3 diagnostic sequence invalid")
		}
		last = r.Sequence
		if err := validateDiagnosticRecordV3(r); err != nil {
			return err
		}
		encoded, err := json.Marshal(r)
		if err != nil || retained > MaxDiagnosticBytes-len(encoded) {
			return errors.New("V3 diagnostic byte accounting overflow")
		}
		retained += len(encoded)
	}
	if retained != d.RetainedBytes {
		return errors.New("V3 retained diagnostic bytes mismatch")
	}
	switch d.Status {
	case manageddiagnostic.QueryUnavailable:
		if len(d.Records) != 0 || d.EvictedRecords != 0 || d.OmittedRecords != 0 || retained != 0 {
			return errors.New("V3 unavailable diagnostic accounting invalid")
		}
	case manageddiagnostic.QueryEvicted:
		if len(d.Records) != 0 || d.EvictedRecords == 0 || d.OmittedRecords != 0 || retained != 0 {
			return errors.New("V3 evicted diagnostic accounting invalid")
		}
	case manageddiagnostic.QueryAvailable:
		if d.OmittedRecords > 0 && len(d.Records) != MaxDiagnosticRecords {
			return errors.New("V3 omitted diagnostic accounting invalid")
		}
	}
	return nil
}

func validateForV3(raw []byte) (string, error) {
	if len(raw) > MaxEnvelopeBytesV2+MaxDiagnosticBytes {
		return "", errors.New("V3 envelope byte limit")
	}
	// Structural schema admission intentionally precedes all typed semantics.
	if _, err := schema.ValidateStructure(raw, Family, "v3"); err != nil {
		return "", err
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
	var authority EvidenceV2
	if err := json.Unmarshal(v2, &authority); err != nil {
		return "", err
	}
	if e.SessionID != authority.Acquisition.Request.Context.SessionID || e.Generation != authority.Acquisition.Request.Context.Generation {
		return "", errors.New("V3 identity differs from embedded V2 authority")
	}
	sum := sha256.Sum256(v2)
	if e.GraphProvenanceV2SHA256 != fmt.Sprintf("sha256:%x", sum) {
		return "", errors.New("V3 embedded V2 digest mismatch")
	}
	if err := validateDiagnosticsV3(e.Diagnostics); err != nil {
		return "", err
	}
	return VersionV3, nil
}
