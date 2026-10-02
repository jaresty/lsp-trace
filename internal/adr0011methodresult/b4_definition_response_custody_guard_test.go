package adr0011methodresult

import (
	"path/filepath"
	"testing"
)

// Additive field-isolation guard: the frozen test-first RED remains unchanged.
// Each held-field substitution leaves the B4a-verified WRITE and response intact.
func TestB4DefinitionBridgeHeldResponseFieldGuards(t *testing.T) {
	evidence := filepath.Join("..", "..", ".pi", "evidence")
	var oracle struct {
		Rows []bridgeOracleRow `json:"rows"`
	}
	bridgeJSON(t, bridgePinnedBytes(t, filepath.Join(evidence, "adr0011-definition-bridge-independent-oracle-v1", "oracle.json"), bridgeOracleSHA), &oracle)
	var row bridgeOracleRow
	for _, item := range oracle.Rows {
		if item.Scenario == "CORE" && item.WriteOrdinal == 6 {
			row = item
		}
	}
	if row.BridgeStatus != "CANDIDATE_ITEMS" || row.CandidateCount != 1 {
		bridgeFixtureFatal(t, "oracle lacks CORE/WRITE6 positive control")
	}
	root := filepath.Join(evidence, "adr0011-definition-response-custody-held-v1")
	assets := bridgeManifestAssets(t, filepath.Join(root, "manifest.json"), bridgeHeldResponseManifestSHA)
	var packet struct {
		Bindings []bridgeHeldBinding `json:"bindings"`
	}
	bridgeJSON(t, bridgePinnedBytes(t, filepath.Join(root, "binding.json"), bridgeHeldResponseBindingSHA), &packet)
	responseRoot := filepath.Join(evidence, "adr0011-definition-response-held-originals-v1-corrected")
	responseAssets := bridgeManifestAssets(t, filepath.Join(responseRoot, "manifest.json"), "2621c234bd4aef4e7fc2e190b9672c89faa1ebada39aac9d0543ff91b804f1d7")
	in := bridgeFixture(t, evidence, row, responseAssets)
	held := bridgeHeldResponseFixture(t, root, row, in, assets, packet.Bindings)
	if got := CheckB4DefinitionBridgeWithHeldResponse(in, held); got.Status != DefinitionBridgeCandidateItems || len(got.Candidates) != 1 {
		bridgeFixtureFatal(t, "valid held response control failed: %+v", got)
	}
	cases := []struct {
		name  string
		alter func(*DefinitionResponseHold)
	}{
		{"session", func(h *DefinitionResponseHold) { h.Session += "-foreign" }},
		{"generation", func(h *DefinitionResponseHold) { h.Generation++ }},
		{"transaction", func(h *DefinitionResponseHold) { h.Transaction += "-foreign" }},
		{"method", func(h *DefinitionResponseHold) { h.Method = "textDocument/references" }},
		{"request-owner-key", func(h *DefinitionResponseHold) { h.RequestOwnerKey += "-foreign" }},
		{"typed-request-ID", func(h *DefinitionResponseHold) { h.RequestID++ }},
		{"typed-response-ID", func(h *DefinitionResponseHold) { h.ResponseID++ }},
		{"response-length", func(h *DefinitionResponseHold) { h.ResponseLength++ }},
		{"response-digest", func(h *DefinitionResponseHold) { h.ResponseSHA256 = "" }},
		{"raw-result-length", func(h *DefinitionResponseHold) { h.RawResultLength++ }},
		{"raw-result-digest", func(h *DefinitionResponseHold) { h.RawResultSHA256 = "" }},
		{"missing-original", func(h *DefinitionResponseHold) { h.ResponseOriginal = nil }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			changed := held
			tc.alter(&changed)
			got := CheckB4DefinitionBridgeWithHeldResponse(in, changed)
			if got.Status != DefinitionBridgeResponseInvalid || len(got.Candidates) != 0 {
				t.Errorf("semantic assertion %s: status=%q candidates=%d; want RESPONSE_INVALID/0", tc.name, got.Status, len(got.Candidates))
			}
			bridgeAssertNoPositiveExclusion(t, tc.name, got)
		})
	}
}
