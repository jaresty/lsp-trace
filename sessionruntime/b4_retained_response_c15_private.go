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

var ErrRetainedResponseLeaseClosed = errors.New("retained response lease closed")

type privateB4RetainedResponseOwner struct {
	mu                   sync.Mutex
	cond                 sync.Cond
	responses            []lspwire.Message
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

type RetainedResponseLease struct {
	owner *privateB4RetainedResponseOwner
}

func (r RoundTripResult) PrivateRetainedResponseLease() (RetainedResponseLease, bool) {
	if r.Failure != "" || r.privateB4RetainedResponses == nil {
		return RetainedResponseLease{}, false
	}
	o := r.privateB4RetainedResponses
	o.mu.Lock()
	live := !o.admissionClosed && !o.released
	o.mu.Unlock()
	if !live {
		return RetainedResponseLease{}, false
	}
	return RetainedResponseLease{owner: o}, true
}

func retainedResponseCapacity(messages []lspwire.Message) (uint64, bool) {
	messageBytes := uint64(unsafe.Sizeof(lspwire.Message{}))
	if uint64(len(messages)) > math.MaxUint64/messageBytes {
		return 0, false
	}
	total := uint64(len(messages)) * messageBytes
	add := func(n uint64) bool {
		if n > math.MaxUint64-total {
			return false
		}
		total += n
		return true
	}
	for i := range messages {
		m := &messages[i]
		for _, n := range []int{len(m.JSONRPC), len(m.ID), len(m.Method), len(m.Params), len(m.Result)} {
			if !add(uint64(n)) {
				return 0, false
			}
		}
		if m.Error != nil {
			if !add(uint64(unsafe.Sizeof(lspwire.RPCError{}))) || !add(uint64(len(m.Error.Message))) || !add(uint64(len(m.Error.Data))) {
				return 0, false
			}
		}
	}
	return total, true
}

func cloneRetainedResponse(source lspwire.Message) lspwire.Message {
	m := source
	m.JSONRPC = strings.Clone(source.JSONRPC)
	m.Method = strings.Clone(source.Method)
	clone := func(source []byte) []byte {
		if len(source) == 0 {
			return nil
		}
		result := make([]byte, len(source))
		copy(result, source)
		return result
	}
	m.ID = clone(source.ID)
	m.Params = clone(source.Params)
	m.Result = clone(source.Result)
	if source.Error != nil {
		m.Error = &lspwire.RPCError{Code: source.Error.Code, Message: strings.Clone(source.Error.Message), Data: clone(source.Error.Data)}
	}
	return m
}

func newPrivateB4RetainedResponseOwner(account *privateB4ByteAccountV2, source []lspwire.Message) (*privateB4RetainedResponseOwner, session.Failure) {
	if len(source) == 0 || account == nil {
		return nil, ""
	}
	capacity, ok := retainedResponseCapacity(source)
	if !ok {
		return nil, session.ResourceExhausted
	}
	_ = privateC15SiteC15Copy30RetainedResponseSet
	transactionID, objectID, sequence := "", "", uint64(0)
	reachPrivateC18OwnerBarrier(transactionID, objectID, sequence, privateC18RetentionCommit)
	charge, failure := account.reservePrivateB4C15(capacity, privateC15SiteC15Copy30RetainedResponseSet, privateB4C15Operation("copy"))
	if failure != "" {
		return nil, failure
	}
	responses := make([]lspwire.Message, len(source))
	for i := range source {
		responses[i] = cloneRetainedResponse(source[i])
	}
	o := &privateB4RetainedResponseOwner{responses: responses, charge: charge, c18Capacity: capacity, c18Pair: privateC18NewPairCoordinator(9)}
	o.cond.L = &o.mu
	return o, ""
}

// WithResponses exposes callback-scoped, read-only response views. The slice,
// messages, and all backing fields must not escape fn.
func (l RetainedResponseLease) WithResponses(fn func([]lspwire.Message) error) error {
	if l.owner == nil || fn == nil {
		return ErrRetainedResponseLeaseClosed
	}
	o := l.owner
	o.mu.Lock()
	live := !o.admissionClosed && !o.released
	o.mu.Unlock()
	if !live {
		return ErrRetainedResponseLeaseClosed
	}
	_ = privateC15SiteC15Alias31ResponsesCallback
	transactionID, objectID, sequence := o.privateC18Metadata()
	if err := o.reachPrivateC18OwnerBarrier(transactionID, objectID, sequence, privateC18ReadbackBorrow); err != nil {
		return ErrRetainedResponseLeaseClosed
	}
	failure := o.charge.account.aliasPrivateB4C15(o.charge.lease, privateC15SiteC15Alias31ResponsesCallback)
	o.mu.Lock()
	admitted := failure == "" && !o.admissionClosed && !o.released
	var responses []lspwire.Message
	if admitted {
		o.active++
		responses = o.responses
	}
	o.mu.Unlock()
	if !admitted {
		return ErrRetainedResponseLeaseClosed
	}
	defer func() {
		o.mu.Lock()
		o.active--
		if o.active == 0 {
			o.cond.Broadcast()
		}
		o.mu.Unlock()
	}()
	return fn(responses)
}

func (o *privateB4RetainedResponseOwner) requestTerminal() {
	if o == nil {
		return
	}
	o.mu.Lock()
	o.admissionClosed = true
	o.cond.Broadcast()
	o.mu.Unlock()
}

func (o *privateB4RetainedResponseOwner) waitReleased() {
	if o == nil {
		return
	}
	o.mu.Lock()
	for !o.released {
		o.cond.Wait()
	}
	o.mu.Unlock()
}

func (o *privateB4RetainedResponseOwner) preparePrivateC18Release() (string, string, uint64, bool) {
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

func (o *privateB4RetainedResponseOwner) cancelPrivateC18Release() {
	o.mu.Lock()
	o.releaseStarted = false
	o.admissionClosed = false
	o.cond.Broadcast()
	o.mu.Unlock()
}

func (o *privateB4RetainedResponseOwner) finishPrivateC18Release() {
	o.mu.Lock()
	for i := range o.responses {
		m := &o.responses[i]
		for j := range m.ID {
			m.ID[j] = 0
		}
		for j := range m.Params {
			m.Params[j] = 0
		}
		for j := range m.Result {
			m.Result[j] = 0
		}
		if m.Error != nil {
			for j := range m.Error.Data {
				m.Error.Data[j] = 0
			}
		}
	}
	o.responses = nil
	o.released = true
	o.cond.Broadcast()
	o.mu.Unlock()
}

func (o *privateB4RetainedResponseOwner) restartPrivateC18Custody(generation uint64) bool {
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
	charge, failure := account.reservePrivateB4C15(capacity, privateC15SiteC15Copy30RetainedResponseSet, privateB4C15Operation("copy"))
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

func (l RetainedResponseLease) Release() bool {
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
