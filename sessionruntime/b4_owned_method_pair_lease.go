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
	resultCharge    privateB4ByteLeaseV2
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
	var charges [2]privateB4ByteLeaseV2
	creations := [...]privateB4C15Creation{
		{siteID: privateC15SiteC15Copy34OwnedPairParams, operation: privateB4C15Operation("copy")},
		{siteID: privateC15SiteC15Copy35OwnedPairResult, operation: privateB4C15Operation("copy")},
	}
	if _, failure := account.reserveManyPrivateB4C15(
		[]uint64{uint64(len(source.Params)), uint64(len(source.Result))}, creations[:], charges[:],
	); failure != "" {
		return nil, failure
	}
	pair := source
	_ = privateC15SiteC15Copy34OwnedPairParams
	pair.Params = append([]byte(nil), source.Params...)
	_ = privateC15SiteC15Copy35OwnedPairResult
	pair.Result = append([]byte(nil), source.Result...)
	if source.Source != nil {
		copied := *source.Source
		pair.Source = &copied
	}
	o := &privateB4OwnedMethodPairOwner{pair: pair, charge: charges[0], resultCharge: charges[1]}
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
	_ = privateC15SiteC15Alias36OwnedPairCallback
	if failure := o.charge.account.aliasPrivateB4C15(o.charge.lease, privateC15SiteC15Alias36OwnedPairCallback); failure != "" {
		o.mu.Unlock()
		return errors.New(string(failure))
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
	o.resultCharge.release()
	o.released = true
	o.cond.Broadcast()
	o.mu.Unlock()
	return true
}

type callHierarchyObservationOwner struct {
	mu              sync.Mutex
	cond            sync.Cond
	observation     callHierarchyObservation
	active          uint32
	admissionClosed bool
	releaseStarted  bool
	released        bool
}

type callHierarchyObservationLease struct {
	owner *callHierarchyObservationOwner
}

func newCallHierarchyObservationOwner(source callHierarchyObservation) *callHierarchyObservationOwner {
	owner := &callHierarchyObservationOwner{observation: copyCallHierarchyObservation(source)}
	owner.cond.L = &owner.mu
	return owner
}

func copyCallHierarchyObservation(source callHierarchyObservation) callHierarchyObservation {
	owned := source
	owned.predecessors.commitments = append([]callHierarchyPredecessorCommitment(nil), source.predecessors.commitments...)
	if source.request != nil {
		request := *source.request
		request.frameBytes = append([]byte(nil), source.request.frameBytes...)
		owned.request = &request
	}
	if source.response != nil {
		response := *source.response
		response.frameBytes = append([]byte(nil), source.response.frameBytes...)
		owned.response = &response
	}
	if source.selected != nil {
		selected := *source.selected
		selected.exactBytes = append([]byte(nil), source.selected.exactBytes...)
		owned.selected = &selected
	}
	if source.handoff != nil {
		handoff := *source.handoff
		handoff.selected.exactBytes = append([]byte(nil), source.handoff.selected.exactBytes...)
		handoff.successorBytes = append([]byte(nil), source.handoff.successorBytes...)
		owned.handoff = &handoff
	}
	return owned
}

func callHierarchyObservationViewOf(source callHierarchyObservation) callHierarchyObservationView {
	owned := copyCallHierarchyObservation(source)
	return callHierarchyObservationView{
		identity: owned.identity, predecessors: owned.predecessors.commitments,
		request: owned.request, response: owned.response, selected: owned.selected,
		handoff: owned.handoff, terminal: owned.terminal, accounting: owned.accounting,
	}
}

func (lease callHierarchyObservationLease) withObservation(fn func(callHierarchyObservationView) error) (err error) {
	if lease.owner == nil || fn == nil {
		return ErrOwnedMethodPairLeaseClosed
	}
	owner := lease.owner
	owner.mu.Lock()
	if owner.admissionClosed || owner.released {
		owner.mu.Unlock()
		return ErrOwnedMethodPairLeaseClosed
	}
	owner.active++
	view := callHierarchyObservationViewOf(owner.observation)
	owner.mu.Unlock()
	defer func() {
		if recover() != nil {
			err = errors.New("call hierarchy observation callback panicked")
		}
		owner.mu.Lock()
		owner.active--
		if owner.active == 0 {
			owner.cond.Broadcast()
		}
		owner.mu.Unlock()
	}()
	return fn(view)
}

func (owner *callHierarchyObservationOwner) requestTerminal() {
	if owner == nil {
		return
	}
	owner.mu.Lock()
	owner.admissionClosed = true
	owner.cond.Broadcast()
	owner.mu.Unlock()
}

func (lease callHierarchyObservationLease) release() bool {
	if lease.owner == nil {
		return false
	}
	owner := lease.owner
	owner.mu.Lock()
	if owner.releaseStarted || owner.released {
		owner.mu.Unlock()
		return false
	}
	owner.admissionClosed = true
	owner.releaseStarted = true
	owner.cond.Broadcast()
	for owner.active != 0 {
		owner.cond.Wait()
	}
	owner.observation = callHierarchyObservation{}
	owner.released = true
	owner.cond.Broadcast()
	owner.mu.Unlock()
	return true
}
