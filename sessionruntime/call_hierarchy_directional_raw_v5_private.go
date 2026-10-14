package sessionruntime

import (
	"context"
	"errors"
	"math"
	"sync"

	"lsp-trace/internal/directionalraw"
	"lsp-trace/internal/lspwire"
	"lsp-trace/internal/session"
)

const privateDirectionalMaxV5 uint64 = 32 << 20
const privateDirectionalCombinedMaxV5 uint64 = 64 << 20

type privateDirectionalPartitionV5 struct {
	live, cumulative, allocations, releases uint64
	occupied                                bool
}
type acceptedV2 struct {
	key          lspwire.RequestKey
	method       string
	observations []directionalraw.Observation
	failure      session.Failure
}

// Snapshots are captured at the physical allocation boundary, not reconstructed
// from final accounting. They carry no raw backing or acceptance authority.
type privateDirectionalAllocationV5 struct {
	Component        string `json:"component"`
	Length           int    `json:"length"`
	Live             uint64 `json:"live"`
	Cumulative       uint64 `json:"cumulative"`
	Allocations      uint64 `json:"allocations"`
	OwnershipObjects int    `json:"ownership_objects"`
	Occupied         bool   `json:"occupied"`
}

type privateDirectionalV5 struct {
	allocationBoundaries []privateDirectionalAllocationV5
	mu                   sync.Mutex
	reservation          directionalraw.Reservation
	account              *privateB4ByteAccountV2
	owner                *callHierarchyLayer1GenericOwner
	accepted             []acceptedV2
	c15Events            []privateB4C15Event
	frameObjects         map[uint64]string
	responseObjects      map[uint64]uint64
	lastResponseObject   uint64
}
type privateDirectionalCallV5 struct {
	execution *privateDirectionalV5
	lease     directionalraw.Lease
	key       lspwire.RequestKey
}

// RoundTripDirectionalPrivate is an exported-private interface precedent like
// RoundTripPrivateB4. Only an explicitly installed carrier enables it; no wire,
// MCP, CLI, host configuration or public request schema exposes this operation.
func (m *Manager) RoundTripDirectionalPrivate(ctx context.Context, req RoundTripRequest) (RoundTripResult, directionalraw.Lease) {
	r, ok := directionalraw.FromContext(ctx)
	if !ok || r.IdentityPrivate() == "" || !directionalraw.Allowed(req.Method) || req.EnablePrivateC16ObjectAccounting || req.EnablePrivateC17EventAccounting {
		return RoundTripResult{Failure: session.ToolNotImplemented}, directionalraw.Lease{}
	}
	req.directionalV5 = &privateDirectionalCallV5{execution: &privateDirectionalV5{reservation: r}}
	// Ordinary RoundTrip is deliberately not changed. The shared transaction owns
	// every session/generation/deadline/protocol and pending-key admission check.
	result := m.roundTripWithPrivateTransaction(ctx, req, nil, &privateB4Transaction{directionalV5: req.directionalV5})
	// External frames remain available only as reserved immutable observations;
	// don't expose Message.RawMessage aliases into our retained frame backing.
	result.Notifications = nil
	result.Responses = nil
	return result, req.directionalV5.lease
}

// Called under Manager.mu, after all ordinary preflight and before pending.Begin.
func (m *Manager) admitDirectionalV5Locked(req RoundTripRequest) session.Failure {
	call := req.directionalV5
	if call == nil {
		return ""
	}
	r := call.execution.reservation
	d := m.directionalV5
	if d != nil && !d.reservation.Closed() && !d.reservation.Same(r) {
		return session.ResourceExhausted
	}
	if d == nil || d.reservation.Closed() {
		if r.Attached() || r.Closed() {
			return session.ResourceExhausted
		}
		d = &privateDirectionalV5{reservation: r, frameObjects: make(map[uint64]string), responseObjects: make(map[uint64]uint64)}
		custody, failure := newPrivateB4C15CustodyContext(req.SessionID, req.Generation, r.IdentityPrivate(), func(events []privateB4C15Event) error {
			d.mu.Lock()
			defer d.mu.Unlock()
			d.c15Events = append(d.c15Events, events...)
			return nil
		})
		if failure != "" {
			return failure
		}
		account, failure := newPrivateB4ByteAccountV2(custody)
		if failure != "" {
			return failure
		}
		d.account = account
		o := newCallHierarchyLayer1GenericOwner(callHierarchyLayer1GenericObservation{})
		o.observation.identity.request.session = callHierarchySessionIdentity(req.SessionID)
		o.observation.identity.request.generation = callHierarchyGeneration(req.Generation)
		o.directionalV5 = d
		d.owner = o
		if !r.Attach(d.allocate, d.observe, func() bool { return (callHierarchyLayer1GenericLease{owner: o}).Release() }, d.releaseAccount) {
			return session.ResourceExhausted
		}
		m.directionalV5 = d
		m.callHierarchyLayer1GenericOwners = append(m.callHierarchyLayer1GenericOwners, o)
	}
	if !d.reservation.Same(r) || string(d.owner.observation.identity.request.session) != req.SessionID || uint64(d.owner.observation.identity.request.generation) != req.Generation {
		return session.ToolNotImplemented
	}
	d.mu.Lock()
	full := len(d.accepted) >= privateB4ByteSlots
	d.mu.Unlock()
	if full {
		return session.ResourceExhausted
	}
	d.owner.mu.Lock()
	defer d.owner.mu.Unlock()
	if d.owner.admissionClosed || d.owner.released {
		return session.LifecycleConflict
	}
	d.owner.active++
	call.execution = d
	return ""
}
func (d *privateDirectionalV5) allocate(component string, n int) ([]byte, error) {
	if component != "request-frame" && component != "response-frame" {
		return nil, errors.New("unknown directional component")
	}
	a := d.account
	a.mu.Lock()
	defer a.mu.Unlock()
	p := &a.directionalV5
	if n <= 0 || a.terminalRequested || a.backingReleased || uint64(n) > privateDirectionalMaxV5-p.live || p.cumulative > math.MaxUint64-uint64(n) || a.table.general.live > privateB4MaxOwnedBytes || a.table.general.live+p.live+uint64(n) > privateDirectionalCombinedMaxV5 {
		return nil, directionalraw.ErrReservationRefused
	}
	site := "c15.directional.v5." + component
	objectID, failure := a.c15.create(site, privateB4C15Operation("allocation"))
	if failure != "" {
		return nil, errors.New(string(failure))
	}
	d.frameObjects[objectID] = site
	if component == "response-frame" {
		d.lastResponseObject = objectID
	}
	p.occupied = true
	p.live += uint64(n)
	p.cumulative += uint64(n)
	p.allocations++
	// Reservation is committed before the only retained allocation. Raw aliases
	// and provider-element Views add no charge and never copy this backing.
	return d.allocateBackingLocked(component, n), nil
}

// The only retained raw make. Called with account.mu held; observe the actual
// quota charge and ownership installation immediately before allocating.
func (d *privateDirectionalV5) allocateBackingLocked(component string, n int) []byte {
	p := &d.account.directionalV5
	d.allocationBoundaries = append(d.allocationBoundaries, privateDirectionalAllocationV5{
		Component: component, Length: n, Live: p.live, Cumulative: p.cumulative,
		Allocations: p.allocations, OwnershipObjects: len(d.frameObjects), Occupied: p.occupied,
	})
	return make([]byte, n)
}
func (d *privateDirectionalV5) observe(o directionalraw.Observation) {
	a := d.account
	a.mu.Lock()
	if o.Component == "response-frame" {
		d.responseObjects[o.RequestID] = d.lastResponseObject
	}
	if o.Component == "element" {
		a.c15.alias(d.responseObjects[o.RequestID], "c15.directional.v5.selected-element")
	}
	a.mu.Unlock()
	d.mu.Lock()
	defer d.mu.Unlock()
	for i := range d.accepted {
		if d.accepted[i].key.ID == o.RequestID && d.accepted[i].key.Generation == o.Generation {
			d.accepted[i].observations = append(d.accepted[i].observations, o)
			return
		}
	}
}
func (d *privateDirectionalV5) releaseAccount() {
	a := d.account
	a.mu.Lock()
	p := &a.directionalV5
	if p.occupied {
		for id, site := range d.frameObjects {
			a.c15.release(id, site)
		}
		p.releases++
		p.live = 0
		p.occupied = false
	}
	a.mu.Unlock()
	a.requestTerminalRelease()
}
func (c *privateDirectionalCallV5) admit(key lspwire.RequestKey, method string) {
	c.key = key
	c.lease = c.execution.reservation.Begin(method, key.ID, key.Generation)
	c.execution.mu.Lock()
	c.execution.accepted = append(c.execution.accepted, acceptedV2{key: key, method: method})
	c.execution.mu.Unlock()
}
func (c *privateDirectionalCallV5) finish(failure session.Failure) {
	c.lease.Complete(string(failure))
	d := c.execution
	d.mu.Lock()
	for i := range d.accepted {
		if d.accepted[i].key == c.key {
			d.accepted[i].failure = failure
		}
	}
	d.mu.Unlock()
	o := d.owner
	o.mu.Lock()
	o.active--
	if o.active == 0 {
		if o.admissionClosed {
			o.releaseLocked()
		}
		o.cond.Broadcast()
	}
	o.mu.Unlock()
}

// Observation accessors expose no raw bytes or acceptance authority. They are
// private qualification surfaces, not serialized response fields.
func (m *Manager) DirectionalObservationsPrivate() []directionalraw.Observation {
	m.mu.Lock()
	d := m.directionalV5
	m.mu.Unlock()
	if d == nil {
		return nil
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	var out []directionalraw.Observation
	for _, a := range d.accepted {
		for _, o := range a.observations {
			o.Failure = string(a.failure)
			out = append(out, o)
		}
	}
	return out
}

// DirectionalFailurePrivate retains the exact typed Manager terminal failure,
// independently of the client's formatted error and observation string labels.
func (m *Manager) DirectionalFailurePrivate() session.Failure {
	m.mu.Lock()
	d := m.directionalV5
	m.mu.Unlock()
	if d == nil {
		return ""
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if len(d.accepted) == 0 {
		return ""
	}
	return d.accepted[len(d.accepted)-1].failure
}

func (m *Manager) DirectionalGenericTerminalPrivate() (active uint32, admissionClosed, releaseStarted, released bool) {
	m.mu.Lock()
	d := m.directionalV5
	m.mu.Unlock()
	if d == nil {
		return
	}
	o := d.owner
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.active, o.admissionClosed, o.releaseStarted, o.released
}
func (m *Manager) DirectionalC15EventsPrivate() []privateB4C15Event {
	m.mu.Lock()
	d := m.directionalV5
	m.mu.Unlock()
	if d == nil {
		return nil
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	return append([]privateB4C15Event(nil), d.c15Events...)
}
func (m *Manager) DirectionalAllocationBoundariesPrivate() []privateDirectionalAllocationV5 {
	m.mu.Lock()
	d := m.directionalV5
	m.mu.Unlock()
	if d == nil {
		return nil
	}
	d.account.mu.Lock()
	defer d.account.mu.Unlock()
	return append([]privateDirectionalAllocationV5(nil), d.allocationBoundaries...)
}

func (m *Manager) DirectionalAccountingPrivate() (live, cumulative, allocations, releases, general uint64, occupied bool) {
	m.mu.Lock()
	d := m.directionalV5
	m.mu.Unlock()
	if d == nil {
		return
	}
	a := d.account
	a.mu.Lock()
	defer a.mu.Unlock()
	p := a.directionalV5
	return p.live, p.cumulative, p.allocations, p.releases, a.table.general.live, p.occupied
}
