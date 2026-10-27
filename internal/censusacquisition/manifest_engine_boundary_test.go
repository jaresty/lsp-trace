package censusacquisition_test

import (
	"encoding/json"
	"strings"
	"testing"

	"lsp-trace/acquisitionops"
	"lsp-trace/internal/acquisitionengine"
	"lsp-trace/internal/censusacquisition"
	"lsp-trace/internal/lsp"
)

// External-package placement preserves the real public alias/engine validation
// contract without creating a test-only census -> engine -> pilot -> census cycle.
func TestAcquisitionManifestSingleTargetUsesCanonicalEmptyRequiredTargets(t *testing.T) {
	batch := censusacquisition.BatchRequest{DownDepth: 1, Targets: []censusacquisition.PreparedTarget{{URI: "file:///w/a.go", SelectionRange: lsp.Range{Start: lsp.Position{}}, Name: "Target", Kind: 12, SymbolIdentity: "a.go#0:0:12:Target:0"}}}
	manifest := batch.AcquisitionManifest(acquisitionops.Limits{})
	if manifest.RequiredTargets == nil {
		t.Fatal("ASSERT_SINGLE_TARGET_REQUIRED_TARGETS_NONNIL")
	}
	rawManifest, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(rawManifest), `"required_targets":[]`) || strings.Contains(string(rawManifest), `"required_targets":null`) {
		t.Fatalf("ASSERT_SINGLE_TARGET_REQUIRED_TARGETS_JSON_ARRAY: %s", rawManifest)
	}
	raw, err := json.Marshal(acquisitionops.Input{SessionID: "s", Generation: 1, SeedManifest: manifest})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := acquisitionengine.CanonicalInput(raw); err != nil {
		t.Fatalf("ASSERT_SINGLE_TARGET_CANONICAL_INPUT_ROUNDTRIP: %v", err)
	}
}
