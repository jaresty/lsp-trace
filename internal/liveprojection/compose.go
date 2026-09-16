package liveprojection

import (
	"fmt"
	"sort"

	"lsp-trace/internal/sourceprojection"
	"lsp-trace/sessionruntime"
)

type CompositionStatus string

const (
	CompositionComplete CompositionStatus = "COMPLETE"
	CompositionFailed   CompositionStatus = "FAILED"
)

type CompositionFailure struct {
	URI   string `json:"uri,omitempty"`
	Cause string `json:"cause"`
}

type CompositionResult struct {
	Status                CompositionStatus                 `json:"status"`
	Failure               *CompositionFailure               `json:"failure,omitempty"`
	Authority             int                               `json:"authority"`
	SourceGraphComplete   string                            `json:"source_graph_complete"`
	GraphFactsAdded       int                               `json:"graph_facts_added"`
	Bindings              []sourceprojection.LiveBinding    `json:"bindings"`
	PhysicalProjectionIDs []string                          `json:"physical_projection_ids"`
	Projection            sourceprojection.Result           `json:"projection"`
	Resolutions           []sourceprojection.LiveResolution `json:"-"`
	Candidates            []sourceprojection.Candidate      `json:"-"`
}

func Compose(prepared PreparationResult, sessionID string, generation uint64, candidates []sourceprojection.Candidate, policy sourceprojection.Policy) CompositionResult {
	result := CompositionResult{Status: CompositionFailed, Authority: 0, SourceGraphComplete: "UNKNOWN", GraphFactsAdded: 0, Bindings: []sourceprojection.LiveBinding{}, PhysicalProjectionIDs: []string{}}
	if prepared.Status != PreparationComplete {
		result.Failure = &CompositionFailure{Cause: "preparation not complete"}
		return result
	}

	supplies := append([]*sessionruntime.DocumentSupply(nil), prepared.Supplies...)
	sort.Slice(supplies, func(i, j int) bool {
		if supplies[i] == nil {
			return true
		}
		if supplies[j] == nil {
			return false
		}
		return supplies[i].URI < supplies[j].URI
	})
	resolutions := make([]sourceprojection.LiveResolution, 0, len(supplies))
	sources := make(map[string]sourceprojection.Source, len(supplies))
	bindings := make([]sourceprojection.LiveBinding, 0, len(supplies))
	physicalIDs := make([]string, 0, len(supplies))
	for _, supply := range supplies {
		uri := ""
		if supply != nil {
			uri = supply.URI
		}
		if _, duplicate := sources[uri]; duplicate {
			result.Failure = &CompositionFailure{URI: uri, Cause: "duplicate live source"}
			return result
		}
		resolved, err := sourceprojection.ResolveLive(supply, sourceprojection.LiveExpectation{SessionID: sessionID, Generation: generation, URI: uri, PositionEncoding: "utf-16"})
		if err != nil {
			result.Failure = &CompositionFailure{URI: uri, Cause: err.Error()}
			return result
		}
		sources[uri] = resolved.Source
		resolutions = append(resolutions, resolved)
		bindings = append(bindings, resolved.Binding)
		physicalIDs = append(physicalIDs, resolved.PhysicalProjectionID)
	}

	projection, err := sourceprojection.Project(candidates, sources, policy)
	if err != nil {
		result.Failure = &CompositionFailure{Cause: fmt.Sprintf("source projection: %v", err)}
		return result
	}
	result.Status = CompositionComplete
	result.Projection = projection
	result.Bindings = bindings
	result.PhysicalProjectionIDs = physicalIDs
	result.Resolutions = resolutions
	result.Candidates = append([]sourceprojection.Candidate(nil), candidates...)
	sort.Slice(result.Candidates, func(i, j int) bool { return result.Candidates[i].UnitID < result.Candidates[j].UnitID })
	return result
}
