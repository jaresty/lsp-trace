//go:build darwin || linux

package censuslineagepilot

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestTerminalOriginalOwnerRefusalsPrivateV2(t *testing.T) {
	for _, kind := range []string{"source-drift", "canceled", "foreign-runtime", "foreign-identity", "foreign-frozen"} {
		t.Run(kind, func(t *testing.T) {
			_, run := terminalFixturePrivateV2(t, 84, func(ctx context.Context, f FrozenScopePrivateV2, o TerminalDiscoveryOutcomePrivateV2) {
				roots, _, _ := packetRootsPrivateV2(t, f)
				switch kind {
				case "source-drift":
					path := filepath.Join(f.state.spec.Root, f.state.spec.Members[0].CanonicalRelativePath)
					if e := os.WriteFile(path, []byte("package owned\n"), 0600); e != nil {
						t.Fatal(e)
					}
					defer func() {
						if e := os.WriteFile(path, f.state.files[0].Bytes, 0600); e != nil {
							t.Fatal(e)
						}
					}()
				case "canceled":
					var cancel context.CancelFunc
					ctx, cancel = context.WithCancel(ctx)
					cancel()
				case "foreign-runtime", "foreign-identity":
					// Construct a structurally wrong internal counterfeit only for refusal.
					original := o.state
					s := &terminalDiscoveryStatePrivateV2{adapter: original.adapter, frozen: original.frozen, manager: original.manager, guard: original.guard, identity: original.identity, lifetime: original.lifetime, raw: original.raw, reason: original.reason}
					s.self = s
					if kind == "foreign-runtime" {
						s.manager = nil
					} else {
						s.identity.Generation++
					}
					o = TerminalDiscoveryOutcomePrivateV2{s}
				case "foreign-frozen":
					other, e := FreezeScopePrivateV2(ctx, f.state.spec)
					if e != nil {
						t.Fatal(e)
					}
					f = other
				}
				if _, e := RetainTerminalDiscoveryPrivateV2(ctx, f, o, roots); e == nil {
					t.Fatal("ASSERT_TERMINAL_ORIGINAL_OWNER_REFUSAL", kind)
				}
			})
			_, _, _, err := run(false)
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}
func TestTerminalNoLeaseRestorationPrivateV2(t *testing.T) {
	var saved TerminalDiscoveryOutcomePrivateV2
	f, run := terminalFixturePrivateV2(t, 84, func(_ context.Context, _ FrozenScopePrivateV2, o TerminalDiscoveryOutcomePrivateV2) { saved = o })
	_, _, _, err := run(false)
	if err != nil {
		t.Fatal(err)
	}
	roots, _, _ := packetRootsPrivateV2(t, f)
	if _, err = RetainTerminalDiscoveryPrivateV2(context.Background(), f, saved, roots); err == nil {
		t.Fatal("ASSERT_TERMINAL_CANNOT_RESTORE_ORIGINAL_LEASE")
	}
	if _, err = RetainTerminalDiscoveryPrivateV2(context.Background(), f, saved, roots); err == nil {
		t.Fatal("ASSERT_TERMINAL_FAILED_RETENTION_SPENT")
	}
}
