package sessionruntime

import (
	"context"
	"errors"
	"sync"
	"unsafe"

	"lsp-trace/internal/session"
)

const privateB4ObjectLimit = uint64(4096)

var (
	errPrivateB4ObjectAdmission = errors.New("private B4 object admission refused")
	// ErrPrivateB4C16NotEnabled is returned without invoking the callback when
	// the private marked request did not enable the experimental C16 profile.
	ErrPrivateB4C16NotEnabled = errors.New("private B4 object admission refused: C16_NOT_ENABLED")
)

// privateB4AccountC16 is a separately allocated, versioned profile. The legacy
// C15 account remains unchanged and is referenced rather than embedded.
type privateB4AccountC16 struct {
	bytes *privateB4ByteAccountV2

	mu                sync.Mutex
	settled           sync.Cond
	materialized      uint64
	inFlight          uint64
	activeCallbacks   uint64
	terminalRequested bool
	finalizing        bool
	backingReleased   bool
	backing           privateB4ByteLeaseV2
}

func privateB4AccountC16SelfCost() uint64 { return uint64(unsafe.Sizeof(privateB4AccountC16{})) }

func newPrivateB4AccountC16() (*privateB4AccountC16, session.Failure) {
	bytes, failure := newPrivateB4ByteAccountV2()
	if failure != "" {
		return nil, failure
	}
	profile, failure := newPrivateB4AccountC16WithBytes(bytes)
	if failure != "" {
		bytes.requestTerminalRelease()
	}
	return profile, failure
}

func newPrivateB4AccountC16WithBytes(bytes *privateB4ByteAccountV2) (*privateB4AccountC16, session.Failure) {
	if bytes == nil {
		return nil, session.ResourceExhausted
	}
	backing, failure := bytes.reserve(privateB4AccountC16SelfCost())
	if failure != "" {
		return nil, failure
	}
	profile := &privateB4AccountC16{bytes: bytes, backing: backing}
	profile.settled.L = &profile.mu
	return profile, ""
}

func (a *privateB4AccountC16) WithObjectAdmission(fn func() error) (err error) {
	if a == nil || fn == nil {
		return errPrivateB4ObjectAdmission
	}
	a.mu.Lock()
	if a.terminalRequested || a.backingReleased || a.materialized+a.inFlight >= privateB4ObjectLimit {
		a.mu.Unlock()
		return errPrivateB4ObjectAdmission
	}
	a.inFlight++
	a.activeCallbacks++
	a.mu.Unlock()
	completed := false
	defer func() {
		a.mu.Lock()
		a.inFlight--
		if completed && err == nil {
			a.materialized++
		}
		a.activeCallbacks--
		if a.terminalRequested && a.activeCallbacks == 0 {
			a.settled.Broadcast()
		}
		a.mu.Unlock()
	}()
	err = fn()
	completed = true
	return err
}

func (a *privateB4AccountC16) beginBorrow() error {
	if a == nil {
		return ErrPrivateB4C16NotEnabled
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.terminalRequested || a.backingReleased {
		return errPrivateB4ObjectAdmission
	}
	a.activeCallbacks++
	return nil
}

func (a *privateB4AccountC16) endBorrow() {
	if a == nil {
		return
	}
	a.mu.Lock()
	a.activeCallbacks--
	if a.terminalRequested && a.activeCallbacks == 0 {
		a.settled.Broadcast()
	}
	a.mu.Unlock()
}

func (a *privateB4AccountC16) requestTerminalReleaseC16(ctx context.Context) session.Failure {
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
	a.mu.Unlock()
	backing.release()
	a.bytes.requestTerminalRelease()
	a.mu.Lock()
	a.finalizing = false
	a.backingReleased = true
	a.settled.Broadcast()
	a.mu.Unlock()
	return ""
}

type privateB4ObjectSnapshot struct {
	Materialized, InFlight, ActiveCallbacks uint64
	TerminalRequested, BackingReleased      bool
}

func (a *privateB4AccountC16) objectSnapshot() privateB4ObjectSnapshot {
	if a == nil {
		return privateB4ObjectSnapshot{}
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	return privateB4ObjectSnapshot{Materialized: a.materialized, InFlight: a.inFlight, ActiveCallbacks: a.activeCallbacks, TerminalRequested: a.terminalRequested, BackingReleased: a.backingReleased}
}
