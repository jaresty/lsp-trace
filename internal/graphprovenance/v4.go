package graphprovenance

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"lsp-trace/internal/acquisition"
	"lsp-trace/internal/graph"
	"lsp-trace/internal/manageddiagnostic"
	"lsp-trace/internal/schema"
	"lsp-trace/internal/strictjson"
)

const (
	VersionV4          = "lsp-trace.graph-provenance.v4"
	PolicyV4           = "DIRECT_TYPED_ACQUISITION_WITH_DIAGNOSTICS_V4"
	MaxEnvelopeBytesV4 = MaxEnvelopeBytesV2 + MaxDiagnosticBytes
	MaxOmissionsV4     = 100000
)

type EvidenceV4 struct {
	SchemaVersion     string             `json:"schema_version"`
	Policy            string             `json:"policy"`
	SessionID         string             `json:"session_id"`
	Generation        uint64             `json:"generation"`
	Acquisition       acquisition.Result `json:"acquisition"`
	AcquisitionSHA256 string             `json:"acquisition_sha256"`
	GraphSHA256       string             `json:"graph_sha256"`
	Diagnostics       DiagnosticsV3      `json:"diagnostics"`
}

type evidenceV4Alias EvidenceV4

func (e *EvidenceV4) UnmarshalJSON(raw []byte) error {
	var wire struct {
		evidenceV4Alias
		Acquisition json.RawMessage `json:"acquisition"`
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(&wire); err != nil {
		return err
	}
	*e = EvidenceV4(wire.evidenceV4Alias)
	var acquisitionWire struct {
		*acquisition.Result
		Graph json.RawMessage `json:"graph"`
	}
	acquisitionWire.Result = &e.Acquisition
	d = json.NewDecoder(bytes.NewReader(wire.Acquisition))
	d.DisallowUnknownFields()
	if err := d.Decode(&acquisitionWire); err != nil {
		return err
	}
	decoded, err := graph.DecodeNativeV3(acquisitionWire.Graph)
	if err != nil {
		return err
	}
	e.Acquisition.Graph = decoded
	return nil
}

func projectDiagnostics(query manageddiagnostic.QueryResult, sessionID string, generation uint64) (DiagnosticsV3, error) {
	d := DiagnosticsV3{Status: query.Status, Records: []DiagnosticRecordV3{}, EvictedRecords: query.EvictedRecords, MaxRecords: MaxDiagnosticRecords, MaxBytes: MaxDiagnosticBytes}
	var last uint64
	for _, r := range query.Records {
		if err := manageddiagnostic.Validate(r); err != nil || r.SessionID != sessionID || r.Generation != generation || r.Sequence <= last {
			return DiagnosticsV3{}, errors.New("diagnostic identity or ordering invalid")
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
		return DiagnosticsV3{}, err
	}
	return d, nil
}

// CaptureV4 requires the caller to grant exclusive read ownership of result,
// query, and every reachable slice, map, and raw byte for the duration of the
// call. Concurrent unsynchronized caller mutation is invalid and outside this
// contract. On return, the encoded bytes and captured evidence are fully
// detached: the caller may freely reuse or mutate every input-owned value.
func CaptureV4(result acquisition.Result, query manageddiagnostic.QueryResult) ([]byte, error) {
	// Establish one detached capture boundary before validation or hashing; no
	// caller-owned backing array enters the evidence value returned from this call.
	snapshotWire, err := json.Marshal(EvidenceV4{Acquisition: result})
	if err != nil {
		return nil, err
	}
	var detached EvidenceV4
	if err := json.Unmarshal(snapshotWire, &detached); err != nil {
		return nil, err
	}
	result = detached.Acquisition
	if len(result.Omissions) > MaxOmissionsV4 {
		return nil, errors.New("V4 omission count exceeds bound")
	}
	if err := acquisition.ValidateResult(result); err != nil {
		return nil, err
	}
	sessionID, generation := result.Request.Context.SessionID, result.Request.Context.Generation
	if sessionID == "" || generation == 0 {
		return nil, errors.New("V4 exact session and generation required")
	}
	diagnostics, err := projectDiagnostics(query, sessionID, generation)
	if err != nil {
		return nil, err
	}
	acquisitionBytes, err := json.Marshal(result)
	if err != nil {
		return nil, err
	}
	graphBytes, err := json.Marshal(result.Graph)
	if err != nil {
		return nil, err
	}
	e := EvidenceV4{
		SchemaVersion:     VersionV4,
		Policy:            PolicyV4,
		SessionID:         sessionID,
		Generation:        generation,
		Acquisition:       result,
		AcquisitionSHA256: digestV4(VersionV4+":acquisition", acquisitionBytes),
		GraphSHA256:       digestV4(VersionV4+":graph", graphBytes),
		Diagnostics:       diagnostics,
	}
	raw, err := json.Marshal(e)
	if err != nil {
		return nil, err
	}
	raw = append(raw, '\n')
	if _, err := validateForV4(raw); err != nil {
		return nil, err
	}
	return raw, nil
}

func digestV4(domain string, raw []byte) string {
	sum := sha256.Sum256(append(append([]byte(domain), 0), raw...))
	return fmt.Sprintf("sha256:%x", sum)
}

func validateForV4(raw []byte) (string, error) {
	if len(raw) == 0 || len(raw) > MaxEnvelopeBytesV4 {
		return "", errors.New("V4 envelope byte limit")
	}
	if err := preflightV2(raw, MaxEnvelopeBytesV4); err != nil {
		return "", err
	}
	if _, err := schema.ValidateStructure(raw, Family, "v4"); err != nil {
		return "", err
	}
	if err := strictjson.RejectDuplicates(raw); err != nil {
		return "", err
	}
	var e EvidenceV4
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(&e); err != nil {
		return "", err
	}
	if err := d.Decode(&struct{}{}); err != io.EOF {
		return "", errors.New("V4 trailing JSON")
	}
	if e.SchemaVersion != VersionV4 || e.Policy != PolicyV4 || e.SessionID == "" || e.Generation == 0 || e.Acquisition.Request.Context.SessionID != e.SessionID || e.Acquisition.Request.Context.Generation != e.Generation {
		return "", errors.New("invalid V4 identity")
	}
	if len(e.Acquisition.Omissions) > MaxOmissionsV4 {
		return "", errors.New("V4 omission count exceeds bound")
	}
	if err := acquisition.ValidateResult(e.Acquisition); err != nil {
		return "", err
	}
	if err := validateDiagnosticsV3(e.Diagnostics); err != nil {
		return "", err
	}
	acquisitionBytes, err := json.Marshal(e.Acquisition)
	if err != nil || e.AcquisitionSHA256 != digestV4(VersionV4+":acquisition", acquisitionBytes) {
		return "", errors.New("V4 acquisition digest mismatch")
	}
	graphBytes, err := json.Marshal(e.Acquisition.Graph)
	if err != nil || e.GraphSHA256 != digestV4(VersionV4+":graph", graphBytes) {
		return "", errors.New("V4 graph digest mismatch")
	}
	canonical, err := json.Marshal(e)
	if err != nil || !bytes.Equal(bytes.TrimSuffix(raw, []byte{'\n'}), canonical) {
		return "", errors.New("V4 noncanonical encoding")
	}
	return VersionV4, nil
}
