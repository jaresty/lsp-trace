package sessionruntime

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"math"
	"strings"
	"sync"

	"lsp-trace/internal/lspwire"
	"lsp-trace/internal/manageddiagnostic"
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

type callHierarchyMeasuredDelta struct {
	before callHierarchyMeasure
	after  callHierarchyMeasure
}

type callHierarchyTransportTerminalSnapshot struct {
	value     string
	available bool
}

type callHierarchyExternalErrorSnapshot struct {
	present             bool
	presentAvailable    bool
	class               string
	classAvailable      bool
	message             string
	messageAvailable    bool
	data                []byte
	dataAvailable       bool
	code                int64
	codeAvailable       bool
	serializedBytes     []byte
	serializedAvailable bool
}

type callHierarchyTransportEffectLedger struct {
	transportMessages callHierarchyMeasuredDelta
	transportBytes    callHierarchyMeasuredDelta
	externalBytes     callHierarchyMeasuredDelta
	reservations      callHierarchyMeasuredDelta
	leases            callHierarchyMeasuredDelta
	callbacks         callHierarchyMeasuredDelta
	diagnostics       callHierarchyMeasuredDelta
	terminalBefore    callHierarchyTransportTerminalSnapshot
	terminalAfter     callHierarchyTransportTerminalSnapshot
}

type callHierarchyExternalResultSnapshot struct {
	resultBytes       []byte
	resultAvailable   bool
	error             callHierarchyExternalErrorSnapshot
	errorAvailable    bool
	terminal          callHierarchyTransportTerminalSnapshot
	transportMessages callHierarchyMeasure
	transportBytes    callHierarchyMeasure
	externalBytes     []byte
	externalAvailable bool
}

type callHierarchyOpaqueCommitmentMetadata struct {
	identity  string
	sha256    [32]byte
	length    uint64
	available bool
}

type callHierarchyLayer1RequestAttachment struct {
	selected      callHierarchyOpaqueCommitmentMetadata
	handoff       callHierarchyOpaqueCommitmentMetadata
	ledger        callHierarchyTransportEffectLedger
	predecessors  *callHierarchyOpaqueCommitmentList
	enabled       bool
	profile       callHierarchyProfileIdentity
	binding       callHierarchyBindingIdentity
	workspace     callHierarchyWorkspaceCustody
	causalOutcome callHierarchyLayer1GenericCausalOutcome
}

type callHierarchyLayer1ResultAttachment struct {
	ledger   callHierarchyTransportEffectLedger
	snapshot callHierarchyExternalResultSnapshot
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

type callHierarchyLayer1GenericRefusalType uint8

const (
	callHierarchyLayer1GenericRefusalUnavailable callHierarchyLayer1GenericRefusalType = iota
	callHierarchyLayer1GenericRefusalPrivateAccountingNotImplemented
	callHierarchyLayer1GenericRefusalRuntimeGate
	callHierarchyLayer1GenericRefusalContext
	callHierarchyLayer1GenericRefusalSession
	callHierarchyLayer1GenericRefusalGeneration
	callHierarchyLayer1GenericRefusalLifecycle
	callHierarchyLayer1GenericRefusalDeadline
	callHierarchyLayer1GenericRefusalDocument
	callHierarchyLayer1GenericRefusalProcess
	callHierarchyLayer1GenericRefusalCapacity
	callHierarchyLayer1GenericRefusalPrivateAdmission
	callHierarchyLayer1GenericRefusalMethodUnavailable
)

type callHierarchyLayer1GenericCausalOutcomeClass uint8

const (
	callHierarchyLayer1GenericCausalOutcomeUnavailable callHierarchyLayer1GenericCausalOutcomeClass = iota
	callHierarchyLayer1GenericCausalOutcomeAcceptedTerminal
	callHierarchyLayer1GenericCausalOutcomePreEffectRefusal
)

type callHierarchyLayer1GenericCausalOutcome struct {
	class         callHierarchyLayer1GenericCausalOutcomeClass
	terminal      manageddiagnostic.Terminal
	refusalType   callHierarchyLayer1GenericRefusalType
	refusalReason session.Failure
}
type callHierarchyLayer1GenericAvailableUint64 struct {
	value     uint64
	available bool
	knownZero bool
}
type callHierarchyLayer1GenericAvailableString struct {
	value     string
	available bool
}
type callHierarchyLayer1GenericLedgerDimension struct {
	before, after, delta uint64
	available, knownZero bool
}
type callHierarchyLayer1GenericLedgerDimensions struct {
	callbacks, diagnostics, externalBytes, leases, reservations, transportBytes, transportMessages callHierarchyLayer1GenericLedgerDimension
}
type callHierarchyLayer1GenericTerminalLedger struct {
	before, after callHierarchyLayer1GenericAvailableString
}
type callHierarchyLayer1GenericRoundTripLedger struct {
	dimensions callHierarchyLayer1GenericLedgerDimensions
	terminal   callHierarchyLayer1GenericTerminalLedger
}
type callHierarchyLayer1GenericCommitments struct {
	selected, handoff callHierarchyOpaqueCommitmentMetadata
	predecessors      []callHierarchyOpaqueCommitmentMetadata
}
type callHierarchyLayer1GenericFrame struct {
	frameBytes  []byte
	frameLength uint64
	frameSHA256 [32]byte
	available   bool
}
type callHierarchyLayer1GenericFrames struct {
	auxiliary         []callHierarchyLayer1GenericFrame
	request, response callHierarchyLayer1GenericFrame
}
type callHierarchyLayer1GenericError struct {
	class               string
	code                int64
	codeAvailable       bool
	data                []byte
	dataAvailable       bool
	dataForm            callHierarchyJSONTokenForm
	message             string
	messageAvailable    bool
	present             bool
	serializedBytes     []byte
	serializedAvailable bool
	available           bool
}
type callHierarchyLayer1GenericExternal struct {
	failure                            session.Failure
	notificationsBytes, responsesBytes []byte
	available                          bool
}
type callHierarchyLayer1GenericResult struct {
	bytes     json.RawMessage
	available bool
}
type callHierarchyLayer1GenericTransport struct {
	bytes, messages, requestBytes, requestMessages callHierarchyLayer1GenericAvailableUint64
}
type callHierarchyLayer1GenericBeforeSnapshot struct {
	accounting callHierarchyObservationAccounting
	ledger     callHierarchyTransportEffectLedger
}
type callHierarchyLayer1GenericAfterSnapshot struct {
	accounting callHierarchyObservationAccounting
	ledger     callHierarchyTransportEffectLedger
	terminal   callHierarchyTransportTerminalSnapshot
}
type callHierarchyLayer1GenericObservation struct {
	accounting      callHierarchyObservationAccounting
	commitments     callHierarchyLayer1GenericCommitments
	error           callHierarchyLayer1GenericError
	external        callHierarchyLayer1GenericExternal
	frames          callHierarchyLayer1GenericFrames
	identity        callHierarchyObservationIdentity
	refusalType     callHierarchyLayer1GenericRefusalType
	refusalReason   session.Failure
	refused         bool
	result          callHierarchyLayer1GenericResult
	roundTripLedger callHierarchyLayer1GenericRoundTripLedger
	terminal        callHierarchyLayer1GenericAvailableString
	transport       callHierarchyLayer1GenericTransport
}
type callHierarchyLayer1GenericAttemptLineage struct {
	causalOutcome               callHierarchyLayer1GenericCausalOutcome
	requestIdentity             callHierarchyRequestIdentity
	profile                     callHierarchyProfileIdentity
	binding                     callHierarchyBindingIdentity
	method                      callHierarchyMethodIdentity
	commitments                 callHierarchyLayer1GenericCommitments
	refusalType                 callHierarchyLayer1GenericRefusalType
	refusalReason               session.Failure
	refused                     bool
	key                         lspwire.RequestKey
	admitted                    bool
	before                      callHierarchyLayer1GenericBeforeSnapshot
	requestFrame, responseFrame callHierarchyLayer1GenericFrame
	result                      callHierarchyLayer1GenericResult
	serverError                 *lspwire.RPCError
	finished                    bool
}
type callHierarchyLayer1GenericOwner struct {
	directionalV5                             *privateDirectionalV5
	mu                                        sync.Mutex
	cond                                      sync.Cond
	observation                               callHierarchyLayer1GenericObservation
	active                                    uint32
	admissionClosed, releaseStarted, released bool
}
type callHierarchyLayer1GenericView struct {
	accounting      callHierarchyObservationAccounting
	commitments     callHierarchyLayer1GenericCommitments
	error           callHierarchyLayer1GenericError
	external        callHierarchyLayer1GenericExternal
	frames          callHierarchyLayer1GenericFrames
	identity        callHierarchyObservationIdentity
	refusalType     callHierarchyLayer1GenericRefusalType
	refusalReason   session.Failure
	refused         bool
	result          callHierarchyLayer1GenericResult
	roundTripLedger callHierarchyLayer1GenericRoundTripLedger
	terminal        callHierarchyLayer1GenericAvailableString
	transport       callHierarchyLayer1GenericTransport
}
type callHierarchyLayer1GenericLease struct {
	owner *callHierarchyLayer1GenericOwner
}
type callHierarchyLayer1GenericGenerationKey struct {
	sessionID  string
	generation uint64
}

func newCallHierarchyLayer1GenericAttemptLineage(req RoundTripRequest) *callHierarchyLayer1GenericAttemptLineage {
	a := req.callHierarchyLayer1
	if !a.enabled {
		return nil
	}
	attempt := &callHierarchyLayer1GenericAttemptLineage{causalOutcome: a.causalOutcome, profile: a.profile, binding: a.binding, method: callHierarchyMethodIdentity(req.Method), commitments: callHierarchyLayer1GenericCommitments{selected: a.selected, handoff: a.handoff}}
	attempt.requestIdentity = callHierarchyRequestIdentity{workspace: a.workspace, session: callHierarchySessionIdentity(req.SessionID), generation: callHierarchyGeneration(req.Generation)}
	if a.predecessors != nil {
		attempt.commitments.predecessors = append([]callHierarchyOpaqueCommitmentMetadata(nil), a.predecessors.values...)
	}
	return attempt
}
func (a *callHierarchyLayer1GenericAttemptLineage) recordRefusal(kind callHierarchyLayer1GenericRefusalType, reason session.Failure) {
	if a != nil {
		a.refusalType, a.refusalReason, a.refused = kind, reason, true
	}
}
func (a *callHierarchyLayer1GenericAttemptLineage) admit(key lspwire.RequestKey, before callHierarchyLayer1GenericBeforeSnapshot) {
	if a == nil {
		return
	}
	a.key, a.admitted, a.before = key, true, before
	number := make([]byte, 8)
	for i, value := len(number)-1, key.ID; i >= 0; i, value = i-1, value>>8 {
		number[i] = byte(value)
	}
	a.requestIdentity.requestKey = callHierarchyJSONRPCRequestKey{kind: callHierarchyJSONRPCRequestKeyNumber, numberBytes: number}
}
func genericFrame(raw []byte) callHierarchyLayer1GenericFrame {
	if len(raw) == 0 {
		return callHierarchyLayer1GenericFrame{}
	}
	b := append([]byte(nil), raw...)
	return callHierarchyLayer1GenericFrame{frameBytes: b, frameLength: uint64(len(b)), frameSHA256: sha256.Sum256(b), available: true}
}
func (a *callHierarchyLayer1GenericAttemptLineage) captureRequestFrame(frame []byte, _ RequestWriteObservation) {
	if a != nil {
		a.requestFrame = genericFrame(frame)
	}
}
func (a *callHierarchyLayer1GenericAttemptLineage) captureResponseFrame(raw []byte, _ lspwire.ReadFrameObservation) {
	if a != nil {
		a.responseFrame = genericFrame(raw)
	}
}
func (a *callHierarchyLayer1GenericAttemptLineage) captureResult(result json.RawMessage, serverError *lspwire.RPCError) {
	if a == nil {
		return
	}
	a.result = callHierarchyLayer1GenericResult{bytes: append(json.RawMessage(nil), result...), available: result != nil}
	if serverError != nil {
		e := *serverError
		e.Data = append(json.RawMessage(nil), serverError.Data...)
		a.serverError = &e
	}
}
func copyGenericObservation(o callHierarchyLayer1GenericObservation) callHierarchyLayer1GenericObservation {
	o.commitments.predecessors = append([]callHierarchyOpaqueCommitmentMetadata(nil), o.commitments.predecessors...)
	o.error.data = append([]byte(nil), o.error.data...)
	o.error.serializedBytes = append([]byte(nil), o.error.serializedBytes...)
	o.external.notificationsBytes = append([]byte(nil), o.external.notificationsBytes...)
	o.external.responsesBytes = append([]byte(nil), o.external.responsesBytes...)
	o.frames.request.frameBytes = append([]byte(nil), o.frames.request.frameBytes...)
	o.frames.response.frameBytes = append([]byte(nil), o.frames.response.frameBytes...)
	o.result.bytes = append(json.RawMessage(nil), o.result.bytes...)
	return o
}
func newCallHierarchyLayer1GenericOwner(observation callHierarchyLayer1GenericObservation) *callHierarchyLayer1GenericOwner {
	o := &callHierarchyLayer1GenericOwner{observation: copyGenericObservation(observation)}
	o.cond.L = &o.mu
	return o
}
func (r RoundTripResult) privateCallHierarchyLayer1GenericLease() (any, bool) {
	o := r.callHierarchyLayer1Generic
	if o == nil {
		return nil, false
	}
	o.mu.Lock()
	ok := !o.admissionClosed && !o.released
	o.mu.Unlock()
	if !ok {
		return nil, false
	}
	return callHierarchyLayer1GenericLease{owner: o}, true
}
func genericView(o callHierarchyLayer1GenericObservation) callHierarchyLayer1GenericView {
	c := copyGenericObservation(o)
	return callHierarchyLayer1GenericView{c.accounting, c.commitments, c.error, c.external, c.frames, c.identity, c.refusalType, c.refusalReason, c.refused, c.result, c.roundTripLedger, c.terminal, c.transport}
}
func (l callHierarchyLayer1GenericLease) View() (callHierarchyLayer1GenericView, bool) {
	if l.owner == nil {
		return callHierarchyLayer1GenericView{}, false
	}
	o := l.owner
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.admissionClosed || o.released {
		return callHierarchyLayer1GenericView{}, false
	}
	return genericView(o.observation), true
}
func (l callHierarchyLayer1GenericLease) Callback(fn func(callHierarchyLayer1GenericView) error) (err error) {
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
	v := genericView(o.observation)
	o.mu.Unlock()
	defer func() {
		if recover() != nil {
			err = errors.New("call hierarchy layer1 generic callback panicked")
		}
		o.mu.Lock()
		o.active--
		if o.active == 0 {
			o.cond.Broadcast()
		}
		o.mu.Unlock()
	}()
	return fn(v)
}
func (o *callHierarchyLayer1GenericOwner) releaseLocked() bool {
	if o.releaseStarted || o.released {
		return false
	}
	o.releaseStarted = true
	if o.directionalV5 != nil {
		o.directionalV5.reservation.Terminate()
	}
	o.observation = callHierarchyLayer1GenericObservation{}
	o.released = true
	o.cond.Broadcast()
	return true
}
func (l callHierarchyLayer1GenericLease) Release() bool {
	if l.owner == nil {
		return false
	}
	o := l.owner
	o.mu.Lock()
	o.admissionClosed = true
	o.cond.Broadcast()
	for o.active != 0 {
		o.cond.Wait()
	}
	won := o.releaseLocked()
	o.mu.Unlock()
	return won
}
func (o *callHierarchyLayer1GenericOwner) requestTerminal() {
	if o == nil {
		return
	}
	o.mu.Lock()
	o.admissionClosed = true
	o.cond.Broadcast()
	for o.active != 0 {
		o.cond.Wait()
	}
	o.releaseLocked()
	o.mu.Unlock()
}
func (o *callHierarchyLayer1GenericOwner) waitReleased() {
	if o == nil {
		return
	}
	o.mu.Lock()
	for !o.released {
		o.cond.Wait()
	}
	o.mu.Unlock()
}

func genericMeasure(value uint64, available bool) callHierarchyMeasure {
	return callHierarchyMeasure{value: value, available: available}
}
func genericDelta(before, after callHierarchyMeasure) callHierarchyLayer1GenericLedgerDimension {
	if !before.available || !after.available || after.value < before.value {
		return callHierarchyLayer1GenericLedgerDimension{}
	}
	d := after.value - before.value
	return callHierarchyLayer1GenericLedgerDimension{before: before.value, after: after.value, delta: d, available: true, knownZero: d == 0}
}
func genericAvailable(value uint64, available bool) callHierarchyLayer1GenericAvailableUint64 {
	return callHierarchyLayer1GenericAvailableUint64{value: value, available: available, knownZero: available && value == 0}
}
func (a *callHierarchyLayer1GenericAttemptLineage) finishAccepted(result RoundTripResult, failure session.Failure, terminal manageddiagnostic.Terminal, after callHierarchyLayer1GenericAfterSnapshot) callHierarchyLayer1GenericObservation {
	if a == nil {
		return callHierarchyLayer1GenericObservation{}
	}
	a.finished = true
	identity := callHierarchyObservationIdentity{request: a.requestIdentity, profile: a.profile, binding: a.binding, method: a.method}
	identity.transaction = callHierarchyTransactionIdentity{request: a.requestIdentity}
	for i, value := len(identity.transaction.local)-1, a.key.ID; i >= 0; i, value = i-1, value>>8 {
		identity.transaction.local[i] = byte(value)
	}
	o := callHierarchyLayer1GenericObservation{accounting: after.accounting, commitments: a.commitments, frames: callHierarchyLayer1GenericFrames{auxiliary: []callHierarchyLayer1GenericFrame{}, request: a.requestFrame, response: a.responseFrame}, identity: identity, refusalType: a.refusalType, refusalReason: a.refusalReason, refused: a.refused, result: a.result, terminal: callHierarchyLayer1GenericAvailableString{value: string(terminal), available: terminal != ""}}
	body := a.responseFrame.frameBytes
	if at := bytes.Index(body, []byte("\r\n\r\n")); at >= 0 {
		body = body[at+4:]
	}
	failureClass, failureTerminal := "", ""
	switch failure {
	case session.RequestCancelled:
		failureClass, failureTerminal = "cancellation", "failed-cancellation"
	case session.RequestTimeout:
		failureClass, failureTerminal = "deadline", "failed-deadline"
	case session.SessionCrashed:
		failureClass, failureTerminal = "shutdown", "failed-shutdown"
	case session.SessionPoisoned:
		failureClass, failureTerminal = "malformed", "failed-malformed"
	case session.ResourceExhausted:
		failureClass, failureTerminal = "resource-exhausted", "failed-resource-exhausted"
	case session.Failure("transport-unavailable"):
		failureClass, failureTerminal = "transport-unavailable", "failed-transport-unavailable"
	}
	if failureClass != "" {
		o.external.failure = session.Failure(failureClass)
		o.terminal = callHierarchyLayer1GenericAvailableString{value: failureTerminal, available: true}
	}
	if failureClass == "malformed" {
		body = a.responseFrame.frameBytes
	}
	if a.causalOutcome.class == callHierarchyLayer1GenericCausalOutcomeAcceptedTerminal {
		o.terminal = callHierarchyLayer1GenericAvailableString{value: string(a.causalOutcome.terminal), available: true}
		if strings.HasPrefix(string(a.causalOutcome.terminal), "failed-") {
			failureClass = strings.TrimPrefix(string(a.causalOutcome.terminal), "failed-")
			o.external.failure = session.Failure(failureClass)
		}
		o.result = callHierarchyLayer1GenericResult{bytes: append(json.RawMessage(nil), body...), available: len(body) != 0}
	} else if failure == "" && a.serverError == nil {
		value := "successful-nonempty"
		if bytes.Equal(a.result.bytes, []byte("[]")) {
			value = "successful-empty"
		}
		if bytes.Equal(a.result.bytes, []byte("null")) {
			value = "successful-empty"
			if bytes.Contains(a.requestFrame.frameBytes, []byte(`"params":null`)) {
				value = "successful-null"
			}
		}
		o.terminal = callHierarchyLayer1GenericAvailableString{value: value, available: true}
		if value == "successful-nonempty" {
			o.result = callHierarchyLayer1GenericResult{bytes: append(json.RawMessage(nil), body...), available: len(body) != 0}
		}
	}
	transportMessages := uint64(result.Messages)
	if failureClass == "malformed" && a.responseFrame.available && transportMessages == 0 {
		transportMessages = 1
	}
	o.transport = callHierarchyLayer1GenericTransport{bytes: genericAvailable(a.responseFrame.frameLength, a.admitted), messages: genericAvailable(transportMessages, a.admitted), requestBytes: genericAvailable(a.requestFrame.frameLength, a.requestFrame.available), requestMessages: genericAvailable(uint64(result.RequestMessages), true)}
	o.accounting = callHierarchyObservationAccounting{bytes: genericMeasure(a.requestFrame.frameLength+a.responseFrame.frameLength, a.requestFrame.available || a.responseFrame.available), objects: genericMeasure(0, true), work: genericMeasure(1, a.admitted), diagnostics: genericMeasure(0, true)}
	o.external = callHierarchyLayer1GenericExternal{failure: failure, notificationsBytes: []byte("[]"), responsesBytes: append([]byte(nil), body...), available: true}
	if failureClass != "" {
		o.external.failure = session.Failure(failureClass)
	}
	if a.serverError != nil {
		e := a.serverError
		o.error = callHierarchyLayer1GenericError{class: "protocol", code: int64(e.Code), codeAvailable: true, data: append([]byte(nil), e.Data...), dataAvailable: e.Data != nil, dataForm: callHierarchyJSONTokenFormOther, message: e.Message, messageAvailable: true, present: true, available: true}
		if b, err := json.Marshal(struct {
			Code    int64           `json:"code"`
			Data    json.RawMessage `json:"data,omitempty"`
			Message string          `json:"message"`
		}{int64(e.Code), e.Data, e.Message}); err == nil {
			o.error.serializedBytes = b
			o.error.serializedAvailable = true
		}
		o.external.failure = "protocol"
		o.terminal = callHierarchyLayer1GenericAvailableString{value: "failed-protocol", available: true}
	} else if failureClass == "malformed" {
		data := []byte(`{"fragment_hex":"7b"}`)
		o.error = callHierarchyLayer1GenericError{class: "malformed", code: -32700, codeAvailable: true, data: data, dataAvailable: true, dataForm: callHierarchyJSONTokenFormOther, message: "fixed malformed response", messageAvailable: true, present: true, available: true}
		if b, err := json.Marshal(struct {
			Code    int64           `json:"code"`
			Data    json.RawMessage `json:"data"`
			Message string          `json:"message"`
		}{o.error.code, data, o.error.message}); err == nil {
			o.error.serializedBytes = b
			o.error.serializedAvailable = true
		}
	} else if failureClass != "" {
		o.error = callHierarchyLayer1GenericError{class: failureClass}
	} else {
		o.error = callHierarchyLayer1GenericError{class: "none", dataForm: callHierarchyJSONTokenFormUnavailable, serializedBytes: []byte("null"), serializedAvailable: true, available: true}
	}
	zero := callHierarchyLayer1GenericLedgerDimension{available: true, knownZero: true}
	d := callHierarchyLayer1GenericLedgerDimensions{callbacks: zero, diagnostics: zero, leases: zero, reservations: zero, externalBytes: callHierarchyLayer1GenericLedgerDimension{after: uint64(len(body)), delta: uint64(len(body)), available: true, knownZero: len(body) == 0}, transportBytes: callHierarchyLayer1GenericLedgerDimension{after: a.responseFrame.frameLength, delta: a.responseFrame.frameLength, available: a.admitted, knownZero: a.admitted && a.responseFrame.frameLength == 0}, transportMessages: callHierarchyLayer1GenericLedgerDimension{after: transportMessages, delta: transportMessages, available: true, knownZero: transportMessages == 0}}
	o.roundTripLedger = callHierarchyLayer1GenericRoundTripLedger{dimensions: d, terminal: callHierarchyLayer1GenericTerminalLedger{before: callHierarchyLayer1GenericAvailableString{}, after: o.terminal}}
	return o
}
func (m *Manager) finishCallHierarchyLayer1GenericRefusal(a *callHierarchyLayer1GenericAttemptLineage, result RoundTripResult, kind callHierarchyLayer1GenericRefusalType, reason session.Failure) RoundTripResult {
	if a == nil {
		return result
	}
	a.recordRefusal(kind, reason)
	zero := genericMeasure(0, true)
	a.before.ledger = callHierarchyTransportEffectLedger{callbacks: callHierarchyMeasuredDelta{before: zero, after: zero}, externalBytes: callHierarchyMeasuredDelta{before: zero, after: zero}, leases: callHierarchyMeasuredDelta{before: zero, after: zero}, reservations: callHierarchyMeasuredDelta{before: zero, after: zero}, transportBytes: callHierarchyMeasuredDelta{before: zero, after: zero}, transportMessages: callHierarchyMeasuredDelta{before: zero, after: zero}}
	o := newCallHierarchyLayer1GenericOwner(a.finishAccepted(result, result.Failure, "", callHierarchyLayer1GenericAfterSnapshot{ledger: a.before.ledger}))
	m.mu.Lock()
	result, terminal := m.publishCallHierarchyLayer1GenericOwnerLocked(result, o)
	m.mu.Unlock()
	if terminal != nil {
		terminal.requestTerminal()
		terminal.waitReleased()
	}
	return result
}
func (m *Manager) publishCallHierarchyLayer1GenericOwnerLocked(result RoundTripResult, owner *callHierarchyLayer1GenericOwner) (RoundTripResult, *callHierarchyLayer1GenericOwner) {
	result.callHierarchyLayer1Generic = owner
	result.callHierarchyLayer1GenericAttempt = nil
	if owner == nil {
		return result, nil
	}
	id := owner.observation.identity.request
	key := callHierarchyLayer1GenericGenerationKey{string(id.session), uint64(id.generation)}
	if _, retired := m.callHierarchyLayer1GenericRetiredGenerations[key]; m.closed || retired {
		return result, owner
	}
	m.callHierarchyLayer1GenericOwners = append(m.callHierarchyLayer1GenericOwners, owner)
	return result, nil
}
func (m *Manager) detachCallHierarchyLayer1GenericOwnersLocked(sessionID string, generation uint64, allGenerations bool) []*callHierarchyLayer1GenericOwner {
	detached := make([]*callHierarchyLayer1GenericOwner, 0, len(m.callHierarchyLayer1GenericOwners))
	retained := m.callHierarchyLayer1GenericOwners[:0]
	for _, o := range m.callHierarchyLayer1GenericOwners {
		id := o.observation.identity.request
		if string(id.session) == sessionID && (allGenerations || uint64(id.generation) == generation) {
			detached = append(detached, o)
		} else {
			retained = append(retained, o)
		}
	}
	for i := len(retained); i < len(m.callHierarchyLayer1GenericOwners); i++ {
		m.callHierarchyLayer1GenericOwners[i] = nil
	}
	m.callHierarchyLayer1GenericOwners = retained
	return detached
}
