package sourceprojection

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"

	"lsp-trace/internal/graph"
	"lsp-trace/internal/transientstructural"
)

var ErrDerivationNotImplemented = errors.New("source projection candidate derivation not implemented")

func DeriveCandidates(result transientstructural.Result, mode string, includeRelationOccurrences bool) ([]Candidate, error) {
	if mode != "TARGET" && mode != "PROJECTED" && mode != "COMPLETE_CAPTURE" {
		return nil, fmt.Errorf("unsupported projection mode %q", mode)
	}
	if result.Qualification.PositionEncoding != "utf-16" || result.TargetID == "" {
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
			DisplayProvenance: "CALL_HIERARCHY_ITEM", PositionEncoding: "utf-16", PrivacyClassification: "PUBLIC",
		}
		candidate.UnitID = canonicalID(struct {
			Role, Subject, Source string
			Range                 Range
		}{candidate.Role, candidate.GraphSubjectID, candidate.LogicalSourceID, candidate.Range})
		candidate.CitationID = canonicalID(struct{ Unit, Role, Subject string }{candidate.UnitID, candidate.Role, candidate.GraphSubjectID})
		candidates = append(candidates, candidate)
	}
	if mode != "TARGET" && includeRelationOccurrences {
		for _, occurrence := range result.Analysis.Occurrences {
			occurrenceID := canonicalID(struct{ Observed string }{occurrence.ID})
			subjectID := canonicalID(struct{ Caller, Callee string }{occurrence.CallerID, occurrence.CalleeID})
			candidate := Candidate{
				Role: "RELATION", GraphSubjectID: subjectID, OccurrenceID: occurrenceID, LogicalSourceID: occurrence.URI,
				Range: projectionRange(occurrence.Range), EvidenceRange: projectionRange(occurrence.Range),
				DisplayProvenance: "CALL_SITE_OCCURRENCE", PositionEncoding: "utf-16", RelationProvenance: "SERVER_REPORTED", PrivacyClassification: "PUBLIC",
			}
			candidate.UnitID = canonicalID(struct {
				Role, Subject, Occurrence, Source string
				Range                             Range
			}{candidate.Role, subjectID, occurrenceID, candidate.LogicalSourceID, candidate.Range})
			candidate.CitationID = canonicalID(struct{ Unit, Role, Subject, Occurrence string }{candidate.UnitID, candidate.Role, subjectID, occurrenceID})
			candidates = append(candidates, candidate)
		}
	}
	sort.Slice(candidates, func(i, j int) bool { return candidates[i].UnitID < candidates[j].UnitID })
	return candidates, nil
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
