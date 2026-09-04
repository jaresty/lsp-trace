package graph

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// Historical V2 and V3 documents are compatibility identities. Their complete
// canonical bytes are frozen: additive work belongs in a new schema version.
func TestHistoricalIdentityCanonicalBytes(t *testing.T) {
	for _, version := range []string{SchemaVersionV2, SchemaVersionV3} {
		t.Run(version, func(t *testing.T) {
			encoded, err := json.Marshal(historicalIdentityFixture(version))
			if err != nil {
				t.Fatal(err)
			}
			fixture := filepath.Join("testdata", "historical_identity_"+version[len(version)-2:]+".json")
			assertion := "ASSERT_HISTORICAL_IDENTITY_CANONICAL_BYTES_" + version[len(version)-2:]
			want, err := os.ReadFile(fixture)
			if err != nil {
				t.Fatalf("%s: %v", assertion, err)
			}
			if !bytes.Equal(encoded, want) {
				t.Fatalf("%s: canonical bytes changed\nwant: %s\n got: %s", assertion, want, encoded)
			}
		})
	}
}

func historicalIdentityFixture(version string) Result {
	node := NewNode(Item{
		Name: "legacy", Kind: 12, Detail: "func()", URI: "file:///workspace/legacy.go",
		Range:          Range{Start: Position{Line: 1}, End: Position{Line: 3}},
		SelectionRange: Range{Start: Position{Line: 1, Character: 5}, End: Position{Line: 1, Character: 11}},
		Data:           json.RawMessage(`{"transport":"opaque"}`),
	})
	return Result{
		SchemaVersion: version,
		Invocation: Invocation{
			WorkspaceURI: "file:///workspace",
			Target:       Target{URI: node.URI, Line: 2, Column: 6},
			Server:       ServerInvocation{Command: "legacy-lsp", Arguments: []string{"--stdio"}},
			Limits:       Limits{MaxDepth: 2, MaxNodes: 10},
		},
		Targets: []string{node.ID}, Nodes: []Node{node},
		Terminals: []Boundary{{NodeID: node.ID, Reason: ServerReportedNoIncoming}},
		Summary:   Summary{NodeCount: 1, TerminalCount: 1, Complete: true},
	}
}
