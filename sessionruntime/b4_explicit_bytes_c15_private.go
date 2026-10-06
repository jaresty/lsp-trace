package sessionruntime

import (
	"encoding/json"
	"math"
	"runtime"
	"sync"
	"unsafe"

	"lsp-trace/internal/session"
)

const (
	privateB4MaxOwnedBytes = uint64(32 << 20)
	privateB4ByteSlots     = 1024
)

type privateB4ByteLedgerSnapshot struct {
	Live       uint64
	Cumulative uint64
}

type privateB4ByteEntry struct {
	capacity   uint64
	generation uint64
	references uint32
	active     bool
}

type privateB4ByteLedger struct {
	live       uint64
	cumulative uint64
	nextGen    uint64
	entries    [privateB4ByteSlots]privateB4ByteEntry
}

type privateB4ByteLease struct {
	ledger     *privateB4ByteLedger
	slot       uint16
	generation uint64
	active     bool
}

// privateB4ByteLedgerV2 preserves the legacy 1,024-entry general ledger and
// adds one dedicated source-descriptor entry that general reservations cannot
// address. The complete 1,025-entry backing is a permanent live charge until
// terminal cleanup.
type privateB4ByteLedgerV2 struct {
	general           privateB4ByteLedger
	descriptor        privateB4ByteEntry
	descriptorNextGen uint64
}

type privateB4DescriptorLease struct {
	ledger     *privateB4ByteLedgerV2
	generation uint64
	active     bool
}

const privateB4ByteLedgerV2TableBytes = uint64((privateB4ByteSlots + 1) * 24)

func privateB4ByteLedgerV2RepresentationSupported() bool {
	return runtime.Version() == privateB4QualifiedGoVersion && runtime.GOOS == "darwin" && runtime.GOARCH == "arm64" &&
		unsafe.Sizeof(privateB4ByteEntry{}) == 24 && unsafe.Alignof(privateB4ByteEntry{}) == 8 &&
		unsafe.Sizeof(privateB4ByteLedger{}) == 24600 && unsafe.Alignof(privateB4ByteLedger{}) == 8 &&
		unsafe.Sizeof(privateB4ByteLedgerV2{}) == 24632 && unsafe.Alignof(privateB4ByteLedgerV2{}) == 8
}

func newPrivateB4ByteLedgerV2() (*privateB4ByteLedgerV2, session.Failure) {
	if !privateB4ByteLedgerV2RepresentationSupported() || privateB4ByteLedgerV2TableBytes > privateB4MaxOwnedBytes {
		return nil, session.ResourceExhausted
	}
	ledger := &privateB4ByteLedgerV2{}
	ledger.general.live = privateB4ByteLedgerV2TableBytes
	ledger.general.cumulative = privateB4ByteLedgerV2TableBytes
	return ledger, ""
}

func (l *privateB4ByteLedgerV2) reserveDescriptor(capacity uint64) (privateB4DescriptorLease, session.Failure) {
	if l == nil || l.descriptor.active || capacity > privateB4MaxOwnedBytes ||
		l.general.live > privateB4MaxOwnedBytes-capacity || l.descriptorNextGen == math.MaxUint64 {
		return privateB4DescriptorLease{}, session.ResourceExhausted
	}
	l.descriptorNextGen++
	l.descriptor = privateB4ByteEntry{capacity: capacity, generation: l.descriptorNextGen, references: 1, active: true}
	l.general.live += capacity
	return privateB4DescriptorLease{ledger: l, generation: l.descriptorNextGen, active: true}, ""
}

func (h *privateB4DescriptorLease) release() {
	if h == nil || !h.active {
		return
	}
	h.active = false
	if h.ledger == nil || !h.ledger.descriptor.active || h.ledger.descriptor.generation != h.generation {
		return
	}
	h.ledger.general.live -= h.ledger.descriptor.capacity
	h.ledger.descriptor = privateB4ByteEntry{}
}

func (l *privateB4ByteLedgerV2) releaseTableBacking() bool {
	if l == nil || l.descriptor.active || l.general.live != privateB4ByteLedgerV2TableBytes {
		return false
	}
	for i := range l.general.entries {
		if l.general.entries[i].active {
			return false
		}
	}
	l.general.live = 0
	return true
}

// privateB4ByteAccountV2 is the versioned transaction owner. It deliberately
// wraps, rather than changes, the frozen legacy ledger and lease layouts.
type privateB4ByteAccountV2 struct {
	mu                sync.Mutex
	table             privateB4ByteLedgerV2
	terminalRequested bool
	backingReleased   bool
}

type privateB4ByteLeaseV2 struct {
	account *privateB4ByteAccountV2
	lease   privateB4ByteLease
}

type privateB4DescriptorLeaseV2 struct {
	account *privateB4ByteAccountV2
	lease   privateB4DescriptorLease
}

func newPrivateB4ByteAccountV2() (*privateB4ByteAccountV2, session.Failure) {
	table, failure := newPrivateB4ByteLedgerV2()
	if failure != "" {
		return nil, failure
	}
	return &privateB4ByteAccountV2{table: *table}, ""
}

func (a *privateB4ByteAccountV2) reserve(capacity uint64) (privateB4ByteLeaseV2, session.Failure) {
	if a == nil {
		return privateB4ByteLeaseV2{}, session.ResourceExhausted
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.terminalRequested || a.backingReleased {
		return privateB4ByteLeaseV2{}, session.ResourceExhausted
	}
	lease, failure := a.table.general.reserve(capacity)
	if failure != "" {
		return privateB4ByteLeaseV2{}, failure
	}
	return privateB4ByteLeaseV2{account: a, lease: lease}, ""
}

func (a *privateB4ByteAccountV2) reserveMany(capacities []uint64, dst []privateB4ByteLeaseV2) (int, session.Failure) {
	if a == nil || len(capacities) > privateB4ByteSlots || len(dst) < len(capacities) {
		return 0, session.ResourceExhausted
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.terminalRequested || a.backingReleased {
		return 0, session.ResourceExhausted
	}
	var tmp [privateB4ByteSlots]privateB4ByteLease
	count, failure := a.table.general.reserveMany(capacities, tmp[:len(capacities)])
	if failure != "" {
		return 0, failure
	}
	for i := 0; i < count; i++ {
		dst[i] = privateB4ByteLeaseV2{account: a, lease: tmp[i]}
	}
	return count, ""
}

func (h privateB4ByteLeaseV2) alias() privateB4ByteLeaseV2 {
	if h.account == nil {
		return privateB4ByteLeaseV2{}
	}
	h.account.mu.Lock()
	defer h.account.mu.Unlock()
	if h.account.terminalRequested || h.account.backingReleased {
		return privateB4ByteLeaseV2{}
	}
	lease := h.lease.alias()
	if !lease.active {
		return privateB4ByteLeaseV2{}
	}
	return privateB4ByteLeaseV2{account: h.account, lease: lease}
}

func (h *privateB4ByteLeaseV2) grow(capacity uint64) (privateB4ByteLeaseV2, uint64, session.Failure) {
	if h == nil || h.account == nil {
		return privateB4ByteLeaseV2{}, 0, session.ResourceExhausted
	}
	a := h.account
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.terminalRequested || a.backingReleased {
		return privateB4ByteLeaseV2{}, a.table.general.live, session.ResourceExhausted
	}
	lease, peak, failure := h.lease.grow(capacity)
	if failure != "" {
		return privateB4ByteLeaseV2{}, peak, failure
	}
	return privateB4ByteLeaseV2{account: a, lease: lease}, peak, ""
}

func (h *privateB4ByteLeaseV2) shrinkExact(capacity uint64) bool {
	if h == nil || h.account == nil {
		return false
	}
	a := h.account
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.terminalRequested || a.backingReleased {
		return false
	}
	return h.lease.shrinkExact(capacity)
}

func (h *privateB4ByteLeaseV2) transfer() privateB4ByteLeaseV2 {
	if h == nil || h.account == nil {
		return privateB4ByteLeaseV2{}
	}
	a := h.account
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.terminalRequested || a.backingReleased {
		return privateB4ByteLeaseV2{}
	}
	lease := h.lease.transfer()
	if !lease.active {
		return privateB4ByteLeaseV2{}
	}
	return privateB4ByteLeaseV2{account: a, lease: lease}
}

func (h *privateB4ByteLeaseV2) Release() { h.release() }

func (a *privateB4ByteAccountV2) transferDiagnosticBacking(source *privateB4ByteLeaseV2, destination *managerDiagnosticOwner) (managerDiagnosticLease, session.Failure) {
	if a == nil || source == nil || source.account != a || destination == nil {
		return managerDiagnosticLease{}, session.ResourceExhausted
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	entry := source.lease.entry()
	if !source.lease.active || entry == nil {
		return managerDiagnosticLease{}, session.ResourceExhausted
	}
	if entry.references != 1 {
		return managerDiagnosticLease{}, session.ResourceExhausted
	}
	capacity := entry.capacity
	slot, failure := destination.preflightSlot(capacity)
	if failure != "" {
		return managerDiagnosticLease{}, failure
	}
	retained := destination.installTransferred(slot, capacity)
	a.table.general.live -= capacity
	*entry = privateB4ByteEntry{}
	source.lease.active = false
	a.maybeReleaseBackingLocked()
	return retained, ""
}

func (a *privateB4ByteAccountV2) transferDiagnosticBackingPair(first, second *privateB4ByteLeaseV2, destination *managerDiagnosticOwner) (managerDiagnosticLease, session.Failure) {
	if a == nil || first == nil || first.account != a || destination == nil || (second != nil && second.account != nil && second.account != a) {
		return managerDiagnosticLease{}, session.ResourceExhausted
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	entries := [2]*privateB4ByteEntry{first.lease.entry(), nil}
	if !first.lease.active || entries[0] == nil || entries[0].references != 1 {
		return managerDiagnosticLease{}, session.ResourceExhausted
	}
	capacity := entries[0].capacity
	if second != nil && second.account != nil {
		entries[1] = second.lease.entry()
		if !second.lease.active || entries[1] == nil || entries[1].references != 1 || capacity > math.MaxUint64-entries[1].capacity {
			return managerDiagnosticLease{}, session.ResourceExhausted
		}
		capacity += entries[1].capacity
	}
	slot, failure := destination.preflightSlot(capacity)
	if failure != "" {
		return managerDiagnosticLease{}, failure
	}
	retained := destination.installTransferred(slot, capacity)
	for i, entry := range entries {
		if entry == nil {
			continue
		}
		a.table.general.live -= entry.capacity
		*entry = privateB4ByteEntry{}
		if i == 0 {
			first.lease.active = false
		} else {
			second.lease.active = false
		}
	}
	a.maybeReleaseBackingLocked()
	return retained, ""
}

func (a *privateB4ByteAccountV2) reserveDescriptor(capacity uint64) (privateB4DescriptorLeaseV2, session.Failure) {
	if a == nil {
		return privateB4DescriptorLeaseV2{}, session.ResourceExhausted
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.terminalRequested || a.backingReleased {
		return privateB4DescriptorLeaseV2{}, session.ResourceExhausted
	}
	lease, failure := a.table.reserveDescriptor(capacity)
	if failure != "" {
		return privateB4DescriptorLeaseV2{}, failure
	}
	return privateB4DescriptorLeaseV2{account: a, lease: lease}, ""
}

func (h *privateB4ByteLeaseV2) release() {
	if h == nil || h.account == nil {
		return
	}
	a := h.account
	a.mu.Lock()
	h.lease.release()
	a.maybeReleaseBackingLocked()
	a.mu.Unlock()
}

func (h *privateB4DescriptorLeaseV2) release() {
	if h == nil || h.account == nil {
		return
	}
	a := h.account
	a.mu.Lock()
	h.lease.release()
	a.maybeReleaseBackingLocked()
	a.mu.Unlock()
}

func (a *privateB4ByteAccountV2) requestTerminalRelease() {
	if a == nil {
		return
	}
	a.mu.Lock()
	a.terminalRequested = true
	a.maybeReleaseBackingLocked()
	a.mu.Unlock()
}

func (a *privateB4ByteAccountV2) maybeReleaseBackingLocked() {
	if a.terminalRequested && !a.backingReleased && a.table.releaseTableBacking() {
		a.backingReleased = true
	}
}

func (a *privateB4ByteAccountV2) snapshot() privateB4ByteLedgerSnapshot {
	if a == nil {
		return privateB4ByteLedgerSnapshot{}
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.table.general.snapshot()
}

const (
	privateB4SourceIngressSlots      = 102927
	privateB4SourceIngressEntryBytes = uint64(32)
	privateB4SourceIngressStateBytes = uint64(294)
	privateB4SourceIngressTableBytes = uint64(privateB4SourceIngressSlots) * privateB4SourceIngressEntryBytes
	privateB4SourceIngressTotalBytes = privateB4SourceIngressTableBytes + uint64(privateB4SourceIngressSlots)*privateB4SourceIngressStateBytes
)

type privateB4SourceIngressEntry struct {
	capacity   uint64
	generation uint64
	state      *privateB4SourceLease
	active     bool
}

type privateB4SourceIngressOwner struct {
	entries    []privateB4SourceIngressEntry
	live       uint64
	cumulative uint64
	nextGen    uint64
	nextFree   int
}

type privateB4SourceIngressLease struct {
	owner      *privateB4SourceIngressOwner
	generation uint64
	slot       uint32
	active     bool
}

func newPrivateB4SourceIngressOwner() (*privateB4SourceIngressOwner, session.Failure) {
	if !privateB4ByteLedgerV2RepresentationSupported() || unsafe.Sizeof(privateB4SourceIngressEntry{}) != 32 || unsafe.Alignof(privateB4SourceIngressEntry{}) != 8 ||
		unsafe.Sizeof(privateB4SourceIngressLease{}) != 24 || unsafe.Alignof(privateB4SourceIngressLease{}) != 8 ||
		unsafe.Sizeof(privateB4SourceLease{}) != 152 || unsafe.Alignof(privateB4SourceLease{}) != 8 || privateB4SourceIngressTotalBytes > privateB4MaxOwnedBytes {
		return nil, session.ResourceExhausted
	}
	return &privateB4SourceIngressOwner{entries: make([]privateB4SourceIngressEntry, privateB4SourceIngressSlots), live: privateB4SourceIngressTableBytes, cumulative: privateB4SourceIngressTableBytes}, ""
}

func (o *privateB4SourceIngressOwner) releaseTableBacking() {
	if o == nil || o.live == 0 {
		return
	}
	for i := range o.entries {
		entry := &o.entries[i]
		if entry.active && entry.state != nil && entry.state.state.CompareAndSwap(privateB4SourceHeld, privateB4SourceReleased) {
			entry.state.source.Bytes = nil
		}
		*entry = privateB4SourceIngressEntry{}
	}
	o.entries = nil
	o.live = 0
	o.nextFree = 0
}

func (o *privateB4SourceIngressOwner) snapshot() privateB4ByteLedgerSnapshot {
	if o == nil {
		return privateB4ByteLedgerSnapshot{}
	}
	return privateB4ByteLedgerSnapshot{Live: o.live, Cumulative: o.cumulative}
}

func (o *privateB4SourceIngressOwner) reserve(capacity uint64, state *privateB4SourceLease) (privateB4SourceIngressLease, session.Failure) {
	if o == nil || capacity > privateB4MaxOwnedBytes || o.live > privateB4MaxOwnedBytes-capacity || o.cumulative > math.MaxUint64-capacity || o.nextGen == math.MaxUint64 {
		return privateB4SourceIngressLease{}, session.ResourceExhausted
	}
	for scanned := 0; scanned < len(o.entries); scanned++ {
		i := (o.nextFree + scanned) % len(o.entries)
		if !o.entries[i].active {
			o.nextGen++
			o.entries[i] = privateB4SourceIngressEntry{capacity: capacity, generation: o.nextGen, state: state, active: true}
			o.nextFree = (i + 1) % len(o.entries)
			o.live += capacity
			o.cumulative += capacity
			return privateB4SourceIngressLease{owner: o, generation: o.nextGen, slot: uint32(i), active: true}, ""
		}
	}
	return privateB4SourceIngressLease{}, session.ResourceExhausted
}

func (h *privateB4SourceIngressLease) entry() *privateB4SourceIngressEntry {
	if h == nil || !h.active || h.owner == nil || int(h.slot) >= len(h.owner.entries) {
		return nil
	}
	e := &h.owner.entries[h.slot]
	if !e.active || e.generation != h.generation {
		return nil
	}
	return e
}

func (h *privateB4SourceIngressLease) release() {
	if h == nil || !h.active {
		return
	}
	e := h.entry()
	h.active = false
	if e == nil {
		return
	}
	h.owner.live -= e.capacity
	*e = privateB4SourceIngressEntry{}
	if int(h.slot) < h.owner.nextFree {
		h.owner.nextFree = int(h.slot)
	}
}

// transferStates transfers an already deduplicated admission set without
// allocating a temporary handle array or consuming a general-ledger slot.
func (o *privateB4SourceIngressOwner) transferStates(states map[*privateB4SourceLease]struct{}, dst *privateB4ByteAccountV2) (privateB4DescriptorLeaseV2, session.Failure) {
	var total uint64
	for state := range states {
		if state == nil || state.ingress.owner != o || state.ingress.entry() == nil || state.ingress.entry().state != state {
			return privateB4DescriptorLeaseV2{}, session.ResourceExhausted
		}
		capacity := state.ingress.entry().capacity
		if total > privateB4MaxOwnedBytes-capacity {
			return privateB4DescriptorLeaseV2{}, session.ResourceExhausted
		}
		total += capacity
	}
	descriptor, failure := dst.reserveDescriptor(total)
	if failure != "" {
		return privateB4DescriptorLeaseV2{}, failure
	}
	for state := range states {
		entry := state.ingress.entry()
		o.live -= entry.capacity
		*entry = privateB4SourceIngressEntry{}
		state.ingress.active = false
	}
	return descriptor, ""
}

func (o *privateB4SourceIngressOwner) transferMany(handles []*privateB4SourceIngressLease, dst *privateB4ByteAccountV2) (privateB4DescriptorLeaseV2, session.Failure) {
	type identity struct {
		slot       uint32
		generation uint64
	}
	seen := make(map[identity]struct{}, min(len(handles), privateB4SourceIngressSlots))
	var total uint64
	for _, h := range handles {
		if h == nil || h.owner != o || h.entry() == nil {
			return privateB4DescriptorLeaseV2{}, session.ResourceExhausted
		}
		key := identity{slot: h.slot, generation: h.generation}
		if _, duplicate := seen[key]; duplicate {
			continue
		}
		seen[key] = struct{}{}
		if total > privateB4MaxOwnedBytes-h.entry().capacity {
			return privateB4DescriptorLeaseV2{}, session.ResourceExhausted
		}
		total += h.entry().capacity
	}
	descriptor, failure := dst.reserveDescriptor(total)
	if failure != "" {
		return privateB4DescriptorLeaseV2{}, failure
	}
	clear(seen)
	for _, h := range handles {
		key := identity{slot: h.slot, generation: h.generation}
		if _, duplicate := seen[key]; duplicate {
			h.active = false
			continue
		}
		seen[key] = struct{}{}
		e := h.entry()
		o.live -= e.capacity
		*e = privateB4SourceIngressEntry{}
		h.active = false
	}
	return descriptor, ""
}

func newPrivateB4ByteLedger() *privateB4ByteLedger { return &privateB4ByteLedger{} }

// copyPrivateB4Params reserves the independent Manager-owned ingress copy
// before allocation. The caller owns the returned temporary lease for exactly
// as long as the copied params remain live.
func copyPrivateB4Params(account *privateB4ByteAccountV2, params json.RawMessage) (json.RawMessage, privateB4ByteLeaseV2, session.Failure) {
	lease, failure := account.reserve(uint64(len(params)))
	if failure != "" {
		return nil, privateB4ByteLeaseV2{}, failure
	}
	return append(json.RawMessage(nil), params...), lease, ""
}

// reservePrivateB4OutboundTransport charges the private path's independent
// exact-capacity encoded body and canonical header allocations. The caller must
// reserve before either allocation and release the temporary on every exit.
func (a *privateB4ByteAccountV2) reservePrivateB4OutboundTransport(bodyBytes int) (privateB4ByteLeaseV2, session.Failure) {
	if bodyBytes < 0 {
		return privateB4ByteLeaseV2{}, session.ResourceExhausted
	}
	frameBytes := canonicalRequestFrameBytes(bodyBytes)
	if frameBytes < int64(bodyBytes) {
		return privateB4ByteLeaseV2{}, session.ResourceExhausted
	}
	return a.reserve(uint64(frameBytes))
}

func (l *privateB4ByteLedger) snapshot() privateB4ByteLedgerSnapshot {
	if l == nil {
		return privateB4ByteLedgerSnapshot{}
	}
	return privateB4ByteLedgerSnapshot{Live: l.live, Cumulative: l.cumulative}
}

func (l *privateB4ByteLedger) reserve(capacity uint64) (privateB4ByteLease, session.Failure) {
	if l == nil || capacity > privateB4MaxOwnedBytes || l.live > privateB4MaxOwnedBytes-capacity || l.cumulative > math.MaxUint64-capacity {
		return privateB4ByteLease{}, session.ResourceExhausted
	}
	slot := -1
	for i := range l.entries {
		if !l.entries[i].active {
			slot = i
			break
		}
	}
	if slot < 0 || l.nextGen == math.MaxUint64 {
		return privateB4ByteLease{}, session.ResourceExhausted
	}
	generation := l.nextGen + 1
	l.entries[slot] = privateB4ByteEntry{capacity: capacity, generation: generation, references: 1, active: true}
	l.nextGen = generation
	l.live += capacity
	l.cumulative += capacity
	return privateB4ByteLease{ledger: l, slot: uint16(slot), generation: generation, active: true}, ""
}

// reserveMany preflights the complete fixed-table mutation so refusal leaves
// both live and cumulative observations unchanged.
func (l *privateB4ByteLedger) reserveMany(capacities []uint64, dst []privateB4ByteLease) (int, session.Failure) {
	if l == nil {
		return 0, session.ResourceExhausted
	}
	count := 0
	var total uint64
	for _, capacity := range capacities {
		if capacity == 0 {
			continue
		}
		if capacity > privateB4MaxOwnedBytes || total > privateB4MaxOwnedBytes-capacity || l.cumulative > math.MaxUint64-capacity-total {
			return 0, session.ResourceExhausted
		}
		total += capacity
		count++
	}
	if count > len(dst) || l.live > privateB4MaxOwnedBytes-total || uint64(count) > math.MaxUint64-l.nextGen {
		return 0, session.ResourceExhausted
	}
	free := 0
	for i := range l.entries {
		if !l.entries[i].active {
			free++
		}
	}
	if free < count {
		return 0, session.ResourceExhausted
	}
	written := 0
	for _, capacity := range capacities {
		if capacity == 0 {
			continue
		}
		lease, failure := l.reserve(capacity)
		if failure != "" {
			panic("private B4 byte-ledger preflight diverged")
		}
		dst[written] = lease
		written++
	}
	return written, ""
}

func (h privateB4ByteLease) alias() privateB4ByteLease {
	if entry := h.entry(); entry != nil && entry.references < math.MaxUint32 {
		entry.references++
		return privateB4ByteLease{ledger: h.ledger, slot: h.slot, generation: h.generation, active: true}
	}
	return privateB4ByteLease{}
}

func (h *privateB4ByteLease) release() {
	if h == nil || !h.active {
		return
	}
	h.active = false
	entry := h.entry()
	if entry == nil || entry.references == 0 {
		return
	}
	entry.references--
	if entry.references == 0 {
		h.ledger.live -= entry.capacity
		*entry = privateB4ByteEntry{}
	}
}

func (h *privateB4ByteLease) grow(capacity uint64) (privateB4ByteLease, uint64, session.Failure) {
	if h == nil || !h.active || h.entry() == nil {
		return privateB4ByteLease{}, 0, session.ResourceExhausted
	}
	grown, failure := h.ledger.reserve(capacity)
	if failure != "" {
		return privateB4ByteLease{}, h.ledger.live, failure
	}
	peak := h.ledger.live
	h.release()
	return grown, peak, ""
}

func (h *privateB4ByteLease) shrinkExact(capacity uint64) bool {
	entry := h.entry()
	if h == nil || !h.active || entry == nil || capacity > entry.capacity {
		return false
	}
	h.ledger.live -= entry.capacity - capacity
	entry.capacity = capacity
	return true
}

func (h *privateB4ByteLease) transfer() privateB4ByteLease {
	entry := h.entry()
	if h == nil || !h.active || entry == nil || h.ledger.nextGen == math.MaxUint64 {
		return privateB4ByteLease{}
	}
	h.ledger.nextGen++
	entry.generation = h.ledger.nextGen
	h.active = false
	return privateB4ByteLease{ledger: h.ledger, slot: h.slot, generation: entry.generation, active: true}
}

func (h *privateB4ByteLease) entry() *privateB4ByteEntry {
	if h == nil || h.ledger == nil || int(h.slot) >= len(h.ledger.entries) {
		return nil
	}
	entry := &h.ledger.entries[h.slot]
	if !entry.active || entry.generation != h.generation {
		return nil
	}
	return entry
}
