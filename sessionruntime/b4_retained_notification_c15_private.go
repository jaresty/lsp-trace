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
	mu                   sync.Mutex
	cond                 sync.Cond
	notifications        []lspwire.Message
	charge               privateB4ByteLeaseV2
	c18Capacity          uint64
	active               uint32
	admissionClosed      bool
	releaseStarted       bool
	released             bool
	c18Barrier           privateC18OwnerBarrier
	c18BarrierGeneration uint64
	c18BarrierToken      *struct{}
	c18Pair              privateC18PairCoordinator
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
	_ = privateC15SiteC15Copy28RetainedNotificationSet
	transactionID, objectID, sequence := "", "", uint64(0)
	reachPrivateC18OwnerBarrier(transactionID, objectID, sequence, privateC18RetentionCommit)
	charge, failure := account.reservePrivateB4C15(capacity, privateC15SiteC15Copy28RetainedNotificationSet, privateB4C15Operation("copy"))
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
	o := &privateB4RetainedNotificationOwner{notifications: messages, charge: charge, c18Capacity: capacity, c18Pair: privateC18NewPairCoordinator(8)}
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
	live := !o.admissionClosed && !o.released
	o.mu.Unlock()
	if !live {
		return ErrRetainedNotificationLeaseClosed
	}
	_ = privateC15SiteC15Alias29NotificationsCallback
	transactionID, objectID, sequence := o.privateC18Metadata()
	if err := o.reachPrivateC18OwnerBarrier(transactionID, objectID, sequence, privateC18ReadbackBorrow); err != nil {
		return ErrRetainedNotificationLeaseClosed
	}
	failure := o.charge.account.aliasPrivateB4C15(o.charge.lease, privateC15SiteC15Alias29NotificationsCallback)
	o.mu.Lock()
	admitted := failure == "" && !o.admissionClosed && !o.released
	var messages []lspwire.Message
	if admitted {
		o.active++
		messages = o.notifications
	}
	o.mu.Unlock()
	if !admitted {
		return ErrRetainedNotificationLeaseClosed
	}
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

func (o *privateB4RetainedNotificationOwner) preparePrivateC18Release() (string, string, uint64, bool) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.releaseStarted || o.released {
		return "", "", 0, false
	}
	o.admissionClosed = true
	o.releaseStarted = true
	o.cond.Broadcast()
	for o.active != 0 {
		o.cond.Wait()
	}
	transactionID, objectID, sequence := o.privateC18MetadataLocked()
	return transactionID, objectID, sequence, true
}

func (o *privateB4RetainedNotificationOwner) cancelPrivateC18Release() {
	o.mu.Lock()
	o.releaseStarted = false
	o.admissionClosed = false
	o.cond.Broadcast()
	o.mu.Unlock()
}

func (o *privateB4RetainedNotificationOwner) finishPrivateC18Release() {
	o.mu.Lock()
	for i := range o.notifications {
		for j := range o.notifications[i].Params {
			o.notifications[i].Params[j] = 0
		}
	}
	o.notifications = nil
	o.released = true
	o.cond.Broadcast()
	o.mu.Unlock()
}

func (o *privateB4RetainedNotificationOwner) restartPrivateC18Custody(generation uint64) bool {
	return false
	/* permanently closed owner
	o.mu.Lock()
	if !o.released || o.active != 0 || o.charge.account == nil {
		o.mu.Unlock()
		return false
	}
	account, capacity := o.charge.account, o.c18Capacity
	o.mu.Unlock()

	account.mu.Lock()
	if account.c15 != nil {
		account.c15.generation++
		account.c15.transactionID += "-restart-" + privateC18Decimal(generation)
	}
	account.mu.Unlock()
	charge, failure := account.reservePrivateB4C15(capacity, privateC15SiteC15Copy28RetainedNotificationSet, privateB4C15Operation("copy"))
	if failure != "" {
		return false
	}
	o.mu.Lock()
	o.charge = charge
	o.admissionClosed = false
	o.releaseStarted = false
	o.released = false
	o.mu.Unlock()
	return true
	*/
}

func (l RetainedNotificationLease) Release() bool {
	if l.owner == nil {
		return false
	}
	o := l.owner
	transactionID, objectID, sequence, release := o.preparePrivateC18Release()
	if !release {
		return false
	}
	if err := o.reachPrivateC18OwnerBarrier(transactionID, objectID, sequence, privateC18Release); err != nil {
		o.cancelPrivateC18Release()
		return false
	}
	o.charge.release()
	o.finishPrivateC18Release()
	return true
}
