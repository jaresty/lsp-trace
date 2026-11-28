package programc

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"testing"

	"lsp-trace/internal/graph"
	"lsp-trace/internal/programcadmission"
	"lsp-trace/internal/programccompose"
	rg "lsp-trace/internal/referencegrouping"
)

func bundleReportDigestPrivate(raw []byte) string {
	sum := sha256.Sum256(raw)
	return "sha256:" + hex.EncodeToString(sum[:])
}

// Test-only bridge, absent from ordinary imports. Cancellation is requested
// inside the selected synchronous stage, after its preceding context check.
// The real stage still completes; this is not an interruption or runtime bound.
func TypedExecutionCancellationForTest(parent context.Context, replay bool) context.Context {
	ctx, cancel := context.WithCancel(parent)
	stage := bundleExecutionStartedPrivate
	if replay {
		stage = bundleReplayStartedPrivate
	}
	return context.WithValue(ctx, bundleExecutionObserverKeyPrivate{}, func(s bundleExecutionStagePrivate) {
		if s == stage {
			cancel()
		}
	})
}

func typedStageBundlePrivateTest(t *testing.T) GroupingCaseBundlePrivate {
	t.Helper()
	a, b := node("stage-a", 0), node("stage-b", 1)
	raw := validV5(t, []graph.Node{a, b}, []graph.Edge{{CallerNodeID: a.ID, CalleeNodeID: b.ID, CallSites: []graph.Range{{End: graph.Position{Character: 1}}}}})
	empty := validV5(t, nil, nil)
	metadata := programccompose.ExactMetadata{WorkspaceIdentity: "controlled-stage-workspace", RevisionCustody: "CALLER_ASSERTED", PositionEncoding: "utf-16", AcquisitionSemantics: "managed-lsp-v1", PrivacyPolicy: "controlled-private"}
	composite, e := programccompose.Compose([]programccompose.Input{{Bytes: raw, Identity: "controlled-stage-calls", SHA256: bundleReportDigestPrivate(raw), ByteLength: len(raw), ExactMetadata: metadata}, {Bytes: empty, Identity: "controlled-stage-empty", SHA256: bundleReportDigestPrivate(empty), ByteLength: len(empty), ExactMetadata: metadata}})
	if e != nil {
		t.Fatal(e)
	}
	admission, e := programcadmission.Admit(composite.Bytes)
	if e != nil {
		t.Fatal(e)
	}
	typed, e := programcadmission.PrepareTypedInput(programcadmission.TypedInputRequest{Calls: admission.Admission, Kinds: []programcadmission.RelationKind{programcadmission.TypedCalls}})
	if e != nil {
		t.Fatal(e)
	}
	projection, e := programcadmission.PreparePrivateLeidenProjection(typed)
	if e != nil {
		t.Fatal(e)
	}
	marshal := func(v any) []byte {
		r, e := json.Marshal(v)
		if e != nil {
			t.Fatal(e)
		}
		return append(r, '\n')
	}
	type occurrence struct {
		Identity   string `json:"identity"`
		Kind       string `json:"kind"`
		From       string `json:"from"`
		To         string `json:"to"`
		Ordinal    int    `json:"ordinal"`
		RelationID string `json:"relation_id"`
	}
	rows := []occurrence{}
	outcomes := []rg.CandidateOutcome{}
	for _, o := range typed.Occurrences {
		rows = append(rows, occurrence{o.Identity, string(o.Kind), o.From, o.To, o.Ordinal, o.RelationID})
		outcomes = append(outcomes, rg.CandidateOutcome{OccurrenceIdentity: o.Identity, Disposition: "ADMITTED"})
	}
	ledger := marshal(struct {
		Domain      string       `json:"domain"`
		Version     string       `json:"version"`
		Claim       string       `json:"claim"`
		Occurrences []occurrence `json:"occurrences"`
	}{"PROGRAM_C_TYPED_OCCURRENCES", "v1", "ADMITTED_TYPED_OCCURRENCES_EXACT", rows})
	preds := []rg.PredecessorRecord{{Role: "OCCURRENCE_LEDGER", Selector: "ledgers/occurrences.json", Schema: "lsp-trace.private.program-c-occurrence-ledger.v1", Bytes: ledger}, {Role: "ACQUISITION_RECEIPT", Selector: "calls/acquisition.json", Schema: "lsp-trace.private.program-c-acquisition-receipt.v1", Bytes: marshal(projection.CompositeSourceBinding())}, {Role: "RETAINED_COMPOSITE", Selector: "calls/composite.json", Schema: "lsp-trace.private.program-c-composite.v1", Bytes: projection.RetainedCallsCanonicalBytes()}}
	artifact, e := rg.BuildInput(rg.InputRequest{TypedInput: typed, Predecessors: preds, OccurrenceLedgerBytes: ledger, EndpointCustody: []rg.EndpointCustody{{Endpoint: a.ID, PredecessorSelector: "calls/acquisition.json", Custody: "CALLER_ASSERTED"}, {Endpoint: b.ID, PredecessorSelector: "calls/acquisition.json", Custody: "CALLER_ASSERTED"}}, OccurrenceAdmissionPolicyID: "controlled-occurrence-v1", CompositionPolicyID: "controlled-identity-v1", PrivacyPolicy: "controlled-private", EligibleKinds: []programcadmission.RelationKind{programcadmission.TypedCalls, programcadmission.TypedReferencesSymbol, programcadmission.TypedResolvesToDefinition}, Limits: rg.ResourceLimits{MaxArtifacts: 16, MaxNodes: 32, MaxOccurrences: 32, MaxContributions: 32, MaxCanonicalBytes: 1 << 20, MaxStringBytes: 4096}, CandidateOutcomes: outcomes, Accounting: rg.Accounting{Candidates: len(rows), Included: len(rows), Terminal: true}})
	if e != nil {
		t.Fatal(e)
	}
	input, e := rg.AdmitInput(artifact, preds)
	if e != nil {
		t.Fatal(e)
	}
	policy, e := rg.BuildGroupingPolicy(rg.GroupingPolicyRequest{PolicyID: "CONTROLLED_STAGE", Epoch: 8, EligibleKinds: []string{"CALLS", "REFERENCES_SYMBOL", "RESOLVES_TO_DEFINITION"}, Rules: []rg.GroupingPolicyRule{{RelationKind: "CALLS", Action: "INCLUDE", RationaleCode: "CONTROLLED_STAGE", Weight: rg.Rational{Numerator: 1, Denominator: 1}}, {RelationKind: "REFERENCES_SYMBOL", Action: "EXCLUDE", RationaleCode: "CONTROLLED_STAGE", Weight: rg.Rational{Numerator: 0, Denominator: 1}}, {RelationKind: "RESOLVES_TO_DEFINITION", Action: "EXCLUDE", RationaleCode: "CONTROLLED_STAGE", Weight: rg.Rational{Numerator: 0, Denominator: 1}}}, Limits: rg.GroupingPolicyLimits{MaxRules: 3, MaxCanonicalBytes: 1 << 16, MaxStringBytes: 256, MaxAbsoluteNumerator: 1 << 20, MaxDenominator: 1 << 20, MaxCap: 128}})
	if e != nil {
		t.Fatal(e)
	}
	req := GroupingCaseBundleRequestPrivate{Schema: GroupingCaseBundleSchemaPrivate, BatchID: "controlled-stage-two", ImplementationID: "controlled-stage-not-qualified", ImplementationBytes: []byte("offline declaration\n")}
	for i, seed := range []uint64{7, 19} {
		c, e := NewGroupingEvaluationCasePrivate(GroupingEvaluationCaseRequestPrivate{CaseID: fmt.Sprintf("controlled-stage-%d", i), SelectorID: "controlled-selector", RunID: "controlled-run", SelectorBytes: []byte("not an executed query\n"), Input: input, Policy: policy, ProjectionLimits: rg.CompositionLimits{MaxRawBytes: 1 << 20, MaxOccurrences: 128, MaxPairContributions: 128, MaxPairs: 128, MaxCanonicalBytes: 1 << 20, MaxStringBytes: 4096, MaxDenominator: 1 << 20, MaxAccumulator: 1 << 20}, Seed: seed})
		if e != nil {
			t.Fatal(e)
		}
		nodes := c.Projection().NodeIdentities()
		pairs := []GroupingPairJudgmentPrivate{{Left: nodes[0], Right: nodes[1], Label: "SAME"}}
		j, e := NewGroupingJudgmentsPrivate(c, "independent-controlled", pairs)
		if e != nil {
			t.Fatal(e)
		}
		req.Slots = append(req.Slots, GroupingCaseBundleSlotPrivate{InputBytes: input.CanonicalBytes(), PolicyBytes: policy.CanonicalBytes(), Predecessors: preds, CaseID: fmt.Sprintf("controlled-stage-%d", i), SelectorID: "controlled-selector", RunID: "controlled-run", SelectorBytes: []byte("not an executed query\n"), ProjectionLimits: rg.CompositionLimits{MaxRawBytes: 1 << 20, MaxOccurrences: 128, MaxPairContributions: 128, MaxPairs: 128, MaxCanonicalBytes: 1 << 20, MaxStringBytes: 4096, MaxDenominator: 1 << 20, MaxAccumulator: 1 << 20}, Seed: seed, JudgmentID: "independent-controlled", Pairs: pairs, CaseBytes: c.CanonicalBytes(), JudgmentBytes: j.CanonicalBytes()})
	}
	bundle, e := NewGroupingCaseBundlePrivate(req)
	if e != nil {
		t.Fatal(e)
	}
	return bundle
}

func TestTypedBundleObserverNoninterference(t *testing.T) {
	bundle := typedStageBundlePrivateTest(t)
	baseline, e := bundle.ExecuteReport()
	if e != nil {
		t.Fatal(e)
	}
	old := groupingMeasurementObserverPrivate
	t.Cleanup(func() { groupingMeasurementObserverPrivate = old })
	for _, mode := range []string{"nil", "record", "panic"} {
		t.Run(mode, func(t *testing.T) {
			calls := 0
			groupingMeasurementObserverPrivate = nil
			if mode != "nil" {
				groupingMeasurementObserverPrivate = func(groupingMeasurementStagePrivate, int) {
					calls++
					if mode == "panic" {
						panic("controlled notification panic")
					}
				}
			}
			got, e := bundle.ExecuteReport()
			if e != nil || !bytes.Equal(got.CanonicalBytes(), baseline.CanonicalBytes()) {
				t.Fatalf("ASSERT_TYPED_OBSERVER_NONINTERFERENCE: mode=%s err=%v", mode, e)
			}
			if e = bundle.CheckReport(got.CanonicalBytes()); e != nil {
				t.Fatal(e)
			}
			if mode != "nil" && calls != 5 {
				t.Fatalf("ASSERT_TYPED_OBSERVER_NONINTERFERENCE: mode=%s calls=%d", mode, calls)
			}
			t.Logf("ASSERT_TYPED_OBSERVER_NONINTERFERENCE: PASS mode=%s notifications=%d", mode, calls)
			calls = 0
			expected, err := NewGroupingBundleExpectationPrivate(bundle)
			if err != nil {
				t.Fatal(err)
			}
			report, observed, failed := ExecuteGroupingCaseBundlePrivate(context.Background(), expected, bundle.CanonicalBytes())
			if failed != nil || !bytes.Equal(report, baseline.CanonicalBytes()) || !observed.Checked || (mode != "nil" && calls != 5) {
				t.Fatalf("ASSERT_EXECUTION_FULL_REPLAY: mode=%s failure=%v calls=%d observation=%+v", mode, failed, calls, observed)
			}
		})
	}
}
