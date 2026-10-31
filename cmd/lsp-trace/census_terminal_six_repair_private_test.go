//go:build darwin || linux

package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	pilot "lsp-trace/internal/censuslineagepilot"
)

func terminalSixStatePrivate(t *testing.T, c censusInventoryConfigPrivate, args []string, r censusInventoryReportPrivate) *censusApplicationProgressPrivate {
	t.Helper()
	o, e := parseCensusCLIOptions(args)
	if e != nil {
		t.Fatal(e)
	}
	raw, e := os.ReadFile(filepath.Join(c.Application.Directory, "full-source-plan.json"))
	if e != nil {
		t.Fatal(e)
	}
	var p censusApplicationFullPlanPrivate
	if decodeCensusApplicationPrivate(raw, &p) != nil {
		t.Fatal("plan")
	}
	store, e := openCensusApplicationStorePrivate(context.Background(), c.Application.Directory, c.Application.MaxReadBytes)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { _ = store.close() })
	s := &censusApplicationProgressPrivate{store: store, options: o, config: c, plan: p, raw: raw, id: pilot.Digest(raw), pages: p.Denominator}
	for _, page := range p.Denominator {
		b, e := os.ReadFile(filepath.Join(r.AttemptDirectory, page.Selector))
		if e != nil {
			t.Fatal(e)
		}
		var rows []censusInventoryFilePrivate
		if json.Unmarshal(b, &rows) != nil {
			t.Fatal("rows")
		}
		s.rows = append(s.rows, rows...)
	}
	return s
}
func terminalSixResealPrivate(t *testing.T, c censusInventoryConfigPrivate, r censusInventoryReportPrivate) {
	t.Helper()
	raw, _ := json.Marshal(r)
	rewriteApplicationFixturePrivate(t, filepath.Join(r.AttemptDirectory, "report.json"), r)
	path := filepath.Join(c.Application.Directory, censusApplicationLeafPrivate(0, "result"))
	b, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	var result censusApplicationResultPrivate
	if json.Unmarshal(b, &result) != nil {
		t.Fatal("result")
	}
	result.ReportSHA256 = pilot.Digest(raw)
	rewriteApplicationFixturePrivate(t, path, result)
	hostPath := filepath.Join(c.Application.Directory, censusApplicationLeafPrivate(0, "host"))
	if e = os.Remove(hostPath); e != nil && !os.IsNotExist(e) {
		t.Fatal(e)
	}
	applicationHostObservationPrivate(t, r, 0, r.ApplicationBinding.ProcessID, true, false)
}
func TestCensusTerminalSixComputedPacketPrivate(t *testing.T) {
	for _, kind := range []string{"missing-batches", "missing-source", "report-batch"} {
		t.Run(kind, func(t *testing.T) {
			_, src, path, args := terminalApplicationFixturePrivate(t)
			if e := os.Rename(filepath.Join(src, "c.go"), filepath.Join(src, "0-normal.go")); e != nil {
				t.Fatal(e)
			}
			c, e := loadCensusInventoryConfigPrivate(path)
			if e != nil {
				t.Fatal(e)
			}
			c.MaxOrigins = 2
			saveInventoryConfigPrivate(t, path, c)
			t.Setenv("LSP_TRACE_OWNED_TERMINAL_SERVER", "1")
			deps, _, _ := inventoryTwoDepsPrivate(t)
			r, code := runInventoryCLIPrivate(t, args, deps)
			if code != 0 {
				t.Fatal("control execution", r.Failure)
			}
			terminalSixResealPrivate(t, c, r)
			s := terminalSixStatePrivate(t, c, args, r)
			if _, e = s.verify(0); e != nil {
				t.Fatal("accepted mixed control", e)
			}
			t.Log("ASSERT_SIX_COMPUTED_CONTROL PASS")
			receipt := r.Origins[0].ProducerCopies[0].Receipt
			raw, e := os.ReadFile(filepath.Join(r.AttemptDirectory, "producer-0-0", receipt.Digest))
			if e != nil {
				t.Fatal(e)
			}
			var packet pilot.ProducerPacketInspectionPrivateV2
			if json.Unmarshal(raw, &packet) != nil {
				t.Fatal("packet")
			}
			switch kind {
			case "missing-batches":
				packet.Batches = nil
			case "missing-source":
				packet.Batches[0].SourceArtifact = nil
			case "report-batch":
				r.Origins[0].Batches = nil
			}
			if kind != "report-batch" {
				raw, _ = json.Marshal(packet)
				id := pilot.Digest(raw)
				for i := 0; i < 2; i++ {
					copy := r.Origins[0].ProducerCopies[i].Receipt
					old := filepath.Join(r.AttemptDirectory, "producer-0-"+string(rune('0'+i)), copy.Digest)
					if e = os.Remove(old); e != nil {
						t.Fatal(e)
					}
					copy.Digest = id
					copy.FinalSelector = id
					copy.ByteLength = uint64(len(raw))
					if e = os.WriteFile(filepath.Join(filepath.Dir(old), id), raw, 0600); e != nil {
						t.Fatal(e)
					}
				}
			}
			terminalSixResealPrivate(t, c, r)
			_, e = s.verify(0)
			assertInventoryPrivate(t, "ASSERT_SIX_COMPUTED_"+kind+"_REFUSED", e != nil && strings.Contains(e.Error(), "producer"))
			t.Logf("computed kind=%s observed=%v", kind, e)
		})
	}
}
func TestCensusTerminalSixPairSelectorPrivate(t *testing.T) {
	_, _, path, args := terminalApplicationFixturePrivate(t)
	c, e := loadCensusInventoryConfigPrivate(path)
	if e != nil {
		t.Fatal(e)
	}
	t.Setenv("LSP_TRACE_OWNED_TERMINAL_SERVER", "1")
	deps, _, _ := inventoryTwoDepsPrivate(t)
	r, code := runInventoryCLIPrivate(t, args, deps)
	if code != 0 {
		t.Fatal(r.Failure)
	}
	terminalSixResealPrivate(t, c, r)
	s := terminalSixStatePrivate(t, c, args, r)
	if _, e = s.verify(0); e != nil {
		t.Fatal("accepted pair control", e)
	}
	t.Log("ASSERT_SIX_PAIR_CONTROL PASS")
	copy := r.Origins[0].Terminal.Copies[1].Receipt
	dir := filepath.Join(r.AttemptDirectory, "terminal-0-1")
	if e = os.Rename(filepath.Join(dir, copy.Digest), filepath.Join(dir, "renamed-object")); e != nil {
		t.Fatal(e)
	}
	_, e = s.verify(0)
	assertInventoryPrivate(t, "ASSERT_SIX_PAIR_SELECTOR_REFUSED", e != nil && strings.Contains(e.Error(), "pair constituent"))
	t.Logf("selector observed=%v", e)
}
