package lsp

import (
	"encoding/json"
	"testing"
)

func TestDocumentSymbolDecodesFlatSymbolInformationLocation(t *testing.T) {
	var symbols []DocumentSymbol
	raw := []byte(`[{"name":"Execute","kind":12,"location":{"uri":"file:///workspace/a.go","range":{"start":{"line":7,"character":5},"end":{"line":9,"character":1}}},"containerName":"Runtime"}]`)
	if err := json.Unmarshal(raw, &symbols); err != nil {
		t.Fatal(err)
	}
	want := Range{Start: Position{Line: 7, Character: 5}, End: Position{Line: 9, Character: 1}}
	if len(symbols) != 1 || symbols[0].Name != "Execute" || symbols[0].Kind != 12 || symbols[0].Range != want || symbols[0].SelectionRange != want || !symbols[0].Flat || symbols[0].ContainerName != "Runtime" {
		t.Fatalf("ASSERT_FLAT_SYMBOL_INFORMATION_LOCATION_NORMALIZED: %+v", symbols)
	}
}

func TestDocumentSymbolPreservesHierarchicalRangesAndChildren(t *testing.T) {
	var symbols []DocumentSymbol
	raw := []byte(`[{"name":"Runtime","kind":5,"range":{"start":{"line":1,"character":0},"end":{"line":5,"character":1}},"selectionRange":{"start":{"line":1,"character":5},"end":{"line":1,"character":12}},"children":[{"name":"Run","kind":12,"range":{"start":{"line":2,"character":0},"end":{"line":4,"character":1}},"selectionRange":{"start":{"line":2,"character":5},"end":{"line":2,"character":8}}}]}]`)
	if err := json.Unmarshal(raw, &symbols); err != nil {
		t.Fatal(err)
	}
	if len(symbols) != 1 || symbols[0].Flat || symbols[0].SelectionRange.Start != (Position{Line: 1, Character: 5}) || len(symbols[0].Children) != 1 || symbols[0].Children[0].Flat || symbols[0].Children[0].SelectionRange.Start != (Position{Line: 2, Character: 5}) {
		t.Fatalf("ASSERT_HIERARCHICAL_DOCUMENT_SYMBOL_PRESERVED: %+v", symbols)
	}
}
