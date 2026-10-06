package sessionruntime

import (
	"context"
	"testing"
	"unsafe"

	"lsp-trace/internal/session"
)

func TestPrivateB4SourceIngressLayoutEqualityAndPlusOne(t *testing.T) {
	if unsafe.Sizeof(privateB4SourceIngressEntry{}) != 32 || unsafe.Alignof(privateB4SourceIngressEntry{}) != 8 ||
		unsafe.Sizeof(privateB4SourceIngressLease{}) != 24 || unsafe.Alignof(privateB4SourceIngressLease{}) != 8 {
		t.Fatal("ASSERT_C15_SOURCE_INGRESS_LAYOUT")
	}
	owner, failure := newPrivateB4SourceIngressOwner()
	if failure != "" {
		t.Fatal(failure)
	}
	if got := owner.snapshot(); got != (privateB4ByteLedgerSnapshot{Live: privateB4SourceIngressTableBytes, Cumulative: privateB4SourceIngressTableBytes}) {
		t.Fatalf("ASSERT_C15_SOURCE_INGRESS_TABLE snapshot=%+v", got)
	}
	leases := make([]privateB4SourceIngressLease, privateB4SourceIngressSlots)
	for i := range leases {
		leases[i], failure = owner.reserve(privateB4SourceIngressStateBytes, nil)
		if failure != "" {
			t.Fatalf("ASSERT_C15_SOURCE_INGRESS_EQUALITY_%d failure=%q", i, failure)
		}
	}
	if got := owner.snapshot(); got.Live != privateB4SourceIngressTotalBytes || got.Cumulative != privateB4SourceIngressTotalBytes {
		t.Fatalf("ASSERT_C15_SOURCE_INGRESS_EQUALITY snapshot=%+v", got)
	}
	before := owner.snapshot()
	if _, failure = owner.reserve(privateB4SourceIngressStateBytes, nil); failure != session.ResourceExhausted || owner.snapshot() != before {
		t.Fatalf("ASSERT_C15_SOURCE_INGRESS_PLUS_ONE failure=%q before=%+v after=%+v", failure, before, owner.snapshot())
	}
	for i := range leases {
		leases[i].release()
	}
	if got := owner.snapshot(); got.Live != privateB4SourceIngressTableBytes || got.Cumulative != privateB4SourceIngressTotalBytes {
		t.Fatalf("ASSERT_C15_SOURCE_INGRESS_RELEASE snapshot=%+v", got)
	}
}

func TestPrivateB4SourceIngressManagerShutdownReleasesBacking(t *testing.T) {
	owner, failure := newPrivateB4SourceIngressOwner()
	if failure != "" {
		t.Fatal(failure)
	}
	state := &privateB4SourceLease{source: B4DefinitionTargetSource{Bytes: []byte("source")}}
	state.state.Store(privateB4SourceHeld)
	lease, failure := owner.reserve(privateB4SourceIngressStateBytes, state)
	if failure != "" {
		t.Fatal(failure)
	}
	lease.entry().state = state
	m := &Manager{privateB4SourceIngress: owner}
	if err := m.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
	if owner.snapshot().Live != 0 || owner.entries != nil || state.state.Load() != privateB4SourceReleased || state.source.Bytes != nil {
		t.Fatalf("ASSERT_C15_SOURCE_INGRESS_SHUTDOWN snapshot=%+v entries=%v state=%d bytes=%d", owner.snapshot(), owner.entries != nil, state.state.Load(), len(state.source.Bytes))
	}
	owner.releaseTableBacking()
	lease.release()
	if owner.snapshot().Live != 0 {
		t.Fatal("ASSERT_C15_SOURCE_INGRESS_SHUTDOWN_REPLAY")
	}
}

func TestPrivateB4SourceIngressMaximumTransferAndDuplicateCopy(t *testing.T) {
	owner, failure := newPrivateB4SourceIngressOwner()
	if failure != "" {
		t.Fatal(failure)
	}
	leases := make([]privateB4SourceIngressLease, privateB4SourceIngressSlots)
	handles := make([]*privateB4SourceIngressLease, privateB4SourceIngressSlots+1)
	for i := range leases {
		leases[i], failure = owner.reserve(privateB4SourceIngressStateBytes, nil)
		if failure != "" {
			t.Fatalf("ASSERT_C15_SOURCE_MAX_RESERVE_%d failure=%q", i, failure)
		}
		handles[i] = &leases[i]
	}
	duplicate := leases[0]
	handles[len(handles)-1] = &duplicate
	transaction, failure := newPrivateB4ByteAccountV2()
	if failure != "" {
		t.Fatal(failure)
	}
	descriptor, failure := owner.transferMany(handles, transaction)
	if failure != "" || !descriptor.lease.active || duplicate.active || owner.snapshot().Live != privateB4SourceIngressTableBytes {
		t.Fatalf("ASSERT_C15_SOURCE_MAX_TRANSFER failure=%q ingress=%+v", failure, owner.snapshot())
	}
	if got := transaction.snapshot(); got.Live != privateB4ByteLedgerV2TableBytes+uint64(privateB4SourceIngressSlots)*privateB4SourceIngressStateBytes || got.Cumulative != privateB4ByteLedgerV2TableBytes {
		t.Fatalf("ASSERT_C15_SOURCE_MAX_TRANSFER_ACCOUNTING snapshot=%+v", got)
	}
	descriptor.release()
}

func TestPrivateB4SourceIngressTransferStatesAtomicAndSlotNeutral(t *testing.T) {
	owner, failure := newPrivateB4SourceIngressOwner()
	if failure != "" {
		t.Fatal(failure)
	}
	first, second := &privateB4SourceLease{}, &privateB4SourceLease{}
	first.ingress, failure = owner.reserve(privateB4SourceIngressStateBytes, first)
	if failure != "" {
		t.Fatal(failure)
	}
	second.ingress, failure = owner.reserve(privateB4SourceIngressStateBytes, second)
	if failure != "" {
		t.Fatal(failure)
	}
	states := map[*privateB4SourceLease]struct{}{first: {}, second: {}}
	account := mustPrivateB4ByteAccountV2(t)
	beforeIngress, beforeAccount := owner.snapshot(), account.snapshot()
	blocker, failure := account.reserveDescriptor(1)
	if failure != "" {
		t.Fatal(failure)
	}
	if _, failure = owner.transferStates(states, account); failure != session.ResourceExhausted || owner.snapshot() != beforeIngress || account.snapshot() != (privateB4ByteLedgerSnapshot{Live: beforeAccount.Live + 1, Cumulative: beforeAccount.Cumulative}) || !first.ingress.active || !second.ingress.active {
		t.Fatalf("ASSERT_C15_SOURCE_STATE_TRANSFER_ROLLBACK failure=%q ingress=%+v account=%+v", failure, owner.snapshot(), account.snapshot())
	}
	blocker.release()
	descriptor, failure := owner.transferStates(states, account)
	if failure != "" || !descriptor.lease.active || first.ingress.active || second.ingress.active {
		t.Fatalf("ASSERT_C15_SOURCE_STATE_TRANSFER failure=%q descriptor=%+v", failure, descriptor)
	}
	if got := account.snapshot(); got.Live != beforeAccount.Live+2*privateB4SourceIngressStateBytes || got.Cumulative != beforeAccount.Cumulative {
		t.Fatalf("ASSERT_C15_SOURCE_STATE_TRANSFER_SLOT_NEUTRAL snapshot=%+v", got)
	}
	descriptor.release()
}

func TestPrivateB4SourceIngressAtomicAggregateTransfer(t *testing.T) {
	owner, failure := newPrivateB4SourceIngressOwner()
	if failure != "" {
		t.Fatal(failure)
	}
	first, failure := owner.reserve(privateB4SourceIngressStateBytes, nil)
	if failure != "" {
		t.Fatal(failure)
	}
	second, failure := owner.reserve(privateB4SourceIngressStateBytes, nil)
	if failure != "" {
		t.Fatal(failure)
	}
	transaction, failure := newPrivateB4ByteAccountV2()
	if failure != "" {
		t.Fatal(failure)
	}
	beforeIngress, beforeTransaction := owner.snapshot(), transaction.snapshot()
	blocker, failure := transaction.reserveDescriptor(1)
	if failure != "" {
		t.Fatal(failure)
	}
	if _, failure = owner.transferMany([]*privateB4SourceIngressLease{&first, &second}, transaction); failure != session.ResourceExhausted || owner.snapshot() != beforeIngress || !first.active || !second.active {
		t.Fatalf("ASSERT_C15_SOURCE_TRANSFER_ROLLBACK failure=%q", failure)
	}
	blocker.release()
	descriptor, failure := owner.transferMany([]*privateB4SourceIngressLease{&first, &second}, transaction)
	if failure != "" || !descriptor.lease.active || first.active || second.active {
		t.Fatalf("ASSERT_C15_SOURCE_TRANSFER failure=%q descriptor=%+v", failure, descriptor)
	}
	if owner.snapshot().Live != beforeIngress.Live-2*privateB4SourceIngressStateBytes || transaction.snapshot().Live != beforeTransaction.Live+2*privateB4SourceIngressStateBytes || transaction.snapshot().Cumulative != beforeTransaction.Cumulative {
		t.Fatal("ASSERT_C15_SOURCE_TRANSFER_ACCOUNTING")
	}
	descriptor.release()
}
