package sourcecontinuation

import (
	"context"
	"lsp-trace/internal/sourceprojection"
	"strings"
	"testing"
	"time"

	cc "lsp-trace/internal/censuscontinuation"
	"lsp-trace/internal/describerequest"
	inv "lsp-trace/internal/inventorysuccessor"
	rg "lsp-trace/internal/referencegrouping"
	tp "lsp-trace/internal/targetpacket"
)

// Controlled DATA ONLY. These rows/units are not native admissions, captured
// originals, qualifications or evidence that the native-positive path executed.
func selectedRowsTestV2(mode string, duplicate bool) (cc.PrivateAllModeResult, PrivateAllModeSourcesResult, inv.PrivateSelectedConfigurationV2) {
	c := inv.PrivateSelectedConfigurationV2{Mode: mode, RunID: "fresh-run", SeedNamespace: "fresh-namespace", Lineage: "fresh-lineage", Seed: 19}
	slot := selectedSlotV2(c)
	kind, role := "CALLS", "CALLEE_TARGET"
	for _, r := range mode {
		if r == 'R' {
			kind, role = "REFERENCES_SYMBOL", "REFERENCED_SYMBOL"
		}
		if r == 'D' {
			kind, role = "RESOLVES_TO_DEFINITION", "DEFINITION_TARGET"
			break
		}
	}
	e := tp.PrivateTypedEndpoint{Node: "non-query-target", Kind: kind, Occurrence: "original-occurrence", Role: role, Predecessor: "original-predecessor", Custody: "CALLER_ASSERTED", Disposition: "INCLUDED", Ordinal: 1}
	n := tp.PrivateNomination{ID: "nomination", Status: "PROVISIONAL_STRUCTURAL_HYPOTHESIS", ClaimCeiling: rg.EvaluationCeilingPrivate, RejectionCondition: "SPLIT_OR_WITHDRAW_IF_ADMITTED_SOURCE_REVIEW_CONTRADICTS_MEMBERSHIP", Mode: mode, RunID: c.RunID, Lineage: c.Lineage, Seed: c.Seed, Slot: slot.Ordinal, Community: 0, InputID: "input", PolicyID: "policy", CaseID: "case", ResultID: "result", Members: []string{e.Node}, Endpoints: []tp.PrivateTypedEndpoint{e}}
	if duplicate {
		n.Endpoints = append(n.Endpoints, e)
	}
	intents, err := tp.BuildPrivateSourceIntents(n, 1000)
	if err != nil {
		panic(err)
	}
	meta := cc.PrivateAllModeResult{Nominations: []tp.PrivateNomination{n}, Intents: intents, Counts: cc.PrivateAllModeCounts{Ready: 1, Missing: 6}}
	source := PrivateAllModeSourcesResult{}
	for row := range intents {
		source.Outcomes = append(source.Outcomes, PrivateSourceOutcome{Intent: row, Status: "SOURCE_ADMITTED", AdmissionID: "shared-span-id", RequestID: "shared-request-id"})
	}
	return meta, source, c
}
func TestSelectedV2RowsSevenConfigurations(t *testing.T) {
	for _, mode := range []string{"RD", "C", "CRD", "D", "CD", "R", "CR"} {
		t.Run(mode, func(t *testing.T) {
			m, s, c := selectedRowsTestV2(mode, false)
			row, e := selectSelectedRowV2(m, s, c, nil)
			if e != nil || row.Intent.Endpoint.Node != "non-query-target" {
				t.Fatal("full row selection", e)
			}
			if mode == "RD" && (row.Intent.Endpoint.Role != "DEFINITION_TARGET" || row.Intent.Endpoint.Kind != "RESOLVES_TO_DEFINITION") {
				t.Fatal("RD non-query definition target")
			}
			if (inv.PrivateSourceAdmission{}).Matches(row.Intent) {
				t.Fatal("DATA became admission")
			}
		})
	}
}
func TestSelectedV2DuplicateRowPosition(t *testing.T) {
	m, s, c := selectedRowsTestV2("RD", true)
	if _, e := selectSelectedRowV2(m, s, c, nil); e == nil {
		t.Fatal("ASSERT_SELECTED_ROW: identical actual rows require position")
	}
	selector := PrivateSelectedEndpointV2{RowPosition: -1, Slot: selectedSlotV2(c), Intent: m.Intents[0]}
	if _, e := selectSelectedRowV2(m, s, c, &selector); e == nil {
		t.Fatal("ASSERT_SELECTED_ROW: tuple alone cannot select identical rows")
	}
	selector.RowPosition = 1
	row, e := selectSelectedRowV2(m, s, c, &selector)
	if e != nil || row.RowPosition != 1 {
		t.Fatal("ASSERT_SELECTED_ROW: original position must select duplicate", e)
	}
	selector.Intent.Endpoint.Ordinal++
	if _, e := selectSelectedRowV2(m, s, c, &selector); e == nil {
		t.Fatal("wrong local ordinal accepted")
	}
	t.Log("ASSERT_SELECTED_ROW: PASS")
}
func TestSelectedV2FullTuplePerturbations(t *testing.T) {
	m, s, c := selectedRowsTestV2("RD", false)
	correct := PrivateSelectedEndpointV2{RowPosition: 0, Slot: selectedSlotV2(c), Intent: m.Intents[0]}
	mutations := []func(*PrivateSelectedEndpointV2){
		func(v *PrivateSelectedEndpointV2) { v.Slot.SeedNamespace += "x" }, func(v *PrivateSelectedEndpointV2) { v.Slot.Mode = "R" }, func(v *PrivateSelectedEndpointV2) { v.Slot.Ordinal++ }, func(v *PrivateSelectedEndpointV2) { v.Slot.RunID += "x" }, func(v *PrivateSelectedEndpointV2) { v.Slot.Seed++ },
		func(v *PrivateSelectedEndpointV2) { v.Intent.NominationID += "x" }, func(v *PrivateSelectedEndpointV2) { v.Intent.Community++ }, func(v *PrivateSelectedEndpointV2) { v.Intent.InputID += "x" }, func(v *PrivateSelectedEndpointV2) { v.Intent.PolicyID += "x" }, func(v *PrivateSelectedEndpointV2) { v.Intent.CaseID += "x" }, func(v *PrivateSelectedEndpointV2) { v.Intent.ResultID += "x" }, func(v *PrivateSelectedEndpointV2) { v.Intent.Lineage += "x" },
		func(v *PrivateSelectedEndpointV2) { v.Intent.Endpoint.Node += "x" }, func(v *PrivateSelectedEndpointV2) { v.Intent.Endpoint.Kind = "REFERENCES_SYMBOL" }, func(v *PrivateSelectedEndpointV2) { v.Intent.Endpoint.Role = "QUERY_OCCURRENCE" }, func(v *PrivateSelectedEndpointV2) { v.Intent.Endpoint.Occurrence += "x" }, func(v *PrivateSelectedEndpointV2) { v.Intent.Endpoint.Ordinal++ }, func(v *PrivateSelectedEndpointV2) { v.Intent.Endpoint.Predecessor += "x" }, func(v *PrivateSelectedEndpointV2) { v.Intent.Endpoint.Custody += "x" }, func(v *PrivateSelectedEndpointV2) { v.Intent.Endpoint.Disposition = "EXCLUDED" },
	}
	for i, mutate := range mutations {
		v := correct
		mutate(&v)
		if _, e := selectSelectedRowV2(m, s, c, &v); e == nil {
			t.Fatalf("full tuple mutation %d", i)
		}
	}
	for _, status := range []string{"UNAVAILABLE", "OBSERVED_EMPTY", "OBSERVED_NONEMPTY", "EXCLUDED", "DECLARED_MISSING_UNRESOLVED"} {
		copy := s
		copy.Outcomes = append([]PrivateSourceOutcome(nil), s.Outcomes...)
		copy.Outcomes[0].Status = status
		if _, e := selectSelectedRowV2(m, copy, c, nil); e == nil {
			t.Fatal("non-admitted state selected", status)
		}
	}
}
func TestSelectedV2ReadbackBody(t *testing.T) {
	a := PrivateAllModeSourcesResult{CheckpointID: "checkpoint", HandoffID: "handoff", Units: []tp.PrivateSourceUnit{{Body: "own FULL_DEFINITION"}}}
	b := a
	b.Units = append([]tp.PrivateSourceUnit(nil), a.Units...)
	if !sameSelectedReadbackV2(a, b) {
		t.Fatal("ASSERT_SELECTED_READBACK: exact body refused")
	}
	b.Units[0].Body = "query-side R substitution"
	if sameSelectedReadbackV2(a, b) {
		t.Fatal("ASSERT_SELECTED_READBACK: body substitution accepted")
	}
	b = a
	b.CheckpointID = "foreign"
	if sameSelectedReadbackV2(a, b) {
		t.Fatal("checkpoint substitution")
	}
	b = a
	b.Requests = []describerequest.Record{{RecordID: "foreign"}}
	if sameSelectedReadbackV2(a, b) {
		t.Fatal("request substitution")
	}
	t.Log("ASSERT_SELECTED_READBACK: PASS")
}

type selectedCorruptStoreTestV2 struct {
	*selectedStoreV2
	corrupt bool
}

func (s *selectedCorruptStoreTestV2) Get(ctx context.Context, id string) ([]byte, error) {
	b, e := s.selectedStoreV2.Get(ctx, id)
	if e == nil && s.corrupt && len(b) > 0 {
		b[0] ^= 1
	}
	return b, e
}

// This exercises the ACTUAL consumers with metadata-only missing slots. It is
// expressly not a native admission, positive source qualification or real job.
func TestSelectedV2ActualRunWholeResumeMissingOnly(t *testing.T) {
	pin := "sha256:" + strings.Repeat("0", 64)
	contract, e := cc.NewContinuationContract(cc.ContractInput{ContinuationID: pin, ProfileID: pin, WorkerPinID: pin, CaptureLimits: cc.ProductionCaptureLimits(), PacketSourcePolicy: sourceprojection.Policy{PolicyID: "CONTROLLED_MISSING_ONLY"}, PacketMaxResponseBytes: 1 << 20, DescribeDeadlineMS: 1000, RetryPolicy: cc.RetryPolicy{MaxAttemptsPerRequest: 1}})
	if e != nil {
		t.Fatal(e)
	}
	slots := make([]cc.PrivateAllModeSlot, 7)
	entries := make([]cc.PrivateAllModeEntry, 7)
	for i, mode := range []string{"RD", "C", "CRD", "D", "CD", "R", "CR"} {
		slots[i] = cc.PrivateAllModeSlot{Ordinal: i, Mode: mode, RunID: "synthetic-missing/" + mode, SeedNamespace: "synthetic-namespace", Seed: 3}
		entries[i] = cc.PrivateAllModeEntry{Slot: slots[i], Disposition: cc.PrivateAllModeDeclaredMissing, Lineage: "synthetic-lineage", MissingReason: "CONTROLLED_MISSING_TEST_DATA"}
	}
	h, e := cc.NewPrivateAllModeHandoff("synthetic-workspace", "synthetic-revision", slots, entries)
	if e != nil {
		t.Fatal(e)
	}
	deadline := time.Now().Add(10 * time.Second)
	ctx, cancel := context.WithDeadline(context.Background(), deadline)
	defer cancel()
	memory, e := newSelectedStoreV2(8<<20, 64<<20)
	if e != nil {
		t.Fatal(e)
	}
	defer func() { _ = memory.close() }()
	store := &selectedCorruptStoreTestV2{selectedStoreV2: memory}
	source := PrivateAllModeSourcesRequest{PolicyID: "CONTROLLED_MISSING_ONLY", Deadline: deadline, Limits: tp.PrivateSourceLimits{MaxDocumentBytes: 1024, MaxTotalBytes: 4096, MaxWork: 10000, MaxObjects: 1000}}
	run := Run(ctx, Request{PrivateSources: &source, PrivateAllMode: &h, Workspace: "synthetic-workspace", Contract: contract, Store: store, StopAfter: StopAfterPrivateSourceRequests})
	if run.Err != nil || run.PrivateSources == nil || run.PrivateSources.Counts.Admitted != 0 || run.PrivateSources.Counts.Unresolved != 7 || len(run.PrivateSources.Requests) != 0 {
		t.Fatal("actual missing-only Run", run.Err)
	}
	q := ResumeRequest{PrivateSources: &PrivateAllModeSourcesResume{Version: PrivateAllModeSourcesVersion, Expected: run.PrivateSources.Expected, ExpectedHandoff: h}, Workspace: "synthetic-workspace", Store: store, StopAfter: StopAfterPrivateSourceRequests, Selector: run.PrivateSources.CheckpointID}
	resumed := Resume(ctx, q)
	if resumed.Err != nil || resumed.PrivateSources == nil || !sameSelectedReadbackV2(*run.PrivateSources, *resumed.PrivateSources) || memory.puts != 7 || memory.gets != 7 {
		t.Fatal("actual missing-only whole Resume", resumed.Err)
	}
	// A fresh read budget for the independent corruption witness; no native
	// operation, accounting reservation or producer is reset/reused here.
	corruptMemory := &selectedStoreV2{memory: memory.memory, cap: memory.cap, parent: memory.parent}
	corrupted := &selectedCorruptStoreTestV2{selectedStoreV2: corruptMemory, corrupt: true}
	q.Store = corrupted
	bad := Resume(ctx, q)
	if bad.Err == nil || bad.PrivateSources != nil {
		t.Fatal("actual whole Resume accepted corrupted material")
	}
	t.Log("ASSERT_SELECTED_ACTUAL_MISSING_ONLY_RUN_RESUME: PASS; ZERO_NATIVE_ADMISSIONS")
}

func TestSelectedV2StoreAndZeroNative(t *testing.T) {
	ctx := context.Background()
	s, e := newSelectedStoreV2(64, 128)
	if e != nil {
		t.Fatal(e)
	}
	original := []byte("body")
	id, e := s.Put(ctx, original)
	if e != nil {
		t.Fatal(e)
	}
	original[0] = 'X'
	got, e := s.Get(ctx, id)
	if e != nil || string(got) != "body" {
		t.Fatal("store Put clone")
	}
	got[0] = 'Y'
	got, e = s.Get(ctx, id)
	if e != nil || string(got) != "body" {
		t.Fatal("store Get clone")
	}
	if _, e = s.Put(ctx, make([]byte, 65)); e == nil {
		t.Fatal("pre-Put cap")
	}
	if e = s.close(); e != nil {
		t.Fatal(e)
	}
	if _, e = s.Get(ctx, id); e == nil {
		t.Fatal("closed store")
	}
	result, e := PrepareSelectedV2(ctx, inv.PrivateSelectedSourceOperationV2{}, nil)
	if e == nil || result.Available() {
		t.Fatal("zero native authority became result")
	}
}
