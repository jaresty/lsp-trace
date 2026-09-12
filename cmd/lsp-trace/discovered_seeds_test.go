package main

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"lsp-trace/internal/lsp"
	"lsp-trace/internal/seedformat"
	"lsp-trace/internal/slicer"
)

func TestCanonicalDiscoveredSeedsUseSelectionRangeAndStableOrder(t *testing.T) {
	workspace := t.TempDir()
	sources := []resolvedSliceSource{
		{path: "z.go", uri: "file:///z.go"},
		{path: filepath.Join("nested", "a.go"), uri: "file:///nested/a.go"},
	}
	discoveries := map[string]slicer.Discovery{
		"file:///z.go": {PreparationDispositions: []slicer.PreparationDisposition{
			{Name: "Ignored", Kind: 12, Status: "not_preparable", SelectionRange: lsp.Range{Start: lsp.Position{Line: 1, Character: 1}}},
			{Name: "Zulu", Kind: 12, Status: "prepared", SelectionRange: lsp.Range{Start: lsp.Position{Line: 8, Character: 3}}},
		}},
		"file:///nested/a.go": {PreparationDispositions: []slicer.PreparationDisposition{
			{Name: "Alpha", Kind: 6, Status: "prepared", SelectionRange: lsp.Range{Start: lsp.Position{Line: 2, Character: 5}}},
		}},
	}
	file, raw, err := canonicalDiscoveredSeeds(workspace, sources, discoveries)
	if err != nil {
		t.Fatal(err)
	}
	if len(file.Seeds) != 2 {
		t.Fatalf("ASSERT_DISCOVERED_SEEDS_CALLABLE_ONLY: %#v", file.Seeds)
	}
	first, second := file.Seeds[0].Position, file.Seeds[1].Position
	if first.Path != "nested/a.go" || first.Line != 3 || first.Column != 6 || second.Path != "z.go" || second.Line != 9 || second.Column != 4 {
		t.Fatalf("ASSERT_DISCOVERED_SEEDS_SELECTION_RANGE_ONE_BASED: first=%#v second=%#v", first, second)
	}
	if first.Label != "symbol-001" || second.Label != "symbol-002" {
		t.Fatalf("ASSERT_DISCOVERED_SEEDS_COLLISION_SAFE_LABELS: %q %q", first.Label, second.Label)
	}
	reencoded, err := seedformat.EncodeCanonical(file, workspace)
	if err != nil || !bytes.Equal(raw, reencoded) {
		t.Fatalf("ASSERT_DISCOVERED_SEEDS_CANONICAL_BYTES: err=%v raw=%q reencoded=%q", err, raw, reencoded)
	}
}

func TestCanonicalDiscoveredSeedsFailsClosedOnEmptyAndOverflow(t *testing.T) {
	workspace := t.TempDir()
	source := resolvedSliceSource{path: "a.go", uri: "file:///a.go"}
	if _, _, err := canonicalDiscoveredSeeds(workspace, []resolvedSliceSource{source}, map[string]slicer.Discovery{}); err == nil || !strings.Contains(err.Error(), "no callable seeds") {
		t.Fatalf("ASSERT_DISCOVERED_SEEDS_EMPTY_FAILS_CLOSED: %v", err)
	}
	dispositions := make([]slicer.PreparationDisposition, seedformat.MaxSeeds+1)
	for i := range dispositions {
		dispositions[i] = slicer.PreparationDisposition{Name: fmt.Sprintf("F%d", i), Kind: 12, Status: "prepared", SelectionRange: lsp.Range{Start: lsp.Position{Line: uint32(i)}}}
	}
	if _, _, err := canonicalDiscoveredSeeds(workspace, []resolvedSliceSource{source}, map[string]slicer.Discovery{source.uri: {PreparationDispositions: dispositions}}); err == nil || !strings.Contains(err.Error(), "65 callable seeds; maximum is 64") {
		t.Fatalf("ASSERT_DISCOVERED_SEEDS_OVERFLOW_FAILS_CLOSED_WITH_DENOMINATOR: %v", err)
	}
}
