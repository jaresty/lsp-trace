package programc_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	pc "lsp-trace/internal/programc"
	"lsp-trace/internal/programcadmission"
	"strings"
	"testing"
)

func bundleRequest(t *testing.T, mask int, probe string) (pc.GroupingCaseBundleRequestPrivate, pc.GroupingMeasurementBatchPrivate) {
	t.Helper()
	kinds := []programcadmission.RelationKind{}
	for i, k := range []programcadmission.RelationKind{programcadmission.TypedCalls, programcadmission.TypedReferencesSymbol, programcadmission.TypedResolvesToDefinition} {
		if mask&(1<<i) != 0 {
			kinds = append(kinds, k)
		}
	}
	input, preds := bundleFixtureInput(t, kinds, true)
	r := pc.GroupingEvaluationCaseRequestPrivate{CaseID: fmt.Sprintf("controlled-%d-%s", mask, probe), RunID: "run-7", SelectorID: "declared-not-executed", SelectorBytes: []byte("controlled selection\n"), Seed: 7, Input: input, Policy: evaluationPolicy(t, probe), ProjectionLimits: evaluationLimits()}
	c, e := pc.NewGroupingEvaluationCasePrivate(r)
	if e != nil {
		t.Fatal(e)
	}
	nodes := c.Projection().NodeIdentities()
	pairs := []pc.GroupingPairJudgmentPrivate{{Left: nodes[0], Right: nodes[1], Label: "SAME"}}
	j, e := pc.NewGroupingJudgmentsPrivate(c, "independent-controlled", pairs)
	if e != nil {
		t.Fatal(e)
	}
	req := pc.GroupingCaseBundleRequestPrivate{Schema: pc.GroupingCaseBundleSchemaPrivate, BatchID: "controlled-offline", ImplementationID: "declaration-only", ImplementationBytes: []byte("exact declaration\n"), Slots: []pc.GroupingCaseBundleSlotPrivate{{InputBytes: input.CanonicalBytes(), Predecessors: preds, PolicyBytes: r.Policy.CanonicalBytes(), CaseID: r.CaseID, SelectorID: r.SelectorID, RunID: r.RunID, SelectorBytes: r.SelectorBytes, Seed: r.Seed, ProjectionLimits: r.ProjectionLimits, JudgmentID: "independent-controlled", Pairs: pairs, CaseBytes: c.CanonicalBytes(), JudgmentBytes: j.CanonicalBytes()}}}
	batch, e := pc.NewGroupingMeasurementBatchPrivate(pc.GroupingMeasurementBatchRequestPrivate{BatchID: req.BatchID, ImplementationID: req.ImplementationID, ImplementationBytes: req.ImplementationBytes, Slots: []pc.GroupingMeasurementSlotPrivate{{Case: c, Judgments: j}}})
	if e != nil {
		t.Fatal(e)
	}
	return req, batch
}
func TestGroupingCaseBundlePrivateRoundtrip(t *testing.T) {
	req, baseline := bundleRequest(t, 6, "H")
	report, e := pc.ExecuteGroupingMeasurementBatchPrivate(baseline)
	if e != nil {
		t.Fatal("BLOCKED_NOT_RED", e)
	}
	if e = pc.CheckGroupingMeasurementBatchPrivate(baseline, report.CanonicalBytes()); e != nil {
		t.Fatal(e)
	}
	t.Log("CONTROLLED_BASELINE_EVALUATOR_BATCH_READY")
	b, e := pc.NewGroupingCaseBundlePrivate(req)
	if e != nil {
		t.Fatalf("ASSERT_OFFLINE_RECONSTRUCT_REPORT_ROUNDTRIP: %v", e)
	}
	raw := b.CanonicalBytes()
	expected := b.Commitment()
	reopened, e := pc.ImportGroupingCaseBundlePrivate(expected, raw)
	if e != nil {
		t.Fatalf("ASSERT_OFFLINE_RECONSTRUCT_REPORT_ROUNDTRIP: %v", e)
	}
	if !bytes.Equal(raw, reopened.CanonicalBytes()) || !bytes.Equal(baseline.CanonicalBytes(), reopened.Batch().CanonicalBytes()) {
		t.Fatal("ASSERT_OFFLINE_RECONSTRUCT_REPORT_ROUNDTRIP: changed bytes")
	}
	if e = pc.CheckGroupingMeasurementBatchPrivate(reopened.Batch(), report.CanonicalBytes()); e != nil {
		t.Fatal(e)
	}
	actual, e := pc.ExecuteGroupingMeasurementBatchPrivate(reopened.Batch())
	if e != nil || !bytes.Equal(actual.CanonicalBytes(), report.CanonicalBytes()) {
		t.Fatal("ASSERT_OFFLINE_RECONSTRUCT_REPORT_ROUNDTRIP", e)
	}
	t.Log("ASSERT_OFFLINE_RECONSTRUCT_REPORT_ROUNDTRIP: PASS")
}
func bundleRaw(t *testing.T, r pc.GroupingCaseBundleRequestPrivate) []byte {
	t.Helper()
	b, e := json.Marshal(r)
	if e != nil {
		t.Fatal(e)
	}
	return append(b, '\n')
}
func TestGroupingCaseBundlePrivateExpectedIdentity(t *testing.T) {
	r, _ := bundleRequest(t, 7, "L")
	b, e := pc.NewGroupingCaseBundlePrivate(r)
	if e != nil {
		t.Fatal(e)
	}
	foreign := r
	foreign.ImplementationBytes = []byte("foreign self-consistent declaration\n")
	f, e := pc.NewGroupingCaseBundlePrivate(foreign)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = pc.ImportGroupingCaseBundlePrivate(b.Commitment(), f.CanonicalBytes()); e == nil {
		t.Fatal("ASSERT_FOREIGN_EXPECTED_BUNDLE_REFUSED")
	}
	t.Log("ASSERT_FOREIGN_EXPECTED_BUNDLE_REFUSED: PASS")
}
func TestGroupingCaseBundlePrivateMissingBinding(t *testing.T) {
	r, _ := bundleRequest(t, 6, "H")
	r.Slots[0].CaseBytes = nil
	if _, e := pc.NewGroupingCaseBundlePrivate(r); e == nil {
		t.Fatal("ASSERT_MISSING_CASE_BINDING_REFUSED")
	}
	t.Log("ASSERT_MISSING_CASE_BINDING_REFUSED: PASS")
}
func TestGroupingCaseBundlePrivateAllModesProbes(t *testing.T) {
	req := pc.GroupingCaseBundleRequestPrivate{Schema: pc.GroupingCaseBundleSchemaPrivate, BatchID: "explicit-35", ImplementationID: "declared", ImplementationBytes: []byte("controlled, not attestation\n")}
	for mask := 1; mask < 8; mask++ {
		for _, p := range []string{"H", "U", "L", "B", "N"} {
			r, _ := bundleRequest(t, mask, p)
			req.Slots = append(req.Slots, r.Slots[0])
		}
	}
	b, e := pc.NewGroupingCaseBundlePrivate(req)
	if e != nil {
		t.Fatal(e)
	}
	reopened, e := pc.ImportGroupingCaseBundlePrivate(b.Commitment(), b.CanonicalBytes())
	if e != nil {
		t.Fatal(e)
	}
	result, e := pc.ExecuteGroupingMeasurementBatchPrivate(reopened.Batch())
	if e != nil {
		t.Fatal(e)
	}
	if e = pc.CheckGroupingMeasurementBatchPrivate(reopened.Batch(), result.CanonicalBytes()); e != nil {
		t.Fatal(e)
	}
	d := result.Document()
	if len(d.Rows) != 35 || len(d.Arms) != 7 || d.Authority != 0 || d.Accepted || d.SourceNodeUniverse != "UNKNOWN" || d.UniverseScope != "CONTRIBUTION_ENDPOINTS_ONLY" || d.DownstreamReadiness != "INCOMPLETE" || len(d.NotComputed) != 16 {
		t.Fatal("ASSERT_OFFLINE_CEILING_AND_SCHEDULE")
	}
	for _, row := range d.Rows {
		if row.Case.ID == "" || row.Result.ID == "" || row.Judgments.ID == "" {
			t.Fatal("ASSERT_OFFLINE_ROW_BINDING")
		}
	}
}
func TestGroupingCaseBundlePrivateBindingsAndMalformed(t *testing.T) {
	r, _ := bundleRequest(t, 7, "L")
	b, e := pc.NewGroupingCaseBundlePrivate(r)
	if e != nil {
		t.Fatal(e)
	}
	for _, name := range []string{"input", "predecessor", "query", "policy", "selector", "judgment", "case", "run", "slot", "implementation"} {
		t.Run(name, func(t *testing.T) {
			var x pc.GroupingCaseBundleRequestPrivate
			if e := json.Unmarshal(b.CanonicalBytes(), &x); e != nil {
				t.Fatal(e)
			}
			switch name {
			case "input":
				x.Slots[0].InputBytes = append(x.Slots[0].InputBytes, ' ')
			case "predecessor":
				x.Slots[0].Predecessors[0].Bytes = append(x.Slots[0].Predecessors[0].Bytes, ' ')
			case "query":
				for i := range x.Slots[0].Predecessors {
					if x.Slots[0].Predecessors[i].Role == "METHOD_QUERY" {
						x.Slots[0].Predecessors[i].Bytes = []byte("{\"query\":false}\n")
					}
				}
			case "policy":
				x.Slots[0].PolicyBytes = evaluationPolicy(t, "U").CanonicalBytes()
			case "selector":
				x.Slots[0].SelectorBytes = []byte("foreign selector\n")
			case "judgment":
				x.Slots[0].Pairs[0].Label = "DIFFERENT"
			case "case":
				x.Slots[0].CaseBytes = nil
			case "run":
				x.Slots[0].RunID = "foreign-run"
			case "slot":
				x.Slots = nil
			case "implementation":
				x.ImplementationBytes = []byte("foreign")
			}
			raw := bundleRaw(t, x)
			if _, e := pc.ImportGroupingCaseBundlePrivate(b.Commitment(), raw); e == nil {
				t.Fatal("ASSERT_SUBSTITUTION_REFUSED", name)
			}
			if name != "implementation" {
				if _, e := pc.NewGroupingCaseBundlePrivate(x); e == nil {
					t.Fatal("ASSERT_RECONSTRUCTION_REFUSED", name)
				}
			}
		})
	}
	for _, raw := range [][]byte{nil, b.CanonicalBytes()[:20], append(b.CanonicalBytes(), []byte("{}")...), []byte("{\"Schema\":\"unknown\"}\n"), []byte(strings.Repeat("[", 300)), []byte(strings.Repeat("x", (2<<20)+1)), bytes.Replace(b.CanonicalBytes(), []byte("\"Schema\":"), []byte("\"Unknown\":0,\"Schema\":"), 1), bytes.Replace(b.CanonicalBytes(), []byte("\"Schema\":"), []byte("\"Schema\":\"duplicate\",\"Schema\":"), 1)} {
		if _, e := pc.ImportGroupingCaseBundlePrivate(b.Commitment(), raw); e == nil {
			t.Fatal("ASSERT_MALFORMED_REFUSED")
		}
	}
}
