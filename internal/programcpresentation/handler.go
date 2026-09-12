package programcpresentation

import (
	"encoding/json"
	"fmt"

	"lsp-trace/internal/graphprovenance"
	"lsp-trace/internal/programc"
)

// Request is transport-neutral. Input is exact Graph Provenance V5 envelope bytes.
type Request struct {
	Input        []byte
	Seed         uint64
	PageRankTopK int
	HubTopK      int
}

// Handle computes Program C and returns one fully joined view or no view.
func Handle(r Request) (Artifact, error) {
	if r.PageRankTopK < 1 || r.HubTopK < 1 {
		return Artifact{}, fmt.Errorf("pagerank_top_k and hub_top_k are mandatory positive values")
	}
	var identity struct {
		SchemaVersion string `json:"schema_version"`
	}
	if err := json.Unmarshal(r.Input, &identity); err != nil || identity.SchemaVersion != graphprovenance.VersionV5 {
		return Artifact{}, fmt.Errorf("native Graph Provenance V5 input required; composite admission is not authorized")
	}
	if _, err := graphprovenance.ValidateFor(r.Input, graphprovenance.Family, "v5"); err != nil {
		return Artifact{}, fmt.Errorf("native Graph Provenance V5 input required; composite admission is not authorized: %w", err)
	}
	o, failure := programc.Compute(r.Input, r.Seed)
	if failure != nil {
		return Artifact{}, failure
	}
	b, err := programc.ComputeBoundary(o, programc.BoundaryRequest{PageRankTopK: r.PageRankTopK, HubTopK: r.HubTopK})
	if err != nil {
		return Artifact{}, err
	}
	return Build(o, b)
}
