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

	// These errors are exported only through Go's repository-internal package
	// boundary so other private owners can preserve typed refusal semantics.
	ErrTerminal         = errPrivateWorkTerminal
	ErrOverflow         = errPrivateWorkOverflow
	ErrIdentityMismatch = errPrivateWorkIdentity
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
	mu       sync.Mutex
	identity occurrenceIdentity
	work     privateWorkDelta
	terminal bool
	failure  error
}

func newPrivateWorkAccount(identity occurrenceIdentity) *privateWorkAccount {
	return &privateWorkAccount{identity: identity}
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
		overflows(a.work.eventsEmitted, delta.eventsEmitted) ||
		overflows(a.work.documentsAcquired, delta.documentsAcquired) ||
		overflows(a.work.logicalBufferBytesReserved, delta.logicalBufferBytesReserved) {
		a.terminal = true
		a.failure = errPrivateWorkOverflow
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

func (a *Account) ChargeEventsEmitted(identity string, amount uint64) error {
	return a.charge(identity, privateWorkDelta{eventsEmitted: amount})
}

func (a *Account) ChargeDocumentsAcquired(identity string, amount uint64) error {
	return a.charge(identity, privateWorkDelta{documentsAcquired: amount})
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
	a.private.mu.Lock()
	defer a.private.mu.Unlock()
	if !a.private.terminal {
		a.private.terminal = true
	}
	return exportSnapshot(a.private.snapshotLocked(), a.private.terminal, a.private.failure)
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
