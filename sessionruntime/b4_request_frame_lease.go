package sessionruntime

import (
	"errors"
	"sync"
)

var (
	errPrivateB4RequestFrameClosing  = errors.New("private B4 request-frame lease closing")
	errPrivateB4RequestFrameReleased = errors.New("private B4 request-frame lease released")
	errPrivateB4RequestFrameCallback = errors.New("private B4 request-frame callback is nil")
)

type privateB4RequestFrameState uint8

const (
	privateB4RequestFrameOpen privateB4RequestFrameState = iota
	privateB4RequestFrameClosing
	privateB4RequestFrameReleased
)

type privateB4RequestFrameOwner struct {
	mu         sync.Mutex
	callbackMu sync.Mutex
	state      privateB4RequestFrameState
	active     uint32
	bytes      []byte
	charge     privateB4ByteLeaseV2
}

// B4RequestFrameLease is an opaque, private-B4-specific ownership capability.
// Bytes supplied to WithBytes are read-only evidence and valid only for the
// dynamic extent of the callback. Mutation or retention of the slice or any
// alias after callback return is prohibited misuse.
type B4RequestFrameLease struct{ owner *privateB4RequestFrameOwner }

// PrivateB4RequestFrameLease returns the caller-owned request-frame lease only
// for a successful custodial private-B4 result.
func (r RoundTripResult) PrivateB4RequestFrameLease() (B4RequestFrameLease, bool) {
	if r.privateB4RequestFrame == nil {
		return B4RequestFrameLease{}, false
	}
	return B4RequestFrameLease{owner: r.privateB4RequestFrame}, true
}

// WithBytes serializes callback execution while allowing Release to request
// nonblocking deferred finalization. A borrow accepted before closing runs even
// when queued behind another callback.
func (l *B4RequestFrameLease) WithBytes(fn func([]byte) error) error {
	if l == nil || l.owner == nil {
		return errPrivateB4RequestFrameReleased
	}
	if fn == nil {
		return errPrivateB4RequestFrameCallback
	}
	o := l.owner
	o.mu.Lock()
	if o.state != privateB4RequestFrameOpen {
		err := errPrivateB4RequestFrameClosing
		if o.state == privateB4RequestFrameReleased {
			err = errPrivateB4RequestFrameReleased
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
	if o.state == privateB4RequestFrameClosing && o.active == 0 {
		o.finalizeLocked()
	}
	o.mu.Unlock()
	return err
}

// Release requests terminal release and never waits for an active callback.
// True means finalization completed synchronously; false means it was deferred
// or the lease was already closing/released.
func (l *B4RequestFrameLease) Release() bool {
	if l == nil || l.owner == nil {
		return false
	}
	o := l.owner
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.state != privateB4RequestFrameOpen {
		return false
	}
	o.state = privateB4RequestFrameClosing
	if o.active != 0 {
		return false
	}
	o.finalizeLocked()
	return true
}

func (o *privateB4RequestFrameOwner) finalizeLocked() {
	if o.state == privateB4RequestFrameReleased {
		return
	}
	for i := range o.bytes {
		o.bytes[i] = 0
	}
	o.bytes = nil
	o.charge.release()
	o.state = privateB4RequestFrameReleased
}
