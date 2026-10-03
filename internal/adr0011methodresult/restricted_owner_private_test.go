package adr0011methodresult

import (
	"errors"
	"runtime"
	"testing"
	"unsafe"
)

func restrictedCandidate() DefinitionCandidate {
	r := Range{}
	return DefinitionCandidate{OccurrenceID: "sha256:" + string(make([]byte, 64)), QueryOccurrenceID: "q", TargetID: "sha256:" + string(make([]byte, 64)), QueryURI: "file:///query.go", TargetURI: "file:///target.go", TargetRange: &r}
}

func restrictedMaxCandidate() DefinitionCandidate {
	r := Range{}
	return DefinitionCandidate{OccurrenceID: string(make([]byte, 71)), QueryOccurrenceID: string(make([]byte, 1024)), TargetID: string(make([]byte, 71)), QueryURI: string(make([]byte, 4096)), TargetURI: string(make([]byte, 4096)), TargetRange: &r}
}

func newRestrictedRootManager(t *testing.T, used uint64) (*restrictedBudgetRoot, *restrictedOwnerManager) {
	t.Helper()
	root := &restrictedBudgetRoot{}
	m, err := root.bootstrap(used)
	if err != nil {
		t.Fatal(err)
	}
	return root, m
}

func newRestrictedLease(t *testing.T, m *restrictedOwnerManager, owner, session, generation uint64, n int) restrictedLease {
	t.Helper()
	p, err := m.reserveProcessing(owner, session, generation)
	if err != nil {
		t.Fatal(err)
	}
	in := make([]DefinitionCandidate, n)
	for i := range in {
		in[i] = restrictedCandidate()
		in[i].Ordinal = i
	}
	l, err := m.construct(p, in)
	if err != nil {
		t.Fatal(err)
	}
	return l
}

func acceptAndView(t *testing.T, m *restrictedOwnerManager, l restrictedLease, r restrictedRecipient) restrictedView {
	t.Helper()
	if err := m.accept(l, l.Transfer, r); err != nil {
		t.Fatal(err)
	}
	v, err := m.view(l, r)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func finishRestricted(t *testing.T, m *restrictedOwnerManager, l restrictedLease, v restrictedView) {
	t.Helper()
	if err := m.returnView(l, v); err != nil {
		t.Fatal(err)
	}
	if err := m.recordTerminal(l); err != nil {
		t.Fatal(err)
	}
	if err := m.joinWork(l); err != nil {
		t.Fatal(err)
	}
	if err := m.disposeRetention(l); err != nil {
		t.Fatal(err)
	}
	if err := m.forget(l); err != nil {
		t.Fatal(err)
	}
	if err := m.release(l); err != nil {
		t.Fatal(err)
	}
}

func TestRestrictedOwnerV2BootstrapSharedBudget(t *testing.T) {
	root := &restrictedBudgetRoot{}
	before := root.constructorCalls
	if _, err := root.bootstrap(restrictedGlobalBytes); !errors.Is(err, errRestrictedBusy) {
		t.Fatalf("ASSERT_BOOTSTRAP_PREEXISTING fail err=%v", err)
	}
	if root.constructorCalls != before {
		t.Fatal("ASSERT_MANAGER_NOT_CONSTRUCTED fail")
	}
	m, err := root.bootstrap(1234)
	if err != nil {
		t.Fatal(err)
	}
	if root.globalUsed != 1234+restrictedLogicalControlBytes || m.root != root {
		t.Fatalf("ASSERT_BOOTSTRAP_CHARGE fail used=%d", root.globalUsed)
	}
	if _, err := root.bootstrap(0); !errors.Is(err, errRestrictedState) {
		t.Fatal("ASSERT_SINGLE_BOOTSTRAP fail")
	}
}

func TestRestrictedOwnerV2ExactGenerationSlotsAndBudgets(t *testing.T) {
	root, m := newRestrictedRootManager(t, 0)
	leases := make([]restrictedLease, 4)
	for i := range leases {
		leases[i] = newRestrictedLease(t, m, 1, 2, 77, 0)
	}
	if _, err := m.reserveProcessing(1, 2, 77); !errors.Is(err, errRestrictedBusy) {
		t.Fatalf("ASSERT_EXACT_GEN_FIFTH fail err=%v", err)
	}
	if _, err := m.reserveProcessing(1, 3, 77); err != nil {
		t.Fatalf("ASSERT_SESSION_ISOLATION fail %v", err)
	}
	if root.globalSlots != 5 {
		t.Fatalf("ASSERT_GLOBAL_SLOT_COUNT fail got=%d", root.globalSlots)
	}

	_, m2 := newRestrictedRootManager(t, restrictedGlobalBytes-restrictedLogicalControlBytes-restrictedManualPeak+1)
	if _, err := m2.reserveProcessingCaps(9, 9, 9, restrictedManualCaps); !errors.Is(err, errRestrictedBusy) {
		t.Fatalf("ASSERT_PREVIOUS_USAGE_DENIES fail err=%v", err)
	}
	root3, m3 := newRestrictedRootManager(t, 0)
	if _, err := m3.reserveProcessing(1, 1, 1); err != nil {
		t.Fatal(err)
	}
	root3.globalUsed = restrictedGlobalBytes - 29
	if _, err := m3.reserveProcessing(2, 2, 2); !errors.Is(err, errRestrictedBusy) {
		t.Fatalf("ASSERT_OTHER_GEN_GLOBAL fail err=%v", err)
	}
}

func TestRestrictedOwnerV2GlobalSlotsAndSuccessorIsolation(t *testing.T) {
	root, m := newRestrictedRootManager(t, 0)
	for i := 0; i < restrictedGlobalSlots; i++ {
		if _, err := m.reserveProcessing(uint64(i+1), uint64(i+101), 1); err != nil {
			t.Fatalf("ASSERT_GLOBAL_16 fail slot=%d err=%v", i, err)
		}
	}
	if _, err := m.reserveProcessing(99, 199, 1); !errors.Is(err, errRestrictedBusy) {
		t.Fatalf("ASSERT_GLOBAL_17_REJECT fail err=%v", err)
	}
	if root.globalSlots != restrictedGlobalSlots {
		t.Fatalf("ASSERT_GLOBAL_SLOT_EXACT fail got=%d", root.globalSlots)
	}

	_, m2 := newRestrictedRootManager(t, 0)
	old := newRestrictedLease(t, m2, 7, 8, 9, 1)
	oldView := acceptAndView(t, m2, old, restrictedRecipient{ID: 1})
	finishRestricted(t, m2, old, oldView)
	successor := newRestrictedLease(t, m2, 7, 8, 9, 1)
	before := m2.snapshot(successor)
	if err := m2.accept(old, old.Transfer, restrictedRecipient{ID: 1}); !errors.Is(err, errRestrictedIdentity) || m2.snapshot(successor) != before {
		t.Fatal("ASSERT_OLD_CANNOT_MUTATE_SUCCESSOR fail")
	}

	quarantined := newRestrictedLease(t, m2, 11, 12, 13, 0)
	if err := m2.quarantine(quarantined); err != nil {
		t.Fatal(err)
	}
	_ = newRestrictedLease(t, m2, 11, 12, 13, 0)
	charged := m2.root.globalUsed
	if err := m2.release(quarantined); !errors.Is(err, errRestrictedIncomplete) || m2.root.globalUsed != charged {
		t.Fatal("ASSERT_OLD_QUARANTINE_CANNOT_FREE_SUCCESSOR fail")
	}
}

func TestRestrictedOwnerV2CounterWrap(t *testing.T) {
	_, m := newRestrictedRootManager(t, 0)
	m.nextAttempt = ^uint64(0)
	if _, err := m.reserveProcessing(1, 2, 3); err != nil {
		t.Fatalf("ASSERT_LAST_ATTEMPT_ID fail %v", err)
	}
	if _, err := m.reserveProcessing(4, 5, 6); !errors.Is(err, errRestrictedState) {
		t.Fatalf("ASSERT_ATTEMPT_WRAP fail err=%v", err)
	}

	_, m2 := newRestrictedRootManager(t, 0)
	p, err := m2.reserveProcessing(1, 2, 3)
	if err != nil {
		t.Fatal(err)
	}
	m2.nextBacking = ^uint64(0) - 1
	if _, err := m2.construct(p, nil); !errors.Is(err, errRestrictedState) {
		t.Fatalf("ASSERT_BACKING_WRAP fail err=%v", err)
	}
	if m2.attempts[p.slot].outputBuilt {
		t.Fatal("ASSERT_BACKING_WRAP_ZERO_EFFECT fail")
	}
}

func TestRestrictedOwnerV2ProcessingCustodyAndMaxOutput(t *testing.T) {
	_, m := newRestrictedRootManager(t, 0)
	forged := restrictedProcessingReservation{Owner: 1, Session: 2, Generation: 3, Version: 1}
	if _, err := m.construct(forged, nil); !errors.Is(err, errRestrictedIdentity) {
		t.Fatalf("ASSERT_FORGED_PROCESSING fail err=%v", err)
	}
	p, err := m.reserveProcessingCaps(1, 2, 3, restrictedManualCaps)
	if err != nil {
		t.Fatal(err)
	}
	if p.Capacity != restrictedManualPeak || len(p.regions) != restrictedManualBackingCount {
		t.Fatalf("ASSERT_ACTUAL_REGIONS fail cap=%d count=%d", p.Capacity, len(p.regions))
	}
	in := make([]DefinitionCandidate, 64)
	for i := range in {
		in[i] = restrictedMaxCandidate()
		in[i].Ordinal = i
	}
	l, err := m.construct(p, in)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := any(l).(interface{ Output() []DefinitionCandidate }); ok {
		t.Fatal("ASSERT_NO_UNTRACKED_OUTPUT_FIELD fail")
	}
	v := acceptAndView(t, m, l, restrictedRecipient{ID: 7})
	if len(v.Output()) != 64 || cap(v.Output()) != 64 || v.Output()[63].TargetURI != in[63].TargetURI {
		t.Fatal("ASSERT_MAX_CORRESPONDENCE fail")
	}
	used := m.root.globalUsed
	if err := m.release(l); !errors.Is(err, errRestrictedIncomplete) || m.root.globalUsed != used {
		t.Fatal("ASSERT_VIEW_RETAINS_CHARGE fail")
	}
	finishRestricted(t, m, l, v)
}

func TestRestrictedOwnerV2ACKAndViewZeroEffects(t *testing.T) {
	_, m := newRestrictedRootManager(t, 0)
	l := newRestrictedLease(t, m, 1, 2, 3, 1)
	before := m.snapshot(l)
	wrong := l.Transfer
	wrong.Backings[1].Version++
	if !errors.Is(m.accept(l, wrong, restrictedRecipient{ID: 7}), errRestrictedIdentity) || m.snapshot(l) != before {
		t.Fatal("ASSERT_WRONG_ACK_ZERO_EFFECT fail")
	}
	partial := l.Transfer
	partial.Backings[2] = restrictedBackingID{}
	if !errors.Is(m.accept(l, partial, restrictedRecipient{ID: 7}), errRestrictedIdentity) || m.snapshot(l) != before {
		t.Fatal("ASSERT_PARTIAL_ACK_ZERO_EFFECT fail")
	}
	if err := m.accept(l, l.Transfer, restrictedRecipient{ID: 7}); err != nil {
		t.Fatal(err)
	}
	accepted := m.snapshot(l)
	if !errors.Is(m.accept(l, l.Transfer, restrictedRecipient{ID: 8}), errRestrictedState) || m.snapshot(l) != accepted {
		t.Fatal("ASSERT_WRONG_RECIPIENT_ZERO_EFFECT fail")
	}
	if _, err := m.view(l, restrictedRecipient{ID: 8}); !errors.Is(err, errRestrictedIdentity) {
		t.Fatal("ASSERT_NONRECIPIENT_VIEW fail")
	}
	v, err := m.view(l, restrictedRecipient{ID: 7})
	if err != nil {
		t.Fatal(err)
	}
	if err := m.returnView(l, v); err != nil {
		t.Fatal(err)
	}
	if err := m.returnView(l, v); err != nil {
		t.Fatal("ASSERT_VIEW_RETURN_IDEMPOTENT fail")
	}
}

func TestRestrictedOwnerV2CompletionAtomicAndWorkJoin(t *testing.T) {
	_, m := newRestrictedRootManager(t, 0)
	l := newRestrictedLease(t, m, 1, 2, 3, 1)
	v := acceptAndView(t, m, l, restrictedRecipient{ID: 7})
	before := m.snapshot(l)
	for i := 0; i < 3; i++ {
		if !errors.Is(m.release(l), errRestrictedIncomplete) || m.snapshot(l) != before {
			t.Fatal("ASSERT_FAILED_RELEASE_ZERO_EFFECT fail")
		}
	}
	if err := m.cancelWork(l); err != nil {
		t.Fatal(err)
	}
	if err := m.returnView(l, v); err != nil {
		t.Fatal(err)
	}
	if err := m.recordTerminal(l); err != nil {
		t.Fatal(err)
	}
	if err := m.disposeRetention(l); err != nil {
		t.Fatal(err)
	}
	if err := m.forget(l); err != nil {
		t.Fatal(err)
	}
	if !errors.Is(m.release(l), errRestrictedIncomplete) {
		t.Fatal("ASSERT_CANCEL_NOT_JOIN fail")
	}
	if err := m.joinWork(l); err != nil {
		t.Fatal(err)
	}
	if err := m.release(l); err != nil {
		t.Fatal(err)
	}
}

func TestRestrictedOwnerV2NonblockingAndStaleIsolation(t *testing.T) {
	_, m := newRestrictedRootManager(t, 0)
	l := newRestrictedLease(t, m, 1, 2, 3, 1)
	m.mu.Lock()
	if err := m.accept(l, l.Transfer, restrictedRecipient{ID: 7}); !errors.Is(err, errRestrictedBusy) {
		t.Fatalf("ASSERT_NONBLOCK_ACK fail err=%v", err)
	}
	m.mu.Unlock()
	if err := m.accept(l, l.Transfer, restrictedRecipient{ID: 7}); err != nil {
		t.Fatal(err)
	}
	v, _ := m.view(l, restrictedRecipient{ID: 7})
	stale := l
	stale.Version++
	before := m.snapshot(l)
	if err := m.returnView(stale, v); !errors.Is(err, errRestrictedIdentity) || m.snapshot(l) != before {
		t.Fatal("ASSERT_STALE_NO_SUCCESSOR_EFFECT fail")
	}
	if err := m.quarantine(l); err != nil {
		t.Fatal(err)
	}
	charged := m.root.globalUsed
	if err := m.release(l); !errors.Is(err, errRestrictedIncomplete) || m.root.globalUsed != charged {
		t.Fatal("ASSERT_QUARANTINE_RETAINS fail")
	}
}

func TestRestrictedOwnerV2RowReuseAndWrap(t *testing.T) {
	_, m := newRestrictedRootManager(t, 0)
	l := newRestrictedLease(t, m, 1, 2, 3, 1)
	v := acceptAndView(t, m, l, restrictedRecipient{ID: 7})
	if err := m.returnView(l, v); err != nil {
		t.Fatal(err)
	}
	v2, err := m.view(l, restrictedRecipient{ID: 7})
	if err != nil {
		t.Fatal(err)
	}
	if v.token.Row != v2.token.Row || v.token.RowVersion == v2.token.RowVersion {
		t.Fatal("ASSERT_SAFE_ROW_REUSE fail")
	}
	if !errors.Is(m.returnView(l, v), errRestrictedIdentity) {
		t.Fatal("ASSERT_STALE_ROW_REFUSED fail")
	}
	if err := m.returnView(l, v2); err != nil {
		t.Fatal(err)
	}
	m.attempts[l.slot].rows[v2.token.Row].version = ^uint32(0)
	if _, err := m.view(l, restrictedRecipient{ID: 7}); !errors.Is(err, errRestrictedState) {
		t.Fatalf("ASSERT_ROW_WRAP_FAIL_CLOSED fail err=%v", err)
	}
}

func TestRestrictedOwnerV2LayoutsAndC1C14Mapping(t *testing.T) {
	if runtime.GOOS != "darwin" || runtime.GOARCH != "arm64" {
		t.Skip("pinned layout")
	}
	if unsafe.Sizeof(DefinitionCandidate{}) != 136 || unsafe.Sizeof(Range{}) != 16 {
		t.Fatal("ASSERT_PINNED_LAYOUT fail")
	}
	claims := restrictedC1C14Claims()
	if len(claims) != 14 {
		t.Fatalf("ASSERT_C1_C14_COUNT fail got=%d", len(claims))
	}
	for i, c := range claims {
		if c.ID != i+1 || c.Exercised == "" || c.NotProved == "" {
			t.Fatalf("ASSERT_C1_C14_MAPPING fail row=%d", i)
		}
	}
	t.Logf("UNIT2_V2_ACCOUNTING root_control=%d manager_control=%d processing_value=%d lease_value=%d view_value=%d processing_backings=%d construction_backings=%d steady_backings=%d", unsafe.Sizeof(restrictedBudgetRoot{}), unsafe.Sizeof(restrictedOwnerManager{}), restrictedProcessingValueBytes, restrictedLeaseValueBytes, restrictedViewValueBytes, restrictedManualPeak, restrictedConstructionMax, restrictedOutputSteadyMax)
}
