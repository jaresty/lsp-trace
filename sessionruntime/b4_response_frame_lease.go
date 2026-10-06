package sessionruntime

import (
	"errors"
	"sync"

	"lsp-trace/internal/lspwire"
)

var (
	errPrivateB4ResponseFrameClosing  = errors.New("private B4 response-frame lease closing")
	errPrivateB4ResponseFrameReleased = errors.New("private B4 response-frame lease released")
	errPrivateB4ResponseFrameCallback = errors.New("private B4 response-frame callback is nil")
)

type privateB4ResponseFrameState uint8

const (
	privateB4ResponseFrameOpen privateB4ResponseFrameState = iota
	privateB4ResponseFrameClosing
	privateB4ResponseFrameReleased
)

type privateB4ResponseFrameOwner struct {
	mu          sync.Mutex
	callbackMu  sync.Mutex
	state       privateB4ResponseFrameState
	active      uint32
	managerHeld bool
	callerHeld  bool
	bytes       []byte
	charge      lspwire.SuccessorAllocationLease
}

type B4ResponseFrameLease struct{ owner *privateB4ResponseFrameOwner }

func (r RoundTripResult) PrivateB4ResponseFrameLease() (B4ResponseFrameLease, bool) {
	if r.privateB4ResponseFrame == nil {
		return B4ResponseFrameLease{}, false
	}
	return B4ResponseFrameLease{owner: r.privateB4ResponseFrame}, true
}

func (l *B4ResponseFrameLease) WithBytes(fn func([]byte) error) error {
	if l == nil || l.owner == nil {
		return errPrivateB4ResponseFrameReleased
	}
	if fn == nil {
		return errPrivateB4ResponseFrameCallback
	}
	o := l.owner
	o.mu.Lock()
	if o.state != privateB4ResponseFrameOpen || !o.callerHeld {
		err := errPrivateB4ResponseFrameClosing
		if o.state == privateB4ResponseFrameReleased {
			err = errPrivateB4ResponseFrameReleased
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

func (l *B4ResponseFrameLease) Release() bool {
	if l == nil || l.owner == nil {
		return false
	}
	o := l.owner
	o.mu.Lock()
	defer o.mu.Unlock()
	if !o.callerHeld || o.state != privateB4ResponseFrameOpen {
		return false
	}
	o.callerHeld = false
	o.state = privateB4ResponseFrameClosing
	return o.finalizeIfUnheldLocked()
}

func (o *privateB4ResponseFrameOwner) managerBytes() ([]byte, bool) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if !o.managerHeld || o.state == privateB4ResponseFrameReleased || o.bytes == nil {
		return nil, false
	}
	return o.bytes, true
}

func (o *privateB4ResponseFrameOwner) acquireCaller() bool {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.state != privateB4ResponseFrameOpen || o.callerHeld {
		return false
	}
	o.callerHeld = true
	return true
}

func (o *privateB4ResponseFrameOwner) releaseManager() {
	o.mu.Lock()
	defer o.mu.Unlock()
	if !o.managerHeld {
		return
	}
	o.managerHeld = false
	o.finalizeIfUnheldLocked()
}

func (o *privateB4ResponseFrameOwner) finalizeIfUnheldLocked() bool {
	if o.state == privateB4ResponseFrameReleased || o.managerHeld || o.callerHeld || o.active != 0 {
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
	o.state = privateB4ResponseFrameReleased
	return true
}
