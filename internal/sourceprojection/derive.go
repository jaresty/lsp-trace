package sourceprojection

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"

	"lsp-trace/internal/graph"
	"lsp-trace/internal/sourceposition"
	"lsp-trace/internal/transientstructural"
	"lsp-trace/internal/transientstructuralresult"
)

var ErrDerivationNotImplemented = errors.New("source projection candidate derivation not implemented")

// DeriveWorkspaceCandidates limits source candidates to the exact canonical
// workspace boundary used by the public V2 structural projection.
func DeriveWorkspaceCandidates(result transientstructural.Result, mode string, includeRelationOccurrences bool, workspaceRoot string) ([]Candidate, error) {
	admitted := make(map[string]struct{}, len(result.Analysis.Nodes))
	nodes := make([]transientstructural.NodeFact, 0, len(result.Analysis.Nodes))
	for _, node := range result.Analysis.Nodes {
		if _, err := transientstructuralresult.WorkspaceRelativeURI(workspaceRoot, node.URI); err != nil {
			if transientstructuralresult.IsOutsideWorkspace(err) {
				continue
			}
			return nil, err
		}
		admitted[node.ID] = struct{}{}
		nodes = append(nodes, node)
	}
	if _, ok := admitted[result.TargetID]; !ok {
		return nil, errors.New("target absent")
	}
	occurrences := make([]transientstructural.OccurrenceFact, 0, len(result.Analysis.Occurrences))
	for _, occurrence := range result.Analysis.Occurrences {
		if _, ok := admitted[occurrence.CallerID]; !ok {
			continue
		}
		if _, ok := admitted[occurrence.CalleeID]; !ok {
			continue
		}
		if _, err := transientstructuralresult.WorkspaceRelativeURI(workspaceRoot, occurrence.URI); err != nil {
			return nil, err
		}
		occurrences = append(occurrences, occurrence)
	}
	result.Analysis.Nodes = nodes
	result.Analysis.Occurrences = occurrences
	return DeriveCandidates(result, mode, includeRelationOccurrences)
}

func DeriveCandidates(result transientstructural.Result, mode string, includeRelationOccurrences bool) ([]Candidate, error) {
	if mode != "TARGET" && mode != "PROJECTED" && mode != "COMPLETE_CAPTURE" {
		return nil, fmt.Errorf("unsupported projection mode %q", mode)
	}
	encoding := result.Qualification.PositionEncoding
	if !sourceposition.Supported(encoding) || result.TargetID == "" {
		return nil, errors.New("transient projection binding invalid")
	}
	candidates := make([]Candidate, 0, len(result.Analysis.Nodes)+len(result.Analysis.Occurrences))
	for _, node := range result.Analysis.Nodes {
		if mode == "TARGET" && node.ID != result.TargetID {
			continue
		}
		candidate := Candidate{
			Role: "ENDPOINT", GraphSubjectID: node.ID, LogicalSourceID: node.URI,
			Range: projectionRange(node.Range), EvidenceRange: projectionRange(node.SelectionRange),
			ItemRange: projectionRange(node.ItemRange), SelectionRange: projectionRange(node.SelectionRange),
			DisplayProvenance: "CALL_HIERARCHY_ITEM", PositionEncoding: encoding, PrivacyClassification: "PUBLIC",
		}
		candidate.UnitID = candidateUnitID(candidate)
		candidate.CitationID = candidateCitationID(candidate)
		candidates = append(candidates, candidate)
	}
	if mode != "TARGET" && includeRelationOccurrences {
		for _, occurrence := range result.Analysis.Occurrences {
			occurrenceID := canonicalID(struct{ Observed string }{occurrence.ID})
			subjectID := canonicalID(struct{ Caller, Callee string }{occurrence.CallerID, occurrence.CalleeID})
			candidate := Candidate{
				Role: "RELATION", GraphSubjectID: subjectID, OccurrenceID: occurrenceID, LogicalSourceID: occurrence.URI,
				Range: projectionRange(occurrence.Range), EvidenceRange: projectionRange(occurrence.Range),
				DisplayProvenance: "CALL_SITE_OCCURRENCE", PositionEncoding: encoding, RelationProvenance: "SERVER_REPORTED", PrivacyClassification: "PUBLIC",
			}
			candidate.UnitID = candidateUnitID(candidate)
			candidate.CitationID = candidateCitationID(candidate)
			candidates = append(candidates, candidate)
		}
	}
	sort.Slice(candidates, func(i, j int) bool { return candidates[i].UnitID < candidates[j].UnitID })
	return candidates, nil
}

func candidateUnitID(candidate Candidate) string {
	if candidate.Role == "RELATION" {
		return canonicalID(struct {
			Role, Subject, Occurrence, Source, Encoding, Privacy string
			Range                                                Range
		}{candidate.Role, candidate.GraphSubjectID, candidate.OccurrenceID, candidate.LogicalSourceID, candidate.PositionEncoding, candidate.PrivacyClassification, candidate.Range})
	}
	return canonicalID(struct {
		Role, Subject, Source, Encoding, Privacy string
		Range                                    Range
	}{candidate.Role, candidate.GraphSubjectID, candidate.LogicalSourceID, candidate.PositionEncoding, candidate.PrivacyClassification, candidate.Range})
}

func candidateCitationID(candidate Candidate) string {
	if candidate.Role == "RELATION" {
		return canonicalID(struct{ Unit, Role, Subject, Occurrence string }{candidate.UnitID, candidate.Role, candidate.GraphSubjectID, candidate.OccurrenceID})
	}
	return canonicalID(struct{ Unit, Role, Subject string }{candidate.UnitID, candidate.Role, candidate.GraphSubjectID})
}

func projectionRange(r graph.Range) Range {
	return Range{Start: Position{Line: r.Start.Line, Character: r.Start.Character}, End: Position{Line: r.End.Line, Character: r.End.Character}}
}

func canonicalID(value any) string {
	raw, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return digest(raw)
}
