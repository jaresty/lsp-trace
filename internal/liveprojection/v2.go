package liveprojection

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"

	"lsp-trace/internal/sourceprojection"
)

const V2SchemaVersion = "lsp-trace.source-projection.v2"

type V2LiveBinding struct {
	Custody          string `json:"custody"`
	SessionID        string `json:"session_id"`
	Generation       uint64 `json:"generation"`
	PositionEncoding string `json:"position_encoding"`
}

type V2DocumentSelection struct {
	Ordering     string   `json:"ordering"`
	TargetURI    string   `json:"target_uri"`
	SelectedURIs []string `json:"selected_uris"`
}

type V2DocumentBinding struct {
	URI              string `json:"uri"`
	Role             string `json:"role"`
	Ordinal          int    `json:"ordinal"`
	Status           string `json:"status"`
	DocumentVersion  int    `json:"document_version,omitempty"`
	PositionEncoding string `json:"position_encoding,omitempty"`
	SourceDigest     string `json:"source_digest,omitempty"`
	SourceByteLength int    `json:"source_byte_length,omitempty"`
}

type V2DocumentAccounting struct {
	Candidates         int `json:"candidates"`
	Selected           int `json:"selected"`
	Acquired           int `json:"acquired"`
	Unavailable        int `json:"unavailable"`
	Withheld           int `json:"withheld"`
	LimitOmitted       int `json:"limit_omitted"`
	TotalAcquiredBytes int `json:"total_acquired_bytes"`
}

type V2DisplayProvenance struct {
	Kind   string `json:"kind"`
	Method string `json:"method"`
}

type V2Unit struct {
	UnitID                string                  `json:"unit_id"`
	Role                  string                  `json:"role"`
	GraphSubjectID        string                  `json:"graph_subject_id"`
	OccurrenceID          string                  `json:"occurrence_id,omitempty"`
	LogicalSourceID       string                  `json:"logical_source_id"`
	EvidenceRange         sourceprojection.Range  `json:"evidence_range"`
	DisplayRange          sourceprojection.Range  `json:"display_range"`
	DisplayProvenance     V2DisplayProvenance     `json:"display_provenance"`
	ItemRange             *sourceprojection.Range `json:"item_range,omitempty"`
	SelectionRange        *sourceprojection.Range `json:"selection_range,omitempty"`
	PositionEncoding      string                  `json:"position_encoding"`
	SourceDigest          string                  `json:"source_digest"`
	SourceByteLength      int                     `json:"source_byte_length"`
	SelectionDisposition  string                  `json:"selection_disposition"`
	BodyDisposition       string                  `json:"body_disposition"`
	PrivacyClassification string                  `json:"privacy_classification"`
	RelationProvenance    string                  `json:"relation_provenance,omitempty"`
	Body                  string                  `json:"body,omitempty"`
}

type V2Citation struct {
	CitationID    string                 `json:"citation_id"`
	UnitID        string                 `json:"unit_id"`
	Role          string                 `json:"role"`
	SubjectID     string                 `json:"subject_id"`
	OccurrenceID  string                 `json:"occurrence_id,omitempty"`
	EvidenceRange sourceprojection.Range `json:"evidence_range"`
	DisplayRange  sourceprojection.Range `json:"display_range"`
}

type V2WireResult struct {
	SchemaVersion        string                          `json:"schema_version"`
	Authority            int                             `json:"authority"`
	SourceGraphComplete  string                          `json:"source_graph_complete"`
	GraphFactsAdded      int                             `json:"graph_facts_added"`
	CustodyMode          string                          `json:"custody_mode"`
	CustodyBinding       V2LiveBinding                   `json:"custody_binding"`
	PhysicalProjectionID string                          `json:"physical_projection_id"`
	RequestPolicyID      string                          `json:"request_policy_id"`
	Status               string                          `json:"status"`
	DocumentSelection    V2DocumentSelection             `json:"document_selection"`
	DocumentBindings     []V2DocumentBinding             `json:"document_bindings"`
	DocumentAccounting   V2DocumentAccounting            `json:"document_accounting"`
	Units                []V2Unit                        `json:"units"`
	Citations            []V2Citation                    `json:"citations"`
	EmittedSpans         []sourceprojection.Span         `json:"emitted_spans"`
	Accounting           sourceprojection.Accounting     `json:"accounting"`
	Omissions            []sourceprojection.Omission     `json:"omissions"`
	PrivacySummary       sourceprojection.PrivacySummary `json:"privacy_summary"`
}

func AssembleV2Bounded(composed CompositionResult, prepared PreparationResult, targetURI string, selectedURIs []string, requestPolicyID string, maxResponseBytes int) (V2WireResult, error) {
	if composed.Status != CompositionComplete || prepared.Status != PreparationComplete {
		return V2WireResult{}, errors.New("liveprojection: V2 assembly requires complete composition and preparation")
	}
	if targetURI == "" || len(selectedURIs) == 0 || selectedURIs[0] != targetURI {
		return V2WireResult{}, errors.New("liveprojection: V2 document selection must be target-first")
	}
	byURI := make(map[string]sourceprojection.LiveBinding, len(composed.Resolutions))
	for _, resolution := range composed.Resolutions {
		byURI[resolution.Binding.URI] = resolution.Binding
	}
	documents := make([]V2DocumentBinding, 0, len(selectedURIs))
	for ordinal, uri := range selectedURIs {
		binding, ok := byURI[uri]
		if !ok {
			return V2WireResult{}, fmt.Errorf("liveprojection: selected document %q has no physical binding", uri)
		}
		role := "ADDITIONAL"
		if uri == targetURI {
			role = "TARGET"
		}
		documents = append(documents, V2DocumentBinding{URI: uri, Role: role, Ordinal: ordinal, Status: "ACQUIRED", DocumentVersion: binding.DocumentVersion, PositionEncoding: binding.PositionEncoding, SourceDigest: binding.SourceDigest, SourceByteLength: binding.SourceByteLength})
	}
	candidateByID := make(map[string]sourceprojection.Candidate, len(composed.Candidates))
	for _, candidate := range composed.Candidates {
		candidateByID[candidate.UnitID] = candidate
	}
	units := make([]V2Unit, 0, len(composed.Projection.Units))
	for _, unit := range composed.Projection.Units {
		candidate, ok := candidateByID[unit.UnitID]
		if !ok {
			return V2WireResult{}, fmt.Errorf("liveprojection: unit %q has no retained candidate", unit.UnitID)
		}
		out := V2Unit{UnitID: unit.UnitID, Role: unit.Role, GraphSubjectID: unit.GraphSubjectID, OccurrenceID: unit.OccurrenceID, LogicalSourceID: unit.LogicalSourceID, EvidenceRange: candidate.EvidenceRange, DisplayRange: candidate.Range, DisplayProvenance: V2DisplayProvenance{Kind: displayKind(candidate), Method: displayMethod(displayKind(candidate))}, PositionEncoding: unit.PositionEncoding, SourceDigest: unit.SourceDigest, SourceByteLength: unit.SourceByteLength, SelectionDisposition: unit.SelectionDisposition, BodyDisposition: unit.BodyDisposition, PrivacyClassification: unit.PrivacyClassification, RelationProvenance: unit.RelationProvenance, Body: unit.Body}
		if candidate.Role == "ENDPOINT" {
			item, selection := candidate.ItemRange, candidate.SelectionRange
			out.ItemRange, out.SelectionRange = &item, &selection
		}
		units = append(units, out)
	}
	citations := make([]V2Citation, 0, len(composed.Projection.Citations))
	for _, citation := range composed.Projection.Citations {
		candidate, ok := candidateByID[citation.UnitID]
		if !ok {
			return V2WireResult{}, fmt.Errorf("liveprojection: citation %q has no retained candidate", citation.CitationID)
		}
		citations = append(citations, V2Citation{CitationID: citation.CitationID, UnitID: citation.UnitID, Role: citation.Role, SubjectID: citation.SubjectID, OccurrenceID: citation.OccurrenceID, EvidenceRange: candidate.EvidenceRange, DisplayRange: candidate.Range})
	}
	physicalID, err := physicalProjectionID(documents)
	if err != nil {
		return V2WireResult{}, err
	}
	result := V2WireResult{
		SchemaVersion: V2SchemaVersion, Authority: 0, SourceGraphComplete: "UNKNOWN", GraphFactsAdded: 0,
		CustodyMode: "LIVE", CustodyBinding: V2LiveBinding{Custody: "LIVE", SessionID: composed.Resolutions[0].Binding.SessionID, Generation: composed.Resolutions[0].Binding.Generation, PositionEncoding: "utf-16"},
		PhysicalProjectionID: physicalID, RequestPolicyID: requestPolicyID, Status: string(composed.Projection.Status),
		DocumentSelection:  V2DocumentSelection{Ordering: "TARGET_FIRST_THEN_URI_LEXICOGRAPHIC", TargetURI: targetURI, SelectedURIs: append([]string(nil), selectedURIs...)},
		DocumentBindings:   documents,
		DocumentAccounting: V2DocumentAccounting{Candidates: prepared.Accounting.Documents.Observed, Selected: len(selectedURIs), Acquired: len(documents), TotalAcquiredBytes: prepared.Accounting.Bytes.Observed},
		Units:              units, Citations: citations, EmittedSpans: append([]sourceprojection.Span(nil), composed.Projection.EmittedSpans...), Accounting: composed.Projection.Accounting,
		Omissions: append([]sourceprojection.Omission{}, composed.Projection.Omissions...), PrivacySummary: composed.Projection.PrivacySummary,
	}
	raw, err := json.Marshal(result)
	if err != nil {
		return V2WireResult{}, fmt.Errorf("liveprojection: encode V2 result: %w", err)
	}
	if maxResponseBytes <= 0 || len(raw) > maxResponseBytes {
		return V2WireResult{}, fmt.Errorf("liveprojection: V2 response bytes %d exceed limit %d", len(raw), maxResponseBytes)
	}
	return result, nil
}

func displayKind(candidate sourceprojection.Candidate) string {
	if candidate.DisplayProvenance != "" {
		return candidate.DisplayProvenance
	}
	if candidate.Role == "RELATION" {
		return "CALL_SITE_OCCURRENCE"
	}
	return "CALL_HIERARCHY_ITEM"
}

func displayMethod(kind string) string {
	if kind == "SERVER_REPORTED_DOCUMENT_SYMBOL" {
		return "textDocument/documentSymbol"
	}
	return "callHierarchy"
}

func physicalProjectionID(documents []V2DocumentBinding) (string, error) {
	raw, err := json.Marshal(documents)
	if err != nil {
		return "", fmt.Errorf("liveprojection: encode document identities: %w", err)
	}
	digest := sha256.Sum256(raw)
	return "sha256:" + hex.EncodeToString(digest[:]), nil
}
