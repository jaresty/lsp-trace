//go:build darwin || linux

package censuslineagepilot

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"testing"
)

func TestTerminalRetentionOwnedPrivateV2(t *testing.T) {
	for _, n := range []int{84, 0} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			_, run := terminalFixturePrivateV2(t, n, func(ctx context.Context, f FrozenScopePrivateV2, o TerminalDiscoveryOutcomePrivateV2) {
				roots, _, _ := packetRootsPrivateV2(t, f)
				copies, err := RetainTerminalDiscoveryPrivateV2(ctx, f, o, roots)
				if err != nil || copies[0].Receipt == nil || copies[1].Receipt == nil {
					t.Fatal("ASSERT_TERMINAL_RETAIN_ORIGINAL_OWNER", err)
				}
				raw, err := roots.state.outputs[0].ReadTerminalDiscoveryPrivateV2(ctx, copies[0].Receipt.Digest, int64(copies[0].Receipt.ByteLength))
				if err != nil {
					t.Fatal(err)
				}
				if _, err = InspectTerminalDiscoveryPrivateV2(raw); err != nil {
					t.Fatal("ASSERT_TERMINAL_CLOSED_READBACK", err, string(raw))
				}
				if _, err = RetainTerminalDiscoveryPrivateV2(ctx, f, o, roots); err == nil {
					t.Fatal("ASSERT_TERMINAL_RETENTION_SPENT")
				}
				for _, kind := range []string{"reason", "count", "unknown", "omitted", "policy", "scope", "metadata", "closure", "provider"} {
					t.Run(kind, func(t *testing.T) {
						var v TerminalDiscoveryInspectionPrivateV2
						if json.Unmarshal(raw, &v) != nil {
							t.Fatal("setup")
						}
						switch kind {
						case "reason":
							v.Status = "COMPLETE"
						case "count":
							v.Discovery.Accounting.SymbolDenominator++
						case "unknown":
							bad := bytes.Replace(raw, []byte(`"Authority":0`), []byte(`"Authority":0,"ExtraControl":true`), 1)
							if _, e := InspectTerminalDiscoveryPrivateV2(bad); e == nil {
								t.Fatal("ASSERT_TERMINAL_REJECT_UNKNOWN")
							}
							return
						case "omitted":
							bad := bytes.Replace(raw, []byte(`"Accepted":false,`), nil, 1)
							if _, e := InspectTerminalDiscoveryPrivateV2(bad); e == nil {
								t.Fatal("ASSERT_TERMINAL_REJECT_OMITTED")
							}
							return
						case "policy":
							v.EffectivePolicy = bytes.Replace(v.EffectivePolicy, []byte(`"max_targets":63`), []byte(`"max_targets":84`), 1)
							v.PolicySHA256 = Digest(v.EffectivePolicy)
						case "scope":
							v.ScopeSHA256 = Digest([]byte("foreign"))
						case "metadata":
							v.RuntimeMetadata = bytes.Replace(v.RuntimeMetadata, []byte(`{`), []byte(`{"ForeignControl":true,`), 1)
						case "closure":
							v.Discovery.Closure = 0
						case "provider":
							v.Discovery.Complete = false
						}
						if kind == "provider" && n == 0 {
							v.Discovery.Observations[0].Known = true
						}
						d, _ := json.Marshal(v.Discovery)
						v.DiscoverySHA256 = Digest(d)
						bad, _ := json.Marshal(v)
						if bytes.Equal(bad, raw) {
							t.Fatal("negative control did not perturb record")
						}
						if _, e := InspectTerminalDiscoveryPrivateV2(bad); e == nil {
							t.Fatalf("ASSERT_TERMINAL_REJECT_%s", kind)
						}
					})
				}
			})
			_, _, _, err := run(false)
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}
func TestTerminalRetentionForeignPrivateV2(t *testing.T) {
	_, run := terminalFixturePrivateV2(t, 84, func(ctx context.Context, f FrozenScopePrivateV2, o TerminalDiscoveryOutcomePrivateV2) {
		roots, _, _ := packetRootsPrivateV2(t, f)
		foreign := o
		original := o.state
		copy := &terminalDiscoveryStatePrivateV2{adapter: original.adapter, frozen: original.frozen, manager: original.manager, identity: original.identity, lifetime: original.lifetime, raw: original.raw, reason: original.reason}
		copy.self = copy
		foreign.state = copy // deliberately missing the exact original guard
		if _, err := RetainTerminalDiscoveryPrivateV2(ctx, f, foreign, roots); err == nil {
			t.Fatal("ASSERT_TERMINAL_FOREIGN_GUARD")
		}
		if _, err := RetainTerminalDiscoveryPrivateV2(ctx, FrozenScopePrivateV2{}, o, roots); err == nil {
			t.Fatal("ASSERT_TERMINAL_FOREIGN_SCOPE")
		}
		if _, err := RetainTerminalDiscoveryPrivateV2(ctx, f, o, roots); err != nil {
			t.Fatal(err)
		}
	})
	_, _, _, err := run(false)
	if err != nil {
		t.Fatal(err)
	}
}
