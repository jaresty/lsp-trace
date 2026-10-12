package adr0011work

import (
	"context"
	"errors"
	"math"
	"sync"
)

var (
	errPrivateWorkTerminal = errors.New("private work account terminal")
	errPrivateWorkOverflow = errors.New("private work account overflow")
	errPrivateWorkIdentity = errors.New("private work account identity mismatch")
	errPrivateWorkCallback = errors.New("private work account callback required")

	// These errors are exported only through Go's repository-internal package
	// boundary so other private owners can preserve typed refusal semantics.
	ErrTerminal         = errPrivateWorkTerminal
	ErrOverflow         = errPrivateWorkOverflow
	ErrIdentityMismatch = errPrivateWorkIdentity
	ErrCallbackRequired = errPrivateWorkCallback
)

type occurrenceIdentity string

type privateWorkDelta struct {
	bytesScanned               uint64
	messagesProcessed          uint64
	objectsMaterialized        uint64
	eventsEmitted              uint64
	documentsAcquired          uint64
	logicalBufferBytesReserved uint64
}

type privateWorkSnapshot struct {
	identity                   occurrenceIdentity
	bytesScanned               uint64
	messagesProcessed          uint64
	objectsMaterialized        uint64
	eventsEmitted              uint64
	documentsAcquired          uint64
	logicalBufferBytesReserved uint64
}

type privateWorkAccount struct {
	mu                   sync.Mutex
	settled              *sync.Cond
	identity             occurrenceIdentity
	work                 privateWorkDelta
	objectsReserved      uint64
	eventsReserved       uint64
	eventCallbacks       uint64
	eventCommitBlocked   bool
	documentsReserved    uint64
	documentCallbacks    uint64
	documentPending      map[string]uint64
	logicalBytesReserved uint64
	logicalByteCallbacks uint64
	logicalCommitBlocked bool
	documentCommitted    map[string]bool
	terminal             bool
	failure              error
}

func newPrivateWorkAccount(identity occurrenceIdentity) *privateWorkAccount {
	a := &privateWorkAccount{
		identity:          identity,
		documentPending:   make(map[string]uint64),
		documentCommitted: make(map[string]bool),
	}
	a.settled = sync.NewCond(&a.mu)
	return a
}

func (a *privateWorkAccount) charge(identity occurrenceIdentity, delta privateWorkDelta) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.terminal {
		return errPrivateWorkTerminal
	}
	if identity != a.identity {
		return errPrivateWorkIdentity
	}
	if overflows(a.work.bytesScanned, delta.bytesScanned) ||
		overflows(a.work.messagesProcessed, delta.messagesProcessed) ||
		overflows(a.work.objectsMaterialized, delta.objectsMaterialized) ||
		overflows(a.work.eventsEmitted, a.eventsReserved) ||
		overflows(a.work.eventsEmitted+a.eventsReserved, delta.eventsEmitted) ||
		overflows(a.work.documentsAcquired, a.documentsReserved) ||
		overflows(a.work.documentsAcquired+a.documentsReserved, delta.documentsAcquired) ||
		overflows(a.work.logicalBufferBytesReserved, a.logicalBytesReserved) ||
		overflows(a.work.logicalBufferBytesReserved+a.logicalBytesReserved, delta.logicalBufferBytesReserved) {
		a.terminal = true
		a.failure = errPrivateWorkOverflow
		if delta.eventsEmitted != 0 && a.eventsReserved != 0 {
			a.eventCommitBlocked = true
		}
		if delta.logicalBufferBytesReserved != 0 && a.logicalBytesReserved != 0 {
			a.logicalCommitBlocked = true
		}
		return a.failure
	}

	a.work.bytesScanned += delta.bytesScanned
	a.work.messagesProcessed += delta.messagesProcessed
	a.work.objectsMaterialized += delta.objectsMaterialized
	a.work.eventsEmitted += delta.eventsEmitted
	a.work.documentsAcquired += delta.documentsAcquired
	a.work.logicalBufferBytesReserved += delta.logicalBufferBytesReserved
	return nil
}

func (a *privateWorkAccount) snapshot() privateWorkSnapshot {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.snapshotLocked()
}

func (a *privateWorkAccount) seal() privateWorkSnapshot {
	a.mu.Lock()
	defer a.mu.Unlock()
	if !a.terminal {
		a.terminal = true
	}
	return a.snapshotLocked()
}

func (a *privateWorkAccount) snapshotLocked() privateWorkSnapshot {
	return privateWorkSnapshot{
		identity:                   a.identity,
		bytesScanned:               a.work.bytesScanned,
		messagesProcessed:          a.work.messagesProcessed,
		objectsMaterialized:        a.work.objectsMaterialized,
		eventsEmitted:              a.work.eventsEmitted,
		documentsAcquired:          a.work.documentsAcquired,
		logicalBufferBytesReserved: a.work.logicalBufferBytesReserved,
	}
}

func overflows(current, delta uint64) bool {
	return delta > math.MaxUint64-current
}

// Account is an opaque repository-internal work account. Callers can mutate it
// only through row-specific charge methods bound to its construction identity.
type Account struct {
	private *privateWorkAccount
}

// Snapshot is an immutable observation of one account.
type Snapshot struct {
	Identity                   string
	BytesScanned               uint64
	MessagesProcessed          uint64
	ObjectsMaterialized        uint64
	EventsEmitted              uint64
	DocumentsAcquired          uint64
	LogicalBufferBytesReserved uint64
	Terminal                   bool
	Failure                    error
}

func New(identity string) *Account {
	return &Account{private: newPrivateWorkAccount(occurrenceIdentity(identity))}
}

func (a *Account) charge(identity string, delta privateWorkDelta) error {
	if a == nil || a.private == nil {
		return errPrivateWorkIdentity
	}
	return a.private.charge(occurrenceIdentity(identity), delta)
}

func (a *Account) ChargeBytesScanned(identity string, amount uint64) error {
	return a.charge(identity, privateWorkDelta{bytesScanned: amount})
}

func (a *Account) ChargeMessagesProcessed(identity string, amount uint64) error {
	return a.charge(identity, privateWorkDelta{messagesProcessed: amount})
}

func (a *Account) ChargeObjectsMaterialized(identity string, amount uint64) error {
	return a.charge(identity, privateWorkDelta{objectsMaterialized: amount})
}

// WithObjectMaterialization reserves one object before materialize runs and
// commits it only after materialize successfully constructs the object.
func (a *Account) WithObjectMaterialization(identity string, materialize func() error) (err error) {
	if a == nil || a.private == nil {
		return errPrivateWorkIdentity
	}
	if materialize == nil {
		return errPrivateWorkCallback
	}

	p := a.private
	p.mu.Lock()
	if p.terminal {
		p.mu.Unlock()
		return errPrivateWorkTerminal
	}
	if occurrenceIdentity(identity) != p.identity {
		p.mu.Unlock()
		return errPrivateWorkIdentity
	}
	if overflows(p.work.objectsMaterialized, p.objectsReserved) ||
		overflows(p.work.objectsMaterialized+p.objectsReserved, 1) {
		p.terminal = true
		p.failure = errPrivateWorkOverflow
		p.mu.Unlock()
		return errPrivateWorkOverflow
	}
	p.objectsReserved++
	p.mu.Unlock()

	defer func() {
		p.mu.Lock()
		p.objectsReserved--
		if recovered := recover(); recovered != nil {
			if p.objectsReserved == 0 && p.eventCallbacks == 0 {
				p.settled.Broadcast()
			}
			p.mu.Unlock()
			panic(recovered)
		}
		if err == nil {
			p.work.objectsMaterialized++
		}
		if p.objectsReserved == 0 && p.eventCallbacks == 0 {
			p.settled.Broadcast()
		}
		p.mu.Unlock()
	}()

	return materialize()
}

// WithEventsEmitted reserves amount events before emit runs and commits them
// only after emit returns successfully.
func (a *Account) WithEventsEmitted(identity string, amount uint64, emit func() error) (err error) {
	if a == nil || a.private == nil {
		return errPrivateWorkIdentity
	}
	if emit == nil {
		return errPrivateWorkCallback
	}

	p := a.private
	p.mu.Lock()
	if p.terminal {
		p.mu.Unlock()
		return errPrivateWorkTerminal
	}
	if occurrenceIdentity(identity) != p.identity {
		p.mu.Unlock()
		return errPrivateWorkIdentity
	}
	if overflows(p.work.eventsEmitted, p.eventsReserved) ||
		overflows(p.work.eventsEmitted+p.eventsReserved, amount) {
		p.terminal = true
		p.failure = errPrivateWorkOverflow
		p.mu.Unlock()
		return errPrivateWorkOverflow
	}
	p.eventsReserved += amount
	p.eventCallbacks++
	p.mu.Unlock()

	defer func() {
		p.mu.Lock()
		p.eventsReserved -= amount
		p.eventCallbacks--
		if recovered := recover(); recovered != nil {
			if p.objectsReserved == 0 && p.eventCallbacks == 0 {
				p.settled.Broadcast()
			}
			p.mu.Unlock()
			panic(recovered)
		}
		if err == nil && !p.eventCommitBlocked {
			p.work.eventsEmitted += amount
		}
		if p.objectsReserved == 0 && p.eventCallbacks == 0 {
			p.settled.Broadcast()
		}
		p.mu.Unlock()
	}()

	return emit()
}

func (a *Account) ChargeEventsEmitted(identity string, amount uint64) error {
	return a.charge(identity, privateWorkDelta{eventsEmitted: amount})
}

// WithDocumentAcquisition admits every callback but accounts only the first
// successful acquisition of each exact document identity.
func (a *Account) WithDocumentAcquisition(identity, documentIdentity string, acquire func() (bool, error)) (err error) {
	if a == nil || a.private == nil {
		return errPrivateWorkIdentity
	}
	if acquire == nil {
		return errPrivateWorkCallback
	}

	p := a.private
	p.mu.Lock()
	if p.terminal {
		p.mu.Unlock()
		return errPrivateWorkTerminal
	}
	if occurrenceIdentity(identity) != p.identity {
		p.mu.Unlock()
		return errPrivateWorkIdentity
	}
	if !p.documentCommitted[documentIdentity] && p.documentPending[documentIdentity] == 0 {
		if overflows(p.work.documentsAcquired, p.documentsReserved) ||
			overflows(p.work.documentsAcquired+p.documentsReserved, 1) {
			p.terminal = true
			p.failure = errPrivateWorkOverflow
			p.mu.Unlock()
			return errPrivateWorkOverflow
		}
		p.documentsReserved++
	}
	p.documentPending[documentIdentity]++
	p.documentCallbacks++
	p.mu.Unlock()

	var acquired bool
	defer func() {
		p.mu.Lock()
		p.documentPending[documentIdentity]--
		p.documentCallbacks--
		recovered := recover()
		if recovered == nil && err == nil && acquired && !p.documentCommitted[documentIdentity] {
			p.documentCommitted[documentIdentity] = true
			p.documentsReserved--
			p.work.documentsAcquired++
		}
		if p.documentPending[documentIdentity] == 0 {
			delete(p.documentPending, documentIdentity)
			if !p.documentCommitted[documentIdentity] {
				p.documentsReserved--
			}
		}
		if p.objectsReserved == 0 && p.eventCallbacks == 0 && p.documentCallbacks == 0 {
			p.settled.Broadcast()
		}
		p.mu.Unlock()
		if recovered != nil {
			panic(recovered)
		}
	}()

	acquired, err = acquire()
	return err
}

func (a *Account) ChargeDocumentsAcquired(identity string, amount uint64) error {
	return a.charge(identity, privateWorkDelta{documentsAcquired: amount})
}

// WithLogicalBufferReservation reserves amount bytes before reserve runs and
// commits them only after reserve returns successfully.
func (a *Account) WithLogicalBufferReservation(identity string, amount uint64, reserve func() error) (err error) {
	if a == nil || a.private == nil {
		return errPrivateWorkIdentity
	}
	if reserve == nil {
		return errPrivateWorkCallback
	}

	p := a.private
	p.mu.Lock()
	if p.terminal {
		p.mu.Unlock()
		return errPrivateWorkTerminal
	}
	if occurrenceIdentity(identity) != p.identity {
		p.mu.Unlock()
		return errPrivateWorkIdentity
	}
	if overflows(p.work.logicalBufferBytesReserved, p.logicalBytesReserved) ||
		overflows(p.work.logicalBufferBytesReserved+p.logicalBytesReserved, amount) {
		p.terminal = true
		p.failure = errPrivateWorkOverflow
		p.mu.Unlock()
		return errPrivateWorkOverflow
	}
	p.logicalBytesReserved += amount
	p.logicalByteCallbacks++
	p.mu.Unlock()

	defer func() {
		p.mu.Lock()
		p.logicalBytesReserved -= amount
		p.logicalByteCallbacks--
		recovered := recover()
		if recovered == nil && err == nil && !p.logicalCommitBlocked {
			p.work.logicalBufferBytesReserved += amount
		}
		if p.objectsReserved == 0 && p.eventCallbacks == 0 && p.documentCallbacks == 0 && p.logicalByteCallbacks == 0 {
			p.settled.Broadcast()
		}
		p.mu.Unlock()
		if recovered != nil {
			panic(recovered)
		}
	}()

	return reserve()
}

func (a *Account) ChargeLogicalBufferBytesReserved(identity string, amount uint64) error {
	return a.charge(identity, privateWorkDelta{logicalBufferBytesReserved: amount})
}

func (a *Account) Snapshot() Snapshot {
	if a == nil || a.private == nil {
		return Snapshot{}
	}
	a.private.mu.Lock()
	defer a.private.mu.Unlock()
	return exportSnapshot(a.private.snapshotLocked(), a.private.terminal, a.private.failure)
}

func (a *Account) Seal() Snapshot {
	if a == nil || a.private == nil {
		return Snapshot{}
	}
	p := a.private
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.terminal {
		p.terminal = true
	}
	for p.objectsReserved != 0 || p.eventCallbacks != 0 || p.documentCallbacks != 0 || p.logicalByteCallbacks != 0 {
		p.settled.Wait()
	}
	return exportSnapshot(p.snapshotLocked(), p.terminal, p.failure)
}

func exportSnapshot(s privateWorkSnapshot, terminal bool, failure error) Snapshot {
	return Snapshot{
		Identity:                   string(s.identity),
		BytesScanned:               s.bytesScanned,
		MessagesProcessed:          s.messagesProcessed,
		ObjectsMaterialized:        s.objectsMaterialized,
		EventsEmitted:              s.eventsEmitted,
		DocumentsAcquired:          s.documentsAcquired,
		LogicalBufferBytesReserved: s.logicalBufferBytesReserved,
		Terminal:                   terminal,
		Failure:                    failure,
	}
}

type accountContextKey struct{}

// WithAccount installs account only when no account is already attached.
func WithAccount(ctx context.Context, account *Account) context.Context {
	if ctx == nil || account == nil || account.private == nil {
		return ctx
	}
	if _, ok := AccountFromContext(ctx); ok {
		return ctx
	}
	return context.WithValue(ctx, accountContextKey{}, account)
}

func AccountFromContext(ctx context.Context) (*Account, bool) {
	if ctx == nil {
		return nil, false
	}
	account, ok := ctx.Value(accountContextKey{}).(*Account)
	return account, ok && account != nil && account.private != nil
}
