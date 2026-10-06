package sessionruntime

import (
	"context"
	"errors"
	"math"
	"sync"
	"unsafe"

	"lsp-trace/internal/session"
)

const privateB4EventLimit = 8192

var (
	errPrivateB4EventAdmission = errors.New("private B4 event admission refused")
	ErrPrivateB4C17NotEnabled  = errors.New("private B4 event admission refused: C17_NOT_ENABLED")
)

type privateB4EventIdentityC17 struct {
	Family  string
	Kind    string
	Ordinal int
}

type privateB4EventEntryC17 struct {
	identity    privateB4EventIdentityC17
	batch       uint64
	batchFamily uint8
	state       uint8
}

const (
	privateB4EventBatchCapabilityC17 uint8 = iota + 1
	privateB4EventBatchAcquisitionC17
)

type privateB4EventAccountC17 struct {
	bytes     *privateB4ByteAccountV2
	ownsBytes bool

	mu                      sync.Mutex
	settled                 sync.Cond
	entries                 [privateB4EventLimit]privateB4EventEntryC17
	admitted                uint64
	inFlight                uint64
	activeCallbacks         uint64
	nextCapabilityOrdinal   uint64
	nextCapabilityBatch     uint64
	capabilityBatchInFlight bool
	nextAcquisitionBatch    uint64
	acquisitionAdmission    *privateB4AcquisitionAdmissionC17
	acquisitionInFlight     bool
	acquisitionCommitted    bool
	terminalRequested       bool
	terminalFailure         session.Failure
	finalizing              bool
	backingReleased         bool
	backing                 privateB4ByteLeaseV2
}

func privateB4EventAccountC17SelfCost() uint64 {
	return uint64(unsafe.Sizeof(privateB4EventAccountC17{}))
}

func newPrivateB4EventAccountC17() (*privateB4EventAccountC17, session.Failure) {
	bytes, failure := newPrivateB4ByteAccountV2()
	if failure != "" {
		return nil, failure
	}
	profile, failure := newPrivateB4EventAccountC17WithBytes(bytes)
	if failure != "" {
		bytes.requestTerminalRelease()
		return nil, failure
	}
	profile.ownsBytes = true
	return profile, ""
}

func newPrivateB4EventAccountC17WithBytes(bytes *privateB4ByteAccountV2) (*privateB4EventAccountC17, session.Failure) {
	if bytes == nil {
		return nil, session.ResourceExhausted
	}
	backing, failure := bytes.reserve(privateB4EventAccountC17SelfCost())
	if failure != "" {
		return nil, failure
	}
	profile := &privateB4EventAccountC17{bytes: bytes, backing: backing}
	profile.settled.L = &profile.mu
	return profile, ""
}

type privateB4AcquisitionAdmissionC17 struct {
	account *privateB4EventAccountC17
	batch   uint64
	count   uint64
	slots   []int
	sealed  bool
	settled bool
}

func (a *privateB4EventAccountC17) WithEventAdmission(identity privateB4EventIdentityC17, appendCanonical func() error) error {
	return a.withEventAdmissions([]privateB4EventIdentityC17{identity}, appendCanonical)
}

func privateB4AcquisitionIdentity(elementCount, index int) privateB4EventIdentityC17 {
	switch {
	case index == 0:
		return privateB4EventIdentityC17{Family: "ACQUISITION", Kind: "QUERY_BEGIN", Ordinal: 0}
	case index == 2*elementCount+1:
		return privateB4EventIdentityC17{Family: "ACQUISITION", Kind: "QUERY_TERMINAL", Ordinal: 0}
	case index%2 == 1:
		return privateB4EventIdentityC17{Family: "ACQUISITION", Kind: "ELEMENT_BEGIN", Ordinal: (index - 1) / 2}
	default:
		return privateB4EventIdentityC17{Family: "ACQUISITION", Kind: "ELEMENT_TERMINAL", Ordinal: (index - 2) / 2}
	}
}

func (a *privateB4EventAccountC17) beginAcquisitionAdmission(elementCount int) (*privateB4AcquisitionAdmissionC17, error) {
	if a == nil || elementCount < 0 || elementCount > privateB4MaxSourceDocuments {
		return nil, errPrivateB4EventAdmission
	}
	count := uint64(2 + 2*elementCount)
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.terminalRequested || a.backingReleased || a.acquisitionInFlight || a.acquisitionCommitted {
		return nil, errPrivateB4EventAdmission
	}
	if count > privateB4EventLimit || a.admitted+a.inFlight > privateB4EventLimit-count {
		a.terminalRequested = true
		a.terminalFailure = session.ResourceExhausted
		if a.activeCallbacks == 0 {
			a.settled.Broadcast()
		}
		return nil, errPrivateB4EventAdmission
	}
	a.nextAcquisitionBatch++
	if a.nextAcquisitionBatch == 0 {
		a.nextAcquisitionBatch++
	}
	batch := a.nextAcquisitionBatch
	filled := 0
	for slot := range a.entries {
		if a.entries[slot].state != 0 {
			continue
		}
		a.entries[slot] = privateB4EventEntryC17{
			identity:    privateB4AcquisitionIdentity(elementCount, filled),
			batch:       batch,
			batchFamily: privateB4EventBatchAcquisitionC17,
			state:       1,
		}
		filled++
		if filled == int(count) {
			break
		}
	}
	if filled != int(count) {
		for slot := range a.entries {
			if a.entries[slot].batchFamily == privateB4EventBatchAcquisitionC17 && a.entries[slot].batch == batch {
				a.entries[slot] = privateB4EventEntryC17{}
			}
		}
		return nil, errPrivateB4EventAdmission
	}
	a.inFlight += count
	a.activeCallbacks++
	a.acquisitionInFlight = true
	admission := &privateB4AcquisitionAdmissionC17{account: a, batch: batch, count: count}
	a.acquisitionAdmission = admission
	return admission, nil
}

func (t *privateB4AcquisitionAdmissionC17) openSlotsLocked() ([]int, error) {
	if t == nil || t.account == nil || t.settled || t.sealed {
		return nil, errPrivateB4EventAdmission
	}
	a := t.account
	maxCount := uint64(2 + 2*privateB4MaxSourceDocuments)
	if a.acquisitionAdmission != t || !a.acquisitionInFlight || a.acquisitionCommitted ||
		t.count < 2 || t.count%2 != 0 || t.count > maxCount || t.count > privateB4EventLimit ||
		t.count > a.inFlight || a.activeCallbacks == 0 {
		return nil, errPrivateB4EventAdmission
	}
	slots := make([]int, 0, t.count)
	for slot := range a.entries {
		entry := a.entries[slot]
		if entry.batchFamily == privateB4EventBatchAcquisitionC17 && entry.batch == t.batch && entry.state == 1 {
			slots = append(slots, slot)
		}
	}
	if uint64(len(slots)) != t.count {
		return nil, errPrivateB4EventAdmission
	}
	return slots, nil
}

func (t *privateB4AcquisitionAdmissionC17) seal() error {
	if t == nil || t.account == nil {
		return errPrivateB4EventAdmission
	}
	a := t.account
	a.mu.Lock()
	defer a.mu.Unlock()
	slots, err := t.openSlotsLocked()
	if err != nil {
		return err
	}
	t.slots = slots
	t.sealed = true
	return nil
}

func (t *privateB4AcquisitionAdmissionC17) rollback() error {
	if t == nil || t.account == nil {
		return errPrivateB4EventAdmission
	}
	a := t.account
	a.mu.Lock()
	defer a.mu.Unlock()
	if t.settled {
		return errPrivateB4EventAdmission
	}
	slots := t.slots
	if !t.sealed {
		var err error
		slots, err = t.openSlotsLocked()
		if err != nil {
			return err
		}
	}
	for _, slot := range slots {
		a.entries[slot] = privateB4EventEntryC17{}
	}
	a.inFlight -= t.count
	a.activeCallbacks--
	a.acquisitionAdmission = nil
	a.acquisitionInFlight = false
	t.settled = true
	if a.terminalRequested && a.activeCallbacks == 0 {
		a.settled.Broadcast()
	}
	return nil
}

func (t *privateB4AcquisitionAdmissionC17) finalize() {
	if t == nil || t.account == nil {
		return
	}
	a := t.account
	a.mu.Lock()
	defer a.mu.Unlock()
	if t.settled || !t.sealed {
		return
	}
	for _, slot := range t.slots {
		a.entries[slot].batch = 0
		a.entries[slot].batchFamily = 0
		a.entries[slot].state = 2
	}
	a.admitted += t.count
	a.inFlight -= t.count
	a.activeCallbacks--
	a.acquisitionAdmission = nil
	a.acquisitionInFlight = false
	a.acquisitionCommitted = true
	t.settled = true
	if a.terminalRequested && a.activeCallbacks == 0 {
		a.settled.Broadcast()
	}
}

func (t *privateB4AcquisitionAdmissionC17) commit() error {
	if err := t.seal(); err != nil {
		return err
	}
	t.finalize()
	return nil
}

func (a *privateB4EventAccountC17) withAcquisitionAdmission(elementCount int, appendCanonical func() error) (err error) {
	if appendCanonical == nil {
		return errPrivateB4EventAdmission
	}
	token, err := a.beginAcquisitionAdmission(elementCount)
	if err != nil {
		return err
	}
	defer func() {
		if !token.settled {
			_ = token.rollback()
		}
	}()
	if err = appendCanonical(); err != nil {
		return err
	}
	if err = token.seal(); err != nil {
		return err
	}
	token.finalize()
	return nil
}

func (a *privateB4EventAccountC17) withEventAdmissions(identities []privateB4EventIdentityC17, appendCanonical func() error) (err error) {
	if a == nil || appendCanonical == nil || len(identities) == 0 {
		return errPrivateB4EventAdmission
	}
	a.mu.Lock()
	if a.terminalRequested || a.backingReleased {
		a.mu.Unlock()
		return errPrivateB4EventAdmission
	}
	count := uint64(len(identities))
	var slots [2]int
	if len(identities) > len(slots) {
		a.mu.Unlock()
		return errPrivateB4EventAdmission
	}
	for i, identity := range identities {
		if identity.Family == "" || identity.Kind == "" || identity.Ordinal < 0 {
			a.mu.Unlock()
			return errPrivateB4EventAdmission
		}
		for j := 0; j < i; j++ {
			if identities[j] == identity {
				a.mu.Unlock()
				return errPrivateB4EventAdmission
			}
		}
		for j := range a.entries {
			if a.entries[j].state != 0 && a.entries[j].identity == identity {
				a.mu.Unlock()
				return errPrivateB4EventAdmission
			}
		}
	}
	if count > privateB4EventLimit || a.admitted+a.inFlight > privateB4EventLimit-count {
		a.terminalRequested = true
		a.terminalFailure = session.ResourceExhausted
		if a.activeCallbacks == 0 {
			a.settled.Broadcast()
		}
		a.mu.Unlock()
		return errPrivateB4EventAdmission
	}
	for i := range identities {
		free := -1
		for j := range a.entries {
			used := false
			for k := 0; k < i; k++ {
				used = used || slots[k] == j
			}
			if a.entries[j].state == 0 && !used {
				free = j
				break
			}
		}
		if free < 0 {
			a.mu.Unlock()
			return errPrivateB4EventAdmission
		}
		slots[i] = free
	}
	for i, identity := range identities {
		a.entries[slots[i]] = privateB4EventEntryC17{identity: identity, state: 1}
	}
	a.inFlight += count
	a.activeCallbacks++
	a.mu.Unlock()

	completed := false
	defer func() {
		a.mu.Lock()
		a.inFlight -= count
		if completed && err == nil {
			for _, slot := range slots[:len(identities)] {
				a.entries[slot].state = 2
			}
			a.admitted += count
		} else {
			for _, slot := range slots[:len(identities)] {
				a.entries[slot] = privateB4EventEntryC17{}
			}
		}
		a.activeCallbacks--
		if a.terminalRequested && a.activeCallbacks == 0 {
			a.settled.Broadcast()
		}
		a.mu.Unlock()
	}()
	err = appendCanonical()
	completed = true
	return err
}

// beginPublication keeps transaction-owned C17 backing alive across publication
// without admitting or charging an event. Terminal retirement waits for the
// matching endPublication call.
func (a *privateB4EventAccountC17) beginPublication() error {
	if a == nil {
		return ErrPrivateB4C17NotEnabled
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.terminalRequested || a.backingReleased {
		return errPrivateB4EventAdmission
	}
	a.activeCallbacks++
	return nil
}

func (a *privateB4EventAccountC17) endPublication() {
	if a == nil {
		return
	}
	a.mu.Lock()
	if a.activeCallbacks > 0 {
		a.activeCallbacks--
	}
	if a.terminalRequested && a.activeCallbacks == 0 {
		a.settled.Broadcast()
	}
	a.mu.Unlock()
}

func (a *privateB4EventAccountC17) withCapabilityBatchAdmission(count int, appendCanonical func() (session.Failure, bool)) (session.Failure, bool) {
	return a.withCapabilityBatchAdmissionObserved(count, func(uint64) (session.Failure, bool) {
		return appendCanonical()
	})
}

func (a *privateB4EventAccountC17) withCapabilityBatchAdmissionObserved(count int, appendCanonical func(uint64) (session.Failure, bool)) (failure session.Failure, poison bool) {
	if a == nil || appendCanonical == nil || count <= 0 {
		return session.ResourceExhausted, false
	}
	batchCount := uint64(count)
	a.mu.Lock()
	if a.terminalRequested || a.backingReleased || a.capabilityBatchInFlight {
		a.mu.Unlock()
		return session.ResourceExhausted, false
	}
	if batchCount > privateB4EventLimit || a.admitted+a.inFlight > privateB4EventLimit-batchCount {
		a.terminalRequested = true
		a.terminalFailure = session.ResourceExhausted
		if a.activeCallbacks == 0 {
			a.settled.Broadcast()
		}
		a.mu.Unlock()
		return session.ResourceExhausted, false
	}
	a.nextCapabilityBatch++
	if a.nextCapabilityBatch == 0 {
		a.nextCapabilityBatch++
	}
	batch := a.nextCapabilityBatch
	startOrdinal := a.nextCapabilityOrdinal
	filled := uint64(0)
	for i := range a.entries {
		if a.entries[i].state != 0 {
			continue
		}
		a.entries[i] = privateB4EventEntryC17{
			identity:    privateB4EventIdentityC17{Family: "CAPABILITY", Kind: "CAPABILITY_ENTRY", Ordinal: int(startOrdinal + filled)},
			batch:       batch,
			batchFamily: privateB4EventBatchCapabilityC17,
			state:       1,
		}
		filled++
		if filled == batchCount {
			break
		}
	}
	if filled != batchCount {
		for i := range a.entries {
			if a.entries[i].batchFamily == privateB4EventBatchCapabilityC17 && a.entries[i].batch == batch {
				a.entries[i] = privateB4EventEntryC17{}
			}
		}
		a.mu.Unlock()
		return session.ResourceExhausted, false
	}
	a.inFlight += batchCount
	a.activeCallbacks++
	a.capabilityBatchInFlight = true
	a.mu.Unlock()

	completed := false
	defer func() {
		a.mu.Lock()
		owned := uint64(0)
		for i := range a.entries {
			if a.entries[i].batchFamily == privateB4EventBatchCapabilityC17 && a.entries[i].batch == batch && a.entries[i].state == 1 {
				owned++
			}
		}
		if owned != batchCount || batchCount > a.inFlight || a.activeCallbacks == 0 {
			failure = session.ResourceExhausted
			poison = false
			a.terminalRequested = true
			a.terminalFailure = session.ResourceExhausted
			a.capabilityBatchInFlight = false
			if a.activeCallbacks > 0 {
				a.activeCallbacks--
			}
			if a.activeCallbacks == 0 {
				a.settled.Broadcast()
			}
			a.mu.Unlock()
			return
		}
		if completed && failure == "" {
			for i := range a.entries {
				if a.entries[i].batchFamily == privateB4EventBatchCapabilityC17 && a.entries[i].batch == batch && a.entries[i].state == 1 {
					a.entries[i].batch = 0
					a.entries[i].batchFamily = 0
					a.entries[i].state = 2
				}
			}
			a.admitted += batchCount
			a.nextCapabilityOrdinal += batchCount
		} else {
			for i := range a.entries {
				if a.entries[i].batchFamily == privateB4EventBatchCapabilityC17 && a.entries[i].batch == batch && a.entries[i].state == 1 {
					a.entries[i] = privateB4EventEntryC17{}
				}
			}
		}
		a.inFlight -= batchCount
		a.capabilityBatchInFlight = false
		a.activeCallbacks--
		if a.terminalRequested && a.activeCallbacks == 0 {
			a.settled.Broadcast()
		}
		a.mu.Unlock()
	}()
	failure, poison = appendCanonical(startOrdinal)
	completed = true
	return failure, poison
}

func (a *privateB4EventAccountC17) withTargetAppendAdmission(originalOrdinal uint64, appendCanonical func() error) error {
	if originalOrdinal > uint64(math.MaxInt) {
		return errPrivateB4EventAdmission
	}
	ordinal := int(originalOrdinal)
	return a.withEventAdmissions([]privateB4EventIdentityC17{
		{Family: "TARGET", Kind: "TARGET_BEGIN", Ordinal: ordinal},
		{Family: "TARGET", Kind: "TARGET_TERMINAL", Ordinal: ordinal},
	}, appendCanonical)
}

func (a *privateB4EventAccountC17) requestTerminalReleaseC17(ctx context.Context) session.Failure {
	if a == nil {
		return ""
	}
	if ctx == nil {
		return session.RequestCancelled
	}
	a.mu.Lock()
	a.terminalRequested = true
	stop := context.AfterFunc(ctx, func() { a.mu.Lock(); a.settled.Broadcast(); a.mu.Unlock() })
	defer stop()
	for (a.activeCallbacks != 0 || a.finalizing) && ctx.Err() == nil {
		a.settled.Wait()
	}
	if failure := contextFailure(ctx); failure != "" {
		a.mu.Unlock()
		return failure
	}
	if a.backingReleased {
		a.mu.Unlock()
		return ""
	}
	a.finalizing = true
	backing := a.backing
	ownsBytes := a.ownsBytes
	bytes := a.bytes
	a.mu.Unlock()
	backing.release()
	if ownsBytes {
		bytes.requestTerminalRelease()
	}
	a.mu.Lock()
	a.finalizing = false
	a.backingReleased = true
	a.settled.Broadcast()
	a.mu.Unlock()
	return ""
}

type privateB4EventSnapshotC17 struct {
	Admitted, InFlight, ActiveCallbacks uint64
	TerminalRequested, BackingReleased  bool
	TerminalFailure                     session.Failure
}

func (a *privateB4EventAccountC17) eventSnapshot() privateB4EventSnapshotC17 {
	if a == nil {
		return privateB4EventSnapshotC17{}
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	return privateB4EventSnapshotC17{
		Admitted: a.admitted, InFlight: a.inFlight, ActiveCallbacks: a.activeCallbacks,
		TerminalRequested: a.terminalRequested, BackingReleased: a.backingReleased,
		TerminalFailure: a.terminalFailure,
	}
}
