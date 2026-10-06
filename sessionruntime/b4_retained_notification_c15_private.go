package sessionruntime

import (
	"errors"
	"math"
	"strings"
	"sync"
	"unsafe"

	"lsp-trace/internal/lspwire"
	"lsp-trace/internal/session"
)

var ErrRetainedNotificationLeaseClosed = errors.New("retained notification lease closed")

type privateB4RetainedNotificationOwner struct {
	mu              sync.Mutex
	cond            sync.Cond
	notifications   []lspwire.Message
	charge          privateB4ByteLeaseV2
	active          uint32
	admissionClosed bool
	releaseStarted  bool
	released        bool
}

type RetainedNotificationLease struct {
	owner *privateB4RetainedNotificationOwner
}

// PrivateRetainedNotificationLease returns the private custodial notification
// lease. The zero-notification case deliberately publishes no lease.
func (r RoundTripResult) PrivateRetainedNotificationLease() (RetainedNotificationLease, bool) {
	if r.Failure != "" || r.privateB4RetainedNotifications == nil {
		return RetainedNotificationLease{}, false
	}
	o := r.privateB4RetainedNotifications
	o.mu.Lock()
	live := !o.admissionClosed && !o.released
	o.mu.Unlock()
	if !live {
		return RetainedNotificationLease{}, false
	}
	return RetainedNotificationLease{owner: o}, true
}

func retainedNotificationCapacity(messages []lspwire.Message) (uint64, bool) {
	if uint64(len(messages)) > math.MaxUint64/uint64(unsafe.Sizeof(lspwire.Message{})) {
		return 0, false
	}
	total := uint64(len(messages)) * uint64(unsafe.Sizeof(lspwire.Message{}))
	for i := range messages {
		for _, n := range []int{len(messages[i].JSONRPC), len(messages[i].Method), len(messages[i].Params)} {
			if uint64(n) > math.MaxUint64-total {
				return 0, false
			}
			total += uint64(n)
		}
	}
	return total, true
}

func newPrivateB4RetainedNotificationOwner(account *privateB4ByteAccountV2, source []lspwire.Message) (*privateB4RetainedNotificationOwner, session.Failure) {
	if len(source) == 0 || account == nil {
		return nil, ""
	}
	capacity, ok := retainedNotificationCapacity(source)
	if !ok {
		return nil, session.ResourceExhausted
	}
	charge, failure := account.reserve(capacity)
	if failure != "" {
		return nil, failure
	}
	messages := make([]lspwire.Message, len(source))
	for i := range source {
		messages[i] = source[i]
		messages[i].JSONRPC = strings.Clone(source[i].JSONRPC)
		messages[i].Method = strings.Clone(source[i].Method)
		if len(source[i].Params) != 0 {
			messages[i].Params = make([]byte, len(source[i].Params))
			copy(messages[i].Params, source[i].Params)
		}
	}
	o := &privateB4RetainedNotificationOwner{notifications: messages, charge: charge}
	o.cond.L = &o.mu
	return o, ""
}

// WithNotifications exposes callback-scoped, read-only notification views.
// The slice, messages, and their backing bytes must not escape fn.
func (l RetainedNotificationLease) WithNotifications(fn func([]lspwire.Message) error) error {
	if l.owner == nil || fn == nil {
		return ErrRetainedNotificationLeaseClosed
	}
	o := l.owner
	o.mu.Lock()
	if o.admissionClosed || o.released {
		o.mu.Unlock()
		return ErrRetainedNotificationLeaseClosed
	}
	o.active++
	messages := o.notifications
	o.mu.Unlock()
	defer func() {
		o.mu.Lock()
		o.active--
		if o.active == 0 {
			o.cond.Broadcast()
		}
		o.mu.Unlock()
	}()
	return fn(messages)
}

func (o *privateB4RetainedNotificationOwner) requestTerminal() {
	if o == nil {
		return
	}
	o.mu.Lock()
	o.admissionClosed = true
	o.cond.Broadcast()
	o.mu.Unlock()
}

func (o *privateB4RetainedNotificationOwner) waitReleased() {
	if o == nil {
		return
	}
	o.mu.Lock()
	for !o.released {
		o.cond.Wait()
	}
	o.mu.Unlock()
}

func (l RetainedNotificationLease) Release() bool {
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
	for i := range o.notifications {
		for j := range o.notifications[i].Params {
			o.notifications[i].Params[j] = 0
		}
	}
	o.notifications = nil
	o.charge.release()
	o.released = true
	o.cond.Broadcast()
	o.mu.Unlock()
	return true
}
