package sessionruntime

import (
	"errors"
	"math"

	"lsp-trace/internal/manageddiagnostic"
	"runtime"
	"unsafe"

	"lsp-trace/internal/session"
)

type privateB4EventBackingOwner struct{ account *privateB4ByteAccountV2 }
type privateB4EventBackingLease struct {
	charge privateB4ByteLeaseV2
	bytes  uint64
}

func (o privateB4EventBackingOwner) ReserveEventBacking(bytes uint64) (manageddiagnostic.EventBackingLease, error) {
	lease, failure := o.account.reserve(bytes)
	if failure != "" {
		return nil, errors.New(string(failure))
	}
	return &privateB4EventBackingLease{charge: lease, bytes: bytes}, nil
}
func (l *privateB4EventBackingLease) EventBackingBytes() uint64 {
	if l == nil {
		return 0
	}
	return l.bytes
}
func (l *privateB4EventBackingLease) ReleaseEventBacking() {
	if l != nil {
		l.charge.release()
	}
}

const (
	managerDiagnosticMaxOwnedBytes  = uint64(32 << 20)
	managerDiagnosticSlots          = 1024
	managerDiagnosticTableBytes     = uint64(managerDiagnosticSlots * 24)
	managerDiagnosticOwnerBytes     = uint64(unsafe.Sizeof(managerDiagnosticOwner{}))
	managerDiagnosticOwnerSelfBytes = managerDiagnosticOwnerBytes - managerDiagnosticTableBytes
)

type managerDiagnosticEntry struct {
	capacity   uint64
	generation uint64
	references uint32
	active     bool
}

type managerDiagnosticOwner struct {
	live            uint64
	cumulative      uint64
	nextGen         uint64
	entries         [managerDiagnosticSlots]managerDiagnosticEntry
	backingReleased bool
}

type managerDiagnosticLease struct {
	owner      *managerDiagnosticOwner
	generation uint64
	slot       uint16
	active     bool
}

func managerDiagnosticRepresentationSupported() bool {
	return runtime.Version() == privateB4QualifiedGoVersion &&
		unsafe.Sizeof(managerDiagnosticEntry{}) == 24 && unsafe.Alignof(managerDiagnosticEntry{}) == 8 &&
		unsafe.Sizeof(managerDiagnosticLease{}) == 24 && unsafe.Alignof(managerDiagnosticLease{}) == 8 &&
		unsafe.Sizeof(managerDiagnosticOwner{}) == 24608 && unsafe.Alignof(managerDiagnosticOwner{}) == 8 &&
		unsafe.Offsetof(managerDiagnosticOwner{}.entries) == 24 && unsafe.Offsetof(managerDiagnosticOwner{}.backingReleased) == 24600
}

func managerDiagnosticFixedBytes() uint64 {
	return managerDiagnosticTableBytes + managerDiagnosticOwnerSelfBytes + managerPrivateDiagnosticHistoryTableBytes()
}

func newManagerDiagnosticOwner() (*managerDiagnosticOwner, session.Failure) {
	fixed := managerDiagnosticFixedBytes()
	if !managerDiagnosticRepresentationSupported() || !privateDiagnosticHistoryRepresentationSupported() || fixed > managerDiagnosticMaxOwnedBytes {
		return nil, session.ResourceExhausted
	}
	o := &managerDiagnosticOwner{live: fixed, cumulative: fixed}
	return o, ""
}

func (o *managerDiagnosticOwner) snapshot() privateB4ByteLedgerSnapshot {
	if o == nil {
		return privateB4ByteLedgerSnapshot{}
	}
	return privateB4ByteLedgerSnapshot{Live: o.live, Cumulative: o.cumulative}
}

func (o *managerDiagnosticOwner) preflightSlot(capacity uint64) (int, session.Failure) {
	if o == nil || o.backingReleased || capacity > managerDiagnosticMaxOwnedBytes || o.live > managerDiagnosticMaxOwnedBytes-capacity || o.cumulative > math.MaxUint64-capacity || o.nextGen == math.MaxUint64 {
		return -1, session.ResourceExhausted
	}
	for i := range o.entries {
		if !o.entries[i].active {
			return i, ""
		}
	}
	return -1, session.ResourceExhausted
}

func (o *managerDiagnosticOwner) installTransferred(slot int, capacity uint64) managerDiagnosticLease {
	o.nextGen++
	o.entries[slot] = managerDiagnosticEntry{capacity: capacity, generation: o.nextGen, references: 1, active: true}
	o.live += capacity
	o.cumulative += capacity
	return managerDiagnosticLease{owner: o, generation: o.nextGen, slot: uint16(slot), active: true}
}

func (o *managerDiagnosticOwner) reserve(capacity uint64) (managerDiagnosticLease, session.Failure) {
	slot, failure := o.preflightSlot(capacity)
	if failure != "" {
		return managerDiagnosticLease{}, failure
	}
	return o.installTransferred(slot, capacity), ""
}

func (l *managerDiagnosticLease) entry() *managerDiagnosticEntry {
	if l == nil || !l.active || l.owner == nil || int(l.slot) >= len(l.owner.entries) {
		return nil
	}
	e := &l.owner.entries[l.slot]
	if !e.active || e.generation != l.generation {
		return nil
	}
	return e
}
func (l *managerDiagnosticLease) release() {
	if l == nil || !l.active {
		return
	}
	l.active = false
	e := l.entryAfterDeactivate()
	if e == nil {
		return
	}
	l.owner.live -= e.capacity
	*e = managerDiagnosticEntry{}
}
func (l *managerDiagnosticLease) entryAfterDeactivate() *managerDiagnosticEntry {
	if l == nil || l.owner == nil || int(l.slot) >= len(l.owner.entries) {
		return nil
	}
	e := &l.owner.entries[l.slot]
	if !e.active || e.generation != l.generation {
		return nil
	}
	return e
}
func (o *managerDiagnosticOwner) releaseTableBacking() bool {
	if o == nil || o.backingReleased || o.live != managerDiagnosticFixedBytes() {
		return false
	}
	for i := range o.entries {
		if o.entries[i].active {
			return false
		}
	}
	o.live = 0
	o.backingReleased = true
	return true
}
