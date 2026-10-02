package adr0011methodresult

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"path/filepath"
	"strconv"
	"testing"
)

const (
	bridgeHeldResponseManifestSHA = "37d43c854523e3546ac5eebe48a7b2639fe3925c48d59c22eada0daf59ea73a5"
	bridgeHeldResponseBindingSHA  = "8f13c7dc95a0ed210d27a421799ba5c35f0d5abc04b2788d39060d716b94f67a"
)

type bridgeHeldBinding struct {
	Write           string `json:"write"`
	Session         string `json:"session"`
	Transaction     string `json:"transaction"`
	Method          string `json:"method"`
	RequestOwnerKey string `json:"request_owner_key"`
	Generation      uint64 `json:"generation"`
	RequestID       uint64 `json:"request_id"`
	ResponseID      uint64 `json:"response_id"`
	RequestIDType   string `json:"request_id_type"`
	ResponseIDType  string `json:"response_id_type"`
	ResponsePath    string `json:"response_path"`
	ResponseSHA256  string `json:"response_sha256"`
	RawResultSHA256 string `json:"raw_result_sha256"`
	ResponseLength  int    `json:"response_length"`
	RawResultLength int    `json:"raw_result_length"`
}

// The pinned packet contains predictions, not a producer-observed response.
// Its binding must be independent of the ResponseFrame supplied to the bridge.
func bridgeHeldResponseFixture(t *testing.T, root string, row bridgeOracleRow, in DefinitionBridgeInput, assets map[string]bridgeAsset, bindings []bridgeHeldBinding) DefinitionResponseHold {
	t.Helper()
	write := row.Scenario + "/WRITE" + strconv.FormatUint(row.WriteOrdinal, 10)
	var matched *bridgeHeldBinding
	for i := range bindings {
		if bindings[i].Write == write {
			if matched != nil {
				bridgeFixtureFatal(t, "duplicate held response binding %s", write)
			}
			matched = &bindings[i]
		}
	}
	if matched == nil {
		bridgeFixtureFatal(t, "no held response binding for %s", write)
	}
	b := *matched
	if b.ResponsePath != write+"/response.json" || b.ResponseIDType != "number" || b.RequestIDType != "number" ||
		b.Session != in.Replay.Write.Session || b.Generation != in.Replay.Write.Generation ||
		b.Transaction != in.Replay.Write.Transaction || b.Method != in.Replay.Write.Method ||
		b.RequestOwnerKey != in.Replay.Write.CompletedKey ||
		strconv.FormatUint(b.RequestID, 10) != string(in.Replay.Write.RequestID) ||
		b.RequestID != b.ResponseID || b.ResponseSHA256 != row.ResponseSHA {
		bridgeFixtureFatal(t, "held binding/WRITE/oracle disagreement for %s", write)
	}
	original := bridgeAssetBytes(t, root, b.ResponsePath, assets)
	if len(original) != b.ResponseLength || assets[b.ResponsePath].SHA256 != b.ResponseSHA256 || !bytes.Equal(original, in.ResponseFrame) {
		bridgeFixtureFatal(t, "held response copy mismatch for %s", write)
	}
	result, ok := bridgeResponseResult(DefinitionBridgeInput{Replay: in.Replay, ResponseFrame: original})
	if !ok || len(result) != b.RawResultLength || rawSHA(result) != "sha256:"+b.RawResultSHA256 {
		bridgeFixtureFatal(t, "held response raw-result mismatch for %s", write)
	}
	return DefinitionResponseHold{Session: b.Session, Generation: b.Generation, Transaction: b.Transaction,
		Method: b.Method, RequestOwnerKey: b.RequestOwnerKey, RequestID: b.RequestID, ResponseID: b.ResponseID,
		ResponseOriginal: original, ResponseLength: b.ResponseLength, ResponseSHA256: b.ResponseSHA256,
		RawResultLength: b.RawResultLength, RawResultSHA256: b.RawResultSHA256}
}

func TestB4DefinitionBridgeHeldResponseCustody(t *testing.T) {
	evidence := filepath.Join("..", "..", ".pi", "evidence")
	root := filepath.Join(evidence, "adr0011-definition-response-custody-held-v1")
	assets := bridgeManifestAssets(t, filepath.Join(root, "manifest.json"), bridgeHeldResponseManifestSHA)
	var packet struct {
		Scope        string              `json:"scope"`
		Authority    int                 `json:"authority"`
		Accepted     bool                `json:"accepted"`
		Completeness string              `json:"completeness"`
		Bindings     []bridgeHeldBinding `json:"bindings"`
	}
	bindingBytes := bridgeAssetBytes(t, root, "binding.json", assets)
	if digest := sha256.Sum256(bindingBytes); hex.EncodeToString(digest[:]) != bridgeHeldResponseBindingSHA {
		bridgeFixtureFatal(t, "unexpected response binding SHA")
	}
	bridgeJSON(t, bindingBytes, &packet)
	if packet.Scope != "PRIVATE_SYNTHETIC_NOT_OBSERVED_OR_AUTHENTICATED" || packet.Authority != 0 ||
		packet.Accepted || packet.Completeness != "UNKNOWN" || len(packet.Bindings) != 8 {
		bridgeFixtureFatal(t, "held response packet scope or row count invalid")
	}
	var oracle struct {
		Rows []bridgeOracleRow `json:"rows"`
	}
	bridgeJSON(t, bridgePinnedBytes(t, filepath.Join(evidence, "adr0011-definition-bridge-independent-oracle-v1", "oracle.json"), bridgeOracleSHA), &oracle)
	if len(oracle.Rows) != 8 {
		bridgeFixtureFatal(t, "oracle row count is not eight")
	}
	responseRoot := filepath.Join(evidence, "adr0011-definition-response-held-originals-v1-corrected")
	responseAssets := bridgeManifestAssets(t, filepath.Join(responseRoot, "manifest.json"), "2621c234bd4aef4e7fc2e190b9672c89faa1ebada39aac9d0543ff91b804f1d7")
	for _, row := range oracle.Rows {
		row := row
		t.Run(row.Scenario+"/WRITE"+strconv.FormatUint(row.WriteOrdinal, 10), func(t *testing.T) {
			in := bridgeFixture(t, evidence, row, responseAssets)
			held := bridgeHeldResponseFixture(t, root, row, in, assets, packet.Bindings)
			got := CheckB4DefinitionBridgeWithHeldResponse(in, held)
			if string(got.Status) != row.BridgeStatus || got.ChronologyTerminal != row.ChronologyTerminal || len(got.Candidates) != row.CandidateCount {
				t.Errorf("semantic assertion held response: status=%q chronology=%q candidates=%d; want %q/%q/%d", got.Status, got.ChronologyTerminal, len(got.Candidates), row.BridgeStatus, row.ChronologyTerminal, row.CandidateCount)
			}
			bridgeAssertNoPositiveExclusion(t, "held response", got)
			if row.Scenario != "CORE" || row.WriteOrdinal != 6 {
				return
			}
			foreign := bridgeAssetBytes(t, root, "CORE/WRITE8/response.json", assets)
			foreign = bytes.Replace(foreign, []byte(`"id":142`), []byte(`"id":141`), 1)
			if bytes.Equal(foreign, in.ResponseFrame) || !bytes.Contains(foreign, []byte(`"id":141`)) || !json.Valid(foreign) {
				bridgeFixtureFatal(t, "same-ID foreign response substitution site invalid")
			}
			changed := in
			changed.ResponseFrame = foreign
			legacy := CheckB4DefinitionBridge(changed)
			if legacy.Status != DefinitionBridgeCandidateItems || len(legacy.Candidates) != 2 {
				bridgeFixtureFatal(t, "same-ID foreign response is not a valid legacy candidate: %+v", legacy)
			}
			t.Run("same-ID-foreign-response", func(t *testing.T) {
				blocked := CheckB4DefinitionBridgeWithHeldResponse(changed, held)
				if blocked.Status != DefinitionBridgeResponseInvalid || len(blocked.Candidates) != 0 {
					t.Errorf("semantic assertion foreign same-ID response: status=%q candidates=%d; want RESPONSE_INVALID/0", blocked.Status, len(blocked.Candidates))
				}
				bridgeAssertNoPositiveExclusion(t, "foreign same-ID response", blocked)
			})
			t.Run("same-ID-cross-transaction-bound-record", func(t *testing.T) {
				adversarial := held
				adversarial.Transaction = "definition-foreign-write-with-same-id"
				adversarial.ResponseOriginal = foreign
				adversarial.ResponseLength = len(foreign)
				digest := sha256.Sum256(foreign)
				adversarial.ResponseSHA256 = hex.EncodeToString(digest[:])
				result, ok := bridgeResponseResult(changed)
				if !ok {
					bridgeFixtureFatal(t, "foreign response ID not numeric or mismatched")
				}
				adversarial.RawResultLength = len(result)
				adversarial.RawResultSHA256 = rawSHA(result)[len("sha256:"):]
				blocked := CheckB4DefinitionBridgeWithHeldResponse(changed, adversarial)
				if blocked.Status != DefinitionBridgeResponseInvalid || len(blocked.Candidates) != 0 {
					t.Errorf("semantic assertion same-ID cross-transaction record: status=%q candidates=%d; want RESPONSE_INVALID/0", blocked.Status, len(blocked.Candidates))
				}
				bridgeAssertNoPositiveExclusion(t, "same-ID cross-transaction record", blocked)
			})
		})
	}
}
