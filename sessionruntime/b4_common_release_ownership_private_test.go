package sessionruntime

import "testing"

func TestPrivateB4CommonReleasePreservesForeignAndNonTransferredSources(t *testing.T) {
	owner := &Manager{}
	foreignManager := &Manager{}
	newSource := func(manager *Manager, state uint32) B4DefinitionSourceLease {
		source := &privateB4SourceLease{manager: manager}
		source.state.Store(state)
		return B4DefinitionSourceLease{state: source}
	}

	ownedTransferredOne := newSource(owner, privateB4SourceTransferred)
	ownedTransferredTwo := newSource(owner, privateB4SourceTransferred)
	foreignTransferred := newSource(foreignManager, privateB4SourceTransferred)
	ownedHeld := newSource(owner, privateB4SourceHeld)
	ownedReleased := newSource(owner, privateB4SourceReleased)
	reservation := &privateB4Reservation{
		slot:      1,
		maxCharge: 41,
		sourceLeases: []B4DefinitionSourceLease{
			ownedTransferredOne,
			foreignTransferred,
			ownedHeld,
			ownedReleased,
			{},
			ownedTransferredTwo,
		},
	}
	reservation.token[0] = 7
	reservation.explicitBytes = mustPrivateB4ByteAccountV2(t)
	if failure := reservation.reservePrivateB4ExplicitBytes([]uint64{17}); failure != "" {
		t.Fatal(failure)
	}
	owner.privateB4Leases[reservation.slot] = privateB4Slot{occupied: true, token: reservation.token, reservation: reservation}
	owner.privateB4Bytes = reservation.maxCharge

	owner.mu.Lock()
	owner.releasePrivateB4Locked(reservation)
	owner.mu.Unlock()

	if got := foreignTransferred.state.state.Load(); got != privateB4SourceTransferred {
		t.Fatalf("ASSERT_B4_COMMON_RELEASE_PRESERVES_FOREIGN_TRANSFERRED got=%d want=%d", got, privateB4SourceTransferred)
	}
	if got := ownedHeld.state.state.Load(); got != privateB4SourceHeld {
		t.Fatalf("ASSERT_B4_COMMON_RELEASE_PRESERVES_OWNED_HELD got=%d want=%d", got, privateB4SourceHeld)
	}
	if got := ownedReleased.state.state.Load(); got != privateB4SourceReleased {
		t.Fatalf("ASSERT_B4_COMMON_RELEASE_PRESERVES_OWNED_RELEASED got=%d want=%d", got, privateB4SourceReleased)
	}
	for i, source := range []B4DefinitionSourceLease{ownedTransferredOne, ownedTransferredTwo} {
		if got := source.state.state.Load(); got != privateB4SourceReleased {
			t.Fatalf("ASSERT_B4_COMMON_RELEASE_RELEASES_OWNED_TRANSFERRED source=%d got=%d want=%d", i, got, privateB4SourceReleased)
		}
	}
	if slots := owner.privateB4LeaseCountLocked(); slots != 0 || owner.privateB4Bytes != 0 || reservation.explicitBytes.snapshot().Live != 0 {
		t.Fatalf("ASSERT_B4_COMMON_RELEASE_CLEARS_CHARGE_ONCE slots=%d bytes=%d explicit=%+v", slots, owner.privateB4Bytes, reservation.explicitBytes.snapshot())
	}

	owner.mu.Lock()
	owner.releasePrivateB4Locked(reservation)
	owner.mu.Unlock()

	if slots := owner.privateB4LeaseCountLocked(); slots != 0 || owner.privateB4Bytes != 0 || reservation.explicitBytes.snapshot().Live != 0 {
		t.Fatalf("ASSERT_B4_COMMON_RELEASE_REPEAT_NO_DOUBLE_DECREMENT slots=%d bytes=%d explicit=%+v", slots, owner.privateB4Bytes, reservation.explicitBytes.snapshot())
	}
	if got := foreignTransferred.state.state.Load(); got != privateB4SourceTransferred {
		t.Fatalf("ASSERT_B4_COMMON_RELEASE_REPEAT_PRESERVES_FOREIGN_TRANSFERRED got=%d want=%d", got, privateB4SourceTransferred)
	}
	if got := ownedHeld.state.state.Load(); got != privateB4SourceHeld {
		t.Fatalf("ASSERT_B4_COMMON_RELEASE_REPEAT_DOES_NOT_RESURRECT_HELD got=%d want=%d", got, privateB4SourceHeld)
	}
	if got := ownedReleased.state.state.Load(); got != privateB4SourceReleased {
		t.Fatalf("ASSERT_B4_COMMON_RELEASE_REPEAT_PRESERVES_RELEASED got=%d want=%d", got, privateB4SourceReleased)
	}
}
