package programc_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	pc "lsp-trace/internal/programc"
	rg "lsp-trace/internal/referencegrouping"
	"runtime"
	"testing"
	"time"
)

// This guard compiles before the adapter exists: it exercises the existing real
// dispatch with a valid independently constructed bundle, not an absent symbol.
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
	t.Log("DIRECT_TYPED_BUNDLE_CHECKED")
	request, err := json.Marshal(struct {
		Version  string `json:"version"`
		Input    []byte `json:"input"`
		Seed     uint64 `json:"seed"`
		Expected any    `json:"bundle_expected,omitempty"`
	}{"lsp-trace.private-program-c-bundle-worker.v1", b.CanonicalBytes(), 0, b.Commitment()})
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	handled, code := pc.RunPrivateWorker([]string{"__lsp_trace_private_program_c_worker_v1"}, bytes.NewReader(request), &out, &bytes.Buffer{})
	var response struct {
		Version string `json:"version"`
		Report  []byte `json:"report"`
	}
	if err = json.Unmarshal(out.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if !handled || code != 0 || !bytes.Equal(response.Report, direct.CanonicalBytes()) {
		t.Fatalf("ASSERT_TYPED_BUNDLE_DISPATCH_EXACT: handled=%v code=%d report_bytes=%d expected_bytes=%d", handled, code, len(response.Report), len(direct.CanonicalBytes()))
	}
	t.Log("ASSERT_TYPED_BUNDLE_DISPATCH_EXACT: PASS")
}

func TestTypedBundleRequestDuplicateContract(t *testing.T) {
	req, _ := bundleRequest(t, 7, "H")
	// Opaque declaration bytes may themselves contain JSON-looking duplicates;
	// only the outer request objects are the contract under examination.
	req.ImplementationBytes = []byte("{\"opaque\":1,\"opaque\":2}\n")
	bundle, err := pc.NewGroupingCaseBundlePrivate(req)
	if err != nil {
		t.Fatal(err)
	}
	expected := bundle.Commitment() // independently held actual constructor result
	direct, err := bundle.ExecuteReport()
	if err != nil {
		t.Fatal(err)
	}
	if err = bundle.CheckReport(direct.CanonicalBytes()); err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(struct {
		Version  string `json:"version"`
		Input    []byte `json:"input"`
		Seed     uint64 `json:"seed"`
		Expected any    `json:"bundle_expected"`
	}{"lsp-trace.private-program-c-bundle-worker.v1", bundle.CanonicalBytes(), 0, expected})
	if err != nil {
		t.Fatal(err)
	}
	dispatch := func(candidate []byte) (*pc.SupervisionFailure, []byte) {
		t.Helper()
		var out bytes.Buffer
		handled, code := pc.RunPrivateWorker([]string{"__lsp_trace_private_program_c_worker_v1"}, bytes.NewReader(candidate), &out, &bytes.Buffer{})
		if !handled || code != 0 {
			t.Fatalf("dispatch protocol handled=%v code=%d", handled, code)
		}
		var response struct {
			Failure *pc.SupervisionFailure `json:"failure"`
			Report  []byte                 `json:"report"`
		}
		if e := json.Unmarshal(out.Bytes(), &response); e != nil {
			t.Fatal(e)
		}
		return response.Failure, response.Report
	}
	if failed, report := dispatch(raw); failed != nil || !bytes.Equal(report, direct.CanonicalBytes()) {
		t.Fatalf("ASSERT_TYPED_REQUEST_POSITIVE_EXACT: failure=%v bytes=%d", failed, len(report))
	}
	t.Log("ASSERT_TYPED_REQUEST_POSITIVE_EXACT: PASS")
	cases := []struct {
		name     string
		from, to []byte
	}{
		{"seed", []byte(`"seed":0`), []byte(`"seed":1,"seed":0`)},
		{"escaped-seed", []byte(`"seed":0`), []byte(`"se\u0065d":1,"seed":0`)},
		{"version", []byte(`"version":`), []byte(`"version":"unknown-before-typed","version":`)},
		{"expected-id", []byte(`"id":`), []byte(`"id":"wrong-before-correct","id":`)},
		{"expected-digest", []byte(`"digest":`), []byte(`"digest":"wrong-before-correct","digest":`)},
		{"expected-length", []byte(`"byte_length":`), []byte(`"byte_length":1,"byte_length":`)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if bytes.Count(raw, tc.from) != 1 {
				t.Fatalf("fixture replacement count for %s", tc.name)
			}
			candidate := bytes.Replace(raw, tc.from, tc.to, 1)
			failed, report := dispatch(candidate)
			if failed == nil || failed.Code != pc.CodeInternalAdmission || len(report) != 0 {
				t.Fatalf("ASSERT_TYPED_REQUEST_DUPLICATES: case=%s failure=%v report_bytes=%d", tc.name, failed, len(report))
			}
			t.Logf("ASSERT_TYPED_REQUEST_DUPLICATES: PASS case=%s", tc.name)
		})
	}
}

func TestTypedBundleSupervisedOrderedSchedule(t *testing.T) {
	if runtime.GOOS != "darwin" || runtime.GOARCH != "arm64" {
		t.Skip("controlled Darwin arm64 process qualification only")
	}
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
	secondReport, _, secondFailure := pc.ExecuteGroupingCaseBundleSupervisedPrivate(context.Background(), secondExpected, second.CanonicalBytes())
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
	report, obs, failed := pc.ExecuteGroupingCaseBundleSupervisedPrivate(ctx, expected, b.CanonicalBytes())
	if failed != nil || !bytes.Equal(report, direct.CanonicalBytes()) || obs.Samples < 1 {
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
			got, _, f := pc.ExecuteGroupingCaseBundleSupervisedPrivate(context.Background(), exp, raw)
			if f == nil || len(got) != 0 {
				t.Fatalf("ASSERT_TYPED_FOREIGN_NO_ARTIFACT: %s failure=%v bytes=%d", name, f, len(got))
			}
			t.Log("ASSERT_TYPED_FOREIGN_NO_ARTIFACT: PASS")
		})
	}
}

func TestTypedBundleSupervisedResponseAndCancellation(t *testing.T) {
	if runtime.GOOS != "darwin" || runtime.GOARCH != "arm64" {
		t.Skip("controlled Darwin arm64 process qualification only")
	}
	req, _ := bundleRequest(t, 7, "L")
	b, err := pc.NewGroupingCaseBundlePrivate(req)
	if err != nil {
		t.Fatal(err)
	}
	expected, err := pc.NewGroupingBundleExpectationPrivate(b)
	if err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"typed-response-truncated", "typed-response-duplicate", "typed-response-stderr", "typed-response-digest", "typed-response-foreign", "malformed"} {
		t.Run(mode, func(t *testing.T) {
			t.Setenv("PROGRAMC_TEST_WORKER", mode)
			report, _, failed := pc.ExecuteGroupingCaseBundleSupervisedPrivate(context.Background(), expected, b.CanonicalBytes())
			if failed == nil || len(report) != 0 {
				t.Fatalf("ASSERT_TYPED_RESPONSE_NO_ARTIFACT: mode=%s failure=%v bytes=%d", mode, failed, len(report))
			}
			t.Log("ASSERT_TYPED_RESPONSE_NO_ARTIFACT: PASS")
		})
	}
	t.Run("cancelled", func(t *testing.T) {
		t.Setenv("PROGRAMC_TEST_WORKER", "never-read")
		ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
		defer cancel()
		report, _, failed := pc.ExecuteGroupingCaseBundleSupervisedPrivate(ctx, expected, b.CanonicalBytes())
		if failed == nil || (failed.Code != pc.CodeCancelled && failed.Code != pc.CodeTimeout) || len(report) != 0 {
			t.Fatalf("ASSERT_TYPED_CANCEL_NO_ARTIFACT: failure=%v bytes=%d", failed, len(report))
		}
		t.Log("ASSERT_TYPED_CANCEL_NO_ARTIFACT: PASS")
	})
}

func TestTypedBundleUnsupportedBeforeEffects(t *testing.T) {
	if runtime.GOOS == "darwin" && runtime.GOARCH == "arm64" {
		t.Skip("compile-selected unsupported port")
	}
	req, _ := bundleRequest(t, 7, "H")
	b, err := pc.NewGroupingCaseBundlePrivate(req)
	if err != nil {
		t.Fatal(err)
	}
	expected, err := pc.NewGroupingBundleExpectationPrivate(b)
	if err != nil {
		t.Fatal(err)
	}
	report, _, failed := pc.ExecuteGroupingCaseBundleSupervisedPrivate(nil, expected, b.CanonicalBytes())
	if failed == nil || failed.Code != pc.CodePlatformUnsupported || len(report) != 0 {
		t.Fatalf("ASSERT_TYPED_UNSUPPORTED_PRE_EFFECTS: failure=%v", failed)
	}
	t.Log("ASSERT_TYPED_UNSUPPORTED_PRE_EFFECTS: PASS")
}

func TestTypedBundleSupervisedMissingExpectation(t *testing.T) {
	got, _, f := pc.ExecuteGroupingCaseBundleSupervisedPrivate(nil, pc.GroupingBundleExpectationPrivate{}, []byte("{}\n"))
	if f == nil || len(got) != 0 {
		t.Fatal("ASSERT_TYPED_MISSING_EXPECTATION_NO_ARTIFACT")
	}
	t.Log("ASSERT_TYPED_MISSING_EXPECTATION_NO_ARTIFACT: PASS")
}
