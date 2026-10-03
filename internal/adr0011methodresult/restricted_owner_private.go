package adr0011methodresult

import (
	"errors"
	"sync"
	"unsafe"
)

// Private and deliberately unwired. The fixed-region reservation below proves
// custody and accounting; it does not claim the existing dynamic JSON bridge is
// a bounded producer of those regions.

const (
	restrictedGenerationBytes                 uint64 = 16_777_216
	restrictedGlobalBytes                     uint64 = 67_108_864
	restrictedGenerationSlots                        = 4
	restrictedGlobalSlots                            = 16
	restrictedRegistrationRows                       = 32
	restrictedCandidateMax                           = 64
	restrictedManualPeak                      uint64 = 4_206_170
	restrictedStringBytes                     uint64 = 598_912
	restrictedConstructionMax                 uint64 = 1_207_552
	restrictedOutputSteadyMax                 uint64 = 608_640
	restrictedManualBackingCount                     = 30
	restrictedProvenanceRawBytes              uint64 = 2_097_152
	restrictedProvenanceRecordBytes           uint64 = 12_288
	restrictedProvenanceHandleBytes           uint64 = 96
	restrictedProvenanceCandidateControlBytes        = uint64(restrictedCandidateMax) * uint64(unsafe.Sizeof(DefinitionCandidate{}))
	restrictedProvenanceCharge                uint64 = restrictedProvenanceRawBytes + restrictedProvenanceRecordBytes + 2*restrictedProvenanceHandleBytes + restrictedProvenanceCandidateControlBytes
)

var (
	errRestrictedBusy       = errors.New("restricted owner busy")
	errRestrictedBound      = errors.New("restricted owner bound")
	errRestrictedIdentity   = errors.New("restricted owner identity")
	errRestrictedState      = errors.New("restricted owner state")
	errRestrictedIncomplete = errors.New("restricted owner incomplete")
)

var restrictedManualCaps = [restrictedManualBackingCount]uint32{
	4096, 65537, 131072, 131072, 131072, 65536, 65536, 65537, 262144, 262144,
	262144, 196608, 196608, 4096, 131072, 512, 131072, 2048, 262144, 8192,
	524288, 524288, 87384, 262144, 196608, 16384, 131072, 2560, 1024, 82176,
}

type restrictedRecipient struct{ ID uint64 }
type restrictedBackingKind uint8

const (
	restrictedCandidatesBacking restrictedBackingKind = iota + 1
	restrictedRangesBacking
	restrictedStringsBacking
)

type restrictedBackingID struct {
	Owner, Session, Generation, Attempt, Serial, Version uint64
	Kind                                                 restrictedBackingKind
}

type restrictedTransfer struct {
	Owner, Session, Generation, Attempt, Version uint64
	Backings                                     [3]restrictedBackingID
}

type restrictedViewToken struct {
	Owner, Session, Generation, Attempt, Version uint64
	Recipient                                    restrictedRecipient
	Row, RowVersion                              uint32
	Backings                                     [3]restrictedBackingID
}

type restrictedView struct {
	output []DefinitionCandidate
	token  restrictedViewToken
}

func (v restrictedView) Output() []DefinitionCandidate { return v.output }

type restrictedRegistration struct {
	version uint32
	used    bool
	closed  bool
	primary bool
	token   restrictedViewToken
}

type restrictedAttempt struct {
	used, outputBuilt, retired, quarantined      bool
	owner, session, generation                   uint64
	attempt, version                             uint64
	regionCharge, processingCharge, outputCharge uint64
	regions                                      [restrictedManualBackingCount][]byte
	regionCaps                                   [restrictedManualBackingCount]uint32
	provenanceRaw, provenanceRecords             []byte
	provenanceState                              uint8
	provenanceFlags                              uint8
	provenanceChronology                         uint8
	provenanceRecordVersion, provenanceEpoch     uint32
	provenanceRawUsed                            uint32
	provenanceRequestOff, provenanceRequestLen   uint32
	provenanceResponseOff, provenanceResponseLen uint32
	provenanceResultOff, provenanceResultLen     uint32
	provenanceBorrowers, provenanceObligations   uint16
	provenanceCandidateCount                     uint16
	provenanceCandidates                         [restrictedCandidateMax]DefinitionCandidate
	output                                       []DefinitionCandidate
	ranges                                       []Range
	pool                                         string
	backings                                     [3]restrictedBackingID
	recipient                                    restrictedRecipient
	acked                                        [3]bool
	rows                                         [restrictedRegistrationRows]restrictedRegistration
	workRegistered, workJoined, workCanceled     uint32
	terminal, retentionDisposed, forgotten       bool
}

type restrictedOwnerManager struct {
	mu          sync.Mutex
	root        *restrictedBudgetRoot
	attempts    [restrictedGlobalSlots]restrictedAttempt
	nextAttempt uint64
	nextBacking uint64
	allocations uint64
}

type restrictedBudgetRoot struct {
	mu                sync.Mutex
	manager           restrictedOwnerManager
	bootstrapped      bool
	globalUsed        uint64
	globalSlots       uint32
	constructorCalls  uint64
	provenanceRaw     [restrictedGlobalSlots][]byte
	provenanceRecords [restrictedGlobalSlots][]byte
}

// U is the complete fixed root including the inline manager, fixed attempt
// table, registration rows, mutexes and counters. Runtime allocator classes,
// GC metadata, stacks, semaphore waiters and RSS are not claimed by this
// logical-owner contract.
const restrictedLogicalControlBytes = uint64(unsafe.Sizeof(restrictedBudgetRoot{}))

type restrictedProcessingReservation struct {
	manager                                      *restrictedOwnerManager
	slot                                         uint8
	Owner, Session, Generation, Attempt, Version uint64
	Capacity                                     uint64
	regions                                      [restrictedManualBackingCount]uint32
}

type restrictedLease struct {
	manager                                      *restrictedOwnerManager
	slot                                         uint8
	Owner, Session, Generation, Attempt, Version uint64
	Transfer                                     restrictedTransfer
}

var (
	restrictedProcessingValueBytes = uint64(unsafe.Sizeof(restrictedProcessingReservation{}))
	restrictedLeaseValueBytes      = uint64(unsafe.Sizeof(restrictedLease{}))
	restrictedViewValueBytes       = uint64(unsafe.Sizeof(restrictedView{}))
)

type restrictedSnapshot struct {
	used, outputBuilt, retired, quarantined  bool
	recipient                                restrictedRecipient
	acked                                    [3]bool
	workRegistered, workJoined, workCanceled uint32
	terminal, retentionDisposed, forgotten   bool
	openRows                                 uint32
	charge                                   uint64
}

type restrictedCaseClaim struct {
	ID                   int
	Exercised, NotProved string
}

func restrictedC1C14Claims() [14]restrictedCaseClaim {
	return [14]restrictedCaseClaim{
		{1, "registered view blocks release", "external reader scheduling"}, {2, "distinct output backings retain charge", "all upstream copies"},
		{3, "cancellation is not join", "process death"}, {4, "exact stale identity refusal", "all callback transports"},
		{5, "row version blocks late reuse", "cross-process delivery"}, {6, "ordinary correspondence survives copy", "dynamic parser conformance"},
		{7, "full transfer identity precedes ACK", "semantic dedup"}, {8, "view requires exact return", "caller language aliasing"},
		{9, "quarantine retains charge", "OS crash recovery"}, {10, "actual headroom precedes allocation", "all production occupancy"},
		{11, "three aliases share charged backings", "arbitrary independent copies"}, {12, "fixed rows refuse overflow", "production reporter membership"},
		{13, "post-reuse stale identity refused", "unbounded history"}, {14, "positive conjunction controls release", "production joins"},
	}
}

func restrictedAddFits(used, add, limit uint64) bool { return used <= limit && add <= limit-used }

func (r *restrictedBudgetRoot) bootstrap(preexisting uint64) (*restrictedOwnerManager, error) {
	if r == nil || !r.mu.TryLock() {
		return nil, errRestrictedBusy
	}
	defer r.mu.Unlock()
	if r.bootstrapped {
		return nil, errRestrictedState
	}
	if !restrictedAddFits(preexisting, restrictedLogicalControlBytes, restrictedGlobalBytes) {
		return nil, errRestrictedBusy
	}
	// The root is caller-preallocated. Charge its full fixed logical storage
	// before initializing the inline manager; no manager allocation precedes it.
	r.globalUsed = preexisting + restrictedLogicalControlBytes
	r.constructorCalls++
	r.manager = restrictedOwnerManager{root: r, nextAttempt: 1, nextBacking: 1}
	r.bootstrapped = true
	return &r.manager, nil
}

func (m *restrictedOwnerManager) tryBoth() bool {
	if m == nil || m.root == nil || !m.root.mu.TryLock() {
		return false
	}
	if !m.mu.TryLock() {
		m.root.mu.Unlock()
		return false
	}
	return true
}
func (m *restrictedOwnerManager) unlockBoth() { m.mu.Unlock(); m.root.mu.Unlock() }

func restrictedSumCaps(caps [restrictedManualBackingCount]uint32) (uint64, bool) {
	var n uint64
	for i, c := range caps {
		if c > restrictedManualCaps[i] {
			return 0, false
		}
		if !restrictedAddFits(n, uint64(c), restrictedManualPeak) {
			return 0, false
		}
		n += uint64(c)
	}
	return n, n > 0
}

func (m *restrictedOwnerManager) reserveProcessing(owner, session, generation uint64) (restrictedProcessingReservation, error) {
	caps := [restrictedManualBackingCount]uint32{}
	for i := range caps {
		caps[i] = 1
	}
	return m.reserveProcessingCaps(owner, session, generation, caps)
}

func (m *restrictedOwnerManager) reserveProcessingCaps(owner, session, generation uint64, caps [restrictedManualBackingCount]uint32) (restrictedProcessingReservation, error) {
	regionCharge, ok := restrictedSumCaps(caps)
	charge := regionCharge + restrictedProcessingValueBytes
	if owner == 0 || session == 0 || generation == 0 || !ok {
		return restrictedProcessingReservation{}, errRestrictedBound
	}
	if !m.tryBoth() {
		return restrictedProcessingReservation{}, errRestrictedBusy
	}
	defer m.unlockBoth()
	if !m.root.bootstrapped || m.root.globalSlots >= restrictedGlobalSlots || !restrictedAddFits(m.root.globalUsed, charge, restrictedGlobalBytes) {
		return restrictedProcessingReservation{}, errRestrictedBusy
	}
	slot := -1
	genSlots := 0
	var genUsed uint64
	for i := range m.attempts {
		a := &m.attempts[i]
		if !a.used && slot < 0 {
			slot = i
		}
		if a.used && a.owner == owner && a.session == session && a.generation == generation {
			genSlots++
			genUsed += a.processingCharge + a.outputCharge
		}
	}
	if slot < 0 || genSlots >= restrictedGenerationSlots || !restrictedAddFits(genUsed, charge, restrictedGenerationBytes) {
		return restrictedProcessingReservation{}, errRestrictedBusy
	}
	attempt := m.nextAttempt
	if attempt == 0 {
		return restrictedProcessingReservation{}, errRestrictedState
	}
	m.nextAttempt++
	a := &m.attempts[slot]
	versions := [restrictedRegistrationRows]uint32{}
	for i := range a.rows {
		versions[i] = a.rows[i].version
	}
	*a = restrictedAttempt{used: true, owner: owner, session: session, generation: generation, attempt: attempt, version: 1, regionCharge: regionCharge, processingCharge: charge, regionCaps: caps, workRegistered: 1}
	for i := range a.rows {
		a.rows[i].version = versions[i]
	}
	m.root.globalUsed += charge
	m.root.globalSlots++
	// Reservation is visible before every actual fixed-region allocation.
	for i, c := range caps {
		if c > 0 {
			m.allocations++
			a.regions[i] = make([]byte, int(c))
			if uint32(cap(a.regions[i])) != c {
				panic("restricted region capacity")
			}
		}
	}
	return restrictedProcessingReservation{manager: m, slot: uint8(slot), Owner: owner, Session: session, Generation: generation, Attempt: attempt, Version: 1, Capacity: regionCharge, regions: caps}, nil
}

func (p restrictedProcessingReservation) exact(m *restrictedOwnerManager) (*restrictedAttempt, error) {
	if m == nil || p.manager != m || int(p.slot) >= len(m.attempts) {
		return nil, errRestrictedIdentity
	}
	a := &m.attempts[p.slot]
	if !a.used || a.owner != p.Owner || a.session != p.Session || a.generation != p.Generation || a.attempt != p.Attempt || a.version != p.Version || a.regionCharge != p.Capacity || a.regionCaps != p.regions {
		return nil, errRestrictedIdentity
	}
	for i, c := range a.regionCaps {
		if uint32(cap(a.regions[i])) != c {
			return nil, errRestrictedIdentity
		}
	}
	return a, nil
}
func (l restrictedLease) exact(m *restrictedOwnerManager) (*restrictedAttempt, error) {
	if m == nil || l.manager != m || int(l.slot) >= len(m.attempts) {
		return nil, errRestrictedIdentity
	}
	a := &m.attempts[l.slot]
	if !a.used || a.owner != l.Owner || a.session != l.Session || a.generation != l.Generation || a.attempt != l.Attempt || a.version != l.Version {
		return nil, errRestrictedIdentity
	}
	return a, nil
}

func (m *restrictedOwnerManager) construct(p restrictedProcessingReservation, in []DefinitionCandidate) (restrictedLease, error) {
	if len(in) > restrictedCandidateMax {
		return restrictedLease{}, errRestrictedBound
	}
	var strings uint64
	for i := range in {
		c := &in[i]
		if len(c.OccurrenceID) > 71 || len(c.QueryOccurrenceID) > 1024 || len(c.TargetID) > 71 || len(c.QueryURI) > 4096 || len(c.TargetURI) > 4096 {
			return restrictedLease{}, errRestrictedBound
		}
		n := uint64(len(c.OccurrenceID) + len(c.QueryOccurrenceID) + len(c.TargetID) + len(c.QueryURI) + len(c.TargetURI))
		if !restrictedAddFits(strings, n, restrictedStringBytes) {
			return restrictedLease{}, errRestrictedBound
		}
		strings += n
	}
	candidateBytes := uint64(len(in)) * uint64(unsafe.Sizeof(DefinitionCandidate{}))
	rangeBytes := uint64(len(in)) * uint64(unsafe.Sizeof(Range{}))
	backingConstruction := candidateBytes + rangeBytes + 2*strings
	backingSteady := candidateBytes + rangeBytes + strings
	if backingConstruction > restrictedConstructionMax {
		return restrictedLease{}, errRestrictedBound
	}
	construction := backingConstruction + restrictedLeaseValueBytes + restrictedViewValueBytes
	steady := backingSteady + restrictedLeaseValueBytes + restrictedViewValueBytes
	if construction < backingConstruction || steady < backingSteady {
		return restrictedLease{}, errRestrictedBound
	}
	if !m.tryBoth() {
		return restrictedLease{}, errRestrictedBusy
	}
	defer m.unlockBoth()
	a, err := p.exact(m)
	if err != nil {
		return restrictedLease{}, err
	}
	if a.outputBuilt {
		return restrictedLease{}, errRestrictedState
	}
	var genUsed uint64
	for i := range m.attempts {
		x := &m.attempts[i]
		if x.used && x.owner == a.owner && x.session == a.session && x.generation == a.generation {
			genUsed += x.processingCharge + x.outputCharge
		}
	}
	if !restrictedAddFits(genUsed, construction, restrictedGenerationBytes) || !restrictedAddFits(m.root.globalUsed, construction, restrictedGlobalBytes) {
		return restrictedLease{}, errRestrictedBusy
	}
	if m.nextBacking == 0 || m.nextBacking > ^uint64(0)-3 {
		return restrictedLease{}, errRestrictedState
	}
	m.root.globalUsed += construction
	a.outputCharge = construction
	a.outputBuilt = true
	m.allocations++
	a.output = make([]DefinitionCandidate, len(in))
	m.allocations++
	a.ranges = make([]Range, len(in))
	m.allocations++
	stage := make([]byte, int(strings))
	pos := 0
	for i := range in {
		c := in[i]
		a.output[i] = c
		for _, s := range []string{c.OccurrenceID, c.QueryOccurrenceID, c.TargetID, c.QueryURI, c.TargetURI} {
			pos += copy(stage[pos:], s)
		}
		if c.TargetRange != nil {
			a.ranges[i] = *c.TargetRange
			a.output[i].TargetRange = &a.ranges[i]
		}
	}
	m.allocations++
	a.pool = string(stage)
	pos = 0
	take := func(n int) string { s := a.pool[pos : pos+n]; pos += n; return s }
	for i := range a.output {
		src := &in[i]
		dst := &a.output[i]
		dst.OccurrenceID = take(len(src.OccurrenceID))
		dst.QueryOccurrenceID = take(len(src.QueryOccurrenceID))
		dst.TargetID = take(len(src.TargetID))
		dst.QueryURI = take(len(src.QueryURI))
		dst.TargetURI = take(len(src.TargetURI))
	}
	m.root.globalUsed -= strings
	a.outputCharge = steady
	for i, k := range [...]restrictedBackingKind{restrictedCandidatesBacking, restrictedRangesBacking, restrictedStringsBacking} {
		serial := m.nextBacking
		m.nextBacking++
		a.backings[i] = restrictedBackingID{a.owner, a.session, a.generation, a.attempt, serial, a.version, k}
	}
	t := restrictedTransfer{Owner: a.owner, Session: a.session, Generation: a.generation, Attempt: a.attempt, Version: a.version, Backings: a.backings}
	return restrictedLease{manager: m, slot: p.slot, Owner: a.owner, Session: a.session, Generation: a.generation, Attempt: a.attempt, Version: a.version, Transfer: t}, nil
}

func (m *restrictedOwnerManager) accept(l restrictedLease, t restrictedTransfer, r restrictedRecipient) error {
	if !m.tryBoth() {
		return errRestrictedBusy
	}
	defer m.unlockBoth()
	a, e := l.exact(m)
	if e != nil {
		return e
	}
	if r.ID == 0 || t.Owner != a.owner || t.Session != a.session || t.Generation != a.generation || t.Attempt != a.attempt || t.Version != a.version || t.Backings != a.backings {
		return errRestrictedIdentity
	}
	if a.recipient.ID != 0 && a.recipient != r {
		return errRestrictedState
	}
	a.recipient = r
	for i := range a.acked {
		a.acked[i] = true
	}
	return nil
}

func (m *restrictedOwnerManager) view(l restrictedLease, r restrictedRecipient) (restrictedView, error) {
	if !m.tryBoth() {
		return restrictedView{}, errRestrictedBusy
	}
	defer m.unlockBoth()
	a, e := l.exact(m)
	if e != nil {
		return restrictedView{}, e
	}
	if a.recipient.ID == 0 || a.recipient != r {
		return restrictedView{}, errRestrictedIdentity
	}
	for i := range a.rows {
		if a.rows[i].used && !a.rows[i].closed && a.rows[i].primary {
			return restrictedView{}, errRestrictedBusy
		}
	}
	for i := range a.rows {
		row := &a.rows[i]
		if !row.used || row.closed {
			row.version++
			if row.version == 0 {
				return restrictedView{}, errRestrictedState
			}
			tok := restrictedViewToken{a.owner, a.session, a.generation, a.attempt, a.version, r, uint32(i), row.version, a.backings}
			*row = restrictedRegistration{version: row.version, used: true, primary: true, token: tok}
			if a.provenanceState == restrictedProvenanceValidated {
				a.provenanceState = restrictedProvenanceHandedOff
			}
			return restrictedView{output: a.output, token: tok}, nil
		}
	}
	return restrictedView{}, errRestrictedBusy
}

func (m *restrictedOwnerManager) returnView(l restrictedLease, v restrictedView) error {
	if !m.tryBoth() {
		return errRestrictedBusy
	}
	defer m.unlockBoth()
	a, e := l.exact(m)
	if e != nil {
		return e
	}
	t := v.token
	if t.Owner != a.owner || t.Session != a.session || t.Generation != a.generation || t.Attempt != a.attempt || t.Version != a.version || t.Recipient != a.recipient || t.Backings != a.backings || int(t.Row) >= len(a.rows) {
		return errRestrictedIdentity
	}
	row := &a.rows[t.Row]
	if !row.used || row.version != t.RowVersion || row.token != t {
		return errRestrictedIdentity
	}
	if row.closed {
		return nil
	}
	row.closed = true
	return nil
}

func (m *restrictedOwnerManager) event(l restrictedLease, kind uint8) error {
	if !m.tryBoth() {
		return errRestrictedBusy
	}
	defer m.unlockBoth()
	a, e := l.exact(m)
	if e != nil {
		return e
	}
	switch kind {
	case 1:
		a.terminal = true
	case 2:
		if a.workJoined < a.workRegistered {
			a.workJoined++
		}
	case 3:
		if a.workCanceled < a.workRegistered {
			a.workCanceled++
		}
	case 4:
		a.retentionDisposed = true
	case 5:
		a.forgotten = true
	default:
		return errRestrictedState
	}
	return nil
}
func (m *restrictedOwnerManager) recordTerminal(l restrictedLease) error   { return m.event(l, 1) }
func (m *restrictedOwnerManager) joinWork(l restrictedLease) error         { return m.event(l, 2) }
func (m *restrictedOwnerManager) cancelWork(l restrictedLease) error       { return m.event(l, 3) }
func (m *restrictedOwnerManager) disposeRetention(l restrictedLease) error { return m.event(l, 4) }
func (m *restrictedOwnerManager) forget(l restrictedLease) error           { return m.event(l, 5) }

func (m *restrictedOwnerManager) release(l restrictedLease) error {
	if !m.tryBoth() {
		return errRestrictedBusy
	}
	defer m.unlockBoth()
	a, e := l.exact(m)
	if e != nil {
		return e
	}
	for _, ok := range a.acked {
		if !ok {
			return errRestrictedIncomplete
		}
	}
	for i := range a.rows {
		if a.rows[i].used && !a.rows[i].closed {
			return errRestrictedIncomplete
		}
	}
	if !a.terminal || a.workJoined != a.workRegistered || !a.retentionDisposed || !a.forgotten {
		return errRestrictedIncomplete
	}
	charge := a.processingCharge + a.outputCharge
	rootRaw, rootRecords := m.root.provenanceRaw[l.slot], m.root.provenanceRecords[l.slot]
	if m.root.globalUsed < charge || m.root.globalSlots == 0 || int(l.slot) >= len(m.root.provenanceRaw) ||
		len(rootRaw) != len(a.provenanceRaw) || len(rootRecords) != len(a.provenanceRecords) ||
		(len(a.provenanceRaw) != 0 && &rootRaw[0] != &a.provenanceRaw[0]) ||
		(len(a.provenanceRecords) != 0 && &rootRecords[0] != &a.provenanceRecords[0]) {
		return errRestrictedState
	}
	versions := [restrictedRegistrationRows]uint32{}
	for i := range a.rows {
		versions[i] = a.rows[i].version
	}
	// Clear the exact global-slot roots before resetting and crediting release.
	// A reused generation may occupy another slot; the index is always l.slot.
	m.root.provenanceRaw[l.slot] = nil
	m.root.provenanceRecords[l.slot] = nil
	m.root.globalUsed -= charge
	m.root.globalSlots--
	*a = restrictedAttempt{}
	for i := range a.rows {
		a.rows[i].version = versions[i]
	}
	return nil
}
func (m *restrictedOwnerManager) quarantine(l restrictedLease) error {
	if !m.tryBoth() {
		return errRestrictedBusy
	}
	defer m.unlockBoth()
	a, e := l.exact(m)
	if e != nil {
		return e
	}
	a.retired = true
	a.quarantined = true
	return nil
}

func (m *restrictedOwnerManager) snapshot(l restrictedLease) restrictedSnapshot {
	if !m.tryBoth() {
		return restrictedSnapshot{}
	}
	defer m.unlockBoth()
	a, e := l.exact(m)
	if e != nil {
		return restrictedSnapshot{}
	}
	var open uint32
	for i := range a.rows {
		if a.rows[i].used && !a.rows[i].closed {
			open++
		}
	}
	return restrictedSnapshot{a.used, a.outputBuilt, a.retired, a.quarantined, a.recipient, a.acked, a.workRegistered, a.workJoined, a.workCanceled, a.terminal, a.retentionDisposed, a.forgotten, open, a.processingCharge + a.outputCharge}
}
