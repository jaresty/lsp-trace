package adr0011methodresult

import (
	"bytes"
	"path/filepath"
	"testing"
)

// The frozen eight-row oracle test checks zero candidates on substitutions.
// This separate private guard checks that each one reaches its intended
// behavioral rejection, not the old all-rejecting scaffold.
func TestB4DefinitionBridgeNegativeStatusGuards(t *testing.T) {
	evidence := "testdata"
	oracleRoot := filepath.Join("testdata", "adr0011-definition-bridge-independent-oracle-v1")
	var oracle struct {
		Rows []bridgeOracleRow `json:"rows"`
	}
	bridgeJSON(t, bridgePinnedBytes(t, filepath.Join(oracleRoot, "oracle.json"), bridgeOracleSHA), &oracle)
	var row bridgeOracleRow
	for _, item := range oracle.Rows {
		if item.Scenario == "CORE" && item.WriteOrdinal == 6 {
			row = item
		}
	}
	if row.ResponseSHA == "" || row.BridgeStatus != "CANDIDATE_ITEMS" || row.CandidateCount != 1 {
		bridgeFixtureFatal(t, "pinned oracle lacks expected CORE/WRITE6 positive control")
	}
	responseRoot := filepath.Join(evidence, "adr0011-definition-response-held-originals-v1-corrected")
	assets := bridgeManifestAssets(t, filepath.Join(responseRoot, "manifest.json"), "2621c234bd4aef4e7fc2e190b9672c89faa1ebada39aac9d0543ff91b804f1d7")
	original := bridgeFixture(t, evidence, row, assets)
	t.Run("original", func(t *testing.T) {
		got := CheckB4DefinitionBridge(original)
		if got.Status != DefinitionBridgeCandidateItems || len(got.Candidates) != 1 {
			t.Errorf("semantic assertion original: status=%q candidates=%d; want CANDIDATE_ITEMS/1", got.Status, len(got.Candidates))
		}
		bridgeAssertNoPositiveExclusion(t, "original", got)
	})
	cases := []struct {
		name  string
		want  DefinitionBridgeStatus
		alter func(*DefinitionBridgeInput)
	}{
		{"SOURCE", DefinitionBridgeCorrespondenceInvalid, func(v *DefinitionBridgeInput) {
			v.Replay.Write.Source = append([]byte(nil), v.Replay.Write.Source...)
			v.Replay.Write.Source[0] ^= 1
		}},
		{"QUERY", DefinitionBridgeCorrespondenceInvalid, func(v *DefinitionBridgeInput) {
			v.Replay.Write.QueryOriginal = append([]byte(nil), v.Replay.Write.QueryOriginal...)
			v.Replay.Write.QueryOriginal[0] ^= 1
		}},
		{"transaction", DefinitionBridgeCorrespondenceInvalid, func(v *DefinitionBridgeInput) {
			v.Replay.Write.Transaction += "-substituted"
		}},
		{"references", DefinitionBridgeCorrespondenceInvalid, func(v *DefinitionBridgeInput) {
			v.Replay.Write.Method = "textDocument/references"
		}},
		{"typed-response-ID", DefinitionBridgeResponseInvalid, func(v *DefinitionBridgeInput) {
			v.ResponseFrame = bytes.Replace(v.ResponseFrame, []byte(`"id":141`), []byte(`"id":"141"`), 1)
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			changed := original
			tc.alter(&changed)
			if tc.name == "typed-response-ID" && bytes.Equal(changed.ResponseFrame, original.ResponseFrame) {
				bridgeFixtureFatal(t, "held response has no numeric ID substitution site")
			}
			got := CheckB4DefinitionBridge(changed)
			if got.Status != tc.want || len(got.Candidates) != 0 {
				t.Errorf("semantic assertion %s: status=%q candidates=%d; want %q/0", tc.name, got.Status, len(got.Candidates), tc.want)
			}
			bridgeAssertNoPositiveExclusion(t, tc.name, got)
		})
	}
}
