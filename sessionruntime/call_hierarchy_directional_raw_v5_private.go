package sessionruntime

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"sync"
	"time"

	"lsp-trace/internal/directionalraw"
	"lsp-trace/internal/lsp"
	"lsp-trace/internal/lspwire"
	"lsp-trace/internal/session"
)

const privateDirectionalMaxV5 uint64 = 32 << 20
const privateDirectionalCombinedMaxV5 uint64 = 64 << 20

type privateDirectionalPartitionV5 struct {
	live, cumulative, allocations, releases uint64
	directionLive                           [2]uint64
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
	allocationBoundaries   []privateDirectionalAllocationV5
	mu                     sync.Mutex
	reservation            directionalraw.Reservation
	account                *privateB4ByteAccountV2
	owner                  *callHierarchyLayer1GenericOwner
	accepted               []acceptedV2
	c15Events              []privateB4C15Event
	frameObjects           map[uint64]string
	responseObjects        map[lspwire.RequestKey]uint64
	preparationLive        uint64
	preparationCumulative  uint64
	preparationAllocations uint64
	preparationReleases    uint64
	resourceRefusal        session.Failure
}
type privateDirectionalCallV5 struct {
	execution *privateDirectionalV5
	lease     directionalraw.Lease
	key       lspwire.RequestKey
	mu        sync.Mutex
	terminal  uint8
}

const (
	privateDirectionalPendingV5 uint8 = iota
	privateDirectionalCompletedV5
	privateDirectionalCancelledV5
)

func (c *privateDirectionalCallV5) acceptCompletedResponse(pending *lspwire.Pending, key lspwire.ResponseKey) (lspwire.ResponseDisposition, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.terminal != privateDirectionalPendingV5 {
		return lspwire.ResponseUnknown, false
	}
	disposition := pending.Accept(key)
	if disposition != lspwire.ResponseAccepted {
		return disposition, false
	}
	c.terminal = privateDirectionalCompletedV5
	return disposition, true
}

func (c *privateDirectionalCallV5) cancelBeforeCompletion() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.terminal != privateDirectionalPendingV5 {
		return false
	}
	c.terminal = privateDirectionalCancelledV5
	return true
}

type DirectionalCallHierarchyRequestPrivate struct {
	SessionID, URI, LanguageID, Symbol string
	Generation                         uint64
	Line, Character                    *uint32
	Deadline                           time.Time
	MaxMessages                        int
	MaxBytes                           int64
}

type DirectionalWireReceiptPrivate struct {
	method       string
	metadata     directionalraw.CompletionMetadataPrivate
	observations []directionalraw.Observation
}

func (r DirectionalWireReceiptPrivate) MethodPrivate() string    { return r.method }
func (r DirectionalWireReceiptPrivate) RequestIDPrivate() uint64 { return r.metadata.RequestIDPrivate }
func (r DirectionalWireReceiptPrivate) GenerationPrivate() uint64 {
	return r.metadata.GenerationPrivate
}
func (r DirectionalWireReceiptPrivate) RequestLengthPrivate() uint64 {
	return r.metadata.RequestLengthPrivate
}
func (r DirectionalWireReceiptPrivate) ResponseLengthPrivate() uint64 {
	return r.metadata.ResponseLengthPrivate
}
func (r DirectionalWireReceiptPrivate) ResultLengthPrivate() uint64 {
	return r.metadata.ResultLengthPrivate
}
func (r DirectionalWireReceiptPrivate) RequestSHA256Private() [32]byte {
	return r.metadata.RequestSHA256Private
}
func (r DirectionalWireReceiptPrivate) ResponseSHA256Private() [32]byte {
	return r.metadata.ResponseSHA256Private
}
func (r DirectionalWireReceiptPrivate) ResultSHA256Private() [32]byte {
	return r.metadata.ResultSHA256Private
}
func (r DirectionalWireReceiptPrivate) ObservationsPrivate() []directionalraw.Observation {
	return append([]directionalraw.Observation(nil), r.observations...)
}

type DirectionalCallHierarchyResultPrivate struct {
	Incoming                       []lsp.CallHierarchyIncomingCall
	Outgoing                       []lsp.CallHierarchyOutgoingCall
	IncomingWire, OutgoingWire     DirectionalWireReceiptPrivate
	IncomingNull, OutgoingNull     bool
	Failure                        session.Failure
	FailurePostEffect              bool
	SessionPoisoned                bool
	RequestEffectObservedPrivate   bool
	TerminalPoisonObservedPrivate  bool
	RequestWriteCompletedPrivate   bool
	IncomingEffectCompletedPrivate bool
	OutgoingEffectCompletedPrivate bool
	CancellationPhasePrivate       string
	TerminalLifecyclePrivate       session.State
}

func decodeDirectionalPrivate(raw []byte, target any) error {
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(target); err != nil {
		return err
	}
	if d.Decode(&struct{}{}) != io.EOF {
		return errors.New("trailing result")
	}
	return nil
}

func flattenDirectionalSymbolsPrivate(symbols []lsp.DocumentSymbol) []lsp.DocumentSymbol {
	var out []lsp.DocumentSymbol
	var visit func([]lsp.DocumentSymbol)
	visit = func(values []lsp.DocumentSymbol) {
		for _, value := range values {
			out = append(out, value)
			visit(value.Children)
		}
	}
	visit(symbols)
	return out
}

func (m *Manager) directionalOrdinaryCallPrivate(ctx context.Context, req DirectionalCallHierarchyRequestPrivate, method string, params, target any) error {
	raw, err := json.Marshal(params)
	if err != nil {
		return err
	}
	observed := m.RoundTrip(ctx, RoundTripRequest{SessionID: req.SessionID, Generation: req.Generation, Method: method, Params: raw, Deadline: req.Deadline, MaxMessages: req.MaxMessages, MaxBytes: req.MaxBytes})
	if observed.Failure != "" {
		return errors.New(string(observed.Failure))
	}
	if observed.ServerError != nil {
		return fmt.Errorf("rpc %d: %s", observed.ServerError.Code, observed.ServerError.Message)
	}
	return decodeDirectionalPrivate(observed.Result, target)
}

func (m *Manager) prepareDirectionalItemPrivate(ctx context.Context, req DirectionalCallHierarchyRequestPrivate, reservation directionalraw.Reservation) (lsp.CallHierarchyItem, directionalraw.PreparedSelection, error) {
	position := lsp.Position{}
	if req.Symbol != "" {
		var symbols []lsp.DocumentSymbol
		if err := m.directionalOrdinaryCallPrivate(ctx, req, "textDocument/documentSymbol", lsp.DocumentSymbolParams{TextDocument: lsp.TextDocumentIdentifier{URI: req.URI}}, &symbols); err != nil {
			return lsp.CallHierarchyItem{}, directionalraw.PreparedSelection{}, err
		}
		var match *lsp.DocumentSymbol
		for _, symbol := range flattenDirectionalSymbolsPrivate(symbols) {
			if symbol.Name == req.Symbol && (symbol.Kind == 6 || symbol.Kind == 9 || symbol.Kind == 12) {
				if match != nil {
					return lsp.CallHierarchyItem{}, directionalraw.PreparedSelection{}, errors.New("document symbol ambiguous")
				}
				copy := symbol
				match = &copy
			}
		}
		if match == nil {
			return lsp.CallHierarchyItem{}, directionalraw.PreparedSelection{}, errors.New("document symbol absent")
		}
		position = match.SelectionRange.Start
	} else {
		if req.Line == nil || req.Character == nil {
			return lsp.CallHierarchyItem{}, directionalraw.PreparedSelection{}, errors.New("position unavailable")
		}
		position = lsp.Position{Line: *req.Line, Character: *req.Character}
	}
	raw, err := json.Marshal(lsp.PrepareCallHierarchyParams{TextDocument: lsp.TextDocumentIdentifier{URI: req.URI}, Position: position})
	if err != nil {
		return lsp.CallHierarchyItem{}, directionalraw.PreparedSelection{}, err
	}
	observed, lease := m.RoundTripDirectionalPrivate(reservation.Context(ctx), RoundTripRequest{SessionID: req.SessionID, Generation: req.Generation, Method: "textDocument/prepareCallHierarchy", Params: raw, Deadline: req.Deadline, MaxMessages: req.MaxMessages, MaxBytes: req.MaxBytes})
	clear(raw)
	if observed.Failure != "" {
		return lsp.CallHierarchyItem{}, directionalraw.PreparedSelection{}, errors.New(string(observed.Failure))
	}
	if observed.ServerError != nil {
		return lsp.CallHierarchyItem{}, directionalraw.PreparedSelection{}, fmt.Errorf("rpc %d: %s", observed.ServerError.Code, observed.ServerError.Message)
	}
	if err := lease.BindElementsPrivate(); err != nil {
		return lsp.CallHierarchyItem{}, directionalraw.PreparedSelection{}, err
	}
	var items []lsp.CallHierarchyItem
	if err := lease.Decode(&items); err != nil {
		return lsp.CallHierarchyItem{}, directionalraw.PreparedSelection{}, err
	}
	if len(items) != 1 || items[0].URI != req.URI {
		return lsp.CallHierarchyItem{}, directionalraw.PreparedSelection{}, errors.New("prepared identity unavailable")
	}
	selected, ok := directionalraw.Select(reservation.Context(ctx), directionalraw.Request{Method: "textDocument/prepareCallHierarchy", Ordinal: 0})
	if !ok {
		return lsp.CallHierarchyItem{}, directionalraw.PreparedSelection{}, errors.New("prepared lexical identity unavailable")
	}
	var same lsp.CallHierarchyItem
	if err := selected.DecodePrivate(&same); err != nil || same.URI != items[0].URI || same.Name != items[0].Name {
		return lsp.CallHierarchyItem{}, directionalraw.PreparedSelection{}, errors.New("prepared semantic identity unavailable")
	}
	return items[0], selected, nil
}

type directionalCallFailurePrivate struct {
	failure                session.Failure
	postEffect             bool
	poisoned               bool
	requestEffectObserved  bool
	terminalPoisonObserved bool
	requestCompleted       bool
	lifecycle              session.State
}

func (e directionalCallFailurePrivate) Error() string { return string(e.failure) }

func (e directionalCallFailurePrivate) Unwrap() error {
	if e.failure == session.RequestTimeout {
		return context.DeadlineExceeded
	}
	return nil
}

func (m *Manager) directionalPostEffectFailurePrivate(req DirectionalCallHierarchyRequestPrivate, failure session.Failure) directionalCallFailurePrivate {
	terminal := m.Poison(req.SessionID, req.Generation)
	if terminal.Failure != "" && terminal.Failure != session.SessionPoisoned {
		failure = terminal.Failure
	}
	return directionalCallFailurePrivate{
		failure: failure, postEffect: true, poisoned: terminal.State == session.Poisoned,
		requestEffectObserved: true, terminalPoisonObserved: terminal.State == session.Poisoned,
		requestCompleted: true, lifecycle: terminal.State,
	}
}

func (m *Manager) executeDirectionalCallPrivate(ctx context.Context, req DirectionalCallHierarchyRequestPrivate, reservation directionalraw.Reservation, method string, selected directionalraw.PreparedSelection, target any) (DirectionalWireReceiptPrivate, bool, error) {
	raw := make([]byte, 0, selected.View().Len()+9)
	raw = append(raw, `{"item":`...)
	var ok bool
	raw, ok = selected.AppendToPrivate(raw)
	if !ok {
		return DirectionalWireReceiptPrivate{}, false, errors.New("prepared lexical identity unavailable")
	}
	raw = append(raw, '}')
	defer clear(raw)
	observed, lease := m.RoundTripDirectionalPrivate(reservation.Context(ctx), RoundTripRequest{SessionID: req.SessionID, Generation: req.Generation, Method: method, Params: raw, Deadline: req.Deadline, MaxMessages: req.MaxMessages, MaxBytes: req.MaxBytes})
	cancelCompleted := false
	if observed.Failure == session.RequestCancelled {
		if bindErr := lease.BindElementsPrivate(); bindErr == nil {
			_, cancelCompleted = lease.CompletedMetadataPrivate()
		}
		if !cancelCompleted {
			return DirectionalWireReceiptPrivate{}, false, context.Canceled
		}
	}
	if observed.Failure != "" && !cancelCompleted {
		failure := observed.Failure
		if resourceFailure := m.directionalResourceFailurePrivate(); resourceFailure != "" {
			failure = resourceFailure
		} else if directionalFailure := m.DirectionalFailurePrivate(); directionalFailure != "" {
			failure = directionalFailure
		}
		_, requestCompleted := observed.CompletedRequestWrite()
		if requestCompleted {
			return DirectionalWireReceiptPrivate{}, false, m.directionalPostEffectFailurePrivate(req, failure)
		}
		if failure == session.RequestTimeout {
			return DirectionalWireReceiptPrivate{}, false, context.DeadlineExceeded
		}
		return DirectionalWireReceiptPrivate{}, false, directionalCallFailurePrivate{failure: failure}
	}
	_, requestCompleted := observed.CompletedRequestWrite()
	if observed.ServerError == nil && !cancelCompleted {
		if err := lease.BindElementsPrivate(); err != nil {
			if requestCompleted {
				return DirectionalWireReceiptPrivate{}, false, m.directionalPostEffectFailurePrivate(req, session.SessionPoisoned)
			}
			return DirectionalWireReceiptPrivate{}, false, err
		}
	}
	metadata, ok := lease.CompletedMetadataPrivate()
	if !ok {
		if requestCompleted {
			return DirectionalWireReceiptPrivate{}, false, m.directionalPostEffectFailurePrivate(req, session.SessionPoisoned)
		}
		return DirectionalWireReceiptPrivate{}, false, errors.New("directional completion unavailable")
	}
	isNull := lease.Null()
	if observed.ServerError == nil && !isNull {
		if err := lease.Decode(target); err != nil {
			if requestCompleted {
				return DirectionalWireReceiptPrivate{}, false, m.directionalPostEffectFailurePrivate(req, session.SessionPoisoned)
			}
			return DirectionalWireReceiptPrivate{}, false, err
		}
	}
	call := observed.directionalV5
	call.execution.mu.Lock()
	var observations []directionalraw.Observation
	for _, accepted := range call.execution.accepted {
		if accepted.key == call.key {
			observations = append(observations, accepted.observations...)
			break
		}
	}
	call.execution.mu.Unlock()
	receipt := DirectionalWireReceiptPrivate{method: method, metadata: metadata, observations: append([]directionalraw.Observation(nil), observations...)}
	if observed.ServerError != nil {
		return receipt, isNull, fmt.Errorf("rpc %d: %s", observed.ServerError.Code, observed.ServerError.Message)
	}
	return receipt, isNull, nil
}

func finalizeDirectionalReservationPrivate(reservation directionalraw.Reservation, result *DirectionalCallHierarchyResultPrivate, err *error) {
	if reservation.Release() {
		return
	}
	*err = errors.Join(*err, directionalraw.ErrTerminalReleasePrivate)
	result.Failure = session.Failure("DIRECTIONAL_RELEASE_FAILED")
}

func (m *Manager) ExecuteCallHierarchyDirectionsPrivate(ctx context.Context, req DirectionalCallHierarchyRequestPrivate) (result DirectionalCallHierarchyResultPrivate, err error) {
	if ctx == nil {
		return result, errors.New("nil context")
	}
	if err := ctx.Err(); err != nil {
		return result, err
	}
	document := m.PrepareDocument(ctx, DocumentRequest{SessionID: req.SessionID, Generation: req.Generation, URI: req.URI, LanguageID: req.LanguageID})
	if document.Failure != "" {
		result.Failure = document.Failure
		return result, errors.New(string(document.Failure))
	}
	reservation := directionalraw.NewReservation(fmt.Sprintf("directions:%s:%d:%d", req.SessionID, req.Generation, time.Now().UnixNano()))
	defer func() { finalizeDirectionalReservationPrivate(reservation, &result, &err) }()
	item, selected, err := m.prepareDirectionalItemPrivate(ctx, req, reservation)
	if err != nil {
		return result, err
	}
	_ = item // One semantic identity is validated against the selected token.
	result.IncomingWire, result.IncomingNull, err = m.executeDirectionalCallPrivate(ctx, req, reservation, "callHierarchy/incomingCalls", selected, &result.Incoming)
	if err != nil {
		var directionalFailure directionalCallFailurePrivate
		if errors.As(err, &directionalFailure) {
			result.Failure = directionalFailure.failure
			result.FailurePostEffect = directionalFailure.postEffect
			result.SessionPoisoned = directionalFailure.poisoned
			result.RequestEffectObservedPrivate = directionalFailure.requestEffectObserved
			result.TerminalPoisonObservedPrivate = directionalFailure.terminalPoisonObserved
			result.RequestWriteCompletedPrivate = directionalFailure.requestCompleted
			result.TerminalLifecyclePrivate = directionalFailure.lifecycle
		}
		if errors.Is(err, context.Canceled) {
			result.CancellationPhasePrivate = "BEFORE_INCOMING_EFFECT"
		}
		return result, err
	}
	result.IncomingEffectCompletedPrivate = true
	result.RequestEffectObservedPrivate = true
	result.RequestWriteCompletedPrivate = true
	if err := ctx.Err(); err != nil {
		result.CancellationPhasePrivate = "AFTER_INCOMING_BEFORE_OUTGOING_EFFECT"
		return result, err
	}
	result.OutgoingWire, result.OutgoingNull, err = m.executeDirectionalCallPrivate(ctx, req, reservation, "callHierarchy/outgoingCalls", selected, &result.Outgoing)
	if err != nil {
		var directionalFailure directionalCallFailurePrivate
		if errors.As(err, &directionalFailure) {
			result.Failure = directionalFailure.failure
			result.FailurePostEffect = directionalFailure.postEffect
			result.SessionPoisoned = directionalFailure.poisoned
			result.RequestEffectObservedPrivate = directionalFailure.requestEffectObserved
			result.TerminalPoisonObservedPrivate = directionalFailure.terminalPoisonObserved
			result.RequestWriteCompletedPrivate = directionalFailure.requestCompleted
			result.TerminalLifecyclePrivate = directionalFailure.lifecycle
		}
		if errors.Is(err, context.Canceled) && result.RequestEffectObservedPrivate {
			result.CancellationPhasePrivate = "AFTER_OUTGOING_REQUEST_EFFECT"
		}
		return result, err
	}
	result.OutgoingEffectCompletedPrivate = true
	result.RequestEffectObservedPrivate = true
	result.RequestWriteCompletedPrivate = true
	return result, nil
}

func (m *Manager) RoundTripDirectionalPrivate(ctx context.Context, req RoundTripRequest) (RoundTripResult, directionalraw.Lease) {
	r, ok := directionalraw.FromContext(ctx)
	if !ok || r.IdentityPrivate() == "" || !directionalraw.Allowed(req.Method) || req.EnablePrivateC16ObjectAccounting || req.EnablePrivateC17EventAccounting {
		return RoundTripResult{Failure: session.ToolNotImplemented}, directionalraw.Lease{}
	}
	req.directionalV5 = &privateDirectionalCallV5{execution: &privateDirectionalV5{reservation: r}}
	// Ordinary RoundTrip is deliberately not changed. The shared transaction owns
	// every session/generation/deadline/protocol and pending-key admission check.
	result := m.roundTripWithPrivateTransaction(ctx, req, nil, &privateB4Transaction{directionalV5: req.directionalV5})
	result.directionalV5 = req.directionalV5
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
		d = &privateDirectionalV5{reservation: r, frameObjects: make(map[uint64]string), responseObjects: make(map[lspwire.RequestKey]uint64)}
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
		if !r.AttachPrivate(d.allocate, d.observe, func() bool { return (callHierarchyLayer1GenericLease{owner: o}).Release() }, d.releaseAccount) {
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
func (d *privateDirectionalV5) allocate(method, component string, n int) ([]byte, error) {
	if component != "request-frame" && component != "response-frame" {
		return nil, errors.New("unknown directional component")
	}
	a := d.account
	a.mu.Lock()
	defer a.mu.Unlock()
	if method == "textDocument/prepareCallHierarchy" {
		if n <= 0 || a.terminalRequested || a.backingReleased || uint64(n) > privateDirectionalMaxV5-d.preparationLive {
			return nil, directionalraw.ErrReservationRefused
		}
		site := "c15.directional.v5.prepare-" + component
		objectID, failure := a.c15.create(site, privateB4C15Operation("allocation"))
		if failure != "" {
			return nil, errors.New(string(failure))
		}
		d.frameObjects[objectID] = site
		if component == "response-frame" {
			d.mu.Lock()
			for i := len(d.accepted) - 1; i >= 0; i-- {
				if d.accepted[i].method == method {
					d.responseObjects[d.accepted[i].key] = objectID
					break
				}
			}
			d.mu.Unlock()
		}
		d.preparationLive += uint64(n)
		d.preparationCumulative += uint64(n)
		d.preparationAllocations++
		d.allocationBoundaries = append(d.allocationBoundaries, privateDirectionalAllocationV5{
			Component: component, Length: n, Live: d.preparationLive, Cumulative: d.preparationCumulative,
			Allocations: d.preparationAllocations, OwnershipObjects: len(d.frameObjects), Occupied: true,
		})
		return make([]byte, n), nil
	}
	direction := 0
	if method == "callHierarchy/outgoingCalls" {
		direction = 1
	} else if method != "callHierarchy/incomingCalls" {
		return nil, errors.New("unknown directional method")
	}
	p := &a.directionalV5
	if n <= 0 || a.terminalRequested || a.backingReleased || uint64(n) > privateDirectionalMaxV5-p.directionLive[direction] || p.cumulative > math.MaxUint64-uint64(n) || a.table.general.live > privateB4MaxOwnedBytes || p.live+uint64(n) > privateDirectionalCombinedMaxV5 {
		d.mu.Lock()
		d.resourceRefusal = session.ResourceExhausted
		d.mu.Unlock()
		return nil, directionalraw.ErrReservationRefused
	}
	site := "c15.directional.v5." + component
	objectID, failure := a.c15.create(site, privateB4C15Operation("allocation"))
	if failure != "" {
		return nil, errors.New(string(failure))
	}
	d.frameObjects[objectID] = site
	if component == "response-frame" {
		d.mu.Lock()
		for i := len(d.accepted) - 1; i >= 0; i-- {
			if d.accepted[i].method == method {
				d.responseObjects[d.accepted[i].key] = objectID
				break
			}
		}
		d.mu.Unlock()
	}
	p.occupied = true
	p.live += uint64(n)
	p.directionLive[direction] += uint64(n)
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
	key := lspwire.RequestKey{Generation: o.Generation, ID: o.RequestID}
	if o.Component == "element" {
		a.c15.alias(d.responseObjects[key], "c15.directional.v5.selected-element")
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
	if p.occupied || d.preparationLive > 0 {
		for id, site := range d.frameObjects {
			a.c15.release(id, site)
		}
		if p.occupied {
			p.releases++
		}
		if d.preparationLive > 0 {
			d.preparationReleases++
			d.preparationLive = 0
		}
		p.live = 0
		p.directionLive = [2]uint64{}
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
	owners := append([]*callHierarchyLayer1GenericOwner(nil), m.callHierarchyLayer1GenericOwners...)
	current := m.directionalV5
	m.mu.Unlock()
	legacyPrepare := false
	for _, owner := range owners {
		if owner == nil || owner.directionalV5 == nil {
			continue
		}
		owner.directionalV5.mu.Lock()
		for _, accepted := range owner.directionalV5.accepted {
			if accepted.method == "textDocument/prepareCallHierarchy" {
				legacyPrepare = true
				break
			}
		}
		owner.directionalV5.mu.Unlock()
	}
	if legacyPrepare {
		owners = nil
		if current != nil {
			owners = append(owners, current.owner)
		}
	}
	var out []directionalraw.Observation
	for _, owner := range owners {
		if owner == nil || owner.directionalV5 == nil {
			continue
		}
		d := owner.directionalV5
		d.mu.Lock()
		for _, accepted := range d.accepted {
			for _, observation := range accepted.observations {
				observation.Failure = string(accepted.failure)
				out = append(out, observation)
			}
		}
		d.mu.Unlock()
	}
	return out
}

func (m *Manager) directionalResourceFailurePrivate() session.Failure {
	m.mu.Lock()
	d := m.directionalV5
	m.mu.Unlock()
	if d == nil {
		return ""
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.resourceRefusal
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
	owners := append([]*callHierarchyLayer1GenericOwner(nil), m.callHierarchyLayer1GenericOwners...)
	m.mu.Unlock()
	var frameAllocations uint64
	for _, owner := range owners {
		if owner == nil || owner.directionalV5 == nil {
			continue
		}
		a := owner.directionalV5.account
		a.mu.Lock()
		p := a.directionalV5
		live += p.live
		cumulative += p.cumulative
		frameAllocations += p.allocations
		releases += p.releases
		general += a.table.general.live
		occupied = occupied || p.occupied
		a.mu.Unlock()
	}
	return live, cumulative, frameAllocations / 2, releases, general, occupied
}

func (m *Manager) DirectionalPreparationAccountingPrivate() (live, cumulative, allocations, releases uint64, occupied bool) {
	m.mu.Lock()
	d := m.directionalV5
	m.mu.Unlock()
	if d == nil || d.account == nil {
		return
	}
	d.account.mu.Lock()
	defer d.account.mu.Unlock()
	return d.preparationLive, d.preparationCumulative, d.preparationAllocations, d.preparationReleases, d.preparationLive > 0
}

// Compile-only V2 overlay contract: preserve the baseline Manager body while
// pinning the exact directional transport, observation, and accounting surface.
var _ interface {
	RoundTripDirectionalPrivate(context.Context, RoundTripRequest) (RoundTripResult, directionalraw.Lease)
	DirectionalObservationsPrivate() []directionalraw.Observation
	DirectionalAccountingPrivate() (uint64, uint64, uint64, uint64, uint64, bool)
} = (*Manager)(nil)
