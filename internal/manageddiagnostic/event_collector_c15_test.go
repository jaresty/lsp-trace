package manageddiagnostic

import (
	"errors"
	"testing"
	"unsafe"
)

type testEventBackingLease struct {
	bytes    uint64
	released bool
}

func (l *testEventBackingLease) EventBackingBytes() uint64 { return l.bytes }
func (l *testEventBackingLease) ReleaseEventBacking()      { l.released = true }

type testEventBackingOwner struct {
	want   uint64
	refuse bool
	lease  *testEventBackingLease
}

func (o *testEventBackingOwner) ReserveEventBacking(bytes uint64) (EventBackingLease, error) {
	o.want = bytes
	if o.refuse {
		return nil, errors.New("refused")
	}
	o.lease = &testEventBackingLease{bytes: bytes}
	return o.lease, nil
}

func TestEventCollectorOwnedBackingExactCapacityAndDetach(t *testing.T) {
	if !EventRepresentationSupported() {
		t.Skip("exact event representation is not qualified on this platform")
	}
	if EventElementBytes() != 40 || unsafe.Alignof(Event{}) != 8 || unsafe.Offsetof(Event{}.Sequence) != 0 || unsafe.Offsetof(Event{}.ElapsedNS) != 8 || unsafe.Offsetof(Event{}.Kind) != 16 || unsafe.Offsetof(Event{}.Code) != 18 || unsafe.Offsetof(Event{}.Count) != 24 || unsafe.Offsetof(Event{}.Flag) != 32 || unsafe.Sizeof(EventSnapshot{}) != 48 || unsafe.Alignof(EventSnapshot{}) != 8 || unsafe.Offsetof(EventSnapshot{}.Events) != 0 || unsafe.Offsetof(EventSnapshot{}.Omitted) != 24 || unsafe.Offsetof(EventSnapshot{}.Late) != 32 || unsafe.Offsetof(EventSnapshot{}.Closed) != 40 {
		t.Fatal("ASSERT_C15_EVENT_REPRESENTATION")
	}
	owner := &testEventBackingOwner{}
	collector, err := NewOwnedEventCollector(3, nil, owner)
	want := uint64(3) * uint64(unsafe.Sizeof(Event{}))
	if err != nil || owner.want != want || cap(collector.events) != 3 || len(collector.events) != 0 {
		t.Fatalf("ASSERT_C15_EVENT_BACKING_EXACT err=%v reserved=%d cap=%d", err, owner.want, cap(collector.events))
	}
	if _, ok := collector.DetachEventBacking(); ok {
		t.Fatal("ASSERT_C15_EVENT_BACKING_OPEN_DETACH")
	}
	collector.Record(1, 0, false)
	collector.Terminal(9)
	lease, ok := collector.DetachEventBacking()
	if !ok || lease != owner.lease || lease.EventBackingBytes() != want {
		t.Fatal("ASSERT_C15_EVENT_BACKING_TERMINAL_DETACH")
	}
	if _, replay := collector.DetachEventBacking(); replay {
		t.Fatal("ASSERT_C15_EVENT_BACKING_DETACH_REPLAY")
	}
	snapshot := collector.Snapshot()
	snapshot.Events[0].Code = 99
	if collector.Snapshot().Events[0].Code == 99 {
		t.Fatal("ASSERT_C15_EVENT_SNAPSHOT_INDEPENDENT")
	}
}

func TestEventCollectorOwnedBackingRefusalPrecedesAllocation(t *testing.T) {
	if !EventRepresentationSupported() {
		t.Skip("exact event representation is not qualified on this platform")
	}
	owner := &testEventBackingOwner{refuse: true}
	collector, err := NewOwnedEventCollector(3, nil, owner)
	if err == nil || collector != nil || owner.want != uint64(3)*uint64(unsafe.Sizeof(Event{})) {
		t.Fatalf("ASSERT_C15_EVENT_BACKING_REFUSAL collector=%v err=%v reserved=%d", collector, err, owner.want)
	}
}
