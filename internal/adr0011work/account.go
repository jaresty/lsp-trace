package adr0011work

import (
	"errors"
	"math"
	"sync"
)

var (
	errPrivateWorkTerminal = errors.New("private work account terminal")
	errPrivateWorkOverflow = errors.New("private work account overflow")
	errPrivateWorkIdentity = errors.New("private work account identity mismatch")
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
