package programc_test

import (
	"bytes"
	"context"
	"fmt"
	pc "lsp-trace/internal/programc"
	rg "lsp-trace/internal/referencegrouping"
	"testing"
	"time"
)

func TestTypedBundlePrivateDispatch(t *testing.T) {
	req, _ := bundleRequest(t, 7, "H")
	b, err := pc.NewGroupingCaseBundlePrivate(req)
	if err != nil {
		t.Fatal(err)
	}
	direct, err := b.ExecuteReport()
	if err != nil {
		t.Fatal(err)
	}
	if err = b.CheckReport(direct.CanonicalBytes()); err != nil {
		t.Fatal(err)
	}
	expected, err := pc.NewGroupingBundleExpectationPrivate(b)
	if err != nil {
		t.Fatal(err)
	}
	got, obs, failed := pc.ExecuteGroupingCaseBundlePrivate(context.Background(), expected, b.CanonicalBytes())
	if failed != nil || !bytes.Equal(got, direct.CanonicalBytes()) || !obs.Imported || !obs.Executed || !obs.Checked {
		t.Fatalf("ASSERT_TYPED_BUNDLE_DISPATCH_EXACT: %v %+v", failed, obs)
	}
}

func TestTypedBundleOpaqueDeclaration(t *testing.T) {
	req, _ := bundleRequest(t, 7, "H")
	req.ImplementationBytes = []byte("{\"opaque\":1,\"opaque\":2}\n")
	b, e := pc.NewGroupingCaseBundlePrivate(req)
	if e != nil {
		t.Fatal(e)
	}
	expected, e := pc.NewGroupingBundleExpectationPrivate(b)
	if e != nil {
		t.Fatal(e)
	}
	direct, e := b.ExecuteReport()
	if e != nil {
		t.Fatal(e)
	}
	got, _, failed := pc.ExecuteGroupingCaseBundlePrivate(nil, expected, b.CanonicalBytes())
	if failed != nil || !bytes.Equal(got, direct.CanonicalBytes()) {
		t.Fatalf("ASSERT_TYPED_REQUEST_POSITIVE_EXACT: %v", failed)
	}
}

func TestTypedBundleExecutionOrderedSchedule(t *testing.T) {
	req := pc.GroupingCaseBundleRequestPrivate{Schema: pc.GroupingCaseBundleSchemaPrivate, BatchID: "supervised-35-ordered", ImplementationID: "controlled-not-qualified", ImplementationBytes: []byte("exact LF declaration\n")}
	for mask := 1; mask < 8; mask++ {
		for _, probe := range []string{"H", "U", "L", "B", "N"} {
			r, _ := bundleRequest(t, mask, probe)
			req.Slots = append(req.Slots, r.Slots[0])
			// Keep the original ordered case/probe and add a second explicit seed,
			// rebuilding its frozen case and independent judgment binding.
			s := r.Slots[0]
			input, e := rg.ParseAndAdmit(s.InputBytes, s.Predecessors, rg.ParseLimits{MaxRawBytes: 1 << 20, MaxArtifacts: 16, MaxNodes: 8192, MaxOccurrences: 4096, MaxContributions: 4096, MaxCanonicalBytes: 1 << 20, MaxStringBytes: 4096})
			if e != nil {
				t.Fatal(e)
			}
			s.Seed = 19
			s.CaseID = fmt.Sprintf("%s-seed-19", s.CaseID)
			c, e := pc.NewGroupingEvaluationCasePrivate(pc.GroupingEvaluationCaseRequestPrivate{CaseID: s.CaseID, SelectorID: s.SelectorID, RunID: s.RunID, SelectorBytes: s.SelectorBytes, Input: input, Policy: evaluationPolicy(t, probe), ProjectionLimits: s.ProjectionLimits, Seed: s.Seed})
			if e != nil {
				t.Fatal(e)
			}
			j, e := pc.NewGroupingJudgmentsPrivate(c, s.JudgmentID, s.Pairs)
			if e != nil {
				t.Fatal(e)
			}
			s.CaseBytes, s.JudgmentBytes = c.CanonicalBytes(), j.CanonicalBytes()
			req.Slots = append(req.Slots, s)
		}
	}
	if _, e := pc.NewGroupingCaseBundlePrivate(req); e == nil {
		t.Fatal("ASSERT_COMBINED_70_SLOT_BOUND_REFUSED")
	}
	t.Log("ASSERT_COMBINED_70_SLOT_BOUND_REFUSED: PASS")
	secondary := req
	secondary.BatchID = "supervised-35-ordered-seed-19"
	secondary.Slots = nil
	original := req.Slots
	req.Slots = nil
	for i, slot := range original {
		if i%2 == 0 {
			req.Slots = append(req.Slots, slot)
		} else {
			secondary.Slots = append(secondary.Slots, slot)
		}
	}
	second, e := pc.NewGroupingCaseBundlePrivate(secondary)
	if e != nil {
		t.Fatal(e)
	}
	secondExpected, e := pc.NewGroupingBundleExpectationPrivate(second)
	if e != nil {
		t.Fatal(e)
	}
	secondDirect, e := second.ExecuteReport()
	if e != nil {
		t.Fatal(e)
	}
	if e = second.CheckReport(secondDirect.CanonicalBytes()); e != nil {
		t.Fatal(e)
	}
	secondReport, _, secondFailure := pc.ExecuteGroupingCaseBundlePrivate(context.Background(), secondExpected, second.CanonicalBytes())
	if secondFailure != nil || !bytes.Equal(secondReport, secondDirect.CanonicalBytes()) {
		t.Fatalf("ASSERT_TYPED_SECOND_SEED_EXACT: failure=%v", secondFailure)
	}
	t.Log("ASSERT_TYPED_SECOND_SEED_EXACT: PASS")
	b, err := pc.NewGroupingCaseBundlePrivate(req)
	if err != nil {
		t.Fatal(err)
	}
	expected, err := pc.NewGroupingBundleExpectationPrivate(b)
	if err != nil {
		t.Fatal(err)
	}
	direct, err := b.ExecuteReport()
	if err != nil {
		t.Fatal(err)
	}
	if err = b.CheckReport(direct.CanonicalBytes()); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	report, obs, failed := pc.ExecuteGroupingCaseBundlePrivate(ctx, expected, b.CanonicalBytes())
	if failed != nil || !bytes.Equal(report, direct.CanonicalBytes()) || !obs.Imported || !obs.Executed || !obs.Checked {
		t.Fatalf("ASSERT_TYPED_SUPERVISED_ORDERED_EXACT: failure=%v bytes=%d expected=%d observation=%+v", failed, len(report), len(direct.CanonicalBytes()), obs)
	}
	if err = b.CheckReport(report); err != nil {
		t.Fatal(err)
	}
	t.Log("ASSERT_TYPED_SUPERVISED_ORDERED_EXACT: PASS")
	for _, name := range []string{"foreign-expected", "corrupt", "truncated", "reordered"} {
		t.Run(name, func(t *testing.T) {
			raw := b.CanonicalBytes()
			exp := expected
			switch name {
			case "foreign-expected":
				r := req
				r.ImplementationBytes = []byte("independently foreign\n")
				other, e := pc.NewGroupingCaseBundlePrivate(r)
				if e != nil {
					t.Fatal(e)
				}
				exp, e = pc.NewGroupingBundleExpectationPrivate(other)
				if e != nil {
					t.Fatal(e)
				}
			case "corrupt":
				raw[len(raw)/2] ^= 1
			case "truncated":
				raw = raw[:len(raw)-1]
			case "reordered":
				r := req
				r.Slots = append([]pc.GroupingCaseBundleSlotPrivate(nil), req.Slots...)
				r.Slots[0], r.Slots[1] = r.Slots[1], r.Slots[0]
				other, e := pc.NewGroupingCaseBundlePrivate(r)
				if e != nil {
					t.Fatal(e)
				}
				raw = other.CanonicalBytes()
			}
			got, _, f := pc.ExecuteGroupingCaseBundlePrivate(context.Background(), exp, raw)
			if f == nil || len(got) != 0 {
				t.Fatalf("ASSERT_TYPED_FOREIGN_NO_ARTIFACT: %s failure=%v bytes=%d", name, f, len(got))
			}
			t.Log("ASSERT_TYPED_FOREIGN_NO_ARTIFACT: PASS")
		})
	}
}

func TestTypedBundleExecutionMissingExpectation(t *testing.T) {
	got, _, f := pc.ExecuteGroupingCaseBundlePrivate(nil, pc.GroupingBundleExpectationPrivate{}, []byte("{}\n"))
	if f == nil || got != nil {
		t.Fatal("ASSERT_TYPED_MISSING_EXPECTATION_NO_ARTIFACT")
	}
	if _, e := pc.NewGroupingBundleExpectationPrivate(pc.GroupingCaseBundlePrivate{}); e == nil {
		t.Fatal("empty constructed bundle accepted")
	}
}
