//go:build darwin || linux

package main

import (
	"encoding/json"
	"testing"
	"time"

	product "lsp-trace/internal/privatehostproductv3"
)

// Synthetic request DATA only: no issuer, native owner, provider or HOST runs.
func TestCensusCallerHostSettingsReboundRequest(t *testing.T) {
	c := callerHostSettingsFixture(t)
	q := ownerCommandDataV3(t)
	encode := func() {
		t.Helper()
		ownerCommandSealV3(t, &q)
		raw, err := json.Marshal(q)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := product.DecodeInventoryRequestV3(raw); err != nil {
			t.Fatal("ASSERT_REBOUND_PRODUCTION_DECODER", err)
		}
		c.Request = raw
	}
	c.Action = "setup"
	encode()
	saved, err := censusCallerHostStable(c)
	if err != nil {
		t.Fatal(err)
	}
	originalUnit, originalSelection := q.Configuration.UnitPolicySHA256, q.Configuration.SelectionRuleSHA256
	q.Configuration.Store += "-fresh"
	q.Inventory.NativeOwners[0].RunID += "-fresh"
	q.Inventory.NativeOwners[0].SeedNamespace += "-fresh"
	q.Inventory.NativeOwners[0].Deadline = time.Now().Add(7 * time.Second)
	q.Inventory.Manifest.Provenance += "-fresh-retained-cursor"
	encode()
	if originalUnit == q.Configuration.UnitPolicySHA256 || originalSelection == q.Configuration.SelectionRuleSHA256 {
		t.Fatal("ASSERT_REBOUND_SEALS_CHANGED")
	}
	if !censusCallerHostSettingsMatch(saved, c) {
		t.Fatal("ASSERT_SETTINGS_REBOUND_REQUEST_MATCH")
	}
	t.Log("ASSERT_SETTINGS_REBOUND_REQUEST_MATCH A-pass")
	for _, kind := range []string{"budget", "worker-pin", "source-limit"} {
		t.Run(kind, func(t *testing.T) {
			raw := append([]byte(nil), c.Request...)
			var changed product.InventoryRequestV3
			if json.Unmarshal(raw, &changed) != nil {
				t.Fatal("fixture")
			}
			switch kind {
			case "budget":
				changed.Inventory.BudgetBytes++
			case "worker-pin":
				changed.Configuration.ProducerExecutableSHA256 = censusInventoryDigest([]byte("different worker"))
			case "source-limit":
				changed.Inventory.NativeOwners[0].SourceLimits.MaxWork++
			}
			ownerCommandSealV3(t, &changed)
			candidate := c
			candidate.Request, _ = json.Marshal(changed)
			if _, err := product.DecodeInventoryRequestV3(candidate.Request); err != nil {
				t.Fatal("ASSERT_POLICY_CHANGE_VALID_REQUEST", err)
			}
			if censusCallerHostSettingsMatch(saved, candidate) {
				t.Fatal("ASSERT_SETTINGS_REBOUND_POLICY_CHANGE_REFUSED", kind)
			}
			t.Log("ASSERT_SETTINGS_REBOUND_POLICY_CHANGE_REFUSED A-pass", kind)
		})
	}
}

func TestCensusCallerHostSettingsExportVersion(t *testing.T) {
	c := callerHostSettingsFixture(t)
	s, err := censusCallerHostStable(c)
	if err != nil {
		t.Fatal(err)
	}
	if s.Schema != "census-caller-host-settings.v2" {
		t.Fatal("ASSERT_SETTINGS_V2_EXPLICIT_EXPORT")
	}
	raw, _ := json.Marshal(s)
	var old censusCallerHostSettings
	if json.Unmarshal(raw, &old) != nil {
		t.Fatal("fixture")
	}
	old.Schema = "census-caller-host-settings.v1"
	legacy, _ := json.Marshal(old)
	if _, err := decodeCensusCallerHostSettings(legacy); err == nil {
		t.Fatal("ASSERT_SETTINGS_V1_NOT_SILENTLY_UPGRADED")
	}
	t.Log("ASSERT_SETTINGS_V2_EXPLICIT_EXPORT A-pass; ASSERT_SETTINGS_V1_NOT_SILENTLY_UPGRADED A-pass")
}
