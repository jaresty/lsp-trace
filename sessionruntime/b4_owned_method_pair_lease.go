package sessionruntime

import (
	"errors"
	"math"
	"sync"

	"lsp-trace/internal/lspwire"
	"lsp-trace/internal/session"
)

var ErrOwnedMethodPairLeaseClosed = errors.New("owned method pair lease closed")

type privateB4OwnedMethodPairHooks struct {
	beforeReserve func(*privateB4ByteAccountV2, uint64)
	afterOwned    func(*privateB4ByteAccountV2, uint64)
	refused       func(*privateB4ByteAccountV2, uint64)
}

type OwnedMethodPairView struct {
	SessionID  string
	Generation uint64
	Key        lspwire.RequestKey
	Method     string
	Params     []byte
	Result     []byte
	Write      RequestWriteObservation
	Read       ResponseReadObservation
	Source     *OwnedDocumentBinding
}

type privateB4OwnedMethodPairOwner struct {
	mu              sync.Mutex
	cond            sync.Cond
	pair            OwnedMethodPair
	charge          privateB4ByteLeaseV2
	active          uint32
	admissionClosed bool
	releaseStarted  bool
	released        bool
}

type OwnedMethodPairLease struct {
	owner *privateB4OwnedMethodPairOwner
}

func (r RoundTripResult) PrivateOwnedMethodPairLease() (OwnedMethodPairLease, bool) {
	if r.Failure != "" || r.ServerError != nil || r.privateB4OwnedMethodPair == nil {
		return OwnedMethodPairLease{}, false
	}
	o := r.privateB4OwnedMethodPair
	o.mu.Lock()
	live := !o.admissionClosed && !o.released
	o.mu.Unlock()
	if !live {
		return OwnedMethodPairLease{}, false
	}
	return OwnedMethodPairLease{owner: o}, true
}

func newPrivateB4OwnedMethodPairOwner(account *privateB4ByteAccountV2, source OwnedMethodPair) (*privateB4OwnedMethodPairOwner, session.Failure) {
	if account == nil || uint64(len(source.Params)) > math.MaxUint64-uint64(len(source.Result)) {
		return nil, session.ResourceExhausted
	}
	charge, failure := account.reserve(uint64(len(source.Params)) + uint64(len(source.Result)))
	if failure != "" {
		return nil, failure
	}
	pair := source
	pair.Params = append([]byte(nil), source.Params...)
	pair.Result = append([]byte(nil), source.Result...)
	if source.Source != nil {
		copied := *source.Source
		pair.Source = &copied
	}
	o := &privateB4OwnedMethodPairOwner{pair: pair, charge: charge}
	o.cond.L = &o.mu
	return o, ""
}

func (l OwnedMethodPairLease) WithPair(fn func(OwnedMethodPairView) error) error {
	if l.owner == nil || fn == nil {
		return ErrOwnedMethodPairLeaseClosed
	}
	o := l.owner
	o.mu.Lock()
	if o.admissionClosed || o.released {
		o.mu.Unlock()
		return ErrOwnedMethodPairLeaseClosed
	}
	o.active++
	p := &o.pair
	view := OwnedMethodPairView{SessionID: p.SessionID, Generation: p.Generation, Key: p.Key, Method: p.Method, Params: p.Params, Result: p.Result, Write: p.Write, Read: p.Read, Source: p.Source}
	o.mu.Unlock()
	defer func() {
		o.mu.Lock()
		o.active--
		if o.active == 0 {
			o.cond.Broadcast()
		}
		o.mu.Unlock()
	}()
	return fn(view)
}

func (o *privateB4OwnedMethodPairOwner) requestTerminal() {
	if o == nil {
		return
	}
	o.mu.Lock()
	o.admissionClosed = true
	o.cond.Broadcast()
	o.mu.Unlock()
}

func (o *privateB4OwnedMethodPairOwner) waitReleased() {
	if o == nil {
		return
	}
	o.mu.Lock()
	for !o.released {
		o.cond.Wait()
	}
	o.mu.Unlock()
}

func (l OwnedMethodPairLease) Release() bool {
	if l.owner == nil {
		return false
	}
	o := l.owner
	o.mu.Lock()
	if o.releaseStarted || o.released {
		o.mu.Unlock()
		return false
	}
	o.admissionClosed = true
	o.releaseStarted = true
	o.cond.Broadcast()
	for o.active != 0 {
		o.cond.Wait()
	}
	o.pair = OwnedMethodPair{}
	o.charge.release()
	o.released = true
	o.cond.Broadcast()
	o.mu.Unlock()
	return true
}
