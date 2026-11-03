//go:build darwin

package censuslineagepilot_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	pilot "lsp-trace/internal/censuslineagepilot"
	prep "lsp-trace/internal/privatedescribeprep"
	derived "lsp-trace/internal/retainedpartialprogramc"
)

// This bridge uses only existing controlled fixture APIs. The measured boundary
// is ONE selected target from the twenty-target controlled retained fixture,
// through three-reference publication; it is not whole-cohort preparation.
// Baseline and measured modes independently create the same fresh fixture.
func TestRetainedCheckpointPrivateV3CostWitnessExternalFixture(t *testing.T) {
	mode := os.Getenv("LSP_COST_MODE")
	if mode == "" {
		t.Skip("isolated coverage helper only")
	}
	if mode != "flow-baseline" && mode != "flow" {
		t.Fatal("unknown external mode")
	}
	ctx := context.Background()
	grant := pipelineGrantV3()
	dir := t.TempDir()
	if e := os.Chmod(dir, 0700); e != nil {
		t.Fatal(e)
	}
	path := filepath.Join(dir, "transfer.json")
	pilot.ControlledRetainedCheckpointExportPrivateV3(t, path)
	owner, _ := pilot.ControlledRetainedCheckpointOpenPrivateV3(t, path)
	raw, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	var envelope struct {
		Manifest   []byte
		Spec       pilot.ScopeSpecPrivateV2
		LedgerPath string
		Expected   [20][4]string
	}
	if e = json.Unmarshal(raw, &envelope); e != nil {
		t.Fatal(e)
	}
	var manifest pilot.RetainedPageCheckpointManifestPrivateV3
	if e = json.Unmarshal(envelope.Manifest, &manifest); e != nil {
		t.Fatal(e)
	}
	type record struct {
		Schema      string                  `json:"schema"`
		Application string                  `json:"application"`
		Generation  uint64                  `json:"generation"`
		Previous    string                  `json:"previous_sha256"`
		Allowance   pilot.PageCostPrivateV3 `json:"application_allowance"`
		Reserved    pilot.PageCostPrivateV3 `json:"reserved"`
		Cumulative  pilot.PageCostPrivateV3 `json:"cumulative"`
	}
	// Start at the just-created transfer's known generation, never the journal's
	// historical beginning. The fixture-only scan cap derives from the current
	// page/index counts plus a fixed margin; exhausting it REFUSES measurement,
	// not a price bound or permission to extend a producer allowance.
	frontier := func(start, bound uint64) (uint64, string, pilot.PageCostPrivateV3, []pilot.PageCostPrivateV3) {
		var g uint64
		var tip string
		var spent pilot.PageCostPrivateV3
		var records []pilot.PageCostPrivateV3
		for i := start; i <= start+bound; i++ {
			data, err := os.ReadFile(filepath.Join(envelope.LedgerPath, fmt.Sprintf("reservation-%020d.json", i)))
			if os.IsNotExist(err) {
				if i == start {
					t.Fatal("CW_FLOW_LEDGER: missing starting frontier")
				}
				return g, tip, spent, records
			}
			if err != nil {
				t.Fatal(err)
			}
			var r record
			if err = json.Unmarshal(data, &r); err != nil || r.Generation != i || r.Schema != "lsp-trace.resource-reservations.private.v3" || r.Application != manifest.Application || r.Allowance != manifest.Allowance {
				t.Fatal("CW_FLOW_LEDGER: malformed", err)
			}
			canonical, err := json.Marshal(r)
			if err != nil || !bytes.Equal(canonical, data) {
				t.Fatal("CW_FLOW_LEDGER: noncanonical/unknown/duplicate fields")
			}
			if i > start {
				if r.Previous != tip || r.Cumulative != (pilot.PageCostPrivateV3{LogicalBytes: spent.LogicalBytes + r.Reserved.LogicalBytes, ReadHashBytes: spent.ReadHashBytes + r.Reserved.ReadHashBytes, WriteBytes: spent.WriteBytes + r.Reserved.WriteBytes, Work: spent.Work + r.Reserved.Work}) {
					t.Fatal("CW_FLOW_LEDGER: chain")
				}
				records = append(records, r.Reserved)
			}
			g = i
			tip = pilot.Digest(data)
			spent = r.Cumulative
		}
		t.Fatal("CW_FLOW_LEDGER: bounded frontier exhausted")
		return 0, "", pilot.PageCostPrivateV3{}, nil
	}
	g, _, before, _ := frontier(manifest.LedgerGeneration, manifest.Revision.Cursor.PageCount+manifest.Objects+64)
	if before != owner.BudgetSpent() {
		t.Fatal("CW_FLOW_LEDGER: initial spent")
	}
	if mode == "flow" {
		b, err := owner.SelectedBatch(ctx, 0, grant)
		if err != nil {
			t.Fatal(err)
		}
		v, err := b.Read(ctx, grant)
		if err != nil {
			t.Fatal(err)
		}
		if v.Ordinal != 0 || v.FullTargetCount != 20 {
			t.Fatal("CW_FLOW: selected extent")
		}
		p, err := derived.ProjectRetainedCheckpointPrivateV3(ctx, b, grant)
		if err != nil {
			t.Fatal(err)
		}
		a, err := derived.ComputeInstabilityRetainedCheckpointPrivateV3(ctx, p, []uint64{3, 7}, grant)
		if err != nil {
			t.Fatal(err)
		}
		limits := prep.Limits{MaxTargets: 64, MaxPartBytes: 64 << 10, MaxTotalBytes: 8 << 20, MaxParts: 256, MaxSourceBytes: 2 << 20}
		reader, err := prep.NewOwnASTReaderRetainedCheckpointPrivateV3(ctx, p, a, limits, grant)
		if err != nil {
			t.Fatal(err)
		}
		_, own, err := reader.Advance(ctx, 1, grant)
		if err != nil {
			t.Fatal(err)
		}
		body, err := own.Read(ctx, grant)
		if err != nil {
			t.Fatal(err)
		}
		var report prep.OwnASTReportRetainedCheckpointPrivateV3
		if err = json.Unmarshal(body, &report); err != nil || report.PreparedCount != 1 || len(report.Bodies) != 1 || report.Bodies[0].Declaration.Body != "func Owned00() {}" {
			t.Fatal("CW_FLOW: own body", err)
		}
		pub, err := own.Publication(ctx, grant)
		if err != nil {
			t.Fatal(err)
		}
		refs, err := pub.Advance(ctx, 1, grant)
		if err != nil || refs.VerifiedReferences != 3 {
			t.Fatal("CW_FLOW: publication", err)
		}
	}
	_, _, after, records := frontier(g, 512)
	if after != owner.BudgetSpent() {
		t.Fatal("CW_FLOW_LEDGER: final spent")
	}
	// Independent policy inputs, not inferred from the charged-record pattern.
	n := uint64(128<<10) + 4*manifest.Revision.Policy.ComputationUnitBytes
	for _, m := range envelope.Spec.Members {
		info, err := os.Stat(filepath.Join(envelope.Spec.Root, m.CanonicalRelativePath))
		if err != nil {
			t.Fatal(err)
		}
		n += uint64(info.Size())
	}
	pn := uint64(envelope.Spec.EffectivePolicy.TotalBytes)
	if x := uint64(len(envelope.Spec.Members) * envelope.Spec.EffectivePolicy.FileBytes); x < pn {
		pn = x
	}
	pn += uint64(len(envelope.Spec.Members)) + 128<<10
	// The internal driver supplies the private topology-work constant; logical
	// and read-hash costs use independently observed source policy inputs.
	result := struct {
		Files                     int
		Stage, Source, Allocation pilot.PageCostPrivateV3
		Expected, Records         []pilot.PageCostPrivateV3
		Delta                     pilot.PageCostPrivateV3
		Outcome                   string
		SourceHashes              map[string]string
	}{Files: len(envelope.Spec.Members), Stage: pilot.PageCostPrivateV3{LogicalBytes: 256 * n, ReadHashBytes: 256 * n, Work: 1000000}, Source: pilot.PageCostPrivateV3{LogicalBytes: 24 * pn, ReadHashBytes: 32 * pn}, Records: records, Delta: pilot.PageCostPrivateV3{LogicalBytes: after.LogicalBytes - before.LogicalBytes, ReadHashBytes: after.ReadHashBytes - before.ReadHashBytes, WriteBytes: after.WriteBytes - before.WriteBytes, Work: after.Work - before.Work}, Outcome: "FLOW"}
	// The source identity set is fixed and shared with the internal reader.
	result.SourceHashes = map[string]string{}
	for _, name := range []string{"producer_pages_checkpoint_admission_private_v3.go", "variable_scope_private_v2.go", "feature_scope_source_unix.go", "producer_pages_ledger_private_v3.go"} {
		data, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		result.SourceHashes[name] = pilot.Digest(data)
	}
	data, e := json.Marshal(result)
	if e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(os.Getenv("LSP_COST_RESULT"), data, 0600); e != nil {
		t.Fatal(e)
	}
	t.Log("CW_FIXTURE_COMPLETED", mode)
}
