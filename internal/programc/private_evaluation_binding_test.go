package programc_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	cc "lsp-trace/internal/censuscontinuation"
	"lsp-trace/internal/describerequest"
	"lsp-trace/internal/describeworker"
	pc "lsp-trace/internal/programc"
	"lsp-trace/internal/programcadmission"
	rg "lsp-trace/internal/referencegrouping"
	"lsp-trace/internal/retainedprojection"
	"lsp-trace/internal/sourceprojection"
	"lsp-trace/internal/v5sourcesnapshotv3"
)

// Genuine opaque admission/evaluation fixtures constructed BEFORE Run. These
// are controlled serialized observations, not authenticated provider execution.
func allModeFixture(t *testing.T) ([]cc.PrivateAllModeSlot, []cc.PrivateAllModeEntry) {
	t.Helper()
	modes := []string{"RD", "C", "CRD", "D", "CD", "R", "CR"}
	var slots []cc.PrivateAllModeSlot
	var entries []cc.PrivateAllModeEntry
	for i, mode := range modes {
		var kinds []programcadmission.RelationKind
		var names []string
		for _, pair := range []struct{ letter, kind string }{{"C", "CALLS"}, {"R", "REFERENCES_SYMBOL"}, {"D", "RESOLVES_TO_DEFINITION"}} {
			if strings.Contains(mode, pair.letter) {
				kinds = append(kinds, programcadmission.RelationKind(pair.kind))
				names = append(names, pair.kind)
			}
		}
		input, _ := bundleFixtureInput(t, kinds, true)
		// Admission's eligible universe is fixed to all three kinds; the exact
		// frozen three-rule policy, not nonzero edges, declares this slot's mode.
		var rules []rg.GroupingPolicyRule
		for _, kind := range []string{"CALLS", "REFERENCES_SYMBOL", "RESOLVES_TO_DEFINITION"} {
			r := rg.GroupingPolicyRule{RelationKind: kind, Action: "EXCLUDE", RationaleCode: "CONTROLLED_MODE", Weight: rg.Rational{Numerator: 0, Denominator: 1}}
			for _, selected := range names {
				if kind == selected {
					r.Action = "INCLUDE"
					r.Weight.Numerator = 1
				}
			}
			rules = append(rules, r)
		}
		policy, err := rg.BuildGroupingPolicy(rg.GroupingPolicyRequest{PolicyID: "controlled-" + mode, Epoch: 8, EligibleKinds: []string{"CALLS", "REFERENCES_SYMBOL", "RESOLVES_TO_DEFINITION"}, Rules: rules, Limits: rg.GroupingPolicyLimits{MaxRules: 3, MaxCanonicalBytes: 1 << 16, MaxStringBytes: 256, MaxAbsoluteNumerator: 1, MaxDenominator: 1, MaxCap: 1}})
		if err != nil {
			t.Fatal(err)
		}
		slot := cc.PrivateAllModeSlot{Ordinal: i, Mode: mode, RunID: fmt.Sprintf("controlled-run-%d", i), SeedNamespace: "original-declared-schedule", Seed: 7} // repeated seeds intentional
		c, err := pc.NewGroupingEvaluationCasePrivate(pc.GroupingEvaluationCaseRequestPrivate{CaseID: fmt.Sprintf("controlled-case-%d", i), RunID: slot.RunID, SelectorID: "controlled-selection", SelectorBytes: []byte("frozen selection\n"), Seed: slot.Seed, Input: input, Policy: policy, ProjectionLimits: evaluationLimits()})
		if err != nil {
			t.Fatal(err)
		}
		result, err := pc.EvaluateGroupingCasePrivate(c)
		if err != nil {
			t.Fatal(err)
		}
		slots = append(slots, slot)
		entries = append(entries, cc.PrivateAllModeEntry{Slot: slot, Disposition: cc.PrivateAllModeReady, Lineage: fmt.Sprintf("controlled-predecessor-%d", i), Input: input, Policy: policy, Case: c, Result: result})
	}
	return slots, entries
}

func TestPrivateEvaluationOpaquePairBinding(t *testing.T) {
	_, entries := allModeFixture(t)
	for _, e := range entries {
		b, err := pc.BindGroupingPairPrivate(e.Case, e.Result, e.Input)
		if err != nil || b.Case.ID != e.Case.ID() || b.Result.ID != e.Result.ID() || b.Bindings.Input.ID != e.Input.ID() || b.Seed != e.Slot.Seed || b.RunID != e.Slot.RunID {
			t.Fatal("ASSERT_ALLMODE_OPAQUE_PAIR_EXACT", b, err)
		}
	}
	e := entries[0]
	for _, tc := range []struct {
		c     pc.GroupingEvaluationCasePrivate
		r     pc.GroupingEvaluationResultPrivate
		input rg.AdmittedInput
	}{{pc.GroupingEvaluationCasePrivate{}, e.Result, e.Input}, {e.Case, pc.GroupingEvaluationResultPrivate{}, e.Input}, {e.Case, e.Result, rg.AdmittedInput{}}, {e.Case, entries[1].Result, e.Input}, {e.Case, e.Result, entries[1].Input}, {entries[1].Case, e.Result, e.Input}} {
		if _, err := pc.BindGroupingPairPrivate(tc.c, tc.r, tc.input); err == nil {
			t.Fatal("ASSERT_ALLMODE_FOREIGN_PAIR_REFUSED")
		}
	}
	b, err := pc.BindGroupingPairPrivate(e.Case, e.Result, e.Input)
	if err != nil {
		t.Fatal(err)
	}
	foreign, err := pc.NewGroupingEvaluationCasePrivate(pc.GroupingEvaluationCaseRequestPrivate{CaseID: b.CaseID, RunID: b.RunID, SelectorID: b.SelectorID, SelectorBytes: []byte("frozen selection\n"), Seed: b.Seed, Input: e.Input, Policy: evaluationPolicy(t, "L"), ProjectionLimits: evaluationLimits()})
	if err != nil {
		t.Fatal(err)
	}
	if foreign.Projection().ID() == e.Case.Projection().ID() {
		t.Fatal("CONTROL_FOREIGN_PROJECTION_NOT_DISTINCT")
	}
	if _, err = pc.BindGroupingPairPrivate(foreign, e.Result, e.Input); err == nil {
		t.Fatal("ASSERT_ALLMODE_FOREIGN_PROJECTION_REFUSED")
	}
	foreignResult, err := pc.EvaluateGroupingCasePrivate(foreign)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = pc.BindGroupingPairPrivate(foreign, foreignResult, e.Input); err != nil {
		t.Fatal("CONTROL_LEGITIMATE_FOREIGN_PAIR_REJECTED", err)
	}
	t.Log("ASSERT_ALLMODE_FOREIGN_PROJECTION_REFUSED: PASS")
	t.Log("ASSERT_ALLMODE_OPAQUE_PAIR_EXACT: PASS")
	t.Log("ASSERT_ALLMODE_FOREIGN_PAIR_REFUSED: PASS")
}

type allModeStore struct {
	*cc.MemoryStore
	puts    int
	fail    bool
	corrupt string
}

func (s *allModeStore) MaxObjectBytes() int64 { return 8 << 20 }
func (s *allModeStore) Put(ctx context.Context, b []byte) (string, error) {
	s.puts++
	if s.fail {
		return "", errors.New("controlled-store-failure")
	}
	return s.MemoryStore.Put(ctx, b)
}
func (s *allModeStore) Get(ctx context.Context, id string) ([]byte, error) {
	b, err := s.MemoryStore.Get(ctx, id)
	if id == s.corrupt && len(b) > 0 {
		b[0] ^= 1
	}
	return b, err
}

type allModeWorker struct{ calls int }

func (w *allModeWorker) Run(context.Context, describerequest.Record, string) (describeworker.RunResult, error) {
	w.calls++
	return describeworker.RunResult{}, errors.New("worker must not run")
}

func allModeContract(t *testing.T) cc.ContinuationContract {
	t.Helper()
	d := "sha256:" + strings.Repeat("1", 64)
	c, err := cc.NewContinuationContract(cc.ContractInput{ContinuationID: d, ProfileID: d, WorkerPinID: d, ProgramCSeed: 9, CaptureLimits: v5sourcesnapshotv3.Limits{MaxArtifactBytes: 8 << 20, MaxParentBytes: 8 << 20, MaxReceipts: 100, MaxSourceBytes: 1 << 20, MaxTotalSourceBytes: 8 << 20, MaxBindings: 1000, MaxWork: 10000}, PacketSourcePolicy: sourceprojection.Policy{PolicyID: "controlled", BodyRequested: true, MaxBytes: 1 << 20, MaxRanges: 100, MaxObjects: 100, MaxWork: 1000, EnforceLimits: true}, PacketResolveLimits: retainedprojection.ResolveLimits{MaxDistinctObjects: 100, MaxUniqueSourceBytes: 8 << 20, MaxLogicalSelections: 100}, PacketMaxResponseBytes: 1 << 20, DescribeDeadlineMS: 1000, RetryPolicy: cc.RetryPolicy{MaxAttemptsPerRequest: 1}})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestPrivateAllModeConnectedRunResume(t *testing.T) {
	slots, entries := allModeFixture(t)
	h, err := cc.NewPrivateAllModeHandoff("controlled-workspace", "controlled-revision", slots, entries)
	if err != nil {
		t.Fatal(err)
	}
	store := &allModeStore{MemoryStore: cc.NewMemoryStore()}
	worker := &allModeWorker{}
	contract := allModeContract(t)
	req := cc.Request{PrivateAllMode: &h, Workspace: "controlled-workspace", Contract: contract, Store: store, Worker: worker, StopAfter: cc.StopAfterSourceIntents}
	got := cc.Run(context.Background(), req)
	if got.Err != nil || got.Status != cc.StatusPaused || got.PrivateAllMode == nil {
		t.Fatal("ASSERT_ALLMODE_CONNECTED_PAUSE", got.Err)
	}
	out := got.PrivateAllMode
	if out.Counts.Slots != 7 || out.Counts.Ready != 7 || out.Counts.Missing != 0 || out.Counts.Intents == 0 || out.Counts.Nominations == 0 || worker.calls != 0 || got.RequestCount != 0 || store.puts != 4 {
		t.Fatal("ASSERT_ALLMODE_NO_DESCRIBE_AND_COUNTS", out.Counts, worker.calls, store.puts)
	}
	t.Logf("ALLMODE_CONNECTED_COUNTS slots=%d ready=%d missing=%d empty=%d excluded=%d nominations=%d intents=%d worker_calls=%d describe_requests=%d", out.Counts.Slots, out.Counts.Ready, out.Counts.Missing, out.Counts.Empty, out.Counts.Excluded, out.Counts.Nominations, out.Counts.Intents, worker.calls, got.RequestCount)
	perSlot := make([]int, 7)
	for _, intent := range out.Intents {
		if intent.Slot < 0 || intent.Slot >= 7 {
			t.Fatal("ASSERT_ALLMODE_ORDER_BINDING")
		}
		e := entries[intent.Slot]
		if intent.Mode != slots[intent.Slot].Mode || intent.Seed != 7 || intent.CaseID != e.Case.ID() || intent.InputID != e.Input.ID() || intent.ResultID != e.Result.ID() || intent.BindingState != "UNRESOLVED" || !strings.Contains(intent.ClaimCeiling, "NO_DESCRIBE") || intent.Endpoint.Predecessor == "" || intent.Endpoint.Custody != "CALLER_ASSERTED" {
			t.Fatal("ASSERT_ALLMODE_TYPED_JOINS", intent)
		}
		matched := false
		for _, c := range e.Input.Contributions() {
			if c.OccurrenceIdentity != intent.Endpoint.Occurrence {
				continue
			}
			if c.Kind != intent.Endpoint.Kind || c.Ordinal != intent.Endpoint.Ordinal {
				t.Fatal("ASSERT_ALLMODE_TYPED_ENDPOINT_COORDINATES", intent.Endpoint, c)
			}
			for _, u := range e.Input.EndpointCustody() {
				if u.OccurrenceIdentity == c.OccurrenceIdentity && u.EndpointRole == intent.Endpoint.Role && u.Endpoint == intent.Endpoint.Node && u.PredecessorSelector == intent.Endpoint.Predecessor && u.Custody == intent.Endpoint.Custody {
					matched = true
				}
			}
		}
		if !matched {
			t.Fatal("ASSERT_ALLMODE_TYPED_ENDPOINT_COORDINATES", intent.Endpoint)
		}
		perSlot[intent.Slot]++
	}
	for i, n := range perSlot {
		if n != 2*len(entries[i].Input.Contributions()) {
			t.Fatal("ASSERT_ALLMODE_MULTIPLICITY", i, n)
		}
	}
	resume := cc.ResumeRequest{PrivateAllMode: &cc.PrivateAllModeResume{Version: cc.PrivateAllModeVersion, Expected: h, Contract: contract}, Selector: got.CheckpointID, Workspace: req.Workspace, Store: store, Worker: worker, StopAfter: cc.StopAfterSourceIntents}
	again := cc.Resume(context.Background(), resume)
	if again.Err != nil || !reflect.DeepEqual(got.PrivateAllMode, again.PrivateAllMode) || again.CheckpointID != got.CheckpointID || store.puts != 4 || worker.calls != 0 {
		t.Fatal("ASSERT_ALLMODE_DURABLE_RECONCILIATION", again.Err)
	}
	for _, stop := range []cc.StopAfter{"", cc.StopAfterDescribeRequests} {
		r := resume
		r.StopAfter = stop
		if cc.Resume(context.Background(), r).Err == nil {
			t.Fatal("ASSERT_ALLMODE_NO_DESCRIBE_RESUME")
		}
	}
	r := resume
	binding := *r.PrivateAllMode
	binding.Version = "foreign"
	r.PrivateAllMode = &binding
	if cc.Resume(context.Background(), r).Err == nil {
		t.Fatal("ASSERT_ALLMODE_VERSION_REFUSED")
	}
	foreignHead, err := cc.NewPrivateAllModeHandoff(req.Workspace, "foreign-revision", slots, entries)
	if err != nil {
		t.Fatal(err)
	}
	foreignResume := resume
	foreignBinding := *resume.PrivateAllMode
	foreignBinding.Expected = foreignHead
	foreignResume.PrivateAllMode = &foreignBinding
	if cc.Resume(context.Background(), foreignResume).Err == nil {
		t.Fatal("ASSERT_ALLMODE_EXPECTED_REVISION_REFUSED")
	}
	t.Log("ASSERT_ALLMODE_EXPECTED_REVISION_REFUSED: PASS")
	store.corrupt = got.CheckpointID
	if cc.Resume(context.Background(), resume).Err == nil {
		t.Fatal("ASSERT_ALLMODE_PERSISTED_TAMPER_REFUSED")
	}
	store.corrupt = ""
	// Every durable artifact, not merely the outer checkpoint, is reconciled.
	checkpointRaw, readErr := store.Get(context.Background(), got.CheckpointID)
	if readErr != nil {
		t.Fatal(readErr)
	}
	var cp struct{ Artifacts []cc.ArtifactRef }
	if err := json.Unmarshal(checkpointRaw, &cp); err != nil {
		t.Fatal(err)
	}
	if len(cp.Artifacts) != 3 {
		t.Fatal("ASSERT_ALLMODE_PERSISTED_REFERENCE_MEMBERSHIP")
	}
	for _, ref := range cp.Artifacts {
		store.corrupt = ref.ID
		if cc.Resume(context.Background(), resume).Err == nil {
			t.Fatal("ASSERT_ALLMODE_EVERY_ARTIFACT_TAMPER_REFUSED", ref.Kind)
		}
	}
	store.corrupt = ""
	t.Log("ASSERT_ALLMODE_EVERY_ARTIFACT_TAMPER_REFUSED: PASS")
	t.Log("ASSERT_ALLMODE_TYPED_ENDPOINT_COORDINATES: PASS")
	// Caller resealing a checkpoint under a new content hash is not admission.
	raw, err := store.Get(context.Background(), got.CheckpointID)
	if err != nil {
		t.Fatal(err)
	}
	raw = bytes.Replace(raw, []byte("controlled-revision"), []byte("foreign-revision"), 1)
	selector, err := store.Put(context.Background(), raw)
	if err != nil {
		t.Fatal(err)
	}
	r = resume
	r.Selector = selector
	if cc.Resume(context.Background(), r).Err == nil {
		t.Fatal("ASSERT_ALLMODE_RESEALED_REFUSED")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	before := store.puts
	if cc.Run(ctx, req).Err == nil || store.puts != before {
		t.Fatal("ASSERT_ALLMODE_CANCEL_BEFORE_PUT")
	}
	store.fail = true
	if cc.Run(context.Background(), req).Err == nil || worker.calls != 0 {
		t.Fatal("ASSERT_ALLMODE_STORE_FAILURE_CLOSED")
	}
	for _, label := range []string{"ASSERT_ALLMODE_CONNECTED_PAUSE", "ASSERT_ALLMODE_NO_DESCRIBE_AND_COUNTS", "ASSERT_ALLMODE_TYPED_JOINS", "ASSERT_ALLMODE_MULTIPLICITY", "ASSERT_ALLMODE_DURABLE_RECONCILIATION", "ASSERT_ALLMODE_NO_DESCRIBE_RESUME", "ASSERT_ALLMODE_VERSION_REFUSED", "ASSERT_ALLMODE_PERSISTED_TAMPER_REFUSED", "ASSERT_ALLMODE_RESEALED_REFUSED", "ASSERT_ALLMODE_CANCEL_BEFORE_PUT", "ASSERT_ALLMODE_STORE_FAILURE_CLOSED"} {
		t.Log(label + ": PASS")
	}
}

func TestPrivateAllModeEmptyExcludedMissingDistinct(t *testing.T) {
	slots, entries := allModeFixture(t)
	// RD's admitted occurrences remain in a separate layer, making the active
	// base projection empty without claiming an observed-empty source universe.
	rules := entries[0].Policy.Rules()
	for i := range rules {
		if rules[i].Action != "EXCLUDE" {
			rules[i].Action = "SEPARATE_LAYER"
			rules[i].Layer = "CONTROLLED_LAYER"
		}
	}
	policy, err := rg.BuildGroupingPolicy(rg.GroupingPolicyRequest{PolicyID: "controlled-RD-layer", Epoch: 8, EligibleKinds: []string{"CALLS", "REFERENCES_SYMBOL", "RESOLVES_TO_DEFINITION"}, Rules: rules, Limits: rg.GroupingPolicyLimits{MaxRules: 3, MaxCanonicalBytes: 1 << 16, MaxStringBytes: 256, MaxAbsoluteNumerator: 1, MaxDenominator: 1, MaxCap: 1}})
	if err != nil {
		t.Fatal(err)
	}
	entries[0].Policy = policy
	// C slot retains real R occurrences, all excluded by the frozen C policy.
	entries[1].Input, _ = bundleFixtureInput(t, []programcadmission.RelationKind{programcadmission.TypedReferencesSymbol}, true)
	for _, i := range []int{0, 1} {
		e := &entries[i]
		var err error
		e.Case, err = pc.NewGroupingEvaluationCasePrivate(pc.GroupingEvaluationCaseRequestPrivate{CaseID: fmt.Sprintf("controlled-empty-excluded-%d", i), RunID: e.Slot.RunID, SelectorID: "controlled", SelectorBytes: []byte("controlled selection\n"), Seed: e.Slot.Seed, Input: e.Input, Policy: e.Policy, ProjectionLimits: evaluationLimits()})
		if err != nil {
			t.Fatal(err)
		}
		e.Result, err = pc.EvaluateGroupingCasePrivate(e.Case)
		if err != nil {
			t.Fatal(err)
		}
	}
	entries[2] = cc.PrivateAllModeEntry{Slot: slots[2], Disposition: cc.PrivateAllModeDeclaredMissing, Lineage: "controlled-upstream-selector", MissingReason: "NO_AUTHENTICATED_AVAILABILITY"}
	h, err := cc.NewPrivateAllModeHandoff("controlled-workspace", "controlled-revision", slots, entries)
	if err != nil {
		t.Fatal(err)
	}
	out := cc.Run(context.Background(), cc.Request{PrivateAllMode: &h, Workspace: "controlled-workspace", Contract: allModeContract(t), Store: &allModeStore{MemoryStore: cc.NewMemoryStore()}, StopAfter: cc.StopAfterSourceIntents})
	if out.Err != nil || out.PrivateAllMode == nil {
		t.Fatal("ASSERT_ALLMODE_EMPTY_EXCLUDED_MISSING", out.Err)
	}
	c := out.PrivateAllMode.Counts
	if c.Slots != 7 || c.Ready != 6 || c.Missing != 1 || c.Empty != 1 || c.Excluded != 1 {
		t.Fatal("ASSERT_ALLMODE_EMPTY_EXCLUDED_MISSING", c)
	}
	if len(out.PrivateAllMode.Diagnostics) != 3 || out.PrivateAllMode.Diagnostics[0].Code == out.PrivateAllMode.Diagnostics[1].Code || out.PrivateAllMode.Diagnostics[1].Code == out.PrivateAllMode.Diagnostics[2].Code {
		t.Fatal("ASSERT_ALLMODE_DISTINCT_GAP_DIAGNOSTICS")
	}
	t.Log("ASSERT_ALLMODE_EMPTY_EXCLUDED_MISSING: PASS")
	t.Log("ASSERT_ALLMODE_DISTINCT_GAP_DIAGNOSTICS: PASS")
}

func TestPrivateAllModeScheduleAndSwaps(t *testing.T) {
	slots, entries := allModeFixture(t)
	for _, mutate := range []func([]cc.PrivateAllModeSlot, []cc.PrivateAllModeEntry){
		func(s []cc.PrivateAllModeSlot, e []cc.PrivateAllModeEntry) { e[0].Result = e[1].Result },
		func(s []cc.PrivateAllModeSlot, e []cc.PrivateAllModeEntry) { e[0].Case = e[1].Case },
		func(s []cc.PrivateAllModeSlot, e []cc.PrivateAllModeEntry) { e[0].Input = e[1].Input },
		func(s []cc.PrivateAllModeSlot, e []cc.PrivateAllModeEntry) { s[0].Mode = "CRD"; e[0].Slot = s[0] },
		func(s []cc.PrivateAllModeSlot, e []cc.PrivateAllModeEntry) { s[0].Seed++; e[0].Slot = s[0] },
		func(s []cc.PrivateAllModeSlot, e []cc.PrivateAllModeEntry) { s[0].Ordinal++; e[0].Slot = s[0] },
		func(s []cc.PrivateAllModeSlot, e []cc.PrivateAllModeEntry) { e[0], e[1] = e[1], e[0] },
	} {
		s := append([]cc.PrivateAllModeSlot(nil), slots...)
		e := append([]cc.PrivateAllModeEntry(nil), entries...)
		mutate(s, e)
		if _, err := cc.NewPrivateAllModeHandoff("controlled-workspace", "controlled-revision", s, e); err == nil {
			t.Fatal("ASSERT_ALLMODE_SCHEDULE_SWAP_REFUSED")
		}
	}
	t.Log("ASSERT_ALLMODE_SCHEDULE_SWAP_REFUSED: PASS")
}
