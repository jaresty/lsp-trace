package sourceprojection

import (
	"errors"
	"fmt"
	"sort"

	"lsp-trace/internal/sourceposition"
)

const SchemaVersion = "lsp-trace.source-projection.v1"

type Position struct {
	Line      uint32 `json:"line"`
	Character uint32 `json:"character"`
}

type Range struct {
	Start Position `json:"start"`
	End   Position `json:"end"`
}

type Candidate struct {
	UnitID                string
	CitationID            string
	Role                  string
	GraphSubjectID        string
	OccurrenceID          string
	LogicalSourceID       string
	Range                 Range
	EvidenceRange         Range
	ItemRange             Range
	SelectionRange        Range
	DisplayProvenance     string
	PositionEncoding      string
	RelationProvenance    string
	PrivacyClassification string
	Withheld              bool
}

type Source struct {
	LogicalSourceID string
	Digest          string
	ByteLength      int
	Bytes           []byte
	Available       bool
}

type Policy struct {
	PolicyID      string
	BodyRequested bool
	MaxBytes      int
	MaxRanges     int
	MaxObjects    int
	MaxWork       int
	EnforceLimits bool
}

type Unit struct {
	UnitID                string `json:"unit_id"`
	Role                  string `json:"role"`
	GraphSubjectID        string `json:"graph_subject_id"`
	OccurrenceID          string `json:"occurrence_id,omitempty"`
	LogicalSourceID       string `json:"logical_source_id"`
	Range                 Range  `json:"range"`
	PositionEncoding      string `json:"position_encoding"`
	SourceDigest          string `json:"source_digest"`
	SourceByteLength      int    `json:"source_byte_length"`
	SelectionDisposition  string `json:"selection_disposition"`
	BodyDisposition       string `json:"body_disposition"`
	PrivacyClassification string `json:"privacy_classification"`
	RelationProvenance    string `json:"relation_provenance,omitempty"`
	Body                  string `json:"body,omitempty"`
}

type Citation struct {
	CitationID   string `json:"citation_id"`
	UnitID       string `json:"unit_id"`
	Role         string `json:"role"`
	SubjectID    string `json:"subject_id"`
	OccurrenceID string `json:"occurrence_id,omitempty"`
}

type Span struct {
	LogicalSourceID string   `json:"logical_source_id"`
	Range           Range    `json:"range"`
	SourceDigest    string   `json:"source_digest"`
	ByteLength      int      `json:"byte_length"`
	UnitIDs         []string `json:"unit_ids"`
	Body            string   `json:"body,omitempty"`
}

type Omission struct {
	UnitID string `json:"unit_id"`
	Cause  string `json:"cause"`
}

type Accounting struct {
	Candidates           int `json:"candidates"`
	Selected             int `json:"selected"`
	Omitted              int `json:"omitted"`
	LogicalSelectedBytes int `json:"logical_selected_bytes"`
	UniqueEmittedBytes   int `json:"unique_emitted_bytes"`
	Evaluated            int `json:"evaluated"`
	Terminal             int `json:"terminal"`
	Unevaluated          int `json:"unevaluated"`
}

type PrivacySummary struct {
	PolicyID      string `json:"policy_id"`
	BodyRequested bool   `json:"body_requested"`
	BodyReturned  int    `json:"body_returned"`
	BodyWithheld  int    `json:"body_withheld"`
}

type Result struct {
	Status         string
	Units          []Unit
	Citations      []Citation
	EmittedSpans   []Span
	Accounting     Accounting
	Omissions      []Omission
	PrivacySummary PrivacySummary
}

var ErrNotImplemented = errors.New("source projection core not implemented")

func Project(candidates []Candidate, sources map[string]Source, policy Policy) (Result, error) {
	result := Result{
		Units:        []Unit{},
		Citations:    []Citation{},
		EmittedSpans: []Span{},
		Omissions:    []Omission{},
		PrivacySummary: PrivacySummary{
			PolicyID:      policy.PolicyID,
			BodyRequested: policy.BodyRequested,
		},
	}
	ordered := append([]Candidate(nil), candidates...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].UnitID < ordered[j].UnitID })
	result.Accounting.Candidates = len(ordered)
	if policy.EnforceLimits && policy.MaxWork < len(ordered) {
		return Result{}, fmt.Errorf("projection work limit: need %d have %d", len(ordered), policy.MaxWork)
	}
	result.Accounting.Evaluated = len(ordered)
	result.Accounting.Terminal = len(ordered)
	if len(ordered) == 0 {
		result.Status = "SUCCESSFUL_EMPTY"
		return result, nil
	}
	seen := make(map[string]struct{}, len(ordered))
	encodingBySource := make(map[string]string)
	for _, candidate := range ordered {
		if !sourceposition.Supported(candidate.PositionEncoding) {
			return Result{}, fmt.Errorf("unsupported position encoding %q", candidate.PositionEncoding)
		}
		if candidate.UnitID == "" || candidate.CitationID == "" || candidate.GraphSubjectID == "" || candidate.LogicalSourceID == "" {
			return Result{}, errors.New("projection candidate identity is incomplete")
		}
		if candidate.PrivacyClassification == "" {
			return Result{}, errors.New("projection candidate privacy classification is empty")
		}
		if candidate.Role != "ENDPOINT" && candidate.Role != "RELATION" {
			return Result{}, fmt.Errorf("unsupported projection role %q", candidate.Role)
		}
		if previous, ok := encodingBySource[candidate.LogicalSourceID]; ok && previous != candidate.PositionEncoding {
			return Result{}, fmt.Errorf("source %q mixes position encodings %q and %q", candidate.LogicalSourceID, previous, candidate.PositionEncoding)
		}
		encodingBySource[candidate.LogicalSourceID] = candidate.PositionEncoding
		if candidate.Role == "RELATION" && candidate.RelationProvenance != "SERVER_REPORTED" {
			return Result{}, fmt.Errorf("relation unit %s provenance must be SERVER_REPORTED", candidate.UnitID)
		}
		if _, exists := seen[candidate.UnitID]; exists {
			return Result{}, fmt.Errorf("duplicate unit id %q", candidate.UnitID)
		}
		seen[candidate.UnitID] = struct{}{}
	}

	selected := make([]Candidate, 0, len(ordered))
	for _, candidate := range ordered {
		source, exists := sources[candidate.LogicalSourceID]
		if !exists || source.LogicalSourceID != candidate.LogicalSourceID || (!source.Available && policy.BodyRequested) {
			omit(&result, candidate.UnitID, "SOURCE_UNAVAILABLE")
			continue
		}
		if candidate.Withheld && policy.BodyRequested {
			omit(&result, candidate.UnitID, "POLICY_WITHHELD")
			result.PrivacySummary.BodyWithheld++
			continue
		}
		selected = append(selected, candidate)
	}

	if policy.EnforceLimits && len(selected) > policy.MaxObjects {
		priority := append([]Candidate(nil), selected...)
		sort.Slice(priority, func(i, j int) bool {
			pi, pj := rolePriority(priority[i].Role), rolePriority(priority[j].Role)
			if pi != pj {
				return pi < pj
			}
			return priority[i].UnitID < priority[j].UnitID
		})
		keep := make(map[string]struct{}, policy.MaxObjects)
		for _, candidate := range priority[:policy.MaxObjects] {
			keep[candidate.UnitID] = struct{}{}
		}
		selected = selected[:0]
		for _, candidate := range ordered {
			if _, ok := keep[candidate.UnitID]; ok {
				selected = append(selected, candidate)
			} else if !hasOmission(result.Omissions, candidate.UnitID) {
				omit(&result, candidate.UnitID, "OBJECT_LIMIT")
			}
		}
	}

	if policy.BodyRequested && (!policy.EnforceLimits || policy.MaxBytes >= 0) && len(selected) > 0 {
		spans, err := assembleSpans(selected, sources, false)
		if err != nil {
			return Result{}, err
		}
		uniqueBytes := 0
		for _, span := range spans {
			uniqueBytes += span.ByteLength
		}
		if (policy.EnforceLimits && uniqueBytes > policy.MaxBytes) || (!policy.EnforceLimits && policy.MaxBytes > 0 && uniqueBytes > policy.MaxBytes) {
			for _, candidate := range selected {
				omit(&result, candidate.UnitID, "BYTE_LIMIT")
			}
			selected = selected[:0]
		}
	}

	if policy.BodyRequested && ((policy.EnforceLimits && len(selected) > policy.MaxRanges) || (!policy.EnforceLimits && policy.MaxRanges > 0 && len(selected) > policy.MaxRanges)) {
		priority := append([]Candidate(nil), selected...)
		sort.Slice(priority, func(i, j int) bool {
			pi, pj := rolePriority(priority[i].Role), rolePriority(priority[j].Role)
			if pi != pj {
				return pi < pj
			}
			return priority[i].UnitID < priority[j].UnitID
		})
		keep := make(map[string]struct{}, policy.MaxRanges)
		for _, candidate := range priority[:policy.MaxRanges] {
			keep[candidate.UnitID] = struct{}{}
		}
		selected = selected[:0]
		for _, candidate := range ordered {
			if _, ok := keep[candidate.UnitID]; ok {
				selected = append(selected, candidate)
			} else if !hasOmission(result.Omissions, candidate.UnitID) {
				omit(&result, candidate.UnitID, "RANGE_LIMIT")
			}
		}
	}

	for _, candidate := range selected {
		source := sources[candidate.LogicalSourceID]
		unit := Unit{
			UnitID:                candidate.UnitID,
			Role:                  candidate.Role,
			GraphSubjectID:        candidate.GraphSubjectID,
			OccurrenceID:          candidate.OccurrenceID,
			LogicalSourceID:       candidate.LogicalSourceID,
			Range:                 candidate.Range,
			PositionEncoding:      candidate.PositionEncoding,
			SourceDigest:          source.Digest,
			SourceByteLength:      sourceByteLength(source),
			SelectionDisposition:  "SELECTED",
			PrivacyClassification: candidate.PrivacyClassification,
			RelationProvenance:    candidate.RelationProvenance,
		}
		if policy.BodyRequested {
			body, err := rangeBytes(source.Bytes, candidate.PositionEncoding, candidate.Range)
			if err != nil {
				return Result{}, fmt.Errorf("unit %s: %w", candidate.UnitID, err)
			}
			unit.BodyDisposition = "RETURNED"
			unit.Body = string(body)
			result.Accounting.LogicalSelectedBytes += len(body)
			result.PrivacySummary.BodyReturned++
		} else {
			unit.BodyDisposition = "NOT_REQUESTED"
		}
		result.Units = append(result.Units, unit)
		result.Citations = append(result.Citations, Citation{CitationID: candidate.CitationID, UnitID: candidate.UnitID, Role: candidate.Role, SubjectID: candidate.GraphSubjectID, OccurrenceID: candidate.OccurrenceID})
	}
	result.Accounting.Selected = len(result.Units)
	result.Accounting.Omitted = len(result.Omissions)
	if policy.BodyRequested && len(selected) > 0 {
		spans, err := assembleSpans(selected, sources, true)
		if err != nil {
			return Result{}, err
		}
		result.EmittedSpans = spans
		for _, span := range spans {
			result.Accounting.UniqueEmittedBytes += span.ByteLength
		}
	}
	result.Status = projectionStatus(result)
	return result, nil
}

func sourceByteLength(source Source) int {
	if source.ByteLength > 0 {
		return source.ByteLength
	}
	return len(source.Bytes)
}

func omit(result *Result, unitID, cause string) {
	result.Omissions = append(result.Omissions, Omission{UnitID: unitID, Cause: cause})
}

func hasOmission(omissions []Omission, unitID string) bool {
	for _, omission := range omissions {
		if omission.UnitID == unitID {
			return true
		}
	}
	return false
}

func rolePriority(role string) int {
	switch role {
	case "ENDPOINT":
		return 0
	case "RELATION":
		return 1
	default:
		return 2
	}
}

type boundedCandidate struct {
	candidate Candidate
	start     int
	end       int
}

func assembleSpans(candidates []Candidate, sources map[string]Source, includeBody bool) ([]Span, error) {
	bySource := make(map[string][]boundedCandidate)
	for _, candidate := range candidates {
		source := sources[candidate.LogicalSourceID]
		start, err := positionOffset(source.Bytes, candidate.PositionEncoding, candidate.Range.Start)
		if err != nil {
			return nil, fmt.Errorf("unit %s start: %w", candidate.UnitID, err)
		}
		end, err := positionOffset(source.Bytes, candidate.PositionEncoding, candidate.Range.End)
		if err != nil {
			return nil, fmt.Errorf("unit %s end: %w", candidate.UnitID, err)
		}
		if end < start {
			return nil, fmt.Errorf("unit %s: inverted range", candidate.UnitID)
		}
		bySource[candidate.LogicalSourceID] = append(bySource[candidate.LogicalSourceID], boundedCandidate{candidate: candidate, start: start, end: end})
	}
	sourceIDs := make([]string, 0, len(bySource))
	for sourceID := range bySource {
		sourceIDs = append(sourceIDs, sourceID)
	}
	sort.Strings(sourceIDs)
	spans := make([]Span, 0)
	for _, sourceID := range sourceIDs {
		items := bySource[sourceID]
		sort.Slice(items, func(i, j int) bool {
			if items[i].start != items[j].start {
				return items[i].start < items[j].start
			}
			if items[i].end != items[j].end {
				return items[i].end > items[j].end
			}
			return items[i].candidate.UnitID < items[j].candidate.UnitID
		})
		for i := 0; i < len(items); {
			start, end := items[i].start, items[i].end
			unitIDs := []string{items[i].candidate.UnitID}
			j := i + 1
			for j < len(items) && items[j].start <= end {
				if items[j].end > end {
					end = items[j].end
				}
				unitIDs = append(unitIDs, items[j].candidate.UnitID)
				j++
			}
			sort.Strings(unitIDs)
			source := sources[sourceID]
			endPosition, err := offsetPosition(source.Bytes, items[i].candidate.PositionEncoding, end)
			if err != nil {
				return nil, fmt.Errorf("source %s span end: %w", sourceID, err)
			}
			span := Span{LogicalSourceID: sourceID, Range: Range{Start: items[i].candidate.Range.Start, End: endPosition}, SourceDigest: source.Digest, ByteLength: end - start, UnitIDs: unitIDs}
			if includeBody {
				span.Body = string(source.Bytes[start:end])
			}
			spans = append(spans, span)
			i = j
		}
	}
	return spans, nil
}

func rangeBytes(raw []byte, encoding string, r Range) ([]byte, error) {
	start, end, err := sourceposition.Offsets(raw, encoding, sourceposition.Range{
		Start: sourcePosition(r.Start),
		End:   sourcePosition(r.End),
	})
	if err != nil {
		return nil, err
	}
	return raw[start:end], nil
}

func positionOffset(raw []byte, encoding string, position Position) (int, error) {
	return sourceposition.Offset(raw, encoding, sourcePosition(position))
}

func sourcePosition(position Position) sourceposition.Position {
	return sourceposition.Position{Line: position.Line, Character: position.Character}
}

func offsetPosition(raw []byte, encoding string, target int) (Position, error) {
	position, err := sourceposition.PositionAtOffset(raw, encoding, target)
	if err != nil {
		return Position{}, err
	}
	return Position{Line: position.Line, Character: position.Character}, nil
}

func projectionStatus(result Result) string {
	if result.Accounting.Candidates == 0 {
		return "SUCCESSFUL_EMPTY"
	}
	if result.Accounting.Omitted == 0 {
		return "COMPLETE"
	}
	for _, omission := range result.Omissions {
		if omission.Cause == "BYTE_LIMIT" || omission.Cause == "RANGE_LIMIT" || omission.Cause == "OBJECT_LIMIT" {
			return "TRUNCATED"
		}
	}
	if result.Accounting.Selected == 0 {
		allUnavailable := true
		for _, omission := range result.Omissions {
			if omission.Cause != "SOURCE_UNAVAILABLE" {
				allUnavailable = false
				break
			}
		}
		if allUnavailable {
			return "SOURCE_UNAVAILABLE"
		}
	}
	return "PARTIAL"
}
