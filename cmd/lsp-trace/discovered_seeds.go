package main

import (
	"fmt"
	"path/filepath"
	"sort"

	"lsp-trace/internal/lsp"
	"lsp-trace/internal/seedformat"
	"lsp-trace/internal/slicer"
)

type discoveredSeedCandidate struct {
	path string
	name string
	kind int
	at   lsp.Position
}

func canonicalDiscoveredSeeds(workspace string, sources []resolvedSliceSource, discoveries map[string]slicer.Discovery) (seedformat.File, []byte, error) {
	candidates := make([]discoveredSeedCandidate, 0)
	for _, source := range sources {
		for _, disposition := range discoveries[source.uri].PreparationDispositions {
			if disposition.Status != "prepared" {
				continue
			}
			candidates = append(candidates, discoveredSeedCandidate{
				path: filepath.ToSlash(source.path),
				name: disposition.Name,
				kind: disposition.Kind,
				at:   disposition.SelectionRange.Start,
			})
		}
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].path != candidates[j].path {
			return candidates[i].path < candidates[j].path
		}
		if candidates[i].at != candidates[j].at {
			if candidates[i].at.Line != candidates[j].at.Line {
				return candidates[i].at.Line < candidates[j].at.Line
			}
			return candidates[i].at.Character < candidates[j].at.Character
		}
		if candidates[i].kind != candidates[j].kind {
			return candidates[i].kind < candidates[j].kind
		}
		return candidates[i].name < candidates[j].name
	})
	if len(candidates) == 0 {
		return seedformat.File{}, nil, fmt.Errorf("document-symbol discovery produced no callable seeds")
	}
	if len(candidates) > seedformat.MaxSeeds {
		return seedformat.File{}, nil, fmt.Errorf("document-symbol discovery produced %d callable seeds; maximum is %d", len(candidates), seedformat.MaxSeeds)
	}
	file := seedformat.File{
		SchemaVersion:        seedformat.Version,
		CoordinateConvention: seedformat.CoordinateConvention,
		Seeds:                make([]seedformat.Seed, len(candidates)),
	}
	for i, candidate := range candidates {
		file.Seeds[i] = seedformat.Seed{Type: seedformat.PositionType, Position: &seedformat.Position{
			Label:  fmt.Sprintf("symbol-%03d", i+1),
			Path:   candidate.path,
			Line:   uint64(candidate.at.Line) + 1,
			Column: uint64(candidate.at.Character) + 1,
		}}
	}
	raw, err := seedformat.EncodeCanonical(file, workspace)
	if err != nil {
		return seedformat.File{}, nil, err
	}
	return file, raw, nil
}
