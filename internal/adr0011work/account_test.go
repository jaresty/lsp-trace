package adr0011work

import (
	"errors"
	"math"
	"testing"
)

func TestPrivateWorkAccountStartsZeroAndFreezesTerminally(t *testing.T) {
	identity := occurrenceIdentity("occurrence-1")
	account := newPrivateWorkAccount(identity)
	if got := account.snapshot(); got != (privateWorkSnapshot{identity: identity}) {
		t.Fatalf("new account snapshot = %#v, want exact identity and six zero counters", got)
	}

	if err := account.charge(identity, privateWorkDelta{
		bytesScanned:               1,
		messagesProcessed:          2,
		objectsMaterialized:        3,
		eventsEmitted:              4,
		documentsAcquired:          5,
		logicalBufferBytesReserved: 6,
	}); err != nil {
		t.Fatalf("charge: %v", err)
	}
	want := privateWorkSnapshot{
		identity:                   identity,
		bytesScanned:               1,
		messagesProcessed:          2,
		objectsMaterialized:        3,
		eventsEmitted:              4,
		documentsAcquired:          5,
		logicalBufferBytesReserved: 6,
	}
	if got := account.seal(); got != want {
		t.Fatalf("sealed snapshot = %#v, want %#v", got, want)
	}
	if err := account.charge(identity, privateWorkDelta{bytesScanned: 1}); !errors.Is(err, errPrivateWorkTerminal) {
		t.Fatalf("post-terminal charge error = %v, want %v", err, errPrivateWorkTerminal)
	}
	if got := account.snapshot(); got != want {
		t.Fatalf("post-terminal snapshot changed: got %#v want %#v", got, want)
	}
	if got := newPrivateWorkAccount(occurrenceIdentity("occurrence-2")).snapshot(); got != (privateWorkSnapshot{identity: occurrenceIdentity("occurrence-2")}) {
		t.Fatalf("new account inherited prior work: %#v", got)
	}
}

func TestPrivateWorkAccountCheckedOverflowHasZeroRelevantEffect(t *testing.T) {
	identity := occurrenceIdentity("occurrence-overflow")
	account := newPrivateWorkAccount(identity)
	if err := account.charge(identity, privateWorkDelta{bytesScanned: math.MaxUint64}); err != nil {
		t.Fatalf("priming charge: %v", err)
	}
	before := account.snapshot()
	if err := account.charge(identity, privateWorkDelta{bytesScanned: 1, eventsEmitted: 9}); !errors.Is(err, errPrivateWorkOverflow) {
		t.Fatalf("overflow error = %v, want %v", err, errPrivateWorkOverflow)
	}
	if got := account.snapshot(); got != before {
		t.Fatalf("overflow had an effect before refusal: got %#v want %#v", got, before)
	}
	if err := account.charge(identity, privateWorkDelta{messagesProcessed: 1}); !errors.Is(err, errPrivateWorkTerminal) {
		t.Fatalf("overflow did not seal failure: %v", err)
	}
}

func TestPrivateWorkAccountIdentityMismatchHasZeroEffect(t *testing.T) {
	identity := occurrenceIdentity("occurrence-owned")
	account := newPrivateWorkAccount(identity)
	before := account.snapshot()
	if err := account.charge(occurrenceIdentity("occurrence-other"), privateWorkDelta{
		bytesScanned:               1,
		messagesProcessed:          1,
		objectsMaterialized:        1,
		eventsEmitted:              1,
		documentsAcquired:          1,
		logicalBufferBytesReserved: 1,
	}); !errors.Is(err, errPrivateWorkIdentity) {
		t.Fatalf("identity mismatch error = %v, want %v", err, errPrivateWorkIdentity)
	}
	if got := account.snapshot(); got != before {
		t.Fatalf("identity mismatch changed account: got %#v want %#v", got, before)
	}
}
