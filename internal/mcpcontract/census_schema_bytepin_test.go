package mcpcontract

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"testing"
)

func TestCensusAdditiveSchemaBytesAndRegistryIdentityPinned(t *testing.T) {
	pins := map[string]string{
		CensusDomainErrorID:                      "b6a7694652cfa0dcfc2cff297e3ca964f78a2b923826b76c6e64449d7103510e",
		FutureCensusV2InputID:                    "ccd60f6526220827cb5fe408f07e78a30eb7a4cb8a95866a9513cbc04a89928c",
		FutureCensusCompositeResultID:            "d676bff3bacdf35142dc5c6e63ab48e64c6b6efc5860448fc079f1561289c767",
		FutureCensusCompositeSuccessID:           "6ce12bdb832349cdf46606e914ab0e9d3b395324a399a2034401d952426df2cd",
		CensusContinuationDiagnosticID:           "eba4f6951d81a801068917c19271958982359b423bd0671cee60007b21bc9ea5",
		CensusContinuationDiagnosticEnvelopeID:   "18b869b28cd7f6f1f630b712f8ea21ccc281019e23d177baa25061ca5935617d",
		CensusContinuationDiagnosticV2ID:         "aa1883f0b932c59afe6e40626912b20f3d1676af9bc205925bbe920ec86a7f06",
		CensusContinuationDiagnosticEnvelopeV2ID: "c3bf4f96f14d8bbdf1d38c5c95abe7608c8c2808c1b5f50ae31b160375292a83",
		CensusRequestReceiptID:                   "2cf01f26d3e9d440e1b9eba719ab88587be7ce187f8da86d8fb2979fe6d87c38",
		CensusDiscoveryDiagnosticV2ID:            "7dca4257c8b705b7c3e6034be5ab672e2bbad22c654cf2bc79f80ac16980b1b4",
		CensusDiscoveryDiagnosticEnvelopeV2ID:    "6d6a2c579a86e5383d4aab090439a454f3013463808fcf8ae6228bfaedec14d7",
	}
	for id, want := range pins {
		raw, err := FutureCensusSchemaJSON(id)
		if err != nil {
			t.Fatalf("schema %s: %v", id, err)
		}
		sum := sha256.Sum256(raw)
		if got := hex.EncodeToString(sum[:]); got != want {
			t.Fatalf("schema %s sha256=%s want=%s", id, got, want)
		}
		var identity struct {
			ID string `json:"$id"`
		}
		if err := json.Unmarshal(raw, &identity); err != nil || identity.ID != id {
			t.Fatalf("schema identity %s: id=%q err=%v", id, identity.ID, err)
		}
	}
	registrations := WithCensus(&Manifest{}).Schemas
	seen := map[string]bool{}
	for _, registration := range registrations {
		seen[registration.ID] = true
	}
	for id := range pins {
		if !seen[id] {
			t.Fatalf("schema %s missing registry identity", id)
		}
	}
}
