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

type SeedSpecEvidence struct {
	SchemaVersion string `json:"schema_version"`
	Encoding      string `json:"encoding"`
	Bytes         []byte `json:"bytes"`
	SHA256        string `json:"sha256"`
}

type EvidenceV5 struct {
	SchemaVersion          string            `json:"schema_version"`
	SessionID              string            `json:"session_id"`
	Generation             uint64            `json:"generation"`
	GraphV5                string            `json:"graph_v5"`
	GraphV5SHA256          string            `json:"graph_v5_sha256"`
	GraphV5SchemaID        string            `json:"graph_v5_schema_id"`
	SourcePolicy           string            `json:"source_policy"`
	WorkspaceURI           string            `json:"workspace_uri"`
	AnalyzedVersion        string            `json:"analyzed_version"`
	DependencyCompleteness string            `json:"dependency_completeness"`
	CaptureBudget          CaptureBudgetV2   `json:"capture_budget"`
	Supplies               []SupplyReceiptV2 `json:"supplies"`
	Captures               []Receipt         `json:"captures"`
	Bindings               []BindingV2       `json:"bindings"`
	SeedSpec               *SeedSpecEvidence `json:"seed_spec,omitempty"`
	Diagnostics            DiagnosticsV3     `json:"diagnostics"`
}

// CaptureV5 retains the native graph.v5 serialization exactly. It deliberately
// does not project sibling candidates into CALLS or any support relation.
func CaptureV5(native []byte, sessionID string, generation uint64, query manageddiagnostic.QueryResult, source ...*EvidenceV2) ([]byte, error) {
	return captureV5(native, sessionID, generation, query, nil, source...)
}

func CaptureV5WithSeedSpec(native []byte, sessionID string, generation uint64, query manageddiagnostic.QueryResult, seedSpec []byte, source ...*EvidenceV2) ([]byte, error) {
	if err := validateSeedSpecBytes(seedSpec); err != nil {
		return nil, err
	}
	return captureV5(native, sessionID, generation, query, seedSpec, source...)
}

func captureV5(native []byte, sessionID string, generation uint64, query manageddiagnostic.QueryResult, seedSpec []byte, source ...*EvidenceV2) ([]byte, error) {
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
	e := EvidenceV5{SchemaVersion: VersionV5, SessionID: sessionID, Generation: generation, GraphV5: base64.StdEncoding.EncodeToString(native), GraphV5SHA256: fmt.Sprintf("sha256:%x", sum), GraphV5SchemaID: GraphV5SchemaID, SourcePolicy: PolicyV2, Supplies: []SupplyReceiptV2{}, Captures: []Receipt{}, Bindings: []BindingV2{}, Diagnostics: d}
	if seedSpec != nil {
		seedSum := sha256.Sum256(seedSpec)
		e.SeedSpec = &SeedSpecEvidence{SchemaVersion: "lsp-trace.seeds.v2", Encoding: "canonical-json", Bytes: append([]byte(nil), seedSpec...), SHA256: fmt.Sprintf("sha256:%x", seedSum)}
	}
	if len(source) > 1 || (len(source) == 1 && source[0] == nil) {
		return nil, errors.New("V5 requires at most one source snapshot")
	}
	if len(source) == 1 {
		s := source[0]
		if s.SchemaVersion != VersionV2 || s.Policy != PolicyV2 {
			return nil, errors.New("V5 source snapshot must be admitted provenance v2")
		}
		e.WorkspaceURI, e.AnalyzedVersion, e.DependencyCompleteness, e.CaptureBudget = s.WorkspaceURI, s.AnalyzedVersion, s.DependencyCompleteness, s.CaptureBudget
		e.Supplies = append(e.Supplies, s.Supplies...)
		e.Captures = append(e.Captures, s.Captures...)
		e.Bindings = append(e.Bindings, s.Bindings...)
		receipts := map[string][]string{}
		for _, r := range e.Captures {
			receipts[r.URI] = append(receipts[r.URI], r.ID)
		}
		for _, sr := range e.Supplies {
			if sr.Receipt != nil {
				receipts[sr.Receipt.URI] = append(receipts[sr.Receipt.URI], sr.Receipt.ID)
			}
		}
		var g graph.Result
		if err := json.Unmarshal(native, &g); err != nil {
			return nil, err
		}
		for i, relation := range g.SiblingCandidates {
			for _, endpoint := range []struct {
				name string
				node graph.Node
			}{{"origin", relation.Origin}, {"candidate", relation.Candidate}} {
				ids := append([]string{}, receipts[endpoint.node.URI]...)
				e.Bindings = append(e.Bindings, BindingV2{Pointer: fmt.Sprintf("/graph/sibling_candidates/%d/%s/range", i, endpoint.name), URI: endpoint.node.URI, Attribution: "SOURCE", AnchorStatus: "VALID", ReceiptIDs: ids})
			}
		}
	}
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
	if e.SchemaVersion != VersionV5 || e.SessionID == "" || e.Generation == 0 || e.GraphV5SchemaID != GraphV5SchemaID || e.SourcePolicy != PolicyV2 || e.Supplies == nil || e.Captures == nil || e.Bindings == nil {
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
	receipts := map[string]bool{}
	for _, r := range e.Captures {
		if receipts[r.ID] || validateReceiptV2(e.WorkspaceURI, r) != nil {
			return "", errors.New("V5 invalid or duplicate source capture")
		}
		receipts[r.ID] = true
	}
	for _, supply := range e.Supplies {
		if supply.Receipt == nil {
			continue
		}
		r := *supply.Receipt
		if receipts[r.ID] || validateReceiptV2(e.WorkspaceURI, r) != nil || validateSupplyV2(&r) != nil {
			return "", errors.New("V5 invalid or duplicate source supply")
		}
		receipts[r.ID] = true
	}
	for _, binding := range e.Bindings {
		for _, id := range binding.ReceiptIDs {
			if !receipts[id] {
				return "", errors.New("V5 source binding foreign key")
			}
		}
	}
	sum := sha256.Sum256(native)
	if e.GraphV5SHA256 != fmt.Sprintf("sha256:%x", sum) {
		return "", errors.New("V5 embedded graph digest mismatch")
	}
	if e.SeedSpec != nil {
		if err := validateSeedSpecEvidence(*e.SeedSpec); err != nil {
			return "", err
		}
	}
	if err := validateDiagnosticsV3(e.Diagnostics); err != nil {
		return "", err
	}
	return VersionV5, nil
}

func validateSeedSpecBytes(raw []byte) error {
	if len(raw) == 0 || len(raw) > 1<<20 {
		return errors.New("V5 seed spec unavailable or exceeds byte limit")
	}
	if err := strictjson.RejectDuplicates(raw); err != nil {
		return err
	}
	var header struct {
		SchemaVersion string `json:"schema_version"`
	}
	if err := json.Unmarshal(raw, &header); err != nil || header.SchemaVersion != "lsp-trace.seeds.v2" {
		return errors.New("V5 seed spec is not lsp-trace.seeds.v2 JSON")
	}
	return nil
}

func validateSeedSpecEvidence(spec SeedSpecEvidence) error {
	if spec.SchemaVersion != "lsp-trace.seeds.v2" || spec.Encoding != "canonical-json" {
		return errors.New("V5 seed spec identity invalid")
	}
	if err := validateSeedSpecBytes(spec.Bytes); err != nil {
		return err
	}
	sum := sha256.Sum256(spec.Bytes)
	if spec.SHA256 != fmt.Sprintf("sha256:%x", sum) {
		return errors.New("V5 seed spec digest mismatch")
	}
	return nil
}
