//go:build darwin || linux

package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"lsp-trace/sessionruntime"
)

// Exercise the real connected discovery admission/custody code. The existing
// fixture creates fresh synthetic test accounting only; the starter refuses
// before constructing any native provider or application HOST.
func TestCensusCallerHostDiscoveryCustody(t *testing.T) {
	_, _, header, _ := connectedCursorFixture(t, 16)
	first := custodyFundedConfig46(t, header)
	second := custodyFundedConfig46(t, header)
	starts := 0
	deps := censusRunnerDependencies{newStarter: func() (sessionruntime.Starter, error) {
		starts++
		return nil, errors.New("controlled refusal before provider construction")
	}}
	invoke := func(c censusInventoryBridgeConfig) int {
		t.Helper()
		raw, err := json.Marshal(c)
		if err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(t.TempDir(), "connected.json")
		if err := os.WriteFile(path, raw, 0600); err != nil {
			t.Fatal(err)
		}
		var out, diagnostic bytes.Buffer
		return runCensusWithDependencies(publicCallerHostArgs(path), &out, &diagnostic, deps)
	}
	if code := invoke(first); code == 0 || starts != 1 {
		t.Fatalf("ASSERT_PUBLIC_HOST_CONNECTED_DISPATCH: code=%d starts=%d", code, starts)
	}
	t.Log("ASSERT_PUBLIC_HOST_CONNECTED_DISPATCH PASS")
	if code := invoke(second); code == 0 || starts != 1 {
		t.Fatalf("ASSERT_PUBLIC_HOST_NO_REPLAY: code=%d starts=%d", code, starts)
	}
	t.Log("ASSERT_PUBLIC_HOST_NO_REPLAY PASS")
}
