package programc_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	pc "lsp-trace/internal/programc"
	"lsp-trace/internal/programcadmission"
	rg "lsp-trace/internal/referencegrouping"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func measurementFixture(t *testing.T) (pc.GroupingEvaluationCasePrivate, pc.GroupingJudgmentsPrivate) {
	t.Helper()
	r := evaluationRequest(t, []programcadmission.RelationKind{programcadmission.TypedReferencesSymbol, programcadmission.TypedResolvesToDefinition}, "H")
	r.Input = evaluationInput(t, []programcadmission.RelationKind{programcadmission.TypedReferencesSymbol, programcadmission.TypedResolvesToDefinition}, true)
	c, err := pc.NewGroupingEvaluationCasePrivate(r)
	if err != nil {
		t.Fatal(err)
	}
	// These labels are chosen before observing any produced partition.
	n := c.Projection().NodeIdentities()
	if len(n) != 3 {
		t.Fatal("controlled three-endpoint prerequisite")
	}
	j, err := pc.NewGroupingJudgmentsPrivate(c, "independent-controlled-labels", []pc.GroupingPairJudgmentPrivate{{Left: n[0], Right: n[1], Label: "SAME"}, {Left: n[0], Right: n[2], Label: "DIFFERENT"}, {Left: n[1], Right: n[2], Label: "ABSTAIN"}})
	if err != nil {
		t.Fatal(err)
	}
	return c, j
}
func TestGroupingMeasurementPrivateScheduledManualOracle(t *testing.T) {
	c, j := measurementFixture(t)
	// H excludes R/D. The actual adopted evaluator must already run successfully;
	// an empty active projection means three singletons, not source absence.
	out, err := pc.EvaluateGroupingCasePrivate(c)
	if err != nil || len(out.Partition().Communities) != 3 {
		t.Fatal("BLOCKED_NOT_RED evaluator prerequisite", err)
	}
	t.Log("CONTROLLED_ADOPTED_EVALUATOR_READY")
	b, err := pc.NewGroupingMeasurementBatchPrivate(pc.GroupingMeasurementBatchRequestPrivate{BatchID: "controlled-batch", ImplementationID: "declared-implementation", ImplementationBytes: []byte("controlled implementation declaration, not attestation"), Slots: []pc.GroupingMeasurementSlotPrivate{{Case: c, Judgments: j}}})
	if err != nil {
		t.Fatal(err)
	}
	result, err := pc.ExecuteGroupingMeasurementBatchPrivate(b)
	if err != nil {
		t.Fatalf("ASSERT_SCHEDULED_MANUAL_COUNTER_REPORT: %v", err)
	}
	d := result.Document()
	want := pc.GroupingPairCountsPrivate{UniversePairs: 3, Declared: 3, Decisive: 2, Abstained: 1, Unjudged: 0, TruePositive: 0, FalsePositive: 0, TrueNegative: 1, FalseNegative: 1}
	if len(d.Rows) != 1 || d.Rows[0].Counts != want {
		t.Fatalf("ASSERT_SCHEDULED_MANUAL_COUNTER_REPORT: got %+v want %+v", d.Rows, want)
	}
	t.Log("ASSERT_SCHEDULED_MANUAL_COUNTER_REPORT: PASS")
}

func measurementBatch(t *testing.T, slots []pc.GroupingMeasurementSlotPrivate) pc.GroupingMeasurementBatchPrivate {
	t.Helper()
	b, e := pc.NewGroupingMeasurementBatchPrivate(pc.GroupingMeasurementBatchRequestPrivate{BatchID: "controlled-explicit-schedule", ImplementationID: "controlled-implementation", ImplementationBytes: []byte("exact controlled declaration; not build attestation"), Slots: slots})
	if e != nil {
		t.Fatal(e)
	}
	return b
}
func measurementExecute(t *testing.T, b pc.GroupingMeasurementBatchPrivate) pc.GroupingMeasurementResultPrivate {
	t.Helper()
	r, e := pc.ExecuteGroupingMeasurementBatchPrivate(b)
	if e != nil {
		t.Fatal(e)
	}
	return r
}
func TestGroupingMeasurementPrivateAllKindsProbesSeeds(t *testing.T) {
	probes, e := rg.CalibrationProbesPrivate(8)
	if e != nil {
		t.Fatal(e)
	}
	kinds := []programcadmission.RelationKind{programcadmission.TypedCalls, programcadmission.TypedReferencesSymbol, programcadmission.TypedResolvesToDefinition}
	for _, seed := range []uint64{7, 83} {
		t.Run(fmt.Sprint(seed), func(t *testing.T) {
			slots := []pc.GroupingMeasurementSlotPrivate{}
			for mask := 1; mask < 8; mask++ {
				selected := []programcadmission.RelationKind{}
				for i, k := range kinds {
					if mask&(1<<i) != 0 {
						selected = append(selected, k)
					}
				}
				input := evaluationInput(t, selected, true)
				var shared pc.GroupingJudgmentsPrivate
				for p, probe := range []string{"H", "U", "L", "B", "N"} {
					c, e := pc.NewGroupingEvaluationCasePrivate(pc.GroupingEvaluationCaseRequestPrivate{CaseID: fmt.Sprintf("controlled-%d-%s", mask, probe), RunID: fmt.Sprintf("run-%d", seed), Seed: seed, SelectorID: "controlled-declared-selection", SelectorBytes: []byte("in-memory controlled selected contributions"), Input: input, Policy: probes[p], ProjectionLimits: evaluationLimits()})
					if e != nil {
						t.Fatal(e)
					}
					if p == 0 {
						n := c.Projection().NodeIdentities()
						shared, e = pc.NewGroupingJudgmentsPrivate(c, fmt.Sprintf("independent-labels-%d", mask), []pc.GroupingPairJudgmentPrivate{{Left: n[0], Right: n[1], Label: "SAME"}})
						if e != nil {
							t.Fatal(e)
						}
					}
					slots = append(slots, pc.GroupingMeasurementSlotPrivate{Case: c, Judgments: shared})
				}
			}
			b := measurementBatch(t, slots)
			r := measurementExecute(t, b)
			d := r.Document()
			if len(d.Rows) != 35 || len(d.Arms) != 7 {
				t.Fatal("ASSERT_ALL_35_SCHEDULED_SLOTS", len(d.Rows), len(d.Arms))
			}
			for i, row := range d.Rows {
				actual, e := pc.EvaluateGroupingCasePrivate(slots[i].Case)
				if e != nil {
					t.Fatal(e)
				}
				if !bytes.Equal(row.ResultBytes, actual.CanonicalBytes()) || row.Result.ID != actual.ID() || row.Case.ID != slots[i].Case.ID() || row.Judgments.ID != slots[i].Judgments.ID() {
					t.Fatal("ASSERT_ACTUAL_SCHEDULE_RESULT_BINDINGS")
				}
				if row.Counts.Declared != 1 || row.Counts.Decisive != 1 || row.Counts.Unjudged != row.Counts.UniversePairs-1 {
					t.Fatal("ASSERT_FINITE_JUDGED_DOMAIN")
				}
				if e := pc.CheckGroupingEvaluationPrivate(slots[i].Case, row.ResultBytes); e != nil {
					t.Fatal(e)
				}
			}
			for _, arm := range d.Arms {
				if arm.Slots != 5 || arm.Counts.Decisive != 5 {
					t.Fatal("ASSERT_EXACT_ARM_COVERAGE")
				}
			}
			if e := pc.CheckGroupingMeasurementBatchPrivate(b, r.CanonicalBytes()); e != nil {
				t.Fatal(e)
			}
			if !bytes.Equal(r.CanonicalBytes(), measurementExecute(t, b).CanonicalBytes()) {
				t.Fatal("ASSERT_DETERMINISTIC_BATCH_REPLAY")
			}
			if d.Authority != 0 || d.Accepted || d.FeatureIdentity != "UNRESOLVED" || d.Completeness != "UNKNOWN" || d.SourceNodeUniverse != "UNKNOWN" || d.AbsenceInference != "PROHIBITED" || d.DownstreamReadiness != "INCOMPLETE" || len(d.NotComputed) != 16 || d.ClaimCeiling != rg.EvaluationCeilingPrivate {
				t.Fatal("ASSERT_PRIVATE_REPORT_CEILING")
			}
			if _, e := rg.ParseGroupingPolicyQualificationReceipt(r.CanonicalBytes(), rg.QualificationParseLimits{MaxRawBytes: 8 << 20, MaxCases: 128, MaxMetrics: 128, MaxCriteria: 128, MaxReviewers: 128, MaxStringBytes: 4096}); e == nil {
				t.Fatal("ASSERT_MEASUREMENT_NOT_PRODUCTION_QUALIFICATION")
			}
			t.Log("ASSERT_ALL_35_SCHEDULED_SLOTS: PASS")
		})
	}
}

func TestGroupingMeasurementPrivatePositiveAndNegativeManualOracle(t *testing.T) {
	request := evaluationRequest(t, []programcadmission.RelationKind{programcadmission.TypedCalls}, "U")
	c, e := pc.NewGroupingEvaluationCasePrivate(request)
	if e != nil {
		t.Fatal(e)
	}
	nodes := c.Projection().NodeIdentities()
	slots := []pc.GroupingMeasurementSlotPrivate{}
	for i, label := range []string{"SAME", "DIFFERENT"} {
		r := request
		r.RunID = fmt.Sprintf("explicit-%d", i)
		c, e := pc.NewGroupingEvaluationCasePrivate(r)
		if e != nil {
			t.Fatal(e)
		}
		j, e := pc.NewGroupingJudgmentsPrivate(c, "independently-declared-"+label, []pc.GroupingPairJudgmentPrivate{{Left: nodes[0], Right: nodes[1], Label: label}})
		if e != nil {
			t.Fatal(e)
		}
		slots = append(slots, pc.GroupingMeasurementSlotPrivate{Case: c, Judgments: j})
	}
	d := measurementExecute(t, measurementBatch(t, slots)).Document()
	// Reciprocal two-node unit graph has one community. Labels above were fixed
	// independently; expected integer confusion is TP=1, FP=1, FN=TN=0.
	if d.Rows[0].Counts.TruePositive != 1 || d.Rows[1].Counts.FalsePositive != 1 || d.Arms[0].Counts != (pc.GroupingPairCountsPrivate{UniversePairs: 2, Declared: 2, Decisive: 2, TruePositive: 1, FalsePositive: 1}) {
		t.Fatal("ASSERT_POSITIVE_NEGATIVE_MANUAL_ORACLE", d.Rows, d.Arms)
	}
	if d.Arms[0].Rates[0].Numerator != 1 || d.Arms[0].Rates[0].Denominator != 2 {
		t.Fatal("ASSERT_SUM_COUNTS_NOT_MEAN_RATES")
	}
}

func TestGroupingMeasurementPrivateZeroAndPartialCoverage(t *testing.T) {
	c, _ := measurementFixture(t)
	nodes := c.Projection().NodeIdentities()
	for _, label := range []string{"missing", "ABSTAIN", "SAME"} {
		t.Run(label, func(t *testing.T) {
			pairs := []pc.GroupingPairJudgmentPrivate{}
			if label != "missing" {
				pairs = append(pairs, pc.GroupingPairJudgmentPrivate{Left: nodes[0], Right: nodes[1], Label: label})
			}
			j, e := pc.NewGroupingJudgmentsPrivate(c, "finite-labels", pairs)
			if e != nil {
				t.Fatal(e)
			}
			d := measurementExecute(t, measurementBatch(t, []pc.GroupingMeasurementSlotPrivate{{Case: c, Judgments: j}})).Document()
			row := d.Rows[0]
			if row.Counts.Unjudged != 3-len(pairs) || row.Rates[0].Status != "NOT_COMPUTED_ZERO_DENOMINATOR" {
				t.Fatal("ASSERT_ZERO_DENOMINATOR_NOT_PERFECT", row)
			}
			if label != "SAME" && (row.Status != "INCOMPLETE_NO_DECISIVE_JUDGMENTS" || row.Counts.Decisive != 0) {
				t.Fatal("ASSERT_ABSTENTION_NOT_SUCCESS", row)
			}
			if label == "ABSTAIN" && row.Counts.Abstained != 1 {
				t.Fatal("ASSERT_ABSTENTION_NOT_SUCCESS", row)
			}
		})
	}
	_, e := pc.ExecuteGroupingMeasurementBatchPrivate(pc.GroupingMeasurementBatchPrivate{})
	var failure *pc.GroupingMeasurementFailurePrivate
	if !errors.As(e, &failure) || failure.Code != "INCOMPLETE_SCHEDULE" {
		t.Fatal("ASSERT_MISSING_SCHEDULE_TYPED_INCOMPLETE", e)
	}
}

func TestGroupingMeasurementPrivateJudgmentAndScheduleGuards(t *testing.T) {
	c, j := measurementFixture(t)
	n := c.Projection().NodeIdentities()
	base := pc.GroupingPairJudgmentPrivate{Left: n[0], Right: n[1], Label: "SAME"}
	for _, pairs := range [][]pc.GroupingPairJudgmentPrivate{
		{base, base}, {base, {Left: n[1], Right: n[0], Label: "DIFFERENT"}},
		{{Left: n[0], Right: "unknown", Label: "SAME"}}, {{Left: n[0], Right: n[0], Label: "SAME"}}, {{Left: n[0], Right: n[1], Label: "INFERRED"}}, make([]pc.GroupingPairJudgmentPrivate, pc.GroupingMeasurementMaxJudgmentsPrivate+1),
	} {
		if _, e := pc.NewGroupingJudgmentsPrivate(c, "independent", pairs); e == nil {
			t.Fatal("ASSERT_INVALID_JUDGMENTS_REFUSED")
		}
	}
	p, e := pc.NewGroupingJudgmentsPrivate(c, "permutation", []pc.GroupingPairJudgmentPrivate{{Left: n[1], Right: n[0], Label: "SAME"}})
	if e != nil {
		t.Fatal(e)
	}
	q, e := pc.NewGroupingJudgmentsPrivate(c, "permutation", []pc.GroupingPairJudgmentPrivate{base})
	if e != nil || p.ID() != q.ID() {
		t.Fatal("ASSERT_CANONICAL_PAIR_IDS")
	}
	request := pc.GroupingMeasurementBatchRequestPrivate{BatchID: "batch", ImplementationID: "impl", ImplementationBytes: []byte("impl"), Slots: []pc.GroupingMeasurementSlotPrivate{{Case: c, Judgments: j}}}
	for _, mutate := range []func(*pc.GroupingMeasurementBatchRequestPrivate){
		func(r *pc.GroupingMeasurementBatchRequestPrivate) { r.Slots = nil },
		func(r *pc.GroupingMeasurementBatchRequestPrivate) { r.Slots = append(r.Slots, r.Slots[0]) },
		func(r *pc.GroupingMeasurementBatchRequestPrivate) {
			r.Slots = make([]pc.GroupingMeasurementSlotPrivate, pc.GroupingMeasurementMaxSlotsPrivate+1)
		},
		func(r *pc.GroupingMeasurementBatchRequestPrivate) { r.ImplementationBytes = nil },
		func(r *pc.GroupingMeasurementBatchRequestPrivate) { r.ImplementationBytes = make([]byte, (1<<16)+1) },
		func(r *pc.GroupingMeasurementBatchRequestPrivate) { r.ImplementationID = strings.Repeat("x", 4097) },
		func(r *pc.GroupingMeasurementBatchRequestPrivate) { r.Slots = []pc.GroupingMeasurementSlotPrivate{{}} },
	} {
		r := request
		mutate(&r)
		if _, e := pc.NewGroupingMeasurementBatchPrivate(r); e == nil {
			t.Fatal("ASSERT_SCHEDULE_BOUNDS_AND_DUPLICATES")
		}
	}
	foreign := evaluationRequest(t, []programcadmission.RelationKind{programcadmission.TypedCalls}, "U")
	fc, e := pc.NewGroupingEvaluationCasePrivate(foreign)
	if e != nil {
		t.Fatal(e)
	}
	request.Slots = []pc.GroupingMeasurementSlotPrivate{{Case: fc, Judgments: j}}
	if _, e := pc.NewGroupingMeasurementBatchPrivate(request); e == nil {
		t.Fatal("ASSERT_FOREIGN_INPUT_JUDGMENTS_REFUSED")
	}
}

func TestGroupingMeasurementPrivateReplayRejectsResealedSubstitutions(t *testing.T) {
	c, j := measurementFixture(t)
	b := measurementBatch(t, []pc.GroupingMeasurementSlotPrivate{{Case: c, Judgments: j}})
	report := measurementExecute(t, b)
	n := c.Projection().NodeIdentities()
	for _, name := range []string{"input", "seed", "selector", "policy", "run", "judgment", "implementation", "batch-id"} {
		t.Run(name, func(t *testing.T) {
			r := evaluationRequest(t, []programcadmission.RelationKind{programcadmission.TypedReferencesSymbol, programcadmission.TypedResolvesToDefinition}, "H")
			r.Input = evaluationInput(t, []programcadmission.RelationKind{programcadmission.TypedReferencesSymbol, programcadmission.TypedResolvesToDefinition}, true)
			switch name {
			case "input":
				r.Input = evaluationInput(t, []programcadmission.RelationKind{programcadmission.TypedCalls}, true)
			case "seed":
				r.Seed++
			case "selector":
				r.SelectorBytes = []byte("foreign selection")
			case "policy":
				r.Policy = evaluationPolicy(t, "U")
			case "run":
				r.RunID = "foreign-run"
			}
			fc, e := pc.NewGroupingEvaluationCasePrivate(r)
			if e != nil {
				t.Fatal(e)
			}
			fj := j
			if name == "input" {
				m := fc.Projection().NodeIdentities()
				fj, e = pc.NewGroupingJudgmentsPrivate(fc, "foreign-input-labels", []pc.GroupingPairJudgmentPrivate{{Left: m[0], Right: m[1], Label: "SAME"}})
			}
			if name == "judgment" {
				fj, e = pc.NewGroupingJudgmentsPrivate(fc, "independent-controlled-labels", []pc.GroupingPairJudgmentPrivate{{Left: n[0], Right: n[1], Label: "DIFFERENT"}})
			}
			if e != nil {
				t.Fatal(e)
			}
			req := pc.GroupingMeasurementBatchRequestPrivate{BatchID: "controlled-explicit-schedule", ImplementationID: "controlled-implementation", ImplementationBytes: []byte("exact controlled declaration; not build attestation"), Slots: []pc.GroupingMeasurementSlotPrivate{{Case: fc, Judgments: fj}}}
			if name == "implementation" {
				req.ImplementationBytes = []byte("foreign implementation")
			}
			if name == "batch-id" {
				req.BatchID = "foreign-batch"
			}
			fb, e := pc.NewGroupingMeasurementBatchPrivate(req)
			if e != nil {
				t.Fatal(e)
			}
			fr := measurementExecute(t, fb)
			if fr.ID() == report.ID() || pc.CheckGroupingMeasurementBatchPrivate(b, fr.CanonicalBytes()) == nil {
				t.Fatal("ASSERT_RESEALED_FOREIGN_BATCH_REFUSED", name)
			}
		})
	}
	for _, field := range []string{"Schedule", "ScheduleBytes", "Contract", "Rows", "Arms", "Authority", "Accepted", "NotComputed"} {
		t.Run("report-"+field, func(t *testing.T) {
			var doc map[string]json.RawMessage
			if e := json.Unmarshal(report.CanonicalBytes(), &doc); e != nil {
				t.Fatal(e)
			}
			doc[field] = json.RawMessage("null")
			raw, e := json.Marshal(doc)
			if e != nil {
				t.Fatal(e)
			}
			raw = append(raw, '\n')
			_ = sha256.Sum256(raw)
			if pc.CheckGroupingMeasurementBatchPrivate(b, raw) == nil {
				t.Fatal("ASSERT_CALLER_RESEALED_COUNTERS_REFUSED")
			}
		})
	}
	if pc.CheckGroupingMeasurementBatchPrivate(b, make([]byte, (8<<20)+1)) == nil {
		t.Fatal("ASSERT_REPORT_BYTE_BOUND")
	}
}

func TestGroupingMeasurementPrivateOmittedSlotRefused(t *testing.T) {
	c, j := measurementFixture(t)
	b := measurementBatch(t, []pc.GroupingMeasurementSlotPrivate{{Case: c, Judgments: j}})
	d := measurementExecute(t, b).Document()
	d.Rows = nil
	raw, e := json.Marshal(d)
	if e != nil {
		t.Fatal(e)
	}
	raw = append(raw, '\n')
	if pc.CheckGroupingMeasurementBatchPrivate(b, raw) == nil {
		t.Fatal("ASSERT_OMITTED_SCHEDULED_SLOT_REFUSED")
	}
	t.Log("ASSERT_OMITTED_SCHEDULED_SLOT_REFUSED: PASS")
	// Extra rows and altered caller-authored counters cannot be repaired by
	// reencoding or hashing either. Check remains against the expected batch.
	d = measurementExecute(t, b).Document()
	d.Rows = append(d.Rows, d.Rows[0])
	raw, e = json.Marshal(d)
	if e != nil {
		t.Fatal(e)
	}
	if pc.CheckGroupingMeasurementBatchPrivate(b, append(raw, '\n')) == nil {
		t.Fatal("ASSERT_EXTRA_SCHEDULED_SLOT_REFUSED")
	}
	d = measurementExecute(t, b).Document()
	d.Rows[0].Counts.FalseNegative = 0
	raw, e = json.Marshal(d)
	if e != nil {
		t.Fatal(e)
	}
	if pc.CheckGroupingMeasurementBatchPrivate(b, append(raw, '\n')) == nil {
		t.Fatal("ASSERT_RESEALED_COUNTER_SUBSTITUTION_REFUSED")
	}
}

func TestGroupingMeasurementPrivateDetachedCopies(t *testing.T) {
	c, j := measurementFixture(t)
	slots := []pc.GroupingMeasurementSlotPrivate{{Case: c, Judgments: j}}
	impl := []byte("declared implementation")
	b, e := pc.NewGroupingMeasurementBatchPrivate(pc.GroupingMeasurementBatchRequestPrivate{BatchID: "batch", ImplementationID: "impl", ImplementationBytes: impl, Slots: slots})
	if e != nil {
		t.Fatal(e)
	}
	out := measurementExecute(t, b)
	want := out.CanonicalBytes()
	slots[0] = pc.GroupingMeasurementSlotPrivate{}
	impl[0] ^= 1
	for _, raw := range [][]byte{j.CanonicalBytes(), b.CanonicalBytes(), out.CanonicalBytes()} {
		raw[0] ^= 1
	}
	d := out.Document()
	d.Rows[0].ResultBytes[0] ^= 1
	d.Rows[0].Counts.TruePositive++
	d.Rows[0].Rates[0].Numerator++
	d.ScheduleBytes[0] ^= 1
	d.NotComputed[0] = "foreign"
	if !bytes.Equal(want, out.CanonicalBytes()) || !reflect.DeepEqual(out.Document(), measurementExecute(t, b).Document()) {
		t.Fatal("ASSERT_DETACHED_IMMUTABLE_BATCH")
	}
}

func TestGroupingMeasurementPrivateUnequalDenominatorsAndFrozenOrder(t *testing.T) {
	c, j := measurementFixture(t)
	req := evaluationRequest(t, []programcadmission.RelationKind{programcadmission.TypedReferencesSymbol, programcadmission.TypedResolvesToDefinition}, "H")
	req.RunID = "second-run"
	small, e := pc.NewGroupingEvaluationCasePrivate(req)
	if e != nil {
		t.Fatal(e)
	}
	missing, e := pc.NewGroupingJudgmentsPrivate(small, "missing-controlled-judgments", nil)
	if e != nil {
		t.Fatal(e)
	}
	slots := []pc.GroupingMeasurementSlotPrivate{{Case: c, Judgments: j}, {Case: small, Judgments: missing}}
	expected := measurementBatch(t, slots)
	d := measurementExecute(t, expected).Document()
	a := d.Arms[0]
	if len(d.Arms) != 1 || a.Slots != 2 || a.Counts.UniversePairs != 4 || a.Counts.Declared != 3 || a.Rates[2].Numerator != 3 || a.Rates[2].Denominator != 4 || a.Status != "INCOMPLETE_FINITE_JUDGED_DOMAIN" {
		t.Fatal("ASSERT_UNEQUAL_DENOMINATORS_SUM_NOT_AVERAGE", a)
	}
	for _, foreign := range [][]pc.GroupingMeasurementSlotPrivate{slots[:1], {slots[1], slots[0]}} {
		b := measurementBatch(t, foreign)
		r := measurementExecute(t, b)
		if pc.CheckGroupingMeasurementBatchPrivate(expected, r.CanonicalBytes()) == nil {
			t.Fatal("ASSERT_FROZEN_SCHEDULE_OMISSION_OR_ORDER_REFUSED")
		}
	}
	// Each case is valid and the oversized collection is still rejected before
	// evaluation; no worst-case Leiden invocation is needed to test byte budgets.
	oversized := make([]pc.GroupingMeasurementSlotPrivate, pc.GroupingMeasurementMaxSlotsPrivate)
	large := evaluationInput(t, []programcadmission.RelationKind{programcadmission.TypedReferencesSymbol, programcadmission.TypedResolvesToDefinition}, true)
	for i := range oversized {
		r := req
		r.Input = large
		r.RunID = fmt.Sprint(i)
		cc, e := pc.NewGroupingEvaluationCasePrivate(r)
		if e != nil {
			t.Fatal(e)
		}
		oversized[i] = pc.GroupingMeasurementSlotPrivate{Case: cc, Judgments: j}
	}
	if _, e := pc.NewGroupingMeasurementBatchPrivate(pc.GroupingMeasurementBatchRequestPrivate{BatchID: "oversized-bytes", ImplementationID: "impl", ImplementationBytes: []byte("declared"), Slots: oversized}); e == nil {
		t.Fatal("ASSERT_AGGREGATE_BYTE_BOUND")
	}
}

func TestGroupingMeasurementPrivateSourcePreservation(t *testing.T) {
	hashes := map[string]string{
		"internal/referencegrouping/composition.go":                      "7ec9e121648fcadfd6e14d7601ae8ad080a6f729d2790693ba571c542dd154be",
		"internal/referencegrouping/evaluation_private.go":               "9d654f60f67888c4140d7b728b86fa28eba16df3a8544c04b3bb3709e43db432",
		"internal/referencegrouping/evaluation_private_test.go":          "8399f8906f3396009116200c5d8ec0fb49318880d073a4dba4e1a0fd1e716509",
		"internal/programc/grouping_evaluation_private.go":               "d1063dcddf93aea3a9a5e2f1af4dd9c1199438e7ceda4657fe5c5db3bd37b18f",
		"internal/programc/grouping_evaluation_private_test.go":          "dfcd506686f284d00580f5b84809e0984d3227ca758db3f513791e8204b0e318",
		"internal/programc/grouping_evaluation_private_external_test.go": "14c909faf7584edd64d77163c7d3378dea292a1b9a4c9bd4c0f6996f04b0722d",
		"docs/pilot/adr0007/private-variable-v2-host-handoff.md":         "13e0b3f37d7eade816a266c2eed2fad6ef50af7cdbd6933561c9d4f2e3d325f9",
		"docs/pilot/adr0007/variable-scope-private-v2-status.md":         "350834897cdec7f053ae7e03791e01e38d0cca8a4016defc4486cc6e00eca128",
	}
	for path, want := range hashes {
		raw, e := os.ReadFile(filepath.Join("../..", path))
		if e != nil {
			t.Fatal(e)
		}
		if fmt.Sprintf("%x", sha256.Sum256(raw)) != want {
			t.Fatalf("ASSERT_LITERAL_SOURCE_PRESERVATION: %s", path)
		}
	}
	t.Log("ASSERT_LITERAL_SOURCE_PRESERVATION: PASS")
}
