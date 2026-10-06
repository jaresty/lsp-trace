package sessionruntime

import (
	"errors"
	"sync"

	"lsp-trace/internal/lspwire"
)

var (
	errPrivateB4ResultClosing  = errors.New("private B4 result lease closing")
	errPrivateB4ResultReleased = errors.New("private B4 result lease released")
	errPrivateB4ResultCallback = errors.New("private B4 result callback is nil")
)

type privateB4ResultState uint8

const (
	privateB4ResultOpen privateB4ResultState = iota
	privateB4ResultClosing
	privateB4ResultReleased
)

type privateB4ResultOwner struct {
	mu          sync.Mutex
	callbackMu  sync.Mutex
	state       privateB4ResultState
	active      uint32
	managerHeld bool
	callerHeld  bool
	bytes       []byte
	charge      lspwire.SuccessorAllocationLease
}

type B4ResultLease struct{ owner *privateB4ResultOwner }

func (r RoundTripResult) PrivateB4ResultLease() (B4ResultLease, bool) {
	if r.privateB4Result == nil {
		return B4ResultLease{}, false
	}
	return B4ResultLease{owner: r.privateB4Result}, true
}

func (l *B4ResultLease) WithBytes(fn func([]byte) error) error {
	if l == nil || l.owner == nil {
		return errPrivateB4ResultReleased
	}
	if fn == nil {
		return errPrivateB4ResultCallback
	}
	o := l.owner
	o.mu.Lock()
	if o.state != privateB4ResultOpen || !o.callerHeld {
		err := errPrivateB4ResultClosing
		if o.state == privateB4ResultReleased {
			err = errPrivateB4ResultReleased
		}
		o.mu.Unlock()
		return err
	}
	o.active++
	backing := o.bytes
	o.mu.Unlock()

	o.callbackMu.Lock()
	err := fn(backing)
	o.callbackMu.Unlock()

	o.mu.Lock()
	o.active--
	o.finalizeIfUnheldLocked()
	o.mu.Unlock()
	return err
}

func (l *B4ResultLease) Release() bool {
	if l == nil || l.owner == nil {
		return false
	}
	o := l.owner
	o.mu.Lock()
	defer o.mu.Unlock()
	if !o.callerHeld || o.state != privateB4ResultOpen {
		return false
	}
	o.callerHeld = false
	o.state = privateB4ResultClosing
	return o.finalizeIfUnheldLocked()
}

func (o *privateB4ResultOwner) managerBytes() ([]byte, bool) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if !o.managerHeld || o.state == privateB4ResultReleased || o.bytes == nil {
		return nil, false
	}
	return o.bytes, true
}

func (o *privateB4ResultOwner) acquireCaller() bool {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.state != privateB4ResultOpen || o.callerHeld {
		return false
	}
	o.callerHeld = true
	return true
}

func (o *privateB4ResultOwner) releaseManager() {
	o.mu.Lock()
	defer o.mu.Unlock()
	if !o.managerHeld {
		return
	}
	o.managerHeld = false
	o.finalizeIfUnheldLocked()
}

func (o *privateB4ResultOwner) finalizeIfUnheldLocked() bool {
	if o.state == privateB4ResultReleased || o.managerHeld || o.callerHeld || o.active != 0 {
		return false
	}
	for i := range o.bytes {
		o.bytes[i] = 0
	}
	o.bytes = nil
	if o.charge != nil {
		o.charge.Release()
		o.charge = nil
	}
	o.state = privateB4ResultReleased
	return true
}
