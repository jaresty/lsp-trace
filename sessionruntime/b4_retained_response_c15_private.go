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
	mu              sync.Mutex
	cond            sync.Cond
	responses       []lspwire.Message
	charge          privateB4ByteLeaseV2
	active          uint32
	admissionClosed bool
	releaseStarted  bool
	released        bool
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
	charge, failure := account.reserve(capacity)
	if failure != "" {
		return nil, failure
	}
	responses := make([]lspwire.Message, len(source))
	for i := range source {
		responses[i] = cloneRetainedResponse(source[i])
	}
	o := &privateB4RetainedResponseOwner{responses: responses, charge: charge}
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
	if o.admissionClosed || o.released {
		o.mu.Unlock()
		return ErrRetainedResponseLeaseClosed
	}
	o.active++
	responses := o.responses
	o.mu.Unlock()
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

func (l RetainedResponseLease) Release() bool {
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
	o.charge.release()
	o.released = true
	o.cond.Broadcast()
	o.mu.Unlock()
	return true
}
