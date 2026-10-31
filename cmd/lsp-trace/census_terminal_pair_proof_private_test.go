//go:build darwin || linux

package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	pilot "lsp-trace/internal/censuslineagepilot"
)

func terminalPairProofPrivate(t *testing.T, r censusInventoryReportPrivate) {
	t.Helper()
	origin := r.Origins[0]
	copy := origin.Terminal.Copies[0].Receipt
	var raw [2][]byte
	var stat [2]os.FileInfo
	for i := 0; i < 2; i++ {
		path := filepath.Join(r.AttemptDirectory, fmt.Sprintf("terminal-0-%d", i), copy.Digest)
		var err error
		raw[i], err = os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		stat[i], err = os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
	}
	inspected, err := pilot.InspectTerminalDiscoveryPrivateV2(raw[0])
	assertInventoryPrivate(t, "ASSERT_TERMINAL_ACTUAL_DISTINCT_EXACT_RAW_PAIR", err == nil && bytes.Equal(raw[0], raw[1]) && !os.SameFile(stat[0], stat[1]) && pilot.Digest(raw[0]) == copy.Digest && uint64(len(raw[0])) == copy.ByteLength && inspected.DiscoverySHA256 == pilot.Digest(origin.Discovery))
	t.Logf("TERMINAL_RAW_PAIR digest=%s bytes=%d distinct=true targets=%d symbols=%d reason=%s attempt=%s", copy.Digest, copy.ByteLength, len(inspected.Discovery.Targets), inspected.Discovery.Accounting.SymbolDenominator, inspected.Status, r.AttemptDirectory)
}
