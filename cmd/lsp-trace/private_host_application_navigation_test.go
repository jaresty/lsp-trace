//go:build darwin

package main

import (
	"bytes"
	"encoding/json"
	"testing"

	pilot "lsp-trace/internal/censuslineagepilot"
	product "lsp-trace/internal/privatehostproductv3"
)

// Decode only the test input permissively so this guard also compiles against
// the pre-Navigation envelope. The production request decoder remains strict.
func navigationApplicationRequest(t *testing.T, enabled bool) hostApplicationRequest {
	t.Helper()
	r := hostApplicationRequest{UnitResponseBytes: 8192, UnitTimeoutMS: 180000}
	r.Template.OperationTimeoutMS = 240000
	r.Template.ExchangeTimeoutMS = 60000
	r.Template.Policy.TotalBytes = 1
	raw := []byte(`{"Schema":"private-application-unit-envelope.v1","MaxCalls":6,"MaxRequestBytes":131072,"MaxTotalBytes":835584,"MaxOutputBytes":1048576,"UnitTimeoutMS":180000,"Navigation":false}`)
	if enabled {
		raw = bytes.Replace(raw, []byte(`"Navigation":false`), []byte(`"Navigation":true`), 1)
	}
	if err := json.Unmarshal(raw, &r.UnitEnvelope); err != nil {
		t.Fatal(err)
	}
	if _, err := applicationUnitEnvelope(r); err != nil {
		t.Fatal(err)
	}
	return r
}

func TestHostApplicationNavigationOptIn(t *testing.T) {
	t.Run("propagation", func(t *testing.T) {
		for _, enabled := range []bool{false, true} {
			r := navigationApplicationRequest(t, enabled)
			got := applicationDerivedCohort(r, product.Config{}, "unit", "/controlled-unit")
			if got.Navigation != enabled {
				t.Fatalf("ASSERT_APPLICATION_NAVIGATION_PROPAGATION enabled=%t got=%t", enabled, got.Navigation)
			}
		}
		t.Log("ASSERT_APPLICATION_NAVIGATION_PROPAGATION A-pass")
	})
	t.Run("legacyBytes", func(t *testing.T) {
		// These types freeze the exact pre-Navigation fields, order and tags;
		// comparing two new envelopes would not establish legacy compatibility.
		type legacyEnvelope struct {
			Schema          string
			MaxCalls        int
			MaxRequestBytes int
			MaxTotalBytes   int
			MaxOutputBytes  int
			UnitTimeoutMS   int `json:",omitempty"`
		}
		for _, kind := range []string{"nil", "false-default", "false-opted-duration"} {
			r := navigationApplicationRequest(t, false)
			if kind != "false-opted-duration" {
				r.UnitTimeoutMS = 60000
				r.UnitEnvelope = nil
				if kind == "false-default" {
					p, err := applicationUnitEnvelope(r)
					if err != nil {
						t.Fatal(err)
					}
					r.UnitEnvelope = &p
				}
			}
			var old *legacyEnvelope
			if p := r.UnitEnvelope; p != nil {
				old = &legacyEnvelope{p.Schema, p.MaxCalls, p.MaxRequestBytes, p.MaxTotalBytes, p.MaxOutputBytes, p.UnitTimeoutMS}
			}
			legacy := struct {
				Schema            string
				Configuration     product.ApplicationConfig
				Template          hostProductRequestV3
				MaxUnitsPerRun    int
				UnitResponseBytes int
				UnitTimeoutMS     int
				UnitEnvelope      *legacyEnvelope `json:",omitempty"`
				ApprovalSHA256    string
			}{r.Schema, r.Configuration, r.Template, r.MaxUnitsPerRun, r.UnitResponseBytes, r.UnitTimeoutMS, old, ""}
			oldPolicy := struct {
				Template                         hostProductRequestV3
				UnitResponseBytes, UnitTimeoutMS int
				UnitEnvelope                     *legacyEnvelope `json:",omitempty"`
			}{r.Template, r.UnitResponseBytes, r.UnitTimeoutMS, old}
			oldRaw, err := json.Marshal(legacy)
			if err != nil {
				t.Fatal(err)
			}
			newRaw, err := json.Marshal(r)
			if err != nil {
				t.Fatal(err)
			}
			policyRaw, err := json.Marshal(oldPolicy)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(oldRaw, newRaw) || hostApplicationDigest(r) != pilot.Digest(oldRaw) || hostApplicationUnitPolicyDigest(r) != pilot.Digest(policyRaw) {
				t.Fatalf("ASSERT_APPLICATION_NAVIGATION_LEGACY_BYTES kind=%s", kind)
			}
			if applicationDerivedCohort(r, product.Config{}, "unit", "/controlled-unit").Navigation {
				t.Fatal("ASSERT_APPLICATION_NAVIGATION_LEGACY_DEFAULT")
			}
		}
		t.Log("ASSERT_APPLICATION_NAVIGATION_LEGACY_BYTES A-pass")
	})
	t.Run("hashBinding", func(t *testing.T) {
		off, on := navigationApplicationRequest(t, false), navigationApplicationRequest(t, true)
		if hostApplicationUnitPolicyDigest(off) == hostApplicationUnitPolicyDigest(on) || hostApplicationDigest(off) == hostApplicationDigest(on) {
			t.Fatal("ASSERT_APPLICATION_NAVIGATION_HASH_BOUND")
		}
		// Controlled local fixture only: neither setup nor acquisition is run.
		r, _, _ := hostApplicationCLIFixture(t)
		r.UnitEnvelope = on.UnitEnvelope
		r.UnitTimeoutMS = on.UnitTimeoutMS
		r.Template.OperationTimeoutMS = on.Template.OperationTimeoutMS
		r.Template.ExchangeTimeoutMS = on.Template.ExchangeTimeoutMS
		decode := func(r hostApplicationRequest) error {
			raw, err := json.Marshal(r)
			if err != nil {
				t.Fatal(err)
			}
			_, err = decodeHostApplicationRaw(raw, true)
			return err
		}
		if decode(r) == nil {
			t.Fatal("ASSERT_APPLICATION_NAVIGATION_STALE_APPROVAL_REJECTED")
		}
		policy := hostApplicationUnitPolicyDigest(r)
		r.Configuration.UnitPolicySHA256 = policy
		r.Configuration.ProducerConfigSHA256 = policy
		r.Configuration.ApprovalSHA256 = product.ApplicationConfigDigest(r.Configuration)
		if decode(r) == nil {
			t.Fatal("ASSERT_APPLICATION_NAVIGATION_REQUEST_APPROVAL_REQUIRED")
		}
		r.ApprovalSHA256 = hostApplicationDigest(r)
		if err := decode(r); err != nil {
			t.Fatal("ASSERT_APPLICATION_NAVIGATION_APPROVAL_JOIN", err)
		}
		raw, _ := json.Marshal(r)
		for _, bad := range [][]byte{
			bytes.Replace(raw, []byte(`"Navigation":true`), []byte(`"Navigation":"true"`), 1),
			bytes.Replace(raw, []byte(`"Navigation":true`), []byte(`"navigation":true`), 1),
			bytes.Replace(raw, []byte(`"Navigation":true`), []byte(`"Navigation":true,"Navigation":true`), 1),
		} {
			if _, err := decodeHostApplicationRaw(bad, true); err == nil {
				t.Fatal("ASSERT_APPLICATION_NAVIGATION_STRICT_CANONICAL")
			}
		}
		t.Log("ASSERT_APPLICATION_NAVIGATION_HASH_BOUND A-pass")
	})
	t.Run("allowance", func(t *testing.T) {
		off, on := navigationApplicationRequest(t, false), navigationApplicationRequest(t, true)
		// Nonzero, distinct currencies ensure equality covers processing too.
		off.Template.ApplicationAllowance = pilot.PageCostPrivateV3{LogicalBytes: 11, ReadHashBytes: 13, WriteBytes: 17, Work: 19}
		off.Template.BootstrapAllowance = pilot.PageCostPrivateV3{LogicalBytes: 23, ReadHashBytes: 29, WriteBytes: 31, Work: 37}
		on.Template = off.Template
		a, err := applicationChildAllowance(off, off.Template)
		if err != nil {
			t.Fatal(err)
		}
		b, err := applicationChildAllowance(on, on.Template)
		if err != nil {
			t.Fatal(err)
		}
		if a != b || b.Calls != 6 || b.RequestBytes != 786432 || b.ResponseBytes != 49152 || b.WireBytes != 835584 || b.OutputBytes != 1048576 || b.LogicalBytes != 34 || b.ReadHashBytes != 42 || b.WriteBytes != 48 || b.Work != 56 || b.ActiveNS != 180000000000 {
			t.Fatalf("ASSERT_APPLICATION_NAVIGATION_ALLOWANCE_UNCHANGED off=%+v on=%+v", a, b)
		}
		t.Log("ASSERT_APPLICATION_NAVIGATION_ALLOWANCE_UNCHANGED A-pass")
	})
	t.Run("fallback", func(t *testing.T) {
		r := navigationApplicationRequest(t, true)
		r.UnitEnvelope.MaxCalls = 2
		c := applicationDerivedCohort(r, product.Config{}, "unit", "/controlled-unit")
		if !c.Navigation || c.MaxCalls != 2 {
			t.Fatal("ASSERT_APPLICATION_NAVIGATION_NO_TOPUP")
		}
		r.UnitEnvelope.Schema = "invalid"
		if got := applicationDerivedCohort(r, product.Config{}, "unit", "/controlled-unit"); got != (product.CohortConfig{}) {
			t.Fatal("ASSERT_APPLICATION_NAVIGATION_INVALID_FAIL_CLOSED")
		}
		t.Log("ASSERT_APPLICATION_NAVIGATION_NO_TOPUP A-pass")
	})
}
