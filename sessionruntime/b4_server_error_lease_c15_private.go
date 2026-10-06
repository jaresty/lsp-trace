package sessionruntime

import (
	"errors"
	"math"
	"strings"
	"sync"

	"lsp-trace/internal/lspwire"
	"lsp-trace/internal/session"
)

var ErrServerErrorLeaseClosed = errors.New("server error lease closed")

type ServerErrorView struct {
	Code    int
	Message string
	Data    []byte
}

type privateB4ServerErrorOwner struct {
	mu              sync.Mutex
	cond            sync.Cond
	value           ServerErrorView
	charge          privateB4ByteLeaseV2
	active          uint32
	admissionClosed bool
	releaseStarted  bool
	released        bool
	manager         *Manager
	reservation     *privateB4Reservation
}

type ServerErrorLease struct {
	owner *privateB4ServerErrorOwner
}

func (r RoundTripResult) PrivateServerErrorLease() (ServerErrorLease, bool) {
	if r.Failure != "" || r.privateB4ServerError == nil {
		return ServerErrorLease{}, false
	}
	o := r.privateB4ServerError
	o.mu.Lock()
	live := !o.admissionClosed && !o.released
	o.mu.Unlock()
	if !live {
		return ServerErrorLease{}, false
	}
	return ServerErrorLease{owner: o}, true
}

func privateB4ServerErrorCapacity(source *lspwire.RPCError) (uint64, bool) {
	if source == nil {
		return 0, true
	}
	message := uint64(len(source.Message))
	data := uint64(len(source.Data))
	if message > math.MaxUint64-data {
		return 0, false
	}
	return message + data, true
}

func newPrivateB4ServerErrorOwner(account *privateB4ByteAccountV2, source *lspwire.RPCError) (*privateB4ServerErrorOwner, session.Failure) {
	if source == nil || account == nil {
		return nil, ""
	}
	capacity, ok := privateB4ServerErrorCapacity(source)
	if !ok {
		return nil, session.ResourceExhausted
	}
	charge, failure := account.reserve(capacity)
	if failure != "" {
		return nil, failure
	}
	value := ServerErrorView{Code: source.Code, Message: strings.Clone(source.Message)}
	if len(source.Data) != 0 {
		value.Data = make([]byte, len(source.Data))
		copy(value.Data, source.Data)
	}
	o := &privateB4ServerErrorOwner{value: value, charge: charge}
	o.cond.L = &o.mu
	return o, ""
}

// WithServerError exposes callback-scoped, read-only Message and Data backing.
// The view and its backing must not escape fn.
func (l ServerErrorLease) WithServerError(fn func(ServerErrorView) error) error {
	if l.owner == nil || fn == nil {
		return ErrServerErrorLeaseClosed
	}
	o := l.owner
	o.mu.Lock()
	if o.admissionClosed || o.released {
		o.mu.Unlock()
		return ErrServerErrorLeaseClosed
	}
	o.active++
	value := o.value
	o.mu.Unlock()
	defer func() {
		o.mu.Lock()
		o.active--
		if o.active == 0 {
			o.cond.Broadcast()
		}
		o.mu.Unlock()
	}()
	return fn(value)
}

func (o *privateB4ServerErrorOwner) bindRetirement(manager *Manager, reservation *privateB4Reservation) {
	o.mu.Lock()
	o.manager = manager
	o.reservation = reservation
	o.mu.Unlock()
}

func (o *privateB4ServerErrorOwner) requestTerminal() {
	if o == nil {
		return
	}
	o.mu.Lock()
	o.admissionClosed = true
	o.cond.Broadcast()
	o.mu.Unlock()
}

func (o *privateB4ServerErrorOwner) waitReleased() {
	if o == nil {
		return
	}
	o.mu.Lock()
	for !o.released {
		o.cond.Wait()
	}
	o.mu.Unlock()
}

func (l ServerErrorLease) Release() bool {
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
	for i := range o.value.Data {
		o.value.Data[i] = 0
	}
	o.value = ServerErrorView{}
	o.charge.release()
	o.released = true
	manager, reservation := o.manager, o.reservation
	o.manager, o.reservation = nil, nil
	o.cond.Broadcast()
	o.mu.Unlock()
	if manager != nil && reservation != nil {
		manager.releasePrivateB4ServerErrorReservation(reservation)
	}
	return true
}
