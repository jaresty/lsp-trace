//go:build darwin || linux

package censuslineagepilot_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	pilot "lsp-trace/internal/censuslineagepilot"
	"lsp-trace/internal/graphprovenance"
)

func TestScopedPartialRehearsalMessageCapPrivate(t *testing.T) {
	h, m, cfg, f := pilot.NewScopedPartialRehearsalFixturePrivate(t)
	f.Fault("provider-budget")
	report, err := executeScopedPartialRehearsalPrivate(context.Background(), h, m, cfg)
	if err != nil || report.Outcome != "PARTIAL_INSPECTION_RETAINED" || report.SourceDispositionCounts["OUTSIDE_SCOPE"] == 0 {
		t.Fatal("ASSERT_MESSAGE_CAP_NOT_FAILURE", report.Outcome, err)
	}
}

func TestScopedPartialRehearsalOutsideWorkspacePrivate(t *testing.T) {
	h, m, cfg, f := pilot.NewScopedPartialExact25RehearsalFixturePrivate(t, "outside-workspace")
	report, err := executeScopedPartialRehearsalPrivate(context.Background(), h, m, cfg)
	if err != nil {
		t.Fatalf("ASSERT_OUTSIDE_WORKSPACE_ACCEPTED: %v", err)
	}
	if report.SourceDispositionCounts["OUTSIDE_SCOPE"] != 1 || report.SourceDispositionCounts["READABLE"] != 5 || report.SourceDispositionCounts["POLICY_WITHHELD"] != 0 {
		t.Fatal("ASSERT_BOUNDARY_DISTINCT_COUNTS", report.SourceDispositionCounts)
	}
	if report.Outcome != "PARTIAL_INSPECTION_RETAINED" || report.WithheldCount != 0 || report.CanonicalComplete || report.SourceCompleteness != "UNKNOWN" || len(report.Records) != 7 || !report.OutcomeReceipt.Verified {
		t.Fatalf("ASSERT_BOUNDARY_REPORT: %+v", report)
	}
	found, views := false, false
	for _, record := range append(report.Records, *report.OutcomeReceipt) {
		primary, e := os.ReadFile(filepath.Join(cfg.OutputRoots[0], record.ID))
		if e != nil {
			t.Fatal(e)
		}
		backup, e := os.ReadFile(filepath.Join(cfg.OutputRoots[1], record.ID))
		if e != nil || !record.Verified || !bytes.Equal(primary, backup) || pilot.Digest(primary) != record.ID {
			t.Fatal("ASSERT_BOUNDARY_PAIRS")
		}
		if record.Name == "inspection-filter-views" {
			views = bytes.Contains(primary, []byte(`"UNKNOWN"`))
		}
		if record.Name != "bounded-partial-artifact" {
			continue
		}
		carrier, e := graphprovenance.ParseSourceWithholdingPrivateV1(primary)
		if e != nil {
			t.Fatal(e)
		}
		if len(carrier.Evidence.Acquisition.Targets) != 25 || carrier.WithheldCount != 0 || carrier.Evidence.CaptureBudget.Attempts != 5 || len(carrier.Evidence.Captures) != 6 || len(carrier.Evidence.Acquisition.Graph.Edges) != 3 || len(carrier.Evidence.Acquisition.Graph.Frontier) != 2 || carrier.Evidence.Acquisition.Graph.Summary.Complete {
			t.Fatal("ASSERT_BOUNDARY_CAPTURE_ACCOUNTING")
		}
		for _, r := range carrier.Evidence.Captures {
			if r.Status == "OUTSIDE_SCOPE" {
				found = true
				if len(r.Content) != 0 || r.Path != "" {
					t.Fatal("ASSERT_BOUNDARY_NO_SOURCE")
				}
			}
		}
	}
	if !found || !views {
		t.Fatal("ASSERT_BOUNDARY_CARRIER_VIEWS")
	}
	for _, q := range f.RequestsPrivate() {
		if bytes.Contains(q.Params, []byte("uncaptured-outside.go")) {
			t.Fatal("ASSERT_BOUNDARY_NO_QUERY")
		}
	}
	semantic, supplies := f.Traffic()
	if semantic != 104 || supplies != 6 {
		t.Fatalf("ASSERT_BOUNDARY_TRAFFIC %d %d", semantic, supplies)
	}
	live, _, _, _, general, occupied := m.DirectionalAccountingPrivate()
	active, closed, started, released := m.DirectionalGenericTerminalPrivate()
	if live != 0 || general != 0 || occupied || active != 0 || !closed || !started || !released {
		t.Fatal("ASSERT_BOUNDARY_REAL_RELEASE")
	}
}
