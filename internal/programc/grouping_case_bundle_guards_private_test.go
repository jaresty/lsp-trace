package programc_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	pc "lsp-trace/internal/programc"
	rg "lsp-trace/internal/referencegrouping"
	"strings"
	"testing"
)

func bundleExpectation(raw []byte) rg.CompositionCommitment {
	h := sha256.Sum256(raw)
	id := fmt.Sprintf("sha256:%x", h)
	return rg.CompositionCommitment{ID: id, Digest: id, ByteLength: len(raw)}
}
func cloneBundleRequest(t *testing.T, r pc.GroupingCaseBundleRequestPrivate) pc.GroupingCaseBundleRequestPrivate {
	t.Helper()
	var x pc.GroupingCaseBundleRequestPrivate
	if e := json.Unmarshal(bundleRaw(t, r), &x); e != nil {
		t.Fatal(e)
	}
	return x
}
func TestGroupingCaseBundlePrivateSeparateLayerUniverse(t *testing.T) {
	r, _ := bundleRequest(t, 6, "U")
	s := &r.Slots[0]
	input, e := rg.ParseAndAdmit(s.InputBytes, s.Predecessors, rg.ParseLimits{MaxRawBytes: 1 << 20, MaxArtifacts: 16, MaxNodes: 8192, MaxOccurrences: 4096, MaxContributions: 4096, MaxCanonicalBytes: 1 << 20, MaxStringBytes: 4096})
	if e != nil {
		t.Fatal(e)
	}
	rules := evaluationPolicy(t, "U").Rules()
	for i := range rules {
		if rules[i].RelationKind == "RESOLVES_TO_DEFINITION" {
			rules[i].Action = "SEPARATE_LAYER"
			rules[i].Layer = "CONTROLLED_DEFINITIONS"
		}
	}
	policy, e := rg.BuildGroupingPolicy(rg.GroupingPolicyRequest{PolicyID: "controlled-separate-layer", Epoch: 8, EligibleKinds: []string{"CALLS", "REFERENCES_SYMBOL", "RESOLVES_TO_DEFINITION"}, Rules: rules, Limits: rg.GroupingPolicyLimits{MaxRules: 3, MaxCanonicalBytes: 1 << 16, MaxStringBytes: 256, MaxAbsoluteNumerator: 1 << 20, MaxDenominator: 1 << 20, MaxCap: 128}})
	if e != nil {
		t.Fatal(e)
	}
	c, e := pc.NewGroupingEvaluationCasePrivate(pc.GroupingEvaluationCaseRequestPrivate{CaseID: s.CaseID, SelectorID: s.SelectorID, RunID: s.RunID, SelectorBytes: s.SelectorBytes, Seed: s.Seed, ProjectionLimits: s.ProjectionLimits, Input: input, Policy: policy})
	if e != nil {
		t.Fatal(e)
	}
	j, e := pc.NewGroupingJudgmentsPrivate(c, s.JudgmentID, s.Pairs)
	if e != nil {
		t.Fatal(e)
	}
	s.PolicyBytes = policy.CanonicalBytes()
	s.CaseBytes = c.CanonicalBytes()
	s.JudgmentBytes = j.CanonicalBytes()
	b, e := pc.NewGroupingCaseBundlePrivate(r)
	if e != nil {
		t.Fatal(e)
	}
	reopened, e := pc.ImportGroupingCaseBundlePrivate(b.Commitment(), b.CanonicalBytes())
	if e != nil {
		t.Fatal(e)
	}
	result, e := reopened.ExecuteReport()
	if e != nil {
		t.Fatal(e)
	}
	// Inactive definition-only endpoint remains in the contribution universe.
	if result.Document().Rows[0].Counts.UniversePairs != 3 {
		t.Fatal("ASSERT_SEPARATE_LAYER_ENDPOINT_UNIVERSE")
	}
	if e = reopened.CheckReport(result.CanonicalBytes()); e != nil {
		t.Fatal(e)
	}
	t.Log("ASSERT_SEPARATE_LAYER_ENDPOINT_UNIVERSE: PASS")
}
func TestGroupingCaseBundlePrivateFrozenCaseBinding(t *testing.T) {
	r, _ := bundleRequest(t, 6, "H")
	r.Slots[0].CaseBytes = []byte("nonempty wrong frozen case\n")
	if _, e := pc.NewGroupingCaseBundlePrivate(r); e == nil {
		t.Fatal("ASSERT_FROZEN_CASE_BINDING_REFUSED")
	}
	t.Log("ASSERT_FROZEN_CASE_BINDING_REFUSED: PASS")
}
func TestGroupingCaseBundlePrivateCanonicalAndBounds(t *testing.T) {
	r, _ := bundleRequest(t, 7, "N")
	valid := bundleRaw(t, r)
	malformed := [][]byte{
		nil, valid[:len(valid)-1], append(append([]byte(nil), valid...), []byte("{}")...), append([]byte(" "), valid...),
		bytes.Replace(valid, []byte(`"Schema":`), []byte(`"Unknown":0,"Schema":`), 1),
		bytes.Replace(valid, []byte(`"Schema":`), []byte(`"Schema":"duplicate","Schema":`), 1),
		bytes.Replace(valid, []byte(`"Schema":`), []byte(`"schema":`), 1),
		bytes.Replace(valid, []byte(`"Schema":`), []byte(`"\u0053chema":`), 1),
		[]byte(strings.Repeat("[", 17) + strings.Repeat("]", 17) + "\n"),
		[]byte(strings.Repeat("x", pc.GroupingCaseBundleMaxBytesPrivate+1)),
		[]byte("{\"Slots\":[" + strings.Repeat("null,", 128) + "null]}\n"),
		[]byte("{\"Predecessors\":[" + strings.Repeat("null,", 16) + "null]}\n"),
		[]byte("{\"Seed\":123456789012345678901}\n"),
		[]byte("{\"SelectorID\":\"" + strings.Repeat("x", (2<<20)+1) + "\"}\n"),
	}
	for i, raw := range malformed {
		if _, e := pc.ImportGroupingCaseBundlePrivate(bundleExpectation(raw), raw); e == nil {
			t.Fatalf("ASSERT_MATCHING_DIGEST_MALFORMED_REFUSED: %d", i)
		}
	}
	tests := []struct {
		name   string
		mutate func(*pc.GroupingCaseBundleRequestPrivate)
	}{
		{"version", func(x *pc.GroupingCaseBundleRequestPrivate) { x.Schema = "unsupported-v2" }},
		{"slots", func(x *pc.GroupingCaseBundleRequestPrivate) { x.Slots = make([]pc.GroupingCaseBundleSlotPrivate, 129) }},
		{"predecessors", func(x *pc.GroupingCaseBundleRequestPrivate) {
			x.Slots[0].Predecessors = make([]rg.PredecessorRecord, 17)
		}},
		{"judgments", func(x *pc.GroupingCaseBundleRequestPrivate) {
			x.Slots[0].Pairs = make([]pc.GroupingPairJudgmentPrivate, 129)
		}},
		{"string", func(x *pc.GroupingCaseBundleRequestPrivate) { x.Slots[0].RunID = strings.Repeat("x", 4097) }},
		{"numeric", func(x *pc.GroupingCaseBundleRequestPrivate) { x.Slots[0].ProjectionLimits.MaxAccumulator = 1<<50 + 1 }},
		{"aggregate", func(x *pc.GroupingCaseBundleRequestPrivate) {
			s := x.Slots[0]
			s.InputBytes = make([]byte, 1<<20)
			for i := 0; i < 8; i++ {
				x.Slots = append(x.Slots, s)
			}
		}},
		{"missing-predecessor", func(x *pc.GroupingCaseBundleRequestPrivate) { x.Slots[0].Predecessors = x.Slots[0].Predecessors[1:] }},
		{"extra-predecessor", func(x *pc.GroupingCaseBundleRequestPrivate) {
			p := x.Slots[0].Predecessors[0]
			p.Selector = "extra"
			x.Slots[0].Predecessors = append(x.Slots[0].Predecessors, p)
		}},
		{"duplicate-predecessor", func(x *pc.GroupingCaseBundleRequestPrivate) { x.Slots[0].Predecessors[1] = x.Slots[0].Predecessors[0] }},
		{"duplicate-slot", func(x *pc.GroupingCaseBundleRequestPrivate) { x.Slots = append(x.Slots, x.Slots[0]) }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			x := cloneBundleRequest(t, r)
			test.mutate(&x)
			if _, e := pc.NewGroupingCaseBundlePrivate(x); e == nil {
				t.Fatal("ASSERT_DECLARATION_BOUND_REFUSED")
			}
			raw := bundleRaw(t, x)
			if _, e := pc.ImportGroupingCaseBundlePrivate(bundleExpectation(raw), raw); e == nil {
				t.Fatal("ASSERT_SERIALIZED_BOUND_REFUSED")
			}
		})
	}
	t.Log("ASSERT_MATCHING_DIGEST_MALFORMED_REFUSED: PASS")
}
func TestGroupingCaseBundlePrivateResealedPredecessorActuallyReplayed(t *testing.T) {
	r, _ := bundleRequest(t, 7, "U")
	s := &r.Slots[0]
	// Preserve predecessor byte length and update the canonical input commitment.
	// Hash-only verification would now accept this corrupted evidence; replay must
	// still parse the supplied predecessor and refuse it. No fake Q/source is used.
	for i := range s.Predecessors {
		p := &s.Predecessors[i]
		if p.Role != "RETAINED_COMPOSITE" {
			continue
		}
		old := bundleExpectation(p.Bytes).Digest
		p.Bytes[0] = '!'
		newID := bundleExpectation(p.Bytes).Digest
		s.InputBytes = bytes.Replace(s.InputBytes, []byte(old), []byte(newID), 1)
	}
	if _, e := rg.ParseInput(s.InputBytes, rg.ParseLimits{MaxRawBytes: 1 << 20, MaxArtifacts: 16, MaxNodes: 8192, MaxOccurrences: 4096, MaxContributions: 4096, MaxCanonicalBytes: 1 << 20, MaxStringBytes: 4096}); e != nil {
		t.Fatal("BLOCKED_NOT_REPLAY: canonical input prerequisite", e)
	}
	if _, e := pc.NewGroupingCaseBundlePrivate(r); e == nil || e.Error() != "invalid retained composite" {
		t.Fatalf("ASSERT_RESEALED_PREDECESSOR_REPLAY_REFUSED: %v", e)
	}
	t.Log("ASSERT_RESEALED_PREDECESSOR_REPLAY_REFUSED: PASS")
}
func TestGroupingCaseBundlePrivateScheduleReportAndDetachedBytes(t *testing.T) {
	r, _ := bundleRequest(t, 7, "L")
	second, _ := bundleRequest(t, 6, "H")
	r.Slots = append(r.Slots, second.Slots[0])
	b, e := pc.NewGroupingCaseBundlePrivate(r)
	if e != nil {
		t.Fatal(e)
	}
	expected := b.Commitment()
	original := b.CanonicalBytes()
	for _, name := range []string{"removed", "reordered", "seed", "implementation"} {
		x := cloneBundleRequest(t, r)
		switch name {
		case "removed":
			x.Slots = x.Slots[:1]
		case "reordered":
			x.Slots[0], x.Slots[1] = x.Slots[1], x.Slots[0]
		case "seed":
			x.Slots[0].Seed++
		case "implementation":
			x.ImplementationID = "foreign"
		}
		raw := bundleRaw(t, x)
		if _, e := pc.ImportGroupingCaseBundlePrivate(expected, raw); e == nil {
			t.Fatal("ASSERT_CHANGED_EXPECTED_SCHEDULE_REFUSED", name)
		}
	}
	reopened, e := pc.ImportGroupingCaseBundlePrivate(expected, original)
	if e != nil {
		t.Fatal(e)
	}
	report, e := reopened.ExecuteReport()
	if e != nil {
		t.Fatal(e)
	}
	if e = reopened.CheckReport(report.CanonicalBytes()); e != nil {
		t.Fatal(e)
	}
	for _, kind := range []string{"missing", "reorder", "result", "counters"} {
		d := report.Document()
		switch kind {
		case "missing":
			d.Rows = d.Rows[:1]
		case "reorder":
			d.Rows[0], d.Rows[1] = d.Rows[1], d.Rows[0]
		case "result":
			d.Rows[0].ResultBytes[0] = '!'
		case "counters":
			d.Rows[0].Counts.TruePositive++
		}
		raw, e := json.Marshal(d)
		if e != nil {
			t.Fatal(e)
		}
		if e = reopened.CheckReport(append(raw, '\n')); e == nil {
			t.Fatal("ASSERT_REPORT_EXACT_REPLAY_REFUSED", kind)
		}
	}
	r.Slots[0].InputBytes[0] = '!'
	r.Slots[0].Predecessors[0].Bytes[0] = '!'
	r.ImplementationBytes[0] = '!'
	copyBytes := b.CanonicalBytes()
	copyBytes[0] = '!'
	original[0] = '!'
	if !bytes.Equal(b.CanonicalBytes(), reopened.CanonicalBytes()) || b.Commitment() != expected {
		t.Fatal("ASSERT_DETACHED_BUNDLE_BYTES")
	}
	if e = reopened.CheckReport(report.CanonicalBytes()); e != nil {
		t.Fatal(e)
	}
	t.Log("ASSERT_REPORT_EXACT_REPLAY_REFUSED: PASS")
}
