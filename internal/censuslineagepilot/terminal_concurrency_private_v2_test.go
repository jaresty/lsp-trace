//go:build darwin || linux

package censuslineagepilot

import (
	"context"
	"encoding/json"
	"testing"

	"lsp-trace/internal/lspwire"
)

func TestTerminalOneRetentionAttemptPrivateV2(t *testing.T) {
	_, run := terminalFixturePrivateV2(t, 84, func(ctx context.Context, f FrozenScopePrivateV2, o TerminalDiscoveryOutcomePrivateV2) {
		roots, _, _ := packetRootsPrivateV2(t, f)
		results := make(chan error, 2)
		for i := 0; i < 2; i++ {
			go func() { _, e := RetainTerminalDiscoveryPrivateV2(ctx, f, o, roots); results <- e }()
		}
		first, second := <-results, <-results
		if (first == nil) == (second == nil) {
			t.Fatal("ASSERT_TERMINAL_ONE_CONCURRENT_ATTEMPT", first, second)
		}
	})
	_, _, _, err := run(false)
	if err != nil {
		t.Fatal(err)
	}
}
func TestTerminalProviderFailureNeverGapPrivateV2(t *testing.T) {
	spec, m, id, child := producerFixturePrivateV2(t, 1, 0)
	child.mu.Lock()
	child.response = func(lspwire.Message) json.RawMessage { return json.RawMessage(`{"invalid":"documentSymbol response"}`) }
	child.mu.Unlock()
	f, err := FreezeScopePrivateV2(context.Background(), spec)
	if err != nil {
		t.Fatal(err)
	}
	producerLeasePrivateV2(t, m, id, spec.Root, func(ctx context.Context) error {
		origin, terminal, _, e := DiscoverScopeOutcomePrivateV2(ctx, f, m, id)
		if e == nil || terminal.Reason() != "" || len(origin.Bytes()) != 0 {
			t.Fatal("ASSERT_TERMINAL_PROVIDER_FAILURE_HARD_STOP", e)
		}
		return nil
	})
}
