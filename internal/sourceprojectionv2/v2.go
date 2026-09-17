package sourceprojectionv2

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"

	"lsp-trace/internal/sourceprojection"
)

const SchemaVersion = "lsp-trace.source-projection.v2"

type DocumentSource struct {
	URI              string
	DocumentVersion  int
	PositionEncoding string
	SourceDigest     string
	SourceByteLength int
}

type Input struct {
	TargetURI          string
	SelectedURIs       []string
	Documents          []DocumentSource
	Candidates         []sourceprojection.Candidate
	Projection         sourceprojection.Result
	DocumentsObserved  int
	TotalAcquiredBytes int
	RequestPolicyID    string
}

type DocumentSelection struct {
	Ordering     string   `json:"ordering"`
	TargetURI    string   `json:"target_uri"`
	SelectedURIs []string `json:"selected_uris"`
}

type DocumentBinding struct {
	URI              string `json:"uri"`
	Role             string `json:"role"`
	Ordinal          int    `json:"ordinal"`
	Status           string `json:"status"`
	DocumentVersion  int    `json:"document_version,omitempty"`
	PositionEncoding string `json:"position_encoding,omitempty"`
	SourceDigest     string `json:"source_digest,omitempty"`
	SourceByteLength int    `json:"source_byte_length,omitempty"`
}

type DocumentAccounting struct {
	Candidates         int `json:"candidates"`
	Selected           int `json:"selected"`
	Acquired           int `json:"acquired"`
	Unavailable        int `json:"unavailable"`
	Withheld           int `json:"withheld"`
	LimitOmitted       int `json:"limit_omitted"`
	TotalAcquiredBytes int `json:"total_acquired_bytes"`
}

type DisplayProvenance struct {
	Kind   string `json:"kind"`
	Method string `json:"method"`
}

type Unit struct {
	UnitID                string                  `json:"unit_id"`
	Role                  string                  `json:"role"`
	GraphSubjectID        string                  `json:"graph_subject_id"`
	OccurrenceID          string                  `json:"occurrence_id,omitempty"`
	LogicalSourceID       string                  `json:"logical_source_id"`
	EvidenceRange         sourceprojection.Range  `json:"evidence_range"`
	DisplayRange          sourceprojection.Range  `json:"display_range"`
	DisplayProvenance     DisplayProvenance       `json:"display_provenance"`
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

type Citation struct {
	CitationID    string                 `json:"citation_id"`
	UnitID        string                 `json:"unit_id"`
	Role          string                 `json:"role"`
	SubjectID     string                 `json:"subject_id"`
	OccurrenceID  string                 `json:"occurrence_id,omitempty"`
	EvidenceRange sourceprojection.Range `json:"evidence_range"`
	DisplayRange  sourceprojection.Range `json:"display_range"`
}

type WireResult[T any] struct {
	SchemaVersion        string                          `json:"schema_version"`
	Authority            int                             `json:"authority"`
	SourceGraphComplete  string                          `json:"source_graph_complete"`
	GraphFactsAdded      int                             `json:"graph_facts_added"`
	CustodyMode          string                          `json:"custody_mode"`
	CustodyBinding       T                               `json:"custody_binding"`
	PhysicalProjectionID string                          `json:"physical_projection_id"`
	RequestPolicyID      string                          `json:"request_policy_id"`
	Status               string                          `json:"status"`
	DocumentSelection    DocumentSelection               `json:"document_selection"`
	DocumentBindings     []DocumentBinding               `json:"document_bindings"`
	DocumentAccounting   DocumentAccounting              `json:"document_accounting"`
	Units                []Unit                          `json:"units"`
	Citations            []Citation                      `json:"citations"`
	EmittedSpans         []sourceprojection.Span         `json:"emitted_spans"`
	Accounting           sourceprojection.Accounting     `json:"accounting"`
	Omissions            []sourceprojection.Omission     `json:"omissions"`
	PrivacySummary       sourceprojection.PrivacySummary `json:"privacy_summary"`
}

func AssembleBounded[T any](input Input, custodyMode string, custodyBinding T, maxResponseBytes int) (WireResult[T], error) {
	if input.TargetURI == "" || len(input.SelectedURIs) == 0 || input.SelectedURIs[0] != input.TargetURI {
		return WireResult[T]{}, errors.New("sourceprojectionv2: document selection must be target-first")
	}
	selectedSeen := make(map[string]struct{}, len(input.SelectedURIs))
	for ordinal, uri := range input.SelectedURIs {
		if uri == "" {
			return WireResult[T]{}, errors.New("sourceprojectionv2: selected document URI is empty")
		}
		if _, exists := selectedSeen[uri]; exists {
			return WireResult[T]{}, fmt.Errorf("sourceprojectionv2: duplicate selected document %q", uri)
		}
		selectedSeen[uri] = struct{}{}
		if ordinal > 1 && input.SelectedURIs[ordinal-1] > uri {
			return WireResult[T]{}, errors.New("sourceprojectionv2: additional documents must be URI-lexicographic")
		}
	}
	byURI := make(map[string]DocumentSource, len(input.Documents))
	for _, document := range input.Documents {
		if document.URI == "" {
			return WireResult[T]{}, errors.New("sourceprojectionv2: document binding URI is empty")
		}
		if _, exists := byURI[document.URI]; exists {
			return WireResult[T]{}, fmt.Errorf("sourceprojectionv2: duplicate document binding %q", document.URI)
		}
		byURI[document.URI] = document
	}
	documents := make([]DocumentBinding, 0, len(input.SelectedURIs))
	for ordinal, uri := range input.SelectedURIs {
		source, ok := byURI[uri]
		if !ok {
			return WireResult[T]{}, fmt.Errorf("sourceprojectionv2: selected document %q has no physical binding", uri)
		}
		role := "ADDITIONAL"
		if uri == input.TargetURI {
			role = "TARGET"
		}
		documents = append(documents, DocumentBinding{URI: uri, Role: role, Ordinal: ordinal, Status: "ACQUIRED", DocumentVersion: source.DocumentVersion, PositionEncoding: source.PositionEncoding, SourceDigest: source.SourceDigest, SourceByteLength: source.SourceByteLength})
	}
	candidateByID := make(map[string]sourceprojection.Candidate, len(input.Candidates))
	for _, candidate := range input.Candidates {
		if candidate.UnitID == "" {
			return WireResult[T]{}, errors.New("sourceprojectionv2: candidate unit ID is empty")
		}
		if _, exists := candidateByID[candidate.UnitID]; exists {
			return WireResult[T]{}, fmt.Errorf("sourceprojectionv2: duplicate candidate unit %q", candidate.UnitID)
		}
		candidateByID[candidate.UnitID] = candidate
	}
	unitSeen := make(map[string]struct{}, len(input.Projection.Units))
	units := make([]Unit, 0, len(input.Projection.Units))
	for _, unit := range input.Projection.Units {
		if _, exists := unitSeen[unit.UnitID]; exists {
			return WireResult[T]{}, fmt.Errorf("sourceprojectionv2: duplicate projected unit %q", unit.UnitID)
		}
		unitSeen[unit.UnitID] = struct{}{}
		candidate, ok := candidateByID[unit.UnitID]
		if !ok {
			return WireResult[T]{}, fmt.Errorf("sourceprojectionv2: unit %q has no retained candidate", unit.UnitID)
		}
		kind := displayKind(candidate)
		out := Unit{UnitID: unit.UnitID, Role: unit.Role, GraphSubjectID: unit.GraphSubjectID, OccurrenceID: unit.OccurrenceID, LogicalSourceID: unit.LogicalSourceID, EvidenceRange: candidate.EvidenceRange, DisplayRange: candidate.Range, DisplayProvenance: DisplayProvenance{Kind: kind, Method: displayMethod(kind)}, PositionEncoding: unit.PositionEncoding, SourceDigest: unit.SourceDigest, SourceByteLength: unit.SourceByteLength, SelectionDisposition: unit.SelectionDisposition, BodyDisposition: unit.BodyDisposition, PrivacyClassification: unit.PrivacyClassification, RelationProvenance: unit.RelationProvenance, Body: unit.Body}
		if candidate.Role == "ENDPOINT" {
			item, selection := candidate.ItemRange, candidate.SelectionRange
			out.ItemRange, out.SelectionRange = &item, &selection
		}
		units = append(units, out)
	}
	citations := make([]Citation, 0, len(input.Projection.Citations))
	citationSeen := make(map[string]struct{}, len(input.Projection.Citations))
	for _, citation := range input.Projection.Citations {
		if citation.CitationID == "" {
			return WireResult[T]{}, errors.New("sourceprojectionv2: citation ID is empty")
		}
		if _, exists := citationSeen[citation.CitationID]; exists {
			return WireResult[T]{}, fmt.Errorf("sourceprojectionv2: duplicate citation %q", citation.CitationID)
		}
		citationSeen[citation.CitationID] = struct{}{}
		candidate, ok := candidateByID[citation.UnitID]
		if !ok {
			return WireResult[T]{}, fmt.Errorf("sourceprojectionv2: citation %q has no retained candidate", citation.CitationID)
		}
		citations = append(citations, Citation{CitationID: citation.CitationID, UnitID: citation.UnitID, Role: citation.Role, SubjectID: citation.SubjectID, OccurrenceID: citation.OccurrenceID, EvidenceRange: candidate.EvidenceRange, DisplayRange: candidate.Range})
	}
	physicalID, err := physicalProjectionID(documents)
	if err != nil {
		return WireResult[T]{}, err
	}
	result := WireResult[T]{
		SchemaVersion: SchemaVersion, Authority: 0, SourceGraphComplete: "UNKNOWN", GraphFactsAdded: 0,
		CustodyMode: custodyMode, CustodyBinding: custodyBinding, PhysicalProjectionID: physicalID, RequestPolicyID: input.RequestPolicyID, Status: input.Projection.Status,
		DocumentSelection: DocumentSelection{Ordering: "TARGET_FIRST_THEN_URI_LEXICOGRAPHIC", TargetURI: input.TargetURI, SelectedURIs: append([]string(nil), input.SelectedURIs...)},
		DocumentBindings:  documents, DocumentAccounting: DocumentAccounting{Candidates: input.DocumentsObserved, Selected: len(input.SelectedURIs), Acquired: len(documents), TotalAcquiredBytes: input.TotalAcquiredBytes},
		Units: units, Citations: citations, EmittedSpans: append([]sourceprojection.Span{}, input.Projection.EmittedSpans...), Accounting: input.Projection.Accounting,
		Omissions: append([]sourceprojection.Omission{}, input.Projection.Omissions...), PrivacySummary: input.Projection.PrivacySummary,
	}
	raw, err := json.Marshal(result)
	if err != nil {
		return WireResult[T]{}, fmt.Errorf("sourceprojectionv2: encode V2 result: %w", err)
	}
	if maxResponseBytes <= 0 || len(raw) > maxResponseBytes {
		return WireResult[T]{}, fmt.Errorf("sourceprojectionv2: V2 response bytes %d exceed limit %d", len(raw), maxResponseBytes)
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

func physicalProjectionID(documents []DocumentBinding) (string, error) {
	raw, err := json.Marshal(documents)
	if err != nil {
		return "", fmt.Errorf("sourceprojectionv2: encode document identities: %w", err)
	}
	digest := sha256.Sum256(raw)
	return "sha256:" + hex.EncodeToString(digest[:]), nil
}
